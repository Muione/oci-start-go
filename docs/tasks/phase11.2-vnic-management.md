# Phase 11.2: VNIC Management -- Task Breakdown

> SPEC: `docs/spec/phase11.2-vnic-management.md`
> Pattern references: `internal/oci/vnic.go`, `internal/oci/network.go`, `internal/httpapi/tenant.go`, `internal/service/tenant.go`

---

## Dependency Graph

```
T1 (OCI SDK: add NLB client to Clients struct)
  |
T2 (OCI wrappers: expand internal/oci/vnic.go)  <-- depends on T1
  |
T3 (OCI wrappers: expand internal/oci/network.go)  <-- depends on T1
  |
T4 (New file: internal/oci/nlb.go)  <-- depends on T1
  |
T5 (New file: internal/oci/compute.go -- ResetInstance)  <-- no deps
  |
T6 (Service layer: internal/service/vnic_management.go)  <-- depends on T2, T3, T4, T5
  |
T7 (HTTP handlers: internal/httpapi/vnic_management.go)  <-- depends on T6
  |
T8 (deps.go + route registration)  <-- depends on T7
  |
T9 (Frontend: VnicManagement.vue / dialog)  <-- depends on T8
  |
T10 (Frontend: router registration)  <-- depends on T9
```

---

## Existing Go Code to Reuse

The following functions already exist and should be called directly from the new service/handler code:

**`internal/oci/vnic.go`:**
- `ListVnicAttachmentsForInstance(ctx, computeClient, compartmentID, instanceID)` -- paginated VNIC attachment listing
- `GetVnicInfo(ctx, vcnClient, vnicID)` -- resolves VNIC OCID to IP details
- `ListAllVnicsForInstance(ctx, computeClient, vcnClient, compartmentID, instanceID)` -- combines attachments + VNIC info
- `AssignIpv6ToVnic(ctx, vcnClient, vnicID, forceNew)` -- create IPv6 with optional detach-existing

**`internal/oci/network.go`:**
- `GetPrimaryVnic(ctx, c, instanceID, compartmentID)` -- finds primary VNIC via `isPrimary` flag

**`internal/oci/provider.go`:**
- `Clients` struct with `Compute`, `Vcn` fields already initialized (line 76-84)

---

## Task 1: Add NLB Client to `Clients` Struct

**File:** `internal/oci/provider.go` (modify)

### 1a. Add import

```go
"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
```

### 1b. Extend `Clients` struct (line 76)

Add field:
```go
NLB *networkloadbalancer.NetworkLoadBalancerClient
```

### 1c. Extend `NewClients` function

After the `audit` client construction (line 115), add:
```go
nlbClient, err := networkloadbalancer.NewNetworkLoadBalancerClientWithConfigurationProvider(p)
if err != nil {
    return Clients{}, fmt.Errorf("nlb client: %w", err)
}
```

And include `NLB: &nlbClient` in the return statement.

### 1d. Check `go.mod`

Verify `github.com/oracle/oci-go-sdk/v65/networkloadbalancer` is in `go.mod`. If not:
```bash
go get github.com/oracle/oci-go-sdk/v65/networkloadbalancer
```

---

## Task 2: Expand `internal/oci/vnic.go` -- VNIC Batch Operations

**File:** `internal/oci/vnic.go` (modify)

**Pattern:** Follow existing functions in this file. Functions take `(ctx context.Context, ...)` with explicit client parameters.

**Imports to add:**
```go
import (
    "math/rand"
    "time"
    // existing imports remain
)
```

### New types

