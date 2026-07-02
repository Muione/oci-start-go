# Tenant Management Improvements — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix bugs and improve performance of the tenant management pages (TenantList + TenantDetail + Instances).

**Architecture:** Backend adds `instanceCount` and `planType` to the tenant list API response via a SQL subquery addition. Frontend eliminates N+1 queries, adds loaded-flags to prevent empty-list re-fetching, surfaces OCI API errors with retry buttons, fixes subscription days calculation, enables instance click-through navigation, and filters quota services to only those with data.

**Tech Stack:** Go 1.25, sqlc, SQLite, Vue 3, Element Plus, Axios

## Global Constraints

- Do not change public API signatures (exported functions/types/methods)
- SQLite-only, no new DB engines
- Run `sqlc generate` after any SQL query change
- Run `go vet ./...` after Go changes
- Frontend has no test framework — verify manually via browser
- Commit after each task

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/repo/queries/tenant.sql` | Modify | Add `instance_count` subquery to `ListTenantsWithCounts` |
| `internal/repo/tenant_extra.sql.go` | Regenerate | sqlc output (auto-generated) |
| `internal/service/tenant.go` | Modify | Add `InstanceCount`/`PlanType` to `TenantResp`, update mapping |
| `frontend/src/views/TenantList.vue` | Modify | N+1 fix, account type, batch check, instance count column |
| `frontend/src/views/TenantDetail.vue` | Modify | Loaded flags, error display, subscription days, instance nav, costTotal, quota filter |
| `frontend/src/views/Instances.vue` | Modify | Support `?instanceId=` query param for auto-selection |

---

### Task 1: SQL — Add instance_count subquery

**Files:**
- Modify: `internal/repo/queries/tenant.sql:45-64`
- Regenerate: `internal/repo/tenant_extra.sql.go`
- Test: `internal/repo/tenant_extra_test.go` (existing parity test)

**Interfaces:**
- Produces: `ListTenantsWithCountsRow.InstanceCount int64` (new field)

- [ ] **Step 1: Update the SQL query**

Edit `internal/repo/queries/tenant.sql`, in the `ListTenantsWithCounts` query (line 55), add the instance_count subquery after `t.is_active,` and before the `register_time` subquery:

```sql
-- name: ListTenantsWithCounts :many
-- (existing comment block unchanged)
SELECT
    t.id, t.tenant_id, t.user_name, t.fingerprint, t.tenancy, t.region, t.created_at,
    t.api_synced, t.enable_icmp, t.enable_all_protocol, t.is_home_region, t.paren_id,
    t.tenancy_name, t.tenancy_des, t.account_type, t.cloud_type, t.region_en, t.id_str,
    t.email_address, t.email_enable, t.transfer_status, t.transfer_amount, t.is_active,
    (SELECT COUNT(*) FROM instance_detail WHERE tenant_id = t.id) AS instance_count,
    (SELECT rd.register_time FROM register_detail rd WHERE rd.tenant_id = t.tenant_id LIMIT 1) AS register_time,
    (SELECT COUNT(*) FROM boot_instance b WHERE b.tenant_id = t.id AND b.status = 1) AS boot_count,
    (SELECT COUNT(*) FROM tenant c WHERE c.paren_id = t.id) AS child_count
