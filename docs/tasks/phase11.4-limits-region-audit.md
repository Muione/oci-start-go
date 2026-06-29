# Phase 11.4 Limits / Region Subscription / Audit Log -- Task Breakdown

## Overview

Three independent sub-features, each with its own OCI SDK wrapper, service layer, and HTTP handlers. All three share a common prerequisite: adding `Limits` and `Audit` fields to the `Clients` struct in `provider.go`. No database schema changes are needed for any sub-feature.

---

## Prerequisite: Provider Client Expansion

### P1: Add New OCI Clients to Provider

**File:** `internal/oci/provider.go` (modify)

**Changes to `Clients` struct** (currently at line 74):

```go
type Clients struct {
    Compute       *core.ComputeClient
    Vcn           *core.VirtualNetworkClient
    Identity      *identity.IdentityClient
    ObjectStorage *objectstorage.ObjectStorageClient
    Blockstorage  *core.BlockstorageClient
    Limits        *limits.LimitsClient       // NEW
    Audit         *audit.AuditClient          // NEW
}
```

**New imports:**

```go
import (
    "github.com/oracle/oci-go-sdk/v65/audit"
    "github.com/oracle/oci-go-sdk/v65/limits"
)
```

**Changes to `NewClients`** (add after Blockstorage client creation, around line 103):

```go
limClient, err := limits.NewLimitsClientWithConfigurationProvider(p)
if err != nil {
    return Clients{}, fmt.Errorf("limits client: %w", err)
}
audClient, err := audit.NewAuditClientWithConfigurationProvider(p)
if err != nil {
    return Clients{}, fmt.Errorf("audit client: %w", err)
}
```

Update the return statement to include `Limits: &limClient, Audit: &audClient`.

**File:** `internal/oci/proxy.go` (modify)

**Changes to `NewClientsWithHTTPClient`** (add after line 119):

```go
if c.Limits != nil { c.Limits.HTTPClient = hc }
if c.Audit != nil { c.Audit.HTTPClient = hc }
```

---

## Feature 1: OCI Limits / Quota

### B1.1: OCI SDK Wrapper

**File:** `internal/oci/limits.go` (new)

**Pattern to follow:** `internal/oci/network.go` -- stateless functions with `(ctx, c Clients, ...)` signature.

**Types:**

```go
// QuotaItem represents a single resource quota entry.
type QuotaItem struct {
    Name      string `json:"name"`
    Total     int64  `json:"total"`
    Used      int64  `json:"used"`
    Available int64  `json:"available"`
}

// QuotaPage is a paginated response of quota items.
type QuotaPage struct {
    Items      []QuotaItem `json:"items"`
    Region     string      `json:"region"`
    RegionEn   string      `json:"regionEn"`
    Service    string      `json:"service"`
    Page       int         `json:"page"`
    PageSize   int         `json:"pageSize"`
    HasNextPage bool       `json:"hasNextPage"`
}
```

**Functions:**

```go
// GetServiceQuotasPaged returns paginated quota data for a single service.
// Two-pass approach (matches Java OciLimitsUtils.getSingleServiceQuotasPaged):
//   Pass 1: ListLimitValues (no AD filter) to collect unique non-zero limit names.
//   Pass 2: GetResourceAvailability per limit name for the current page slice.
// AD-level limits are aggregated across all ADs; regional limits queried directly.
// Values ending in "-bytes" are converted to GB (div 1073741824) and renamed "-gb".
func GetServiceQuotasPaged(
    ctx context.Context,
    c Clients,
    compartmentID, serviceName string,
    regionName string,
    page, pageSize int,
) (*QuotaPage, error)

// ListLimitServices returns all services that support limits.
// Uses: limitsClient.ListServices.
func ListLimitServices(ctx context.Context, c Clients, compartmentID string) ([]limits.ServiceSummary, error)

// HasEnoughResource checks if the tenant has enough quota for a requested amount.
func HasEnoughResource(
    ctx context.Context, c Clients,
    compartmentID, serviceName, limitName string,
    required int64,
) (bool, error)
```

