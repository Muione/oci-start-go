# Phase 11.2 — VNIC Management API SPEC

> **Source**: Java `oci-start` — `VnicManagementController`, `VnicService`, `VnicManagementUtils`, `OciNetworkUtils`, `OciIpv6Utils`, `VnicServiceImpl`
> **Go target**: `internal/oci/vnic.go`, `internal/httpapi/` handlers
> **Date**: 2026-06-28

---

## 1. Overview

The VNIC Management feature provides batch VNIC creation/deletion with IPv6 address assignment, single IPv6 CRUD, load balancer configuration, and network restoration. All operations are scoped to a single OCI instance identified by `instanceId`, which is resolved to a tenant (credentials) via the `instance_details` DB table.

### Key Constants (from Java)

| Constant | Value | Description |
|---|---|---|
| `MAX_VNIC_PER_INSTANCE` | 32 | Max VNICs per instance (including primary) |
| `MAX_IPV6_PER_VNIC` | 32 | Max IPv6 addresses per VNIC |
| `DEFAULT_TIMEOUT_SECONDS` | 300 | Polling timeout for attach/detach |
| `POLL_INTERVAL_SECONDS` | 3 | Poll interval for VNIC attachment state |
| `subnetName` | `"oci-start-pro-subnet"` | Display name for auto-created subnets |

---

## 2. API Endpoints

All endpoints are under `/oci/vnic`. The Java controller uses `@Controller` with `@ResponseBody` on each handler. Go routes should be registered under a protected group.

### 2.1 Load VNIC Data

```
GET /oci/vnic/loadData?instanceId={instanceId}
```

**Purpose**: Load all VNIC information for an instance, split into primary and secondary VNICs with statistics.

**Request**: Query parameter `instanceId` (OCI instance OCID).

**Response (200)**:
```json
{
  "success": true,
  "data": {
    "vnicList": [
      {
        "vnicId": "ocid1.vnic.oc1...",
        "vnicDisplayName": "primary-vnic",
        "privateIp": "10.0.0.10",
        "publicIp": "129.146.x.x",
        "subnetId": "ocid1.subnet.oc1...",
        "attachmentId": "ocid1.vnicattachment.oc1...",
        "lifecycleState": "ATTACHED",
        "isPrimary": true,
        "ipv6Addresses": ["2603:c021:..."],
        "ipv6Ids": ["ocid1.ipv6.oc1..."],
        "success": true,
        "errorMessage": null,
        "createdAt": "2025-01-20T10:00:00Z",
        "instanceId": "ocid1.instance.oc1...",
        "instanceName": "instance-1"
      }
    ],
    "primaryVnic": { ... },
    "secondaryVnics": [ ... ],
    "statistics": {
      "totalVnicCount": 5,
      "activeVnicCount": 5,
      "secondaryVnicCount": 4,
      "totalIpv6Count": 20,
      "primaryIpv6Count": 4
    },
    "tenantId": "123"
  },
  "message": "数据加载成功"
}
```

**Error (400)**: `{ "success": false, "message": "找不到对应的租户信息" }`

---

### 2.2 Create VNICs (Batch)

```
POST /oci/vnic/create
Content-Type: application/json
```

**Purpose**: Create multiple secondary VNICs on an instance, each with a configurable number of IPv6 addresses. Optionally creates a new subnet if `isCreateSubnet` is true (controller passes `false`; Java internal callers can pass `true`).

**Request Body**:
```json
{
  "instanceId": "ocid1.instance.oc1...",
  "subnetId": "ocid1.subnet.oc1...",
  "vnicCount": 5,
  "ipv6CountPerVnic": 4
}
```

| Field | Type | Required | Constraints |
|---|---|---|---|
| `instanceId` | string | yes | OCI instance OCID |
| `subnetId` | string | yes | Target subnet OCID |
| `vnicCount` | int | yes | 1..32 |
| `ipv6CountPerVnic` | int | yes | 0..32 |

**Response (200 — all successful)**:
```json
{
  "success": true,
  "message": "实例 ocid1... 的VNIC创建完成 - 成功: 5/5, IPv6地址: 20个, 耗时: 45000ms",
  "details": {
    "instanceId": "ocid1.instance.oc1...",
    "instanceDisplayName": "my-instance",
    "requestedVnicCount": 5,
    "requestedIpv6CountPerVnic": 4,
    "successfulVnicCount": 5,
    "totalIpv6Count": 20,
    "vnicResults": [
      {
        "vnicId": "ocid1.vnic.oc1...",
        "vnicDisplayName": "vnic-my-instance-1",
        "privateIp": "10.0.0.11",
        "publicIp": "129.146.x.x",
        "subnetId": "ocid1.subnet.oc1...",
        "attachmentId": "ocid1.vnicattachment.oc1...",
        "lifecycleState": "ATTACHED",
        "isPrimary": false,
        "ipv6Addresses": ["2603:...", "2603:..."],
        "ipv6Ids": ["ocid1.ipv6.oc1...", "ocid1.ipv6.oc1..."],
        "success": true,
        "errorMessage": null
      }
    ],
    "allSuccessful": true,
    "summary": "...",
    "totalExecutionTimeMs": 45000
  }
}
```

