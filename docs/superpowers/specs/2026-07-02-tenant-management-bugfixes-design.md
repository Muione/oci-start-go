# Tenant Management Bug Fixes & Enhancements

Date: 2026-07-02
Status: Approved for implementation

## Problem Statement

The tenant management pages (list + detail) have multiple bugs and missing features compared to the Java reference. 9 bugs prevent core functionality from working; 4 enhancements improve usability.

## Bug Fixes (9 items)

### BUG-001: Account Cost Not Saving

**Symptom:** Editing account cost in tenant detail appears to succeed but doesn't persist.

**Root cause:** Frontend `saveAccountCost()` calls `PUT /tenants/:id` with spread tenant (missing new cost value). Backend `UpdateTenantFields` SQL doesn't include `accountCost` — it lives in `cloud_tenancy` table.

**Fix:**
- Backend: Add `POST /tenants/:id/account-cost` handler
  - Handler: `tenantAccountCostUpdate` in `httpapi/tenant_ext.go`
  - Service: `TenantService.UpdateCostByID(ctx, id, cost)` — looks up tenant by ID, gets `tenancy` name, calls existing `repo.UpdateCloudTenancyCost`
  - Request body: `{ "cost": "$29.99" }`
  - Route: register in `server.go` under protected group
- Frontend: `saveAccountCost()` calls `POST /tenants/${tenantId}/account-cost` with `{ cost: editCostValue }`, then refreshes tenant data

**Files:**
- `internal/httpapi/tenant_ext.go` — new handler
- `internal/service/tenant.go` — new `UpdateCostByID` method
- `internal/httpapi/server.go` — new route
- `frontend/src/views/TenantDetail.vue` — fix `saveAccountCost()`

---

### BUG-002: Social Config Edit Fails

**Symptom:** Editing an existing social login config silently fails.

**Root cause:** Frontend sends `PUT /tenants/:id/social/:socialId` for edits, but backend only has `POST /tenants/:id/social` (handles both create/update via body `id` field). The `PUT` route doesn't exist.

**Fix:** Frontend only.
- Change `saveSocial()` to always use `POST /tenants/${tenantId}/social`
- Include `id` in request body for edits: `{ ...form, id: socialEditId }`
- Remove the conditional `PUT`/`POST` method selection

**Files:**
- `frontend/src/views/TenantDetail.vue` — fix `saveSocial()`

---

### BUG-003: Create User Form Field Mismatch

**Symptom:** Creating an IAM user always fails with "用户名和邮箱不能为空".

**Root cause:** Frontend sends `{ name, description }` but backend expects `{ username, email }`. Email is required by OCI IAM.

**Fix:** Frontend only.
- Change `addUserForm` from `{ name, description }` to `{ username, email, groupName }`
- Update dialog form: username (required), email (required), group dropdown (optional)
- Groups are already loaded by `loadGroups()` — offer dropdown selection
- Update `createUser()` to send correct field names

**Files:**
- `frontend/src/views/TenantDetail.vue` — fix `addUserForm` and dialog

---

### BUG-004: Account Type Wrong on List Page

**Symptom:** Tenant list shows wrong/empty account type, but detail page cost tab shows correct plan type.

**Root cause:** List page reads `tenant.account_type` from local DB (user-entered, often blank). Cost tab queries live OCI subscription API which returns `FREE_TIER`/`PAYG`.

**Fix:**
- Backend: In `TenantUserService.UpdateAccountDetail()`, after fetching tenancy detail, also fetch subscription info via `oci.GetSubscriptionInfo()` and update `tenant.account_type` with the OCI `planType` value
- Frontend: Update `accountTypeLabel()` in `tenant-utils.ts` to handle OCI values: `FREE_TIER` → 免费层, `PAYG` → 按量付费, in addition to existing `trial`/`paid`/`free`/`enterprise` mappings

**Files:**
- `internal/service/tenant_user.go` — enhance `UpdateAccountDetail()`
- `frontend/src/utils/tenant-utils.ts` — extend `accountTypeLabel()`

---

### BUG-005: Subscription Days Shows "—"

**Symptom:** Tenant detail → Costs → subscription days always shows "—".

**Root cause:** Backend returns `{ activeDays: N }` but frontend reads `r?.days || r?.subscriptionDays`.

**Fix:** Frontend only.
- Change to `r?.activeDays`

**Files:**
- `frontend/src/views/TenantDetail.vue` — fix `loadSubscriptionDays()`

---

### BUG-006: Active Days Calculation Uses Wrong Time Source

**Symptom:** List page "存活天数" doesn't match actual subscription duration.

**Root cause:** `calculateActiveDays()` uses `register_detail.register_time` or `tenant.created_at`. When `UpdateAccountDetail()` runs, it sets `register_time = time.Now()` instead of the actual OCI subscription `timeStart`.

**Fix:**
- Backend: In `TenantUserService.UpdateAccountDetail()`, get the subscription `timeStart` from `oci.GetSubscriptionInfo()` and use it for `register_detail.register_time` instead of `time.Now()`
- Backend: `GetSubscriptionDays()` should also return the `timeStart` in the response so it can be reused

