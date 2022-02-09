/*-
 * ‌
 * Hedera Mirror Node
 * ​
 * Copyright (C) 2019 - 2021 Hedera Hashgraph, LLC
 * ​
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * ‍
 */

package persistence

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash"
	"sync"

	rTypes "github.com/coinbase/rosetta-sdk-go/types"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/domain/types"
	hErrors "github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/errors"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/interfaces"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/persistence/domain"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/tools"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/sha3"
	"gorm.io/gorm"
)

const (
	accountBalanceBatchSize = 10000
	consensusTimestampUnset = -1

	// selectLatestWithIndex - Selects the latest record block
	selectLatestWithIndex string = `select consensus_start,
                                           consensus_end,
                                           hash,
                                           index,
                                           prev_hash
                                    from record_file
                                    order by index desc
                                    limit 1`

	// selectByHashWithIndex - Selects the row by given hash
	selectByHashWithIndex string = `select consensus_start,
                                           consensus_end,
                                           hash,
                                           index,
                                           prev_hash
                                    from record_file
                                    where hash = @hash`

	// selectGenesis - Selects the first block whose consensus_end is after the genesis account balance
	// timestamp. Return the record file with adjusted consensus start
	selectGenesis string = `select
                              consensus_end,
                              consensus_start,
                              hash,
                              index
                            from record_file rf
                            where consensus_end > @genesis_account_balance_timestamp
                            order by consensus_end
                            limit 1`

	selectNonZeroGenesisAccountBalance = `select account_id, balance
                                          from account_balance
                                          where balance <> 0 and account_id > @account_id
                                            and consensus_timestamp = @genesis_account_balance_timestamp
                                          order by account_id
                                          limit @limit`

	// selectRecordBlockByIndex - Selects the record block by its index
	selectRecordBlockByIndex string = `select consensus_start,
                                             consensus_end,
                                             hash,
                                             index,
                                             prev_hash
                                      from record_file
                                      where index = @index`
)

type recordBlock struct {
	ConsensusStart int64
	ConsensusEnd   int64
	Hash           string
	Index          int64
	PrevHash       string
}

func (rb *recordBlock) toBlock(genesisConsensusStart, genesisRecordIndex int64, genesisBlockHash string) *types.Block {
	consensusStart := rb.ConsensusStart
	// the genesis record file will have index 1 because index 0 is the generated block with a list of
	// operations to initialize account balances
	index := rb.Index - genesisRecordIndex + 1
	parentIndex := index - 1
	parentHash := rb.PrevHash

	// Handle the edge case for querying first record block
	if parentIndex == 0 {
		consensusStart = genesisConsensusStart
		parentHash = genesisBlockHash
	} else if parentIndex < 0 {
		parentIndex = 0
	}

	return &types.Block{
		Index:               index,
		Hash:                rb.Hash,
		ParentIndex:         parentIndex,
		ParentHash:          parentHash,
		ConsensusStartNanos: consensusStart,
		ConsensusEndNanos:   rb.ConsensusEnd,
	}
}

// blockRepository struct that has connection to the Database
type blockRepository struct {
	genesisInfoOnce                sync.Once
	genesisTransactionsOnce        sync.Once
	dbClient                       interfaces.DbClient
	genesisAccountBalanceTimestamp int64
	genesisBlock                   recordBlock
	genesisConsensusStart          int64
	genesisRecordFileIndex         int64
	genesisTransactions            []*types.Transaction
}

// NewBlockRepository creates an instance of a blockRepository struct
func NewBlockRepository(dbClient interfaces.DbClient) interfaces.BlockRepository {
	return &blockRepository{
		dbClient:                       dbClient,
		genesisAccountBalanceTimestamp: consensusTimestampUnset,
		genesisConsensusStart:          consensusTimestampUnset,
	}
}

func (br *blockRepository) FindByHash(ctx context.Context, hash string) (*types.Block, *rTypes.Error) {
	if hash == "" {
		return nil, hErrors.ErrInvalidArgument
	}

	if err := br.initGenesisRecordFile(ctx); err != nil {
		return nil, err
	}

	return br.findBlockByHash(ctx, hash)
}

