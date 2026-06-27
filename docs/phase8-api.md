# Phase 8 API — Data Migration & Stub Completion

## 1. Data Migration Endpoints

### POST /migration/import
Import plain SQL database export. Requires `multipart/form-data` with a `file` field (.sql).

**Request:** `multipart/form-data`
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| file  | file | yes      | .sql export file from Java oci-start |

**Response:**
```json
{
  "code": 200,
  "data": {
    "totalLines": 1250,
    "insertLines": 180,
    "inserted": 175,
    "skipped": 2,
    "skippedDups": 0,
    "skippedUser": 3,
    "errors": 0,
    "tablesFound": {"TENANT": 5, "OCI_SSH_CONN": 12},
    "message": "导入完成: 成功 175, 跳过 5, 错误 0"
  }
}
```

### POST /migration/import-encrypted
Import encrypted .enc database export. Requires `multipart/form-data` with `file` and `masterKey` fields.

**Request:** `multipart/form-data`
| Field     | Type   | Required | Description |
|-----------|--------|----------|-------------|
| file      | file   | yes      | .enc encrypted backup |
| masterKey | string | yes      | Decryption key (from X-MASTER-KEY header) |

**Response:** Same as plain import.

### GET /migration/export
Export current database as plain SQL. (Currently unimplemented — returns 501.)

**Test Plan:**
1. Export from Java: `curl -o backup.sql http://java-server:9856/migration/export`
2. Import to Go: `curl -X POST -F "file=@backup.sql" http://go-server:9856/migration/import`
3. Verify tenant list matches
4. Verify instance details match
5. Verify SSH connections match
6. Verify proxy records match
7. Test encrypted workflow: export encrypted → import with master key

---

## 2. CLI Migration Tool

```
Usage: migrate -db <db> -file <export> [-key <master-key>] [-keydir <dir>]

  -db      Path to target SQLite database
  -file    Path to SQL export file (.sql or .enc)
  -key     Master key for encrypted .enc files
  -keydir  PEM key extraction directory (default: /tmp/oci-start-keys)
```

**Test:** `./migrate -db oci-start.db -file backup.sql`

---

## 3. VNC Console WebSocket (consoles.go)

WS endpoint: `ws://host/ws/console`

**Messages:**

| Type | Direction | Description |
|------|-----------|-------------|
| create_connection | Client→Server | Create VNC console: `{type, data: {instanceId, tenantId}}` |
| vnc_data | Client→Server | Relay VNC data: `{type, data: {instanceId, data: []byte}}` |
| disconnect | Client→Server | Close VNC session |
| ping/heartbeat | Client→Server | Keepalive → server responds `pong` |
| connection_created | Server→Client | VNC ready: `{type, instanceId, vncPort, vncHost}` |
| error | Server→Client | Error details |

**Flow:** create_connection → SSH tunnel to instance → VNC port forward → connection_created → vnc_data relay.

**Test Plan:**
1. Connect to `/ws/console`
2. Send `{"type":"create_connection","data":{"instanceId":"ocid1.instance...", "tenantId":1}}`
3. Verify `connection_created` response with VNC port
4. Send `vnc_data` and verify TCP relay
5. Send `disconnect` and verify session cleanup

---

## 4. Rescue WebSocket (rescue.go)

WS endpoint: `ws://host/ws/rescue`

**Messages:**

| Type | Direction | Description |
|------|-----------|-------------|
| init | Client→Server | Start rescue: `{type, data: {instanceId, rescueType, rescueImageId, tenantId}}` |
| status | Client→Server | Query current step |
| cancel | Client→Server | Abort rescue operation |
| progress | Server→Client | Step update: `{step, message, progress, instanceId}` |
| error | Server→Client | Error details |

**Rescue Flow (10 steps):**
1. `get_instance` (5%) — Get instance info
2. `stop` (15-25%) — Stop instance, wait for STOPPED state
3. `detach_original` (30-40%) — Detach boot volume
4. `attach_rescue` (50-60%) — Attach rescue boot volume
5. `start_rescue` (70-80%) — Start with rescue volume → `user_action`: SSH reinstall
6. `stop_rescue` (82-85%) — Stop instance
7. `detach_rescue` (88-92%) — Detach rescue volume
8. `reattach_original` (95-98%) — Reattach original boot volume
9. `start_final` (99%) — Start instance
10. `complete` (100%) — Rescue complete

**Test Plan:**
1. Connect to `/ws/rescue`
2. Send `init` with valid instance ID
3. Verify progress updates arrive in order
4. Send `cancel` mid-flow and verify cleanup
5. Send `status` during `user_action` and verify current step

---

## 5. GCP Compute Integration (gcp/service.go)

