package com.hedera.mirror.importer.domain;

import lombok.Builder;
import lombok.Data;
import org.roaringbitmap.longlong.Roaring64Bitmap;

@Builder
@Data
public class EntityTimestampBitmap {
    private long entityId;
    private Roaring64Bitmap timestampBitmap;
}
