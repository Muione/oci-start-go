# Tenant Frontend Refactor — Design Spec

**Date:** 2026-07-02
**Status:** Draft
**Scope:** Refactor `frontend/src/views/Tenants.vue` (2571 lines, the largest file in the codebase) into a TenantList page + TenantDetail page with tab-based sub-features.

## 1. Problem

`Tenants.vue` is a single 2571-line file containing:
- A data table with 17+ action commands per row.
- 10+ inline dialog components (detail, add, edit name, edit cost, update, users, instances, security rules, quota, region subscriptions, audit log, traffic alerts, traffic queries, email config, social config, export).
- 30+ functions for data loading, CRUD, and state management.

This makes the file hard to maintain, navigate, and test. The user chose "方案 C" (detail page routing): extract the detail view as a dedicated route `/tenants/:id` with a tabbed interface.

## 2. Current State

### API Endpoints (already implemented in backend)
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/tenants/listAll` | GET | List all tenants |
| `/tenants/save` | POST | Create tenant |
| `/tenants/deleteApi` | GET | Delete tenant |
| `/tenants/syncOci` | GET | Sync from OCI |
| `/tenants/:id` | GET/PUT | Get/Update tenant |
| `/tenants/:id/instances` | GET | List instances |
| `/tenants/:id/check` | GET | Check status |
| `/tenants/:id/export` | GET | Export tenant |
| `/tenants/:id/email` | GET/POST/DELETE | Email config CRUD |
| `/tenants/:id/social` | GET/POST/DELETE | Social login CRUD |
| `/tenants/:id/users` | GET/POST/DELETE | IAM user management |
| `/tenants/:id/password-policy` | GET | Password policy |
| `/tenants/:id/notification-recipients` | GET | Notification recipients |
| `/tenants/:id/subscription-days` | GET | Subscription days |
| `/tenants/:id/quota` | GET | Quota |
| `/tenants/:id/regions/summary` | GET | Region summary |
| `/tenants/:id/regions/subscribed` | GET | Subscribed regions |
| `/tenants/:id/mfa/status` | GET | MFA status |
| `/tenants/security-rules` | GET | Security list rules |
| `/tenants/check-batch` | POST | Batch check |

### Current Dialogs (inline in Tenants.vue)
- **Add tenant**: OCI Config parse + manual form (~180 lines)
- **Detail**: tabs (basic info + status + costs + subscription) (~220 lines)
- **Edit name**: inline dialog (~15 lines)
- **Edit cost**: inline dialog (~15 lines)
- **Update from OCI**: dialog with progress (~20 lines)
- **Instances list**: dialog + table (~60 lines)
- **Security rules**: dialog + ingress/egress tabs (~100 lines)
- **Quota**: dialog (~30 lines)
- **Region subscriptions**: dialog (~60 lines)
- **Audit log**: dialog (~50 lines)
- **Traffic alert**: dialog (~30 lines)
- **Traffic query**: dialog (~50 lines)
- **Email config**: dialog (~70 lines)
- **Social config**: dialog (~60 lines)
- **User management**: dialog + create/reset/delete (~130 lines)
- **Export**: dialog (~20 lines)

### Helper Functions (shared)
`maskedName`, `accountTypeTag`, `accountTypeLabel`, `cloudTypeLabel`, `instStateDot`, `formatBytes` (~80 lines)

## 3. Proposed Architecture

```
/frontend/src/
├── views/
│   ├── TenantList.vue       # /tenants — table + toolbar + add dialog + batch ops
│   └── TenantDetail.vue     # /tenants/:id — tabbed detail page
├── utils/
│   └── tenant-utils.ts      # shared helpers (formatting, labels, masking)
└── router/
    └── index.ts              # add /tenants/:id route