**Internal helpers:**

```go
// getAggregatedAvailability sums availability across all ADs for AD-level limits.
func getAggregatedAvailability(ctx context.Context, c Clients, compartmentID, serviceName, limitName string) (total, used, available int64, err error)

// getResourceAvailability returns availability for a single limit.
func getResourceAvailability(ctx context.Context, c Clients, compartmentID, serviceName, limitName string, adName *string) (used, available int64, err error)
```

**Constants:**

```go
const (
    ARMCoreFreeQuotaName = "standard-a1-core-count"
    ARMFreeQuotaName     = "standard-a1-memory-count"
    AMDCoreFreeQuotaName = "standard-e2-micro-core-count"
    AMDVMFreeCountName   = "vm-standard-e2-1-micro-count"
)
```

**OCI SDK calls:**
- `limitsClient.ListLimitValues(ctx, limits.ListLimitValuesRequest{CompartmentId: ..., ServiceName: ..., ...})`
- `limitsClient.GetResourceAvailability(ctx, limits.GetResourceAvailabilityRequest{CompartmentId: ..., ServiceName: ..., LimitName: ..., AvailabilityDomain: ...})`
- `identityClient.ListAvailabilityDomains(ctx, identity.ListAvailabilityDomainsRequest{CompartmentId: ...})`

**Key logic:**
1. Pass 1: paginate through `ListLimitValues` without AD filter, collect unique limit names where `Value > 0`. Stop early once `(page+1)*pageSize+1` names collected.
2. Slice `allNames[from:to]` where `from = page * pageSize`.
3. Pass 2: for each limit name in the slice, check if AD-level (from Pass 1 data). If AD-level, query each AD and sum. If regional, query once.
4. Unit conversion: names ending `-bytes` get renamed to `-gb` and values divided by 1073741824.

---

### B1.2: Service Layer

**File:** `internal/service/quota.go` (new)

**Pattern to follow:** `internal/service/tenant.go` -- struct with store/masterKey/pool, methods that look up tenant and call OCI wrappers via `oci.WithProxy`.

```go
type QuotaService struct {
    store     *db.Store
    masterKey []byte
    pool      *oci.ProxyPool
}

func NewQuotaService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *QuotaService

// GetQuota returns paginated quota for a tenant+service.
func (s *QuotaService) GetQuota(ctx context.Context, tenantID int64, serviceName string, page, pageSize int) (*oci.QuotaPage, error)
```

---

### B1.3: HTTP Handler

**File:** `internal/httpapi/handler_quota.go` (new)

```go
// GET /tenants/:id/quota?serviceName=compute&page=0&pageSize=20
func tenantQuota(deps *Deps) gin.HandlerFunc
// 1. Parse path param :id (int64)
// 2. Parse query: serviceName (default "compute"), page (default 0), pageSize (default 20)
// 3. Call deps.Quota.GetQuota(ctx, tenantID, serviceName, page, pageSize)
// 4. Return JSON QuotaPage
// 5. On error: c.JSON(400/500, gin.H{"error": err.Error()})
```

**Deps addition:** Add `Quota *service.QuotaService` to `Deps` struct.

---

## Feature 2: Region Subscription Management

### B2.1: OCI SDK Wrapper

**File:** `internal/oci/region_sub.go` (new)

**Pattern to follow:** `internal/oci/network.go`.

**Types:**

