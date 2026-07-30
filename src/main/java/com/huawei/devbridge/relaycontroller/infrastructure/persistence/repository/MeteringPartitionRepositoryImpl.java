package com.huawei.devbridge.relaycontroller.infrastructure.persistence.repository;

import com.huawei.devbridge.relaycontroller.domain.repository.MeteringPartitionRepository;
import java.sql.Connection;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Statement;
import java.time.Instant;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.List;
import java.util.StringJoiner;
import lombok.RequiredArgsConstructor;
import org.springframework.jdbc.core.ConnectionCallback;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

@Repository
@RequiredArgsConstructor
public class MeteringPartitionRepositoryImpl implements MeteringPartitionRepository {
    private static final String LOCK_NAME = "relay-metering-partitions";
    private static final String FUTURE_PARTITION = "p_future";
    private static final long HOUR_SECONDS = 3600;
    private static final long RETENTION_SECONDS = 7 * 24 * HOUR_SECONDS;
    private static final int FUTURE_HOURS = 2;
    private static final DateTimeFormatter PARTITION_NAME =
            DateTimeFormatter.ofPattern("'p_'yyyyMMddHH").withZone(ZoneOffset.UTC);

    private final JdbcTemplate jdbcTemplate;

    @Override
    public void maintain(long now) {
        jdbcTemplate.execute((ConnectionCallback<Void>) connection -> {
            if (!acquireLock(connection)) {
                return null;
            }
            try {
                long currentHour = now - now % HOUR_SECONDS;
                ensurePartitions(connection, currentHour);
                dropExpiredPartitions(connection, currentHour - RETENTION_SECONDS);
            } finally {
                releaseLock(connection);
            }
            return null;
        });
    }

    private static void ensurePartitions(Connection connection, long currentHour) throws SQLException {
        List<Partition> partitions = loadPartitions(connection);
        if (partitions.stream().noneMatch(partition -> FUTURE_PARTITION.equals(partition.name()))) {
            throw new SQLException("tunnel_metering future partition is missing");
        }

        long retentionBoundary = currentHour - RETENTION_SECONDS;
        long latestBoundary = partitions.stream()
                .filter(partition -> !FUTURE_PARTITION.equals(partition.name()))
                .mapToLong(Partition::boundary)
                .max()
                .orElse(retentionBoundary - HOUR_SECONDS);
        long nextBoundary = Math.max(latestBoundary + HOUR_SECONDS, retentionBoundary);
        long desiredBoundary = currentHour + (FUTURE_HOURS + 1L) * HOUR_SECONDS;
        if (nextBoundary > desiredBoundary) {
            return;
        }

        StringJoiner definitions = new StringJoiner(", ");
        for (long boundary = nextBoundary; boundary <= desiredBoundary; boundary += HOUR_SECONDS) {
            definitions.add("PARTITION " + partitionName(boundary)
                    + " VALUES LESS THAN (" + boundary + ")");
        }
        definitions.add("PARTITION " + FUTURE_PARTITION + " VALUES LESS THAN MAXVALUE");
        execute(connection, "ALTER TABLE tunnel_metering REORGANIZE PARTITION "
                + FUTURE_PARTITION + " INTO (" + definitions + ")");
    }

    private static void dropExpiredPartitions(Connection connection, long cutoff) throws SQLException {
        for (Partition partition : loadPartitions(connection)) {
            if (!FUTURE_PARTITION.equals(partition.name())
                    && partition.boundary() <= cutoff
                    && !hasUnsettledRows(connection, partition.name())) {
                execute(connection, "ALTER TABLE tunnel_metering DROP PARTITION " + partition.name());
            }
        }
    }

    private static List<Partition> loadPartitions(Connection connection) throws SQLException {
        String sql = """
                SELECT partition_name, partition_description
                FROM information_schema.partitions
                WHERE table_schema = DATABASE()
                  AND table_name = 'tunnel_metering'
                  AND partition_name IS NOT NULL
                ORDER BY partition_ordinal_position
                """;
        List<Partition> partitions = new ArrayList<>();
        try (Statement statement = connection.createStatement();
                ResultSet resultSet = statement.executeQuery(sql)) {
            while (resultSet.next()) {
                String description = resultSet.getString("partition_description");
                if (!"MAXVALUE".equalsIgnoreCase(description)) {
                    partitions.add(new Partition(
                            resultSet.getString("partition_name"),
                            Long.parseLong(description)));
                } else {
                    partitions.add(new Partition(resultSet.getString("partition_name"), Long.MIN_VALUE));
                }
            }
        }
        return partitions;
    }

    private static boolean hasUnsettledRows(Connection connection, String partitionName) throws SQLException {
        requirePartitionName(partitionName);
        String sql = "SELECT EXISTS (SELECT 1 FROM tunnel_metering PARTITION ("
                + partitionName + ") WHERE settled = 0 LIMIT 1)";
        try (Statement statement = connection.createStatement();
                ResultSet resultSet = statement.executeQuery(sql)) {
            return resultSet.next() && resultSet.getBoolean(1);
        }
    }

    private static boolean acquireLock(Connection connection) throws SQLException {
        try (PreparedStatement statement = connection.prepareStatement("SELECT GET_LOCK(?, 0)")) {
            statement.setString(1, LOCK_NAME);
            try (ResultSet resultSet = statement.executeQuery()) {
                return resultSet.next() && resultSet.getInt(1) == 1;
            }
        }
    }

    private static void releaseLock(Connection connection) throws SQLException {
        try (PreparedStatement statement = connection.prepareStatement("SELECT RELEASE_LOCK(?)")) {
            statement.setString(1, LOCK_NAME);
            statement.execute();
        }
    }

    private static void execute(Connection connection, String sql) throws SQLException {
        try (Statement statement = connection.createStatement()) {
            statement.execute(sql);
        }
    }

    private static String partitionName(long boundary) {
        return PARTITION_NAME.format(Instant.ofEpochSecond(boundary - HOUR_SECONDS));
    }

    private static void requirePartitionName(String partitionName) {
        if (!partitionName.matches("p_\\d{10}")) {
            throw new IllegalArgumentException("invalid metering partition name");
        }
    }

    private record Partition(String name, long boundary) {
    }
}