```

### 3.1 TenantList.vue (~500 lines)

**Responsibilities:** Tenant table, toolbar (search, add, batch check, refresh), inline small dialogs.

**Stays in TenantList:**
- Table with columns: tenant name (masked), custom name, account cost, alive days, boot tasks, home region, account type, created time, status.
- Toolbar: search, "新增租户" button, "批量检查" button, "刷新" button.
- Operations dropdown (simplified):
  - **详情** → `router.push('/tenants/:id')` (navigation, not dialog)
  - **同步** → syncOci (inline action)
  - **导出** → inline export dialog (small, stays)
  - **删除** → inline confirmation + delete
- Add tenant dialog (stays — it's a creation flow, not a detail view).
- Data loading: `load()` fetches tenant list.

**Removed from TenantList:** detail dialog, edit name, edit cost, update, instances, security rules, quota, region sub, audit log, traffic alert/queries, email, social, user management. All moved to TenantDetail.

### 3.2 TenantDetail.vue (~800 lines)

**Responsibilities:** Display comprehensive tenant information in tabs. Each tab loads its own data.

**Route:** `/tenants/:id` (name: `tenant-detail`)

**Header:** Tenant name + status badge + action buttons (同步, 更新详情, 检查, 返回列表).

**Tabs:**

| Tab | Content | Data Source |
|-----|---------|-------------|
| 概况 | Basic info (el-descriptions), check result, edit name/cost | `GET /tenants/:id` + `/tenants/:id/check` |
| 实例 | Instance table for this tenant | `GET /tenants/:id/instances` |
| 费用 | Cost stats + subscription days + billing info | `GET /tenants/:id/subscription-days` |
| 用户 | IAM user management (list/create/delete/reset-password) | `/tenants/:id/users` CRUD |
| 邮件 | Email config (domain, SMTP, sender, toggle) | `/tenants/:id/email` CRUD |
| 社交 | Social login providers (GitHub/Google config) | `/tenants/:id/social` CRUD |
| 安全规则 | Security list rules (ingress/egress tabs) | `GET /tenants/security-rules` |
| 设置 | Sub-sections: Update from OCI (更新详情), MFA toggle, notification recipients, export (导出), delete (删除) | Multiple endpoints — grouped logically, not a flat list |

**Data loading:** Each tab loads its data on activation (lazy — only when the tab is selected). The basic info loads on page mount. Sub-tabs (instances, costs, etc.) load on first tab activation.

### 3.3 tenant-utils.ts (~80 lines)

Shared helpers extracted from Tenants.vue:
- `maskedName(name: string): string`
- `accountTypeTag(t: string): TagType`
- `accountTypeLabel(t: string): string`
- `cloudTypeLabel(ct: number): string`
- `instStateDot(state: string): string`
- `formatBytes(bytes: number | string): string`

## 4. Routing Changes

```typescript
// router/index.ts — add tenant-detail route
{
  path: 'tenants/:id',
  name: 'tenant-detail',
  component: () => import('../views/TenantDetail.vue')
}
```

This route is a child of the main layout (same as other pages). The `:id` is the tenant DB row id (int64).

## 5. Data Flow

```
TenantList                  TenantDetail
  ↓ router.push({          route.params.id
    name: 'tenant-detail',       ↓
    params: { id: row.id }  request.get(`/tenants/${id}`)
  })                              ↓
                            load basic info
                            ↓
                            tab selected
                            ↓
                            load tab-specific data
```

- TenantList → TenantDetail: navigation via `router.push`. No props needed (TenantDetail loads its own data).
- TenantDetail → TenantList: "返回" button uses `router.push('/tenants')` or `router.back()`.
- Sub-features in TenantDetail are self-contained (each tab manages its own state + API calls).

## 6. Migration Strategy

**Phase 1 — Extract utilities + TenantList skeleton:**
1. Create `tenant-utils.ts` with shared helpers.
2. Create `TenantList.vue` (copy table + toolbar + add dialog from Tenants.vue).
3. Update router: `/tenants` → TenantList.
4. Verify TenantList works (all list operations functional).

**Phase 2 — TenantDetail page:**
1. Create `TenantDetail.vue` with basic layout (header + tabs).
2. Migrate "概况" tab (basic info + check + edit name/cost) from Tenants.vue detail dialog.
3. Migrate each subsequent tab (instances, costs, users, email, social, security, settings).
4. Update TenantList "详情" button to `router.push`.

**Phase 3 — Cleanup TenantList:**
1. Remove migrated dialog code from TenantList (detail dialog, instances dialog, etc.).
2. Simplify operations dropdown.
3. Remove unused state refs + functions.

**Phase 4 — Verification:**
1. `npm run typecheck` — pass.
2. `npm run build` — pass.
3. Manual smoke test: add tenant, list, detail (all tabs), edit, delete.

Each phase produces a working state. During migration the old `Tenants.vue` stays in place as reference (unused by the router once Phase 1 points `/tenants` to `TenantList.vue`). It is deleted in Phase 3 after all code is extracted.

## 7. Open Questions

None — backend APIs already exist for all sub-features. No new backend work needed.

## 8. Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| TenantDetail lazy-loads tab data, slow for first click | Show loading skeleton; keep basic info eager-loaded |
| Large TenantDetail.vue (~800 lines) | Each tab's template + script is self-contained within the tab pane; can extract tab sub-components later if needed |
| Operations dropdown "17+ items" still overwhelming in detail page | Group related tabs; not all features need prominent placement |
