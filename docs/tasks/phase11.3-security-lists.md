# Phase 11.3 Security Lists -- Task Breakdown

## Overview

Implement security list rule management: list/add/delete rules on OCI Security Lists, plus batch "enable all protocols" across tenants. Security rules are NOT stored in the DB -- they are fetched and mutated live via the OCI VirtualNetworkClient. The only persisted state is the `enable_icmp` and `enable_all_protocol` boolean flags on the tenant table (already exist).

---

## Backend Tasks

### B1: OCI SDK Wrapper

**File:** `internal/oci/security_list.go` (new)

**Pattern to follow:** `internal/oci/network.go` -- stateless functions that accept `(ctx context.Context, c Clients, ...)` and return domain types or errors.

**Types to define:**

```go
// SecurityRuleDTO is the API request/response type for a single security rule.
type SecurityRuleDTO struct {
    ID       string  `json:"id"`
    Type     string  `json:"type"`      // "ingress" or "egress" (request); "入站" or "出站" (response)
    Protocol string  `json:"protocol"`  // "all", "tcp", "udp", "icmp", "6", "17", "1"
    Source   string  `json:"source"`    // CIDR block
    Ports    string  `json:"ports"`     // "80", "8080-9090", "ALL", or ""
    TenantID *int64  `json:"tenantId"`
    ICMPType *string `json:"icmpType"`  // "8, 0" or null
}
```

**Functions to implement:**

```go
// ListSecurityRules lists all ingress or egress rules across ALL Security Lists
// in a compartment. Returns rules with type label "入站" or "出站".
// Parity with Java SecurityRuleService.getSecurityRuleList.
func ListSecurityRules(ctx context.Context, c Clients, compartmentID, ruleType string) ([]SecurityRuleDTO, error)

// AddSecurityRule adds (or replaces) a rule on the FIRST Security List in the
// compartment. Before appending, removes all existing rules that match by
// protocol + CIDR + ports (duplicate detection). Then calls UpdateSecurityList.
// Parity with Java SecurityRuleService.addSecurityRule.
func AddSecurityRule(ctx context.Context, c Clients, compartmentID string, rule SecurityRuleDTO) error

// DeleteSecurityRule deletes a rule identified by composite ID
// "{tenantId}_{ruleIndex}_{type}". Uses global index across all Security Lists
// to locate the target rule, then removes all matching rules.
// Parity with Java SecurityRuleService.deleteSecurityRule.
func DeleteSecurityRule(ctx context.Context, c Clients, compartmentID, compositeID string) error

// EnableAllForTenant adds "all protocol" ingress/egress + ICMP rules for a
// single tenant. Adds: ingress all/0.0.0.0/0, ingress all/::/0, ingress ICMP
// 0.0.0.0/0 (type 8 code 0), ingress ICMP 10.0.0.0/16, egress all/0.0.0.0/0,
// egress all/::/0. IPv6 failures are logged and skipped.
// Parity with Java SecurityRuleService.checkAndEnableRule.
func EnableAllForTenant(ctx context.Context, c Clients, compartmentID string) (addedIPv6 bool, err error)

// EnableIPv6ForTenant adds IPv6 ingress/egress rules for a single tenant.
// Adds: ingress all/::/0, egress all/::/0.
// Parity with Java SecurityRuleService.singleIpv6Rule.
func EnableIPv6ForTenant(ctx context.Context, c Clients, compartmentID string) error

// ConfigureIPv6SecurityRules adds ICMPv6 + SSH + egress rules to the VCN's
// default Security List (obtained via GetVcn). Called during IPv6 enable on
// an instance. Adds ICMPv6 types 128,129,133-137 from ::/0, TCP port 22
// from ::/0, and egress all to ::/0.
// Parity with Java OciUtils.configureIpv6SecurityRules.
func ConfigureIPv6SecurityRules(ctx context.Context, c Clients, vcnID string) error
```

**Internal helpers:**

```go
// getProtocolNumber converts "tcp"->"6", "udp"->"17", "icmp"->"1", etc.
func getProtocolNumber(protocol string) string

// parsePorts parses the DTO ports string into (min, max *int).
// "80" -> (80,80), "8080-9090" -> (8080,9090), "ALL"/"" -> (nil,nil).
func parsePorts(ports string) (min, max *int)

// formatPorts converts OCI PortRange back to a string for the DTO.
func formatPorts(tcpOpts *core.TcpOptions, udpOpts *core.UdpOptions) string

// buildIngressRule converts a SecurityRuleDTO into core.IngressSecurityRule.
func buildIngressRule(rule SecurityRuleDTO) core.IngressSecurityRule

// buildEgressRule converts a SecurityRuleDTO into core.EgressSecurityRule.
func buildEgressRule(rule SecurityRuleDTO) core.EgressSecurityRule

// matchIngressRule checks if two ingress rules match by protocol+CIDR+ports.
func matchIngressRule(a, b core.IngressSecurityRule) bool

// matchEgressRule checks if two egress rules match by protocol+CIDR+ports.
func matchEgressRule(a, b core.EgressSecurityRule) bool

// listAllSecurityLists paginates through all Security Lists in a compartment.
func listAllSecurityLists(ctx context.Context, c Clients, compartmentID string) ([]core.SecurityList, error)
```