FROM tenant t
ORDER BY t.id;
```

- [ ] **Step 2: Regenerate sqlc code**

Run:
```bash
cd /home/ubuntu/workspace-oci-start-rewrite/oci-start-go && sqlc generate
```

Expected: `internal/repo/tenant_extra.sql.go` now has `InstanceCount int64` in `ListTenantsWithCountsRow` and scans it in `ListTenantsWithCounts`.

- [ ] **Step 3: Verify generated struct**

Run:
```bash
grep -A 5 "BootCount" internal/repo/tenant_extra.sql.go | head -6
```

Expected output should include `InstanceCount int64` field between `IsActive` and `RegisterTime`.

- [ ] **Step 4: Run existing parity test**

Run:
```bash
go test ./internal/repo/... -run TestListTenantsWithCounts -v
```

Expected: PASS (the parity test checks structural correctness, not field count).

- [ ] **Step 5: Vet**

Run:
```bash
go vet ./...
```

Expected: no output (clean).

- [ ] **Step 6: Commit**

```bash
git add internal/repo/queries/tenant.sql internal/repo/tenant_extra.sql.go
git commit -m "feat(repo): add instance_count subquery to ListTenantsWithCounts"
```

---

### Task 2: Backend — Add InstanceCount and PlanType to TenantResp

**Files:**
- Modify: `internal/service/tenant.go:35-53` (TenantResp struct)
- Modify: `internal/service/tenant.go:393-421` (toTenantRespFromCounts function)

**Interfaces:**
- Consumes: `ListTenantsWithCountsRow.InstanceCount int64` (from Task 1)
- Produces: `TenantResp.InstanceCount int64`, `TenantResp.PlanType string`

- [ ] **Step 1: Add fields to TenantResp**

In `internal/service/tenant.go`, add two fields to the `TenantResp` struct (after `HasChildren`):

```go
type TenantResp struct {
	ID           int64  `json:"id"`
	TenantID     string `json:"tenantId"`
	UserName     string `json:"userName"`
	Fingerprint  string `json:"fingerprint"`
	Tenancy      string `json:"tenancy"`
	Region       string `json:"region"`
	RegionName   string `json:"regionName"`
	CreatedAt    string `json:"createdAt"`
	ApiSynced    bool   `json:"apiSynced"`
	CloudType    int64  `json:"cloudType"`
	IsActive     bool   `json:"isActive"`
	IsHomeRegion bool   `json:"isHomeRegion"`
	AccountType  string `json:"accountType"`
	TenancyName  string `json:"tenancyName"`
	ActiveDays   string `json:"activeDays"`
	HasBootTask  bool   `json:"hasBootTask"`
	HasChildren  bool   `json:"hasChildren"`
	InstanceCount int64 `json:"instanceCount"`
	PlanType     string `json:"planType"`
}
```

- [ ] **Step 2: Update toTenantRespFromCounts mapping**

In `toTenantRespFromCounts`, add `InstanceCount` and `PlanType` to the return:

```go
func toTenantRespFromCounts(r repo.ListTenantsWithCountsRow) TenantResp {
	createdAt := ns(r.CreatedAt)
	activeDaysInput := ns(r.RegisterTime)
	if activeDaysInput == "" {
		activeDaysInput = createdAt
	}
	accountType := ns(r.AccountType)
	return TenantResp{
		ID:            r.ID,
		TenantID:      ns(r.TenantID),
		UserName:      ns(r.UserName),
		Fingerprint:   ns(r.Fingerprint),
		Tenancy:       ns(r.Tenancy),
		Region:        ns(r.Region),
		RegionName:    region.NameByCode(region.CodeByName(ns(r.Region))),
		CreatedAt:     createdAt,
		ApiSynced:     ni(r.ApiSynced) == 1,
		CloudType:     ni(r.CloudType),
		IsActive:      isActive(r.IsActive),
		IsHomeRegion:  ni(r.IsHomeRegion) != 0,
		AccountType:   accountType,
		TenancyName:   ns(r.TenancyName),
		ActiveDays:    calculateActiveDays(activeDaysInput),
		HasBootTask:   r.BootCount > 0,
		HasChildren:   r.ChildCount > 0,
		InstanceCount: r.InstanceCount,
		PlanType:      accountType,
	}
}
```

Note: `PlanType` uses the same value as `AccountType` — when `UpdateAccountDetail` runs, it stores `FREE_TIER`/`PAYG` in the `account_type` column.

- [ ] **Step 3: Build check**

Run:
```bash
go build ./cmd/oci-start
```

Expected: no errors.

- [ ] **Step 4: Run tests**

Run:
```bash
go test ./internal/service/... -v -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/tenant.go
git commit -m "feat(service): add instanceCount and planType to TenantResp"
```

---

### Task 3: Frontend TenantList — Eliminate N+1 and add instance count column

**Files:**
- Modify: `frontend/src/views/TenantList.vue:174-186` (Tenant interface)
- Modify: `frontend/src/views/TenantList.vue:289-302` (load function)
- Modify: `frontend/src/views/TenantList.vue:45-55` (table columns area)

**Interfaces:**
- Consumes: `Tenant.instanceCount`, `Tenant.planType` (from Task 2 backend)

- [ ] **Step 1: Update Tenant interface**

In `TenantList.vue` script section, update the `Tenant` interface to add `instanceCount` and `planType`:

```ts
interface Tenant {
  id: number; tenantId?: string; userName: string; tenancy: string; region: string
  regionName: string; fingerprint: string; apiSynced: boolean
  tenancyName?: string; tenancyDes?: string; accountType?: string
  cloudType?: number; emailAddress?: string; emailEnable?: boolean
  isActive?: boolean; isHomeRegion?: boolean; createdAt?: string
  enableIcmp?: boolean; enableAllProtocol?: boolean
  parenId?: number; regionEn?: string; idStr?: string
  transferStatus?: number; transferAmount?: string
  instanceCount?: number; planType?: string
  hasBootTask?: boolean; hasChildren?: boolean; activeDays?: string
}
```

- [ ] **Step 2: Replace load() to eliminate N+1**

Replace the `load()` function (lines 289-302) with:

```ts
async function load() {
  loading.value = true
  try {
    rows.value = await request.get('/tenants/listAll') as Tenant[]
  } catch (e: any) { ElMessage.error(e.message) }
  finally { loading.value = false }
}
```

This removes the `Promise.allSettled` block that made N additional API calls.

- [ ] **Step 3: Add instance count column to table**

In the template, add a new column after the "存活天数" column (after line 47):

```html
<el-table-column label="实例数" width="80" align="center">
  <template #default="{ row }">{{ row.instanceCount ?? 0 }}</template>