**Error (400)**: `{ "success": false, "message": "参数不完整" }` or validation error.

**Response (200 — partial failure)**: Same structure but `allSuccessful: false`, `success: false` in the outer envelope, and individual `vnicResults[].success: false` entries with `errorMessage`.

---

### 2.3 Delete VNIC

```
POST /oci/vnic/delete
Content-Type: application/json
```

**Purpose**: Delete a single secondary VNIC and all its IPv6 addresses. Primary VNIC deletion is blocked.

**Request Body**:
```json
{
  "instanceId": "ocid1.instance.oc1...",
  "vnicId": "ocid1.vnic.oc1..."
}
```

**Response (200)**:
```json
{
  "success": true,
  "message": "VNIC删除成功"
}
```

**Deletion sequence**:
1. List all IPv6 addresses on the VNIC (`ListIpv6s`).
2. Delete each IPv6 address (`DeleteIpv6`).
3. Find the VNIC attachment (`ListVnicAttachments` with `instanceId` + `vnicId` filter).
4. Verify it is NOT the primary VNIC (earliest `timeCreated` attachment).
5. Detach the VNIC (`DetachVnic` using `vnicAttachmentId`).
6. Poll until `DETACHED` or timeout (300s, 3s interval).

---

### 2.4 Create IPv6 Addresses

```
POST /oci/vnic/createIpv6
Content-Type: application/json
```

**Purpose**: Create additional IPv6 addresses on an existing VNIC. After creation, the instance is reset (stopped+started) via `OciUtils.resetInstance`.

**Request Body**:
```json
{
  "vnicId": "ocid1.vnic.oc1...",
  "ipv6Count": 5,
  "instanceId": "ocid1.instance.oc1..."
}
```

| Field | Type | Required | Constraints |
|---|---|---|---|
| `vnicId` | string | yes | VNIC OCID |
| `ipv6Count` | int | yes | 1..32 |
| `instanceId` | string | yes | Used for tenant lookup and instance reset |

**Response (200)**:
```json
{
  "success": true,
  "message": "IPv6地址创建完成 - 成功: 5/5",
  "details": [
    {
      "ipv6Id": "ocid1.ipv6.oc1...",
      "ipv6Address": "2603:c021:...",
      "vnicId": "ocid1.vnic.oc1...",
      "success": true,
      "errorMessage": null,
      "createdAt": "2025-01-20T10:00:00Z"
    }
  ]
}
```

**Side effect**: Calls `OciUtils.resetInstance(tenant, instanceId)` after IPv6 creation.

---

### 2.5 Delete IPv6 Address

```
POST /oci/vnic/deleteIpv6
Content-Type: application/json
```

**Purpose**: Delete a single IPv6 address from a VNIC by its IP address string.

**Request Body**:
```json
{
  "ipv6Address": "2603:c021:...",
  "vnicId": "ocid1.vnic.oc1...",
  "instanceId": "ocid1.instance.oc1..."
}
```

**Response (200)**:
```json
{
  "success": true,
  "message": "IPv6地址删除成功"
}
```

**Logic**: Lists all IPv6 on the VNIC, finds the one matching `ipv6Address`, extracts its `ipv6Id`, then calls `DeleteIpv6`.

---

### 2.6 Delete All Secondary VNICs

```
POST /oci/vnic/deleteAllSecondary
Content-Type: application/json
```

**Purpose**: Delete all non-primary VNICs on an instance. Primary VNIC is skipped.

**Request Body**:
```json
{
  "instanceId": "ocid1.instance.oc1..."
}
```

**Response (200)**:
```json
{
  "success": true,
  "message": "辅助VNIC删除完成 - 成功: 4/4",
  "details": {
    "ocid1.vnic1...": true,
    "ocid1.vnic2...": true
  }
}
```

**Logic**:
1. Get all VNICs for the instance via `getInstanceVnics`.
2. For each VNIC, check if it is the primary (earliest `timeCreated` attachment).
3. Skip primary; delete each secondary via `deleteVnicWithIpv6` (IPv6 cleanup + detach).

---

### 2.7 Refresh VNIC Info

