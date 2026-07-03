# Frontend Enhancement Design — 实例/磁盘/网络管理增强

> 日期：2026-07-03
> 状态：待审批
> 方案：抽离子组件 + Tab 切换

## 1. 目标

基于已实现的 ~136 个后端 OCI SDK 函数，增强现有前端页面的功能覆盖。聚焦三个方向：

1. **实例管理增强** — 流量监控、磁盘管理
2. **网络管理增强** — IPv6、安全规则、VCN、NAT/路由表
3. **Bug 修复** — Dashboard 备份计数、通知历史解析

**约束**：增强现有页面，不新建页面。交互采用混合模式——简单操作内嵌，复杂操作弹窗/抽屉。

## 2. 页面结构变更

### 2.1 InstanceDetail.vue（重构为 Tab）

```
InstanceDetail.vue
├── Tab: 概览（现有内容迁入）
├── Tab: 流量监控 → InstanceTrafficPanel.vue
├── Tab: 磁盘管理 → DiskManagementPanel.vue
└── Tab: 控制台连接（现有内容迁入）
```

### 2.2 VnicManagement.vue（重构为 Tab）

```
VnicManagement.vue
├── Tab: VNIC 管理（增强，内嵌 IPv6 操作）
│   ├── VNIC 表格（每行可展开）
│   │   └── 展开行: 该 VNIC 的 IPv6 地址列表 + 操作
│   └── 操作栏: [批量创建 VNIC+IPv6]
├── Tab: 安全规则 → SecurityRulesPanel.vue
├── Tab: VCN 管理 → VCNPanel.vue
└── Tab: 网络配置 → NetworkConfigPanel.vue
```

### 2.3 TenantDetail.vue（不变，仅 Bug 修复）

## 3. 组件设计

### 3.1 InstanceTrafficPanel.vue

**位置**：InstanceDetail → 流量监控 Tab

**数据来源**：`GET /api/instances/traffic?instanceId=xxx`

**展示内容**：
- 今日入站/出站流量（GB）— 两个 Card
- 本月入站/出站流量（GB）— 两个 Card
- 数据来源：`VnicFromNetworkBytes` / `VnicToNetworkBytes` 指标

**交互**：纯展示，内嵌 Card，无弹窗。

### 3.2 DiskManagementPanel.vue

**位置**：InstanceDetail → 磁盘管理 Tab

**子面板结构**：

```
DiskManagementPanel
├── 启动卷信息 Card（只读）
│   ├── 名称、OCID
│   ├── 大小 (GB)
│   ├── VPU/GB（当前性能级别）
│   └── 架构
│
├── 启动卷操作 Card
│   ├── [调整 VPU] → SelectVpuModal
│   ├── [调整大小] → ResizeVolumeModal
│   └── [创建备份] → CreateBackupModal
│
└── 备份列表 Card
    ├── 表格: 备份名、创建时间、大小、状态
    ├── [删除] → 确认弹窗
    └── [跨区域复制] → CopyBackupModal
```

**弹窗组件**：

| 组件 | 用途 | 关键字段 |
|------|------|----------|
| `SelectVpuModal` | 选择 VPU 级别 | Radio: 10(Balanced) / 20(Higher) / 30-120(Ultra)，显示价格提示 |
| `ResizeVolumeModal` | 调整磁盘大小 | InputNumber，只能增不能减，显示当前大小 |
| `CreateBackupModal` | 创建备份 | 输入备份名称，确认创建 |
| `CopyBackupModal` | 跨区域复制 | 选择目标区域（下拉），输入备份名 |

**后端 API**：
- 启动卷信息：`GET /api/instances/:id`（已有，含 bootVolumeId/Name/Size/Vpu）
- 调整 VPU：`POST /api/boot-volume/vpu`（需确认 handler）
- 调整大小：`POST /api/boot-volume/resize`（**需新增 handler**）
- 创建备份：`POST /api/backup/create`（需确认 handler）
- 备份列表：`GET /api/backup/list?bootVolumeId=xxx`（已有）
- 删除备份：`GET /api/backup/delete?id=xxx`（已有）
- 跨区域复制：`POST /api/backup/copy`（需确认 handler）

### 3.3 IPv6 管理（内嵌于 VNIC 管理表格）

**位置**：VnicManagement → VNIC 管理 Tab → 每个 VNIC 行可展开

**设计思路**：IPv6 是 VNIC 的子资源，不单独成 Tab，而是嵌入到 VNIC 表格的展开行中。每个 VNIC 行点击展开后，显示该 VNIC 的 IPv6 地址列表和操作按钮。

