/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
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
 */

package com.hedera.mirror.importer.reader.block;

import com.hedera.mirror.common.domain.transaction.BlockFile;
import com.hedera.mirror.importer.domain.StreamFileData;
import jakarta.inject.Named;

@Named
public class ProtoBlockFileReader implements BlockFileReader {
    // Generates a BlockFile from a StreamFileData.
    // Converts the protobuf BlockItems into mirror node BlockItems.
    // Protobuf BlockItems that do not represent a transaction will be filtered out here.
    // Note that a protobuf BlockFile with no transactions will still produce a mirror node BlockFile and persist that
    // block to the database.
    public BlockFile read(StreamFileData streamFileData) {
        return new BlockFile();
    }
}