```go
// RegionSubInfo represents a subscribed region.
type RegionSubInfo struct {
    RegionKey    string `json:"regionKey"`
    RegionName   string `json:"regionName"`
    Status       string `json:"status"`
    IsHomeRegion bool   `json:"isHomeRegion"`
}

// RegionInfo represents an unsubscribed region.
type RegionInfo struct {
    Key    string `json:"key"`
    Name   string `json:"name"`
    CnName string `json:"cnName"`
}

// RegionSummary holds counts for the summary endpoint.
type RegionSummary struct {
    TotalRegions       int `json:"totalRegions"`
    SubscribedRegions   int `json:"subscribedRegions"`
    UnsubscribedRegions int `json:"unsubscribedRegions"`
}

// RegionSubscribeResult is the per-region result in a batch subscribe.
type RegionSubscribeResult struct {
    RegionKey string `json:"regionKey"`
    Success   bool   `json:"success"`
    Message   string `json:"message"`
}

// RegionSubscribeResponse is the batch subscribe response.
type RegionSubscribeResponse struct {
    Success bool                   `json:"success"`
    Message string                 `json:"message"`
    Details []RegionSubscribeResult `json:"details"`
}
```

**Functions:**

```go
// ListSubscribedRegions returns all regions the tenancy is subscribed to.
// Uses: identityClient.ListRegionSubscriptions with tenancyId = compartmentID.
// Parity with OciRegionSubscriptionUtils.getSubscribedRegions.
func ListSubscribedRegions(ctx context.Context, c Clients, tenancyOCID string) ([]RegionSubInfo, error)

// ListAllRegions returns all OCI regions available.
// Uses: identityClient.ListRegions.
// Parity with OciRegionSubscriptionUtils.getAllAvailableRegions.
func ListAllRegions(ctx context.Context, c Clients) ([]RegionInfo, error)

// ListUnsubscribedRegions computes allRegions - subscribedRegions (by key).
// Parity with OciRegionSubscriptionUtils.getUnsubscribedRegions.
func ListUnsubscribedRegions(ctx context.Context, c Clients, tenancyOCID string) ([]RegionInfo, error)

// GetRegionSummary returns total/subscribed/unsubscribed counts.
func GetRegionSummary(ctx context.Context, c Clients, tenancyOCID string) (*RegionSummary, error)

// SubscribeToRegion subscribes the tenancy to a single region.
// Uses: identityClient.CreateRegionSubscription. Does NOT wait for activation.
// Returns success/failure status.
func SubscribeToRegion(ctx context.Context, c Clients, tenancyOCID, regionKey string) (success bool, message string, err error)

// GetRegionSubscriptionStatus returns "READY", "NOT_SUBSCRIBED", etc.
func GetRegionSubscriptionStatus(ctx context.Context, c Clients, tenancyOCID, regionKey string) (string, error)

// WaitRegionActivation polls ListRegionSubscriptions every 30s until the
// region's status becomes "READY" or "FAILED", or timeout.
// Optional -- v1 can skip the blocking wait.
func WaitRegionActivation(ctx context.Context, c Clients, tenancyOCID, regionKey string, maxWaitMinutes int) (bool, string, error)
```

**OCI SDK calls:**
- `identityClient.ListRegionSubscriptions(ctx, identity.ListRegionSubscriptionsRequest{TenancyId: ...})`
- `identityClient.ListRegions(ctx, identity.ListRegionsRequest{})`
- `identityClient.CreateRegionSubscription(ctx, identity.CreateRegionSubscriptionRequest{CreateRegionSubscriptionDetails: ...})`

**Key logic:**
- Subscribe flow: check if already subscribed -> validate region key exists -> call CreateRegionSubscription -> return immediately (v1).
- `cnName` resolved from the existing `region.NameByCode()` function in `internal/oci/region/`.

---

### B2.2: Service Layer

**File:** `internal/service/region_sub.go` (new)

```go
type RegionSubService struct {
    store     *db.Store
    masterKey []byte
    pool      *oci.ProxyPool
}

func NewRegionSubService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *RegionSubService

// Summary returns region counts for a tenant.
func (s *RegionSubService) Summary(ctx context.Context, tenantID int64) (*oci.RegionSummary, error)

// Subscribed returns subscribed regions for a tenant.
func (s *RegionSubService) Subscribed(ctx context.Context, tenantID int64) ([]oci.RegionSubInfo, error)

// Unsubscribed returns unsubscribed regions for a tenant.
func (s *RegionSubService) Unsubscribed(ctx context.Context, tenantID int64) ([]oci.RegionInfo, error)

// Subscribe subscribes a tenant to one or more regions (batch).
func (s *RegionSubService) Subscribe(ctx context.Context, tenantID int64, regionKeys []string) (*oci.RegionSubscribeResponse, error)

// SubscriptionStatus returns the subscription status for a single region.
func (s *RegionSubService) SubscriptionStatus(ctx context.Context, tenantID int64, regionKey string) (map[string]interface{}, error)
```