```go
// BatchVnicCreationResult mirrors Java BatchVnicCreationResult.
type BatchVnicCreationResult struct {
    InstanceID                string              `json:"instanceId"`
    InstanceDisplayName       string              `json:"instanceDisplayName"`
    RequestedVnicCount        int                 `json:"requestedVnicCount"`
    RequestedIpv6CountPerVnic int                 `json:"requestedIpv6CountPerVnic"`
    SuccessfulVnicCount       int                 `json:"successfulVnicCount"`
    TotalIpv6Count            int                 `json:"totalIpv6Count"`
    VnicResults               []VnicCreationResult `json:"vnicResults"`
    AllSuccessful             bool                `json:"allSuccessful"`
    Summary                   string              `json:"summary"`
    TotalExecutionTimeMs      int64               `json:"totalExecutionTimeMs"`
}

// VnicCreationResult mirrors Java VnicCreationResult.
type VnicCreationResult struct {
    VnicID          string   `json:"vnicId"`
    VnicDisplayName string   `json:"vnicDisplayName"`
    PrivateIP       string   `json:"privateIp"`
    PublicIP        string   `json:"publicIp"`
    SubnetID        string   `json:"subnetId"`
    AttachmentID    string   `json:"attachmentId"`
    LifecycleState  string   `json:"lifecycleState"`
    Ipv6Addresses   []string `json:"ipv6Addresses"`
    Ipv6IDs         []string `json:"ipv6Ids"`
    IsPrimary       bool     `json:"isPrimary"`
    Success         bool     `json:"success"`
    ErrorMessage    string   `json:"errorMessage,omitempty"`
}

// Ipv6CreationResult mirrors Java Ipv6CreationResult.
type Ipv6CreationResult struct {
    Ipv6ID       string `json:"ipv6Id"`
    Ipv6Address  string `json:"ipv6Address"`
    VnicID       string `json:"vnicId"`
    Success      bool   `json:"success"`
    ErrorMessage string `json:"errorMessage,omitempty"`
}
```

### New functions

```go
// CreateMultipleVnicsWithIpv6 creates batch VNICs on an instance, each with IPv6.
// Parity with Java VnicManagementUtils.createMultipleVnicsWithIpv6.
func CreateMultipleVnicsWithIpv6(ctx context.Context, c Clients,
    instanceID, subnetID string, vnicCount, ipv6CountPerVnic int,
) (*BatchVnicCreationResult, error)

// CreateSingleVnicWithIpv6 creates one secondary VNIC with IPv6 addresses.
// Called in a loop by CreateMultipleVnicsWithIpv6.
func CreateSingleVnicWithIpv6(ctx context.Context, c Clients,
    instanceID, subnetID, displayName string, ipv6Count int, subnetSupportsIpv6 bool,
) (*VnicCreationResult, error)

// CreateIpv6ForVnic creates multiple IPv6 addresses on an existing VNIC.
func CreateIpv6ForVnic(ctx context.Context, vcnClient *core.VirtualNetworkClient,
    vnicID string, count int,
) ([]Ipv6CreationResult, error)

// DeleteVnicWithIpv6 deletes a single secondary VNIC and all its IPv6 addresses.
// Returns true on success. Blocks primary VNIC deletion.
func DeleteVnicWithIpv6(ctx context.Context, c Clients,
    compartmentID, instanceID, vnicID string,
) (bool, error)

// DeleteAllIpv6FromVnic removes all IPv6 addresses from a VNIC.
func DeleteAllIpv6FromVnic(ctx context.Context, vcnClient *core.VirtualNetworkClient,
    vnicID string,
) (bool, error)

// DetachVnicFromInstance detaches a VNIC from an instance and polls until DETACHED.
func DetachVnicFromInstance(ctx context.Context, computeClient *core.ComputeClient,
    instanceID, vnicID string,
) (bool, error)

// DeleteAllSecondaryVnics deletes all non-primary VNICs on an instance.
// Returns a map of vnicID -> success.
func DeleteAllSecondaryVnics(ctx context.Context, c Clients,
    instanceID, compartmentID string,
) (map[string]bool, error)

// CheckSubnetIpv6Support returns true if the subnet has IPv6 CIDR blocks.
func CheckSubnetIpv6Support(ctx context.Context, vcnClient *core.VirtualNetworkClient,
    subnetID string,
) (bool, error)

// IsPrimaryVnic checks if a VNIC is the primary VNIC of an instance.
// Uses earliest timeCreated attachment (Java parity).
func IsPrimaryVnic(ctx context.Context, computeClient *core.ComputeClient,
    compartmentID, instanceID, vnicID string,
) bool

// ValidateVnicCreationParams validates vnicCount in [1,32] and ipv6CountPerVnic in [0,32].
func ValidateVnicCreationParams(vnicCount, ipv6CountPerVnic int) error

// WaitForVnicAttachment polls GetVnicAttachment until ATTACHED or timeout.
func WaitForVnicAttachment(ctx context.Context, computeClient *core.ComputeClient,
    attachmentID string, timeout, interval time.Duration,
) (*core.VnicAttachment, error)

// WaitForVnicDetachment polls GetVnicAttachment until DETACHED/NotFound or timeout.
func WaitForVnicDetachment(ctx context.Context, computeClient *core.ComputeClient,
    attachmentID string, timeout, interval time.Duration,
) (bool, error)
```