**数据来源**：
- VNIC 列表（含 IPv6）：`GET /api/vnic/list`（已有）
- IPv6 操作：`POST /api/vnic/ipv6/assign`、`POST /api/vnic/ipv6/delete`
- 子网检查：`GET /api/vnic/ipv6/check-subnet`

**交互布局**：

```
VNIC 管理 Tab
┌─────────────────────────────────────────────────────────────────┐
│  操作栏: [批量创建 VNIC+IPv6]                                    │
├─────────────────────────────────────────────────────────────────┤
│  ▼ vnic-instance1-1    10.0.0.5    129.213.x.x    [删除VNIC]    │
│    ├─────────────────────────────────────────────────────────   │
│    │  IPv6 地址:                                                 │
│    │  ├── 2603:c020:400:xxxx::1              [删除]              │
│    │  └── 2603:c020:400:xxxx::2              [删除]              │
│    │                                                             │
│    │  [分配 IPv6]    [删除全部 IPv6]                              │
│    └─────────────────────────────────────────────────────────   │
│                                                                 │
│  ▼ vnic-instance1-2    10.0.0.6    —               [删除VNIC]    │
│    ├─────────────────────────────────────────────────────────   │
│    │  IPv6 地址: 无                                              │
│    │                                                             │
│    │  [分配 IPv6]                                                │
│    └─────────────────────────────────────────────────────────   │
│                                                                 │
│  ▲ vnic-instance1-3    10.0.0.7    129.213.x.x    [删除VNIC]    │
│    （折叠状态，点击展开）                                         │
└─────────────────────────────────────────────────────────────────┘
```

**展开行内容**：
- IPv6 地址表格（地址、操作列）
- [分配 IPv6] 按钮 — 调用 `AssignIpv6ToVnic`，成功后刷新列表
- [删除全部 IPv6] 按钮 — 确认弹窗，调用 `DeleteAllIpv6FromVnic`
- 子网不支持 IPv6 时显示提示："当前子网未启用 IPv6，需先在 OCI 控制台配置 IPv6 CIDR"

**弹窗组件**：

| 组件 | 用途 | 关键字段 |
|------|------|----------|
| `BatchCreateModal` | 批量创建 VNIC+IPv6 | VNIC 数(1-32)、IPv6数/VNIC(0-32)、子网选择 |

**后端 API**：
- VNIC 列表（含 IPv6）：`GET /api/vnic/list`（已有）
- 分配 IPv6：`POST /api/vnic/ipv6/assign`（需确认 handler）
- 删除 IPv6：`POST /api/vnic/ipv6/delete`（需确认 handler）
- 子网 IPv6 检查：`GET /api/vnic/ipv6/check-subnet`（需确认 handler）
- 批量创建：`POST /api/vnic/batch-create`（需确认 handler）

### 3.4 SecurityRulesPanel.vue

**位置**：VnicManagement → 安全规则 Tab

**数据来源**：
- `GET /api/security/rules?type=ingress`
- `GET /api/security/rules?type=egress`

**布局**：

```
SecurityRulesPanel
├── 操作栏
│   ├── [一键启用全部规则] → ConfirmModal（警告提示）
│   ├── [启用 IPv6 规则] → ConfirmModal
│   └── [添加规则] → AddRuleModal
│
├── 入站规则表格
│   ├── 列: 协议、源 CIDR、端口、ICMP 类型、操作
│   └── 每行: [删除] → 确认弹窗
│
└── 出站规则表格
    ├── 列: 协议、目标 CIDR、端口、操作
    └── 每行: [删除] → 确认弹窗
```

**AddRuleModal 表单**：

| 字段 | 类型 | 说明 |
|------|------|------|
| 类型 | Radio | 入站 / 出站 |
| 协议 | Select | all / tcp / udp / icmp |
| CIDR | Input | 默认 `0.0.0.0/0` |
| 端口 | Input | TCP/UDP 时显示，支持单端口或范围 `8080-9090` |
| ICMP Type | Input | ICMP 时显示，格式 `type, code`（默认 `8, 0`） |

**后端 API**：
- 列出规则：`GET /api/security/rules`（已有）
- 添加规则：`POST /api/security/rules/add`（已有）
- 删除规则：`GET /api/security/rules/delete`（已有）
- 一键启用：`POST /api/security/rules/enable-all`（需确认 handler）
- 启用 IPv6：`POST /api/security/rules/enable-ipv6`（需确认 handler）