---

### B2.3: HTTP Handlers

**File:** `internal/httpapi/handler_regions.go` (new)

```go
// GET /tenants/:id/regions/summary
func tenantRegionSummary(deps *Deps) gin.HandlerFunc
// Return JSON: {totalRegions, subscribedRegions, unsubscribedRegions}

// GET /tenants/:id/regions/subscribed
func tenantRegionsSubscribed(deps *Deps) gin.HandlerFunc
// Return JSON array of RegionSubInfo

// GET /tenants/:id/regions/unsubscribed
func tenantRegionsUnsubscribed(deps *Deps) gin.HandlerFunc
// Return JSON array of RegionInfo (with cnName from region.NameByCode)

// POST /tenants/:id/regions/subscribe
func tenantRegionsSubscribe(deps *Deps) gin.HandlerFunc
// Bind JSON: {"regionKeys": ["af-johannesburg-1", ...]}
// Return JSON: RegionSubscribeResponse

// GET /tenants/:id/regions/subscription-status?regionKey=ap-tokyo-1
func tenantRegionSubStatus(deps *Deps) gin.HandlerFunc
// Return JSON: {regionKey, status, subscribed}
```

**Deps addition:** Add `RegionSub *service.RegionSubService` to `Deps` struct.

---

## Feature 3: Audit Log Querying

### B3.1: OCI SDK Wrapper

**File:** `internal/oci/audit.go` (new)

**Pattern to follow:** `internal/oci/network.go`.

**Types:**

```go
// AuditEventDTO is a simplified audit event for the API response.
type AuditEventDTO struct {
    EventType      string `json:"eventType"`
    UserName       string `json:"userName"`
    UserType       string `json:"userType"`
    IPAddress      string `json:"ipAddress"`
    ClientEnv      string `json:"clientEnv"`
    EventTime      string `json:"eventTime"`
    ResponseStatus string `json:"responseStatus"`
}

// AuditLogPage is a paginated response of audit events.
type AuditLogPage struct {
    Data          []AuditEventDTO `json:"data"`
    NextPageToken string          `json:"nextPageToken,omitempty"`
}
```

**Functions:**

```go
// ListAuditEvents queries audit events for a time range.
// Uses: auditClient.ListEvents with compartmentId, startTime, endTime, pageToken.
// compartmentId is the tenancy OCID.
//
// For each AuditEvent, extracts:
//   - data.identity.principalName -> userName (truncated to 35 chars + "...")
//   - data.identity.authType -> userType
//   - data.identity.ipAddress -> ipAddress (raw IP for v1)
//   - data.identity.userAgent -> clientEnv
//   - eventType
//   - eventTime -> formatted as "yyyy-MM-dd HH:mm:ss"
//   - data.response.status -> responseStatus
//
// Skips events where data or data.identity is null.
func ListAuditEvents(
    ctx context.Context,
    c Clients,
    compartmentID string,
    startTime, endTime time.Time,
    pageToken string,
) (*AuditLogPage, error)

// ListRecentAuditEvents queries the past N days (1-90, clamped).
func ListRecentAuditEvents(
    ctx context.Context,
    c Clients,
    compartmentID string,
    days int,
    pageToken string,
) (*AuditLogPage, error)

// ListAuditEventsByDateRange queries a specific date range (max 90 days).
// startDate/endDate format: "yyyy-MM-dd" (converted to UTC start/end of day).
func ListAuditEventsByDateRange(
    ctx context.Context,
    c Clients,
    compartmentID string,
    startDate, endDate string,
    pageToken string,
) (*AuditLogPage, error)
```

