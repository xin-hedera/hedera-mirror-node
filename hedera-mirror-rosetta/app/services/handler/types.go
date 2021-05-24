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
	"strconv"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/hashgraph/hedera-sdk-go/v2"
)

type ITransaction interface {
	Execute(client *hedera.Client) (hedera.TransactionResponse, error)
	GetNodeAccountIDs() []hedera.AccountID
	GetTransactionHash() ([]byte, error)
	GetTransactionID() hedera.TransactionID
	ToBytes() ([]byte, error)
	String() string
}

type TransactionHandler interface {
	// Construct constructs a transaction from its operations
	Construct(nodeAccountId hedera.AccountID, operations []*types.Operation) (
		ITransaction,
		[]hedera.AccountID,
		*types.Error,
	)

	// ParseConstructedTransaction parses a signed or unsigned transaction to get its operations and required signers
	ParseConstructedTransaction(transaction ITransaction, signed bool) (
		[]*types.Operation,
		[]hedera.AccountID,
		*types.Error,
	)

	// ParseExecutedTransaction parses an executed transaction to get its operations
	ParseExecutedTransaction() ([]*types.Operation, *types.Error)

	// Preprocess preprocesses the operations to get required signers
	Preprocess(operations []*types.Operation) ([]hedera.AccountID, *types.Error)
}

// embed SDK PublicKey and implement the Unmarshaler interface
type publicKey struct {
	hedera.PublicKey
}

func (pk *publicKey) UnmarshalJSON(data []byte) error {
	var err error
	pk.PublicKey, err = hedera.PublicKeyFromString(unquote(data))
	return err
}

// embed SDK TokenID and implement the Unmarshaler interface
type tokenID struct {
	hedera.TokenID
}

func (t *tokenID) UnmarshalJSON(data []byte) (err error) {
	t.TokenID, err = hedera.TokenIDFromString(unquote(data))
	return err
}

func unquote(data []byte) string {
	input := string(data)
	s, err := strconv.Unquote(input)
	if err != nil {
		s = input
	}

	return s
}