```
GET /oci/vnic/refresh?instanceId={instanceId}
```

**Purpose**: Same as `loadData` but without `tenantId` in the response. Used for polling/refresh from the UI.

**Response**: Same structure as `loadData` (2.1) minus `tenantId`.

---

### 2.8 Switch VNIC to Specific IP Range

```
POST /oci/vnic/changeSpecIp
Content-Type: application/json
```

**Purpose**: Repeatedly reassign the public IP of a VNIC until it falls within a user-specified CIDR range. Uses a background retry loop with 60-80 second random intervals between attempts.

**Request Body** (`IpVnicSwitchRequest`):
```json
{
  "instanceId": "ocid1.instance.oc1...",
  "vnicId": "ocid1.vnic.oc1...",
  "cidrRanges": ["129.146.0.0/16", "130.35.0.0/16"]
}
```

**Response (200)**:
```json
{
  "status": "success",
  "message": "IP切换成功",
  "details": {
    "oldIp": "129.146.1.100",
    "newIp": "129.146.2.50"
  }
}
```

**Error (400)**: `{ "status": "error", "message": "IP切换失败: ..." }`

**Concurrency guard**: Uses an in-memory map (`ipSwitchTasks`) keyed by `instanceId` to prevent concurrent IP switch tasks on the same instance.

**Side effect**: Updates `instance_details.public_ips` in the DB. Sends Telegram notification.

---

### 2.9 Configure Load Balancer

```
POST /oci/vnic/network/configureLoadBalancer
Content-Type: application/json
```

**Purpose**: One-click setup of NAT gateway + NAT route table + Network Load Balancer for an instance.

**Request Body**:
```json
{
  "instanceId": "ocid1.instance.oc1..."
}
```

**Preconditions**:
- Account type must be `TRIAL_PAID_ACCOUNT` or `UPGRADE_ACCOUNT`.
- Architecture must be `AMD`.

**Response (200)**:
```json
{
  "success": true,
  "message": "实例网络配置完成",
  "details": {
    "natGatewayId": "ocid1.natgateway.oc1...",
    "natGatewayName": "amd",
    "routeTableId": "ocid1.routetable.oc1...",
    "routeTableName": "amd",
    "networkLoadBalancerId": "ocid1.networkloadbalancer.oc1...",
    "networkLoadBalancerName": "amd",
    "nlpIpAddress": "129.146.x.x"
  }
}
```

**OCI operations sequence**:
1. Create or get NAT gateway named `"amd"` in the VCN.
2. Create or get route table named `"amd"` with `0.0.0.0/0 -> NAT gateway`.
3. Update the primary VNIC's route table to the new NAT route table.
4. Create a Network Load Balancer named `"amd"` with:
   - Backend set `"amd-1"` using the instance's primary private IP, port 22.
   - Listener `"amd"` on port 22, protocol `TcpAndUdp`.
   - Health check: TCP port 22, 10s interval, 3s timeout, 3 retries.
   - Policy: `FiveTuple`.

**Side effect**: Sends Telegram notification on success.

---

### 2.10 Restore Network

```
POST /oci/vnic/network/restoreNetwork
Content-Type: application/json
```

**Purpose**: Revert network configuration to pre-load-balancer state. Removes NAT gateway, custom route table, and NLB.

**Request Body**:
```json
{
  "instanceId": "ocid1.instance.oc1..."
}
```

**Preconditions**: Same account type/architecture check as configureLoadBalancer.

**Response (200)**:
```json
{
  "success": true,
  "message": "网络配置已成功还原到原始状态"
}
```

**Restoration sequence**:
1. Reset primary VNIC route table to a non-`"amd"` route table (or the VCN default).
2. Delete all NLBs with display name containing `"amd"`.
3. For each NAT gateway with display name containing `"amd"`:
   - Find/create its associated route table named `"amd"`.
   - Delete the route table.
   - Delete the NAT gateway.

**Side effect**: Sends Telegram notification.

---

## 3. OCI SDK Operations Reference

### 3.1 Compute Client (`core.ComputeClient`)

| Operation | SDK Method | Java Usage |
|---|---|---|
| List VNIC attachments | `ListVnicAttachments` | `getInstanceVnics`, `getPrimaryVnic`, `getSecondaryVnics`, `detachVnicFromInstance` |
| Get VNIC attachment | `GetVnicAttachment` | `waitForVnicAttachment`, `waitForVnicDetachment` (poll) |
| Attach VNIC | `AttachVnic` | `createSingleVnicWithIpv6` — attaches a new secondary VNIC |
| Detach VNIC | `DetachVnic` | `detachVnicFromInstance` — detaches secondary VNIC |
| Get instance | `GetInstance` | `getInstance` |

