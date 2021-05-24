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
	"time"

	rTypes "github.com/coinbase/rosetta-sdk-go/types"
	"github.com/go-playground/validator/v10"
	hErrors "github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/errors"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/config"
	"github.com/hashgraph/hedera-sdk-go/v2"
)

type tokenUpdate struct {
	Name             *string
	Symbol           *string
	TokenID          *hedera.TokenID
	Treasury         *hedera.AccountID
	AdminKey         *publicKey
	KycKey           *publicKey
	FreezeKey        *publicKey
	WipeKey          *publicKey
	SupplyKey        *publicKey
	Expiry           *time.Time
	AutoRenewAccount *hedera.AccountID
	AutoRenewPeriod  *time.Duration
	Memo             *string
}

type tokenUpdateTransactionHandler struct {
	transactionType string
	validate        *validator.Validate
}

func (t *tokenUpdateTransactionHandler) Construct(nodeAccountId hedera.AccountID, operations []*rTypes.Operation) (
	ITransaction,
	[]hedera.AccountID,
	*rTypes.Error,
) {
	payer, tokenUpdate, err := t.preprocess(operations)
	if err != nil {
		return nil, nil, err
	}

	tx := hedera.NewTokenUpdateTransaction().
		SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
		SetTokenID(*tokenUpdate.TokenID).
		SetTransactionID(hedera.TransactionIDGenerate(*payer))

	if tokenUpdate.AdminKey != nil {
		tx.SetAdminKey(tokenUpdate.AdminKey.PublicKey)
	}

	if tokenUpdate.AutoRenewAccount != nil {
		tx.SetAutoRenewAccount(*tokenUpdate.AutoRenewAccount)
	}

	if tokenUpdate.AutoRenewPeriod != nil {
		tx.SetAutoRenewPeriod(*tokenUpdate.AutoRenewPeriod)
	}

	if tokenUpdate.Expiry != nil {
		tx.SetExpirationTime(*tokenUpdate.Expiry)
	}

	if tokenUpdate.FreezeKey != nil {
		tx.SetFreezeKey(tokenUpdate.FreezeKey.PublicKey)
	}

	if tokenUpdate.KycKey != nil {
		tx.SetKycKey(tokenUpdate.KycKey.PublicKey)
	}

	if tokenUpdate.Memo != nil {
		tx.SetTokenMemo(*tokenUpdate.Memo)
	}

	if tokenUpdate.SupplyKey != nil {
		tx.SetSupplyKey(tokenUpdate.SupplyKey.PublicKey)
	}

	if tokenUpdate.Symbol != nil {
		tx.SetTokenSymbol(*tokenUpdate.Symbol)
	}

	if tokenUpdate.Treasury != nil {
		tx.SetTreasuryAccountID(*tokenUpdate.Treasury)
	}

	if tokenUpdate.WipeKey != nil {
		tx.SetWipeKey(tokenUpdate.WipeKey.PublicKey)
	}

	return tx, []hedera.AccountID{*payer}, nil
}

func (t *tokenUpdateTransactionHandler) GetOperationType() string {
	return config.OperationTypeTokenUpdate
}

func (t *tokenUpdateTransactionHandler) GetSdkTransactionType() string {
	return t.transactionType
}

