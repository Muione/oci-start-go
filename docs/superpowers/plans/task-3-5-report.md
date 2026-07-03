# Task 3-5 Report

## Status: DONE

## Commits

- `1131fbf` — `fix(frontend): fix TenantList N+1 query, account type display, and batch check`

## Changes Made

### Task 3: Eliminate N+1 query and add instance count column

1. **Tenant interface** — added `planType?: string` field (instanceCount already existed)
2. **load() function** — replaced N+1 `Promise.allSettled` loop (one GET per tenant for instance counts) with a single `/tenants/listAll` call; backend now returns `instanceCount` inline
3. **Template** — added "实例数" column (width 80, centered) immediately after "存活天数" column, displaying `row.instanceCount ?? 0`

### Task 4: Fix account type display

- Changed el-tag in "账号类型" column to prefer `row.planType` over `row.accountType` via `row.planType || row.accountType` fallback

### Task 5: Fix batch check to use batch endpoint

- Replaced sequential per-tenant GET loop in `startBatchCheck()` with a single POST to `/tenants/check-batch` sending all tenant IDs at once; progress jumps to 10% on request start, 100% on response

## Verification

- `vue-tsc --noEmit` — passed with zero errors
- Diff: 1 file changed, 9 insertions, 16 deletions (net -7 lines)

## Concerns

None. All three tasks modify the same file and apply cleanly. Backend endpoints `/tenants/listAll` (with instanceCount/planType in response) and `/tenants/check-batch` must exist for these frontend changes to work end-to-end.