### 3.2 Virtual Network Client (`core.VirtualNetworkClient`)

| Operation | SDK Method | Java Usage |
|---|---|---|
| Get VNIC | `GetVnic` | `getInstanceVnics`, `createSingleVnicWithIpv6`, `updateInstanceVnicRouteTable` |
| Update VNIC | `UpdateVnic` | `updateInstanceVnicRouteTable`, `resetVnicToDefaultRouteTable` |
| List IPv6s | `ListIpv6s` | `getVnicIpv6Addresses`, `deleteAllIpv6FromVnic`, `deleteIpv6Address` |
| Create IPv6 | `CreateIpv6` | `createSingleIpv6`, `createIpv6ForVnic` |
| Delete IPv6 | `DeleteIpv6` | `deleteAllIpv6FromVnic`, `deleteIpv6Address` |
| Get Subnet | `GetSubnet` | `checkSubnetIpv6Support`, `getVcnIdFromSubnet` |
| List Subnets | `ListSubnets` | `doCreateSubnet` |
| Create Subnet | `CreateSubnet` | `doCreateSubnet` |
| List VCNs | `ListVcns` | `doCreateSubnet` (find VCN with IPv6) |
| List NAT Gateways | `ListNatGateways` | `createOrGetNatGateway` |
| Create NAT Gateway | `CreateNatGateway` | `createNatGateway` |
| Update NAT Gateway | `UpdateNatGateway` | `updateNatGatewayStatus` |
| Delete NAT Gateway | `DeleteNatGateway` | `deleteNatGateway` |
| List Route Tables | `ListRouteTables` | `createOrGetNatRouteTable`, `resetVnicToDefaultRouteTable` |
| Create Route Table | `CreateRouteTable` | `createNatRouteTable` |
| Update Route Table | `UpdateRouteTable` | `updateRouteTableForNat`, `ensureIpv6RouteRules` |
| Delete Route Table | `DeleteRouteTable` | `deleteRouteTable` |

### 3.3 NLB Client (`networkloadbalancer.NetworkLoadBalancerClient`)

| Operation | SDK Method | Java Usage |
|---|---|---|
| List NLBs | `ListNetworkLoadBalancers` | `listNetworkLoadBalancers` |
| Get NLB | `GetNetworkLoadBalancer` | `waitForNetworkLoadBalancerCreation`, `verifyNetworkLoadBalancerConfig` |
| Create NLB | `CreateNetworkLoadBalancer` | `createNetworkLoadBalancer` |
| Delete NLB | `DeleteNetworkLoadBalancer` | `deleteNetworkLoadBalancer` |

---

## 4. VNIC Creation Flow (Detailed)

The batch creation flow in `createMultipleVnicsWithIpv6` proceeds as follows:

```
1. [Optional] Create new subnet if isCreateSubnet=true
   a. List VCNs, find one with ipv6CidrBlocks
   b. List existing subnets, compute next available IPv4 CIDR (10.0.N.0/24)
   c. Compute next available IPv6 CIDR (/64 from VCN's /56)
   d. Create subnet with DNS label "subnet" + timestamp
   e. Wait for subnet AVAILABLE

2. Validate parameters: vnicCount in [1,32], ipv6CountPerVnic in [0,32]

3. Get instance via ComputeClient.GetInstance

4. Check subnet IPv6 support:
   a. GetSubnet -> check ipv6CidrBlocks non-empty
   b. If not supported, set ipv6CountPerVnic = 0

5. For each VNIC (i = 0..vnicCount-1):
   a. Generate display name: "vnic-{instanceDisplayName}-{i+1}"
   b. Generate hostname: "oci-start-hn" + 6 random lowercase letters
   c. Build CreateVnicDetails:
      - subnetId
      - displayName
      - assignPrivateDnsRecord: true
      - hostnameLabel: generated hostname
      - skipSourceDestCheck: false
      - assignPublicIp: true
   d. Build AttachVnicDetails with instanceId + createVnicDetails
   e. ComputeClient.AttachVnic
   f. Poll GetVnicAttachment until ATTACHED (300s timeout, 3s interval)
   g. GetVnic to retrieve privateIp, publicIp
   h. If ipv6Count > 0 && subnetSupportsIpv6:
      For each IPv6 (j = 0..ipv6Count-1):
        - CreateIpv6 with vnicId + displayName "ipv6-{j+1}"
        - Record ipv6Id and ipv6Address
   i. Record result

6. Return BatchVnicCreationResult
```

### VNIC Naming Convention

