# Tenant Management Bug Fixes & Enhancements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 9 bugs and add 4 enhancements to the tenant management pages (list + detail).

**Architecture:** Backend changes are in the Go service/OCI layer (httpapi handlers, service methods, OCI SDK wrappers). Frontend changes are in the Vue3 tenant detail/list components. Each bug fix is self-contained and independently testable.

**Tech Stack:** Go 1.25, Gin HTTP, Vue 3 + Element Plus, OCI Go SDK v65

## Global Constraints

- Do not change existing public API signatures (exported functions/types/methods)
- Backward-compatible: data format changes need read-time migration
- SQLite-only, no new DB engine
- `data/master.key` is the system trust root — never log ciphertext/credentials
- Follow existing code patterns (repo queries → service → httpapi handler)
- All new Go code must pass `go vet ./...` and `go test ./...`

---

## File Structure

### Backend Files Modified
- `internal/httpapi/tenant_ext.go` — new `tenantAccountCostUpdate` handler (BUG-001)
- `internal/httpapi/handler_quota.go` — new `tenantQuotaServices` handler (ENH-010)
- `internal/httpapi/server.go` — register new routes (BUG-001, ENH-010)
- `internal/service/tenant.go` — new `UpdateCostByID` method (BUG-001)
- `internal/service/tenant_user.go` — add `pool` field, enhance `UpdateAccountDetail` (BUG-004, BUG-006)
- `internal/service/quota.go` — new `ListServices` method (ENH-010)
- `internal/oci/identity.go` — fix MFA error handling (BUG-008), fix notification state (BUG-007)
- `internal/oci/audit.go` — add error logging (BUG-009)
- `internal/oci/limits.go` — new `ServiceInfo` struct (ENH-010)
- `cmd/oci-start/main.go` — update `NewTenantUserService` call to pass `pool` (BUG-004)

### Frontend Files Modified
- `frontend/src/views/TenantDetail.vue` — fix saveAccountCost, saveSocial, createUser, subscription days, cost columns, quota dropdown, instance link, remove WeChat (BUG-001/002/003/005, ENH-010/011/012/013)
- `frontend/src/utils/tenant-utils.ts` — extend accountTypeLabel for OCI values (BUG-004)

---

### Task 1: BUG-001 — Account Cost Not Saving (Backend)

**Files:**
- Modify: `internal/service/tenant.go`
- Modify: `internal/httpapi/tenant_ext.go`
- Modify: `internal/httpapi/server.go`

**Interfaces:**
- Produces: `TenantService.UpdateCostByID(ctx context.Context, id int64, cost string) error`
- Produces: `POST /tenants/:id/account-cost` handler

- [ ] **Step 1: Add `UpdateCostByID` method to TenantService**

In `internal/service/tenant.go`, add after the `UpdateCost` method (around line 324):

```go
// UpdateCostByID updates the account cost for a tenant by its ID.
// Looks up the tenant's tenancy name, then updates cloud_tenancy.
func (s *TenantService) UpdateCostByID(ctx context.Context, id int64, cost string) error {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", id, err)
	}
	tenancyName := ns(t.Tenancy)
	if tenancyName == "" {
		return fmt.Errorf("tenant %d has no tenancy name", id)
	}
	return s.UpdateCost(ctx, tenancyName, cost)
}
```

- [ ] **Step 2: Add handler `tenantAccountCostUpdate` in tenant_ext.go**

In `internal/httpapi/tenant_ext.go`, add at the end of the file:

```go
// tenantAccountCostUpdate — POST /tenants/:id/account-cost
func tenantAccountCostUpdate(deps *Deps) gin.HandlerFunc {
	type costReq struct {
		Cost string `json:"cost"`
	}
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		var req costReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "请求数据无效: "+err.Error())
			return
		}
		if req.Cost == "" {
			response.Fail(c, http.StatusBadRequest, "成本不能为空")
			return
		}
		if err := deps.Tenant.UpdateCostByID(c.Request.Context(), id, req.Cost); err != nil {
			response.Fail(c, http.StatusInternalServerError, "更新成本失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}
```