func (t *tokenUpdateTransactionHandler) ParseConstructedTransaction(transaction ITransaction, signed bool) (
	[]*rTypes.Operation,
	[]hedera.AccountID,
	*rTypes.Error,
) {
	tokenUpdateTransaction, ok := transaction.(*hedera.TokenUpdateTransaction)
	if !ok {
		return nil, nil, hErrors.ErrTransactionInvalidType
	}

	payer := tokenUpdateTransaction.GetTransactionID().AccountID
	operation := &rTypes.Operation{
		OperationIdentifier: &rTypes.OperationIdentifier{
			Index: 0,
		},
		Account: &rTypes.AccountIdentifier{Address: payer.String()},
		// TODO get symbol and decimals from db
		Amount: &rTypes.Amount{
			Value: "0",
			Currency: &rTypes.Currency{
				Symbol:   "",
				Decimals: 0,
			},
		},
		Type: config.OperationTypeTokenUpdate,
	}

	metadata := make(map[string]interface{})
	operation.Metadata = metadata
	// TODO nil deference in SDK when expiration is not set
	// metadata["expiry"] = tokenUpdateTransaction.GetExpirationTime().String()
	// TODO SDK bug, no memo getter
	// metadata["memo"] = tokenUpdateTransaction.GetTokenMemo()
	metadata["name"] = tokenUpdateTransaction.GetTokenName()
	metadata["symbol"] = tokenUpdateTransaction.GetTokenSymbol()

	if tokenUpdateTransaction.GetAdminKey() != nil {
		metadata["admin_key"] = tokenUpdateTransaction.GetAdminKey().String()
	}

	// TODO nil dereference in SDK
	// if tokenUpdateTransaction.GetAutoRenewAccount() != nil {
	// }

	// TODO nil dereference in SDK
	// if tokenUpdateTransaction.GetAutoRenewPeriod() != nil {
	// }

	if tokenUpdateTransaction.GetFreezeKey() != nil {
		metadata["freeze_key"] = tokenUpdateTransaction.GetFreezeKey().String()
	}

	if tokenUpdateTransaction.GetKycKey() != nil {
		metadata["kyc_key"] = tokenUpdateTransaction.GetFreezeKey().String()
	}

	if tokenUpdateTransaction.GetSupplyKey() != nil {
		metadata["supply_key"] = tokenUpdateTransaction.GetSupplyKey().String()
	}

	// TODO SDK bug: nil deference when treasury is not set
	// if tokenUpdateTransaction.GetTreasuryAccountID() != nil {
	// }

	if tokenUpdateTransaction.GetWipeKey() != nil {
		metadata["wipe_key"] = tokenUpdateTransaction.GetWipeKey().String()
	}

	return []*rTypes.Operation{operation}, []hedera.AccountID{*payer}, nil
}

func (t *tokenUpdateTransactionHandler) ParseExecutedTransaction() ([]*rTypes.Operation, *rTypes.Error) {
	return nil, hErrors.ErrNotImplemented
}

func (t *tokenUpdateTransactionHandler) Preprocess(operations []*rTypes.Operation) (
	[]hedera.AccountID,
	*rTypes.Error,
) {
	payer, _, err := t.preprocess(operations)
	if err != nil {
		return nil, err
	}

	return []hedera.AccountID{*payer}, nil
}

func (t *tokenUpdateTransactionHandler) preprocess(operations []*rTypes.Operation) (
	*hedera.AccountID,
	*tokenUpdate,
	*rTypes.Error,
) {
	if rErr := validateOperations(operations, 1, t.GetOperationType(), false); rErr != nil {
		return nil, nil, rErr
	}

	operation := operations[0]
	currencyMetadata := operation.Amount.Currency.Metadata
	if len(currencyMetadata) == 0 {
		return nil, nil, hErrors.ErrInvalidOperationMetadata
	}

	tokenIDStr, ok := currencyMetadata["token"].(string)
	if !ok {
		return nil, nil, hErrors.ErrInvalidOperationMetadata
	}

	tokenID, err := hedera.TokenIDFromString(tokenIDStr)
	if err != nil {
		return nil, nil, hErrors.ErrInvalidOperationMetadata
	}

	tokenUpdate := &tokenUpdate{TokenID: &tokenID}
	if rErr := parseOperationMetadata(t.validate, tokenUpdate, operation.Metadata); rErr != nil {
		return nil, nil, rErr
	}

	payer, err := hedera.AccountIDFromString(operation.Account.Address)
	if err != nil {
		return nil, nil, hErrors.ErrInvalidAccount
	}

	return &payer, tokenUpdate, nil
}
func newTokenUpdateTransactionHandler() typedTransactionHandler {
	transactionType := reflect.TypeOf(hedera.TokenUpdateTransaction{}).Elem().Name()
	return &tokenUpdateTransactionHandler{
		transactionType: transactionType,
		validate:        validator.New(),
	}
}
