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

package account

import (
	"database/sql"
	"errors"

	rTypes "github.com/coinbase/rosetta-sdk-go/types"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/domain/repositories"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/domain/types"
	hErrors "github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/errors"
	"gorm.io/gorm"
)

const (
	tableNameAccountBalance = "account_balance"
	tableNameTokenBalance = "token_balance"
)

const (
	balanceChangeBetween string = `select
                                     sum(amount::bigint) as value,
                                     json_agg(json_build_object("token_id", tbc.token_id::text, "value", tbc.value))
                                       filter (where tbc.token_id is not null) 
                                   from crypto_transfer
                                   full outer join (
                                     select account_id, token_id, sum(amount::bigint) as value
                                     from token_transfer
                                     where consensus_timestamp > @start and consensus_timestamp <= @end
                                     group by account_id, token_id
                                   ) tbc on tbc.account_id = entity_id
                                   where
                                     consensus_timestamp > @start and
                                     consensus_timestamp <= @end and
                                     entity_id = @account_id`

	latestBalanceBeforeConsensus string = `select
                                             ab.consensus_timestamp,
                                             ab.account_id,
                                             ab.balance,
                                             json_agg(
                                               json_build_object(
                                                 'token_id', tb.token_id::text,
                                                 'balance', tb.balance
                                               ) order by token_id
                                             ) token_balances
                                         from (
                                           select max(consensus_timestamp)
                                           from account_balance_file where consensus_timestamp<= @timestamp
                                         ) abm
                                         join account_balance ab on ab.consensus_timestamp = abm.max
                                         join token_balance tb
                                           on tb.consensus_timestamp = abm.max and tb.account_id = ab.account_id
                                         where ab.account_id = @account_id
                                         group by ab.consensus_timestamp, ab.account_id, ab.balance`
)

type accountBalance struct {
	ConsensusTimestamp int64 `gorm:"type:bigint;primaryKey"`
	AccountId          int64 `gorm:"type:bigint;primaryKey"`
	Balance            int64 `gorm:"type:bigint"`
}

type balanceChange struct {
	Value             int64 `gorm:"type:bigint"`
	NumberOfTransfers int64 `gorm:"type:bigint"`
}

// TableName - Set table name of the accountBalance to be `account_balance`
func (accountBalance) TableName() string {
	return tableNameAccountBalance
}

type tokenBalance struct {
	ConsensusTimestamp int64 `gorm:"type:bigint;primaryKey"`
	AccountId          int64 `gorm:"type:bigint;primaryKey"`
	Balance            int64 `gorm:"type:bigint"`
	TokenId            int64 `gorm:"type:bigint;primaryKey"`
}

// TableName - Set table name of the accountBalance to be `account_balance`
func (tokenBalance) TableName() string {
	return tableNameTokenBalance
}

type combinedAccountBalance struct {
	ConsensusTimestamp int64 `gorm:"type:bigint"`
	AccountId          int64 `gorm:"type:bigint"`
	Balance            int64 `gorm:"type:bigint"`
	TokenBalances      string `gorm:"type:text"`
}

// accountRepository struct that has connection to the Database
type accountRepository struct {
	dbClient *gorm.DB
}

// NewAccountRepository creates an instance of a accountRepository struct
func NewAccountRepository(dbClient *gorm.DB) repositories.AccountRepository {
	return &accountRepository{
		dbClient: dbClient,
	}
}

// RetrieveBalanceAtBlock returns the balance of the account at a given block (provided by consensusEnd timestamp).
// balance = balanceAtLatestBalanceSnapshot + balanceChangeBetweenSnapshotAndBlock
func (ar *accountRepository) RetrieveBalanceAtBlock(
	addressStr string,
	consensusEnd int64,
) ([]*types.Amount, *rTypes.Error) {
	acc, err := types.AccountFromString(addressStr)
	if err != nil {
		return nil, err
	}
	entityID, err1 := acc.ComputeEncodedID()
	if err1 != nil {
		return nil, hErrors.ErrInvalidAccount
	}

	// gets the most recent balance before block
	cb := &combinedAccountBalance{}
	result := ar.dbClient.Raw(
		latestBalanceBeforeConsensus,
		sql.Named("account_id", entityID),
		sql.Named("timestamp", consensusEnd),
	).
		First(&cb)
	if result.Error != nil && errors.Is(result.Error, gorm.ErrRecordNotFound) {
		cb.Balance = 0
	}

	r := &balanceChange{}
	// gets the balance change from the Balance snapshot until the target block
	ar.dbClient.Raw(
		balanceChangeBetween,
		sql.Named("start", cb.ConsensusTimestamp),
		sql.Named("end", consensusEnd),
		sql.Named("account_id", entityID),
	).
		Scan(r)

	// return &types.Amount{
	// 	Value: ab.Balance + r.Value,
	// }, nil

	return nil, nil
}