</el-table-column>
```

- [ ] **Step 4: Manual verification**

Build frontend and verify in browser:
```bash
cd frontend && npm run build
```

Open tenant list page — should load faster, show instance counts. No errors in console.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/TenantList.vue
git commit -m "fix(frontend): eliminate N+1 query in tenant list, add instance count column"
```

---

### Task 4: Frontend TenantList — Fix account type display

**Files:**
- Modify: `frontend/src/views/TenantList.vue:64-68` (account type column)

**Interfaces:**
- Consumes: `Tenant.planType` (from Task 3)

- [ ] **Step 1: Update account type column template**

In the template, find the "账号类型" column (around line 64-68) and change the tag to prefer `planType`:

```html
<el-table-column label="账号类型" width="110">
  <template #default="{ row }">
    <el-tag size="small" :type="accountTypeTag(row.planType || row.accountType)">{{ accountTypeLabel(row.planType || row.accountType) }}</el-tag>
  </template>
</el-table-column>
```

- [ ] **Step 2: Manual verification**

Build and verify: accounts with `FREE_TIER` should show "免费层" (warning tag), `PAYG` should show "按量付费" (success tag).

```bash
cd frontend && npm run build
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/TenantList.vue
git commit -m "fix(frontend): show planType for account type in tenant list"
```

---

### Task 5: Frontend TenantList — Fix batch check to use batch endpoint

**Files:**
- Modify: `frontend/src/views/TenantList.vue:401-414` (startBatchCheck function)

**Interfaces:**
- Consumes: `POST /tenants/check-batch` (existing backend endpoint)

- [ ] **Step 1: Replace startBatchCheck function**

Replace the `startBatchCheck()` function with:

```ts
async function startBatchCheck() {
  batchCheckVisible.value = true; batchChecking.value = true; batchProgress.value = 0; batchResults.value = []
  try {
    const ids = rows.value.map(r => r.id)
    batchProgress.value = 10
    const results = await request.post('/tenants/check-batch', ids) as any[]
    batchProgress.value = 100
    batchResults.value = results
  } catch (e: any) { ElMessage.error(e.message) }
  finally { batchChecking.value = false }
}
```

- [ ] **Step 2: Manual verification**

Build and verify: batch check should complete in one request (check Network tab in DevTools).

```bash
cd frontend && npm run build
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/TenantList.vue
git commit -m "fix(frontend): use batch endpoint for tenant check"
```

---

### Task 6: Frontend TenantDetail — Add loaded flags to prevent empty-list re-fetch

**Files:**
- Modify: `frontend/src/views/TenantDetail.vue:436-556` (state declarations + onTabChange + load functions)

**Interfaces:**
- Internal: adds `loaded` refs, no API changes

