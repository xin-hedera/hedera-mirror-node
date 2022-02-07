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

package errata

import (
	"bufio"
	_ "embed"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/app/persistence/domain"
)

const (
	resultSuccess      = 22
	second             = int64(1000000000)
	typeCryptoTransfer = 14
)

// consensus timestamps of account balance files in mainnet with known skewed timestamp issue. Such a skewed account
// balance file doesn't include the balance changes of the transaction at the same consensus timestamp.
//go:embed skewed_account_balance_file_timestamps.txt
var content string

var (
	missingTransactions                []TransactionWithCryptoTransfers
	missingTransactionsByHash          map[string]TransactionWithCryptoTransfers
	skewedAccountBalanceFileTimestamps map[int64]bool
)

type TransactionWithCryptoTransfers struct {
	CryptoTransfers []domain.CryptoTransfer
	domain.Transaction
}

func (t TransactionWithCryptoTransfers) IsEmpty() bool {
	return t.Transaction.ConsensusTimestamp == int64(0)
}

func GetMissingTransactionsBetween(start, end int64) []TransactionWithCryptoTransfers {
	if start > end {
		return []TransactionWithCryptoTransfers{}
	}

	result := make([]TransactionWithCryptoTransfers, 0)
	for _, transaction := range missingTransactions {
		timestamp := transaction.ConsensusTimestamp
		if timestamp >= start && timestamp <= end {
			result = append(result, transaction)
		}
	}

	return result
}

func GetMissingTransactionByHash(h string) TransactionWithCryptoTransfers {
	return missingTransactionsByHash[h]
}

func IsAccountBalanceFileSkewed(timestamp int64) bool {
	return skewedAccountBalanceFileTimestamps[timestamp]
}

func mustDecode(h string) []byte {
	b, err := hex.DecodeString(h)
	if err != nil {
		panic(err)
	}
	return b
}

func init() {
	// read file and populate the map
	skewedAccountBalanceFileTimestamps = make(map[int64]bool)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		timestamp, err := strconv.ParseInt(scanner.Text(), 10, 64)
		if err != nil {
			panic(err)
		}
		skewedAccountBalanceFileTimestamps[timestamp] = true
	}

	// add missing transactions with corresponding transfers
	missingTransactionsByHash = make(map[string]TransactionWithCryptoTransfers)
	consensusTimestamp := int64(1568460600232321050)
	transactionHash := "48ba27acbe8e066d1018a6f7b82b5d08fd6d38e75ffe3a0dfa33d0ab9985f3e712066e5862b4a9f4505d39ecb2612a64"
	transaction1 := TransactionWithCryptoTransfers{
		CryptoTransfers: []domain.CryptoTransfer{
			{
				Amount:             int64(-28408867),
				ConsensusTimestamp: consensusTimestamp,
				EntityId:           domain.MustDecodeEntityId(int64(3)),
			},
			{
				Amount:             int64(28480030),
				ConsensusTimestamp: consensusTimestamp,
				EntityId:           domain.MustDecodeEntityId(int64(98)),
			},
			{
				Amount:             int64(-71163),
				ConsensusTimestamp: consensusTimestamp,
				EntityId:           domain.MustDecodeEntityId(int64(16378)),
			},
		},
		Transaction: domain.Transaction{
			ConsensusTimestamp:   consensusTimestamp,
			PayerAccountId:       domain.MustDecodeEntityId(int64(16378)),
			Result:               resultSuccess,
			TransactionHash:      mustDecode(transactionHash),
			Type:                 typeCryptoTransfer,
			ValidDurationSeconds: 120,
			ValidStartNs:         consensusTimestamp - second,
		},
	}
	missingTransactions = append(missingTransactions, transaction1)
	missingTransactionsByHash[transactionHash] = transaction1

	consensusTimestamp = 1568681100418866500
	transactionHash = "907bbe01f5d0e1231d4764a5dd37c1093437ba4cb83d6d24d928ea46bb172d1e6c8c1d4489da9bcf19b1fabb7e76f452"
	transaction2 := TransactionWithCryptoTransfers{
		CryptoTransfers: []domain.CryptoTransfer{
			{
				Amount:             int64(86828),
				ConsensusTimestamp: consensusTimestamp,
				EntityId:           domain.MustDecodeEntityId(int64(3)),
			},
			{
				Amount:             int64(1942696),
				ConsensusTimestamp: consensusTimestamp,
				EntityId:           domain.MustDecodeEntityId(int64(98)),
			},
			{
				Amount:             int64(-2029524),
				ConsensusTimestamp: consensusTimestamp,
				EntityId:           domain.MustDecodeEntityId(int64(909)),
			},
		},
		Transaction: domain.Transaction{
			ConsensusTimestamp:   consensusTimestamp,
			PayerAccountId:       domain.MustDecodeEntityId(int64(909)),
			Result:               resultSuccess,
			TransactionHash:      mustDecode(transactionHash),
			Type:                 typeCryptoTransfer,
			ValidDurationSeconds: 120,
			ValidStartNs:         consensusTimestamp - second,
		},
	}
	missingTransactions = append(missingTransactions, transaction2)
	missingTransactionsByHash[transactionHash] = transaction2

	consensusTimestamp = 1568834100383317500
	transactionHash = "2ee4b8ef123a0e2f248162f2facd7e1d40e7362940e9407978778e1081f1dd7e2f4b1a76fa1b716da0afe56ee85137d3"
	transaction3 := TransactionWithCryptoTransfers{
		CryptoTransfers: []domain.CryptoTransfer{
			{
				Amount:             int64(2145),
				ConsensusTimestamp: consensusTimestamp,
				EntityId:           domain.MustDecodeEntityId(int64(3)),
			},
			{
				Amount:             int64(-85738),
				ConsensusTimestamp: consensusTimestamp,
				EntityId:           domain.MustDecodeEntityId(int64(57)),
			},
			{
				Amount:             int64(83593),
				ConsensusTimestamp: consensusTimestamp,
				EntityId:           domain.MustDecodeEntityId(int64(98)),
			},
		},
		Transaction: domain.Transaction{
			ConsensusTimestamp:   consensusTimestamp,
			PayerAccountId:       domain.MustDecodeEntityId(int64(57)),
			Result:               resultSuccess,
			TransactionHash:      mustDecode(transactionHash),
			Type:                 typeCryptoTransfer,
			ValidDurationSeconds: 120,
			ValidStartNs:         consensusTimestamp - second,
		},
	}
	missingTransactions = append(missingTransactions, transaction3)
	missingTransactionsByHash[transactionHash] = transaction3
}