**OCI SDK calls used:**
- `c.Vcn.ListSecurityLists(ctx, core.ListSecurityListsRequest{CompartmentId: ...})`
- `c.Vcn.GetSecurityList(ctx, core.GetSecurityListRequest{SecurityListId: ...})`
- `c.Vcn.UpdateSecurityList(ctx, core.UpdateSecurityListRequest{SecurityListId: ..., UpdateSecurityListDetails: ...})`
- `c.Vcn.GetVcn(ctx, core.GetVcnRequest{VcnId: ...})`

**Key implementation notes:**
- `compartmentID` is always the tenancy OCID (same as `provider.getTenantId()` in Java).
- For ingress rules, DTO `type` response label is "入站"; for egress, "出站".
- For egress rules, DTO `source` maps to OCI `EgressSecurityRule.Destination`.
- Protocol "all" must NOT set any TCP/UDP/ICMP options.
- ICMP defaults: type=8, code=0 when not specified.

---

### B2: Service Layer

**File:** `internal/service/security_rule.go` (new)

**Pattern to follow:** `internal/service/tenant.go` -- struct with `store`, `masterKey`, `pool` fields; methods accept `context.Context`, resolve tenant from DB, build OCI clients via `oci.WithProxy`, call OCI wrapper functions.

**Struct:**

```go
type SecurityRuleService struct {
    store     *db.Store
    masterKey []byte
    pool      *oci.ProxyPool
}

func NewSecurityRuleService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *SecurityRuleService
```

**Methods:**

```go
// GetRules lists security rules for a tenant. Looks up tenant by ID, builds
// OCI clients, calls oci.ListSecurityRules.
func (s *SecurityRuleService) GetRules(ctx context.Context, tenantID int64, ruleType string) ([]oci.SecurityRuleDTO, error)

// AddRule adds/replaces a security rule for a tenant. Looks up tenant,
// builds OCI clients, calls oci.AddSecurityRule.
func (s *SecurityRuleService) AddRule(ctx context.Context, tenantID int64, rule oci.SecurityRuleDTO) error

// DeleteRule deletes a security rule by composite ID. Looks up tenant,
// builds OCI clients, calls oci.DeleteSecurityRule.
func (s *SecurityRuleService) DeleteRule(ctx context.Context, compositeID string) error

// BatchEnableAll iterates ALL tenants and calls oci.EnableAllForTenant for
// each. On success, sets tenant.EnableAllProtocol=true and saves. For
// production, use a goroutine pool with rate limiting.
// Parity with Java SecurityRuleService.batchAllSecurityRule.
func (s *SecurityRuleService) BatchEnableAll(ctx context.Context) (successCount, failCount int, err error)

// SingleEnableAll calls oci.EnableAllForTenant + oci.EnableIPv6ForTenant
// for a single tenant, then updates tenant flags.
// Parity with Java SecurityRuleService.singleSecurityAllRule + singleIpv6Rule.
func (s *SecurityRuleService) SingleEnableAll(ctx context.Context, tenantID int64) error
```

**Tenant lookup pattern** (same as `TenantService` methods):
```go
t, err := repo.New(s.store.Read).FindTenantById(ctx, tenantID)
// build creds from t, then oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error { ... })
```

---

### B3: HTTP Handlers

**File:** `internal/httpapi/security_rule.go` (new)

**Pattern to follow:** `internal/httpapi/` existing handler files -- each handler is a function returning `gin.HandlerFunc`, closing over `*Deps`.

**Functions:**

```go
// GET /tenants/security-rules?tenantId={id}&type={ingress|egress}
func getSecurityRules(deps *Deps) gin.HandlerFunc
// 1. Parse query params: tenantId (int64, required), type (string, required: "ingress" or "egress")
// 2. Call deps.SecurityRule.GetRules(ctx, tenantID, ruleType)
// 3. Return JSON array of SecurityRuleDTO
// 4. On error: c.JSON(400/500, gin.H{"error": err.Error()})

// POST /tenants/security-rules
func addSecurityRule(deps *Deps) gin.HandlerFunc
// 1. Bind JSON body to oci.SecurityRuleDTO
// 2. Call deps.SecurityRule.AddRule(ctx, *rule.TenantID, rule)
// 3. Return 200 with the saved DTO
// 4. On error: c.JSON(400/500, gin.H{"error": err.Error()})

// DELETE /tenants/security-rules/:id
func deleteSecurityRule(deps *Deps) gin.HandlerFunc
// 1. Parse path param :id (compositeId string)
// 2. Call deps.SecurityRule.DeleteRule(ctx, compositeID)
// 3. Return 200 OK (empty body)
// 4. On error: c.JSON(400/500, gin.H{"error": err.Error()})

// POST /tenants/enableAll
func batchEnableAll(deps *Deps) gin.HandlerFunc
// 1. Call deps.SecurityRule.BatchEnableAll(ctx)
// 2. Return {"code": 200, "message": "success", "data": null}
// 3. On error: return {"code": 500, "message": "...", "data": null}
```