func (br *blockRepository) FindByIdentifier(ctx context.Context, index int64, hash string) (
	*types.Block,
	*rTypes.Error,
) {
	if index < 0 || hash == "" {
		return nil, hErrors.ErrInvalidArgument
	}

	if err := br.initGenesisRecordFile(ctx); err != nil {
		return nil, err
	}

	block, err := br.findBlockByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	if block.Index != index {
		return nil, hErrors.ErrBlockNotFound
	}

	return block, nil
}

func (br *blockRepository) FindByIndex(ctx context.Context, index int64) (*types.Block, *rTypes.Error) {
	if index < 0 {
		return nil, hErrors.ErrInvalidArgument
	}

	if err := br.initGenesisRecordFile(ctx); err != nil {
		return nil, err
	}

	return br.findBlockByIndex(ctx, index)
}

func (br *blockRepository) RetrieveGenesis(ctx context.Context) (*types.Block, *rTypes.Error) {
	if err := br.initGenesisRecordFile(ctx); err != nil {
		return nil, err
	}

	return br.findBlockByIndex(ctx, 0)
}

func (br *blockRepository) RetrieveGenesisTransactions(_ context.Context) ([]*types.Transaction, *rTypes.Error) {
	if br.genesisTransactions != nil {
		return br.genesisTransactions, nil
	}

	// this will run the function only once and lock any other calls till it finishes
	br.genesisTransactionsOnce.Do(func() {
		accountId := int64(0)
		accountBalancesBatch := make([]domain.AccountBalance, 0, accountBalanceBatchSize)
		index := int64(0)

		hasher := hashTimestamp(br.genesisAccountBalanceTimestamp)
		hasher.Write([]byte("transactions")) // make it different from the block hash
		data := hasher.Sum(make([]byte, 0, 48))
		transaction := &types.Transaction{Hash: tools.SafeAddHexPrefix(hex.EncodeToString(data))}
		for {
			db := br.dbClient.GetDb()

			if err := db.Raw(
				selectNonZeroGenesisAccountBalance, sql.Named("account_id", accountId),
				sql.Named("genesis_account_balance_timestamp", br.genesisAccountBalanceTimestamp),
				sql.Named("limit", accountBalanceBatchSize),
			).Scan(&accountBalancesBatch).Error; err != nil {
				panic(err)
			}

			for _, accountBalance := range accountBalancesBatch {
				transaction.Operations = append(transaction.Operations, &types.Operation{
					Index:   index,
					Type:    types.OperationTypeCryptoTransfer,
					Status:  "SUCCESS",
					Account: types.Account{EntityId: accountBalance.AccountId},
					Amount:  &types.HbarAmount{Value: accountBalance.Balance},
				})

				index++
			}

			if len(accountBalancesBatch) < accountBalanceBatchSize {
				break
			}

			accountId = accountBalancesBatch[len(accountBalancesBatch)-1].AccountId.EncodedId
			accountBalancesBatch = accountBalancesBatch[:0]
		}

		log.Infof("Retreived %d postive genesis account balance", len(transaction.Operations))
		br.genesisTransactions = []*types.Transaction{transaction}
	})

	return br.genesisTransactions, nil
}

func (br *blockRepository) RetrieveLatest(ctx context.Context) (*types.Block, *rTypes.Error) {
	if err := br.initGenesisRecordFile(ctx); err != nil {
		return nil, err
	}

	db, cancel := br.dbClient.GetDbWithContext(ctx)
	defer cancel()

	rb := recordBlock{}
	if err := db.Raw(selectLatestWithIndex).First(&rb).Error; err != nil {
		return nil, handleDatabaseError(err, hErrors.ErrBlockNotFound)
	}

	if rb.Index < br.genesisRecordFileIndex {
		return nil, hErrors.ErrBlockNotFound
	}

	return rb.toBlock(br.genesisConsensusStart, br.genesisRecordFileIndex, br.genesisBlock.Hash), nil
}