### Internal helpers

```go
// generateHostnameLabel generates "oci-start-hn" + 6 random lowercase letters.
func generateHostnameLabel() string

// generateVnicDisplayName generates "vnic-{instanceDisplayName}-{index}".
func generateVnicDisplayName(instanceName string, index int) string
```

### Key implementation notes

- **Primary VNIC detection:** Find the attachment with the earliest `timeCreated`. This is the Java approach (SPEC section 5). The existing Go `GetPrimaryVnic` uses `vnic.IsPrimary` as a first pass, which is acceptable but the Java approach is more reliable. Use earliest `timeCreated` as the definitive check in `IsPrimaryVnic`.
- **VNIC creation polling:** `WaitForVnicAttachment` polls `GetVnicAttachment` with 300s timeout, 3s interval. If `NotFound`, treat as "not yet created" and continue polling.
- **VNIC detachment polling:** `WaitForVnicDetachment` polls until `DETACHED`. If `NotFound`, treat as successful deletion.
- **Batch failure handling:** If one VNIC fails in `CreateMultipleVnicsWithIpv6`, set `allSuccessful = false` and return immediately (do not continue to next VNIC -- Java parity per SPEC section 9.4).
- **Subnet IPv6 check:** If `CheckSubnetIpv6Support` returns false, set `ipv6CountPerVnic = 0` (silently skip IPv6).

---

## Task 3: Expand `internal/oci/network.go` -- Network Configuration

**File:** `internal/oci/network.go` (modify)

**Imports to add:**
```go
// No new imports needed beyond existing core/common.
```

### New types

```go
// NetworkConfigResult mirrors Java OciNetworkUtils.NetworkConfigResult.
type NetworkConfigResult struct {
    Success                 bool     `json:"success"`
    Message                 string   `json:"message,omitempty"`
    ErrorMessage            string   `json:"errorMessage,omitempty"`
    NatGatewayID            string   `json:"natGatewayId,omitempty"`
    NatGatewayName          string   `json:"natGatewayName,omitempty"`
    RouteTableID            string   `json:"routeTableId,omitempty"`
    RouteTableName          string   `json:"routeTableName,omitempty"`
    RouteTableUpdated       bool     `json:"routeTableUpdated"`
    LoadBalancerCreated     bool     `json:"loadBalancerCreated"`
    NetworkLoadBalancerID   string   `json:"networkLoadBalancerId,omitempty"`
    NetworkLoadBalancerName string   `json:"networkLoadBalancerName,omitempty"`
    IPAddresses             []string `json:"ipAddresses,omitempty"`
}
```

### New functions

```go
// ConfigureInstanceNetwork orchestrates NAT gateway + route table + NLB setup.
// Parity with Java OciNetworkUtils.configureInstanceNetwork.
func ConfigureInstanceNetwork(ctx context.Context, c Clients,
    instanceID, vcnID, subnetID string, createLoadBalancer bool,
) (*NetworkConfigResult, error)

// CreateOrGetNatGateway finds or creates a NAT gateway by display name.
func CreateOrGetNatGateway(ctx context.Context, vcnClient *core.VirtualNetworkClient,
    compartmentID, vcnID, displayName string,
) (*core.NatGateway, error)

// CreateOrGetNatRouteTable finds or creates a route table with NAT gateway route.
func CreateOrGetNatRouteTable(ctx context.Context, vcnClient *core.VirtualNetworkClient,
    compartmentID, vcnID, natGatewayID, displayName string,
) (*core.RouteTable, error)

// UpdateInstanceVnicRouteTable updates the primary VNIC's route table.
func UpdateInstanceVnicRouteTable(ctx context.Context, c Clients,
    instanceID, routeTableID string,
) error

// ResetVnicToDefaultRouteTable resets a VNIC's route table to the VCN default.
func ResetVnicToDefaultRouteTable(ctx context.Context, c Clients,
    instanceID, compartmentID string,
) error

// DeleteNatGateway deletes a NAT gateway by ID.
func DeleteNatGateway(ctx context.Context, vcnClient *core.VirtualNetworkClient,
    natGatewayID string,
) error

// DeleteRouteTable deletes a route table by ID.
func DeleteRouteTable(ctx context.Context, vcnClient *core.VirtualNetworkClient,
    routeTableID string,
) error

// RestoreNetwork reverts load balancer configuration.
// Resets route table, deletes NLBs, deletes NAT gateways + associated route tables.
func RestoreNetwork(ctx context.Context, c Clients,
    instanceID, compartmentID string,
) error
```