### 3.5 VCNPanel.vue

**位置**：VnicManagement → VCN 管理 Tab

**数据来源**：
- `GET /api/vcn/list`
- 安全列表、子网等关联数据

**布局**：

```
VCNPanel
├── VCN 列表 Card
│   ├── 表格: 名称、OCID、CIDR、DNS 标签
│   └── 点击行展开详情
│
├── VCN 详情 Card（选中 VCN 后显示）
│   ├── 基本信息: 名称、CIDR、创建时间
│   ├── 关联资源:
│   │   ├── 安全列表数量（链接到安全规则 Tab）
│   │   ├── 子网数量
│   │   ├── NAT 网关数量
│   │   └── 路由表数量
│   └── 操作:
│       ├── [配置 IPv6 安全规则] → ConfigureIPv6Modal
│       └── [查看安全列表] → 跳转安全规则 Tab
│
└── 公网 IP 管理 Card
    ├── 当前公网 IP 显示
    └── [重新分配公网 IP] → ReassignIPModal
```

**弹窗组件**：

| 组件 | 用途 | 关键字段 |
|------|------|----------|
| `ConfigureIPv6Modal` | 配置 IPv6 安全规则 | 确认将添加 ICMPv6 + SSH + egress 规则 |
| `ReassignIPModal` | 重新分配公网 IP | 警告：旧 IP 将失效 |

**后端 API**：
- VCN 列表：`GET /api/vcn/list`（需确认 handler）
- 配置 IPv6 安全规则：`POST /api/vcn/configure-ipv6`（需确认 handler）
- 重新分配公网 IP：`POST /api/vcn/reassign-ip`（需确认 handler）

### 3.6 NetworkConfigPanel.vue

**位置**：VnicManagement → 网络配置 Tab

**布局**：

```
NetworkConfigPanel
├── NAT 网关 Card
│   ├── 列表: 名称、OCID、状态、操作
│   ├── [创建 NAT 网关] → CreateNatModal
│   └── 每行: [删除] → 确认弹窗
│
└── 路由表 Card
    ├── 列表: 名称、关联 NAT 网关、规则数、操作
    ├── [创建路由表] → CreateRouteTableModal
    ├── [更新实例路由表] → 选择路由表
    └── [重置为默认路由表] → 确认弹窗
```

**后端 API**：
- NAT 网关 CRUD：`POST /api/nat/create`、`GET /api/nat/delete`（需确认 handler）
- 路由表 CRUD：`POST /api/route-table/create`、`GET /api/route-table/delete`（需确认 handler）
- 更新 VNIC 路由表：`POST /api/route-table/update-vnic`（需确认 handler）
- 重置默认路由表：`POST /api/route-table/reset`（需确认 handler）

## 4. Bug 修复

| Bug | 位置 | 修复方案 |
|-----|------|----------|
| Dashboard 备份计数未显示 | `Dashboard.vue` | 在统计卡片中渲染 `backupCount` |
| 通知历史解析错误 | `SystemSettings.vue` | 修复响应解析：`data.history` 而非 `data` |
| 通知历史缺少频道筛选 | `SystemSettings.vue` | 添加频道下拉筛选器 |

## 5. 后端 Handler 确认清单

以下 SDK 函数需确认 HTTP handler 是否已注册到 `server.go` 路由：

### 5.1 需确认的 Handler

