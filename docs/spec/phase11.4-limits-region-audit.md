# Phase 11.4 SPEC: Limits/Quota, Region Subscription, Audit Log

> **Status**: DRAFT
> **Java Reference**: `oci-start/oci-server/src/main/java/com/doubledimple/ociserver/`
> **Go Target**: `oci-start-go/`

---

## Table of Contents

1. [Feature 1: OCI Limits / Quota](#feature-1-oci-limits--quota)
2. [Feature 2: Region Subscription Management](#feature-2-region-subscription-management)
3. [Feature 3: Audit Log Querying](#feature-3-audit-log-querying)
4. [Go SDK Dependencies](#go-sdk-dependencies)
5. [Frontend Integration Notes](#frontend-integration-notes)

---

## Feature 1: OCI Limits / Quota

### 1.1 Overview

Allows viewing OCI service quotas (compute cores, memory, block storage, etc.) per tenant/region. Shows total limit, used amount, and remaining availability for each resource. Supports server-side pagination.

**Java reference**: `utils/oracle/OciLimitsUtils.java`, controller endpoint at `TenantController.java` line 1376.

### 1.2 API Endpoints

#### `GET /tenants/:id/quota`

Query the resource quota for a single service of a tenant.

**Query Parameters**:

| Param       | Type   | Default     | Description                          |
|-------------|--------|-------------|--------------------------------------|
| serviceName | string | `"compute"` | OCI service name (see below)         |
| page        | int    | `0`         | Page number (0-based)                |
| pageSize    | int    | `20`        | Items per page                       |

**Common service names**: `compute`, `block-storage`, `object-storage`

**Response** `200 OK`:

```json
{
  "region": "东京",
  "regionEn": "ap-tokyo-1",
  "service": "compute",
  "items": [
    {
      "name": "standard-a1-core-count",
      "total": 4,
      "used": 2,
      "available": 2
    },
    {
      "name": "standard-a1-memory-count-gb",
      "total": 24,
      "used": 12,
      "available": 12
    }
  ],
  "page": 0,
  "pageSize": 20,
  "hasNextPage": false
}
```

**Error Response** `400/500`:

```json
{
  "error": "tenant not found" 
}
```

### 1.3 OCI SDK Operations (Go)

The Go OCI SDK v65 provides `github.com/oracle/oci-go-sdk/v65/limits`.

```go
package oci

import (
    "context"
    "github.com/oracle/oci-go-sdk/v65/common"
    "github.com/oracle/oci-go-sdk/v65/identity"
    "github.com/oracle/oci-go-sdk/v65/limits"
)

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
    Page       int         `json:"page"`
    PageSize   int         `json:"pageSize"`
    HasNextPage bool       `json:"hasNextPage"`
}

// GetServiceQuotasPaged returns paginated quota data for a single service.
// Parity with OciLimitsUtils.getSingleServiceQuotasPaged.
//
// Two-pass approach (matches Java):
//   Pass 1: ListLimitValues (no AD filter) to collect unique non-zero limit names
//           up to (page+1)*pageSize+1 to determine hasNextPage early.
//   Pass 2: For the current page slice, call GetResourceAvailability per limit name.
//           AD-level limits are aggregated across all ADs; regional limits are queried directly.
//           Values ending in "-bytes" are converted to GB (div 1073741824) and renamed to "-gb".
func GetServiceQuotasPaged(
    ctx context.Context,
    clients Clients,          // must include Identity + Limits clients
    compartmentID string,
    serviceName string,
    page int,
    pageSize int,
) (*QuotaPage, error)
```

**Additional helper functions** (optional, for internal use):

```go
// ListLimitServices returns all services that support limits management.
// Parity with OciLimitsUtils.listServices.
func ListLimitServices(ctx context.Context, prov common.ConfigurationProvider, compartmentID string) ([]limits.ServiceSummary, error)

// GetResourceAvailability returns the availability for a single limit.
// Parity with OciLimitsUtils.getResourceAvailability.
func GetResourceAvailability(ctx context.Context, client *limits.LimitsClient, compartmentID, serviceName, limitName string, adName *string) (*limits.ResourceAvailability, error)

// GetAggregatedAvailability aggregates availability across all ADs for AD-level limits.
// Parity with OciLimitsUtils.getAggregatedAvailability.
func GetAggregatedAvailability(ctx context.Context, clients Clients, compartmentID, serviceName, limitName string) (total, used, available int64, err error)

// HasEnoughResource checks if the tenant has enough quota for a requested amount.
// Parity with OciLimitsUtils.hasEnoughResource.
func HasEnoughResource(ctx context.Context, clients Clients, compartmentID, serviceName, limitName string, required int64) (bool, error)
```

### 1.4 Constants

```go
const (
    ARMCoreFreeQuotaName = "standard-a1-core-count"
    ARMFreeQuotaName     = "standard-a1-memory-count"
    AMDCoreFreeQuotaName = "standard-e2-micro-core-count"
    AMDVMFreeCountName   = "vm-standard-e2-1-micro-count"
)
```

### 1.5 Business Logic

1. **Provider setup**: Build `limits.LimitsClient` and `identity.IdentityClient` from the tenant's credentials (same pattern as `NewClients` in `provider.go`).
2. **Pass 1 — Collect limit names**: Call `ListLimitValues` without an `availabilityDomain` filter, paginating through OCI results. Collect unique non-zero limit names in a `LinkedHashMap`-equivalent (Go: `map[string]bool` with insertion-order slice). Stop early once `(page+1)*pageSize+1` names are collected to determine `hasNextPage`.
3. **Slice for current page**: Extract `allNames[from:to]` where `from = page * pageSize`.
4. **Pass 2 — Get availability**: For each limit name in the page slice:
   - If the limit is AD-level (detected by `limitValue.AvailabilityDomain != nil`), query `GetResourceAvailability` for each AD and sum `used` and `available`.
   - If regional, query once without `availabilityDomain`.
   - Compute `total = used + available`.
5. **Unit conversion**: If `limitName` ends with `-bytes`, rename to `-gb` and divide all values by `1073741824`.
6. **Build response** with `items`, `page`, `pageSize`, `hasNextPage`.

### 1.6 Database Changes

None. Quotas are fetched live from OCI.

### 1.7 Error Handling

| Error Condition                  | HTTP | Handling                                                    |
|----------------------------------|------|-------------------------------------------------------------|
| Tenant not found                 | 400  | `{"error": "tenant not found"}`                             |
| OCI API error (ListLimitValues)  | 500  | Return empty items, log error, set `hasNextPage: false`     |
| OCI API error (GetResourceAvail) | skip | Log warning, treat used/available as 0 for that limit       |
| Invalid serviceName              | 200  | Return empty items (OCI returns empty for unknown services) |

### 1.8 New Go File

Create: `internal/oci/limits.go`

Add `Limits *limits.LimitsClient` to the `Clients` struct in `provider.go` and wire it in `NewClients`.

---

## Feature 2: Region Subscription Management

### 2.1 Overview

Manages OCI region subscriptions for a tenancy: view subscribed/unsubscribed regions, subscribe to new regions, check subscription status.

**Java reference**: `utils/oracle/region/OciRegionSubscriptionUtils.java`, controller endpoints in `TenantController.java` lines 249-445.

### 2.2 API Endpoints

#### `GET /tenants/:id/regions/summary`

Get counts of total, subscribed, and unsubscribed regions.

**Response** `200 OK`:

```json
{
  "totalRegions": 44,
  "subscribedRegions": 12,
  "unsubscribedRegions": 32
}
```

#### `GET /tenants/:id/regions/subscribed`

List all subscribed regions.

**Response** `200 OK`:

```json
[
  {
    "regionKey": "ap-tokyo-1",
    "regionName": "ap-tokyo-1",
    "status": { "value": "READY" },
    "isHomeRegion": true
  },
  {
    "regionKey": "ap-singapore-1",
    "regionName": "ap-singapore-1",
    "status": { "value": "READY" },
    "isHomeRegion": false
  }
]
```

#### `GET /tenants/:id/regions/unsubscribed`

List all unsubscribed regions.

**Response** `200 OK`:

```json
[
  {
    "key": "af-johannesburg-1",
    "name": "af-johannesburg-1",
    "cnName": "约翰内斯堡"
  }
]
```

Note: `cnName` is resolved from `region.NameByCode()` (the Go `region` package).

#### `POST /tenants/:id/regions/subscribe`

Subscribe to one or more regions. The Java implementation uses a long-polling pattern that waits up to 30 minutes for activation.

**Request Body**:

```json
{
  "regionKeys": ["af-johannesburg-1", "eu-paris-1"]
}
```

**Response** `200 OK`:

```json
{
  "success": true,
  "message": "All regions subscribed successfully",
  "details": [
    {
      "regionKey": "af-johannesburg-1",
      "success": true,
      "message": "Region subscribed successfully"
    },
    {
      "regionKey": "eu-paris-1",
      "success": false,
      "message": "Subscription failed: timeout waiting for activation"
    }
  ]
}
```

#### `GET /tenants/:id/regions/subscription-status?regionKey=ap-tokyo-1`

Check subscription status for a single region.

**Response** `200 OK`:

```json
{
  "regionKey": "ap-tokyo-1",
  "status": "READY",
  "subscribed": true
}
```

Or if not subscribed:

```json
{
  "regionKey": "af-johannesburg-1",
  "status": "NOT_SUBSCRIBED",
  "subscribed": false
}
```

### 2.3 OCI SDK Operations (Go)

The Go OCI SDK v65 provides region subscription operations through `github.com/oracle/oci-go-sdk/v65/identity`.

```go
package oci

import (
    "context"
    "github.com/oracle/oci-go-sdk/v65/identity"
)

// RegionSubInfo represents a subscribed region.
type RegionSubInfo struct {
    RegionKey   string `json:"regionKey"`
    RegionName  string `json:"regionName"`
    Status      string `json:"status"`       // READY, NOT_SUBSCRIBED, etc.
    IsHomeRegion bool  `json:"isHomeRegion"`
}

// RegionInfo represents an OCI region (all available).
type RegionInfo struct {
    Key  string `json:"key"`
    Name string `json:"name"`
}

// RegionSummary holds counts for the summary endpoint.
type RegionSummary struct {
    TotalRegions       int `json:"totalRegions"`
    SubscribedRegions   int `json:"subscribedRegions"`
    UnsubscribedRegions int `json:"unsubscribedRegions"`
}

// ListSubscribedRegions returns all regions the tenancy is subscribed to.
// Parity with OciRegionSubscriptionUtils.getSubscribedRegions.
// Uses: identityClient.ListRegionSubscriptions with tenancyId = compartmentID.
func ListSubscribedRegions(ctx context.Context, client *identity.IdentityClient, tenancyOCID string) ([]RegionSubInfo, error)

// ListAllRegions returns all OCI regions available for subscription.
// Parity with OciRegionSubscriptionUtils.getAllAvailableRegions.
// Uses: identityClient.ListRegions.
func ListAllRegions(ctx context.Context, client *identity.IdentityClient) ([]RegionInfo, error)

// ListUnsubscribedRegions returns regions not yet subscribed.
// Parity with OciRegionSubscriptionUtils.getUnsubscribedRegions.
// Computed as: allRegions - subscribedRegions (by regionKey).
func ListUnsubscribedRegions(ctx context.Context, client *identity.IdentityClient, tenancyOCID string) ([]RegionInfo, error)

// GetRegionSummary returns total/subscribed/unsubscribed counts.
// Parity with TenantController.getRegionSummary.
func GetRegionSummary(ctx context.Context, client *identity.IdentityClient, tenancyOCID string) (*RegionSummary, error)

// SubscribeToRegion subscribes the tenancy to a single region.
// Parity with OciRegionSubscriptionUtils.subscribeToRegion.
// Uses: identityClient.CreateRegionSubscription.
// Returns success/failure status message. Does NOT wait for activation (see WaitRegionActivation).
func SubscribeToRegion(ctx context.Context, client *identity.IdentityClient, tenancyOCID, regionKey string) (success bool, message string, err error)

// GetRegionSubscriptionStatus returns the status of a region subscription.
// Parity with OciRegionSubscriptionUtils.getRegionSubscriptionStatus.
// Returns "READY", "NOT_SUBSCRIBED", etc.
func GetRegionSubscriptionStatus(ctx context.Context, client *identity.IdentityClient, tenancyOCID, regionKey string) (string, error)

// WaitRegionActivation polls ListRegionSubscriptions every 30s until the
// region's status becomes "READY" or "FAILED", or timeout (maxWaitMinutes).
// Parity with OciRegionSubscriptionUtils.waitForSubscriptionActivation.
func WaitRegionActivation(ctx context.Context, client *identity.IdentityClient, tenancyOCID, regionKey string, maxWaitMinutes int) (bool, string, error)
```

### 2.4 Business Logic

**Subscribe flow** (per region key):
1. Check if already subscribed → return success immediately.
2. Validate the region key exists in the global region list → return error if not found.
3. Call `CreateRegionSubscription` with `tenancyId` and `regionKey`.
4. Poll `ListRegionSubscriptions` every 30 seconds for up to 30 minutes (60 attempts).
5. Return success when status becomes `READY`; return failure on `FAILED` or timeout.

**Batch subscribe**:
- Process each region key sequentially.
- Add a 1-second delay between requests to avoid OCI API rate limiting.
- Collect per-region results into the response details array.

**Go implementation note**: The Java implementation uses blocking `Thread.sleep` in the controller thread. For Go, the subscribe endpoint should either:
- Option A: Use a goroutine + SSE/WebSocket for progress updates (recommended).
- Option B: Return immediately after `CreateRegionSubscription` and have the frontend poll `check-subscription-status` (simpler, recommended for v1).

**Recommendation for v1**: Option B — skip the blocking wait. The `POST /tenants/:id/regions/subscribe` endpoint calls `CreateRegionSubscription`, returns immediately, and the frontend polls status via `GET /tenants/:id/regions/subscription-status`.

### 2.5 Database Changes

None. Region subscriptions are managed entirely by OCI.

### 2.6 Error Handling

| Error Condition                    | HTTP | Handling                                                      |
|------------------------------------|------|---------------------------------------------------------------|
| Tenant not found                   | 400  | `{"error": "tenant not found"}`                               |
| Region key not found               | 400  | `{"success": false, "message": "Region does not exist"}`      |
| Region already subscribed          | 200  | `{"success": true, "message": "Already subscribed"}`          |
| OCI CreateRegionSubscription error | 200  | `{"success": false, "message": "Subscription failed: ..."}`   |
| Timeout waiting for activation     | 200  | `{"success": false, "message": "Timeout"}`                    |

### 2.7 New Go Files

- Create: `internal/oci/region_sub.go` (or add functions to `identity.go`)
- The `identity.IdentityClient` is already available in `Clients.Identity`.

---

## Feature 3: Audit Log Querying

### 3.1 Overview

Queries OCI Audit Events for a tenant's tenancy. Supports two query modes: "recent N days" and "date range". Returns paginated audit events with user info, IP geolocation, event type, and response status.

**Java reference**: `utils/oracle/AuditLogUtils.java`, `pojo/dto/OciAuditEventDto.java`, `pojo/request/AuditLogRequest.java`, controller endpoint at `TenantController.java` line 1334.

### 3.2 API Endpoints

#### `POST /tenants/:id/audit-log`

Query audit logs for a tenant.

**Request Body**:

```json
{
  "days": 7,
  "pageToken": ""
}
```

OR date-range mode:

```json
{
  "startDate": "2026-06-01",
  "endDate": "2026-06-28",
  "pageToken": ""
}
```

| Field     | Type   | Required | Description                                                 |
|-----------|--------|----------|-------------------------------------------------------------|
| days      | int    | no*      | Query past N days (1-90). Default 1. Ignored if startDate set. |
| startDate | string | no*      | Start date `yyyy-MM-dd`. Required for date-range mode.      |
| endDate   | string | no       | End date `yyyy-MM-dd`. Defaults to startDate if omitted.    |
| pageToken | string | no       | Opaque token for next page (from previous response).        |

*Either `days` or `startDate` must be provided.

**Response** `200 OK`:

```json
{
  "success": true,
  "data": {
    "data": [
      {
        "eventType": "com.oraclecloud.computeapi.launchinstance",
        "userName": "oracleidentitycloudservice/admin@example.com",
        "userType": "natv",
        "ipAddress": "203.0.113.42(美国加利福尼亚州)",
        "clientEnv": "Oracle-JavaSDK/2.84.0",
        "eventTime": "2026-06-28 10:30:00",
        "responseStatus": "200"
      }
    ],
    "nextPageToken": "ey..."
  }
}
```

**Error Response**:

```json
{
  "success": false,
  "message": "Audit log query failed"
}
```

### 3.3 OCI SDK Operations (Go)

The Go OCI SDK v65 provides `github.com/oracle/oci-go-sdk/v65/audit`.

```go
package oci

import (
    "context"
    "time"
    "github.com/oracle/oci-go-sdk/v65/audit"
)

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

// ListAuditEvents queries audit events for a time range.
// Parity with AuditLogUtils.listAuditEvents.
//
// Uses: auditClient.ListEvents with compartmentId, startTime, endTime, pageToken.
// The compartmentId is the tenancy OCID (same as provider.getTenantId() in Java).
//
// For each AuditEvent, extracts:
//   - data.identity.principalName → userName (truncated to 35 chars + "...")
//   - data.identity.authType → userType
//   - data.identity.ipAddress → ipAddress (multi-IP resolved with geolocation)
//   - data.identity.userAgent → clientEnv
//   - eventType
//   - eventTime → formatted as "yyyy-MM-dd HH:mm:ss"
//   - data.response.status → responseStatus
//
// IP geolocation is optional for v1. The Java implementation calls PingUtil.getGeoInfoByIP
// which uses a third-party IP geolocation API. For the Go port, pass through the raw IP
// string initially; add geolocation resolution as a follow-up enhancement.
func ListAuditEvents(
    ctx context.Context,
    client *audit.AuditClient,
    compartmentID string,
    startTime time.Time,
    endTime time.Time,
    pageToken string,
) (*AuditLogPage, error)

// ListRecentAuditEvents queries the past N days (1-90, clamped).
// Parity with AuditLogUtils.listRecentAuditEvents.
func ListRecentAuditEvents(
    ctx context.Context,
    client *audit.AuditClient,
    compartmentID string,
    days int,
    pageToken string,
) (*AuditLogPage, error)

// ListAuditEventsByDateRange queries a specific date range (max 90 days).
// Parity with AuditLogUtils.listAuditEventsByDateRange.
// startDate/endDate format: "yyyy-MM-dd" (local dates converted to UTC start/end of day).
func ListAuditEventsByDateRange(
    ctx context.Context,
    client *audit.AuditClient,
    compartmentID string,
    startDate string,
    endDate string,
    pageToken string,
) (*AuditLogPage, error)
```

### 3.4 Business Logic

1. **Route by query mode** (matches `TenantServiceImpl.queryAuditLogs`):
   - If `startDate` is provided and non-empty → use `ListAuditEventsByDateRange`.
   - Otherwise → use `ListRecentAuditEvents` with `days` (default 1, clamped 1-90).
2. **Date range conversion** (for `ListAuditEventsByDateRange`):
   - `startDate` → start of day UTC (`00:00:00Z`).
   - `endDate` → end of day UTC (`23:59:59Z`). If empty, defaults to `startDate`.
   - Validate: `endDate >= startDate` and `diffDays <= 90`.
3. **Event extraction**: For each `AuditEvent` in the OCI response:
   - Skip events where `data` or `data.identity` is null.
   - Extract fields as documented in the function signature above.
   - Truncate `userName` to 35 characters + "..." if longer.
4. **Pagination**: Pass the `pageToken` from the previous response to get the next page. OCI returns `opc-next-page` in the response header.
5. **IP geolocation** (v2 enhancement): The Java code calls `PingUtil.getGeoInfoByIP` for each IP (supports comma-separated multi-IP strings, resolves public IPs via a geolocation API, marks private IPs as "internal"). For Go v1, return raw IPs. For v2, add geolocation using a similar API.

### 3.5 Database Changes

None. Audit events are queried live from OCI.

### 3.6 Error Handling

| Error Condition                | HTTP | Handling                                                    |
|--------------------------------|------|-------------------------------------------------------------|
| Tenant not found               | 500  | `{"success": false, "message": "Tenant not found"}`         |
| Invalid date range             | 200  | `{"success": false, "data": {"data": [], "nextPageToken": ""}}` |
| OCI BmcException (4xx/5xx)     | 200  | Log warning, return empty result with success=false         |
| OCI generic error              | 200  | Log error, return `{"success": false, "message": "..."}`    |
| days out of range              | clamp| Clamp to 1-90, proceed                                    |

### 3.7 New Go Files

Create: `internal/oci/audit.go`

Add `Audit *audit.AuditClient` to the `Clients` struct in `provider.go` and wire it in `NewClients`.

---

## Go SDK Dependencies

### Package Additions to `provider.go`

The `Clients` struct needs two new fields:

```go
import (
    "github.com/oracle/oci-go-sdk/v65/audit"
    "github.com/oracle/oci-go-sdk/v65/limits"
)

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

In `NewClients`, add:

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

Also update `NewClientsWithHTTPClient` in `proxy.go` to inject the HTTP client into the new clients.

### Route Registration (server.go)

Add to the protected routes group in `NewServer`:

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

## Frontend Integration Notes

### Tenants.vue Dropdown Additions

The tenant dropdown menu in `Tenants.vue` currently has no entries for quota, region subscription, or audit log. Add three new items to the `<el-dropdown-menu>` (between "instances" and "trafficAlert"):

```html
<el-dropdown-item command="quota">
  <el-icon><DataLine /></el-icon> 配额查看
</el-dropdown-item>
<el-dropdown-item command="regionSub">
  <el-icon><Location /></el-icon> 区域订阅
</el-dropdown-item>
<el-dropdown-item command="auditLog">
  <el-icon><Document /></el-icon> 审计日志
</el-dropdown-item>
```

### handleAction Cases

Add to the `handleAction` switch:

```typescript
case 'quota': openQuotaDialog(row); break
case 'regionSub': openRegionSubDialog(row); break
case 'auditLog': openAuditLogDialog(row); break
```

### Quota Dialog

A dialog with:
- Service selector dropdown (`compute`, `block-storage`, `object-storage`).
- Table with columns: Name, Total, Used, Available.
- Pagination controls using `page` / `pageSize` / `hasNextPage`.
- API call: `GET /tenants/${tenant.id}/quota?serviceName=${service}&page=${page}&pageSize=20`

### Region Subscription Dialog

A dialog with:
- Summary card showing total/subscribed/unsubscribed counts.
- Two tabs: "Subscribed" and "Unsubscribed".
- Subscribed tab: table with regionKey, regionName, status, isHomeRegion.
- Unsubscribed tab: table with checkbox selection + "Subscribe Selected" button.
- API calls: `GET /tenants/${tenant.id}/regions/summary`, `GET .../subscribed`, `GET .../unsubscribed`, `POST .../subscribe`.

### Audit Log Dialog

A dialog with:
- Date range picker (or "Past N days" quick selector: 1, 3, 7, 30, 90).
- Table with columns: Time, Event Type, User, IP, Client, Status.
- "Next Page" button using `nextPageToken`.
- API call: `POST /tenants/${tenant.id}/audit-log` with body.

---

## Summary of New Files

| File                                | Purpose                                      |
|-------------------------------------|----------------------------------------------|
| `internal/oci/limits.go`            | Limits/Quota OCI SDK wrapper functions       |
| `internal/oci/audit.go`             | Audit Log OCI SDK wrapper functions          |
| `internal/oci/region_sub.go`        | Region subscription OCI SDK wrapper functions|
| `internal/httpapi/handler_quota.go` | HTTP handler for quota endpoint              |
| `internal/httpapi/handler_regions.go`| HTTP handlers for region subscription endpoints|
| `internal/httpapi/handler_audit.go` | HTTP handler for audit log endpoint          |

## Summary of Modified Files

| File                                | Change                                                  |
|-------------------------------------|---------------------------------------------------------|
| `internal/oci/provider.go`          | Add `Limits` and `Audit` fields to `Clients` struct    |
| `internal/oci/proxy.go`             | Wire new clients in `NewClientsWithHTTPClient`          |
| `internal/httpapi/server.go`        | Register 7 new route handlers                          |
| `frontend/src/views/Tenants.vue`    | Add 3 dropdown items + 3 dialog components + handlers  |