**Deps addition:** Add `SecurityRule *service.SecurityRuleService` to the `Deps` struct in `internal/httpapi/deps.go`.

---

### B4: Route Registration

**File:** `internal/httpapi/server.go` (modify)

**Changes:** Add four routes to the protected group, after the Phase 10 routes (around line 83):

```go
// Phase 11.3: Security List rule management
pro.GET("/tenants/security-rules", getSecurityRules(deps))
pro.POST("/tenants/security-rules", addSecurityRule(deps))
pro.DELETE("/tenants/security-rules/:id", deleteSecurityRule(deps))
pro.POST("/tenants/enableAll", batchEnableAll(deps))
```

---

### B5: Wire Dependencies

**File:** `cmd/server/main.go` (or wherever Deps is constructed)

**Changes:**
1. Create `service.NewSecurityRuleService(store, masterKey, pool)`.
2. Assign to `deps.SecurityRule`.

---

## Frontend Tasks

### F1: Security Rules Dialog

**File:** `frontend/src/views/Tenants.vue` (modify existing)

**Pattern to follow:** Existing dialogs in Tenants.vue (traffic alert, email, user management, etc.) -- `ref` for visibility/state, `openXxx(row)` function, `<el-dialog>` in template, API calls via `request`.

**Dropdown menu addition:** Add a new item to the `<el-dropdown-menu>` (after "instances", before "trafficAlert"):

```html
<el-dropdown-item command="securityRules">
  <el-icon><Lock /></el-icon> 安全规则
</el-dropdown-item>
```

Import the `Lock` icon from `@element-plus/icons-vue`.

**handleAction addition:**

```typescript
case 'securityRules': openSecurityRules(row); break
```

**New state variables:**

```typescript
const secRulesVisible = ref(false)
const secRulesLoading = ref(false)
const secRulesTenantId = ref(0)
const secRulesTenantName = ref('')
const secRulesTab = ref<'ingress' | 'egress'>('ingress')
const secRulesData = ref<SecurityRuleDTO[]>([])
const secRuleFormVisible = ref(false)
const secRuleSaving = ref(false)
const secRuleForm = ref({ protocol: 'all', source: '0.0.0.0/0', ports: '', icmpType: '' })
```

**TypeScript interface:**

```typescript
interface SecurityRuleDTO {
  id: string | null
  type: string
  protocol: string
  source: string
  ports: string | null
  tenantId: number | null
  icmpType: string | null
}
```

**Functions:**

```typescript
async function openSecurityRules(row: Tenant) { /* set tenantId/name, show dialog, fetch rules */ }
async function fetchSecRules() { /* GET /tenants/security-rules?tenantId=X&type=ingress|egress */ }
function switchSecRulesTab(tab: string) { /* switch ingress/egress, re-fetch */ }
function openAddSecRule() { /* show add form */ }
async function saveSecRule() { /* POST /tenants/security-rules */ }
async function deleteSecRule(rule: SecurityRuleDTO, index: number) { /* DELETE /tenants/security-rules/{compositeId} */ }
```

**Composite ID generation** (client-side, matching Java):
```typescript
const compositeId = `${secRulesTenantId.value}_${index}_${secRulesTab.value}`
```

**Dialog template structure:**
- `<el-dialog>` with title "安全规则 -- {tenantName}"
- `<el-tabs>` for ingress/egress switching
- `<el-table>` showing: protocol, source, ports, actions (delete)
- Add form: protocol dropdown (all/tcp/udp/icmp), source CIDR input, ports input
- Add button opens inline form (same pattern as add-user form in user management dialog)

---

## Dependencies

```
B1 (OCI Wrapper) --> B2 (Service) --> B3 (Handlers) --> B4 (Routes) --> B5 (Wire)
                                                                          |
B3 -----------------------------------------------------------------> F1 (Frontend needs API)
```

- B1 must complete before B2 (service calls OCI wrapper).
- B2 must complete before B3 (handler calls service).
- B3+B4 must complete before F1 (frontend calls API endpoints).
- F1 can start stub/mock work in parallel with B1-B3, but needs real endpoints for integration.

## Test Checklist

- [ ] GET /tenants/security-rules returns rules for ingress and egress
- [ ] POST /tenants/security-rules adds a rule (with duplicate replace)
- [ ] DELETE /tenants/security-rules/{compositeId} removes a rule
- [ ] POST /tenants/enableAll runs across all tenants
- [ ] Protocol "all" rules have no TCP/UDP/ICMP options
- [ ] Egress rules correctly map DTO `source` to OCI `destination`
- [ ] Frontend dialog shows tabs, adds/deletes rules
