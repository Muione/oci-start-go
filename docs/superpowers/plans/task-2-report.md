# Task 2 Report: Add InstanceCount and PlanType to TenantResp

- **Status:** DONE
- **Commit:** `2ae2cd1` — feat(service): add instanceCount and planType to TenantResp

## Changes

- `internal/service/tenant.go`:
  - Added `InstanceCount int64` and `PlanType string` fields to `TenantResp` struct (after `HasChildren`)
  - Extracted `accountType` local variable in `toTenantRespFromCounts` to avoid duplicate `ns(r.AccountType)` call
  - Populated `InstanceCount: r.InstanceCount` and `PlanType: accountType` in the return struct

## Test Results

- `go build ./cmd/oci-start` — clean, no errors
- `go test ./internal/service/... -v -count=1` — all 47 tests PASS (1.485s)

## Concerns

None.
