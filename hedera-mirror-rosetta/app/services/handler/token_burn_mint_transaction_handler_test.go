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
	"fmt"
	"testing"

	rTypes "github.com/coinbase/rosetta-sdk-go/types"
	hErrors "github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/errors"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/config"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/test/mocks/repository"
	"github.com/hashgraph/hedera-sdk-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const (
	amount = 100
)

func TestTokenBurnMintTransactionHandlerSuite(t *testing.T) {
	suite.Run(t, new(tokenTokenBurnMintTransactionHandlerSuite))
}

type tokenTokenBurnMintTransactionHandlerSuite struct {
	suite.Suite
}

func (suite *tokenTokenBurnMintTransactionHandlerSuite) TestNewTokenBurnTransactionHandler() {
	h := newTokenBurnTransactionHandler(&repository.MockTokenRepository{})
	assert.NotNil(suite.T(), h)
}

func (suite *tokenTokenBurnMintTransactionHandlerSuite) TestNewTokenMintTransactionHandler() {
	h := newTokenMintTransactionHandler(&repository.MockTokenRepository{})
	assert.NotNil(suite.T(), h)
}

func (suite *tokenTokenBurnMintTransactionHandlerSuite) TestGetOperationType() {
	var tests = []struct {
		name       string
		newHandler newHandlerFunc
		expected   string
	}{
		{
			name:       "TokenBurnTransactionHandler",
			newHandler: newTokenBurnTransactionHandler,
			expected:   config.OperationTypeTokenBurn,
		},
		{
			name:       "TokenMintTransactionHandler",
			newHandler: newTokenMintTransactionHandler,
			expected:   config.OperationTypeTokenMint,
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			h := tt.newHandler(&repository.MockTokenRepository{})
			assert.Equal(t, tt.expected, h.GetOperationType())
		})
	}
}

func (suite *tokenTokenBurnMintTransactionHandlerSuite) TestGetSdkTransactionType() {
	var tests = []struct {
		name       string
		newHandler newHandlerFunc
		expected   string
	}{
		{
			name:       "TokenBurnTransactionHandler",
			newHandler: newTokenBurnTransactionHandler,
			expected:   "TokenBurnTransaction",
		},
		{
			name:       "TokenMintTransactionHandler",
			newHandler: newTokenMintTransactionHandler,
			expected:   "TokenMintTransaction",
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			h := tt.newHandler(&repository.MockTokenRepository{})
			assert.Equal(t, tt.expected, h.GetSdkTransactionType())
		})
	}
}

func (suite *tokenTokenBurnMintTransactionHandlerSuite) TestConstruct() {
	var tests = []struct {
		name             string
		updateOperations updateOperationsFunc
		expectError      bool
	}{
		{
			name: "Success",
		},
		{
			name: "EmptyOperations",
			updateOperations: func([]*rTypes.Operation) []*rTypes.Operation {
				return make([]*rTypes.Operation, 0)
			},
			expectError: true,
		},
	}

	runTests := func(t *testing.T, operationType string, newHandler newHandlerFunc) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				operations := suite.getOperations(operationType)
				mockTokenRepo := &repository.MockTokenRepository{}
				h := newHandler(mockTokenRepo)

				configMockTokenRepo(mockTokenRepo, tokenIdA, dbTokenA, nil)

				if tt.updateOperations != nil {
					operations = tt.updateOperations(operations)
				}

				// when
				tx, signers, err := h.Construct(nodeAccountId, operations)

				// then
				if tt.expectError {
					assert.NotNil(t, err)
					assert.Nil(t, signers)
					assert.Nil(t, tx)
				} else {
					assert.Nil(t, err)
					assert.ElementsMatch(t, []hedera.AccountID{payerId}, signers)
					assertTokenBurnMintTransaction(t, operations, nodeAccountId, tx)
					mockTokenRepo.AssertExpectations(t)
				}
			})
		}
	}

	suite.T().Run("TokenBurnTransactionHandler", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenBurn, newTokenBurnTransactionHandler)
	})

	suite.T().Run("TokenDissociateTransactionHandler", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenMint, newTokenMintTransactionHandler)
	})
}

