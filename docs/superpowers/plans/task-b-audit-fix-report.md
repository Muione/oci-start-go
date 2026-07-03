# Task B — Audit Log Empty Fix

## Status
Done.

## Root cause
`tenantAuditLog` returned a custom envelope (`{success, data, nextPageToken}`) where `data` was the raw events array. The frontend axios interceptor unwraps `b.data` on `success`, so the frontend received the array directly — then `r.data` read `undefined` and the list rendered empty.

## Fix
Replaced custom `c.JSON` calls with the standard `response.OK(c, response.SuccessData(result))` / `response.Fail(...)` envelope used by every other handler. `result` is `*oci.AuditLogPage`, whose JSON tags are `data` and `nextPageToken`; the interceptor unwraps to the `AuditLogPage` object, so frontend `r.data` now correctly resolves to the events array. Switched the parse-error status from 500 to 400 (matches `tenant_user.go` pattern).

## Verification
- `go build ./cmd/oci-start` — ok
- `go vet ./internal/httpapi/...` — clean
- `go test ./internal/httpapi/... -count=1 -timeout 30s` — `ok` (0.087s)

## Commits
- `480db23` fix(httpapi): use standard response envelope for audit log to fix empty list

## Files
- `/home/ubuntu/workspace-oci-start-rewrite/oci-start-go/internal/httpapi/handler_audit.go`

## Concerns
- No httpapi-level test exercises the audit handler path (the package test suite passed but does not cover `tenantAuditLog`). Adding one is out of scope for this fix; flagged for the test-debt backlog already noted in CLAUDE.md.
- Did not touch the frontend (`loadAudit`); the new envelope makes the existing `r.data` access correct, so no frontend change is needed.
