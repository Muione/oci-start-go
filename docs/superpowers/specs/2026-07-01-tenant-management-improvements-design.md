# 租户管理页面完善 — 设计文档

**日期**: 2026-07-01
**状态**: 已批准
**范围**: 后端 + 前端全栈修复与增强

---

## 背景

租户管理页面（TenantList.vue + TenantDetail.vue）存在多个 bug 和性能问题，包括：账号类型显示错误、存活天数计算不准确、设置页 MFA/通知/审计/配额全部无法正常加载、列表页 N+1 查询导致加载缓慢、批量检测串行执行、空数据重复请求等。本文档定义修复方案和增强措施。

## 问题清单

### Bug

| ID | 问题 | 根因 |
|----|------|------|
| B1 | 列表页账号类型显示错误 | `TenantResp.AccountType` 存的是 DB 旧值，保存时未设值，只有手动"从OCI获取"才更新 |
| B2 | 存活天数不对 / 订阅天数显示横杠 | 列表用 `registerTime`（需手动更新才有）；详情页调 OCI API 失败后静默返回 `—` |
| B3 | 设置页 MFA 全部显示"未启用" | `loadMfaStatus()` OCI API 失败，`catch` 为空吞掉错误 |
| B4 | 通知接收人不显示 | 同上，OCI API 失败被静默吞掉 |
| B5 | 审计日志空白 | 同上 |
| B6 | 配额下拉框显示无配额的服务 | 免费账户很多服务无配额，选中后表格为空 |
| B7 | 实例名点击跳转失败 | 路由 `/instances/:id` 不存在 |

### 性能问题

| ID | 问题 | 根因 |
|----|------|------|
| P1 | 列表加载慢（N+1） | 前端对每个租户发 `/tenants/:id/instances` 请求计数 |
| P2 | 空数据重复请求 | tab 用 `!list.length` 判断是否已加载，空数组每次重新请求 |
| P3 | 批量检测串行 | 前端逐个 `await`，后端已有 `/tenants/check-batch` 未使用 |

---

## 设计方案

### 1. 后端改动

#### 1.1 `TenantResp` 增加 `instanceCount` 和 `planType`

**文件**: `internal/service/tenant.go`

`TenantResp` 新增两个字段：

```go
type TenantResp struct {
    // ... 现有字段 ...
    InstanceCount int64  `json:"instanceCount"`
    PlanType      string `json:"planType"`
}
```

#### 1.2 SQL 查询增加实例子查询

**文件**: `internal/repo/queries/tenant.sql`

修改 `ListTenantsWithCounts`，增加实例子查询：

```sql
SELECT t.*,
  (SELECT COUNT(*) FROM instance_detail WHERE tenant_id = t.id) AS instance_count,
  rd.register_time,
  (SELECT COUNT(*) FROM boot_instance WHERE tenant_id = t.id AND status = 'running') AS boot_count,
  (SELECT COUNT(*) FROM tenant WHERE paren_id = t.id) AS child_count
FROM tenant t
LEFT JOIN register_detail rd ON rd.tenant_id = t.tenant_id
ORDER BY t.id
```

sqlc 重生成后，`ListTenantsWithCountsRow` 会包含 `InstanceCount` 字段。

#### 1.3 `toTenantRespFromCounts` 映射更新

**文件**: `internal/service/tenant.go`

```go
func toTenantRespFromCounts(r repo.ListTenantsWithCountsRow) TenantResp {
    // ... 现有逻辑 ...
    return TenantResp{
        // ... 现有字段 ...
        InstanceCount: r.InstanceCount,
        PlanType:      ns(r.AccountType),  // accountType 已被 UpdateAccountDetail 更新为 planType
    }
}
```

#### 1.4 订阅天数 API 优先读本地

**文件**: `internal/service/tenant_user.go` — `GetSubscriptionDays` 不变（仍调 OCI API），但前端增加 fallback 逻辑（见 3.3）。

---

### 2. 前端 TenantList.vue 修复

#### 2.1 消除 N+1 查询

删除 `load()` 中的 `Promise.allSettled` 代码，直接使用后端返回的 `instanceCount`：

```ts
async function load() {
  loading.value = true
  try {
    rows.value = await request.get('/tenants/listAll') as Tenant[]
  } catch (e: any) { ElMessage.error(e.message) }
  finally { loading.value = false }
}
```

#### 2.2 账号类型显示修正

表格改用 `planType` 字段（后端从 subscription 信息获取）：

```html
<el-tag :type="accountTypeTag(row.planType || row.accountType)">{{ accountTypeLabel(row.planType || row.accountType) }}</el-tag>
```

`accountTypeLabel` 和 `accountTypeTag` 已支持 `FREE_TIER`/`PAYG`，无需修改。

#### 2.3 批量检测改用批量端点

```ts
async function startBatchCheck() {
  batchCheckVisible.value = true; batchChecking.value = true; batchProgress.value = 0; batchResults.value = []
  try {
    const ids = rows.value.map(r => r.id)
    const results = await request.post('/tenants/check-batch', ids) as any[]
    batchProgress.value = 100
    batchResults.value = results
  } catch (e: any) { ElMessage.error(e.message) }
  finally { batchChecking.value = false }
}
```

#### 2.4 表格增加实例数列

```html
<el-table-column label="实例数" width="80" align="center">
  <template #default="{ row }">{{ row.instanceCount ?? 0 }}</template>
</el-table-column>
```