### Key implementation notes

- **RestoreNetwork order** (SPEC section 9.12): Must delete in this order:
  1. Reset VNIC route table (detach from custom route table).
  2. Delete NLBs with display name containing `"amd"`.
  3. For each NAT gateway with display name containing `"amd"`: find associated route table, delete route table, delete NAT gateway.
- **NAT gateway naming:** All resources use display name `"amd"` (Java constant).
- **Route table rules:** Add `0.0.0.0/0 -> NAT gateway` rule.

---

## Task 4: New File `internal/oci/nlb.go` -- Network Load Balancer

**File:** `internal/oci/nlb.go` (new)

**Imports:**
```go
import (
    "context"
    "fmt"

    "github.com/oracle/oci-go-sdk/v65/common"
    "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)
```

**Functions:**

```go
// CreateOrGetNetworkLoadBalancer finds or creates an NLB with backend set + listener.
// Parity with Java OciNetworkUtils.createNetworkLoadBalancer.
//
// Configuration:
//   - Backend set "amd-1": targetId = instance OCID, port 22
//   - Health check: TCP port 22, interval 10s, timeout 3s, retries 3
//   - Listener "amd": TCP+UDP port 22
//   - Policy: FiveTuple
func CreateOrGetNetworkLoadBalancer(ctx context.Context,
    nlbClient *networkloadbalancer.NetworkLoadBalancerClient,
    compartmentID, subnetID, instanceID, displayName, privateIP string,
) (*networkloadbalancer.NetworkLoadBalancer, error)

// DeleteNetworkLoadBalancer deletes an NLB by OCID.
func DeleteNetworkLoadBalancer(ctx context.Context,
    nlbClient *networkloadbalancer.NetworkLoadBalancerClient,
    nlbID string,
) error

// ListNetworkLoadBalancers lists all NLBs in a compartment.
func ListNetworkLoadBalancers(ctx context.Context,
    nlbClient *networkloadbalancer.NetworkLoadBalancerClient,
    compartmentID string,
) ([]networkloadbalancer.NetworkLoadBalancer, error)

// WaitForNLBCreation polls until the NLB is ACTIVE.
func WaitForNLBCreation(ctx context.Context,
    nlbClient *networkloadbalancer.NetworkLoadBalancerClient,
    nlbID string, timeout, interval time.Duration,
) (*networkloadbalancer.NetworkLoadBalancer, error)
```

### Key implementation notes

- **Backend configuration:** Use `targetId` = instance OCID (NOT VNIC ID) per SPEC section 9.11.
- **Health check:** TCP port 22, interval 10s, timeout 3s, retries 3.
- **Listener:** Protocol `TcpAndUdp`, port 22.
- **NLB SDK methods:**
  - `CreateNetworkLoadBalancer` -> `CreateBackendSet` -> `CreateBackend` -> `CreateListener`
  - Or use `CreateNetworkLoadBalancer` with all details in one call if the SDK supports it.

---

## Task 5: New File `internal/oci/compute.go` -- Instance Reset

**File:** `internal/oci/compute.go` (new)

This file exists? Check first. If `internal/oci/compute.go` already has compute helpers, add to it. Otherwise create new.

**Function:**

```go
// ResetInstance stops then starts an instance. Required for IPv6 addresses to take effect.
// Parity with Java OciUtils.resetInstance.
func ResetInstance(ctx context.Context, computeClient *core.ComputeClient, instanceID string) error {
    // 1. InstanceAction: STOP
    // 2. Poll until state == STOPPED
    // 3. InstanceAction: START
    // 4. Poll until state == RUNNING
}
```

