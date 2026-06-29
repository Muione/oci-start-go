# Phase 11.3 — Security List Rule Management

## Overview

Security List rules control network ingress/egress at the VCN (Virtual Cloud Network) level in OCI. The Java project provides full CRUD for security rules on a tenant's default Security List, plus batch operations to enable all protocols across all tenants. The Go rewrite must provide equivalent REST API endpoints and OCI SDK integration.

---

## 1. API Endpoints

All endpoints are prefixed with `/tenants` and require session authentication.

### 1.1 Get Security Rules

```
GET /tenants/security-rules?tenantId={tenantId}&type={ingress|egress}
```

**Query Parameters:**

| Param    | Type   | Required | Description                     |
|----------|--------|----------|---------------------------------|
| tenantId | string | yes      | Internal tenant DB ID           |
| type     | string | yes      | `"ingress"` or `"egress"`       |

**Response:** `200 OK` — Array of `SecurityRuleDTO`

```json
[
  {
    "id": null,
    "type": "入站",
    "protocol": "all",
    "source": "0.0.0.0/0",
    "ports": null,
    "tenantId": null,
    "icmpType": null
  }
]
```

**Notes:**
- The Java code uses `response.getItems()` from `ListSecurityListsResponse` which returns ALL Security Lists in the compartment. It iterates over every SecurityList and collects rules from each.
- For ingress rules, `type` is set to the Chinese string `"入站"` (inbound).
- For egress rules, `type` is set to `"出站"` (outbound).
- The `source` field for egress rules maps to OCI's `destination` field (the DTO reuses `source` for both directions).

### 1.2 Add Security Rule

```
POST /tenants/security-rules
Content-Type: application/json
```

**Request Body:** `SecurityRuleDTO`

```json
{
  "tenantId": 123,
  "type": "ingress",
  "protocol": "tcp",
  "source": "0.0.0.0/0",
  "ports": "80",
  "icmpType": null
}
```

**Response:** `200 OK` — The saved `SecurityRuleDTO`

**Behavior:**
1. Looks up the tenant by `tenantId`.
2. Lists all Security Lists in the tenant's compartment.
3. Takes the **first** Security List (`response.getItems().get(0)`) — the default one.
4. For ingress: builds an `IngressSecurityRule` and appends it.
5. For egress: builds an `EgressSecurityRule` and appends it.
6. **Duplicate detection before add:** Before appending, the code finds all existing rules matching the same protocol + source/destination + port range, removes them all, then adds the new rule. This is a "replace" semantic, not pure append.
7. Calls `UpdateSecurityList` with the full updated rule list.

### 1.3 Delete Security Rule

```
DELETE /tenants/security-rules/{compositeId}
```

**Path Parameter:**

| Param       | Type   | Description                                              |
|-------------|--------|----------------------------------------------------------|
| compositeId | string | Format: `{tenantId}_{ruleIndex}_{type}` e.g. `123_5_ingress` |

**Response:** `200 OK` (empty body on success)

**Behavior:**
1. Parses `compositeId` into `tenantId`, `ruleIndex` (int), and `type` (`ingress`/`egress`).
2. Lists all Security Lists in the compartment.
3. Uses a **global index** across all Security Lists: iterates through each SecurityList's rules, accumulating a count, until the global index falls within the current SecurityList.
4. Converts the global index to a local index within that SecurityList.
5. Finds all rules in that SecurityList matching the target rule (by protocol + source/destination + port), removes them all.
6. Calls `UpdateSecurityList` with the filtered rule list.

### 1.4 Batch Enable All Protocols (All Tenants)

```
POST /tenants/enableAll
```

**Response:** `200 OK` — `ApiResponse`

```json
{ "code": 200, "message": "success", "data": null }
```

