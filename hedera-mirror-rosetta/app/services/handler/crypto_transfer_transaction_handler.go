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
	"reflect"
	"sort"
	"strconv"

	rTypes "github.com/coinbase/rosetta-sdk-go/types"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/errors"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/config"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/tools/parse"
	"github.com/hashgraph/hedera-sdk-go/v2"
	log "github.com/sirupsen/logrus"
)

type cryptoTransferTransactionHandler struct {
	transactionType string
}

func (c *cryptoTransferTransactionHandler) Construct(nodeAccountId hedera.AccountID, operations []*rTypes.Operation) (
	ITransaction,
	[]hedera.AccountID,
	*rTypes.Error,
) {
	accountAmounts, aggregated, rErr := parseTransferOperations(operations)
	if rErr != nil {
		return nil, nil, rErr
	}

	transaction := hedera.NewTransferTransaction()

	for _, accountAmount := range accountAmounts {
		transaction.AddHbarTransfer(accountAmount.account, hedera.HbarFromTinybar(accountAmount.amount))
	}

	var senders []hedera.AccountID
	for account, amount := range aggregated {
		if amount < 0 {
			senders = append(senders, account)
		}
	}

	sort.Slice(senders, func(i, j int) bool {
		return senders[i].String() < senders[j].String()
	})

	// set to a single node account ID, so later can add signature
	_, err := transaction.
		SetTransactionID(hedera.TransactionIDGenerate(senders[0])).
		SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
		Freeze()
	if err != nil {
		return nil, nil, errors.ErrTransactionFreezeFailed
	}

	return transaction, senders, nil
}

func (c *cryptoTransferTransactionHandler) GetOperationType() string {
	return config.OperationTypeCryptoTransfer
}

func (c *cryptoTransferTransactionHandler) GetSdkTransactionType() string {
	return c.transactionType
}

func (c *cryptoTransferTransactionHandler) ParseConstructedTransaction(transaction ITransaction, signed bool) (
	[]*rTypes.Operation,
	[]hedera.AccountID,
	*rTypes.Error,
) {
	transferTransaction, ok := transaction.(*hedera.TransferTransaction)
	if !ok {
		return nil, nil, errors.ErrTransactionInvalidType
	}

	transfers := transferTransaction.GetHbarTransfers()
	var operations []*rTypes.Operation

	accountIds := make([]hedera.AccountID, 0, len(transfers))
	for accountId := range transfers {
		accountIds = append(accountIds, accountId)
	}

	// sort it so the order is stable
	sort.Slice(accountIds, func(i, j int) bool {
		return accountIds[i].String() < accountIds[j].String()
	})

	aggregated := make(map[hedera.AccountID]int64)
	for i, accountId := range accountIds {
		amount := transfers[accountId].AsTinybar()
		operation := &rTypes.Operation{
			OperationIdentifier: &rTypes.OperationIdentifier{
				Index: int64(i),
			},
			Type: c.GetOperationType(),
			Account: &rTypes.AccountIdentifier{
				Address: accountId.String(),
			},
			Amount: &rTypes.Amount{
				Value:    strconv.FormatInt(amount, 10),
				Currency: config.CurrencyHbar,
			},
		}

		aggregated[accountId] += amount
		operations = append(operations, operation)
	}

	var signers []hedera.AccountID
	for account, amount := range aggregated {
		if amount < 0 {
			signers = append(signers, account)
		}
	}

	return operations, signers, nil
}

func (c *cryptoTransferTransactionHandler) ParseExecutedTransaction() ([]*rTypes.Operation, *rTypes.Error) {
	return nil, nil
}

func (c *cryptoTransferTransactionHandler) Preprocess(operations []*rTypes.Operation) (
	[]hedera.AccountID,
	*rTypes.Error,
) {
	_, aggregated, err := parseTransferOperations(operations)
	if err != nil {
		return nil, err
	}

	var senders []hedera.AccountID
	for account, amount := range aggregated {
		if amount < 0 {
			senders = append(senders, account)
		}
	}

	return senders, nil
}

func newCryptoTransferTransactionHandler() typedTransactionHandler {
	transactionType := reflect.TypeOf(hedera.NewTransferTransaction()).Elem().Name()
	return &cryptoTransferTransactionHandler{transactionType: transactionType}
}

type accountAmount struct {
	account hedera.AccountID
	amount  int64
}

func parseTransferOperations(operations []*rTypes.Operation) (
	[]accountAmount,
	map[hedera.AccountID]int64,
	*rTypes.Error,
) {
	if len(operations) == 0 {
		return nil, nil, errors.ErrEmptyOperations
	}

	accountAmounts := make([]accountAmount, 0, len(operations))
	aggregated := make(map[hedera.AccountID]int64)
	var sum int64 = 0

	for _, operation := range operations {
		if operation.Type != config.OperationTypeCryptoTransfer {
			return nil, nil, errors.ErrInvalidOperationType
		}

		account, err := hedera.AccountIDFromString(operation.Account.Address)
		if err != nil {
			return nil, nil, errors.ErrInvalidAccount
		}

		amount, err := parse.ToInt64(operation.Amount.Value)
		if err != nil || amount == 0 {
			return nil, nil, errors.ErrInvalidAmount
		}

		accountAmounts = append(accountAmounts, accountAmount{
			account: account,
			amount:  amount,
		})
		aggregated[account] += amount
		sum += amount
	}

	for account, amount := range aggregated {
		if amount == 0 {
			log.Errorf("Aggregated amount for account %s is 0", account)
			return nil, nil, errors.ErrInvalidAmount
		}
	}

	if sum != 0 {
		return nil, nil, errors.ErrInvalidOperationsTotalAmount
	}

	return accountAmounts, aggregated, nil
}