- [ ] **Step 1: Add loaded ref declarations**

After the existing state declarations (around line 530, before `onMounted`), add:

```ts
// loaded flags — prevent re-fetch when list is genuinely empty
const instLoaded = ref(false)
const userLoaded = ref(false)
const socialLoaded = ref(false)
const costLoaded = ref(false)
const domainsLoaded = ref(false)
const settingsLoaded = ref(false)
```

- [ ] **Step 2: Update onTabChange to use loaded flags**

Replace the `onTabChange` function with:

```ts
function onTabChange(tab: string | number) {
  const t = String(tab)
  if (t === 'instances' && !instLoaded.value) loadInstances()
  if (t === 'costs' && !costLoaded.value) { loadSubscription(); loadCost(); loadSubscriptionDays() }
  if (t === 'users' && !userLoaded.value) { loadUsers(); loadGroups(); loadPasswordPolicy() }
  if (t === 'social' && !socialLoaded.value) loadSocial()
  if (t === 'settings' && !settingsLoaded.value) { loadMfaStatus(); loadNotifRecipients(); loadQuota(); loadQuotaServices() }
  if (t === 'overview' && !domainsLoaded.value) loadDomains()
  if (t === 'regions') loadRegions()
}
```

- [ ] **Step 3: Set loaded flags in each load function**

Update `loadInstances`:
```ts
async function loadInstances() {
  instLoading.value = true
  try { instances.value = await request.get(`/tenants/${tenantId}/instances`) as any[] }
  catch (e: any) { ElMessage.error(e.message) }
  finally { instLoading.value = false; instLoaded.value = true }
}
```

Update `loadUsers`:
```ts
async function loadUsers() {
  userLoading.value = true
  try { userList.value = await request.get(`/tenants/${tenantId}/users`) as any[] }
  catch { userList.value = [] }
  finally { userLoading.value = false; userLoaded.value = true }
}
```

Update `loadSocial`:
```ts
async function loadSocial() {
  socialLoading.value = true
  try { socialList.value = await request.get(`/tenants/${tenantId}/social`) as any[] }
  catch { socialList.value = [] }
  finally { socialLoading.value = false; socialLoaded.value = true }
}
```

Update `loadDomains`:
```ts
async function loadDomains() {
  domainsLoading.value = true
  try { domains.value = await request.get(`/tenants/${tenantId}/domains`) as any[] }
  catch { domains.value = [] }
  finally { domainsLoading.value = false; domainsLoaded.value = true }
}
```

Update `loadCost` — add `costLoaded.value = true` in the finally block:
```ts
  finally { costLoading.value = false; costLoaded.value = true }
```

Update `loadMfaStatus`, `loadNotifRecipients`, `loadQuota`, `loadQuotaServices` — set `settingsLoaded.value = true` after all settings loads complete. The simplest approach: set it in `onTabChange` after dispatching all loads:

```ts
  if (t === 'settings' && !settingsLoaded.value) {
    loadMfaStatus(); loadNotifRecipients(); loadQuota(); loadQuotaServices()
    settingsLoaded.value = true
  }
```

- [ ] **Step 4: Manual verification**

Build and verify: switching tabs with empty data should NOT trigger repeated API calls (check Network tab).

```bash
cd frontend && npm run build
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/TenantDetail.vue
git commit -m "fix(frontend): use loaded flags to prevent empty-list re-fetch in tenant detail"
```

---

### Task 7: Frontend TenantDetail — Surface OCI API errors with retry

**Files:**
- Modify: `frontend/src/views/TenantDetail.vue` (MFA/notification/audit/quota sections)

**Interfaces:**
- Internal: adds error refs and el-alert components

- [ ] **Step 1: Add error ref declarations**

After the existing state declarations, add:

```ts
const mfaError = ref('')
const notifError = ref('')
const auditError = ref('')
const quotaError = ref('')
```

- [ ] **Step 2: Update loadMfaStatus with error handling**

```ts
async function loadMfaStatus() {
  mfaError.value = ''
  try { mfaStatus.value = await request.get(`/tenants/${tenantId}/mfa/status`) }
  catch (e: any) { mfaError.value = e.message || '加载失败' }
}
```

- [ ] **Step 3: Update loadNotifRecipients with error handling**