**OCI SDK calls:**
- `auditClient.ListEvents(ctx, audit.ListEventsRequest{CompartmentId: ..., StartTime: ..., EndTime: ..., Page: ...})`

**Key logic:**
- Route by query mode: if `startDate` provided -> `ListAuditEventsByDateRange`; else -> `ListRecentAuditEvents` with `days` (default 1, clamped 1-90).
- Date range conversion: `startDate` -> start of day UTC; `endDate` -> end of day UTC. Validate `endDate >= startDate` and `diffDays <= 90`.
- Pagination: pass `pageToken` from previous response; OCI returns `opc-next-page` header.
- IP geolocation: v1 returns raw IPs (no geolocation). v2 enhancement can add `PingUtil.getGeoInfoByIP` equivalent.

---

### B3.2: Service Layer

**File:** `internal/service/audit.go` (new)

```go
type AuditService struct {
    store     *db.Store
    masterKey []byte
    pool      *oci.ProxyPool
}

func NewAuditService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *AuditService

// Query queries audit logs for a tenant. Supports both "recent days" and
// "date range" modes based on the request body.
func (s *AuditService) Query(ctx context.Context, tenantID int64, req AuditLogRequest) (*oci.AuditLogPage, error)
```

**Request type:**

```go
// AuditLogRequest is the request body for the audit log endpoint.
type AuditLogRequest struct {
    Days      int    `json:"days"`       // 1-90, default 1
    StartDate string `json:"startDate"`  // "yyyy-MM-dd"
    EndDate   string `json:"endDate"`    // "yyyy-MM-dd"
    PageToken string `json:"pageToken"`
}
```

---

### B3.3: HTTP Handler

**File:** `internal/httpapi/handler_audit.go` (new)

```go
// POST /tenants/:id/audit-log
func tenantAuditLog(deps *Deps) gin.HandlerFunc
// 1. Parse path param :id (int64)
// 2. Bind JSON body to service.AuditLogRequest
// 3. Call deps.Audit.Query(ctx, tenantID, req)
// 4. Return {"success": true, "data": AuditLogPage}
// 5. On tenant not found: return 500 {"success": false, "message": "Tenant not found"}
// 6. On OCI error: return 200 {"success": false, "message": "..."}
```

**Deps addition:** Add `Audit *service.AuditService` to `Deps` struct.

---

## Route Registration

### B4: All Routes in server.go

**File:** `internal/httpapi/server.go` (modify)

Add to the protected routes group (after Phase 11.3 routes):

```go
// Phase 11.4: Quota, Region Subscription, Audit Log
pro.GET("/tenants/:id/quota", tenantQuota(deps))
pro.GET("/tenants/:id/regions/summary", tenantRegionSummary(deps))
pro.GET("/tenants/:id/regions/subscribed", tenantRegionsSubscribed(deps))
pro.GET("/tenants/:id/regions/unsubscribed", tenantRegionsUnsubscribed(deps))
pro.POST("/tenants/:id/regions/subscribe", tenantRegionsSubscribe(deps))
pro.GET("/tenants/:id/regions/subscription-status", tenantRegionSubStatus(deps))
pro.POST("/tenants/:id/audit-log", tenantAuditLog(deps))
```

---

## Dependency Wiring

### B5: Wire All New Services

**File:** `cmd/server/main.go` (or wherever Deps is constructed)

```go
deps.Quota = service.NewQuotaService(store, masterKey, pool)
deps.RegionSub = service.NewRegionSubService(store, masterKey, pool)
deps.Audit = service.NewAuditService(store, masterKey, pool)
```

---

## Frontend Tasks

### F1: Quota Dialog

**File:** `frontend/src/views/Tenants.vue` (modify)

**Dropdown item** (add after "instances", before "securityRules"/"trafficAlert"):

