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

type tokenCreate struct {
	Name             string            `json:"name" validate:"required"`
	Symbol           string            `validate:"required"`
	Decimals         uint              `json:"decimals"`
	InitialSupply    *uint64           `json:"initial_supply"`
	AdminKey         *publicKey        `json:"admin_key"`
	KycKey           *publicKey        `json:"kyc_key"`
	FreezeKey        *publicKey        `json:"freeze_key"`
	WipeKey          *publicKey        `json:"wipe_key"`
	SupplyKey        *publicKey        `json:"supply_key"`
	FreezeDefault    *bool             `json:"freeze_default"`
	Expiry           *time.Time        `json:"expiry"`
	AutoRenewAccount *hedera.AccountID `json:"auto_renew_account"`
	AutoRenewPeriod  *time.Duration    `json:"auto_renew_period"`
	Memo             *string           `json:"memo"`
}

type tokenCreateTransactionHandler struct {
	transactionType string
	validate        *validator.Validate
}

func (t *tokenCreateTransactionHandler) Construct(nodeAccountId hedera.AccountID, operations []*rTypes.Operation) (
	ITransaction,
	[]hedera.AccountID,
	*rTypes.Error,
) {
	signers, tokenCreate, err := t.preprocess(operations)
	if err != nil {
		return nil, nil, err
	}

	treasury := signers[0]
	tx := hedera.NewTokenCreateTransaction().
		SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
		SetDecimals(tokenCreate.Decimals).
		SetTokenName(tokenCreate.Name).
		SetTokenSymbol(tokenCreate.Symbol).
		SetTransactionID(hedera.TransactionIDGenerate(treasury)).
		SetTreasuryAccountID(treasury)

	if tokenCreate.AdminKey != nil {
		tx.SetAdminKey(tokenCreate.AdminKey.PublicKey)
	}

	if tokenCreate.FreezeDefault != nil {
		tx.SetFreezeDefault(*tokenCreate.FreezeDefault)
	}

	if tokenCreate.FreezeKey != nil {
		tx.SetFreezeKey(tokenCreate.FreezeKey.PublicKey)
	}

	if tokenCreate.InitialSupply != nil {
		tx.SetInitialSupply(*tokenCreate.InitialSupply)
	}

	if tokenCreate.KycKey != nil {
		tx.SetKycKey(tokenCreate.KycKey.PublicKey)
	}

	if tokenCreate.Memo != nil {
		tx.SetTokenMemo(*tokenCreate.Memo)
	}
	if tokenCreate.WipeKey != nil {
		tx.SetWipeKey(tokenCreate.WipeKey.PublicKey)
	}

	if tokenCreate.SupplyKey != nil {
		tx.SetSupplyKey(tokenCreate.SupplyKey.PublicKey)
	}

	if tokenCreate.Expiry != nil {
		tx.SetExpirationTime(*tokenCreate.Expiry)
	}

	if tokenCreate.AutoRenewAccount != nil {
		tx.SetAutoRenewAccount(*tokenCreate.AutoRenewAccount)
	}

	if tokenCreate.AutoRenewPeriod != nil {
		tx.SetAutoRenewPeriod(*tokenCreate.AutoRenewPeriod)
	}

	return tx, signers, nil
}

func (t *tokenCreateTransactionHandler) GetOperationType() string {
	return config.OperationTypeTokenCreate
}

func (t *tokenCreateTransactionHandler) GetSdkTransactionType() string {
	return t.transactionType
}

func (t *tokenCreateTransactionHandler) ParseConstructedTransaction(transaction ITransaction, signed bool) (
	[]*rTypes.Operation,
	[]hedera.AccountID,
	*rTypes.Error,
) {
	tokenCreateTransaction, ok := transaction.(*hedera.TokenCreateTransaction)
	if !ok {
		return nil, nil, hErrors.ErrTransactionInvalidType
	}

	treasury := tokenCreateTransaction.GetTransactionID().AccountID
	operation := &rTypes.Operation{
		OperationIdentifier: &rTypes.OperationIdentifier{
			Index: 0,
		},
		Account: &rTypes.AccountIdentifier{Address: treasury.String()},
		Amount: &rTypes.Amount{
			Value: "0",
			Currency: &rTypes.Currency{
				Symbol:   tokenCreateTransaction.GetTokenSymbol(),
				Decimals: int32(tokenCreateTransaction.GetDecimals()),
			},
		},
		Type: config.OperationTypeTokenCreate,
	}

	metadata := make(map[string]interface{})
	operation.Metadata = metadata
	metadata["expiry"] = tokenCreateTransaction.GetExpirationTime().String()
	metadata["free_default"] = tokenCreateTransaction.GetFreezeDefault()
	metadata["initial_supply"] = tokenCreateTransaction.GetInitialSupply()
	// metadata["memo"] = tokenCreateTransaction.GetTokenMemo() // SDK bug, no memo getter
	metadata["name"] = tokenCreateTransaction.GetTokenName()

	if tokenCreateTransaction.GetAdminKey() != nil {
		metadata["admin_key"] = tokenCreateTransaction.GetAdminKey().String()
	}

	if tokenCreateTransaction.GetFreezeKey() != nil {
		metadata["freeze_key"] = tokenCreateTransaction.GetFreezeKey().String()
	}

	if tokenCreateTransaction.GetKycKey() != nil {
		metadata["kyc_key"] = tokenCreateTransaction.GetFreezeKey().String()
	}

	if tokenCreateTransaction.GetWipeKey() != nil {
		metadata["wipe_key"] = tokenCreateTransaction.GetWipeKey().String()
	}

	if tokenCreateTransaction.GetSupplyKey() != nil {
		metadata["supply_key"] = tokenCreateTransaction.GetSupplyKey().String()
	}

	return []*rTypes.Operation{operation}, []hedera.AccountID{*treasury}, nil
}

func (t *tokenCreateTransactionHandler) ParseExecutedTransaction() ([]*rTypes.Operation, *rTypes.Error) {
	return nil, hErrors.ErrNotImplemented
}

func (t *tokenCreateTransactionHandler) Preprocess(operations []*rTypes.Operation) (
	[]hedera.AccountID,
	*rTypes.Error,
) {
	signers, _, err := t.preprocess(operations)
	if err != nil {
		return nil, err
	}

	return signers, nil
}

func (t *tokenCreateTransactionHandler) preprocess(operations []*rTypes.Operation) (
	[]hedera.AccountID,
	*tokenCreate,
	*rTypes.Error,
) {
	if rErr := validateOperations(operations, 1, t.GetOperationType(), true); rErr != nil {
		return nil, nil, rErr
	}

	operation := operations[0]
	tokenCreate := &tokenCreate{}
	if rErr := parseOperationMetadata(t.validate, tokenCreate, operation.Metadata); rErr != nil {
		return nil, nil, rErr
	}

	var signers []hedera.AccountID

	treasury, err := hedera.AccountIDFromString(operation.Account.Address)
	if err != nil {
		return nil, nil, hErrors.ErrInvalidAccount
	}
	signers = append(signers, treasury)

	if tokenCreate.AutoRenewAccount != nil {
		signers = append(signers, *tokenCreate.AutoRenewAccount)
	}

	return signers, tokenCreate, nil
}

func newTokenCreateTransactionHandler() typedTransactionHandler {
	transactionType := reflect.TypeOf(hedera.TokenCreateTransaction{}).Elem().Name()
	return &tokenCreateTransactionHandler{
		transactionType: transactionType,
		validate:        validator.New(),
	}
}