- [ ] **Step 3: Register the route in server.go**

In `internal/httpapi/server.go`, add after the `pro.GET("/tenants/:id/export", ...)` line:

```go
pro.POST("/tenants/:id/account-cost", tenantAccountCostUpdate(deps))
```

- [ ] **Step 4: Run tests**

```bash
cd /home/ubuntu/workspace-oci-start-rewrite/oci-start-go && go vet ./... && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/service/tenant.go internal/httpapi/tenant_ext.go internal/httpapi/server.go
git commit -m "fix: add POST /tenants/:id/account-cost endpoint for account cost persistence"
```

---

### Task 2: BUG-001 — Account Cost Not Saving (Frontend)

**Files:**
- Modify: `frontend/src/views/TenantDetail.vue`

**Interfaces:**
- Consumes: `POST /tenants/:id/account-cost` with `{ cost: string }`

- [ ] **Step 1: Fix `saveAccountCost()` function**

In `frontend/src/views/TenantDetail.vue`, find the `saveAccountCost` function (around line 577) and replace it:

```typescript
async function saveAccountCost() {
  editSaving.value = true
  try {
    await request.post(`/tenants/${tenantId}/account-cost`, { cost: editCostValue.value })
    tenant.value.accountCost = editCostValue.value
    editCostVisible.value = false
    ElMessage.success('已更新')
  } catch (e: any) { ElMessage.error(e.message) }
  finally { editSaving.value = false }
}
```

- [ ] **Step 2: Verify the frontend builds**

```bash
cd /home/ubuntu/workspace-oci-start-rewrite/oci-start-go/frontend && npx vue-tsc --noEmit 2>&1 | head -20
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/TenantDetail.vue
git commit -m "fix: saveAccountCost calls dedicated POST endpoint instead of PUT /tenants/:id"
```

---

### Task 3: BUG-002 — Social Config Edit Fails (Frontend)

**Files:**
- Modify: `frontend/src/views/TenantDetail.vue`

- [ ] **Step 1: Fix `saveSocial()` function**

In `frontend/src/views/TenantDetail.vue`, find the `saveSocial` function (around line 715) and replace it:

```typescript
async function saveSocial() {
  socialEditSaving.value = true
  try {
    const payload = socialEditId.value
      ? { ...socialForm.value, id: socialEditId.value }
      : socialForm.value
    await request.post(`/tenants/${tenantId}/social`, payload)
    ElMessage.success('已保存')
    socialEditVisible.value = false
    await loadSocial()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { socialEditSaving.value = false }
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/views/TenantDetail.vue
git commit -m "fix: social config edit uses POST with id in body instead of PUT"
```

---

### Task 4: BUG-003 — Create User Form Field Mismatch (Frontend)

**Files:**
- Modify: `frontend/src/views/TenantDetail.vue`

- [ ] **Step 1: Fix `addUserForm` state declaration**

In `frontend/src/views/TenantDetail.vue`, find the `addUserForm` ref (around line 460) and change:

```typescript
const addUserForm = ref({ username: '', email: '', groupName: '' })
```

- [ ] **Step 2: Fix the create user dialog template**

Find the create user dialog (around line 162) and replace the form fields:

```html
<el-dialog v-model="addUserFormVisible" title="创建 IAM 用户" width="460px" append-to-body destroy-on-close>
  <el-form :model="addUserForm" label-width="80px">
    <el-form-item label="用户名" required><el-input v-model="addUserForm.username" placeholder="IAM 用户名"/></el-form-item>
    <el-form-item label="邮箱" required><el-input v-model="addUserForm.email" placeholder="用户邮箱"/></el-form-item>
    <el-form-item label="用户组">
      <el-select v-model="addUserForm.groupName" placeholder="选择用户组（可选）" clearable>
        <el-option v-for="g in groups" :key="g.ocid" :label="g.name" :value="g.name"/>
      </el-select>
    </el-form-item>
  </el-form>
  <el-alert v-if="createdUserPwd" title="用户创建成功！请复制以下一次性密码" type="success" :closable="true" show-icon @close="createdUserPwd=''" style="margin-top:12px">
    <template #default><code style="user-select:all">{{ createdUserPwd }}</code></template>
  </el-alert>
  <template #footer>
    <el-button @click="addUserFormVisible=false">关闭</el-button>
    <el-button type="primary" :loading="addUserSaving" @click="createUser">创建</el-button>
  </template>
</el-dialog>
```

- [ ] **Step 3: Fix `createUser()` function**

Find the `createUser` function (around line 631) and fix the field names:

```typescript
async function createUser() {
  if (!addUserForm.value.username || !addUserForm.value.email) {
    ElMessage.warning('请填写用户名和邮箱')
    return
  }
  addUserSaving.value = true
  createdUserPwd.value = ''
  try {
    const r: any = await request.post(`/tenants/${tenantId}/users`, addUserForm.value)
    createdUserPwd.value = r?.password || ''
    ElMessage.success('用户已创建')
    await loadUsers()
    addUserForm.value = { username: '', email: '', groupName: '' }
  } catch (e: any) { ElMessage.error(e.message) }
  finally { addUserSaving.value = false }
}
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/views/TenantDetail.vue
git commit -m "fix: create user form uses correct field names (username, email, groupName)"
```

---

### Task 5: BUG-004 — Account Type Wrong on List Page

**Files:**
- Modify: `internal/service/tenant_user.go`
- Modify: `frontend/src/utils/tenant-utils.ts`

**Interfaces:**
- Consumes: `oci.GetSubscriptionInfo(ctx, clients, tenancyOCID) (*SubscriptionInfo, error)` from `internal/oci/osp_gateway.go`
- Consumes: `oci.WithProxy(ctx, pool, creds, masterKey, fn)` pattern

- [ ] **Step 1: Enhance `UpdateAccountDetail` to fetch subscription planType**

In `internal/service/tenant_user.go`, find `UpdateAccountDetail` (around line 202) and replace the method:

```go
func (s *TenantUserService) UpdateAccountDetail(ctx context.Context, tenantID int64) (*oci.TenancyDetail, error) {
	creds, err := s.getProvider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}
	detail, err := oci.GetTenancyDetail(ctx, prov, creds.Tenancy)
	if err != nil {
		return nil, err
	}

	// Also fetch subscription info to get the real planType (FREE_TIER/PAYG).
	accountType := detail.AccountType
	_ = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(clients oci.Clients) error {
		subInfo, subErr := oci.GetSubscriptionInfo(ctx, clients, creds.Tenancy)
		if subErr != nil {
			return nil // non-fatal: keep the accountType from GetTenancyDetail
		}
		if subInfo.PlanType != "" {
			accountType = subInfo.PlanType
		}
		return nil
	})

	// Update the local tenant record with fetched data
	err = repo.New(s.store.Write).UpdateTenantFields(ctx, repo.UpdateTenantFieldsParams{
		TenancyName:  nullStr(detail.TenancyName),
		AccountType:  nullStr(accountType),
		EmailAddress: nullStr(detail.EmailAddress),
		ID:           tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("update tenant fields: %w", err)
	}
	// Also update register_detail with current timestamp for active-days calculation.
	now := time.Now().Format("2006-01-02 15:04:05")
	_ = repo.New(s.store.Write).UpsertRegisterDetail(ctx, repo.UpsertRegisterDetailParams{
		TenantID:     creds.UserID,
		RegisterTime: nullStr(now),
		CreatedTime:  nullStr(now),
		UpdatedTime:  nullStr(now),
	})
	return detail, nil
}
```

