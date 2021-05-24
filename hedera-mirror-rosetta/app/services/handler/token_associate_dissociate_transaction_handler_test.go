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
	"testing"

	rTypes "github.com/coinbase/rosetta-sdk-go/types"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/domain/repositories"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/domain/services/encoding"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/domain/types"
	hErrors "github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/errors"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/config"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/test/mocks/repository"
	"github.com/hashgraph/hedera-sdk-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// constants used in all token handler tests
const (
	decimals = 9
	nameA    = "teebar"
	nameB    = "ueebar"
	symbolA  = "foobar"
	symbolB  = "goobar"
)

// variables used in all token handler tests
var (
	accountId = hedera.AccountID{Account: 197}
	dbTokenA  = &types.Token{
		TokenId:  tokenEntityIdA,
		Decimals: decimals,
		Name:     nameA,
		Symbol:   symbolA,
	}
	dbTokenB = &types.Token{
		TokenId:  tokenEntityIdB,
		Decimals: decimals,
		Name:     nameB,
		Symbol:   symbolB,
	}
	nilDbToken     *types.Token
	nilErr         *rTypes.Error
	nodeAccountId  = hedera.AccountID{Account: 7}
	payerId        = hedera.AccountID{Account: 100}
	tokenEntityIdA = entityid.EntityId{EntityNum: 212, EncodedId: 212}
	tokenEntityIdB = entityid.EntityId{EntityNum: 252, EncodedId: 252}
	tokenIdA       = hedera.TokenID{Token: 212}
	tokenIdB       = hedera.TokenID{Token: 252}
)

type newHandlerFunc func(repositories.TokenRepository) typedTransactionHandler
type updateOperationsFunc func([]*rTypes.Operation) []*rTypes.Operation

func TestTokenAssociateDissociateTransactionHandlerSuite(t *testing.T) {
	suite.Run(t, new(tokenAssociateDissociateTransactionHandlerSuite))
}

type tokenAssociateDissociateTransactionHandlerSuite struct {
	suite.Suite
}

func (suite *tokenAssociateDissociateTransactionHandlerSuite) TestNewTokenAssociateTransactionHandler() {
	h := newTokenAssociateTransactionHandler(&repository.MockTokenRepository{})
	assert.NotNil(suite.T(), h)
}

func (suite *tokenAssociateDissociateTransactionHandlerSuite) TestNewTokenDissociateTransactionHandler() {
	h := newTokenDissociateTransactionHandler(&repository.MockTokenRepository{})
	assert.NotNil(suite.T(), h)
}

func (suite *tokenAssociateDissociateTransactionHandlerSuite) TestGetOperationType() {
	var tests = []struct {
		name       string
		newHandler newHandlerFunc
		expected   string
	}{
		{
			name:       "TokenAssociateTransactionHandler",
			newHandler: newTokenAssociateTransactionHandler,
			expected:   config.OperationTypeTokenAssociate,
		},
		{
			name:       "TokenDissociateTransactionHandler",
			newHandler: newTokenDissociateTransactionHandler,
			expected:   config.OperationTypeTokenDissociate,
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			h := tt.newHandler(&repository.MockTokenRepository{})
			assert.Equal(t, tt.expected, h.GetOperationType())
		})
	}
}

func (suite *tokenAssociateDissociateTransactionHandlerSuite) TestGetSdkTransactionType() {
	var tests = []struct {
		name       string
		newHandler newHandlerFunc
		expected   string
	}{
		{
			name:       "TokenAssociateTransactionHandler",
			newHandler: newTokenAssociateTransactionHandler,
			expected:   "TokenAssociateTransaction",
		},
		{
			name:       "TokenDissociateTransactionHandler",
			newHandler: newTokenDissociateTransactionHandler,
			expected:   "TokenDissociateTransaction",
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			h := tt.newHandler(&repository.MockTokenRepository{})
			assert.Equal(t, tt.expected, h.GetSdkTransactionType())
		})
	}
}

