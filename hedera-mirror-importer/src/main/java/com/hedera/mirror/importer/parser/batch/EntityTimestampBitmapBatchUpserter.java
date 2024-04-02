package com.hedera.mirror.importer.parser.batch;

import com.google.common.base.Stopwatch;
import com.hedera.mirror.importer.domain.EntityTimestampBitmap;
import com.hedera.mirror.importer.parser.CommonParserProperties;
import io.micrometer.core.instrument.MeterRegistry;
import jakarta.inject.Named;
import java.io.DataInputStream;
import java.io.DataOutputStream;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Collection;
import java.util.Optional;
import java.util.concurrent.atomic.AtomicLong;
import javax.sql.DataSource;
import lombok.CustomLog;
import org.roaringbitmap.longlong.Roaring64Bitmap;

@CustomLog
@Named
public class EntityTimestampBitmapBatchUpserter extends BatchInserter {

    public static final Path ENTITY_TIMESTAMP_BITMAP_PATH = Path.of("/tmp/entity_timestamp_bitmap");

    static {
        try {
            Files.createDirectories(ENTITY_TIMESTAMP_BITMAP_PATH);
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    public EntityTimestampBitmapBatchUpserter(DataSource dataSource,
            MeterRegistry meterRegistry,
            CommonParserProperties properties) {
        super(EntityTimestampBitmap.class, dataSource, meterRegistry, properties);
    }

    @Override
    public void persist(Collection<?> items) {
        var stopwatch = Stopwatch.createStarted();
        var ioRTime = new AtomicLong();
        var ioWTime = new AtomicLong();
        for (var item : items) {
            var entityTimestampBitmap = (EntityTimestampBitmap) item;
            readEntityTimestampBitmap(entityTimestampBitmap.getEntityId(), ioRTime)
                    .map(existing -> {
                        existing.getTimestampBitmap().or(entityTimestampBitmap.getTimestampBitmap());
                        return existing;
                    })
                    .or(() -> Optional.of(entityTimestampBitmap))
                    .ifPresent((value) -> ioWTime.addAndGet(writeEntityTimestampBitmap(value)));
        }

        long total = ioRTime.get() + ioWTime.get();
        log.info("Upserted {} entity timestamp bitmaps in {}, io read/write/total took {}/{}/{} ms", items.size(),
                stopwatch, ioRTime.get(), ioWTime.get(), total);
    }

    private Optional<EntityTimestampBitmap> readEntityTimestampBitmap(long entityId, AtomicLong ioRTime) {
        var filePath = ENTITY_TIMESTAMP_BITMAP_PATH.resolve(String.valueOf(entityId));
        if (!Files.exists(filePath)) {
            return Optional.empty();
        }

        try (var in = new DataInputStream(new FileInputStream(filePath.toFile()))) {
            var bitmap = new Roaring64Bitmap();
            long start = System.currentTimeMillis();
            bitmap.deserialize(in);
            ioRTime.addAndGet(System.currentTimeMillis() - start);
            return Optional.of(EntityTimestampBitmap.builder()
                    .entityId(entityId)
                    .timestampBitmap(bitmap)
                    .build());
        } catch (IOException e) {
            log.error("Error reading entity timestamp bitmap from file: {}", filePath, e);
            throw new RuntimeException(e);
        }
    }

    private long writeEntityTimestampBitmap(EntityTimestampBitmap entityTimestampBitmap) {
        var filePath = ENTITY_TIMESTAMP_BITMAP_PATH.resolve(String.valueOf(entityTimestampBitmap.getEntityId()));
        try (var out = new DataOutputStream(new FileOutputStream(filePath.toFile()))) {
            var bitmap = entityTimestampBitmap.getTimestampBitmap();
            bitmap.runOptimize();
            long start = System.currentTimeMillis();
            bitmap.serialize(out);
            return System.currentTimeMillis() - start;
        } catch (IOException e) {
            log.error("Error writing entity timestamp bitmap to file: {}", filePath, e);
            throw new RuntimeException(e);
        }
    }
}
