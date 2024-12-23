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
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.atomic.AtomicLong;
import java.util.stream.IntStream;
import lombok.CustomLog;
import lombok.RequiredArgsConstructor;
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

    private long firstAccountId;
    private long firstTokenId;
    private long tokenTreasuryAccountId;

    @BeforeEach
    void setup() {
        recordFileParser.clear();
    }

    @Test
    void scenarios() {
        var nextEntityId = new AtomicLong(entityRepository
                .findTopByOrderByIdDesc()
                .map(Entity::getId)
                .map(i -> i + 1)
                .orElseThrow(() -> new InvalidDatasetException("No entities found")));
        var stopwatch = Stopwatch.createStarted();

        createAccounts(nextEntityId);
        createNfts(nextEntityId);
        transferNfts();

        log.info("Took {} to complete", stopwatch);
        assertThat(stopwatch.elapsed().toMillis()).isNotZero();
    }

    private void createAccounts(AtomicLong nextEntityId) {
        firstAccountId = nextEntityId.get();
        var previous = recordFileRepository.findLatest().orElse(null);
        var properties = performanceProperties.getParser();
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
            long count = Math.min(numAccounts - numCreatedAccounts, 30_000L);
            var recordFile = recordFileBuilder
                    .recordFile()
                    .previous(previous)
                    .recordItems(i -> i.count((int) count).template(cryptoCreateTemplate))
                    .build();
            recordFileParser.parse(recordFile);

            previous = recordFile;
            numCreatedAccounts += count;
        }
    }

    private void createNfts(AtomicLong nextEntityId) {
        var previous = recordFileRepository.findLatest().orElse(null);
        var properties = performanceProperties.getParser();
        final long numNfts = properties.getNumNfts();

        tokenTreasuryAccountId = nextEntityId.getAndIncrement();
        final var adminAccountId =
                AccountID.newBuilder().setAccountNum(tokenTreasuryAccountId).build();
        firstTokenId = nextEntityId.get();

        final int numNonFungibleTokens = (int) (numNfts / properties.getNumSerialsPerToken());

        Function<RecordItemBuilder, RecordItemBuilder.Builder<?>> createTreasuryAccountRecordItem =
                recordItemBuilder -> {
                    var transferList = TransferList.newBuilder()
                            .addAccountAmounts(AccountAmount.newBuilder()
                                    .setAccountID(adminAccountId)
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
                            .receipt(r -> r.setAccountID(adminAccountId))
                            .record(r -> r.mergeTransferList(transferList));
                };

        Function<RecordItemBuilder, RecordItemBuilder.Builder<?>> tokenCreateTemplate = recordItemBuilder -> {
            recordItemBuilder.clearState();
            long tokenId = nextEntityId.getAndIncrement();
            var protoTokenId = TokenID.newBuilder().setTokenNum(tokenId).build();
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
                .recordItem(createTreasuryAccountRecordItem)
                .recordItems(i -> i.count(numNonFungibleTokens).template(tokenCreateTemplate))
                .build();
        recordFileParser.parse(recordFile);
        previous = recordFile;

        for (long tokenId = firstTokenId; tokenId < firstTokenId + numNonFungibleTokens; tokenId++) {
            final var serial = new AtomicLong(1);
            final var protoTokenId = TokenID.newBuilder().setTokenNum(tokenId).build();
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
                                .setReceiverAccountID(adminAccountId)
                                .setSerialNumber(s)
                                .build())
                        .toList();
                var nftTransferList = TokenTransferList.newBuilder()
                        .setToken(protoTokenId)
                        .addAllNftTransfers(nftTransfers)
                        .build();
                return recordItemBuilder
                        .tokenMint()
                        .payerAccountId(adminAccountId)
                        .transactionBody(b ->
                                b.clearMetadata().addAllMetadata(allMetadata).setToken(protoTokenId))
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
    }

    private void transferNfts() {
        // first phase, transfer all NFTs from the treasury account to other accounts, assuming there are the same
        // number of accounts as NFTs
        var previous = recordFileRepository.findLatest().orElse(null);
        var properties = performanceProperties.getParser();
        final var nextAccountId = new AtomicLong(firstAccountId);
        final int numNonFungibleTokens = (int) (properties.getNumNfts() / properties.getNumSerialsPerToken());
        long tokenId = firstTokenId;
        final var treasury =
                AccountID.newBuilder().setAccountNum(tokenTreasuryAccountId).build();

        for (int i = 0; i < numNonFungibleTokens; i++, tokenId++) {
            final var protoTokenId = TokenID.newBuilder().setTokenNum(tokenId).build();
            final var serial = new AtomicLong(1);

            Function<RecordItemBuilder, RecordItemBuilder.Builder<?>> cryptoTransferTemplate = recordItemBuilder -> {
                recordItemBuilder.clearState();
                var nftTransfers = TokenTransferList.newBuilder().setToken(protoTokenId);
                List<TokenAssociation> automaticTokenAssociations = new ArrayList<>();
                IntStream.range(0, 10).forEach(x -> {
                    var receiver = AccountID.newBuilder()
                            .setAccountNum(nextAccountId.getAndIncrement())
                            .build();
                    nftTransfers.addNftTransfers(NftTransfer.newBuilder()
                            .setReceiverAccountID(receiver)
                            .setSenderAccountID(treasury)
                            .setSerialNumber(serial.getAndIncrement())
                            .build());
                    automaticTokenAssociations.add(TokenAssociation.newBuilder()
                            .setAccountId(receiver)
                            .setTokenId(protoTokenId)
                            .build());
                });

                return recordItemBuilder
                        .cryptoTransfer()
                        .transactionBody(b -> b.clear().addTokenTransfers(nftTransfers))
                        .payerAccountId(treasury)
                        .record(r -> r.clearTokenTransferLists()
                                .addTokenTransferLists(nftTransfers)
                                .addAllAutomaticTokenAssociations(automaticTokenAssociations));
            };

            long transferred = 0;
            while (transferred < properties.getNumSerialsPerToken()) {
                long remaining = properties.getNumSerialsPerToken() - transferred;
                long count = Math.min(remaining, 3_000L * NUM_SERIALS_PER_MINT);
                int numTxs = (int) (count / NUM_SERIALS_PER_MINT);

                var recordFile = recordFileBuilder
                        .recordFile()
                        .previous(previous)
                        .recordItems(b -> b.count(numTxs).template(cryptoTransferTemplate))
                        .build();
                recordFileParser.parse(recordFile);

                previous = recordFile;
                transferred += count;
            }
        }

        // second phase, transfer all NFTs among the accounts
        tokenId = firstTokenId;
        for (int i = 0; i < numNonFungibleTokens; i++, tokenId++) {
            final var protoTokenId = TokenID.newBuilder().setTokenNum(tokenId).build();
            final var serial = new AtomicLong(1);
            final var sender = new AtomicLong(firstAccountId + i * properties.getNumSerialsPerToken());
            final var receiver = new AtomicLong(tokenTreasuryAccountId - (i + 1) * properties.getNumSerialsPerToken());

            Function<RecordItemBuilder, RecordItemBuilder.Builder<?>> cryptoTransferTemplate = recordItemBuilder -> {
                recordItemBuilder.clearState();
                var nftTransfers = TokenTransferList.newBuilder().setToken(protoTokenId);
                List<TokenAssociation> automaticTokenAssociations = new ArrayList<>();
                IntStream.range(0, 10).forEach(x -> {
                    var receiverAccountId = AccountID.newBuilder()
                            .setAccountNum(receiver.getAndIncrement())
                            .build();
                    var senderAccountId = AccountID.newBuilder()
                            .setAccountNum(sender.getAndIncrement())
                            .build();
                    nftTransfers.addNftTransfers(NftTransfer.newBuilder()
                            .setReceiverAccountID(receiverAccountId)
                            .setSenderAccountID(senderAccountId)
                            .setSerialNumber(serial.getAndIncrement())
                            .build());
                    automaticTokenAssociations.add(TokenAssociation.newBuilder()
                            .setAccountId(receiverAccountId)
                            .setTokenId(protoTokenId)
                            .build());
                });

                return recordItemBuilder
                        .cryptoTransfer()
                        .transactionBody(b -> b.clear().addTokenTransfers(nftTransfers))
                        .payerAccountId(AccountID.newBuilder()
                                .setAccountNum(sender.get())
                                .build())
                        .record(r -> r.clearTokenTransferLists()
                                .addTokenTransferLists(nftTransfers)
                                .addAllAutomaticTokenAssociations(automaticTokenAssociations));
            };

            long transferred = 0;
            while (transferred < properties.getNumSerialsPerToken()) {
                long remaining = properties.getNumSerialsPerToken() - transferred;
                long count = Math.min(remaining, 3_000L * NUM_SERIALS_PER_MINT);
                int numTxs = (int) (count / NUM_SERIALS_PER_MINT);

                var recordFile = recordFileBuilder
                        .recordFile()
                        .previous(previous)
                        .recordItems(b -> b.count(numTxs).template(cryptoTransferTemplate))
                        .build();
                recordFileParser.parse(recordFile);

                previous = recordFile;
                transferred += count;
            }
        }
    }
}
