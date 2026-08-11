package core

import (
	"testing"
	"time"
)

func TestEncode40Bit(t *testing.T) {
	tests := map[uint64]string{
		1:        "aaaaaaab",
		123456:   "aaaadysa",
		Max40Bit: "77777777",
	}
	for value, expected := range tests {
		if actual := Encode40Bit(value); actual != expected {
			t.Fatalf("Encode40Bit(%d) = %q, want %q", value, actual, expected)
		}
	}
}

func TestIdentifierValidation(t *testing.T) {
	for _, value := range []string{"namespace-a", "ns.sub_1", "A"} {
		if !ValidIdentifier(value) {
			t.Fatalf("expected %q to be valid", value)
		}
	}
	for _, value := range []string{"", " namespace", "namespace/child", "-namespace", string(make([]byte, 129))} {
		if ValidIdentifier(value) {
			t.Fatalf("expected %q to be invalid", value)
		}
	}
}

func TestBillingPeriodUsesShanghaiMonthBoundary(t *testing.T) {
	timestamp := time.Date(2026, 7, 31, 15, 59, 59, 0, time.UTC).Unix()
	start, end := BillingPeriodRange(timestamp)
	if want := time.Date(2026, 6, 30, 16, 0, 0, 0, time.UTC).Unix(); start != want {
		t.Fatalf("period start = %d, want %d", start, want)
	}
	if want := time.Date(2026, 7, 31, 16, 0, 0, 0, time.UTC).Unix(); end != want {
		t.Fatalf("period end = %d, want %d", end, want)
	}
}

func TestNormalizeEnumsIgnoringCase(t *testing.T) {
	tunnelType := "BrIdGe"
	if actual, err := NormalizeTunnelType(&tunnelType, true); err != nil || actual != "bridge" {
		t.Fatalf("normalize tunnel type = %q, %v", actual, err)
	}
	protocol := "HtTpS"
	if actual, err := NormalizeProtocol(&protocol, true); err != nil || actual != "https" {
		t.Fatalf("normalize protocol = %q, %v", actual, err)
	}
}

func TestResolveExpiration(t *testing.T) {
	hours := 24
	resolved, expiration, err := ResolveExpiration(&hours, 72, 1000)
	if err != nil || resolved != 24 || expiration != 87400 {
		t.Fatalf("ResolveExpiration = %d, %d, %v", resolved, expiration, err)
	}
	tooLarge := 721
	if _, _, err := ResolveExpiration(&tooLarge, 72, 1000); err == nil {
		t.Fatal("expected expiration over 720 hours to fail")
	}
}
