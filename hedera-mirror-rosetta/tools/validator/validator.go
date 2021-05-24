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

package validator

import (
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/errors"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/tools/parse"
	"github.com/hashgraph/hedera-sdk-go/v2"
	log "github.com/sirupsen/logrus"
)

func ValidateTransferOperations(operations []*types.Operation) (
	map[hedera.AccountID]int64,
	*types.Error,
) {
	if len(operations) == 0 {
		return nil, errors.ErrEmptyOperations
	}

	accountAmounts := make(map[hedera.AccountID]int64)
	var sum int64 = 0

	for _, operation := range operations {
		account, err := hedera.AccountIDFromString(operation.Account.Address)
		if err != nil {
			return nil, errors.ErrInvalidAccount
		}

		amount, err := parse.ToInt64(operation.Amount.Value)
		if err != nil || amount == 0 {
			return nil, errors.ErrInvalidAmount
		}

		accountAmounts[account] += amount
		sum += amount
	}

	for account, amount := range accountAmounts {
		if amount == 0 {
			log.Error("Aggregated amount for account %s is 0", account)
			return nil, errors.ErrInvalidAmount
		}
	}

	if sum != 0 {
		return nil, errors.ErrInvalidOperationsTotalAmount
	}

	return accountAmounts, nil
}