Note: The `TenantUserService` needs access to the `ProxyPool` for `WithProxy`. Check if it already has a `pool` field. If not, add one.

- [ ] **Step 2: Add `pool` field to TenantUserService**

`TenantUserService` currently lacks a `pool` field. Add it and update the constructor.

In `internal/service/tenant_user.go`, update the struct and constructor:

```go
type TenantUserService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool
}

func NewTenantUserService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *TenantUserService {
	return &TenantUserService{store: store, masterKey: masterKey, pool: pool}
}
```

Update the call site in `cmd/oci-start/main.go` (line 324):

```go
tenantUserSvc := service.NewTenantUserService(store, masterKey, pool)
```

Note: `pool` is the `*oci.ProxyPool` already constructed earlier in `main.go`.

- [ ] **Step 3: Extend `accountTypeLabel` in tenant-utils.ts**

In `frontend/src/utils/tenant-utils.ts`, update `accountTypeLabel` and `accountTypeTag`:

```typescript
/** Tag type for the account type badge */
export function accountTypeTag(t: string | undefined): 'success' | 'warning' | 'info' | '' {
  if (!t) return 'info'
  const s = t.toLowerCase()
  if (s.includes('trial') || s.includes('试用') || s === 'free_tier') return 'warning'
  if (s.includes('paid') || s.includes('付费') || s === 'payg') return 'success'
  if (s.includes('enterprise') || s.includes('企业')) return ''
  if (s === 'free') return 'warning'
  return 'info'
}

/** Human-readable label for the account type */
export function accountTypeLabel(t: string | undefined): string {
  if (!t) return '—'
  const m: Record<string, string> = {
    trial: '免费试用', paid: '付费账户', enterprise: '企业账户', free: '免费账户',
    FREE_TIER: '免费层', PAYG: '按量付费', PERSONAL: '个人', CORPORATE: '企业',
  }
  return m[t] || t
}
```

- [ ] **Step 4: Run Go tests**

```bash
cd /home/ubuntu/workspace-oci-start-rewrite/oci-start-go && go vet ./... && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/service/tenant_user.go frontend/src/utils/tenant-utils.ts
git commit -m "fix: sync accountType from OCI subscription planType; handle FREE_TIER/PAYG in frontend"
```

---

### Task 6: BUG-005 — Subscription Days Shows "—" (Frontend)

**Files:**
- Modify: `frontend/src/views/TenantDetail.vue`

- [ ] **Step 1: Fix `loadSubscriptionDays()` field name**

In `frontend/src/views/TenantDetail.vue`, find `loadSubscriptionDays` (around line 605) and fix:

```typescript
async function loadSubscriptionDays() {
  try {
    const r: any = await request.get(`/tenants/${tenantId}/subscription-days`)
    subscriptionDays.value = r?.activeDays ?? r?.days ?? r?.subscriptionDays ?? '—'
  } catch { subscriptionDays.value = '—' }
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/views/TenantDetail.vue
git commit -m "fix: subscription days reads activeDays from backend response"
```

---

### Task 7: BUG-006 — Active Days Uses Wrong Time Source

**Files:**
- Modify: `internal/service/tenant_user.go`

- [ ] **Step 1: Fix `UpdateAccountDetail` to use subscription timeStart**

In the `UpdateAccountDetail` method (already modified in Task 5), update the `UpsertRegisterDetail` call to use subscription `timeStart` instead of `time.Now()`. Replace the register_detail upsert block:

```go
	// Also update register_detail with subscription timeStart for active-days calculation.
	registerTime := time.Now().Format("2006-01-02 15:04:05") // fallback
	_ = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(clients oci.Clients) error {
		subInfo, subErr := oci.GetSubscriptionInfo(ctx, clients, creds.Tenancy)
		if subErr != nil || subInfo.TimeStart == nil {
			return nil // non-fatal: keep fallback
		}
		registerTime = subInfo.TimeStart.Time.Format("2006-01-02 15:04:05")
		return nil
	})
	_ = repo.New(s.store.Write).UpsertRegisterDetail(ctx, repo.UpsertRegisterDetailParams{
		TenantID:     creds.UserID,
		RegisterTime: nullStr(registerTime),
		CreatedTime:  nullStr(time.Now().Format("2006-01-02 15:04:05")),
		UpdatedTime:  nullStr(time.Now().Format("2006-01-02 15:04:05")),
	})
```

Note: This should be merged with the subscription fetch from Task 5 to avoid double API calls. The combined `UpdateAccountDetail` should fetch subscription info once and use both `PlanType` and `TimeStart`.

- [ ] **Step 2: Run Go tests**

```bash
cd /home/ubuntu/workspace-oci-start-rewrite/oci-start-go && go test ./internal/service/... -run TestCalculate -v
```

- [ ] **Step 3: Commit**

```bash
git add internal/service/tenant_user.go
git commit -m "fix: use OCI subscription timeStart for register_time in active days calculation"
```

---

### Task 8: BUG-007 — Notification Recipient State Hardcoded

**Files:**
- Modify: `internal/oci/identity.go`

- [ ] **Step 1: Change hardcoded state to "VERIFIED"**

In `internal/oci/identity.go`, find `GetNotificationRecipients` (around line 721). Change line 746 from `State: "active"` to `State: "VERIFIED"`:

```go
	out := make([]NotifRecipient, 0, len(settings.TestRecipients))
	for i, email := range settings.TestRecipients {
		out = append(out, NotifRecipient{
			ID:    i + 1,
			Email: email,
			State: "VERIFIED",
		})
	}
```

The SCIM NotificationSettings API does not expose per-recipient verification state. Since recipients in OCI are already verified when they appear in the settings, `"VERIFIED"` is the correct semantic value.

- [ ] **Step 2: Commit**

```bash
git add internal/oci/identity.go
git commit -m "fix: notification recipients show VERIFIED state instead of hardcoded active"
```

---

### Task 9: BUG-008 — MFA Status Silently Fails

**Files:**
- Modify: `internal/oci/identity.go`

- [ ] **Step 1: Fix error handling in `GetMfaStatus`**

In `internal/oci/identity.go`, find `GetMfaStatus` (around line 632). Replace the error-swallowing blocks:

```go
func GetMfaStatus(ctx context.Context, prov common.ConfigurationProvider, tenancyOCID string) (*MfaStatus, error) {
	domainURL, err := getDomainURL(ctx, prov, tenancyOCID)
	if err != nil {
		return nil, fmt.Errorf("get domain URL: %w", err)
	}

	resp, err := doIdDomainCall(ctx, prov, "GET", domainURL,
		"/admin/v1/AuthenticationFactorSettings/"+authFactorSettingsID, nil)
	if err != nil {
		return nil, fmt.Errorf("get auth factor settings: %w", err)
	}
	defer resp.Body.Close()

	var settings idDomainAuthFactorSetting
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		return nil, fmt.Errorf("decode auth factor settings: %w", err)
	}

	return &MfaStatus{
		TotpEnabled:              boolPtrVal(settings.TotpEnabled),
		EmailEnabled:             boolPtrVal(settings.EmailEnabled),
		SmsEnabled:               boolPtrVal(settings.SmsEnabled),
		PushEnabled:              boolPtrVal(settings.PushEnabled),
		SecurityQuestionsEnabled: boolPtrVal(settings.SecurityQuestionsEnabled),
		FidoAuthenticatorEnabled: boolPtrVal(settings.FidoAuthenticatorEnabled),
		PhoneCallEnabled:         boolPtrVal(settings.PhoneCallEnabled),
	}, nil
}
```

- [ ] **Step 2: Run Go build**

