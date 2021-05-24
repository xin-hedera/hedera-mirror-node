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

	rTypes "github.com/coinbase/rosetta-sdk-go/types"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/domain/repositories"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/errors"
	"github.com/hashgraph/hedera-sdk-go/v2"
	log "github.com/sirupsen/logrus"
)

type typedTransactionHandler interface {
	TransactionHandler
	GetOperationType() string
	GetSdkTransactionType() string
}

type compositeTransactionHandler struct {
	handlers                       map[string]typedTransactionHandler
	constructedTransactionHandlers map[string]typedTransactionHandler
}

func (c *compositeTransactionHandler) Construct(
	nodeAccountId hedera.AccountID,
	operations []*rTypes.Operation,
) (ITransaction, []hedera.AccountID, *rTypes.Error) {
	h, err := c.validate(operations)
	if err != nil {
		return nil, nil, err
	}

	return h.Construct(nodeAccountId, operations)
}

func (c *compositeTransactionHandler) ParseConstructedTransaction(transaction ITransaction, signed bool) (
	[]*rTypes.Operation,
	[]hedera.AccountID,
	*rTypes.Error,
) {
	name := reflect.TypeOf(transaction).Elem().Name()
	h := c.constructedTransactionHandlers[name]
	if h == nil {
		log.Errorf("No handler to parse constructed transaction %s", name)
		return nil, nil, errors.ErrInternalServerError
	}

	return h.ParseConstructedTransaction(transaction, signed)
}

func (c *compositeTransactionHandler) Preprocess(operations []*rTypes.Operation) ([]hedera.AccountID, *rTypes.Error) {
	h, err := c.validate(operations)
	if err != nil {
		return nil, err
	}

	return h.Preprocess(operations)
}

func (c *compositeTransactionHandler) ParseExecutedTransaction() ([]*rTypes.Operation, *rTypes.Error) {
	return nil, errors.ErrNotImplemented
}

func (c *compositeTransactionHandler) addHandler(handler typedTransactionHandler) {
	c.handlers[handler.GetOperationType()] = handler
	c.constructedTransactionHandlers[handler.GetSdkTransactionType()] = handler
}

func (c *compositeTransactionHandler) validate(operations []*rTypes.Operation) (typedTransactionHandler, *rTypes.Error) {
	if len(operations) == 0 {
		return nil, errors.ErrEmptyOperations
	}

	operationType := operations[0].Type
	for _, operation := range operations[1:] {
		if operation.Type != operationType {
			return nil, errors.ErrMultipleOperationTypesPresent
		}
	}

	h := c.handlers[operationType]
	if h == nil {
		log.Errorf("Operation type %s is not supported", operationType)
		return nil, errors.ErrOperationTypeUnsupported
	}

	return h, nil
}

func NewTransactionHandler(tokenRepo repositories.TokenRepository) TransactionHandler {
	c := &compositeTransactionHandler{
		handlers:                       make(map[string]typedTransactionHandler),
		constructedTransactionHandlers: make(map[string]typedTransactionHandler),
	}

	c.addHandler(newCryptoTransferTransactionHandler())

	if tokenRepo != nil {
		c.addHandler(newTokenAssociateTransactionHandler(tokenRepo))
		c.addHandler(newTokenBurnTransactionHandler(tokenRepo))
		c.addHandler(newTokenCreateTransactionHandler())
		c.addHandler(newTokenDeleteTransactionHandler(tokenRepo))
		c.addHandler(newTokenDissociateTransactionHandler(tokenRepo))
		c.addHandler(newTokenFreezeTransactionHandler(tokenRepo))
		c.addHandler(newTokenGrantKycTransactionHandler(tokenRepo))
		c.addHandler(newTokenRevokeKycTransactionHandler(tokenRepo))
		c.addHandler(newTokenMintTransactionHandler(tokenRepo))
		c.addHandler(newTokenUnfreezeTransactionHandler(tokenRepo))
		c.addHandler(newTokenUpdateTransactionHandler())
		c.addHandler(newTokenWipeTransactionHandler(tokenRepo))
	}

	return c
}
