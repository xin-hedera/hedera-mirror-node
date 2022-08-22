package com.hedera.mirror.importer.generator;

import java.time.Duration;
import lombok.Data;
import org.jetbrains.annotations.NotNull;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.validation.annotation.Validated;

@Data
@Validated
@ConfigurationProperties("hedera.mirror.importer.generator.record")
public class RecordFileGeneratorProperties {
    @NotNull
    private Duration fileInterval = Duration.ofMillis(200L);
    private long startTimestamp = 1_000_000_000L;
    private int transactionsPerFile = 20_200;
}