#### 2.5 Tenant 接口类型更新

```ts
interface Tenant {
  // ... 现有字段 ...
  instanceCount?: number
  planType?: string
}
```

---

### 3. 前端 TenantDetail.vue 修复

#### 3.1 修复空数据重复请求（loaded 标记）

每个 tab 增加 `loaded` ref，用 `loaded` 代替 `!list.length`：

```ts
const instLoaded = ref(false)
const userLoaded = ref(false)
const socialLoaded = ref(false)
const costLoaded = ref(false)
const domainsLoaded = ref(false)
const settingsLoaded = ref(false)

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

每个 load 函数在成功/失败后设 `xxxLoaded.value = true`。

#### 3.2 设置页 OCI API 错误展示

每个区块增加 `error` ref，失败时显示 `el-alert` + 重试按钮：

```ts
const mfaError = ref('')
const notifError = ref('')
const auditError = ref('')
const quotaError = ref('')

async function loadMfaStatus() {
  mfaError.value = ''
  try { mfaStatus.value = await request.get(`/tenants/${tenantId}/mfa/status`) }
  catch (e: any) { mfaError.value = e.message || '加载失败' }
}
```

模板中：
```html
<el-alert v-if="mfaError" type="warning" :title="'MFA 状态加载失败: ' + mfaError" show-icon style="margin-bottom:8px">
  <template #default><el-button size="small" text @click="loadMfaStatus">重试</el-button></template>
</el-alert>
```

审计日志、通知接收人、配额同理。

#### 3.3 费用 tab — 订阅天数修复

优先从 `subscription.timeStart` 计算，fallback 到 OCI API，最终 fallback 到 `createdAt`：

```ts
async function loadSubscriptionDays() {
  try {
    if (subscriptionData.value?.timeStart) {
      const start = new Date(subscriptionData.value.timeStart)
      const days = Math.ceil((Date.now() - start.getTime()) / (1000 * 60 * 60 * 24))
      subscriptionDays.value = String(days > 0 ? days : 0)
      return
    }
    const r: any = await request.get(`/tenants/${tenantId}/subscription-days`)
    subscriptionDays.value = r?.activeDays ?? '—'
  } catch {
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

#### 3.4 实例点击跳转

改为跳转实例列表页并带 `instanceId` 查询参数：

```html
<el-link type="primary" @click="router.push({ path: '/instances', query: { instanceId: row.instanceId } })">
  {{ row.displayName }}
</el-link>
```

#### 3.5 costTotal 改为 computed

```ts
const costTotal = computed(() => costData.value.reduce((s, i) => s + (Number(i.computedAmount) || 0), 0))
```

删除 `loadCost` 中的 `costTotal.value = ...` 赋值。

#### 3.6 配额服务下拉框 — 仅显示有配额的服务

加载服务列表后，对每个服务并发查询 `pageSize=1`，仅保留有数据的服务：

```ts
async function loadQuotaServices() {
  quotaLoading.value = true
  try {
    const allServices = await request.get(`/tenants/${tenantId}/quota/services`) as any[]
    const checks = await Promise.allSettled(
      allServices.map(s =>
        request.get(`/tenants/${tenantId}/quota`, {
          params: { serviceName: s.name, pageSize: 1 }
        })
      )
    )
    quotaServices.value = allServices.filter((_, i) => {
      const r = checks[i]
      return r.status === 'fulfilled' && (r.value as any)?.items?.length > 0
    })
    if (quotaServices.value.length && !quotaServices.value.find(s => s.name === quotaServiceName.value)) {
      quotaServiceName.value = quotaServices.value[0].name
    }
  } catch { quotaServices.value = [] }
  finally { quotaLoading.value = false }
}
```

无配额时显示提示：
```html
<el-empty v-if="!quotaServices.length && !quotaLoading" description="该租户无任何配额数据" :image-size="40"/>
```

---

### 4. Instances.vue 联动修改

支持从租户详情跳转并选中实例。实现时需先读 Instances.vue 确认实例选中/详情面板的实际变量名（下方 `selectedInstance`/`showDetail` 为示意）：

```ts
const route = useRoute()

onMounted(async () => {
  await loadInstances()
  const targetId = route.query.instanceId as string
  if (targetId) {
    const inst = instances.value.find(i => i.instanceId === targetId)
    if (inst) {
      selectedInstance.value = inst  // 实际变量名以 Instances.vue 为准
      showDetail.value = true        // 实际变量名以 Instances.vue 为准
    }
  }
})
```

---

## 改动文件清单

| 文件 | 改动类型 |
|------|---------|
| `internal/service/tenant.go` | `TenantResp` 加字段 + 映射更新 |
| `internal/repo/queries/tenant.sql` | SQL 增加实例子查询 |
| `frontend/src/views/TenantList.vue` | 消除 N+1、账号类型、批量检测、实例数列 |
| `frontend/src/views/TenantDetail.vue` | loaded 标记、错误展示、订阅天数、实例跳转、costTotal computed、配额过滤 |
| `frontend/src/views/Instances.vue` | 支持 `?instanceId=` 参数自动选中 |

## 不做的事

- 不重构整体架构
- 不引入新的状态管理（Vuex/Pinia）
- 不修改后端 OCI API 调用逻辑（只修前端展示层）
- 不增加离线缓存/数据持久化