**SDK calls:**
```go
computeClient.InstanceAction(ctx, core.InstanceActionRequest{
    InstanceId: common.String(instanceID),
    Action:     core.InstanceActionActionStop,
})
// Poll GetInstance until LifecycleStateStopped

computeClient.InstanceAction(ctx, core.InstanceActionRequest{
    InstanceId: common.String(instanceID),
    Action:     core.InstanceActionActionStart,
})
// Poll GetInstance until LifecycleStateRunning
```

---

## Task 6: Service Layer -- VnicManagementService

**File:** `internal/service/vnic_management.go` (new)

**Pattern:** Follow `internal/service/tenant.go`. Accepts `*db.Store` and `*oci.ProxyPool` (or `MasterKey`). Resolves tenant from `instanceId` via `instance_detail` table.

```go
package service

type VnicManagementService struct {
    store     *db.Store
    masterKey []byte
    pool      *oci.ProxyPool
}

func NewVnicManagementService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *VnicManagementService
```

**Methods:**

```go
// LoadData returns all VNIC info for an instance (primary + secondary + statistics).
func (s *VnicManagementService) LoadData(ctx context.Context, instanceID string) (*VnicLoadDataResult, error)

// CreateVnics batch-creates secondary VNICs with IPv6.
func (s *VnicManagementService) CreateVnics(ctx context.Context, instanceID, subnetID string,
    vnicCount, ipv6CountPerVnic int) (*oci.BatchVnicCreationResult, error)

// DeleteVnic deletes a single secondary VNIC and its IPv6 addresses.
func (s *VnicManagementService) DeleteVnic(ctx context.Context, instanceID, vnicID string) error

// CreateIpv6 creates IPv6 addresses on an existing VNIC, then resets the instance.
func (s *VnicManagementService) CreateIpv6(ctx context.Context, instanceID, vnicID string,
    ipv6Count int) ([]oci.Ipv6CreationResult, error)

// DeleteIpv6 deletes a single IPv6 address by its address string.
func (s *VnicManagementService) DeleteIpv6(ctx context.Context, instanceID, vnicID, ipv6Address string) error

// DeleteAllSecondary deletes all non-primary VNICs on an instance.
func (s *VnicManagementService) DeleteAllSecondary(ctx context.Context, instanceID string) (map[string]bool, error)

// RefreshVnicInfo returns current VNIC data (same as LoadData, for polling).
func (s *VnicManagementService) RefreshVnicInfo(ctx context.Context, instanceID string) (*VnicLoadDataResult, error)

// ChangeSpecIp switches a VNIC's public IP until it falls within specified CIDR ranges.
// Non-blocking: runs in a background goroutine.
func (s *VnicManagementService) ChangeSpecIp(ctx context.Context, instanceID, vnicID string,
    cidrRanges []string) (*IpSwitchResult, error)

// ConfigureLoadBalancer sets up NAT GW + route table + NLB for an instance.
func (s *VnicManagementService) ConfigureLoadBalancer(ctx context.Context, instanceID string) (*oci.NetworkConfigResult, error)

// RestoreNetwork reverts load balancer configuration.
func (s *VnicManagementService) RestoreNetwork(ctx context.Context, instanceID string) error
```

**Response types:**

```go
type VnicLoadDataResult struct {
    VnicList      []VnicInfo   `json:"vnicList"`
    PrimaryVnic   *VnicInfo    `json:"primaryVnic"`
    SecondaryVnics []VnicInfo  `json:"secondaryVnics"`
    Statistics    VnicStats    `json:"statistics"`
    TenantID      string       `json:"tenantId"`
}

type VnicInfo struct {
    VnicID          string   `json:"vnicId"`
    VnicDisplayName string   `json:"vnicDisplayName"`
    PrivateIP       string   `json:"privateIp"`
    PublicIP        string   `json:"publicIp"`
    SubnetID        string   `json:"subnetId"`
    AttachmentID    string   `json:"attachmentId"`
    LifecycleState  string   `json:"lifecycleState"`
    IsPrimary       bool     `json:"isPrimary"`
    Ipv6Addresses   []string `json:"ipv6Addresses"`
    Ipv6IDs         []string `json:"ipv6Ids"`
    Success         bool     `json:"success"`
    ErrorMessage    string   `json:"errorMessage,omitempty"`
    CreatedAt       string   `json:"createdAt"`
    InstanceID      string   `json:"instanceId"`
    InstanceName    string   `json:"instanceName"`
}

type VnicStats struct {
    TotalVnicCount    int `json:"totalVnicCount"`
    ActiveVnicCount   int `json:"activeVnicCount"`
    SecondaryVnicCount int `json:"secondaryVnicCount"`
    TotalIpv6Count    int `json:"totalIpv6Count"`
    PrimaryIpv6Count  int `json:"primaryIpv6Count"`
}

type IpSwitchResult struct {
    OldIP string `json:"oldIp"`
    NewIP string `json:"newIp"`
}
```