- VNIC display name: `vnic-{instanceDisplayName}-{index}` (e.g., `vnic-my-instance-1`)
- IPv6 display name: `ipv6-{index}` (e.g., `ipv6-1`)
- Hostname label: `oci-start-hn` + 6 random lowercase letters (e.g., `oci-start-hnabcdef`)

---

## 5. VNIC Deletion Flow (Detailed)

```
1. Delete all IPv6 addresses on the VNIC:
   a. ListIpv6s(vnicId)
   b. For each IPv6: DeleteIpv6(ipv6Id)

2. Detach VNIC from instance:
   a. ListVnicAttachments(instanceId, vnicId) to find attachmentId
   b. Check isPrimaryVnic — reject if primary
   c. DetachVnic(vnicAttachmentId)
   d. Poll GetVnicAttachment until DETACHED (300s timeout, 3s interval)
```

### Primary VNIC Detection

The Java code determines the primary VNIC by finding the attachment with the **earliest `timeCreated`** among all attachments for the instance. This is important — it does NOT rely on `vnic.isPrimary` for this check (though `isPrimary` is used in the VnicCreationResult response).

```java
VnicAttachment primaryAttachment = attachments.stream()
    .filter(att -> att.getTimeCreated() != null)
    .min(Comparator.comparing(VnicAttachment::getTimeCreated))
    .orElse(null);
return primaryAttachment != null && vnicId.equals(primaryAttachment.getVnicId());
```

---

## 6. Subnet Auto-Creation Flow

When `isCreateSubnet=true` is passed (not from the controller, but from internal callers):

```
1. List VCNs in the compartment, find one with non-empty ipv6CidrBlocks
2. List existing subnets in that VCN
3. Collect used IPv4 CIDRs (10.0.N.0/24 format) and IPv6 CIDRs
4. Find next available IPv4 CIDR: scan 10.0.1.0/24 through 10.0.255.0/24
5. Find next available IPv6 CIDR (/64):
   - For /56 VCN: modify 4th segment of the IPv6 address
   - For /48 VCN: append subnet number as new segment
6. Create subnet:
   - displayName: "oci-start-pro-subnet"
   - cidrBlock: next available IPv4
   - ipv6CidrBlock: next available IPv6
   - routeTableId: VCN default route table
   - dnsLabel: "subnet" + (timestamp % 100000)
7. Wait for subnet AVAILABLE
```

---

## 7. Database Schema

### 7.1 Existing Table: `instance_detail`

Relevant columns for VNIC management:

| Column | Type | Description |
|---|---|---|
| `id` | BIGINT (PK) | Auto-increment |
| `tenant_id` | BIGINT | FK to tenant table |
| `instance_id` | VARCHAR | OCI instance OCID |
| `display_name` | VARCHAR | Instance display name |
| `public_ips` | VARCHAR | Current public IP (updated by IP switch) |
| `private_ips` | VARCHAR | Private IP |
| `availability_domain` | VARCHAR | AD for the instance |
| `compartment_id` | VARCHAR | OCI compartment OCID |
| `ipv6_addresses` | TEXT | Comma-separated IPv6 addresses |
| `vnic_ids` | TEXT | Comma-separated VNIC OCIDs |

### 7.2 Existing Table: `instance_cloud_networks`

Stores network configuration mappings (VCN, subnet, etc.) per tenant/region.

| Column | Type | Description |
|---|---|---|
| `id` | BIGINT (PK) | Auto-increment |
| `tenant_id` | VARCHAR(100) | Tenant identifier |
| `vcn_id` | VARCHAR(100) | OCI VCN OCID |
| `vcn_name` | VARCHAR(255) | VCN display name |
| `subnet_id` | VARCHAR(100) | OCI subnet OCID |
| `subnet_name` | VARCHAR(255) | Subnet display name |
| `region` | VARCHAR(50) | Region name |
| `cidr_block` | VARCHAR(50) | Subnet CIDR |
| `net_work_security_group_id` | VARCHAR(128) | NSG OCID |
| `cloud_type` | INTEGER | Default 1 |
| `created_at` | DATETIME | Auto-set on create |
| `updated_at` | DATETIME | Auto-set on update |

### 7.3 No New Tables Required

The VNIC management feature does not introduce new database tables. VNIC state is always queried live from OCI APIs. The `instance_detail` table stores cached IPs and VNIC IDs for display purposes, but the source of truth is OCI.

---

## 8. Go Implementation Guidance

### 8.1 Existing Go Code to Reuse

The following Go functions already exist and should be reused:

- **`internal/oci/vnic.go`**:
  - `ListVnicAttachmentsForInstance` — paginated VNIC attachment listing
  - `GetVnicInfo` — resolves VNIC OCID to IP details
  - `ListAllVnicsForInstance` — combines attachments + VNIC info
  - `AssignIpv6ToVnic` — create IPv6 with optional force-new (detach existing first)