Status: Framework with service account auth, instance launch API. Full OAuth2 token exchange requires `google.golang.org/api/compute/v1`.

**BootTaskStore** interface: ListBootTasks, CreateBootTask, UpdateBootTask, DeleteBootTask, GetBootTask.

**InMemoryStore** for standalone/dev use.

**Test Plan:**
1. Set GCP_SERVICE_ACCOUNT_JSON in system config
2. Call CreateBootTask with valid GCP parameters
3. Verify task appears in ListBootTasks
4. Verify LaunchGcpInstance creates instance (requires OAuth2 integration)

---

## 6. Monitor Agent (monitor_agent.sh + monitor_api.go)

Embedded bash script with template substitution: `{{SERVER_URL}}`, `{{TOKEN}}`, `{{INTERVAL}}`.

**GET /api/monitor/download?token=&interval=10**
Downloads customized agent script.

**POST /api/monitor/report** (public)
Receives JSON metrics from agent → broadcasts to dashboard via WebSocket.

**Metrics reported:** host (name, os, kernel, uptime), cpu (cores, usage, model, load), memory (total, used, swap), disk (total, used), network (interface, rx/tx rate, rx/tx total).

**Test Plan:**
1. Download agent: `curl "http://host:9856/api/monitor/download?token=test" -o agent.sh`
2. Verify template substitution (SERVER_URL, INTERVAL)
3. Run `DEBUG=true bash agent.sh` to verify JSON output
4. Start agent and verify POST to `/api/monitor/report`
5. Connect to `/ws/monitor` and verify broadcast of received metrics

---

## 7. Notification Channels (channels.go)

### DingTalk (钉钉)
Config keys: `dingtalk.webhook`, `dingtalk.secret` (optional HMAC-SHA256 sign)

### Bark (iOS push)
Config keys: `bark.server` (default: api.day.app), `bark.key`

### Feishu (飞书/Lark)
Config keys: `feishu.webhook`, `feishu.secret` (optional HMAC-SHA256 sign)

### MultiNotifier
Fans out to all configured channels in parallel.

**Test Plan:**
1. Configure each channel in system config
2. Trigger grab success → verify notification arrives on all channels
3. Trigger traffic alert → verify notification on all channels
4. Test with empty config → verify "log-only" behavior (no errors)

---

## 8. EdgeOne DNS Provider (edgeone.go)

Tencent Cloud EdgeOne DNS API via `teo.tencentcloudapi.com`.

Config keys: `edgeone.secretId`, `edgeone.secretKey`, `edgeone.zoneId`

Methods: ListRecords, CreateRecord, UpdateRecord, DeleteRecord.

**Test Plan:**
1. Set EdgeOne credentials in system config
2. Call ListRecords → verify zone records returned
3. Create TXT record for ACME challenge
4. Delete TXT record after challenge
5. Verify Cloudflare still works as primary provider (EdgeOne is secondary)

---

## 9. Cached OCI Computer Info Path (launcher.go)

`computer_create_json` 缓存结构，后续同任务抢机跳过全量资源发现：

```json
{
  "availabilityDomain": "Uocm:PHX-AD-1",
  "shape": "VM.Standard.A1.Flex",
  "imageId": "ocid1.image...",
  "subnetId": "ocid1.subnet...",
  "nsgId": "ocid1.networksecuritygroup...",
  "architecture": "ARM",
  "region": "phx"
}
```

**流程:**
1. `createInstanceData()` → `FindComputerInfoByBootIDStr` 查缓存
2. 命中 → `launchFromCache()` 跳过 AD/Shape/Image/VCN/Subnet/NSG 发现
3. 未命中 → `launchWithDiscovery()` 全量发现 → 成功后 `cacheDiscoveredResources()` 写缓存
4. `GrabResult` 新增 5 字段: `AvailabilityDomain`, `Shape`, `ImageID`, `SubnetID`, `NsgID`

---

## 10. OCI Rescue Operations (instance_control.go + main.go helpers)

5 个救援操作函数已从 stub 升级为真实 OCI SDK 调用：

| 函数 | SDK 调用 |
|------|---------|
| `StopInstance` | `Compute.InstanceAction(STOP)` |
| `StartInstance` | `Compute.InstanceAction(START)` |
| `DetachBootVolume` | `Compute.ListBootVolumeAttachments` → `DetachBootVolume` |
| `AttachBootVolume` | `Compute.AttachBootVolume` (3 参数，无 AvailabilityDomain) |
| `AttachRescueVolume` | 直接挂载已有 boot volume OCID |

