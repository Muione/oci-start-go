# Quota Backend Optimization — Report

## Status
Complete. Build, tests, vet all green. Committed to `master`.

## Summary
Replaced frontend N+1 service probing with a single backend endpoint that
concurrently filters services-with-quota (semaphore=5), cached 5 min per
tenant. Added a 2-min per-(tenant, service) quota cache on page 0.

## Changes
- `internal/oci/limits.go` — added `ServiceHasLimits`: single
  `ListLimitValues` first-page probe (cheap filter), vs the full two-pass
  `GetServiceQuotasPaged`.
- `internal/service/quota.go` — added two stdlib in-memory caches
  (`sync.Mutex` + map + `time`) on `QuotaService`:
  - `svcCache` (tenantID → services-with-quota, 5 min TTL)
  - `quotaCache` (tenantID+service → QuotaPage, 2 min TTL, page 0 only)
  - new `ListServicesWithQuota`: concurrent probe of each service via
    `ServiceHasLimits`, semaphore-limited to 5.
  - `GetQuota`: 2-min cache check at top, store on success (page 0 only).
  - `ListServices` left as-is (raw full list).
- `internal/httpapi/handler_quota.go` — `tenantQuotaServices` now calls
  `ListServicesWithQuota`. `tenantQuota` left as-is (raw `c.JSON` of
  `*QuotaPage` — frontend interceptor treats it as data; contract preserved).

## Commits
- `6ff1a6a` — perf(quota): concurrent service filtering + 5min cache, 2min quota cache
  (3 files, +144/-2)

## Verification
- `go build ./cmd/oci-start` — ok
- `go test ./internal/service/... ./internal/oci/... -count=1 -timeout 60s` — ok
  (service 0.941s, oci 0.279s, region 0.010s)
- `go vet ./...` — clean

## Concerns / Notes
- **Cache is in-process, unbounded.** No eviction beyond TTL-on-read; entries
  for a tenant/service stay in the map until next read past TTL (then replaced).
  For the tenant counts this app handles this is fine; if multi-tenant scale
  grows, add a periodic sweep or a size cap. Stdlib only, no new deps —
  deliberate per spec.
- **Probe errors exclude the service.** `ListServicesWithQuota` sets
  `ok = pErr == nil && has`; a transient OCI error on one service's probe drops
  it from the result for the next 5 min (cached). Acceptable for a filter list
  (the user can still query quota directly via `tenantQuota`); if false-negative
  visibility matters, surface probe errors separately.
- **`probeResult.has` is redundant with `ok`** (`ok` already encodes `has`).
  Kept verbatim per spec; trivial dead field, not worth deviating from the
  explicit requirement.
- **Quota cache stores page 0 only.** Pages >0 always hit OCI. The common
  frontend case is page 0; explicit per spec.
- **Frontend counterpart already merged** as `bc61d37`
  ("eliminate quota service probing") — this backend endpoint is what that
  frontend now expects.