- **`internal/oci/network.go`**:
  - `GetPrimaryVnic` — finds primary VNIC via `isPrimary` flag (or first attachment fallback)
  - `ListVcns` — lists VCNs in a compartment
  - `ReassignPublicIP` — delete old reserved IP, create new ephemeral IP

- **`internal/oci/provider.go`**:
  - `Clients` struct with `Compute`, `Vcn` fields already initialized

### 8.2 New Functions to Implement

#### In `internal/oci/vnic.go` (expand existing file):

```go
// BatchVnicCreationResult mirrors Java BatchVnicCreationResult.
type BatchVnicCreationResult struct {
    InstanceID               string              `json:"instanceId"`
    InstanceDisplayName      string              `json:"instanceDisplayName"`
    RequestedVnicCount       int                 `json:"requestedVnicCount"`
    RequestedIpv6CountPerVnic int                `json:"requestedIpv6CountPerVnic"`
    SuccessfulVnicCount      int                 `json:"successfulVnicCount"`
    TotalIpv6Count           int                 `json:"totalIpv6Count"`
    VnicResults              []VnicCreationResult `json:"vnicResults"`
    AllSuccessful            bool                `json:"allSuccessful"`
    Summary                  string              `json:"summary"`
    TotalExecutionTimeMs     int64               `json:"totalExecutionTimeMs"`
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

// --- New functions ---

func CreateMultipleVnicsWithIpv6(ctx context.Context, c Clients, instanceID, subnetID string,
    vnicCount, ipv6CountPerVnic int) (*BatchVnicCreationResult, error)

func CreateSingleVnicWithIpv6(ctx context.Context, c Clients, instanceID, subnetID,
    displayName string, ipv6Count int, subnetSupportsIpv6 bool) (*VnicCreationResult, error)

func CreateIpv6ForVnic(ctx context.Context, vcnClient *core.VirtualNetworkClient,
    vnicID string, count int) ([]Ipv6CreationResult, error)

func DeleteVnicWithIpv6(ctx context.Context, c Clients, instanceID, vnicID string) (bool, error)

func DeleteAllIpv6FromVnic(ctx context.Context, vcnClient *core.VirtualNetworkClient,
    vnicID string) (bool, error)

func DetachVnicFromInstance(ctx context.Context, computeClient *core.ComputeClient,
    instanceID, vnicID string) (bool, error)

func DeleteAllSecondaryVnics(ctx context.Context, c Clients, instanceID,
    compartmentID string) (map[string]bool, error)

func CheckSubnetIpv6Support(ctx context.Context, vcnClient *core.VirtualNetworkClient,
    subnetID string) (bool, error)

func IsPrimaryVnic(ctx context.Context, computeClient *core.ComputeClient,
    compartmentID, instanceID, vnicID string) bool

func ValidateVnicCreationParams(vnicCount, ipv6CountPerVnic int) error

// Polling helpers
func WaitForVnicAttachment(ctx context.Context, computeClient *core.ComputeClient,
    attachmentID string, timeout, interval time.Duration) (*core.VnicAttachment, error)

func WaitForVnicDetachment(ctx context.Context, computeClient *core.ComputeClient,
    attachmentID string, timeout, interval time.Duration) (bool, error)
```

#### In `internal/oci/network.go` (expand existing file):

```go
// NetworkConfigResult mirrors Java OciNetworkUtils.NetworkConfigResult.
type NetworkConfigResult struct {
    Success                bool     `json:"success"`
    Message                string   `json:"message,omitempty"`
    ErrorMessage           string   `json:"errorMessage,omitempty"`
    NatGatewayID           string   `json:"natGatewayId,omitempty"`
    NatGatewayName         string   `json:"natGatewayName,omitempty"`
    RouteTableID           string   `json:"routeTableId,omitempty"`
    RouteTableName         string   `json:"routeTableName,omitempty"`
    RouteTableUpdated      bool     `json:"routeTableUpdated"`
    LoadBalancerCreated    bool     `json:"loadBalancerCreated"`
    NetworkLoadBalancerID  string   `json:"networkLoadBalancerId,omitempty"`
    NetworkLoadBalancerName string  `json:"networkLoadBalancerName,omitempty"`
    IPAddresses            []string `json:"ipAddresses,omitempty"`
}

func ConfigureInstanceNetwork(ctx context.Context, c Clients, instanceID, vcnID, subnetID string,
    createLoadBalancer bool) (*NetworkConfigResult, error)

func CreateOrGetNatGateway(ctx context.Context, vcnClient *core.VirtualNetworkClient,
    compartmentID, vcnID, displayName string) (*core.NatGateway, error)

func CreateOrGetNatRouteTable(ctx context.Context, vcnClient *core.VirtualNetworkClient,
    compartmentID, vcnID, natGatewayID, displayName string) (*core.RouteTable, error)

func UpdateInstanceVnicRouteTable(ctx context.Context, c Clients,
    instanceID, routeTableId string) error

func DeleteNatGateway(ctx context.Context, vcnClient *core.VirtualNetworkClient,
    natGatewayID string) error

func DeleteRouteTable(ctx context.Context, vcnClient *core.VirtualNetworkClient,
    routeTableID string) error
```

