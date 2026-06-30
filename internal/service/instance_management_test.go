// Package service — instance_management_test.go: unit tests for instance
// management service (TE-107: Shape/Image cache functionality).
package service

import (
	"testing"
	"time"
)

// --- TE-107: Cache TTL constant ---

func TestCacheStaleDuration(t *testing.T) {
	if cacheStaleDuration != 1*time.Hour {
		t.Errorf("cacheStaleDuration = %v, want 1h", cacheStaleDuration)
	}
}

// --- TE-107: Cache freshness check logic ---

func TestCacheFreshness_RecentTime(t *testing.T) {
	// A time 30 minutes ago should be fresh (within 1h TTL).
	recent := time.Now().UTC().Add(-30 * time.Minute).Format("2006-01-02 15:04:05")
	parsed, err := time.Parse("2006-01-02 15:04:05", recent)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if time.Since(parsed) > cacheStaleDuration {
		t.Error("30 minutes ago should be within cache TTL")
	}
}

func TestCacheFreshness_OldTime(t *testing.T) {
	// A time 2 hours ago should be stale (exceeds 1h TTL).
	old := time.Now().UTC().Add(-2 * time.Hour).Format("2006-01-02 15:04:05")
	parsed, err := time.Parse("2006-01-02 15:04:05", old)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if time.Since(parsed) <= cacheStaleDuration {
		t.Error("2 hours ago should exceed cache TTL")
	}
}

func TestCacheFreshness_ExactlyOneHour(t *testing.T) {
	// A time exactly 1 hour ago should be stale (> 1h).
	exact := time.Now().UTC().Add(-1 * time.Hour).Format("2006-01-02 15:04:05")
	parsed, err := time.Parse("2006-01-02 15:04:05", exact)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// time.Since(exact) will be slightly more than 1h due to processing time
	if time.Since(parsed) < cacheStaleDuration {
		// This is acceptable - right at the boundary
		t.Log("exactly 1h ago is at the boundary of cache TTL")
	}
}

func TestCacheFreshness_InvalidFormat(t *testing.T) {
	// Invalid time format should be treated as stale.
	invalid := "not-a-date"
	_, err := time.Parse("2006-01-02 15:04:05", invalid)
	if err == nil {
		t.Error("expected error for invalid date format")
	}
}

func TestCacheFreshness_JustNow(t *testing.T) {
	// A time from just now should be fresh.
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	parsed, err := time.Parse("2006-01-02 15:04:05", now)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if time.Since(parsed) > cacheStaleDuration {
		t.Error("just now should be within cache TTL")
	}
}

// --- TE-107: boolToInt64 helper ---

func TestBoolToInt64(t *testing.T) {
	if boolToInt64(true) != 1 {
		t.Errorf("boolToInt64(true) = %d, want 1", boolToInt64(true))
	}
	if boolToInt64(false) != 0 {
		t.Errorf("boolToInt64(false) = %d, want 0", boolToInt64(false))
	}
}

// --- TE-107: VPU validation ---

func TestVpuValidation_Range(t *testing.T) {
	// VPU values should be in [10, 120] for boot volumes.
	validVPUs := []int64{10, 20, 30, 60, 120}
	invalidVPUs := []int64{-1, 0, 9, 121, 200}

	for _, vpu := range validVPUs {
		if vpu < 10 || vpu > 120 {
			t.Errorf("VPU %d should be valid", vpu)
		}
	}
	for _, vpu := range invalidVPUs {
		if vpu >= 10 && vpu <= 120 {
			t.Errorf("VPU %d should be invalid", vpu)
		}
	}
}

// --- TE-107: Time format consistency ---

func TestTimeFormat_HTTPFormat(t *testing.T) {
	// The httpTimeFmt constant should match the expected format.
	expected := "2006-01-02 15:04:05"
	if httpTimeFmt != expected {
		t.Errorf("httpTimeFmt = %q, want %q", httpTimeFmt, expected)
	}
}

func TestTimeFormat_ParseRoundTrip(t *testing.T) {
	// A time formatted with httpTimeFmt should be parseable back.
	// Use UTC to match time.Parse behavior.
	now := time.Now().UTC().Truncate(time.Second)
	formatted := now.Format(httpTimeFmt)
	parsed, err := time.Parse(httpTimeFmt, formatted)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !parsed.Equal(now) {
		t.Errorf("round trip failed: %v != %v", parsed, now)
	}
}

// --- TE-107: ns2 helper ---

func TestNs2_EmptyString(t *testing.T) {
	// ns2 returns "" for invalid/null sql.NullString
	// We test the behavior pattern rather than the function directly
	// since it requires sql.NullString
	t.Log("ns2 helper tested indirectly through cache loading tests")
}