**Behavior:**
1. Fetches ALL tenants from the database.
2. For each tenant, calls `checkAndEnableRule(tenant)`.
3. `checkAndEnableRule` does the following for each tenant:
   - Gets current ingress and egress rules.
   - Checks for existing "all protocol" rules (`protocol=all`, `source=0.0.0.0/0`).
   - Checks for existing IPv6 rules (`protocol=all`, `source=::/0`).
   - Checks for existing ICMP rules (`protocol=ICMP or 1`, `source=0.0.0.0/0`).
   - Checks for existing local ICMP rules (`protocol=ICMP or 1`, `source=10.0.0.0/16`).
   - Adds any missing rules:
     - Ingress: `all` protocol from `0.0.0.0/0`
     - Ingress: `all` protocol from `::/0` (IPv6, failures logged but ignored)
     - Ingress: ICMP from `0.0.0.0/0` (type 8, code 0)
     - Ingress: ICMP from `10.0.0.0/16` (type 8, code 0)
     - Egress: `all` protocol to `0.0.0.0/0`
     - Egress: `all` protocol to `::/0` (IPv6, failures logged but ignored)
   - If any rule was added, sets `tenant.enableAllProtocol = true` and saves.

### 1.5 Single Tenant — Enable All Protocols

```
Called internally (not a standalone HTTP endpoint in controller):
SecurityRuleService.singleSecurityAllRule(Tenant tenant)
```

**Behavior:**
1. Adds ingress rule: `all` protocol from `0.0.0.0/0`.
2. Adds egress rule: `all` protocol to `0.0.0.0/0`.
3. Sets `tenant.enableIcmp = true` and `tenant.enableAllProtocol = true`.
4. Saves the tenant.

### 1.6 Single Tenant — Enable IPv6 Rules

```
Called internally:
SecurityRuleService.singleIpv6Rule(Tenant tenant)
```

**Behavior:**
1. Adds ingress rule: `all` protocol from `::/0`.
2. Adds egress rule: `all` protocol to `::/0`.
3. Sets `tenant.enableIcmp = true` and `tenant.enableAllProtocol = true`.
4. Saves the tenant.

### 1.7 IPv6 Security Rules (during instance IPv6 enable)

In `OciUtils.configureIpv6SecurityRules()` (called during IPv6 enable on an instance):

1. Gets the VCN's default Security List via `vcn.getDefaultSecurityListId()`.
2. Gets the Security List via `GetSecurityList`.
3. Checks for existing ICMPv6 rules (protocol `58`) from `::/0` for types: 128, 129, 133, 134, 135, 136, 137.
4. Checks for existing IPv6 TCP SSH rule (protocol `6`, source `::/0`, port 22).
5. Checks for existing IPv6 egress rule (destination `::/0`, protocol `all`).
6. Adds any missing rules and calls `UpdateSecurityList`.

---

## 2. Data Structures

### 2.1 SecurityRuleDTO (API Request/Response)

```go
type SecurityRuleDTO struct {
    ID       string  `json:"id"`
    Type     string  `json:"type"`      // "ingress" or "egress" (request); "入站" or "出站" (response)
    Protocol string  `json:"protocol"`  // "all", "tcp", "udp", "icmp", "6", "17", "1"
    Source   string  `json:"source"`    // CIDR: "0.0.0.0/0", "::/0", "10.0.0.0/16"
    Ports    string  `json:"ports"`     // "80", "8080-9090", "ALL", or null
    TenantID *int64  `json:"tenantId"`
    ICMPType *string `json:"icmpType"`  // "8, 0" or "8" or null
}
```

### 2.2 OCI SDK Types Used

**Ingress Rule:**
```go
core.IngressSecurityRule {
    Protocol  *string            // "1"(ICMP), "6"(TCP), "17"(UDP), "58"(ICMPv6), "all"
    Source    *string            // CIDR block
    TcpOptions  *core.TcpOptions
    UdpOptions  *core.UdpOptions
    IcmpOptions *core.IcmpOptions
    Description *string
}
```