```html
<el-dropdown-item command="quota">
  <el-icon><DataLine /></el-icon> 配额查看
</el-dropdown-item>
```

**handleAction addition:**

```typescript
case 'quota': openQuotaDialog(row); break
```

**New state:**

```typescript
const quotaVisible = ref(false)
const quotaLoading = ref(false)
const quotaTenantId = ref(0)
const quotaTenantName = ref('')
const quotaService = ref('compute')
const quotaData = ref<any>({ items: [], page: 0, pageSize: 20, hasNextPage: false })
const quotaServices = ['compute', 'block-storage', 'object-storage']
```

**Functions:**

```typescript
async function openQuotaDialog(row: Tenant) { /* set tenant, show dialog, fetch */ }
async function fetchQuota(page = 0) {
  /* GET /tenants/${quotaTenantId.value}/quota?serviceName=${quotaService.value}&page=${page}&pageSize=20 */
}
```

**Dialog template:**
- Service selector dropdown (compute, block-storage, object-storage)
- `<el-table>` with columns: Name, Total, Used, Available
- Pagination: "Previous"/"Next" buttons using `page`/`hasNextPage`

---

### F2: Region Subscription Dialog

**File:** `frontend/src/views/Tenants.vue` (modify)

**Dropdown item:**

```html
<el-dropdown-item command="regionSub">
  <el-icon><Location /></el-icon> 区域订阅
</el-dropdown-item>
```

**handleAction addition:**

```typescript
case 'regionSub': openRegionSubDialog(row); break
```

**New state:**

```typescript
const regionSubVisible = ref(false)
const regionSubLoading = ref(false)
const regionSubTenantId = ref(0)
const regionSubTenantName = ref('')
const regionSubTab = ref('subscribed')
const regionSummary = ref({ totalRegions: 0, subscribedRegions: 0, unsubscribedRegions: 0 })
const subscribedRegions = ref<any[]>([])
const unsubscribedRegions = ref<any[]>([])
const selectedUnsubRegions = ref<string[]>([])
const regionSubscribing = ref(false)
```

**Functions:**

```typescript
async function openRegionSubDialog(row: Tenant) { /* set tenant, show dialog, fetch all 3 */ }
async function fetchRegionData() {
  /* parallel: GET .../summary, GET .../subscribed, GET .../unsubscribed */
}
async function subscribeSelected() {
  /* POST .../subscribe with {regionKeys: selectedUnsubRegions.value} */
  /* On success, refresh data */
}
```

**Dialog template:**
- Summary card: 3 `<el-statistic>` showing total/subscribed/unsubscribed counts
- `<el-tabs>` for "Subscribed" and "Unsubscribed"
- Subscribed tab: `<el-table>` with regionKey, regionName, status, isHomeRegion
- Unsubscribed tab: `<el-table>` with checkbox selection + "Subscribe Selected" button

---

### F3: Audit Log Dialog

**File:** `frontend/src/views/Tenants.vue` (modify)

**Dropdown item:**

```html
<el-dropdown-item command="auditLog">
  <el-icon><Document /></el-icon> 审计日志
</el-dropdown-item>
```

**handleAction addition:**

```typescript
case 'auditLog': openAuditLogDialog(row); break
```

**New state:**

```typescript
const auditLogVisible = ref(false)
const auditLogLoading = ref(false)
const auditLogTenantId = ref(0)
const auditLogTenantName = ref('')
const auditLogDays = ref(7)
const auditLogData = ref<any[]>([])
const auditLogNextPageToken = ref('')
const auditLogMode = ref<'days' | 'range'>('days')
const auditLogDateRange = ref<string[] | null>(null)
```

**Functions:**

```typescript
async function openAuditLogDialog(row: Tenant) { /* set tenant, show dialog, fetch */ }
async function fetchAuditLog(pageToken = '') {
  /* POST /tenants/${auditLogTenantId.value}/audit-log with body */
  /* body: { days: auditLogDays.value, pageToken } or { startDate, endDate, pageToken } */
}
async function fetchNextAuditPage() {
  /* fetchAuditLog(auditLogNextPageToken.value) */
}
```

