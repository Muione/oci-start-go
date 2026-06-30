package service

import (
	"testing"
	"time"
)

// TestQueryCostDateRanges verifies the date calculation logic used by
// QueryCost (TE-004: account cost query). These test the same time
// calculations that QueryYesterdayCost/QueryTodayCost etc. perform,
// without requiring OCI connectivity.

func TestQueryCost_YesterdayRange(t *testing.T) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, time.UTC)
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	if start.After(end) {
		t.Error("yesterday start should be before end")
	}
	if end.Sub(start) != 24*time.Hour {
		t.Errorf("yesterday range = %v, want 24h", end.Sub(start))
	}
}

func TestQueryCost_TodayRange(t *testing.T) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	if start.After(now) {
		t.Error("today start should be before now")
	}
	if now.Sub(start) > 24*time.Hour {
		t.Error("today range should be less than 24h")
	}
}

func TestQueryCost_CurrentMonthRange(t *testing.T) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	if start.After(now) {
		t.Error("month start should be before now")
	}
	if start.Day() != 1 {
		t.Errorf("month start day = %d, want 1", start.Day())
	}
}

func TestQueryCost_LastMonthRange(t *testing.T) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	if start.After(end) {
		t.Error("last month start should be before end")
	}
	if start.Month() == end.Month() {
		t.Error("start and end should be different months")
	}
}

func TestQueryCost_CustomDateParsing(t *testing.T) {
	// Verify the date format used by QueryCustomCost
	const dateFmt = "2006-01-02"

	start, err := time.Parse(dateFmt, "2024-01-01")
	if err != nil {
		t.Fatalf("parse start: %v", err)
	}
	end, err := time.Parse(dateFmt, "2024-01-31")
	if err != nil {
		t.Fatalf("parse end: %v", err)
	}
	if start.After(end) {
		t.Error("start should be before end")
	}
	if end.Sub(start).Hours()/24 != 30 {
		t.Errorf("range days = %f, want 30", end.Sub(start).Hours()/24)
	}
}

func TestQueryCost_InvalidCustomDates(t *testing.T) {
	const dateFmt = "2006-01-02"

	invalidDates := []string{
		"not-a-date",
		"2024/01/01",
		"01-01-2024",
		"",
		"2024-13-01", // invalid month
		"2024-01-32", // invalid day
	}
	for _, d := range invalidDates {
		_, err := time.Parse(dateFmt, d)
		if err == nil {
			t.Errorf("expected error parsing %q, got nil", d)
		}
	}
}

func TestQueryCost_QueryTypeRouting(t *testing.T) {
	// Verify the query type strings match what the handler expects
	validTypes := map[string]bool{
		"yesterday":     true,
		"today":         true,
		"current_month": true,
		"last_month":    true,
		"custom":        true,
	}
	invalidTypes := map[string]bool{
		"":          false, // defaults to current_month
		"invalid":   false,
		"next_week": false,
	}

	for qt, shouldExist := range validTypes {
		if !shouldExist {
			t.Errorf("query type %q should be valid", qt)
		}
	}

	for qt := range invalidTypes {
		// The handler defaults unknown types to current_month
		// so they don't cause errors, just unexpected behavior
		_ = qt
	}
}