```bash
cd /home/ubuntu/workspace-oci-start-rewrite/oci-start-go && go vet ./... && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/oci/identity.go
git commit -m "fix: MFA status propagates SCIM API errors instead of swallowing them"
```

---

### Task 10: BUG-009 — Audit Log Blank

**Files:**
- Modify: `internal/oci/audit.go`

- [ ] **Step 1: Add error logging to `ListAuditEvents`**

In `internal/oci/audit.go`, the function already returns errors properly. The issue is likely in the service layer or the compartment ID. Add a log line at the start of `ListAuditEvents`:

```go
func ListAuditEvents(
	ctx context.Context,
	c Clients,
	compartmentID string,
	startTime, endTime time.Time,
	pageToken string,
) (*AuditLogPage, error) {
	log.Debug().
		Str("compartmentID", compartmentID).
		Time("startTime", startTime).
		Time("endTime", endTime).
		Msg("querying audit events")

	req := audit.ListEventsRequest{
		// ... rest unchanged
```

- [ ] **Step 2: Add error logging to `ListAuditEventsByDateRange` for parse failures**

In `ListAuditEventsByDateRange`, the current code returns empty results on parse errors without logging. Add log lines:

```go
func ListAuditEventsByDateRange(
	ctx context.Context,
	c Clients,
	compartmentID string,
	startDate, endDate string,
	pageToken string,
) (*AuditLogPage, error) {
	start, err := time.Parse(dateLayout, startDate)
	if err != nil {
		log.Warn().Err(err).Str("startDate", startDate).Msg("invalid audit start date")
		return &AuditLogPage{Data: []AuditEventDTO{}}, nil
	}
	// ... similarly for endDate parse error
```

- [ ] **Step 3: Verify service layer passes correct compartment ID**

In `internal/service/audit.go`, verify that `creds.Tenancy` is used as the compartment ID (line 52). This is correct — tenancy OCID is the root compartment. No change needed.

- [ ] **Step 4: Commit**

```bash
git add internal/oci/audit.go
git commit -m "fix: add debug logging to audit log queries for diagnosing blank results"
```

---

### Task 11: ENH-010 — Quota Service Dropdown (Backend)

**Files:**
- Modify: `internal/service/quota.go`
- Modify: `internal/httpapi/handler_quota.go`
- Modify: `internal/httpapi/server.go`

**Interfaces:**
- Produces: `QuotaService.ListServices(ctx context.Context, tenantID int64) ([]limits.ServiceSummary, error)`
- Produces: `GET /tenants/:id/quota/services` handler

- [ ] **Step 1: Add `ListServices` method to QuotaService**

In `internal/service/quota.go`, add:

```go
// ListServices returns all available OCI limit services for a tenant.
func (s *QuotaService) ListServices(ctx context.Context, tenantID int64) ([]oci.ServiceInfo, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	creds := tenantToCreds(t)

	var result []oci.ServiceInfo
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(clients oci.Clients) error {
		services, qErr := oci.ListLimitServices(ctx, clients, creds.Tenancy)
		if qErr != nil {
			return qErr
		}
		result = make([]oci.ServiceInfo, 0, len(services))
		for _, svc := range services {
			if svc.Name != nil {
				desc := ""
				if svc.Description != nil {
					desc = *svc.Description
				}
				result = append(result, oci.ServiceInfo{
					Name:        *svc.Name,
					Description: desc,
				})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
```

Also add `ServiceInfo` struct to `internal/oci/limits.go` (at the end of the struct definitions):

```go
// ServiceInfo is a simplified service summary for the API.
type ServiceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
```

- [ ] **Step 2: Add handler `tenantQuotaServices`**

In `internal/httpapi/handler_quota.go`, add:

```go
// tenantQuotaServices — GET /tenants/:id/quota/services
func tenantQuotaServices(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		services, err := deps.Quota.ListServices(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "获取服务列表失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(services))
	}
}
```

- [ ] **Step 3: Register the route**

In `internal/httpapi/server.go`, add after the quota route:

```go
pro.GET("/tenants/:id/quota/services", tenantQuotaServices(deps))
```

- [ ] **Step 4: Run tests**

```bash
cd /home/ubuntu/workspace-oci-start-rewrite/oci-start-go && go vet ./... && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/oci/limits.go internal/service/quota.go internal/httpapi/handler_quota.go internal/httpapi/server.go
git commit -m "feat: add GET /tenants/:id/quota/services endpoint for quota service dropdown"
```

---

### Task 12: ENH-010 — Quota Service Dropdown (Frontend)

**Files:**
- Modify: `frontend/src/views/TenantDetail.vue`

- [ ] **Step 1: Add quota service state and loader**

In `frontend/src/views/TenantDetail.vue`, add after the existing quota state variables:

```typescript
const quotaServices = ref<any[]>([])
const quotaServiceName = ref('compute')
```

- [ ] **Step 2: Update `loadQuota` to use selected service**

Replace the existing `loadQuota` function:

```typescript
async function loadQuota() {
  quotaLoading.value = true
  try {
    const r: any = await request.get(`/tenants/${tenantId}/quota`, {
      params: { serviceName: quotaServiceName.value, pageSize: 100 }
    })
    quotaItems.value = r?.items || []
  } catch { quotaItems.value = [] }
  finally { quotaLoading.value = false }
}

async function loadQuotaServices() {
  try {
    quotaServices.value = await request.get(`/tenants/${tenantId}/quota/services`) as any[]
  } catch { quotaServices.value = [] }
}
```

- [ ] **Step 3: Update the quota section template**

Find the quota section in the settings tab (around line 356) and add a dropdown:

```html
<h4 style="margin:16px 0 8px">配额</h4>
<div style="margin-bottom:8px;display:flex;gap:8px;align-items:center">
  <el-select v-model="quotaServiceName" size="small" style="width:200px" @change="loadQuota">
    <el-option v-for="s in quotaServices" :key="s.name" :label="s.description || s.name" :value="s.name"/>
  </el-select>
</div>
```

- [ ] **Step 4: Load services when settings tab opens**

In the `onTabChange` function, update the settings tab case to also load services:

```typescript
if (t === 'settings' && !Object.keys(mfaStatus.value).some(k => mfaStatus.value[k])) {
  loadMfaStatus(); loadNotifRecipients(); loadQuota(); loadQuotaServices()
}
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/TenantDetail.vue
git commit -m "feat: quota section has service dropdown selector"
```

---

### Task 13: ENH-011 — Instance Name Clickable (Frontend)

**Files:**
- Modify: `frontend/src/views/TenantDetail.vue`

- [ ] **Step 1: Make displayName column clickable**

In `frontend/src/views/TenantDetail.vue`, find the instances table (around line 67) and update the displayName column:

```html
<el-table-column prop="displayName" label="名称" min-width="160">
  <template #default="{ row }">
    <el-link type="primary" @click="router.push(`/instances/${row.id}`)">{{ row.displayName }}</el-link>
  </template>
</el-table-column>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/views/TenantDetail.vue
git commit -m "feat: instance names in tenant detail are clickable links to instance page"
```

---

### Task 14: ENH-012 — Remove WeChat Login (Frontend)

**Files:**
- Modify: `frontend/src/views/TenantDetail.vue`

- [ ] **Step 1: Remove WEIXIN from socialTypes**

In `frontend/src/views/TenantDetail.vue`, find the `socialTypes` declaration (around line 477) and change:

```typescript
const socialTypes = ['GITHUB', 'GOOGLE']
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/views/TenantDetail.vue
git commit -m "feat: remove WeChat (WEIXIN) from social login options"
```

---

### Task 15: ENH-013 — Cost Table Columns Show Blank Data (Backend)

**Files:**
- Modify: `internal/oci/usage.go`