func (suite *tokenAssociateDissociateTransactionHandlerSuite) TestConstruct() {
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
				configMockTokenRepo(mockTokenRepo, tokenIdB, dbTokenB, nil)

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
					assertTokenAssociateDissociateTransaction(t, operations, nodeAccountId, tx)
					mockTokenRepo.AssertExpectations(t)
				}
			})
		}
	}

	suite.T().Run("TokenAssociateTransactionHandler", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenAssociate, newTokenAssociateTransactionHandler)
	})

	suite.T().Run("TokenDissociateTransactionHandler", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenDissociate, newTokenDissociateTransactionHandler)
	})
}

func (suite *tokenAssociateDissociateTransactionHandlerSuite) TestParseConstructedTransaction() {
	defaultGetTransaction := func(operationType string) ITransaction {
		if operationType == config.OperationTypeTokenAssociate {
			return hedera.NewTokenAssociateTransaction().
				SetAccountID(payerId).
				SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
				SetTokenIDs(tokenIdA, tokenIdB).
				SetTransactionID(hedera.TransactionIDGenerate(payerId))
		}

		return hedera.NewTokenDissociateTransaction().
			SetAccountID(payerId).
			SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
			SetTokenIDs(tokenIdA, tokenIdB).
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
				if operationType == config.OperationTypeTokenAssociate {
					return hedera.NewTokenDissociateTransaction()
				}

				return hedera.NewTokenAssociateTransaction()

			},
			expectError: true,
		},
		{
			name: "TransactionAccountNotSet",
			getTransaction: func(operationType string) ITransaction {
				// TODO once SDK PR is merged, remove the SetAccountID call
				if operationType == config.OperationTypeTokenAssociate {
					return hedera.NewTokenAssociateTransaction().
						SetAccountID(hedera.AccountID{}).
						SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
						SetTokenIDs(tokenIdA, tokenIdB).
						SetTransactionID(hedera.TransactionIDGenerate(payerId))
				}

				return hedera.NewTokenDissociateTransaction().
					SetAccountID(hedera.AccountID{}).
					SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
					SetTokenIDs(tokenIdA, tokenIdB).
					SetTransactionID(hedera.TransactionIDGenerate(payerId))
			},
			expectError: true,
		},
		{
			name: "TransactionTokenIDsNotSet",
			getTransaction: func(operationType string) ITransaction {
				if operationType == config.OperationTypeTokenAssociate {
					return hedera.NewTokenAssociateTransaction().
						SetAccountID(payerId).
						SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
						SetTransactionID(hedera.TransactionIDGenerate(payerId))
				}

				return hedera.NewTokenDissociateTransaction().
					SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
					SetAccountID(payerId).
					SetTransactionID(hedera.TransactionIDGenerate(payerId))
			},
			expectError: true,
		},
		{
			name: "TransactionTransactionIDNotSet",
			getTransaction: func(operationType string) ITransaction {
				if operationType == config.OperationTypeTokenAssociate {
					return hedera.NewTokenAssociateTransaction().
						SetAccountID(payerId).
						SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
						SetTokenIDs(tokenIdA, tokenIdB)
				}

				return hedera.NewTokenDissociateTransaction().
					SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
					SetAccountID(payerId).
					SetTokenIDs(tokenIdA, tokenIdB)
			},
			expectError: true,
		},
		{
			name: "TransactionAccountPayerMismatch",
			getTransaction: func(operationType string) ITransaction {
				if operationType == config.OperationTypeTokenAssociate {
					return hedera.NewTokenAssociateTransaction().
						SetAccountID(accountId).
						SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
						SetTokenIDs(tokenIdA, tokenIdB).
						SetTransactionID(hedera.TransactionIDGenerate(payerId))
				}

				return hedera.NewTokenDissociateTransaction().
					SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
					SetAccountID(accountId).
					SetTokenIDs(tokenIdA, tokenIdB).
					SetTransactionID(hedera.TransactionIDGenerate(payerId))
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
				configMockTokenRepo(mockTokenRepo, tokenIdB, dbTokenB, tt.tokenRepoErr)

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

	suite.T().Run("TokenAssociateTransactionHandler", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenAssociate, newTokenAssociateTransactionHandler)
	})

	suite.T().Run("TokenDissociateTransactionHandler", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenDissociate, newTokenDissociateTransactionHandler)
	})
}