func (suite *tokenTokenBurnMintTransactionHandlerSuite) TestParseConstructedTransaction() {
	defaultGetTransaction := func(operationType string) ITransaction {
		if operationType == config.OperationTypeTokenBurn {
			return hedera.NewTokenBurnTransaction().
				SetAmount(amount).
				SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
				SetTokenID(tokenIdA).
				SetTransactionID(hedera.TransactionIDGenerate(payerId))
		}

		return hedera.NewTokenMintTransaction().
			SetAmount(amount).
			SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
			SetTokenID(tokenIdA).
			SetTransactionID(hedera.TransactionIDGenerate(payerId))
	}

	var tests = []struct {
		name           string
		tokenRepoErr   *rTypes.Error
		getTransaction func(operationType string) ITransaction
		expectError    bool
	}{
		{
			name:           "Success",
			getTransaction: defaultGetTransaction,
		},
		{
			name:           "TokenNotFound",
			tokenRepoErr:   hErrors.ErrTokenNotFound,
			getTransaction: defaultGetTransaction,
			expectError:    true,
		},
		{
			name: "InvalidTransaction",
			getTransaction: func(operationType string) ITransaction {
				return hedera.NewTransferTransaction()
			},
			expectError: true,
		},
		{
			name: "TransactionMismatch",
			getTransaction: func(operationType string) ITransaction {
				if operationType == config.OperationTypeTokenBurn {
					return hedera.NewTokenMintTransaction()
				}

				return hedera.NewTokenBurnTransaction()

			},
			expectError: true,
		},
		{
			name: "TransactionTokenIDNotSet",
			getTransaction: func(operationType string) ITransaction {
				// TODO once SDK PR to fix nil pointer dereference is merged, remove the SetTokenID call
				if operationType == config.OperationTypeTokenBurn {
					return hedera.NewTokenBurnTransaction().
						SetAmount(amount).
						SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
						SetTokenID(hedera.TokenID{}).
						SetTransactionID(hedera.TransactionIDGenerate(payerId))
				}

				return hedera.NewTokenMintTransaction().
					SetAmount(amount).
					SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
					SetTokenID(hedera.TokenID{}).
					SetTransactionID(hedera.TransactionIDGenerate(payerId))
			},
			expectError: true,
		},
		{
			name: "TransactionTransactionIDNotSet",
			getTransaction: func(operationType string) ITransaction {
				if operationType == config.OperationTypeTokenBurn {
					return hedera.NewTokenBurnTransaction().
						SetAmount(amount).
						SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
						SetTokenID(tokenIdA)
				}

				return hedera.NewTokenMintTransaction().
					SetAmount(amount).
					SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
					SetTokenID(tokenIdA)
			},
			expectError: true,
		},
	}

	runTests := func(t *testing.T, operationType string, newHandler newHandlerFunc) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				expectedOperations := suite.getOperations(operationType)

				mockTokenRepo := &repository.MockTokenRepository{}
				h := newHandler(mockTokenRepo)
				tx := tt.getTransaction(operationType)
				configMockTokenRepo(mockTokenRepo, tokenIdA, dbTokenA, tt.tokenRepoErr)

				// when
				operations, signers, err := h.ParseConstructedTransaction(tx, false)

				// then
				if tt.expectError {
					assert.NotNil(t, err)
					assert.Nil(t, operations)
					assert.Nil(t, signers)
				} else {
					assert.Nil(t, err)
					assert.ElementsMatch(t, []hedera.AccountID{payerId}, signers)
					assert.ElementsMatch(t, expectedOperations, operations)
					mockTokenRepo.AssertExpectations(t)
				}
			})
		}
	}

	suite.T().Run("TokenBurnTransactionHandler", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenBurn, newTokenBurnTransactionHandler)
	})

	suite.T().Run("TokenDissociateTransactionHandler", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenMint, newTokenMintTransactionHandler)
	})
}

func (suite *tokenTokenBurnMintTransactionHandlerSuite) TestParseExecutedTransaction() {
	assert.True(suite.T(), true)
}

