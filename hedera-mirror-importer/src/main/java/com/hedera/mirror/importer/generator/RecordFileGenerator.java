package com.hedera.mirror.importer.generator;

import com.google.common.primitives.Longs;
import com.google.protobuf.ByteString;
import com.hederahashgraph.api.proto.java.AccountAmount;
import com.hederahashgraph.api.proto.java.AccountID;
import com.hederahashgraph.api.proto.java.CryptoTransferTransactionBody;
import com.hederahashgraph.api.proto.java.Duration;
import com.hederahashgraph.api.proto.java.ResponseCodeEnum;
import com.hederahashgraph.api.proto.java.SignatureMap;
import com.hederahashgraph.api.proto.java.SignaturePair;
import com.hederahashgraph.api.proto.java.SignedTransaction;
import com.hederahashgraph.api.proto.java.Transaction;
import com.hederahashgraph.api.proto.java.TransactionBody;
import com.hederahashgraph.api.proto.java.TransactionID;
import com.hederahashgraph.api.proto.java.TransactionRecord;
import com.hederahashgraph.api.proto.java.TransferList;
import java.security.MessageDigest;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.concurrent.atomic.AtomicReference;
import javax.inject.Named;
import lombok.CustomLog;
import lombok.RequiredArgsConstructor;
import lombok.SneakyThrows;
import org.apache.commons.codec.binary.Hex;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.scheduling.annotation.Scheduled;
import reactor.core.publisher.Flux;

import com.hedera.mirror.common.domain.DigestAlgorithm;
import com.hedera.mirror.common.domain.StreamType;
import com.hedera.mirror.common.domain.entity.EntityId;
import com.hedera.mirror.common.domain.transaction.RecordFile;
import com.hedera.mirror.common.domain.transaction.RecordItem;
import com.hedera.mirror.importer.domain.StreamFilename;
import com.hedera.mirror.importer.downloader.StreamFileNotifier;
import com.hedera.mirror.importer.repository.RecordFileRepository;
import com.hedera.mirror.importer.util.ShutdownHelper;
import com.hedera.mirror.importer.util.Utility;

@CustomLog
@ConditionalOnProperty(name = "hedera.mirror.importer.generator.record.enabled", havingValue = "true")
@Named
@RequiredArgsConstructor
public class RecordFileGenerator {

    private final static AccountID FEE_COLLECTOR = AccountID.newBuilder().setAccountNum(98L).build();
    private final static AccountID NODE = AccountID.newBuilder().setAccountNum(3L).build();
    private final static EntityId NODE_ACCOUNT_ID = EntityId.of(NODE);
    private final static AccountID SENDER = AccountID.newBuilder().setAccountNum(3000L).build();
    private final static AccountID RECEIVER = AccountID.newBuilder().setAccountNum(3001L).build();

    private final static SignatureMap DEFAULT_SIGNATURE_MAP = SignatureMap.newBuilder()
            .addSigPair(SignaturePair.newBuilder()
                    .setEd25519(ByteString.copyFrom(Longs.toByteArray(153781L)))
                    .setPubKeyPrefix(ByteString.copyFrom(Longs.toByteArray(4258292L))))
            .build();

    private final RecordFileGeneratorProperties generatorProperties;
    private final AtomicReference<RecordFile> lastRecordFile = new AtomicReference<>();
    private final RecordFileRepository recordFileRepository;
    private final StreamFileNotifier streamFileNotifier;

