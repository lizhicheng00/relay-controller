package com.huawei.devbridge.relaycontroller.infrastructure.persistence;

import static org.junit.jupiter.api.Assertions.assertAll;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.Test;

class MeteringPartitionManagerTest {
    private static final long HOUR_SECONDS = 3600;
    private static final long CURRENT_HOUR =
            Instant.parse("2026-07-30T08:00:00Z").getEpochSecond();

    @Test
    void createsSevenDayWindowWithTwoFutureHours() {
        List<Long> boundaries =
                MeteringPartitionManager.boundariesToCreate(null, CURRENT_HOUR);

        assertAll(
                () -> assertEquals(171, boundaries.size()),
                () -> assertEquals(
                        CURRENT_HOUR - 7 * 24 * HOUR_SECONDS + HOUR_SECONDS,
                        boundaries.get(0)),
                () -> assertEquals(
                        CURRENT_HOUR + 3 * HOUR_SECONDS,
                        boundaries.get(boundaries.size() - 1)));
    }

    @Test
    void createsOnlyMissingFutureBoundary() {
        List<Long> boundaries = MeteringPartitionManager.boundariesToCreate(
                CURRENT_HOUR + 2 * HOUR_SECONDS, CURRENT_HOUR);

        assertEquals(List.of(CURRENT_HOUR + 3 * HOUR_SECONDS), boundaries);
        assertTrue(MeteringPartitionManager.boundariesToCreate(
                CURRENT_HOUR + 3 * HOUR_SECONDS, CURRENT_HOUR).isEmpty());
    }

    @Test
    void namesPartitionByCoveredUtcHour() {
        long boundary = Instant.parse("2026-07-30T09:00:00Z").getEpochSecond();

        assertEquals("p_2026073008", MeteringPartitionManager.partitionName(boundary));
    }
}