```ts
async function loadNotifRecipients() {
  notifError.value = ''
  try { notifRecipients.value = await request.get(`/tenants/${tenantId}/notification-recipients`) as any[] }
  catch (e: any) { notifError.value = e.message || '加载失败'; notifRecipients.value = [] }
}
```

- [ ] **Step 4: Update loadAudit with error handling**

```ts
async function loadAudit(days: number) {
  auditDays.value = days; auditDateRange.value = null; auditLoading.value = true; auditError.value = ''
  try {
    const r: any = await request.post(`/tenants/${tenantId}/audit-log`, { days })
    auditLogs.value = r?.data || []
  } catch (e: any) { auditError.value = e.message || '加载失败'; auditLogs.value = [] }
  finally { auditLoading.value = false }
}
```

Also update `loadAuditCustom` similarly:
```ts
async function loadAuditCustom() {
  if (!auditDateRange.value || auditDateRange.value.length !== 2) { ElMessage.warning('请选择日期范围'); return }
  auditDays.value = 0; auditLoading.value = true; auditError.value = ''
  try {
    const r: any = await request.post(`/tenants/${tenantId}/audit-log`, { startDate: auditDateRange.value[0], endDate: auditDateRange.value[1] })
    auditLogs.value = r?.data || []
  } catch (e: any) { auditError.value = e.message || '加载失败'; auditLogs.value = [] }
  finally { auditLoading.value = false }
}
```

- [ ] **Step 5: Update loadQuota with error handling**

```ts
async function loadQuota() {
  quotaLoading.value = true; quotaError.value = ''
  try {
    const r: any = await request.get(`/tenants/${tenantId}/quota`, {
      params: { serviceName: quotaServiceName.value, pageSize: 100 }
    })
    quotaItems.value = r?.items || []
  } catch (e: any) { quotaError.value = e.message || '加载失败'; quotaItems.value = [] }
  finally { quotaLoading.value = false }
}
```

- [ ] **Step 6: Add error alerts in the settings tab template**

In the settings tab template, add error alerts before each section:

Before MFA section (before `<h4>MFA 多因素认证</h4>`):
```html
<el-alert v-if="mfaError" type="warning" :closable="false" show-icon style="margin-bottom:8px">
  <template #title>MFA 状态加载失败: {{ mfaError }}</template>
  <template #default><el-button size="small" text type="primary" @click="loadMfaStatus">重试</el-button></template>
</el-alert>
```

Before notification recipients section (before `<h4>通知接收人</h4>`):
```html
<el-alert v-if="notifError" type="warning" :closable="false" show-icon style="margin-bottom:8px">
  <template #title>通知接收人加载失败: {{ notifError }}</template>
  <template #default><el-button size="small" text type="primary" @click="loadNotifRecipients">重试</el-button></template>
</el-alert>
```

Before audit log section (before `<h4>审计日志</h4>`):
```html
<el-alert v-if="auditError" type="warning" :closable="false" show-icon style="margin-bottom:8px">
  <template #title>审计日志加载失败: {{ auditError }}</template>
  <template #default><el-button size="small" text type="primary" @click="loadAudit(auditDays || 1)">重试</el-button></template>
</el-alert>
```

Before quota section (before `<h4>配额</h4>`):
```html
<el-alert v-if="quotaError" type="warning" :closable="false" show-icon style="margin-bottom:8px">
  <template #title>配额加载失败: {{ quotaError }}</template>
  <template #default><el-button size="small" text type="primary" @click="loadQuota">重试</el-button></template>
</el-alert>
```

- [ ] **Step 7: Manual verification**

Build and verify: when OCI APIs fail, error alerts with retry buttons should appear instead of silently showing empty/default state.

```bash
cd frontend && npm run build
```

- [ ] **Step 8: Commit**

```bash
git add frontend/src/views/TenantDetail.vue
git commit -m "fix(frontend): surface OCI API errors with retry in tenant detail settings"
```

---

### Task 8: Frontend TenantDetail — Fix subscription days calculation

**Files:**
- Modify: `frontend/src/views/TenantDetail.vue:623-626` (loadSubscriptionDays function)

**Interfaces:**
- Consumes: `subscriptionData.value.timeStart` (from loadSubscription), `tenant.value.createdAt`

