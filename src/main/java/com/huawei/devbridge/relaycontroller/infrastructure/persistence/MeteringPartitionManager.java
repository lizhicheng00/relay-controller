package com.huawei.devbridge.relaycontroller.infrastructure.persistence;

import com.huawei.devbridge.relaycontroller.common.util.TimeUtils;
import java.time.Instant;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.List;
import java.util.StringJoiner;
import lombok.RequiredArgsConstructor;
import net.javacrumbs.shedlock.spring.annotation.SchedulerLock;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class MeteringPartitionManager {
    private static final String FUTURE_PARTITION = "p_future";
    private static final long HOUR_SECONDS = 3600;
    private static final long RETENTION_SECONDS = 7 * 24 * HOUR_SECONDS;
    private static final int FUTURE_HOURS = 2;
    private static final DateTimeFormatter PARTITION_FORMAT =
            DateTimeFormatter.ofPattern("'p_'yyyyMMddHH").withZone(ZoneOffset.UTC);

    private final JdbcTemplate jdbcTemplate;

    @Scheduled(initialDelay = 60_000, fixedDelay = HOUR_SECONDS * 1000)
    @SchedulerLock(name = "meteringPartitions")
    public void maintainPartitions() {
        maintainPartitions(TimeUtils.nowSeconds());
    }

    void maintainPartitions(long now) {
        long currentHour = now - now % HOUR_SECONDS;
        List<Partition> partitions = loadPartitions();
        createMissingPartitions(partitions, currentHour);
        dropExpiredPartitions(partitions, currentHour - RETENTION_SECONDS);
    }

    private void createMissingPartitions(List<Partition> partitions, long currentHour) {
        Long latestBoundary = partitions.isEmpty()
                ? null
                : partitions.get(partitions.size() - 1).boundary();
        List<Long> boundaries = boundariesToCreate(latestBoundary, currentHour);
        if (boundaries.isEmpty()) {
            return;
        }

        StringJoiner definitions = new StringJoiner(", ");
        for (long boundary : boundaries) {
            definitions.add("PARTITION " + partitionName(boundary)
                    + " VALUES LESS THAN (" + boundary + ")");
        }
        definitions.add("PARTITION " + FUTURE_PARTITION + " VALUES LESS THAN MAXVALUE");
        jdbcTemplate.execute("ALTER TABLE tunnel_metering REORGANIZE PARTITION "
                + FUTURE_PARTITION + " INTO (" + definitions + ")");
    }

    private void dropExpiredPartitions(List<Partition> partitions, long cutoff) {
        List<String> expired = new ArrayList<>();
        for (Partition partition : partitions) {
            if (partition.boundary() <= cutoff) {
                expired.add(partition.name());
            }
        }
        if (!expired.isEmpty()) {
            jdbcTemplate.execute("ALTER TABLE tunnel_metering DROP PARTITION "
                    + String.join(", ", expired));
        }
    }

    static List<Long> boundariesToCreate(Long latestBoundary, long currentHour) {
        long firstRetainedBoundary = currentHour - RETENTION_SECONDS + HOUR_SECONDS;
        long nextBoundary = latestBoundary == null
                ? firstRetainedBoundary
                : Math.max(latestBoundary + HOUR_SECONDS, firstRetainedBoundary);
        long finalBoundary = currentHour + (FUTURE_HOURS + 1L) * HOUR_SECONDS;

        List<Long> boundaries = new ArrayList<>();
        for (long boundary = nextBoundary; boundary <= finalBoundary; boundary += HOUR_SECONDS) {
            boundaries.add(boundary);
        }
        return boundaries;
    }

    static String partitionName(long boundary) {
        return PARTITION_FORMAT.format(Instant.ofEpochSecond(boundary - HOUR_SECONDS));
    }

    private List<Partition> loadPartitions() {
        String sql = """
                SELECT partition_name, partition_description
                FROM information_schema.partitions
                WHERE table_schema = DATABASE()
                  AND table_name = 'tunnel_metering'
                  AND partition_name IS NOT NULL
                  AND partition_description != 'MAXVALUE'
                ORDER BY partition_ordinal_position
                """;
        return jdbcTemplate.query(sql, (resultSet, rowNumber) -> new Partition(
                resultSet.getString("partition_name"),
                resultSet.getLong("partition_description")));
    }

    private record Partition(String name, long boundary) {
    }
}