**Internal helpers:**

```go
// resolveTenantFromInstanceID resolves instanceId -> InstanceDetails -> Tenant -> Credentials -> Clients.
func (s *VnicManagementService) resolveTenantFromInstanceID(ctx context.Context, instanceID string,
) (*repo.Tenant, *repo.InstanceDetail, oci.Credentials, oci.Clients, error)

// ipSwitchTasks prevents concurrent IP switches on the same instance.
var ipSwitchTasks sync.Map

func tryAcquireIPSwitch(instanceID string) bool {
    _, loaded := ipSwitchTasks.LoadOrStore(instanceID, struct{}{})
    return !loaded
}

func releaseIPSwitch(instanceID string) {
    ipSwitchTasks.Delete(instanceID)
}
```

**Key implementation notes:**

- **Tenant resolution** (SPEC section 9.2): Every handler resolves `instanceId` -> `instance_detail` -> `tenant_id` -> `tenant` -> OCI `Credentials` -> `Clients`. If tenant not found, return error "找不到对应的租户信息".
- **Primary VNIC detection** (SPEC section 5): Use earliest `timeCreated` attachment. The existing Go `GetPrimaryVnic` uses `isPrimary` flag first (acceptable fallback).
- **DeleteIpv6** (SPEC section 9.7): Receives IPv6 address string, not OCID. Must `ListIpv6s`, find matching address, extract OCID, then `DeleteIpv6`.
- **Instance reset** (SPEC section 9.8): After `CreateIpv6`, call `ResetInstance` (stop + start).
- **IP switch concurrency** (SPEC section 9.9): Use `sync.Map` guard. The retry loop runs 60-80 second random intervals.
- **Load balancer account gate** (SPEC section 9.10): Check `accountType` is `TRIAL_PAID_ACCOUNT` or `UPGRADE_ACCOUNT`, and `architecture` is `AMD`. Otherwise return 400 "当前租户不支持".

---

## Task 7: HTTP Handlers

**File:** `internal/httpapi/vnic_management.go` (new)

**Pattern:** Follow `internal/httpapi/tenant.go` -- factory function returns `gin.HandlerFunc`, closes over `*Deps`.

**Imports:**
```go
import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/Muione/oci-start-go/internal/response"
)
```

**Handler functions (10 total):**

```go
// GET /oci/vnic/loadData?instanceId=
func vnicLoadData(deps *Deps) gin.HandlerFunc

// POST /oci/vnic/create
func vnicCreate(deps *Deps) gin.HandlerFunc

// POST /oci/vnic/delete
func vnicDelete(deps *Deps) gin.HandlerFunc

// POST /oci/vnic/createIpv6
func vnicCreateIpv6(deps *Deps) gin.HandlerFunc

// POST /oci/vnic/deleteIpv6
func vnicDeleteIpv6(deps *Deps) gin.HandlerFunc

// POST /oci/vnic/deleteAllSecondary
func vnicDeleteAllSecondary(deps *Deps) gin.HandlerFunc

// GET /oci/vnic/refresh?instanceId=
func vnicRefresh(deps *Deps) gin.HandlerFunc

// POST /oci/vnic/changeSpecIp
func vnicChangeSpecIp(deps *Deps) gin.HandlerFunc

// POST /oci/vnic/network/configureLoadBalancer
func vnicConfigureLB(deps *Deps) gin.HandlerFunc

// POST /oci/vnic/network/restoreNetwork
func vnicRestoreNetwork(deps *Deps) gin.HandlerFunc
```

**Request structs:**