func (suite *tokenAssociateDissociateTransactionHandlerSuite) TestParseExecutedTransaction() {
	assert.True(suite.T(), true)
}

func (suite *tokenAssociateDissociateTransactionHandlerSuite) TestPreprocess() {
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
				for _, operation := range operations {
					operation.Account.Address = "x.y.z"
				}
				return operations
			},
			expectError: true,
		},
		{
			name: "DifferentAccountAddress",
			updateOperations: func(operations []*rTypes.Operation) []*rTypes.Operation {
				operations[0].Account.Address = "0.1.7"
				return operations
			},
			expectError: true,
		},
		{
			name: "TokenDecimalsMismatch",
			updateOperations: func(operations []*rTypes.Operation) []*rTypes.Operation {
				for _, operation := range operations {
					operation.Amount.Currency.Decimals = 1990
				}
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
				configMockTokenRepo(mockTokenRepo, tokenIdB, dbTokenB, tt.tokenRepoErr)

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

	suite.T().Run("TokenAssociateTransactionHandler", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenAssociate, newTokenAssociateTransactionHandler)
	})

	suite.T().Run("TokenDissociateTransactionHandler", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenDissociate, newTokenDissociateTransactionHandler)
	})
}

func (suite *tokenAssociateDissociateTransactionHandlerSuite) getOperations(operationType string) []*rTypes.Operation {
	return []*rTypes.Operation{
		{
			OperationIdentifier: &rTypes.OperationIdentifier{Index: 0},
			Type:                operationType,
			Account:             &rTypes.AccountIdentifier{Address: payerId.String()},
			Amount: &rTypes.Amount{
				Value: "0",
				Currency: dbTokenA.ToRosettaCurrency(),
			},
		},
		{
			OperationIdentifier: &rTypes.OperationIdentifier{Index: 1},
			Type:                operationType,
			Account:             &rTypes.AccountIdentifier{Address: payerId.String()},
			Amount: &rTypes.Amount{
				Value: "0",
				Currency: dbTokenB.ToRosettaCurrency(),
			},
		},
	}
}

func assertTokenAssociateDissociateTransaction(
	t *testing.T,
	operations []*rTypes.Operation,
	nodeAccountId hedera.AccountID,
	actual ITransaction,
) {
	if operations[0].Type == config.OperationTypeTokenAssociate {
		assert.IsType(t, &hedera.TokenAssociateTransaction{}, actual)
	} else {
		assert.IsType(t, &hedera.TokenDissociateTransaction{}, actual)
	}

	var expectedTokens []hedera.TokenID
	for _, operation := range operations {
		token, _ := hedera.TokenIDFromString(operation.Amount.Currency.Symbol)
		expectedTokens = append(expectedTokens, token)
	}

	var account string
	var payer string
	var tokens []hedera.TokenID

	switch tx := actual.(type) {
	case *hedera.TokenAssociateTransaction:
		account = tx.GetAccountID().String()
		payer = tx.GetTransactionID().AccountID.String()
		tokens = tx.GetTokenIDs()
	case *hedera.TokenDissociateTransaction:
		account = tx.GetAccountID().String()
		payer = tx.GetTransactionID().AccountID.String()
		tokens = tx.GetTokenIDs()
	}

	assert.Equal(t, operations[0].Account.Address, account)
	assert.Equal(t, operations[0].Account.Address, payer)
	assert.ElementsMatch(t, expectedTokens, tokens)
	assert.ElementsMatch(t, []hedera.AccountID{nodeAccountId}, actual.GetNodeAccountIDs())
}

func configMockTokenRepo(
	mock *repository.MockTokenRepository,
	tokenId hedera.TokenID,
	dbToken *types.Token,
	err *rTypes.Error,
) {
	if err != nil {
		dbToken = nil
	}

	mock.On("Find", tokenId.String()).Return(dbToken, err)
}