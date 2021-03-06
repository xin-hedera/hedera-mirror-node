/*-
 * ‌
 * Hedera Mirror Node
 * ​
 * Copyright (C) 2019 - 2020 Hedera Hashgraph, LLC
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
'use strict';

const {RecordFile} = require('../recordFile');
const {base64StringToBuffer, readJSONFile} = require('../utils');

const stateProofJson = readJSONFile('stateProofSample.json');

test('recordFile parsetest', () => {
  const recordFilesString = base64StringToBuffer(stateProofJson['record_file']);
  const recordFileDomain = new RecordFile(recordFilesString, '0.0.3');
  expect(recordFileDomain.hash).toBeDefined();
  expect(Object.keys(recordFileDomain.transactionIdMap).length).toBeGreaterThan(0);
});