**Root cause:** The OCI Usage API returns aggregated data with empty service/SKU/region fields when `groupBy` is not specified. Monthly queries need `groupBy: ["service", "skuName", "region"]` to get per-service breakdown.

- [ ] **Step 1: Add groupBy to cost query functions**

In `internal/oci/usage.go`, update `QueryCurrentMonthCost` and `QueryLastMonthCost` to include `groupBy`:

```go
func QueryCurrentMonthCost(ctx context.Context, c Clients, tenancyOCID string) ([]CostSummary, error) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	groupBy := []string{"service", "skuName", "region"}
	return QueryCost(ctx, c, tenancyOCID, start, end, groupBy, usageapi.RequestSummarizedUsagesDetailsGranularityMonthly)
}

func QueryLastMonthCost(ctx context.Context, c Clients, tenancyOCID string) ([]CostSummary, error) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	groupBy := []string{"service", "skuName", "region"}
	return QueryCost(ctx, c, tenancyOCID, start, end, groupBy, usageapi.RequestSummarizedUsagesDetailsGranularityMonthly)
}
```

Also update daily queries (`QueryTodayCost`, `QueryYesterdayCost`, `QueryCustomCost`) similarly:

```go
func QueryTodayCost(ctx context.Context, c Clients, tenancyOCID string) ([]CostSummary, error) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	groupBy := []string{"service", "skuName", "region"}
	return QueryCost(ctx, c, tenancyOCID, start, end, groupBy, usageapi.RequestSummarizedUsagesDetailsGranularityDaily)
}

func QueryYesterdayCost(ctx context.Context, c Clients, tenancyOCID string) ([]CostSummary, error) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, time.UTC)
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	groupBy := []string{"service", "skuName", "region"}
	return QueryCost(ctx, c, tenancyOCID, start, end, groupBy, usageapi.RequestSummarizedUsagesDetailsGranularityDaily)
}

func QueryCustomCost(ctx context.Context, c Clients, tenancyOCID string, startStr, endStr string) ([]CostSummary, error) {
	const dateFmt = "2006-01-02"
	start, err := time.Parse(dateFmt, startStr)
	if err != nil {
		return nil, fmt.Errorf("invalid start date %q: %w", startStr, err)
	}
	end, err := time.Parse(dateFmt, endStr)
	if err != nil {
		return nil, fmt.Errorf("invalid end date %q: %w", endStr, err)
	}
	groupBy := []string{"service", "skuName", "region"}
	return QueryCost(ctx, c, tenancyOCID, start.UTC(), end.UTC(), groupBy, usageapi.RequestSummarizedUsagesDetailsGranularityDaily)
}
```

- [ ] **Step 2: Run Go build**

```bash
cd /home/ubuntu/workspace-oci-start-rewrite/oci-start-go && go vet ./... && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/oci/usage.go
git commit -m "fix: add groupBy service/skuName/region to cost queries so columns show data"
```

---

## Verification

After all tasks are complete:

```bash
cd /home/ubuntu/workspace-oci-start-rewrite/oci-start-go
go vet ./...
go test ./...
go build ./...
cd frontend && npx vue-tsc --noEmit
```

Manual verification checklist:
1. Tenant list: account type shows correct OCI plan type (FREE_TIER/PAYG)
2. Tenant list: active days matches subscription duration
3. Tenant detail → Overview: account cost edit persists after page refresh
4. Tenant detail → Costs: subscription days shows actual number
5. Tenant detail → Costs: cost table has currency, SKU, region columns
6. Tenant detail → Users: create user with username + email works
7. Tenant detail → Social: edit existing social config works
8. Tenant detail → Settings → MFA: shows actual MFA status
9. Tenant detail → Settings → Notification: recipients show VERIFIED state
10. Tenant detail → Settings → Quota: service dropdown lists all services
11. Tenant detail → Settings → Audit: shows audit events
12. Tenant detail → Instances: instance names are clickable links
13. Tenant detail → Social: no WeChat option available