    @Scheduled(fixedDelay = 10L)
    public void generate() {
        if (ShutdownHelper.isStopping()) {
            return;
        }

        var previousRecordFile = Optional.ofNullable(lastRecordFile.get())
                .or(recordFileRepository::findLatest)
                .orElseGet(() -> RecordFile.builder()
                        .consensusEnd(generatorProperties.getStartTimestamp())
                        .index(0L)
                        .build());
        long timestamp = previousRecordFile.getConsensusEnd() + generatorProperties.getFileInterval().toNanos();
        var startInstant = Instant.ofEpochSecond(0, timestamp);
        var filename = StreamFilename.getFilename(StreamType.RECORD, StreamFilename.FileType.DATA, startInstant);
        log.info("Generating {} transactions for record file {} at timestamp {}",
                generatorProperties.getTransactionsPerFile(), filename, timestamp);

        var recordFile = RecordFile.builder()
                .count((long) generatorProperties.getTransactionsPerFile())
                .consensusStart(timestamp)
                .digestAlgorithm(DigestAlgorithm.SHA_384)
                .fileHash(hashAsString(timestamp - 1))
                .hash(hashAsString(timestamp))
                .hapiVersionMajor(0)
                .hapiVersionMinor(28)
                .hapiVersionPatch(0)
                .index(previousRecordFile.getIndex() + 1)
                .loadStart(Instant.now().getEpochSecond())
                .name(filename)
                .nodeAccountId(NODE_ACCOUNT_ID)
                .previousHash(hashAsString(timestamp - generatorProperties.getFileInterval().toNanos()))
                .size(1024 * 1024)
                .build();

        List<RecordItem> items = new ArrayList<>();
        for (int index = 0; index < generatorProperties.getTransactionsPerFile(); index++) {
            items.add(cryptoTransfer(timestamp, index));
            timestamp++;
        }

        recordFile.setConsensusEnd(timestamp - 1);
        recordFile.setItems(Flux.fromIterable(items));

        streamFileNotifier.verified(recordFile);
        lastRecordFile.set(recordFile);
        log.info("Generated record file {}", recordFile.getName());
    }

    private RecordItem cryptoTransfer(long timestamp, int index) {
        var instant = Instant.ofEpochSecond(0, timestamp);
        var cryptoTransfer = CryptoTransferTransactionBody.newBuilder()
                .setTransfers(TransferList.newBuilder()
                        .addAccountAmounts(AccountAmount.newBuilder().setAccountID(SENDER).setAmount(-100))
                        .addAccountAmounts(accountAmount(SENDER, -100))
                        .addAccountAmounts(accountAmount(RECEIVER, 100)));
        var body = TransactionBody.newBuilder()
                .setCryptoTransfer(cryptoTransfer)
                .setNodeAccountID(NODE)
                .setTransactionFee(200)
                .setTransactionID(TransactionID.newBuilder().setAccountID(SENDER)
                        .setTransactionValidStart(Utility.instantToTimestamp(instant.minusNanos(1))))
                .setTransactionValidDuration(Duration.newBuilder().setSeconds(120L))
                .build();
        var transaction = Transaction.newBuilder()
                .setSignedTransactionBytes(SignedTransaction.newBuilder()
                        .setBodyBytes(body.toByteString())
                        .setSigMap(DEFAULT_SIGNATURE_MAP)
                        .build()
                        .toByteString()
                )
                .build();
        var record = TransactionRecord.newBuilder()
                .setConsensusTimestamp(Utility.instantToTimestamp(instant))
                .setTransactionFee(body.getTransactionFee())
                .setTransactionHash(ByteString.copyFrom(hash(timestamp)))
                .setTransactionID(body.getTransactionID())
                .setTransferList(TransferList.newBuilder()
                        .addAccountAmounts(accountAmount(SENDER, -300))
                        .addAccountAmounts(accountAmount(RECEIVER, 100L))
                        .addAccountAmounts(accountAmount(NODE, 100L))
                        .addAccountAmounts(accountAmount(FEE_COLLECTOR, 100L)));
        record.getReceiptBuilder().setStatus(ResponseCodeEnum.SUCCESS);
        return RecordItem.builder()
                .hapiVersion(RecordFile.HAPI_VERSION_0_23_0)
                .transactionBytes(transaction.toByteArray())
                .transactionIndex(index)
                .recordBytes(record.build().toByteArray())
                .build();
    }

    private AccountAmount accountAmount(AccountID account, long amount) {
        return AccountAmount.newBuilder().setAccountID(account).setAmount(amount).build();
    }

    @SneakyThrows
    private byte[] hash(long value) {
        var digest = MessageDigest.getInstance(DigestAlgorithm.SHA_384.getName());
        return digest.digest(Longs.toByteArray(value));
    }

    private String hashAsString(long value) {
        return Hex.encodeHexString(hash(value));
    }
}