```go
type vnicCreateReq struct {
    InstanceID         string `json:"instanceId"`
    SubnetID           string `json:"subnetId"`
    VnicCount          int    `json:"vnicCount"`
    Ipv6CountPerVnic   int    `json:"ipv6CountPerVnic"`
}

type vnicDeleteReq struct {
    InstanceID string `json:"instanceId"`
    VnicID     string `json:"vnicId"`
}

type vnicCreateIpv6Req struct {
    VnicID     string `json:"vnicId"`
    Ipv6Count  int    `json:"ipv6Count"`
    InstanceID string `json:"instanceId"`
}

type vnicDeleteIpv6Req struct {
    Ipv6Address string `json:"ipv6Address"`
    VnicID      string `json:"vnicId"`
    InstanceID  string `json:"instanceId"`
}

type vnicDeleteAllReq struct {
    InstanceID string `json:"instanceId"`
}

type vnicChangeSpecIpReq struct {
    InstanceID string   `json:"instanceId"`
    VnicID     string   `json:"vnicId"`
    CidrRanges []string `json:"cidrRanges"`
}

type vnicLBReq struct {
    InstanceID string `json:"instanceId"`
}
```

**Handler response patterns:**

- `vnicLoadData` / `vnicRefresh`: `response.OK(c, response.SuccessMsgData("数据加载成功", result))`
- `vnicCreate`: `response.OK(c, response.SuccessMsg(result.Summary))` -- note: HTTP 200 even on partial failure, `success` field in JSON indicates outcome
- `vnicDelete`: `response.OK(c, response.SuccessMsg("VNIC删除成功"))`
- `vnicCreateIpv6`: `response.OK(c, response.SuccessMsgData("IPv6地址创建完成", results))`
- `vnicDeleteIpv6`: `response.OK(c, response.SuccessMsg("IPv6地址删除成功"))`
- `vnicDeleteAllSecondary`: `response.OK(c, response.SuccessMsgData("辅助VNIC删除完成", resultMap))`
- `vnicChangeSpecIp`: `response.OK(c, gin.H{"status": "success", "message": "IP切换成功", "details": result})`
- `vnicConfigureLB`: `response.OK(c, response.SuccessMsgData("实例网络配置完成", result))`
- `vnicRestoreNetwork`: `response.OK(c, response.SuccessMsg("网络配置已成功还原到原始状态"))`

**Validation:**
- All handlers: check required fields non-empty, return 400 "参数不完整" if missing.
- `vnicCreate`: validate `vnicCount` in [1,32], `ipv6CountPerVnic` in [0,32].
- `vnicCreateIpv6`: validate `ipv6Count` in [1,32].
- `vnicConfigureLB` / `vnicRestoreNetwork`: account type and architecture gate.

---

## Task 8: Dependency Injection and Route Registration

### 8a. Update `internal/httpapi/deps.go`

Add to the `Deps` struct:

```go
// Phase 11.2: VNIC Management.
VnicMgmtSvc *service.VnicManagementService
```

### 8b. Update `internal/httpapi/server.go`

Add the 10 VNIC routes under the `pro` group:

```go
// Phase 11.2: VNIC Management.
pro.GET("/oci/vnic/loadData", vnicLoadData(deps))
pro.POST("/oci/vnic/create", vnicCreate(deps))
pro.POST("/oci/vnic/delete", vnicDelete(deps))
pro.POST("/oci/vnic/createIpv6", vnicCreateIpv6(deps))
pro.POST("/oci/vnic/deleteIpv6", vnicDeleteIpv6(deps))
pro.POST("/oci/vnic/deleteAllSecondary", vnicDeleteAllSecondary(deps))
pro.GET("/oci/vnic/refresh", vnicRefresh(deps))
pro.POST("/oci/vnic/changeSpecIp", vnicChangeSpecIp(deps))
pro.POST("/oci/vnic/network/configureLoadBalancer", vnicConfigureLB(deps))
pro.POST("/oci/vnic/network/restoreNetwork", vnicRestoreNetwork(deps))
```

### 8c. Wire `VnicManagementService` in `main.go`

```go
vnicMgmtSvc := service.NewVnicManagementService(store, masterKey, proxyPool)
```

Pass to `Deps`.

---

## Task 9: Frontend -- VnicManagement.vue Page

**File:** `frontend/src/views/VnicManagement.vue` (new)

Alternatively, this could be a dialog component launched from `Instances.vue` when clicking a "Manage VNICs" button on an instance row.