**Dialog template:**
- Mode toggle: "Recent Days" / "Date Range"
- Days mode: quick selector buttons (1, 3, 7, 30, 90 days)
- Range mode: `<el-date-picker type="daterange">`
- `<el-table>` with columns: Time, Event Type, User, IP, Client, Status
- "Next Page" button (enabled when `auditLogNextPageToken` is non-empty)

---

### F4: TypeScript Types

**File:** `frontend/src/types/api.ts` (modify)

Add the following interfaces:

```typescript
/** Security rule from GET /tenants/security-rules */
export interface SecurityRuleDTO {
  id: string | null
  type: string
  protocol: string
  source: string
  ports: string | null
  tenantId: number | null
  icmpType: string | null
}

/** Quota item from GET /tenants/:id/quota */
export interface QuotaItem {
  name: string
  total: number
  used: number
  available: number
}

/** Quota page response */
export interface QuotaPage {
  items: QuotaItem[]
  region: string
  regionEn: string
  service: string
  page: number
  pageSize: number
  hasNextPage: boolean
}

/** Subscribed region */
export interface RegionSubInfo {
  regionKey: string
  regionName: string
  status: string
  isHomeRegion: boolean
}

/** Region summary counts */
export interface RegionSummary {
  totalRegions: number
  subscribedRegions: number
  unsubscribedRegions: number
}

/** Audit event */
export interface AuditEventDTO {
  eventType: string
  userName: string
  userType: string
  ipAddress: string
  clientEnv: string
  eventTime: string
  responseStatus: string
}

/** Audit log page response */
export interface AuditLogPage {
  data: AuditEventDTO[]
  nextPageToken?: string
}
```

---

### F5: Icon Imports

**File:** `frontend/src/views/Tenants.vue` (modify)

Add to the icon import:

```typescript
import {
  Plus, Refresh, Monitor, Connection, InfoFilled, Edit, VideoPlay,
  Warning, DataAnalysis, Message, Share, Download, Delete, Search,
  Operation, MoreFilled, Key, User,
  Lock, DataLine, Location, Document  // NEW
} from '@element-plus/icons-vue'
```

---

## Dependencies

```
P1 (Provider expansion) --> B1.1 / B2.1 / B3.1 (parallel, all need Limits/Audit clients)
B1.1 --> B1.2 --> B1.3 (quota chain)
B2.1 --> B2.2 --> B2.3 (region chain)
B3.1 --> B3.2 --> B3.3 (audit chain)
B1.3 + B2.3 + B3.3 --> B4 (routes) --> B5 (wire)
B4 --> F1 / F2 / F3 / F4 / F5 (frontend, parallel)
```

The three sub-features (Limits, Region, Audit) are independent of each other and can be developed in parallel once P1 is complete.

## Test Checklist

### Limits/Quota
- [ ] GET /tenants/:id/quota returns paginated quota items
- [ ] Service selector works for compute, block-storage, object-storage
- [ ] Pagination (page/hasNextPage) works correctly
- [ ] AD-level limits are aggregated across all ADs
- [ ] "-bytes" values converted to "-gb"

### Region Subscription
- [ ] GET /tenants/:id/regions/summary returns correct counts
- [ ] GET /tenants/:id/regions/subscribed lists subscribed regions
- [ ] GET /tenants/:id/regions/unsubscribed lists unsubscribed regions
- [ ] POST /tenants/:id/regions/subscribe creates subscription(s)
- [ ] GET /tenants/:id/regions/subscription-status returns status
- [ ] Already-subscribed regions handled gracefully

### Audit Log
- [ ] POST /tenants/:id/audit-log with days mode returns events
- [ ] POST /tenants/:id/audit-log with date range mode returns events
- [ ] Pagination via nextPageToken works
- [ ] userName truncated to 35 chars + "..."
- [ ] Invalid date range returns empty result
- [ ] days clamped to 1-90
