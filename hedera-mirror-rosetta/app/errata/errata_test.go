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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAccountBalanceFileSkewed(t *testing.T) {
	assert.False(t, IsAccountBalanceFileSkewed(int64(0)))
	assert.True(t, IsAccountBalanceFileSkewed(int64(1568415600193620000)))
	assert.True(t, IsAccountBalanceFileSkewed(int64(1599580800153882000)))
}

func TestSkewedAccountBalanceFileTimestamps(t *testing.T) {
	assert.Equal(t, 6946, len(skewedAccountBalanceFileTimestamps))

	minTimestamp := int64(1<<63 - 1)
	maxTimestamp := int64(0)

	for timestamp, exist := range skewedAccountBalanceFileTimestamps {
		assert.True(t, exist)
		if timestamp < minTimestamp {
			minTimestamp = timestamp
		}
		if timestamp > maxTimestamp {
			maxTimestamp = timestamp
		}
	}

	assert.Equal(t, int64(1568415600193620000), minTimestamp)
	assert.Equal(t, int64(1599580800153882000), maxTimestamp)
}