**Features:**
- Load VNIC data for an instance (triggered from Instances page action button).
- Display primary VNIC info (IP, subnet, IPv6 addresses).
- Display secondary VNICs in a table with columns: VNIC ID, display name, private IP, public IP, IPv6 count, state, actions.
- Statistics panel: total VNICs, active VNICs, secondary VNICs, total IPv6 count.
- **Create VNICs:** Dialog with subnet selector, VNIC count slider (1-32), IPv6 per VNIC slider (0-32). Shows batch result with success/failure per VNIC.
- **Delete VNIC:** Confirmation dialog, shows progress.
- **Delete All Secondary:** Confirmation dialog with warning.
- **Manage IPv6 per VNIC:** Expand row or sub-dialog showing IPv6 addresses with add/delete buttons.
  - Add IPv6: count input, warning about instance restart.
  - Delete IPv6: click to delete individual address.
- **IP Switch:** Dialog with CIDR range inputs, shows retry progress.
- **Configure Load Balancer:** One-click button, shows result details.
- **Restore Network:** One-click button with confirmation.

**Pattern:** Follow existing views like `Instances.vue` for table layout and `Tenants.vue` for dialog patterns.

---

## Task 10: Frontend -- Router Registration

**File:** `frontend/src/router/index.ts` (modify)

Option A -- Standalone page:
```ts
{ path: 'vnic/:instanceId', name: 'vnic-management', component: () => import('../views/VnicManagement.vue') },
```

Option B -- Dialog from Instances page (no router change, use component import in Instances.vue).

Recommended: Option A for deep-linking support.

---

## Test Checklist

### VNIC Batch Operations

- [ ] Create 1 VNIC with 0 IPv6 -- verify VNIC attached, no IPv6 assigned
- [ ] Create 1 VNIC with 4 IPv6 -- verify 4 IPv6 addresses created
- [ ] Create 5 VNICs with 32 IPv6 each -- verify batch completes (160 IPv6 total)
- [ ] VNIC count validation: 0 returns 400, 33 returns 400
- [ ] IPv6 count validation: -1 returns 400, 33 returns 400
- [ ] Missing `instanceId` returns 400 "参数不完整"
- [ ] Missing tenant for instanceId returns 400 "找不到对应的租户信息"

### VNIC Deletion

- [ ] Delete single secondary VNIC -- verify IPv6 cleanup + detachment
- [ ] Delete primary VNIC attempt -- verify blocked (error returned)
- [ ] Delete all secondary VNICs -- verify primary preserved, secondaries removed
- [ ] VNIC detach timeout (300s) -- verify error returned

### IPv6 Management

- [ ] Create IPv6 on existing VNIC -- verify instance reset triggered (stop+start)
- [ ] Create IPv6 count validation: 0 returns 400, 33 returns 400
- [ ] Delete IPv6 by address string -- verify correct IPv6 removed
- [ ] Delete IPv6 with non-existent address -- verify graceful handling

### Load Balancer

- [ ] Configure load balancer -- verify NAT GW + route table + NLB created
- [ ] Configure on non-AMD architecture -- verify 400 "当前租户不支持"
- [ ] Configure on wrong account type -- verify 400
- [ ] Restore network -- verify all resources cleaned up in correct order
- [ ] Restore network order: VNIC route table first, then NLBs, then NAT route table, then NAT GW

### IP Switch

- [ ] IP switch with CIDR range -- verify retry loop and IP in range
- [ ] Concurrent IP switch on same instance -- verify second request rejected
- [ ] IP switch with no matching CIDR -- verify eventual timeout/error

### Edge Cases

- [ ] Subnet without IPv6 support -- verify graceful fallback (IPv6 skipped, warning logged)
- [ ] Instance not found -- verify error propagation
- [ ] OCI API rate limiting -- verify error handling
- [ ] Partial batch failure -- verify `allSuccessful: false` in response, HTTP still 200

### Frontend

- [ ] VNIC management page loads from instance action button
- [ ] VNIC table displays primary and secondary VNICs
- [ ] Statistics panel shows correct counts
- [ ] Create VNIC dialog validates inputs
- [ ] Create VNIC shows batch result with per-VNIC status
- [ ] Delete VNIC with confirmation
- [ ] IPv6 management per VNIC
- [ ] IP switch dialog with CIDR inputs
- [ ] Configure/Restore load balancer buttons
