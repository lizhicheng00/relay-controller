package service

import (
	"reflect"
	"testing"
	"time"
)

func TestPartitionBoundariesMaintainSevenDaysAndTwoFutureHours(t *testing.T) {
	current := time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC).Unix()
	boundaries := partitionBoundaries(nil, current)
	if len(boundaries) != 171 {
		t.Fatalf("created %d boundaries, want 171", len(boundaries))
	}
	if boundaries[0] != current-7*24*3600+3600 || boundaries[len(boundaries)-1] != current+3*3600 {
		t.Fatalf("unexpected partition range: %d to %d", boundaries[0], boundaries[len(boundaries)-1])
	}
	latest := current + 2*3600
	if actual := partitionBoundaries(&latest, current); !reflect.DeepEqual(actual, []int64{current + 3*3600}) {
		t.Fatalf("missing future boundaries = %v", actual)
	}
}

func TestPartitionNameUsesCoveredUTCHour(t *testing.T) {
	boundary := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC).Unix()
	if actual := partitionName(boundary); actual != "p_2026073008" {
		t.Fatalf("partition name = %q", actual)
	}
}