**Helper 函数 (main.go):**
- `lookupTenantCreds(store, tenantID)` — 查 `tenant` 表获取 OCI 凭证
- `ociOpFromTenant(store, proxy, key, tenantID, instanceID, op)` — 通用操作模板
- `ociDetachFromTenant` — 查实例 → 列引导卷挂载 → 分离
- `ociAttachFromTenant` — 查实例 → 挂载指定 boot volume
- `ociAttachRescueFromTenant` — 直接挂载救援 boot volume

全部通过 `oci.WithProxy` 创建租户级 OCI 客户端。

---

## 11. Frontend Pages — Console & Rescue

### /console — VNC 控制台
- 输入实例 ID → WebSocket `/ws/console` → 显示 VNC 地址/端口
- 复制地址按钮，VNC 客户端连接说明
- 使用步骤引导（SSH 隧道代理 VNC）

### /rescue — 实例救援
- 输入实例 ID + 救援类型(DD/NetBoot) → WebSocket `/ws/rescue`
- `el-progress` 实时进度条 → `el-timeline` 操作历史
- 取消/完成按钮控制救援流程
- `rescue.go` 新增 `handleComplete` 方法，处理前端 `complete` 消息（触发步骤 7-10 还原引导卷）

---

## 12. SSL Certificate Issuance (sslIssue + CertManager)

`POST /ssl/issue {domain, email}` 现在真实调用 ACME 申请证书：

1. 检查 Cloudflare 配置 (`cloudflare.email` + `cloudflare.api.key`)
2. 调用 `deps.CertManager.ObtainCertificate(domain, email, cfEmail, cfKey, staging)`
3. 证书存入 system_config: `ssl.certificate`, `ssl.privateKey`, `ssl.notAfter`
4. `sysconf.Service` 新增 `SetString(key, value)` 方法

**依赖注入链:** `main.go` → `httpapi.Deps.CertManager` → `sslIssue` handler

---

## 13. Frontend — Dashboard & System Settings (completed earlier)

### Dashboard.vue
- 4 统计卡片（实例数/租户数/父池/API池）+ 引擎状态 + 通知渠道状态 + 快捷入口
- 数据源: `/api/stats`, `/boot/systemStatus`, `/api/config/message-enabled`

### SystemSettings.vue
- 6 组配置卡片: 4 通知渠道 + 2 DNS 服务 + SSL + 其他 (MFA/Turnstile/GCP/OAuth)
- 数据源: `/system/config`

### configMessageEnabled
- 检查 Telegram/DingTalk/Bark/Feishu 四项配置，返回 per-channel 状态

## 14. GCP OAuth2 Token Exchange (2026-06-27)

### 14.1 GCP JWT Auth (`cloud/gcp/auth.go`)
- 纯标准库实现：`crypto/rsa` + `crypto/x509` + `encoding/json` + `net/http`
- 解析 GCP service account JSON → 提取 `client_email` + `private_key`
- RS256 JWT 签名 → POST `https://oauth2.googleapis.com/token` → access token
- 自动缓存 token，过期前 30s 自动刷新
- `HTTPClient()` 返回带 `Authorization: Bearer` 的 `*http.Client`

### 14.2 GCP LaunchGcpInstance 真正调用 GCP Compute API
- `POST https://compute.googleapis.com/compute/v1/projects/{p}/zones/{z}/instances`
- 完整的 instance insert body（disks, networkInterfaces, scheduling, metadata/cloud-init）
- 解析 GCP Operation 响应

### 14.3 GCP Service 接入主应用
- `Deps.GcpSvc *gcp.GcpService` 注入
- `main.go` 从 system config (`gcp.serviceAccountJson` + `gcp.projectId`) 初始化
- 新增路由:
  - `POST /boot-instance/gcp/launch` — 启动 GCP 实例
  - `GET /boot-instance/gcp/list` — 列出 GCP boot tasks
  - `GET /boot-instance/gcp/delete` — 删除 GCP boot task
  - `GET /boot-instance/gcp/status` — 检查 GCP 配置状态

### 14.4 后端其他修复
- `POST /dns/sync` 从 501 → 接入 CfClient + SyncFromCloudflare
- `GET /api/google/callback` + `GET /api/google/status` 路由注册
- `POST /system/config/save` 配置编辑端点 + `systemConfigSave` handler

### 14.5 前端优化
- `src/types/api.ts` 共享 TypeScript 类型（DashboardStats, EngineStatus, Instance, BootTask 等）
- Dashboard: 修复 API 响应解包 bug（拦截器已解包但仍在检查 res.code）+ loading/error 状态
- SystemSettings: 修复解包 bug + 编辑弹窗 + 后端保存接口
- 所有表格: 添加 `<el-empty>` 空状态提示
- Instances 详情: 添加「救援模式」「VNC 控制台」操作按钮（带 query 参数导航）
