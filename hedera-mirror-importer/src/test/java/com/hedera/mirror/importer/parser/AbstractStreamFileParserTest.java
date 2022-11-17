package com.hedera.mirror.importer.parser;

/*-
 * ‌
 * Hedera Mirror Node
 * ​
 * Copyright (C) 2019 - 2022 Hedera Hashgraph, LLC
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

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.when;

import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.junit.jupiter.MockitoExtension;

import com.hedera.mirror.common.domain.StreamFile;
import com.hedera.mirror.importer.exception.ParserException;
import com.hedera.mirror.importer.repository.StreamFileRepository;

@ExtendWith(MockitoExtension.class)
public abstract class AbstractStreamFileParserTest<T extends StreamFileParser> {

    protected T parser;

    protected ParserProperties parserProperties;

    protected abstract T getParser();

    protected abstract StreamFile getStreamFile();

    protected abstract StreamFileRepository getStreamFileRepository();

    protected abstract void mockDbFailure();

    @BeforeEach()
    public void before() {
        parser = getParser();
        parserProperties = parser.getProperties();
        parserProperties.setEnabled(true);
    }

    @Test
    void parse() {
        // given
        StreamFile streamFile = getStreamFile();

        // when
        parser.parse(streamFile);

        // then
        assertParsed(streamFile, true, false);
    }

    @Test
    void disabled() {
        // given
        parserProperties.setEnabled(false);
        StreamFile streamFile = getStreamFile();

        // when
        parser.parse(streamFile);

        // then
        assertParsed(streamFile, false, false);
    }

    @Test
    void alreadyExists() {
        // given
        StreamFile streamFile = getStreamFile();
        when(getStreamFileRepository().existsById(streamFile.getConsensusEnd())).thenReturn(true);

        // when
        parser.parse(streamFile);

        // then
        assertParsed(streamFile, false, false);
    }

    @Test
    void failureShouldRollback() {
        // given
        StreamFile streamFile = getStreamFile();
        mockDbFailure();

        // when
        Assertions.assertThrows(ParserException.class, () -> {
            parser.parse(streamFile);
        });

        // then
        assertParsed(streamFile, false, true);
    }

    protected void assertParsed(StreamFile streamFile, boolean parsed, boolean dbError) {
        assertThat(streamFile.getBytes()).isNotNull();
        assertThat(streamFile.getItems()).isNotNull();
    }
}