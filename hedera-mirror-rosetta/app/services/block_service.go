/*-
 * ‌
 * Hedera Mirror Node
 * ​
 * Copyright (C) 2019 - 2022 Hedera Hashgraph, LLC
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

package services

import (
	"context"

	"github.com/Code-Hex/go-generics-cache/policy/lru"
	"github.com/coinbase/rosetta-sdk-go/server"
	rTypes "github.com/coinbase/rosetta-sdk-go/types"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/domain/types"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/interfaces"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/tools"
)

// blockAPIService implements the server.BlockAPIServicer interface.
type blockAPIService struct {
	accountRepo interfaces.AccountRepository
	BaseService
	entityCache *lru.Cache[int64, types.AccountId]
}

// NewBlockAPIService creates a new instance of a blockAPIService.
func NewBlockAPIService(accountRepo interfaces.AccountRepository, baseService BaseService) server.BlockAPIServicer {
	entityCache := lru.NewCache[int64, types.AccountId](lru.WithCapacity(512 * 1024))
	return &blockAPIService{accountRepo: accountRepo, BaseService: baseService, entityCache: entityCache}
}

// Block implements the /block endpoint.
func (s *blockAPIService) Block(
	ctx context.Context,
	request *rTypes.BlockRequest,
) (*rTypes.BlockResponse, *rTypes.Error) {
	block, err := s.RetrieveBlock(ctx, request.BlockIdentifier)
	if err != nil {
		return nil, err
	}

	if block.Transactions, err = s.FindBetween(ctx, block.ConsensusStartNanos, block.ConsensusEndNanos); err != nil {
		return nil, err
	}

	if err = s.updateOperationAccountAlias(ctx, block.Transactions...); err != nil {
		return nil, err
	}

	return &rTypes.BlockResponse{Block: block.ToRosetta()}, nil
}

// BlockTransaction implements the /block/transaction endpoint.
func (s *blockAPIService) BlockTransaction(
	ctx context.Context,
	request *rTypes.BlockTransactionRequest,
) (*rTypes.BlockTransactionResponse, *rTypes.Error) {
	h := tools.SafeRemoveHexPrefix(request.BlockIdentifier.Hash)
	block, err := s.FindByIdentifier(ctx, request.BlockIdentifier.Index, h)
	if err != nil {
		return nil, err
	}

	transaction, err := s.FindByHashInBlock(
		ctx,
		request.TransactionIdentifier.Hash,
		block.ConsensusStartNanos,
		block.ConsensusEndNanos,
	)
	if err != nil {
		return nil, err
	}

	if err = s.updateOperationAccountAlias(ctx, transaction); err != nil {
		return nil, err
	}

	return &rTypes.BlockTransactionResponse{Transaction: transaction.ToRosetta()}, nil
}

func (s *blockAPIService) updateOperationAccountAlias(
	ctx context.Context,
	transactions ...*types.Transaction,
) *rTypes.Error {
	for _, transaction := range transactions {
		operations := transaction.Operations
		for index := range operations {
			id := operations[index].AccountId.GetId()
			if cached, ok := s.entityCache.Get(id); ok {
				operations[index].AccountId = cached
				continue
			}

			accountId, err := s.accountRepo.GetAccountAlias(ctx, operations[index].AccountId)
			if err != nil {
				return err
			}

			operations[index].AccountId = accountId
			s.entityCache.Set(id, accountId)
		}
	}

	return nil
}