- [ ] **Step 1: Replace loadSubscriptionDays function**

```ts
async function loadSubscriptionDays() {
  try {
    // Prefer subscription timeStart (most accurate)
    if (subscriptionData.value?.timeStart) {
      const start = new Date(subscriptionData.value.timeStart)
      const days = Math.ceil((Date.now() - start.getTime()) / (1000 * 60 * 60 * 24))
      subscriptionDays.value = String(days > 0 ? days : 0)
      return
    }
    // Fallback: OCI API
    const r: any = await request.get(`/tenants/${tenantId}/subscription-days`)
    subscriptionDays.value = r?.activeDays ?? '—'
  } catch {
    // Final fallback: tenant createdAt
    if (tenant.value.createdAt) {
      const start = new Date(tenant.value.createdAt)
      const days = Math.ceil((Date.now() - start.getTime()) / (1000 * 60 * 60 * 24))
      subscriptionDays.value = String(days > 0 ? days : 0)
    } else {
      subscriptionDays.value = '—'
    }
  }
}
```

Note: `loadSubscriptionDays` is called after `loadSubscription` in `onTabChange`, so `subscriptionData.value` will be populated by the time this runs (both are async but `loadSubscription` is called first). To ensure ordering, update the costs tab trigger:

```ts
if (t === 'costs' && !costLoaded.value) {
  loadSubscription().then(() => { loadCost(); loadSubscriptionDays() })
}
```

- [ ] **Step 2: Manual verification**

Build and verify: subscription days should show a number (not `—`) even when OCI API fails, as long as subscription data or createdAt is available.

```bash
cd frontend && npm run build
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/TenantDetail.vue
git commit -m "fix(frontend): fix subscription days with multi-level fallback"
```

---

### Task 9: Frontend TenantDetail — Fix instance click navigation

**Files:**
- Modify: `frontend/src/views/TenantDetail.vue:67-70` (instance name column template)

**Interfaces:**
- Produces: navigation to `/instances?instanceId=<ocid>`

- [ ] **Step 1: Update instance name link**

In the instances tab template, change the instance name link (around line 67-70):

```html
<el-table-column prop="displayName" label="名称" min-width="160">
  <template #default="{ row }">
    <el-link type="primary" @click="router.push({ path: '/instances', query: { instanceId: row.instanceId } })">{{ row.displayName }}</el-link>
  </template>
</el-table-column>
```

Changed from `router.push(`/instances/${row.id}`)` (broken — no such route) to query-param navigation.

- [ ] **Step 2: Manual verification**

Build and verify: clicking an instance name should navigate to `/instances?instanceId=ocid1...`.

```bash
cd frontend && npm run build
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/TenantDetail.vue
git commit -m "fix(frontend): fix instance click navigation to use query param"
```

---

### Task 10: Frontend TenantDetail — Convert costTotal to computed

**Files:**
- Modify: `frontend/src/views/TenantDetail.vue:467-468` (costTotal declaration + loadCost assignment)

**Interfaces:**
- Internal: replaces ref with computed

- [ ] **Step 1: Replace costTotal ref with computed**

Change the declaration from:
```ts
const costTotal = ref(0)
```

To:
```ts
const costTotal = computed(() => costData.value.reduce((s, i) => s + (Number(i.computedAmount) || 0), 0))
```

Add `computed` to the Vue import if not already there (check line 419).

- [ ] **Step 2: Remove manual costTotal assignment in loadCost**

In the `loadCost` function, remove the line `costTotal.value = ...`:

Change:
```ts
costData.value = resp || []; costTotal.value = (resp || []).reduce((s, i) => s + (Number(i.computedAmount) || 0), 0)
```

To:
```ts
costData.value = resp || []
```

Also remove `costTotal.value = 0` from the initialization at the top of `loadCost`.

- [ ] **Step 3: Manual verification**

Build and verify: cost total should still display correctly in the costs tab.

```bash
cd frontend && npm run build
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/views/TenantDetail.vue
git commit -m "refactor(frontend): convert costTotal to computed property"
```

---

### Task 11: Frontend TenantDetail — Filter quota services to those with data

