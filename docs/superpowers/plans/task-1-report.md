# Task 1 Report: SQL — Add instance_count subquery

## Status: DONE

## Commits
- `18e86ee` feat(repo): add instance_count subquery to ListTenantsWithCounts

## Changes Made

### Primary change
- `internal/repo/queries/tenant.sql`: Added `(SELECT COUNT(*) FROM instance_detail WHERE tenant_id = t.id) AS instance_count` subquery to `ListTenantsWithCounts`, positioned between `t.is_active` and the `register_time` subquery.

### sqlc regeneration (cascading fixes)
Running `sqlc generate` now that sqlc is available produced correct Go code in `tenant.sql.go` and `instance_detail.sql.go`, but exposed duplicate hand-written placeholder files that had to be removed:

- **Deleted** `internal/repo/tenant_extra.sql.go` — hand-written placeholder, now superseded by sqlc-generated `tenant.sql.go`.
- **Deleted** `internal/repo/instance_detail_extra.sql.go` — same situation; sqlc generates `instance_detail.sql.go`.

The sqlc-generated signatures for `FindConsoleInstanceInfo`, `FindRescueInstanceInfo`, and `FindCompartmentID` changed from `string` to `sql.NullString` parameters. Fixed all callers:
- `cmd/oci-start/main.go` (3 call sites)
- `internal/httpapi/console_connection.go` (2 call sites + added `database/sql` import)
- `internal/repo/instance_detail_extra_test.go` (6 call sites)

### Test updates
- `internal/repo/tenant_extra_test.go`: Added `instance_detail` table to `setupTenantAggDB`, seeded 3 instances for tenant 1, added `instance_count` spot-check assertions.

## Test Results
- `go vet ./...` — clean
- `go test ./internal/repo/... -v` — all 11 tests PASS
- `go test ./...` — all packages PASS (0 failures)

## Generated struct verification
`ListTenantsWithCountsRow` in `internal/repo/tenant.sql.go` now contains:
```go
InstanceCount     int64          `json:"instance_count"`
```
positioned between `IsActive` and `RegisterTime`, matching the SQL column order.

## Concerns
None. The `console_connection_extra.sql.go` and `ssh_keys_extra.sql.go` hand-written files remain because their source SQL files (`console_connections.sql`, `ssh_keys.sql`) are not listed in `sqlc.yaml`. They will need the same treatment if/when those SQL files are added to the sqlc config.