#### New file `internal/oci/nlb.go`:

```go
// Network Load Balancer operations (OCI NLB SDK).
// Uses github.com/oracle/oci-go-sdk/v65/networkloadbalancer

func CreateOrGetNetworkLoadBalancer(ctx context.Context, nlbClient *networkloadbalancer.NetworkLoadBalancerClient,
    compartmentID, subnetID, instanceID, displayName string,
    privateIP string) (*networkloadbalancer.NetworkLoadBalancer, error)

func DeleteNetworkLoadBalancer(ctx context.Context, nlbClient *networkloadbalancer.NetworkLoadBalancerClient,
    nlbID string) error

func ListNetworkLoadBalancers(ctx context.Context, nlbClient *networkloadbalancer.NetworkLoadBalancerClient,
    compartmentID string) ([]networkloadbalancer.NetworkLoadBalancer, error)
```

### 8.3 New HTTP Handlers

Add to `internal/httpapi/server.go` under the protected group:

```go
// Phase 11.2: VNIC Management
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

### 8.4 Clients Struct Extension

The `Clients` struct needs an NLB client:

```go
type Clients struct {
    Compute       *core.ComputeClient
    Vcn           *core.VirtualNetworkClient
    Identity      *identity.IdentityClient
    ObjectStorage *objectstorage.ObjectStorageClient
    Blockstorage  *core.BlockstorageClient
    NLB           *networkloadbalancer.NetworkLoadBalancerClient  // NEW
}
```

### 8.5 Concurrency Guard for IP Switch

The Java code uses a `ConcurrentHashMap<String, String>` (`ipSwitchTasks`) to prevent concurrent IP switch operations on the same instance. In Go, use a `sync.Map` or a `map[string]struct{}` guarded by a `sync.Mutex`:

```go
var ipSwitchTasks sync.Map // map[string]struct{}

func tryAcquireIPSwitch(instanceID string) bool {
    _, loaded := ipSwitchTasks.LoadOrStore(instanceID, struct{}{})
    return !loaded
}

