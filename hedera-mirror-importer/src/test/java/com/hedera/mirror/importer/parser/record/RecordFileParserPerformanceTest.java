/*
 * Copyright (C) 2021-2024 Hedera Hashgraph, LLC
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

package com.hedera.mirror.importer.parser.record;

import static org.assertj.core.api.Assertions.assertThat;

import com.google.common.base.Function;
import com.google.common.base.Stopwatch;
import com.google.protobuf.ByteString;
import com.hedera.mirror.common.config.CommonTestConfiguration;
import com.hedera.mirror.common.domain.entity.Entity;
import com.hedera.mirror.importer.exception.InvalidDatasetException;
import com.hedera.mirror.importer.parser.domain.RecordFileBuilder;
import com.hedera.mirror.importer.parser.domain.RecordItemBuilder;
import com.hedera.mirror.importer.repository.EntityRepository;
import com.hedera.mirror.importer.repository.RecordFileRepository;
import com.hedera.mirror.importer.test.performance.PerformanceProperties;
import com.hederahashgraph.api.proto.java.AccountAmount;
import com.hederahashgraph.api.proto.java.AccountID;
import com.hederahashgraph.api.proto.java.NftTransfer;
import com.hederahashgraph.api.proto.java.TokenAssociation;
import com.hederahashgraph.api.proto.java.TokenID;
import com.hederahashgraph.api.proto.java.TokenTransferList;
import com.hederahashgraph.api.proto.java.TokenType;
import com.hederahashgraph.api.proto.java.TransferList;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.concurrent.atomic.AtomicLong;
import java.util.stream.IntStream;
import lombok.Builder;
import lombok.CustomLog;
import lombok.Data;
import lombok.RequiredArgsConstructor;
import org.apache.commons.math3.stat.descriptive.SummaryStatistics;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.context.annotation.Import;
import org.springframework.test.context.junit.jupiter.EnabledIf;

@CustomLog
@EnabledIf(expression = "${hedera.mirror.importer.test.performance.parser.enabled}", loadContext = true)
@Import(CommonTestConfiguration.class)
@RequiredArgsConstructor
@SpringBootTest
@Tag("performance")
class RecordFileParserPerformanceTest {

    private final AccountID DEFAULT_PAYER =
            AccountID.newBuilder().setAccountNum(2).build();
    private final int NUM_SERIALS_PER_MINT = 10;

    private final EntityRepository entityRepository;
    private final PerformanceProperties performanceProperties;
    private final RecordFileParser recordFileParser;
    private final RecordFileBuilder recordFileBuilder;
    private final RecordFileRepository recordFileRepository;

    @BeforeEach
    void setup() {
        recordFileParser.clear();
        //        domainBuilder.entity()
        //                .customize(e -> e.num(12345L).id(12345L))
        //                .persist();
        //        domainBuilder.recordFile().persist();
    }

    @Test
    void scenarios() {
        var nextEntityId = new AtomicLong(entityRepository
                .findTopByOrderByIdDesc()
                .map(Entity::getId)
                .map(i -> i + 1)
                .orElseThrow(() -> new InvalidDatasetException("No entities found")));
        var properties = performanceProperties.getParser();
        var previous = recordFileRepository.findLatest().orElse(null);
        var stats = new SummaryStatistics();
        var stopwatch = Stopwatch.createStarted();
        //        var scenarios = performanceProperties.getScenarios().getOrDefault(properties.getScenario(),
        // List.of());

        //        long interval = StreamType.RECORD.getFileCloseInterval().toMillis();
        final long numAccounts = properties.getNumAccounts();

        // Step 1 crypto create account
        Function<RecordItemBuilder, RecordItemBuilder.Builder<?>> cryptoCreateTemplate = recordItemBuilder -> {
            recordItemBuilder.clearState();
            var newAccountId = AccountID.newBuilder()
                    .setAccountNum(nextEntityId.getAndIncrement())
                    .build();
            var transferList = TransferList.newBuilder()
                    .addAccountAmounts(AccountAmount.newBuilder()
                            .setAccountID(newAccountId)
                            .setAmount(10_00_000_000L)
                            .build())
                    .addAccountAmounts(AccountAmount.newBuilder()
                            .setAccountID(DEFAULT_PAYER)
                            .setAmount(-10_00_000_000L)
                            .build())
                    .build();
            return recordItemBuilder
                    .cryptoCreate()
                    .transactionBody(b -> b.setInitialBalance(10_00_000_000L))
                    .payerAccountId(DEFAULT_PAYER)
                    .receipt(r -> r.setAccountID(newAccountId))
                    .record(r -> r.mergeTransferList(transferList));
        };
        long numCreatedAccounts = 0;
        while (numCreatedAccounts < numAccounts) {
            long startNanos = System.nanoTime();
            long count = Math.min(numAccounts - numCreatedAccounts, 30_000L);
            var recordFile = recordFileBuilder
                    .recordFile()
                    .previous(previous)
                    .recordItems(i -> i.count((int) count).template(cryptoCreateTemplate))
                    .build();
            recordFileParser.parse(recordFile);
            stats.addValue(System.nanoTime() - startNanos);

            previous = recordFile;
            numCreatedAccounts += count;
        }

        // Step 2, token create non fungible token classes
        final long numNfts = properties.getNumNfts();
        final long firstTokenId = nextEntityId.get();
        final long maxAccountId = firstTokenId - 1;
        final int numNonFungibleTokens = (int) (numNfts / properties.getNumSerialsPerToken());
        final var tokenInfoMap = new HashMap<Long, NonFungibleTokenMeta>(); // token id to admin account id map
        Function<RecordItemBuilder, RecordItemBuilder.Builder<?>> tokenCreateTemplate = recordItemBuilder -> {
            recordItemBuilder.clearState();
            long tokenId = nextEntityId.getAndIncrement();
            var protoTokenId = TokenID.newBuilder().setTokenNum(tokenId).build();
            var adminAccountId = AccountID.newBuilder()
                    .setAccountNum(maxAccountId - (tokenId - maxAccountId - 1))
                    .build();
            tokenInfoMap.put(
                    tokenId,
                    NonFungibleTokenMeta.builder()
                            .adminId(adminAccountId)
                            .tokenId(protoTokenId)
                            .build());
            var association = TokenAssociation.newBuilder()
                    .setAccountId(adminAccountId)
                    .setTokenId(protoTokenId)
                    .build();
            return recordItemBuilder
                    .tokenCreate()
                    .payerAccountId(adminAccountId)
                    .record(r -> r.clearAutomaticTokenAssociations().addAutomaticTokenAssociations(association))
                    .transactionBody(
                            b -> b.setTokenType(TokenType.NON_FUNGIBLE_UNIQUE).setTreasury(adminAccountId))
                    .receipt(r -> r.setTokenID(protoTokenId));
        };
        var recordFile = recordFileBuilder
                .recordFile()
                .previous(previous)
                .recordItems(i -> i.count(numNonFungibleTokens).template(tokenCreateTemplate))
                .build();
        recordFileParser.parse(recordFile);
        previous = recordFile;

        // Step 3, token mint non fungible tokens
        for (long tokenId = firstTokenId; tokenId < firstTokenId + numNonFungibleTokens; tokenId++) {
            final var serial = new AtomicLong(1);
            final var tokenInfo = tokenInfoMap.get(tokenId);
            Function<RecordItemBuilder, RecordItemBuilder.Builder<?>> tokenMintTemplate = recordItemBuilder -> {
                recordItemBuilder.clearState();
                var allMetadata = new ArrayList<ByteString>();
                var serials = new ArrayList<Long>();
                IntStream.range(0, 10).forEach(x -> {
                    allMetadata.add(recordItemBuilder.bytes(4));
                    serials.add(serial.getAndIncrement());
                });
                var nftTransfers = serials.stream()
                        .map(s -> NftTransfer.newBuilder()
                                .setReceiverAccountID(tokenInfo.getAdminId())
                                .setSerialNumber(s)
                                .build())
                        .toList();
                var nftTransferList = TokenTransferList.newBuilder()
                        .setToken(tokenInfo.getTokenId())
                        .addAllNftTransfers(nftTransfers)
                        .build();
                return recordItemBuilder
                        .tokenMint()
                        .payerAccountId(tokenInfo.getAdminId())
                        .transactionBody(b ->
                                b.clearMetadata().addAllMetadata(allMetadata).setToken(tokenInfo.getTokenId()))
                        .record(r -> r.clearTokenTransferLists().addTokenTransferLists(nftTransferList))
                        .receipt(r -> r.clearSerialNumbers().addAllSerialNumbers(serials));
            };

            long totalMinted = 0;
            while (totalMinted < properties.getNumSerialsPerToken()) {
                long remaining = properties.getNumSerialsPerToken() - totalMinted;
                long count = Math.min(remaining, 3_000L * NUM_SERIALS_PER_MINT);
                int numTxs = (int) (count / NUM_SERIALS_PER_MINT);
                recordFile = recordFileBuilder
                        .recordFile()
                        .previous(previous)
                        .recordItems(i -> i.count(numTxs).template(tokenMintTemplate))
                        .build();
                recordFileParser.parse(recordFile);

                previous = recordFile;
                totalMinted += count;
            }
        }

        long mean = (long) (stats.getMean() / 1_000_000.0);
        log.info("Took {} to process {} files for a mean of {} ms per file", stopwatch, stats.getN(), mean);
        assertThat(Duration.ofMillis(mean))
                .as("CryptoCreate had a latency of {} ms", mean)
                .isLessThanOrEqualTo(Duration.ofSeconds(400));
    }

    @Builder
    @Data
    private static class NonFungibleTokenMeta {
        private AccountID adminId;
        private TokenID tokenId;
    }
}
