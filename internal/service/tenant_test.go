// Package service — tenant_test.go: unit tests for tenant-related pure functions.
// Tests calculateActiveDays (TE-001: subscription time query).
package service

import (
	"fmt"
	"testing"
	"time"
)

func TestCalculateActiveDays_EmptyString(t *testing.T) {
	got := calculateActiveDays("")
	if got != "0" {
		t.Errorf("calculateActiveDays(\"\") = %q, want \"0\"", got)
	}
}

func TestCalculateActiveDays_InvalidFormat(t *testing.T) {
	cases := []string{
		"not-a-date",
		"2024/01/01",
		"01-01-2024",
		"abc123",
	}
	for _, ts := range cases {
		got := calculateActiveDays(ts)
		if got != "0" {
			t.Errorf("calculateActiveDays(%q) = %q, want \"0\"", ts, got)
		}
	}
}

func TestCalculateActiveDays_Today(t *testing.T) {
	// Use UTC time since time.Parse returns UTC
	now := time.Now().UTC()
	ts := now.Format("2006-01-02 15:04:05")
	got := calculateActiveDays(ts)
	if got != "1" {
		t.Errorf("calculateActiveDays(today UTC) = %q, want \"1\"", got)
	}
}

func TestCalculateActiveDays_Yesterday(t *testing.T) {
	yesterday := time.Now().UTC().Add(-24 * time.Hour)
	ts := yesterday.Format("2006-01-02 15:04:05")
	got := calculateActiveDays(ts)
	var days int
	fmt.Sscanf(got, "%d", &days)
	if days < 1 || days > 2 {
		t.Errorf("calculateActiveDays(yesterday) = %q, want 1 or 2", got)
	}
}

func TestCalculateActiveDays_KnownPast(t *testing.T) {
	past := time.Now().UTC().AddDate(0, 0, -10)
	ts := past.Format("2006-01-02 15:04:05")
	got := calculateActiveDays(ts)
	var days int
	fmt.Sscanf(got, "%d", &days)
	if days < 10 || days > 11 {
		t.Errorf("calculateActiveDays(10 days ago) = %q, want 10 or 11", got)
	}
}

func TestCalculateActiveDays_FarPast(t *testing.T) {
	past := time.Now().UTC().AddDate(-1, 0, 0)
	ts := past.Format("2006-01-02 15:04:05")
	got := calculateActiveDays(ts)
	var days int
	fmt.Sscanf(got, "%d", &days)
	if days < 365 || days > 366 {
		t.Errorf("calculateActiveDays(1 year ago) = %q, want 365 or 366", got)
	}
}

func TestCalculateActiveDays_RFC3339(t *testing.T) {
	yesterday := time.Now().UTC().Add(-24 * time.Hour)
	ts := yesterday.Format(time.RFC3339)
	got := calculateActiveDays(ts)
	var days int
	fmt.Sscanf(got, "%d", &days)
	if days < 1 || days > 2 {
		t.Errorf("calculateActiveDays(RFC3339 yesterday) = %q, want 1 or 2", got)
	}
}

func TestCalculateActiveDays_FutureDate(t *testing.T) {
	future := time.Now().UTC().AddDate(1, 0, 0)
	ts := future.Format("2006-01-02 15:04:05")
	got := calculateActiveDays(ts)
	if got != "0" {
		t.Errorf("calculateActiveDays(future) = %q, want \"0\"", got)
	}
}

func TestCalculateActiveDays_ZeroTime(t *testing.T) {
	ts := "2020-01-01 00:00:00"
	got := calculateActiveDays(ts)
	var days int
	fmt.Sscanf(got, "%d", &days)
	if days < 2000 {
		t.Errorf("calculateActiveDays(2020-01-01) = %d, want >= 2000", days)
	}
}