func (br *blockRepository) findBlockByIndex(ctx context.Context, index int64) (*types.Block, *rTypes.Error) {
	rb := br.genesisBlock
	if index > 0 {
		db, cancel := br.dbClient.GetDbWithContext(ctx)
		defer cancel()

		dbIndex := br.genesisRecordFileIndex + (index - 1)
		if err := db.Raw(selectRecordBlockByIndex, sql.Named("index", dbIndex)).First(&rb).Error; err != nil {
			return nil, handleDatabaseError(err, hErrors.ErrBlockNotFound)
		}
	}

	return rb.toBlock(br.genesisConsensusStart, br.genesisRecordFileIndex, br.genesisBlock.Hash), nil
}

func (br *blockRepository) findBlockByHash(ctx context.Context, hash string) (*types.Block, *rTypes.Error) {
	rb := br.genesisBlock
	if hash != br.genesisBlock.Hash {
		db, cancel := br.dbClient.GetDbWithContext(ctx)
		defer cancel()

		if err := db.Raw(selectByHashWithIndex, sql.Named("hash", hash)).First(&rb).Error; err != nil {
			return nil, handleDatabaseError(err, hErrors.ErrBlockNotFound)
		}
	}

	return rb.toBlock(br.genesisConsensusStart, br.genesisRecordFileIndex, br.genesisBlock.Hash), nil
}

func (br *blockRepository) initGenesisRecordFile(ctx context.Context) *rTypes.Error {
	if br.genesisConsensusStart != consensusTimestampUnset {
		return nil
	}

	db, cancel := br.dbClient.GetDbWithContext(ctx)
	defer cancel()

	var genesisAccountBalanceTimestamp int64
	if err := db.Raw(genesisTimestampQuery).Scan(&genesisAccountBalanceTimestamp).Error; err != nil {
		log.Debug("Failed to get genesis account balance timestamp", err)
		return handleDatabaseError(err, hErrors.ErrNodeIsStarting)
	}
	if genesisAccountBalanceTimestamp == 0 {
		log.Debug("Genesis account balance timestamp is 0, most likely node is starting")
		return hErrors.ErrNodeIsStarting
	}

	rb := &recordBlock{}
	if err := db.Raw(
		selectGenesis,
		sql.Named("genesis_account_balance_timestamp", genesisAccountBalanceTimestamp),
	).First(rb).Error; err != nil {
		return handleDatabaseError(err, hErrors.ErrNodeIsStarting)
	}

	if rb.ConsensusStart <= genesisAccountBalanceTimestamp {
		rb.ConsensusStart = genesisAccountBalanceTimestamp + 1
	}

	br.genesisInfoOnce.Do(func() {
		bytes := hashTimestamp(genesisAccountBalanceTimestamp).Sum(make([]byte, 0, 48))
		blockHash := hex.EncodeToString(bytes)
		br.genesisAccountBalanceTimestamp = genesisAccountBalanceTimestamp
		br.genesisBlock = recordBlock{
			ConsensusStart: genesisAccountBalanceTimestamp,
			ConsensusEnd:   genesisAccountBalanceTimestamp,
			Hash:           blockHash,
			Index:          rb.Index - 1,
			PrevHash:       blockHash,
		}
		br.genesisConsensusStart = rb.ConsensusStart
		br.genesisRecordFileIndex = rb.Index
	})

	log.Infof("Fetched genesis info, account balance timestamp - %d, genesis record index - %d, consensus start - %d",
		br.genesisAccountBalanceTimestamp, br.genesisRecordFileIndex, br.genesisConsensusStart)
	return nil
}

func handleDatabaseError(err error, recordNotFoundErr *rTypes.Error) *rTypes.Error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return recordNotFoundErr
	}

	log.Errorf(databaseErrorFormat, hErrors.ErrDatabaseError.Message, err)
	return hErrors.ErrDatabaseError
}

func hashTimestamp(timestamp int64) hash.Hash {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, uint64(timestamp))
	hasher := sha3.New384()
	hasher.Write(data)
	return hasher
}