**Egress Rule:**
```go
core.EgressSecurityRule {
    Protocol    *string
    Destination *string          // CIDR block
    TcpOptions  *core.TcpOptions
    UdpOptions  *core.UdpOptions
    IcmpOptions *core.IcmpOptions
    Description *string
}
```

**Port Range:**
```go
core.PortRange {
    Min *int
    Max *int
}
```

**ICMP Options:**
```go
core.IcmpOptions {
    Type *int
    Code *int
}
```

---

## 3. OCI SDK Operations

All operations use the `core.VirtualNetworkClient` (already available as `Clients.Vcn` in Go).

| Operation | OCI SDK Call | Java Reference |
|-----------|-------------|----------------|
| List Security Lists | `ListSecurityLists(compartmentId)` | `vcnClient.listSecurityLists(...)` |
| Get Security List | `GetSecurityList(securityListId)` | `virtualNetworkClient.getSecurityList(...)` |
| Update Security List | `UpdateSecurityList(securityListId, UpdateSecurityListDetails)` | `vcnClient.updateSecurityList(...)` |
| Get VCN (for default SL) | `GetVcn(vcnId)` | `virtualNetworkClient.getVcn(...)` |

**Key detail:** The Java code always uses `compartmentId = provider.getTenantId()` (the tenancy OCID) as the compartment for listing Security Lists. The Go code should do the same.

---

## 4. Rule Matching / Duplicate Detection Logic

The Java code has a sophisticated matching system to detect duplicate rules before add/delete operations.

### 4.1 Match Criteria

Two rules are considered duplicates if:
1. **Protocol** matches (string comparison).
2. **Source** (ingress) or **Destination** (egress) matches (string comparison).
3. **Port options** match based on protocol:
   - TCP (`"6"`): `tcpOptions.destinationPortRange` and `tcpOptions.sourcePortRange` both match.
   - UDP (`"17"`): `udpOptions.destinationPortRange` and `udpOptions.sourcePortRange` both match.
   - ICMP (`"1"`): `icmpOptions.type` and `icmpOptions.code` both match.
   - `"all"` or other protocols: no port check needed (always matches if protocol + CIDR match).

### 4.2 Add with Replace

When adding a rule:
1. Find ALL existing rules that match the new rule (by protocol + CIDR + ports).
2. Remove all matching rules from the list.
3. Append the new rule.
4. Call `UpdateSecurityList` with the modified list.

This means adding a rule is effectively an "upsert" — it replaces any existing duplicate.

### 4.3 Delete with Match

When deleting a rule by composite ID:
1. Use the global index to locate the target SecurityList and local index.
2. Get the target rule at that local index.
3. Find ALL rules in that SecurityList matching the target rule.
4. Remove all matching rules.
5. Call `UpdateSecurityList` with the filtered list.

---

## 5. Protocol Mapping

| Protocol Name | OCI Protocol Number |
|---------------|-------------------|
| TCP           | `"6"`             |
| UDP           | `"17"`            |
| ICMP          | `"1"`             |
| ICMPv6        | `"58"`            |
| ALL           | `"all"`           |

The DTO accepts both names (`"tcp"`, `"udp"`, `"icmp"`) and numbers (`"6"`, `"17"`, `"1"`). The `getProtocolNumber()` method converts names to numbers. The `"all"` protocol is passed through as-is.

---

## 6. Port Parsing

The `ports` field in the DTO is a string with the following formats:

| Input          | Parsed As                     |
|----------------|-------------------------------|
| `"80"`         | min=80, max=80                |
| `"8080-9090"`  | min=8080, max=9090            |
| `"80,443"`     | Only first port used: min=80, max=80 (OCI limitation) |
| `"ALL"` or `""` | null (no port restriction)   |
| `null`         | null                          |

**Important OCI limitation:** OCI does not support multiple non-contiguous ports in a single rule. The Java code only processes the first comma-separated port. The Go implementation should either replicate this or improve by creating multiple rules.

---

## 7. Frontend Data Format

