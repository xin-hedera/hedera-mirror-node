package com.hedera.mirror.importer.parser.batch;

import com.google.common.base.Stopwatch;
import com.hedera.mirror.importer.domain.EntityTimestampBitmap;
import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.Timer;
import jakarta.inject.Named;
import java.util.Collection;
import java.util.HashMap;
import java.util.Map;
import lombok.CustomLog;

@CustomLog
@Named
//@Primary
public class EntityTimestampBitmapBatchUpserterInMemory implements EntityTimestampBitmapBatchUpserter {

    private final Timer latencyMetric;
    private final Counter rowsMetric;
    private final Map<Long, EntityTimestampBitmap> state = new HashMap<>();

    public EntityTimestampBitmapBatchUpserterInMemory(MeterRegistry meterRegistry) {
        latencyMetric = Timer.builder(LATENCY_METRIC)
                .description("The time it took to batch insert rows")
                .tag("table", "entity_timestamp_bitmap")
                .tag("upsert", "false")
                .register(meterRegistry);
        rowsMetric = Counter.builder("hedera.mirror.importer.batch.rows")
                .description("The number of rows inserted into the table")
                .tag("table", "entity_timestamp_bitmap")
                .register(meterRegistry);
    }

    @Override
    public void persist(Collection<?> items) {
        var stopwatch = Stopwatch.createStarted();
        for (var item : items) {
            var entityTimestampBitmap = (EntityTimestampBitmap) item;
            state.merge(entityTimestampBitmap.getEntityId(), entityTimestampBitmap, (existing, value) -> {
                var bitmap = existing.getTimestampBitmap();
                bitmap.or(value.getTimestampBitmap());
                bitmap.runOptimize();
                return existing;
            });
        }

        stopwatch.stop();
        log.info("Upserted {} entity timestamp bitmaps in {}", items.size(), stopwatch);
        latencyMetric.record(stopwatch.elapsed());
        rowsMetric.increment(items.size());
    }
}