**Files:**
- `internal/service/tenant_user.go` — fix `UpdateAccountDetail()` to use subscription `timeStart`
- `internal/oci/identity.go` — ensure `GetSubscriptionDays` returns `timeStart`

---

### BUG-007: Notification Recipient State Hardcoded

**Symptom:** Notification recipients never show green "VERIFIED" tag.

**Root cause:** `GetNotificationRecipients()` hardcodes `State: "active"` for all recipients instead of using actual OCI verification state.

**Fix:**
- Backend: In `oci.GetNotificationRecipients()`, use the actual state from the OCI SCIM API response. If the API doesn't return per-recipient state, set to `"VERIFIED"` (since existing recipients in OCI are already verified).

**Files:**
- `internal/oci/identity.go` — fix `GetNotificationRecipients()`

---

### BUG-008: MFA Status Silently Fails

**Symptom:** MFA settings page shows all "未启用" even when MFA is configured.

**Root cause:** `GetMfaStatus()` silently returns zero-value `&MfaStatus{}` when the SCIM API fails, swallowing the error. The actual cause may also be an incorrect SCIM endpoint or missing Identity Domain ID in the API call.

**Fix:**
- Backend: Return error from `GetMfaStatus()` when the SCIM API call fails (don't swallow)
- Add error logging for diagnosis
- Verify the SCIM endpoint URL matches the Java reference (`MFAUtils.getMFAConfiguration`)
- Handler already propagates errors correctly — just the service function needs fixing

**Files:**
- `internal/oci/identity.go` — fix error handling in `GetMfaStatus()`

**Note:** If the SCIM endpoint itself is wrong, this fix requires verifying the correct endpoint against the Java `MFAUtils` class.

---

### BUG-009: Audit Log Blank

**Symptom:** Audit log tab shows no data.

**Root cause:** Likely OCI API error silently failing or compartment OCID not being passed correctly. The frontend response format was verified as correct (after axios unwrapping, `r?.data` yields the events array). The issue is either:
1. The OCI audit API call itself failing silently
2. Compartment OCID not being derived correctly from tenant credentials
3. Missing error propagation from service to handler

**Fix:**
- Backend: Add error logging in `ListRecentAuditEvents()` and `ListAuditEventsByDateRange()`
- Ensure compartment OCID is derived correctly from tenant credentials (tenancy OCID = compartment root)
- Verify the audit API endpoint and parameters match the Java reference
- If the OCI API returns an error, propagate it to the frontend instead of returning empty results

**Files:**
- `internal/oci/audit.go` — add logging, verify compartment OCID derivation, fix error propagation

**Note:** This may require live OCI credentials to fully diagnose. The fix focuses on adding proper error logging and propagation so the root cause can be identified at runtime.

---

## Enhancements (4 items)

### ENH-010: Quota Service Dropdown

**Current:** Quota section hardcodes `compute` service. Backend supports any service name.

**Fix:**
- Backend: Add `GET /tenants/:id/quota/services` endpoint → calls `oci.ListLimitServices()` → returns list of `{ name, description }` objects
- Frontend: Add `el-select` dropdown above quota table, populated from new endpoint. Default to `compute`. On change, reload quota with selected service name.

**Files:**
- `internal/httpapi/handler_quota.go` — new handler
- `internal/httpapi/server.go` — new route
- `frontend/src/views/TenantDetail.vue` — add service dropdown

---

### ENH-011: Instance Name Clickable

**Current:** Instance names in tenant detail → instances tab are plain text.

**Fix:** Frontend only.
- Make `displayName` column a clickable link
- Navigate to `/instances/${row.id}` on click (matching the existing instance detail route)

**Files:**
- `frontend/src/views/TenantDetail.vue` — add click handler on displayName column

---

### ENH-012: Remove WeChat Login

**Current:** Social types include `WEIXIN`.

**Fix:** Frontend only.
- Remove `'WEIXIN'` from `socialTypes` array

**Files:**
- `frontend/src/views/TenantDetail.vue` — edit `socialTypes`

---

### ENH-013: Cost Table Missing Columns

**Current:** Cost table shows service and amount but lacks currency, SKU, region.

**Fix:** Frontend only.
- Add `el-table-column` for `currency`, `skuName`, `region` to the cost table
- Data already comes from the API response, just not displayed

**Files:**
- `frontend/src/views/TenantDetail.vue` — add table columns

---

## Implementation Order

1. Backend bug fixes (BUG-001, 004, 006, 007, 008, 009) — all in Go service/OCI layer
2. Frontend bug fixes (BUG-002, 003, 005) — pure frontend
3. Enhancements (ENH-010, 011, 012, 013) — mixed BE/FE

## Testing

- BUG-001: Handler test for `POST /tenants/:id/account-cost`
- BUG-002-005: Frontend-only, manual verification
- BUG-006: Unit test for `calculateActiveDays` with subscription `timeStart`
- BUG-007-009: Integration tests require live OCI credentials — manual verification
- ENH-010: Handler test for `GET /tenants/:id/quota/services`
- ENH-011-013: Frontend-only, manual verification