### 7.1 Desktop (tenant_region_list.ftl + JS)

The desktop UI uses a modal (`#securityRulesModal`) with:
- Tab switching between ingress/egress rules.
- A table showing: index, type, protocol, source, ports, actions (edit/delete).
- Add form with: protocol dropdown, source CIDR input, ports input.
- Edit populates the same form with existing rule data.
- Delete uses composite ID format: `{tenantId}_{ruleIndex}_{type}`.
- Pagination with 10 items per page.

The composite ID is generated client-side: `tenantId + "_" + actualIndex + "_" + activeTab`.

### 7.2 Mobile (mobile/security_rules.ftl)

The mobile UI uses:
- Tab bar for ingress/egress switching.
- Card-based rule display with protocol badge, CIDR, and ports.
- Add panel with protocol select, port range input, CIDR input.
- Delete with confirmation modal.
- The mobile frontend sends `source` for ingress and `destination` for egress (note: the Java DTO only has `source` field, so egress uses `source` field too — the backend maps `source` to OCI's `destination` for egress rules).

---

## 8. Database Changes

### 8.1 Tenant Table (already exists in Go)

The `tenant` table already has these columns (confirmed in Go repo):

```sql
enable_icmp         INTEGER DEFAULT 0  -- boolean flag
enable_all_protocol INTEGER DEFAULT 0  -- boolean flag
```

Both fields exist in `repo/models.go` and are read/written by existing Go code. No schema changes needed.

### 8.2 No Separate Security Rules Table

Security rules are NOT stored in the database. They are always fetched live from OCI and managed through the OCI API. The `enable_icmp` and `enable_all_protocol` flags on the tenant are the only persisted state.

---

## 9. Edge Cases and Pitfalls

### 9.1 Multiple Security Lists

The Java code has inconsistent handling:
- **Get rules:** Iterates ALL Security Lists and aggregates rules from each.
- **Add rule:** Uses only the FIRST Security List (`getItems().get(0)`).
- **Delete rule:** Iterates ALL Security Lists using a global index.

This means if a tenant has multiple Security Lists, the global index from GET may not correspond correctly to the first Security List used for ADD. The Go rewrite should consider using a more robust identification scheme (e.g., `{securityListId}_{localIndex}_{type}`).

### 9.2 IPv6 Failures Are Tolerant

Both `checkAndEnableRule` and `singleIpv6Rule` wrap IPv6 rule additions in try/catch and log warnings on failure, continuing execution. This handles tenants that don't support IPv6.

### 9.3 Protocol "all" Has No Port Options

When protocol is `"all"`, no TCP/UDP/ICMP options should be set. The Java code explicitly checks for this and returns early.

### 9.4 ICMP Defaults

When creating an ICMP rule without specific type/code, the Java code defaults to:
- Type: `8` (Echo Request / ping)
- Code: `0`

### 9.5 Egress Source/Destination Confusion

The DTO uses `source` for both ingress and egress. For egress rules:
- DTO `source` maps to OCI `EgressSecurityRule.destination`.
- When converting back from OCI to DTO, `EgressSecurityRule.destination` maps to DTO `source`.

The mobile frontend sends `destination` for egress in the add form body, but the Java DTO only has `source`. The backend should accept both `source` and `destination` for egress, or the Go rewrite should add a `destination` field.

### 9.6 Batch Operation Is Synchronous

`batchAllSecurityRule` iterates ALL tenants sequentially. For large tenant counts this could be slow. Consider adding concurrency with rate limiting in the Go rewrite.

### 9.7 Duplicate Rule Cleanup

The "add with replace" behavior means duplicate rules are automatically cleaned up on every add operation. This is intentional — OCI can accumulate duplicate rules over time.

### 9.8 Security List Creation (OciCliUtils)

During VCN setup (`ensureSecurityList`), if no Security List exists, one is created with:
- Ingress: SSH (port 22) from `0.0.0.0/0`, HTTP (port 80) from `0.0.0.0/0`
- Egress: All protocols to `0.0.0.0/0`

### 9.9 IPv6 Security Rules (OciUtils.configureIpv6SecurityRules)

When enabling IPv6 on an instance, the code adds:
- ICMPv6 rules (protocol `58`) for types 128, 129, 133, 134, 135, 136, 137 from `::/0`
- TCP SSH (port 22) from `::/0`
- Egress: all protocols to `::/0`

These are added to the VCN's **default** Security List (obtained via `vcn.getDefaultSecurityListId()`), not the first one in the list.

---

## 10. Go Implementation Checklist

### 10.1 New File: `internal/oci/security_list.go`

Functions to implement:

```go
// ListSecurityRules lists all ingress or egress rules across all Security Lists in a compartment.
func ListSecurityRules(ctx context.Context, c Clients, compartmentID, ruleType string) ([]SecurityRuleDTO, error)

// AddSecurityRule adds (or replaces) a rule on the first Security List.
func AddSecurityRule(ctx context.Context, c Clients, compartmentID string, rule SecurityRuleDTO) error

// DeleteSecurityRule deletes a rule by global index across all Security Lists.
func DeleteSecurityRule(ctx context.Context, c Clients, compartmentID, compositeID string) error

// EnableAllForTenant adds "all protocol" ingress/egress + ICMP rules for a single tenant.
func EnableAllForTenant(ctx context.Context, c Clients, compartmentID string) error

// EnableIPv6ForTenant adds IPv6 ingress/egress rules for a single tenant.
func EnableIPv6ForTenant(ctx context.Context, c Clients, compartmentID string) error

// ConfigureIPv6SecurityRules adds ICMPv6 + SSH + egress rules to the VCN's default Security List.
func ConfigureIPv6SecurityRules(ctx context.Context, c Clients, vcnID string) error
```

### 10.2 New File: `internal/httpapi/security_rule.go`

Handler functions:

```go
func getSecurityRules(deps *Deps) gin.HandlerFunc  // GET /tenants/security-rules
func addSecurityRule(deps *Deps) gin.HandlerFunc    // POST /tenants/security-rules
func deleteSecurityRule(deps *Deps) gin.HandlerFunc // DELETE /tenants/security-rules/:id
func batchEnableAll(deps *Deps) gin.HandlerFunc     // POST /tenants/enableAll
```

### 10.3 Route Registration in `server.go`

Add to the protected routes group:

```go
// Phase 11.3: Security List rule management
pro.GET("/tenants/security-rules", getSecurityRules(deps))
pro.POST("/tenants/security-rules", addSecurityRule(deps))
pro.DELETE("/tenants/security-rules/:id", deleteSecurityRule(deps))
pro.POST("/tenants/enableAll", batchEnableAll(deps))
```

### 10.4 DTO Definition

Add to `internal/service/` or `internal/httpapi/`:

```go
type SecurityRuleDTO struct {
    ID       string  `json:"id"`
    Type     string  `json:"type"`
    Protocol string  `json:"protocol"`
    Source   string  `json:"source"`
    Ports    string  `json:"ports"`
    TenantID *int64  `json:"tenantId"`
    ICMPType *string `json:"icmpType"`
}
```

---

## 11. Recommended Improvements Over Java

1. **Stable composite ID:** Use `{securityListOCID}_{localIndex}_{type}` instead of global index to avoid cross-SecurityList ambiguity.
2. **Concurrent batch operations:** Use goroutine pool with rate limiting for `enableAll` across tenants.
3. **Proper egress field:** Add `Destination` field to DTO for egress rules instead of reusing `Source`.
4. **Multi-port support:** Create multiple rules for comma-separated ports instead of only using the first.
5. **Idempotency:** Check for existing identical rules before adding (already done in Java, but make it explicit).
6. **Pagination for GET:** The Java code returns all rules at once. Consider adding pagination for tenants with many rules.
