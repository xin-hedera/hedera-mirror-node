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

package com.hedera.mirror.importer.downloader.block;

import static com.hedera.mirror.common.domain.DigestAlgorithm.SHA_384;

import com.google.common.base.Strings;
import com.hedera.mirror.common.domain.transaction.BlockFile;
import com.hedera.mirror.common.domain.transaction.RecordFile;
import com.hedera.mirror.importer.ImporterProperties;
import com.hedera.mirror.importer.addressbook.ConsensusNode;
import com.hedera.mirror.importer.addressbook.ConsensusNodeService;
import com.hedera.mirror.importer.domain.StreamFilename;
import com.hedera.mirror.importer.downloader.StreamFileNotifier;
import com.hedera.mirror.importer.downloader.StreamPoller;
import com.hedera.mirror.importer.downloader.provider.StreamFileProvider;
import com.hedera.mirror.importer.exception.HashMismatchException;
import com.hedera.mirror.importer.exception.InvalidStreamFileException;
import com.hedera.mirror.importer.leader.Leader;
import com.hedera.mirror.importer.reader.block.BlockFileReader;
import com.hedera.mirror.importer.repository.RecordFileRepository;
import jakarta.inject.Named;
import java.util.ArrayList;
import java.util.Collection;
import java.util.Collections;
import java.util.Objects;
import java.util.Optional;
import java.util.concurrent.atomic.AtomicReference;
import lombok.CustomLog;
import lombok.RequiredArgsConstructor;
import org.apache.commons.lang3.StringUtils;
import org.springframework.scheduling.annotation.Scheduled;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

@CustomLog
@Named
@RequiredArgsConstructor
public class BlockStreamPoller implements StreamPoller {

    private static final int BASENAME_LENGTH = 36;
    private static final char BASENAME_PADDING = '0';
    private static final String FILE_SUFFIX = ".blk.gz";

    private final BlockDownloaderProperties properties;
    private final BlockFileReader blockFileReader;
    private final ConsensusNodeService consensusNodeService;
    private final ImporterProperties importerProperties;
    private final AtomicReference<Optional<RecordFile>> lastRecordFile = new AtomicReference<>(Optional.empty());
    private final RecordFileRepository recordFileRepository;
    protected final StreamFileNotifier streamFileNotifier;
    private final StreamFileProvider streamFileProvider;

    @Override
    @Leader
    @Scheduled(fixedDelayString = "#{@blockDownloaderProperties.getFrequency().toMillis()}")
    public void poll() {
        if (!properties.isEnabled()) {
            return;
        }

        long blockNumber = getNextBlockNumber();
        var filename = getBlockFilename(blockNumber);

        var recordFile = Objects.requireNonNull(Flux.fromIterable(getRandomizedNodes()))
                .flatMap(
                        node -> {
                            long nodeId = node.getNodeId();
                            var pathPrefix = getStreamFilePathPrefix(node);
                            var streamFilename =
                                    StreamFilename.from(pathPrefix, filename, StreamFileProvider.SEPARATOR);
                            return streamFileProvider
                                    .get(streamFilename)
                                    .doOnError(e -> log.warn(
                                            "Error downloading block file {} from node {}", filename, nodeId, e))
                                    .doOnNext(s -> log.info("Downloaded block file {} from node {}", filename, nodeId))
                                    .map(blockFileReader::read)
                                    .doOnError(e ->
                                            log.warn("Error reading block file {} from node {}", filename, nodeId, e))
                                    .map(this::transform)
                                    .doOnError(e -> log.warn(
                                            "Error transforming block file {} from node {}", filename, nodeId, e))
                                    .doOnNext(r -> verify(blockNumber, r))
                                    .doOnError(e -> log.warn(
                                            "Failed to verify downloaded block file {} from node {}",
                                            filename,
                                            nodeId,
                                            e))
                                    .onErrorResume(e -> Mono.empty());
                        },
                        1)
                .timeout(properties.getCommon().getTimeout())
                .blockFirst();
        if (recordFile == null) {
            log.warn("Failed to download block file {}", filename);
            return;
        }

        streamFileNotifier.verified(recordFile);
        var copy = recordFile.copy().clear();
        lastRecordFile.set(Optional.of((RecordFile) copy));
    }

    private static String getBlockFilename(long blockNumber) {
        return Strings.padStart(String.valueOf(blockNumber), BASENAME_LENGTH, BASENAME_PADDING) + FILE_SUFFIX;
    }

    private long getNextBlockNumber() {
        return lastRecordFile
                .get()
                .or(() -> {
                    var recordFile = recordFileRepository.findLatest();
                    lastRecordFile.compareAndSet(Optional.empty(), recordFile);
                    return recordFile;
                })
                .map(RecordFile::getIndex)
                .map(v -> v + 1)
                .or(() -> Optional.ofNullable(importerProperties.getStartBlockNumber()))
                .orElse(0L);
    }

    private String getStreamFilePathPrefix(ConsensusNode node) {
        var prefixes = new ArrayList<String>();
        String pathPrefix = properties.getCommon().getPathPrefix();
        if (!StringUtils.isEmpty(pathPrefix)) {
            prefixes.add(pathPrefix);
        }
        prefixes.add(String.valueOf(importerProperties.getShard()));
        prefixes.add(String.valueOf(node.getNodeId()));
        return StringUtils.join(prefixes, StreamFileProvider.SEPARATOR);
    }

    private Collection<ConsensusNode> getRandomizedNodes() {
        var nodes = new ArrayList<>(consensusNodeService.getNodes());
        Collections.shuffle(nodes);
        return nodes;
    }

    private RecordFile transform(BlockFile blockFile) {
        return new RecordFile();
    }

    private void verify(long blockNumber, RecordFile recordFile) {
        if (blockNumber != recordFile.getIndex()) {
            throw new InvalidStreamFileException(String.format(
                    "Block number mismatch: actual - %d, expected - %d", recordFile.getIndex(), blockNumber));
        }

        String expectedPrevHash = lastRecordFile.get().map(RecordFile::getHash).orElse(null);
        if (SHA_384.isHashEmpty(expectedPrevHash)) {
            return;
        }

        if (!recordFile.getPreviousHash().contentEquals(expectedPrevHash)) {
            throw new HashMismatchException(
                    recordFile.getName(), expectedPrevHash, recordFile.getPreviousHash(), "Block");
        }
    }
}
