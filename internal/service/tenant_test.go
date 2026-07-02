// Package service — tenant_test.go: unit tests for tenant-related pure functions.
// Tests calculateActiveDays (TE-001: subscription time query) and the P-1 List
// N+1 fix (ListTenantsWithCounts single round-trip).
package service

import (
	"context"
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

// TestTenantList_NoNPlusOne is the P-1 regression: List must enrich every tenant
// (register_time / boot count / child count) in a single round-trip via
// ListTenantsWithCounts, not with a per-tenant fan-out that scales with N.
func TestTenantList_NoNPlusOne(t *testing.T) {
	store, qc, _ := newCountingStore(t)
	ctx := context.Background()

	// Seed 5 tenants with varied enrichment: register_detail each, boot
	// instances on tenants 1 & 2, and tenants 2 & 3 parented to tenant 1.
	for i := 1; i <= 5; i++ {
		paren := 0
		if i == 2 || i == 3 {
			paren = 1 // tenant 1 has two children
		}
		mustExec(t, store, `INSERT INTO tenant (id, tenant_id, user_name, region, created_at, is_active, paren_id)
			VALUES (?, ?, ?, 'us-phoenix-1', ?, 1, ?)`,
			i, fmt.Sprintf("ocid.tenancy.%d", i), fmt.Sprintf("u%d", i),
			fmt.Sprintf("2026-01-0%d 00:00:00", i), paren)
		mustExec(t, store, `INSERT INTO register_detail (tenant_id, register_time) VALUES (?, ?)`,
			fmt.Sprintf("ocid.tenancy.%d", i), fmt.Sprintf("2026-01-%02d 10:00:00", i))
	}
	mustExec(t, store, `INSERT INTO boot_instance (tenant_id, status) VALUES (1,1),(1,1),(1,0),(2,1)`)

	qc.Store(0) // reset: count only List's queries, not seeding
	res, err := NewTenantService(store, nil, nil).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res) != 5 {
		t.Fatalf("want 5 tenants, got %d", len(res))
	}
	// N+1 guard: one round-trip regardless of tenant count. The old code issued
	// 1 + 3*N queries (ListTenants + per-row register/boot/children).
	if got := qc.Load(); got != 1 {
		t.Errorf("List issued %d DB queries, want 1 (no per-tenant fan-out)", got)
	}

	byID := map[int64]TenantResp{}
	for _, r := range res {
		byID[r.ID] = r
	}
	if !byID[1].HasBootTask {
		t.Errorf("tenant 1: HasBootTask=false, want true (2 active boot instances)")
	}
	if !byID[2].HasBootTask {
		t.Errorf("tenant 2: HasBootTask=false, want true (1 active boot instance)")
	}
	if byID[3].HasBootTask {
		t.Errorf("tenant 3: HasBootTask=true, want false")
	}
	if !byID[1].HasChildren {
		t.Errorf("tenant 1: HasChildren=false, want true (tenants 2,3)")
	}
	if byID[4].HasChildren {
		t.Errorf("tenant 4: HasChildren=true, want false")
	}
}
