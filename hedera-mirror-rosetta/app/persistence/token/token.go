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

package token

import (
	"errors"

	rTypes "github.com/coinbase/rosetta-sdk-go/types"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/domain/repositories"
	entityid "github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/domain/services/encoding"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/domain/types"
	hErrors "github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/errors"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	tableNameToken = "token"
)

type token struct {
	TokenID             int64  `gorm:"type:bigint;primaryKey"`
	CreatedTimestamp    int64  `gorm:"type:bigint"`
	Decimals            int64  `gorm:"type:bigint"`
	FreezeDefault       bool   `gorm:"type:bool"`
	FreezeKey           []byte `gorm:"type:bytea"`
	FreezeKeyEd25519Hex string `gorm:"type:string"`
	InitialSupply       int64  `gorm:"type:bigint"`
	KycKey              []byte `gorm:"type:bytea"`
	KycKeyEd25519Hex    string `gorm:"type:string"`
	ModifiedTimestamp   int64  `gorm:"type:bigint"`
	Name                string `gorm:"type:string"`
	SupplyKey           []byte `gorm:"type:bytea"`
	SupplyKeyEd25519Hex string `gorm:"type:string"`
	Symbol              string `gorm:"type:string"`
	TotalSupply         int64  `gorm:"type:bigint"`
	TreasuryAccountId   int64  `gorm:"type:bigint"`
	WipeKey             []byte `gorm:"type:bytea"`
	WipeKeyEd25519Hex   string `gorm:"type:string"`
}

// TableName - Set table name of the Transactions to be `record_file`
func (token) TableName() string {
	return tableNameToken
}

func (t token) toDomainToken() (*types.Token, *rTypes.Error) {
	tokenId, err := entityid.Decode(t.TokenID)
	if err != nil {
		return nil, hErrors.ErrInvalidToken
	}

	return &types.Token{
		TokenId:  *tokenId,
		Decimals: uint32(t.Decimals),
		Name:     t.Name,
		Symbol:   t.Symbol,
	}, nil
}

// tokenRepository struct that has connection to the Database
type tokenRepository struct {
	dbClient *gorm.DB
}

// NewTokenRepository creates an instance of a TransactionRepository struct
func NewTokenRepository(dbClient *gorm.DB) repositories.TokenRepository {
	return &tokenRepository{dbClient: dbClient}
}

func (tr *tokenRepository) Find(tokenIdStr string) (*types.Token, *rTypes.Error) {
	entityId, err := entityid.FromString(tokenIdStr)
	if err != nil {
		return nil, hErrors.ErrInvalidToken
	}

	token := &token{}
	if err := tr.dbClient.First(token, entityId.EncodedId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, hErrors.ErrTokenNotFound
		}

		log.Errorf("%s: %s", hErrors.ErrDatabaseError.Message, err)
		return nil, hErrors.ErrDatabaseError
	}

	return token.toDomainToken()
}
