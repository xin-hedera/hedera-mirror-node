package com.hedera.mirror.importer.parser.batch;

import javax.sql.DataSource;
import java.util.ArrayList;
import java.util.Collection;
import java.util.LinkedList;
import java.util.List;

import com.google.common.base.Stopwatch;
import com.hedera.mirror.common.domain.transaction.TransactionHash;
import com.hedera.mirror.importer.parser.CommonParserProperties;
import io.micrometer.core.instrument.MeterRegistry;
import lombok.CustomLog;

@CustomLog
public class TransactionHashBatchPersister implements BatchPersister {

    private final List<Collection<TransactionHash>> shards;
    private final List<BatchPersister> persisters;

    public TransactionHashBatchPersister(DataSource dataSource, MeterRegistry meterRegistry, CommonParserProperties commonParserProperties) {
        shards = new ArrayList<>();
        persisters = new ArrayList<>();
        for (int i = 0; i < 32; i++) {
            shards.add(new LinkedList<>());
            String shardName = String.format("transaction_hash_sharded_%02d", i);
            persisters.add(new BatchInserter(TransactionHash.class, dataSource, meterRegistry, commonParserProperties, shardName));
        }
    }

    @Override
    public void persist(Collection<? extends Object> items) {
        if (items.isEmpty()) {
            return;
        }

        var stopwatch = Stopwatch.createStarted();
        for (var item : items) {
            TransactionHash transactionHash = (TransactionHash) item;
            int shardId = Math.floorMod(transactionHash.getHash()[0], 32);
            shards.get(shardId).add(transactionHash);
        }

        for (int i = 0; i < shards.size(); i++) {
            var shard = shards.get(i);
            if (shard.isEmpty()) {
                continue;
            }

            var persister = persisters.get(i);
            persister.persist(shard);
            shard.clear();
        }

        log.info("Inserted {} items to transaction_hash_sharded in {}", items.size(), stopwatch);
    }
}
