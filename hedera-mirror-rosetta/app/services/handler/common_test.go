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
	"github.com/go-playground/validator/v10"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/errors"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/config"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/test/mocks/repository"
	"github.com/stretchr/testify/assert"
)

func TestParseOperationMetadata(t *testing.T) {
	type data struct {
		Name  *string `json:"name" validate:"required"`
		Value *int    `json:"value" validate:"required"`
	}

	name := "foobar"
	value := 10

	expected := &data{
		Name:  &name,
		Value: &value,
	}

	var tests = []struct {
		name        string
		metadatas   []map[string]interface{}
		expectError bool
	}{
		{
			name: "Success",
			metadatas: []map[string]interface{}{{
				"name":  name,
				"value": value,
			}},
		},
		{
			name: "SuccessMultiple",
			metadatas: []map[string]interface{}{
				{
					"name": name,
				},
				{
					"value": value,
				},
			},
		},
		{
			name: "SuccessMultipleHonorLast",
			metadatas: []map[string]interface{}{
				{
					"name":  "bad",
					"value": 50,
				},
				{
					"name":  name,
					"value": value,
				},
			},
		},
		{
			name: "MissingField",
			metadatas: []map[string]interface{}{
				{
					"value": value,
				},
			},
			expectError: true,
		},
		{
			name:        "EmptyMetadata",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &data{}
			err := parseOperationMetadata(validator.New(), output, tt.metadatas...)

			if tt.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, expected, output)
			}

		})
	}
}

func TestValidateOperationsWithType(t *testing.T) {
	var tests = []struct {
		name           string
		operations     []*rTypes.Operation
		size           int
		operationType  string
		allowNilAmount bool
		expectError    bool
	}{
		{
			name:          "SuccessSingleOperation",
			operations:    []*rTypes.Operation{getOperation(0, config.OperationTypeCryptoTransfer)},
			size:          1,
			operationType: config.OperationTypeCryptoTransfer,
		},
		{
			name: "SuccessMultipleOperations",
			operations: []*rTypes.Operation{
				getOperation(0, config.OperationTypeCryptoTransfer),
				getOperation(1, config.OperationTypeCryptoTransfer),
			},
			size:          0,
			operationType: config.OperationTypeCryptoTransfer,
		},
		{
			name: "SuccessAllowNilAmount",
			operations: []*rTypes.Operation{
				{
					OperationIdentifier: &rTypes.OperationIdentifier{Index: 0},
					Account:             &rTypes.AccountIdentifier{Address: "0.0.123"},
					Type:                config.OperationTypeCryptoTransfer,
				},
			},
			size:          0,
			operationType: config.OperationTypeCryptoTransfer,
			allowNilAmount: true,
		},
		{
			name:          "EmptyOperations",
			operationType: config.OperationTypeCryptoTransfer,
			expectError:   true,
		},
		{
			name: "OperationsSizeMismatch",
			operations: []*rTypes.Operation{
				getOperation(0, config.OperationTypeCryptoTransfer),
				getOperation(1, config.OperationTypeCryptoTransfer),
			},
			size:          1,
			operationType: config.OperationTypeCryptoTransfer,
			expectError:   true,
		},
		{
			name:          "OperationTypeMismatch",
			operations:    []*rTypes.Operation{getOperation(0, config.OperationTypeCryptoTransfer)},
			size:          1,
			operationType: config.OperationTypeTokenCreate,
			expectError:   true,
		},
		{
			name: "MultipleOperationTypes",
			operations: []*rTypes.Operation{
				getOperation(0, config.OperationTypeCryptoTransfer),
				getOperation(0, config.OperationTypeTokenCreate),
			},
			size:          0,
			operationType: config.OperationTypeCryptoTransfer,
			expectError:   true,
		},
		{
			name: "OperationMissingOperationIdentifier",
			operations: []*rTypes.Operation{
				{
					Account: &rTypes.AccountIdentifier{Address: "0.0.123"},
					Amount: &rTypes.Amount{
						Value: "0",
						Currency: &rTypes.Currency{
							Symbol:   "foobar",
							Decimals: 10,
						},
					},
					Type: config.OperationTypeCryptoTransfer,
				},
			},
			size:          1,
			operationType: config.OperationTypeCryptoTransfer,
			expectError:   true,
		},
		{
			name: "OperationMissingAccount",
			operations: []*rTypes.Operation{
				{
					OperationIdentifier: &rTypes.OperationIdentifier{Index: 0},
					Amount: &rTypes.Amount{
						Value: "0",
						Currency: &rTypes.Currency{
							Symbol:   "foobar",
							Decimals: 10,
						},
					},
					Type: config.OperationTypeCryptoTransfer,
				},
			},
			size:          1,
			operationType: config.OperationTypeCryptoTransfer,
			expectError:   true,
		},
		{
			name: "OperationMissingAmount",
			operations: []*rTypes.Operation{
				{
					OperationIdentifier: &rTypes.OperationIdentifier{Index: 0},
					Account:             &rTypes.AccountIdentifier{Address: "0.0.123"},
					Type:                config.OperationTypeCryptoTransfer,
				},
			},
			size:          1,
			operationType: config.OperationTypeCryptoTransfer,
			expectError:   true,
		},
		{
			name: "OperationMissingAmountCurrency",
			operations: []*rTypes.Operation{
				{
					OperationIdentifier: &rTypes.OperationIdentifier{Index: 0},
					Account:             &rTypes.AccountIdentifier{Address: "0.0.123"},
					Amount:              &rTypes.Amount{Value: "0"},
					Type:                config.OperationTypeCryptoTransfer,
				},
			},
			size:          1,
			operationType: config.OperationTypeCryptoTransfer,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOperations(tt.operations, tt.size, tt.operationType, tt.allowNilAmount)

			if tt.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestValidateToken(t *testing.T) {
	var tests = []struct {
		name         string
		currency     *rTypes.Currency
		tokenRepoErr *rTypes.Error
		expectError  bool
	}{
		{
			name:     "Success",
			currency: dbTokenA.ToRosettaCurrency(),
		},
		{
			name:         "TokenNotFound",
			currency:     dbTokenA.ToRosettaCurrency(),
			tokenRepoErr: errors.ErrTokenNotFound,
			expectError:  true,
		},
		{
			name:         "DatabaseError",
			currency:     dbTokenA.ToRosettaCurrency(),
			tokenRepoErr: errors.ErrDatabaseError,
			expectError:  true,
		},
		{
			name: "DecimalsMismatch",
			currency: &rTypes.Currency{
				Symbol:   "0.0.212",
				Decimals: 19867,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTokenRepo := &repository.MockTokenRepository{}
			configMockTokenRepo(mockTokenRepo, tokenIdA, dbTokenA, tt.tokenRepoErr)

			token, err := validateToken(mockTokenRepo, tt.currency)

			if tt.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, dbTokenA.ToHederaTokenId(), token)
			}
		})
	}
}

func getOperation(index int64, operationType string) *rTypes.Operation {
	return &rTypes.Operation{
		OperationIdentifier: &rTypes.OperationIdentifier{Index: index},
		Account:             &rTypes.AccountIdentifier{Address: "0.0.100"},
		Amount: &rTypes.Amount{
			Value: "20",
			Currency: &rTypes.Currency{
				Symbol:   "foobar",
				Decimals: 9,
			},
		},
		Type: operationType,
	}
}
