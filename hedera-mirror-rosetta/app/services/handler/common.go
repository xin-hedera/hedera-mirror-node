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

package handler

import (
	"encoding/json"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/go-playground/validator/v10"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/domain/repositories"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/errors"
	"github.com/hashgraph/hedera-sdk-go/v2"
	log "github.com/sirupsen/logrus"
)

func isZeroAccountId(accountId hedera.AccountID) bool {
	return accountId.Shard == 0 && accountId.Realm == 0 && accountId.Account == 0
}

func isZeroTokenId(tokenId hedera.TokenID) bool {
	return tokenId.Shard == 0 && tokenId.Realm == 0 && tokenId.Token == 0
}

func parseOperationMetadata(
	validate *validator.Validate,
	out interface{},
	metadatas ...map[string]interface{},
) *types.Error {
	metadata := make(map[string]interface{})

	for _, m := range metadatas {
		for k, v := range m {
			metadata[k] = v
		}
	}

	if len(metadata) == 0 {
		return errors.ErrInvalidOperationMetadata
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return errors.ErrInvalidOperationMetadata
	}

	if err := json.Unmarshal(data, out); err != nil {
		log.Errorf("Failed to unmarshal operation metadata: %s", err)
		return errors.ErrInvalidOperationMetadata
	}

	if err := validate.Struct(out); err != nil {
		log.Errorf("Failed to validate metadata: %s", err)
		return errors.ErrInvalidOperationMetadata
	}

	return nil
}

func validateOperations(operations []*types.Operation, size int, opType string, allowNilAmount bool) *types.Error {
	if len(operations) == 0 {
		return errors.ErrEmptyOperations
	}

	if size != 0 && len(operations) != size {
		return errors.ErrOperationsInvalid
	}

	for _, operation := range operations {
		if operation.OperationIdentifier == nil {
			return errors.ErrOperationsInvalid
		}

		if operation.Account == nil {
			return errors.ErrOperationsInvalid
		}

		if !allowNilAmount && (operation.Amount == nil || operation.Amount.Currency == nil) {
			return errors.ErrOperationsInvalid
		}

		if operation.Type != opType {
			return errors.ErrInvalidOperationType
		}
	}

	return nil
}

func validateToken(tokenRepo repositories.TokenRepository, currency *types.Currency) (*hedera.TokenID, *types.Error) {
	token, rErr := tokenRepo.Find(currency.Symbol)
	if rErr != nil {
		return nil, rErr
	}

	if token.Decimals != uint32(currency.Decimals) {
		log.Errorf("token decimals mismatch: provided - %d, actual - %d", currency.Decimals, token.Decimals)
		return nil, errors.ErrInvalidToken
	}

	return token.ToHederaTokenId(), nil
}