func releaseIPSwitch(instanceID string) {
    ipSwitchTasks.Delete(instanceID)
}
```

---

## 9. Edge Cases and Error Handling

### 9.1 Parameter Validation
- `vnicCount` must be in `[1, 32]`. Return 400 with descriptive message.
- `ipv6CountPerVnic` must be in `[0, 32]`. Return 400.
- All required fields (`instanceId`, `subnetId`, etc.) must be non-empty. Return 400 "参数不完整".

### 9.2 Tenant Resolution
- Every handler resolves `instanceId` -> `InstanceDetails` -> `tenantId` -> `Tenant`.
- If tenant not found: 400 "找不到对应的租户信息".
- Go: implement a helper `resolveTenantFromInstanceID(ctx, store, instanceID) (*Tenant, *InstanceDetails, error)`.

### 9.3 Subnet IPv6 Incompatibility
- If the target subnet does not support IPv6 (`ipv6CidrBlocks` empty), IPv6 creation is silently skipped (`ipv6CountPerVnic` set to 0). A warning is logged but the operation succeeds.

### 9.4 Partial Batch Failure
- If one VNIC in a batch fails to create, `allSuccessful` is set to `false` and the batch returns immediately (the Java code does NOT continue to the next VNIC on failure).
- The response HTTP status is still 200; the `success` field in the JSON body indicates the outcome.

### 9.5 Primary VNIC Protection
- Deletion of the primary VNIC is blocked. The check uses the earliest `timeCreated` attachment.
- `deleteAllSecondaryVnics` skips the primary VNIC and marks it as `true` in the results.

### 9.6 VNIC Attachment Timeout
- Both attach and detach poll for up to 300 seconds with 3-second intervals.
- If `NotFound` is received during attach polling, it is treated as "not yet created" and polling continues.
- If `NotFound` is received during detach polling, it is treated as successful deletion.

### 9.7 IPv6 Deletion by Address
- `deleteIpv6Address` receives the IPv6 address string (not the OCID). It must first list all IPv6s on the VNIC, find the matching one, extract the OCID, then delete.
- If no match is found: return `false`.

### 9.8 Instance Reset After IPv6 Creation
- After `createIpv6`, the Java code calls `OciUtils.resetInstance(tenant, instanceId)` (stop + start the instance). This is necessary for IPv6 addresses to take effect.
- Go: implement `ResetInstance(ctx, computeClient, instanceID)` that stops then starts the instance.

### 9.9 IP Switch Retry Loop
- `changeSpecIp` retries IP reassignment in a loop until the new IP falls within the specified CIDR range(s).
- Each retry waits 60-80 seconds (random) before the next attempt.
- Maximum retries: configurable (Java uses a `maxRetries` field).
- The operation is non-blocking from the HTTP perspective (returns immediately with a status message, but the actual loop runs synchronously in the handler goroutine).
- A concurrency guard prevents multiple simultaneous IP switches on the same instance.

### 9.10 Load Balancer Account Type Gate
- `configureLoadBalancer` and `restoreNetwork` are restricted to:
  - Account types: `TRIAL_PAID_ACCOUNT` or `UPGRADE_ACCOUNT`
  - Architecture: `AMD`
- Other account types get 400 "当前租户不支持".

### 9.11 NLB Backend Configuration
- Backend uses `targetId` = instance OCID (not VNIC ID).
- Backend port: 22 (SSH).
- Health check: TCP port 22.
- Listener: TCP+UDP port 22.
- Policy: FiveTuple.

### 9.12 Network Restore Order
- Restoration must delete resources in the correct order:
  1. Reset VNIC route table (detach from custom route table).
  2. Delete NLBs.
  3. Find the NAT route table associated with the NAT gateway.
  4. Delete the route table.
  5. Delete the NAT gateway.
- Reversing this order can cause dependency errors (route table references NAT gateway).

---

## 10. Differences from Java Implementation

| Aspect | Java | Go (Recommended) |
|---|---|---|
| Primary VNIC detection | Earliest `timeCreated` attachment | Use `vnic.IsPrimary` field first, fall back to earliest attachment (existing Go code already does this) |
| IPv6 assign approach | `CreateIpv6` + `deleteIpv6` | Existing Go `AssignIpv6ToVnic` uses `Ipv6VnicDetach` for removal (different API call but equivalent effect) |
| NLB client | `NetworkLoadBalancerClient` (separate SDK package) | Add `networkloadbalancer.NetworkLoadBalancerClient` to `Clients` struct |
| Instance reset | `OciUtils.resetInstance` (stop + start) | Implement as `ResetInstance` in `compute.go` |
| IP switch concurrency | `ConcurrentHashMap` | `sync.Map` |
| Batch VNIC create | Iterative, sequential | Keep sequential (OCI has per-resource rate limits); consider goroutine pool for IPv6 creation within a single VNIC |
| Subnet auto-creation | `isCreateSubnet` flag (controller always passes `false`) | Expose as optional parameter or separate endpoint |

---

## 11. Go SDK Dependencies

Required OCI Go SDK packages (already vendored or importable):

```go
"github.com/oracle/oci-go-sdk/v65/core"                    // ComputeClient, VirtualNetworkClient
"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"     // NetworkLoadBalancerClient (NEW)
"github.com/oracle/oci-go-sdk/v65/common"                   // ConfigurationProvider
```

The `networkloadbalancer` package may need to be added to `go.mod` if not already present.

---

## 12. Testing Checklist

- [ ] Create 1 VNIC with 0 IPv6 — verify VNIC attached, no IPv6 assigned
- [ ] Create 1 VNIC with 4 IPv6 — verify 4 IPv6 addresses created
- [ ] Create 5 VNICs with 32 IPv6 each — verify batch completes (160 IPv6 total)
- [ ] Delete single VNIC — verify IPv6 cleanup + detachment
- [ ] Delete all secondary VNICs — verify primary preserved
- [ ] Create IPv6 on existing VNIC — verify instance reset triggered
- [ ] Delete IPv6 by address — verify correct IPv6 removed
- [ ] Configure load balancer — verify NAT GW + route table + NLB created
- [ ] Restore network — verify all resources cleaned up
- [ ] IP switch with CIDR range — verify retry loop and IP in range
- [ ] Concurrent IP switch guard — verify second request rejected
- [ ] Subnet without IPv6 support — verify graceful fallback (IPv6 skipped)
- [ ] Primary VNIC deletion attempt — verify blocked
- [ ] Timeout on VNIC attach — verify error returned after 300s