func (suite *tokenTokenBurnMintTransactionHandlerSuite) TestPreprocess() {
	var tests = []struct {
		name             string
		tokenRepoErr     *rTypes.Error
		updateOperations updateOperationsFunc
		expectError      bool
	}{
		{
			name: "Success",
		},
		{
			name: "InvalidAccountAddress",
			updateOperations: func(operations []*rTypes.Operation) []*rTypes.Operation {
				operations[0].Account.Address = "x.y.z"
				return operations
			},
			expectError: true,
		},
		{
			name: "TokenDecimalsMismatch",
			updateOperations: func(operations []*rTypes.Operation) []*rTypes.Operation {
				operations[0].Amount.Currency.Decimals = 1900
				return operations
			},
			expectError: true,
		},
		{
			name: "ZeroAmountValue",
			updateOperations: func(operations []*rTypes.Operation) []*rTypes.Operation {
				operations[0].Amount.Value = "0"
				return operations
			},
			expectError: true,
		},
		{
			name: "NegativeAmountValue",
			updateOperations: func(operations []*rTypes.Operation) []*rTypes.Operation {
				operations[0].Amount.Value = "-100"
				return operations
			},
			expectError: true,
		},
		{
			name: "MissingAmount",
			updateOperations: func(operations []*rTypes.Operation) []*rTypes.Operation {
				operations[0].Amount = nil
				return operations
			},
			expectError: true,
		},
		{
			name:         "TokenNotFound",
			tokenRepoErr: hErrors.ErrTokenNotFound,
			expectError:  true,
		},
		{
			name: "InvalidOperationType",
			updateOperations: func(operations []*rTypes.Operation) []*rTypes.Operation {
				operations[0].Type = config.OperationTypeCryptoTransfer
				return operations
			},
			expectError: true,
		},
	}

	runTests := func(t *testing.T, operationType string, newHandler newHandlerFunc) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				operations := suite.getOperations(operationType)

				mockTokenRepo := &repository.MockTokenRepository{}
				h := newHandler(mockTokenRepo)
				configMockTokenRepo(mockTokenRepo, tokenIdA, dbTokenA, tt.tokenRepoErr)

				if tt.updateOperations != nil {
					operations = tt.updateOperations(operations)
				}

				// when
				signers, err := h.Preprocess(operations)

				// then
				if tt.expectError {
					assert.NotNil(t, err)
					assert.Nil(t, signers)
				} else {
					assert.Nil(t, err)
					assert.ElementsMatch(t, []hedera.AccountID{payerId}, signers)
					mockTokenRepo.AssertExpectations(t)
				}
			})
		}
	}

	suite.T().Run("TokenBurnTransactionHandler", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenBurn, newTokenBurnTransactionHandler)
	})

	suite.T().Run("TokenDissociateTransactionHandler", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenMint, newTokenMintTransactionHandler)
	})
}

func (suite *tokenTokenBurnMintTransactionHandlerSuite) getOperations(operationType string) []*rTypes.Operation {
	return []*rTypes.Operation{
		{
			OperationIdentifier: &rTypes.OperationIdentifier{Index: 0},
			Type:                operationType,
			Account:             &rTypes.AccountIdentifier{Address: payerId.String()},
			Amount: &rTypes.Amount{
				Value: fmt.Sprintf("%d", amount),
				Currency: dbTokenA.ToRosettaCurrency(),
			},
		},
	}
}

func assertTokenBurnMintTransaction(
	t *testing.T,
	operations []*rTypes.Operation,
	nodeAccountId hedera.AccountID,
	actual ITransaction,
) {
	if operations[0].Type == config.OperationTypeTokenBurn {
		assert.IsType(t, &hedera.TokenBurnTransaction{}, actual)
	} else {
		assert.IsType(t, &hedera.TokenMintTransaction{}, actual)
	}

	var payer string
	var token string

	switch tx := actual.(type) {
	case *hedera.TokenBurnTransaction:
		payer = tx.GetTransactionID().AccountID.String()
		token = tx.GetTokenID().String()
	case *hedera.TokenMintTransaction:
		payer = tx.GetTransactionID().AccountID.String()
		token = tx.GetTokenID().String()
	}

	assert.Equal(t, operations[0].Account.Address, payer)
	assert.Equal(t, operations[0].Amount.Currency.Symbol, token)
	assert.ElementsMatch(t, []hedera.AccountID{nodeAccountId}, actual.GetNodeAccountIDs())
}