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
	"strconv"
	"strings"
)

// consensus timestamps of account balance files in mainnet with known skewed timestamp issue. Such a skewed account
// balance file doesn't include the balance changes of the transaction at the same consensus timestamp.
//go:embed skewed_account_balance_file_timestamps.txt
var content string

var skewedAccountBalanceFileTimestamps map[int64]bool

func IsAccountBalanceFileSkewed(timestamp int64) bool {
	return skewedAccountBalanceFileTimestamps[timestamp]
}

func init() {
	skewedAccountBalanceFileTimestamps = make(map[int64]bool)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		timestamp, err := strconv.ParseInt(scanner.Text(), 10, 64)
		if err != nil {
			panic(err)
		}
		skewedAccountBalanceFileTimestamps[timestamp] = true
	}
}