**Files:**
- Modify: `frontend/src/views/TenantDetail.vue:846-850` (loadQuotaServices function)
- Modify: `frontend/src/views/TenantDetail.vue:366-377` (quota section template)

**Interfaces:**
- Consumes: `GET /tenants/:id/quota?serviceName=&pageSize=1` (existing endpoint)

- [ ] **Step 1: Replace loadQuotaServices function**

```ts
async function loadQuotaServices() {
  quotaLoading.value = true
  try {
    const allServices = await request.get(`/tenants/${tenantId}/quota/services`) as any[]
    // Check each service for actual quota data (pageSize=1 for minimal cost)
    const checks = await Promise.allSettled(
      allServices.map(s =>
        request.get(`/tenants/${tenantId}/quota`, {
          params: { serviceName: s.name, pageSize: 1 }
        })
      )
    )
    // Keep only services that have quota data
    quotaServices.value = allServices.filter((_, i) => {
      const r = checks[i]
      return r.status === 'fulfilled' && (r.value as any)?.items?.length > 0
    })
    // Auto-select first available service if current selection is gone
    if (quotaServices.value.length && !quotaServices.value.find(s => s.name === quotaServiceName.value)) {
      quotaServiceName.value = quotaServices.value[0].name
    }
  } catch { quotaServices.value = [] }
  finally { quotaLoading.value = false }
}
```

- [ ] **Step 2: Add empty state to quota template**

After the quota table, add an empty state for when no services have data:

```html
<el-empty v-if="!quotaServices.length && !quotaLoading" description="该租户无任何配额数据" :image-size="40"/>
```

- [ ] **Step 3: Manual verification**

Build and verify: quota dropdown should only show services that have data for the current tenant. Free tier accounts should see fewer options.

```bash
cd frontend && npm run build
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/views/TenantDetail.vue
git commit -m "feat(frontend): filter quota services to those with data"
```

---

### Task 12: Frontend Instances — Support instanceId query param auto-selection

**Files:**
- Modify: `frontend/src/views/Instances.vue:536-543` (onMounted area)

**Interfaces:**
- Consumes: `route.query.instanceId` (from TenantDetail navigation in Task 9)
- Uses: `detail` ref, `detailVisible` ref, `showDetail()` function (existing)

- [ ] **Step 1: Add route import and auto-selection logic**

Ensure `useRoute` is imported (check if it already is). If not, add it:
```ts
import { useRoute, useRouter } from 'vue-router'
const route = useRoute()
```

In the `onMounted` callback (or after the existing data load), add auto-selection:

```ts
onMounted(async () => {
  // existing load logic...
  await loadInstances()  // or whatever the existing call is

  // Auto-select instance from query param (navigated from TenantDetail)
  const targetId = route.query.instanceId as string
  if (targetId) {
    const inst = instances.value.find(i => i.instanceId === targetId)
    if (inst) {
      showDetail(inst)
    }
  }
})
```

Note: `showDetail(row)` sets `detail.value`, `detailTab.value = 'info'`, and `detailVisible.value = true` — this is the existing function at line 686.

- [ ] **Step 2: Manual verification**

Build and verify: navigating to `/instances?instanceId=ocid1.instance...` should open the instance list with that instance's detail panel open.

```bash
cd frontend && npm run build
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/Instances.vue
git commit -m "feat(frontend): support instanceId query param for auto-selection in Instances"
```

---

## Final Verification

- [ ] **Step 1: Full backend test**

```bash
go test ./... -count=1
```

- [ ] **Step 2: Full frontend build**

```bash
cd frontend && npm run build
```

- [ ] **Step 3: End-to-end manual test**

Verify all fixes in browser:
1. Tenant list loads fast (no N+1), shows instance counts and correct account types
2. Batch check completes in one request
3. Tenant detail tabs don't re-fetch on empty data
4. Settings tab shows error alerts with retry when OCI APIs fail
5. Subscription days shows a number (not `—`)
6. Instance names are clickable and navigate to Instances page
7. Cost total displays correctly
8. Quota dropdown only shows services with data
9. Navigating from TenantDetail instance click auto-selects the instance

- [ ] **Step 4: Final commit (if any touch-ups needed)**

```bash
git add -A && git commit -m "chore: tenant management improvements — final touch-ups"
```