| 功能 | SDK 函数 | 预期路由 | 状态 |
|------|----------|----------|------|
| 启动卷 VPU 调整 | `UpdateBootVolumeVpu` | `POST /api/boot-volume/vpu` | 待确认 |
| 启动卷大小调整 | — | `POST /api/boot-volume/resize` | **需新增** |
| 创建备份 | `CreateBootVolumeBackup` | `POST /api/backup/create` | 待确认 |
| 跨区域复制 | `CopyBootVolumeBackup` | `POST /api/backup/copy` | 待确认 |
| IPv6 分配 | `AssignIpv6ToVnic` | `POST /api/vnic/ipv6/assign` | 待确认 |
| IPv6 删除 | `DeleteAllIpv6FromVnic` | `POST /api/vnic/ipv6/delete` | 待确认 |
| 子网 IPv6 检查 | `CheckSubnetIpv6Support` | `GET /api/vnic/ipv6/check-subnet` | 待确认 |
| 批量创建 VNIC | `CreateMultipleVnicsWithIpv6` | `POST /api/vnic/batch-create` | 待确认 |
| VCN 列表 | `ListVcns` | `GET /api/vcn/list` | 待确认 |
| 配置 IPv6 规则 | `ConfigureIPv6SecurityRules` | `POST /api/vcn/configure-ipv6` | 待确认 |
| 重分配公网 IP | `ReassignPublicIP` | `POST /api/vcn/reassign-ip` | 待确认 |
| 一键启用规则 | `EnableAllForTenant` | `POST /api/security/rules/enable-all` | 待确认 |
| 启用 IPv6 规则 | `EnableIPv6ForTenant` | `POST /api/security/rules/enable-ipv6` | 待确认 |
| NAT 网关 CRUD | `CreateOrGetNatGateway` / `DeleteNatGateway` | `POST /api/nat/create` / `GET /api/nat/delete` | 待确认 |
| 路由表 CRUD | `CreateOrGetNatRouteTable` / `DeleteRouteTable` | `POST /api/route-table/create` / `GET /api/route-table/delete` | 待确认 |
| 更新 VNIC 路由表 | `UpdateInstanceVnicRouteTable` | `POST /api/route-table/update-vnic` | 待确认 |
| 重置默认路由表 | `ResetVnicToDefaultRouteTable` | `POST /api/route-table/reset` | 待确认 |

### 5.2 需新增的 Handler

| 功能 | 说明 |
|------|------|
| `POST /api/boot-volume/resize` | 调整启动卷大小。需要：bootVolumeId、newSizeGB。调用 `BlockstorageClient.UpdateBootVolume` 的 `SizeInGBs` 字段。只能增大不能缩小。 |

## 6. 组件文件结构

```
frontend/src/components/
├── instance/
│   ├── InstanceTrafficPanel.vue
│   ├── DiskManagementPanel.vue
│   ├── SelectVpuModal.vue
│   ├── ResizeVolumeModal.vue
│   ├── CreateBackupModal.vue
│   └── CopyBackupModal.vue
├── vnic/
│   ├── VnicIpv6Row.vue          ← VNIC 展开行内的 IPv6 管理组件
│   ├── SecurityRulesPanel.vue
│   ├── VCNPanel.vue
│   ├── NetworkConfigPanel.vue
│   ├── AddRuleModal.vue
│   ├── BatchCreateModal.vue
│   ├── ConfigureIPv6Modal.vue
│   ├── ReassignIPModal.vue
│   ├── CreateNatModal.vue
│   └── CreateRouteTableModal.vue
└── common/
    └── ConfirmModal.vue（通用确认弹窗，如不存在则新建）
```

## 7. API 类型定义

需在 `frontend/src/types/api.ts` 中补充以下类型：

```typescript
// 流量监控
interface TrafficData {
  ingressToday: number    // GB
  egressToday: number     // GB
  ingressMonth: number    // GB
  egressMonth: number     // GB
}

// 备份
interface BootVolumeBackup {
  id: string
  displayName: string
  bootVolumeId: string
  sizeInGBs: number
  lifecycleState: string
  timeCreated: string
}

// 安全规则
interface SecurityRule {
  id: string
  type: 'ingress' | 'egress' | '入站' | '出站'
  protocol: string
  source: string
  ports: string
  icmpType?: string
}

// NAT 网关
interface NatGateway {
  id: string
  displayName: string
  vcnId: string
  lifecycleState: string
}

// 路由表
interface RouteTable {
  id: string
  displayName: string
  vcnId: string
  routeRules: RouteRule[]
}

interface RouteRule {
  destination: string
  destinationType: string
  networkEntityId: string
}

// VCN
interface VcnInfo {
  id: string
  displayName: string
  cidrBlock: string
  dnsLabel: string
  defaultSecurityListId: string
  defaultRouteTableId: string
  timeCreated: string
}
```

## 8. 实现优先级

| 阶段 | 内容 | 工作量 |
|------|------|--------|
| P1 | Tab 框架重构（InstanceDetail + VnicManagement） | 中 |
| P2 | 磁盘管理面板（VPU/备份/大小调整） | 中 |
| P3 | VNIC 表格增强（展开行 IPv6 管理） | 中 |
| P4 | 安全规则面板 | 中 |
| P5 | VCN 管理面板 | 小 |
| P6 | 网络配置面板（NAT/路由表） | 中 |
| P7 | 流量监控面板 | 小 |
| P8 | Bug 修复 | 小 |
| P9 | 后端 handler 补充（如需） | 小-中 |
