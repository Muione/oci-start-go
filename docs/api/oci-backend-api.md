# OCI 后端 API 功能清单

> 本文档记录 oci-start-go 后端已实现的 OCI SDK 功能，供前端开发和功能 Review 参考。
>
> 最后更新：2026-07-03

---

## 目录

1. [认证与凭据管理](#1-认证与凭据管理)
2. [实例管理](#2-实例管理)
3. [VNIC 管理](#3-vnic-管理)
4. [网络管理](#4-网络管理)
5. [IAM 管理](#5-iam-管理)
6. [存储管理](#6-存储管理)
7. [备份管理](#7-备份管理)
8. [监控与审计](#8-监控与审计)
9. [配额与区域](#9-配额与区域)
10. [控制台连接](#10-控制台连接)
11. [代理池](#11-代理池)

---

## 1. 认证与凭据管理

**源文件**: `internal/oci/provider.go`, `internal/oci/credentials.go`

### 1.1 Provider 构建

| 函数 | 说明 |
|------|------|
| `NewProvider(creds, masterKey)` | 从租户凭据构建 OCI ConfigurationProvider。凭据包括 Tenancy OCID、User OCID、Fingerprint、Region、加密的 API 私钥。私钥通过 AES-256-GCM 解密后传入 SDK。 |
| `NewClients(prov)` | 从 Provider 构建所有 OCI 服务客户端集合（Compute、VCN、Identity、ObjectStorage、Blockstorage、Limits、Audit、NLB、Email、OspGateway、UsageApi）。 |
| `NewClientsWithHTTPClient(prov, hc)` | 构建客户端并注入自定义 HTTP Client，用于代理路由。 |

### 1.2 API Key 管理

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ListApiKeys(ctx, prov, tenancyOCID, userOCID)` | 用户 OCID | `[]ApiKeyInfo` | 列出指定用户的所有 API Key，包含 ID、Fingerprint、公钥内容。通过 Identity Domains SCIM API 实现。 |
| `CreateApiKey(ctx, prov, tenancyOCID, userOCID, key)` | PEM 公钥内容 | `*ApiKeyInfo` | 为用户创建新的 API Key。返回创建后的 Key 信息。 |
| `DeleteApiKey(ctx, prov, tenancyOCID, keyID)` | Key ID | `error` | 删除指定 API Key。 |

### 1.3 Auth Token 管理

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ListAuthTokens(ctx, prov, tenancyOCID, userOCID)` | 用户 OCID | `[]AuthTokenInfo` | 列出用户的所有 Auth Token（用于 API 认证）。 |
| `CreateAuthToken(ctx, prov, tenancyOCID, userOCID, desc)` | 描述 | `*AuthTokenInfo` | 创建新的 Auth Token。**Token 值仅在创建时返回一次**，之后无法再获取。 |
| `DeleteAuthToken(ctx, prov, tenancyOCID, userOCID, tokenID)` | Token ID | `error` | 删除指定 Auth Token。 |

### 1.4 SMTP 凭据管理

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ListSmtpCredentials(ctx, prov, tenancyOCID, userOCID)` | 用户 OCID | `[]SmtpCredentialInfo` | 列出用户的所有 SMTP 凭据（用于邮件发送）。 |
| `CreateSmtpCredential(ctx, prov, tenancyOCID, userOCID, desc)` | 描述 | `*SmtpCredentialInfo` | 创建 SMTP 凭据。**密码仅在创建时返回一次**。 |
| `DeleteSmtpCredential(ctx, prov, tenancyOCID, userOCID, credID)` | 凭据 ID | `error` | 删除指定 SMTP 凭据。 |

### 1.5 Customer Secret Key 管理

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ListCustomerSecretKeys(ctx, prov, tenancyOCID, userOCID)` | 用户 OCID | `[]CustomerSecretKeyInfo` | 列出用户的所有 Customer Secret Key（用于 S3 兼容 API）。 |
| `CreateCustomerSecretKey(ctx, prov, tenancyOCID, userOCID, name)` | 名称 | `*CustomerSecretKeyInfo` | 创建 Customer Secret Key。**Secret 仅在创建时返回一次**。 |
| `DeleteCustomerSecretKey(ctx, prov, tenancyOCID, userOCID, keyID)` | Key ID | `error` | 删除指定 Customer Secret Key。 |

---

## 2. 实例管理

**源文件**: `internal/oci/compute.go`, `internal/oci/instance_control.go`, `internal/oci/instance_sync.go`

### 2.1 实例查询

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ListInstances(ctx, c, compartmentID)` | Compartment OCID | `[]core.Instance` | 列出指定 Compartment 下的所有实例。自动分页（每页 100 个）。返回 OCI SDK 原生 Instance 对象。 |
| `GetInstance(ctx, c, instanceID)` | 实例 OCID | `core.Instance` | 获取单个实例的详细信息，包括 DisplayName、Shape、LifecycleState、ShapeConfig 等。 |
| `GetInstanceFull(ctx, c, instanceID, compartmentID)` | 实例 OCID + Compartment OCID | `*InstanceFull` | 获取实例完整信息，包括主 VNIC 详情（公网 IP、私有 IP、IPv6）、启动卷信息、架构类型（ARM/AMD）。**推荐前端使用此函数获取实例详情**。 |
| `GetInstanceState(ctx, c, instanceID)` | 实例 OCID | `string` | 获取实例当前生命周期状态。返回值：`PROVISIONING`、`RUNNING`、`STARTING`、`STOPPING`、`STOPPED`、`TERMINATING`、`TERMINATED`。 |
| `ListInstancesByTenant(ctx, tenantID, creds, masterKey)` | 租户 ID + 凭据 | `[]repo.InsertInstanceDetailParams` | 枚举租户下所有 Compartment（根 + 子 Compartment）的非终止实例，组装为数据库行。用于同步功能。 |

### 2.2 实例生命周期控制

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `StartInstance(ctx, c, instanceID)` | 实例 OCID | `error` | 启动已停止的实例。异步操作，调用后需轮询状态。 |
| `StopInstance(ctx, c, instanceID)` | 实例 OCID | `error` | 停止运行中的实例。异步操作。 |
| `RebootInstance(ctx, c, instanceID)` | 实例 OCID | `error` | 重启实例（软重启）。 |
| `ResetInstance(ctx, c, instanceID)` | 实例 OCID | `error` | 强制重置实例（Stop → 等待 STOPPED → Start → 等待 RUNNING）。用于 IPv6 地址生效等场景。内部轮询，超时 300 秒。 |
| `TerminateInstance(ctx, c, instanceID, preserveBootVolume)` | 实例 OCID + 是否保留启动卷 | `error` | 终止实例。`preserveBootVolume=true` 时保留启动卷（可用于恢复）。**此操作不可逆**。 |

### 2.3 实例配置变更

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `UpdateInstanceShape(ctx, c, instanceID, shapeName, ocpus, memoryGB)` | 实例 OCID + 新规格 | `error` | 修改实例的 Shape 规格（CPU/内存）。实例需先停止。支持 ARM (A1) 和 AMD 规格变更。 |

### 2.4 启动卷附件

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ListBootVolumeAttachments(ctx, c, compartmentID, instanceID, ad)` | Compartment + 实例 + AD | `[]core.BootVolumeAttachment` | 列出实例的启动卷附件。 |
| `AttachBootVolume(ctx, c, instanceID, bootVolumeID, ad)` | 实例 + 启动卷 + AD | `error` | 将启动卷挂载到实例。 |
| `DetachBootVolume(ctx, c, attachmentID)` | 附件 ID | `error` | 卸载启动卷。 |

### 2.5 辅助函数

| 函数 | 说明 |
|------|------|
| `instanceArchitecture(shape)` | 从 ShapeConfig.ProcessorDescription 推断架构标签：`ARM`（A1/Ampere）、`AMD`（AMD/Intel）、`NONE`（未知）。 |
| `waitForInstanceState(ctx, c, instanceID, target, timeout, interval)` | 轮询实例状态直到达到目标状态或超时。内部使用，超时 300 秒，间隔 3 秒。 |

---

## 3. VNIC 管理

**源文件**: `internal/oci/vnic.go`, `internal/oci/network.go`

### 3.1 VNIC 查询

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ListVnicAttachmentsForInstance(ctx, computeClient, compartmentID, instanceID)` | 实例 OCID | `[]core.VnicAttachment` | 列出实例的所有 VNIC 附件（包括主 VNIC 和辅助 VNIC）。自动分页。 |
| `GetVnicInfo(ctx, vcnClient, vnicID)` | VNIC OCID | `*VnicAttachmentInfo` | 获取 VNIC 详情：公网 IP、私有 IP、IPv6 地址列表、子网 ID。 |
| `ListAllVnicsForInstance(ctx, computeClient, vcnClient, compartmentID, instanceID)` | 实例 OCID | `[]VnicAttachmentInfo` | 获取实例所有 VNIC 的完整信息（IP、IPv6）。逐个解析，单个 VNIC 失败不影响其他。 |
| `GetPrimaryVnic(ctx, c, instanceID, compartmentID)` | 实例 OCID | `core.Vnic` | 获取实例的主 VNIC。单次遍历优化：优先返回 IsPrimary=true 的 VNIC，否则返回第一个可达的 VNIC。 |
| `IsPrimaryVnic(ctx, computeClient, compartmentID, instanceID, vnicID)` | VNIC OCID | `bool` | 判断指定 VNIC 是否为实例的主 VNIC（基于最早 timeCreated 的附件）。 |

### 3.2 VNIC 创建（批量）

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `CreateMultipleVnicsWithIpv6(ctx, c, instanceID, subnetID, vnicCount, ipv6CountPerVnic)` | 实例 + 子网 + VNIC 数 + IPv6 数/VNIC | `*BatchVnicCreationResult` | 批量创建辅助 VNIC 并分配 IPv6 地址。**限制：VNIC 数 1-32，IPv6 数/VNIC 0-32**。返回详细结果：成功数、失败原因、耗时。 |
| `CreateSingleVnicWithIpv6(ctx, c, instanceID, subnetID, displayName, ipv6Count, subnetSupportsIpv6)` | 实例 + 子网 + 显示名 + IPv6 数 | `*VnicCreationResult` | 创建单个辅助 VNIC。内部流程：AttachVnic → 等待 ATTACHED → 获取 IP → 创建 IPv6。 |
| `ValidateVnicCreationParams(vnicCount, ipv6CountPerVnic)` | VNIC 数 + IPv6 数 | `error` | 验证 VNIC/IPv6 创建参数是否在有效范围内。 |

### 3.3 VNIC 删除

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `DeleteVnicWithIpv6(ctx, c, compartmentID, instanceID, vnicID)` | VNIC OCID | `(bool, error)` | 删除单个辅助 VNIC 及其所有 IPv6 地址。**禁止删除主 VNIC**。流程：删除 IPv6 → 卸载 VNIC → 等待 DETACHED。 |
| `DeleteAllSecondaryVnics(ctx, c, instanceID, compartmentID)` | 实例 OCID | `map[string]bool` | 删除实例的所有辅助 VNIC（保留主 VNIC）。返回每个 VNIC 的删除结果。 |
| `DeleteAllIpv6FromVnic(ctx, vcnClient, vnicID)` | VNIC OCID | `(bool, error)` | 删除 VNIC 上的所有 IPv6 地址。 |
| `DetachVnicFromInstance(ctx, computeClient, instanceID, vnicID)` | VNIC OCID | `(bool, error)` | 卸载 VNIC 附件并等待 DETACHED 状态。 |

### 3.4 IPv6 管理

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `AssignIpv6ToVnic(ctx, vcnClient, vnicID, forceNew)` | VNIC OCID + 是否强制刷新 | `string` | 为 VNIC 分配 IPv6 地址。`forceNew=true` 时先释放所有现有 IPv6 再分配新的。返回 IPv6 地址。 |
| `CreateIpv6ForVnic(ctx, vcnClient, vnicID, count)` | VNIC OCID + 数量 | `[]Ipv6CreationResult` | 批量创建 IPv6 地址。返回每个地址的创建结果。 |
| `CheckSubnetIpv6Support(ctx, vcnClient, subnetID)` | 子网 OCID | `(bool, error)` | 检查子网是否已配置 IPv6 CIDR。 |

### 3.5 等待/轮询

| 函数 | 说明 |
|------|------|
| `WaitForVnicAttachment(ctx, computeClient, attachmentID, timeout, interval)` | 轮询 VNIC 附件直到 ATTACHED。NotFound 视为"尚未创建"继续轮询。超时 300 秒。 |
| `WaitForVnicDetachment(ctx, computeClient, attachmentID, timeout, interval)` | 轮询 VNIC 附件直到 DETACHED 或 NotFound（视为成功删除）。超时 300 秒。 |

---

## 4. 网络管理

**源文件**: `internal/oci/network.go`, `internal/oci/security_list.go`, `internal/oci/nlb.go`

### 4.1 VCN 管理

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ListVcns(ctx, c, compartmentID)` | Compartment OCID | `[]core.Vcn` | 列出 Compartment 下的所有 VCN。 |

### 4.2 公网 IP 管理

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ReassignPublicIP(ctx, c, compartmentID, instanceID)` | 实例 OCID | `string` | 重新分配公网 IP。流程：获取主 VNIC → 删除旧的 Reserved Public IP → 在主 VNIC 的私有 IP 上创建新的 Reserved Public IP。返回新公网 IP 地址。 |

### 4.3 NAT 网关

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `CreateOrGetNatGateway(ctx, vcnClient, compartmentID, vcnID, displayName)` | VCN OCID + 显示名 | `*core.NatGateway` | 查找或创建 NAT 网关。按 DisplayName 查找已有网关，不存在则创建。 |
| `DeleteNatGateway(ctx, vcnClient, natGatewayID)` | NAT 网关 OCID | `error` | 删除 NAT 网关。 |

### 4.4 路由表

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `CreateOrGetNatRouteTable(ctx, vcnClient, compartmentID, vcnID, natGatewayID, displayName)` | NAT 网关 OCID + 显示名 | `*core.RouteTable` | 查找或创建包含 NAT 网关路由规则的路由表。规则：`0.0.0.0/0 → NAT Gateway`。 |
| `UpdateInstanceVnicRouteTable(ctx, c, instanceID, compartmentID, routeTableID)` | 实例 + 路由表 OCID | `error` | 更新实例主 VNIC 的路由表。 |
| `ResetVnicToDefaultRouteTable(ctx, c, instanceID, compartmentID)` | 实例 OCID | `error` | 将主 VNIC 的路由表重置为 VCN 默认路由表。 |
| `DeleteRouteTable(ctx, vcnClient, routeTableID)` | 路由表 OCID | `error` | 删除路由表。 |

### 4.5 安全列表

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ListSecurityRules(ctx, c, compartmentID, ruleType)` | Compartment + 类型 (`ingress`/`egress`) | `[]SecurityRuleDTO` | 列出所有安全列表的入站/出站规则。规则包含：协议、源/目标 CIDR、端口范围、ICMP 类型。 |
| `AddSecurityRule(ctx, c, compartmentID, rule)` | 规则 DTO | `error` | 添加安全规则到第一个安全列表。自动去重（相同协议+CIDR+端口的规则会被替换）。 |
| `DeleteSecurityRule(ctx, c, compartmentID, compositeID)` | 复合 ID `{tenantId}_{ruleIndex}_{type}` | `error` | 删除指定安全规则。使用全局索引跨安全列表定位。 |
| `EnableAllForTenant(ctx, c, compartmentID)` | Compartment OCID | `(bool, error)` | 一键启用全部协议规则：入站 all/0.0.0.0/0、入站 all/::/0（IPv6）、入站 ICMP、出站 all/0.0.0.0/0、出站 all/::/0。返回 IPv6 是否成功。 |
| `EnableIPv6ForTenant(ctx, c, compartmentID)` | Compartment OCID | `error` | 仅启用 IPv6 规则：入站 all/::/0、出站 all/::/0。 |
| `ConfigureIPv6SecurityRules(ctx, c, vcnID)` | VCN OCID | `error` | 配置 IPv6 安全规则到 VCN 默认安全列表：ICMPv6（类型 128,129,133-137）、TCP 22（SSH）、出站 all/::/0。 |

**SecurityRuleDTO 结构**:

```go
type SecurityRuleDTO struct {
    ID       string  `json:"id"`       // 复合 ID: {tenantId}_{index}_{type}
    Type     string  `json:"type"`     // "ingress"/"egress" (请求) 或 "入站"/"出站" (响应)
    Protocol string  `json:"protocol"` // "all", "tcp", "udp", "icmp", "icmpv6"
    Source   string  `json:"source"`   // CIDR: "0.0.0.0/0", "::/0", "10.0.0.0/16"
    Ports    string  `json:"ports"`    // "80", "8080-9090", "" (all)
    ICMPType *string `json:"icmpType"` // "8, 0" 或 nil
}
```

### 4.6 NLB 负载均衡

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `CreateOrGetNetworkLoadBalancer(ctx, nlbClient, compartmentID, subnetID, instanceID, displayName, privateIP)` | 子网 + 实例 + 显示名 | `*NetworkLoadBalancer` | 创建或获取 NLB。配置：Backend Set "amd-1"（FiveTuple 策略，TCP 22 健康检查）、Listener "amd"（TCP+UDP 22）。等待 NLB 变为 ACTIVE。 |
| `DeleteNetworkLoadBalancer(ctx, nlbClient, nlbID)` | NLB OCID | `error` | 删除 NLB。 |
| `ListNetworkLoadBalancers(ctx, nlbClient, compartmentID)` | Compartment OCID | `[]NetworkLoadBalancerSummary` | 列出所有 NLB（ID、DisplayName、SubnetID）。 |
| `WaitForNLBCreation(ctx, nlbClient, nlbID, timeout, interval)` | NLB OCID | `*NetworkLoadBalancer` | 轮询 NLB 直到 ACTIVE 或 FAILED。超时 300 秒。 |

---

## 5. IAM 管理

**源文件**: `internal/oci/identity.go`, `internal/oci/signon_policy.go`, `internal/oci/credentials.go`

### 5.1 Compartment 管理

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ListCompartments(ctx, c, tenancyOCID)` | 租户 OCID | `[]identity.Compartment` | 列出租户下所有活跃子 Compartment（递归）。注意：不包含根 Compartment（租户本身），需要时手动添加。 |

### 5.2 用户管理

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ListUsers(ctx, prov, tenancyOCID)` | 租户 OCID | `[]OciUser` | 列出所有 IAM 用户。返回 OCID、Name、Email、Domain、LifecycleState、创建时间、最后登录时间。 |
| `CreateUser(ctx, prov, tenancyOCID, req)` | CreateUserRequest | `*CreateUserResult` | 创建 IAM 用户。可选加入指定组。返回一次性密码（UI Password）。请求结构：`{Username, Email, GroupName?, GroupOCID?}`。 |
| `DeleteUser(ctx, prov, userOCID)` | 用户 OCID | `error` | 删除 IAM 用户。 |
| `ResetUserPassword(ctx, prov, userOCID)` | 用户 OCID | `string` | 重置用户控制台密码。返回新的一次性密码。 |

### 5.3 组管理

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ListGroups(ctx, prov, tenancyOCID)` | 租户 OCID | `[]OciGroup` | 列出所有 IAM 组。返回 OCID 和 Name。 |

### 5.4 MFA 管理

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `GetMfaStatus(ctx, prov, tenancyOCID)` | 租户 OCID | `*MfaStatus` | 获取当前 MFA 配置状态。返回各因素启用状态：TOTP、Email、SMS、Push、安全问题、FIDO、电话。 |
| `ToggleEmailMFA(ctx, prov, tenancyOCID, enable)` | 是否启用 | `(bool, error)` | 启用/禁用 Email MFA。GET → 修改 → PUT 模式。返回新状态。 |
| `ResetMfaForAllUsers(ctx, prov, tenancyOCID)` | 租户 OCID | `(int, error)` | 重置所有用户的 MFA TOTP 设备。返回删除的设备数。用于账户恢复场景。 |

### 5.5 密码策略

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `GetPasswordPolicy(ctx, prov, tenancyOCID)` | 租户 OCID | `*PasswordPolicyDetail` | 获取密码策略。优先返回 Custom 策略。返回：策略名、是否启用过期、过期天数。 |
| `UpdatePasswordPolicy(ctx, prov, tenancyOCID, enableExpiry, expiryDays)` | 是否启用 + 天数 | `error` | 更新密码过期策略。天数范围 0-365。默认过期 120 天，7 天预警。 |

### 5.6 通知设置

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `GetNotificationRecipients(ctx, prov, tenancyOCID)` | 租户 OCID | `[]NotifRecipient` | 获取通知邮件收件人列表。返回 ID、Email、State。 |
| `UpdateNotificationRecipients(ctx, prov, tenancyOCID, emails)` | 邮箱列表 | `error` | 替换通知收件人列表。启用 TestMode 并设置 TestRecipients。 |

### 5.7 账户恢复设置

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `GetAccountRecoverySetting(ctx, prov, tenancyOCID)` | 租户 OCID | `*AccountRecoveryInfo` | 获取账户恢复设置。返回启用的恢复因素（email/sms/secquestions/push/totp）、最大错误尝试次数、锁定时长。 |
| `UpdateAccountRecoverySetting(ctx, prov, tenancyOCID, factors)` | 因素列表 | `*AccountRecoveryInfo` | 更新恢复因素。至少需要一个因素。 |

### 5.8 登录策略

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ListSignOnPolicies(ctx, prov, tenancyOCID)` | 租户 OCID | `[]SignOnPolicyInfo` | 列出所有登录策略。返回 ID、Name、Description、Active、OCID。**只读**，不可修改。 |

### 5.9 租户详情

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `GetTenancyDetail(ctx, prov, tenancyOCID)` | 租户 OCID | `*TenancyDetail` | 获取租户基本信息：名称、描述。账户类型和邮箱需通过 OSP Gateway 获取。 |
| `GetSubscriptionDays(ctx, prov, tenancyOCID)` | 租户 OCID | `*SubscriptionDaysInfo` | 计算租户订阅时长。返回创建时间、当前时间、活跃天数/月数/年数。 |
| `ListDomainTenants(ctx, prov, tenancyOCID)` | 租户 OCID | `[]DomainInfo` | 列出所有 Identity Domain。返回 ID、DisplayName、URL、HomeRegion、Type、LicenseType、State。 |
| `PingIdentity(ctx, prov, tenancyOCID)` | 租户 OCID | `error` | 测试凭据有效性。调用 GetTenancy，成功返回 nil。 |

---

## 6. 存储管理

**源文件**: `internal/oci/storage.go`, `internal/oci/objectstorage.go`, `internal/oci/block_volume.go`

### 6.1 对象存储 - Bucket

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `GetNamespace(ctx, client)` | 无 | `string` | 获取租户的对象存储命名空间。 |
| `ListBuckets(ctx, c, compartmentID)` | Compartment OCID | `[]objectstorage.BucketSummary` | 列出 Bucket（旧版，无分页）。**推荐使用 ListBucketsPaginated**。 |
| `ListBucketsPaginated(ctx, client, namespace, compartmentID, limit, pageToken)` | 命名空间 + Compartment + 分页参数 | `([]BucketSummary, *string, error)` | 分页列出 Bucket。返回 items 和下一页 token。包含 Tags 信息。 |
| `CreateBucket(ctx, client, namespace, compartmentID, bucketName, publicAccessType)` | 命名空间 + 名称 + 访问类型 | `error` | 创建 Bucket。访问类型：`NoPublicAccess`、`ObjectRead`、`ObjectReadWithoutList`。存储 accessType 到 FreeformTags。 |
| `DeleteBucket(ctx, client, namespace, bucketName)` | 命名空间 + 名称 | `error` | 删除 Bucket（必须为空）。 |

### 6.2 对象存储 - Object

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ListObjectsPaginated(ctx, client, namespace, bucketName, prefix, limit, startToken)` | Bucket + 前缀 + 分页 | `([]ObjectSummary, *string, error)` | 分页列出对象。支持前缀过滤。 |
| `PutObject(ctx, client, namespace, bucketName, objectName, body, size, contentType)` | 对象名 + 内容 + 大小 | `error` | 上传单个对象。默认 Content-Type: `application/octet-stream`。 |
| `GetObject(ctx, client, namespace, bucketName, objectName)` | 对象名 | `(io.ReadCloser, string, *int64, error)` | 下载对象。返回：内容 Reader、Content-Type、Content-Length。 |
| `DeleteObject(ctx, client, namespace, bucketName, objectName)` | 对象名 | `error` | 删除对象。 |

### 6.3 预签名 URL (PAR)

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `CreatePresignedURL(ctx, client, namespace, bucketName, objectName, validitySeconds)` | 对象名 + 有效期（秒） | `string` | 创建 Pre-Authenticated Request (PAR) 预签名 URL。默认有效期 3600 秒。URL 格式：`{endpoint}{accessUri}`。**用于前端直接下载/预览文件**。 |

### 6.4 分片上传

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `CreateMultipartUpload(ctx, client, namespace, bucketName, objectName, contentType)` | 对象名 + Content-Type | `string` | 初始化分片上传，返回 Upload ID。 |
| `UploadPart(ctx, client, namespace, bucketName, objectName, uploadID, partNumber, body, size)` | Upload ID + 分片号 + 内容 | `string` | 上传单个分片，返回 ETag。分片号从 1 开始。 |
| `CommitMultipartUpload(ctx, client, namespace, bucketName, objectName, uploadID, parts)` | Upload ID + 分片列表 | `error` | 提交分片上传。parts 包含每个分片的 ETag 和 PartNumber。 |
| `AbortMultipartUpload(ctx, client, namespace, bucketName, objectName, uploadID)` | Upload ID | `error` | 取消分片上传。 |

### 6.5 块存储

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `GetBootVolume(ctx, client, bootVolumeID)` | 启动卷 OCID | `*core.BootVolume` | 获取启动卷详情。 |
| `GetBootVolumeClients(ctx, c, bootVolumeID)` | 启动卷 OCID | `*core.BootVolume` | 通过 Clients 获取启动卷（使用 BlockstorageClient）。 |
| `GetVolume(ctx, c, volumeID)` | 块卷 OCID | `*core.Volume` | 获取块卷详情。 |
| `UpdateBootVolumeVpu(ctx, c, bootVolumeID, vpusPerGB)` | 启动卷 OCID + VPU 值 | `*core.BootVolume` | 更新启动卷性能级别。VPU 值：10=Balanced、20=Higher Performance、30-120=Ultra High。**实例需先停止**。 |
| `UpdateVolumeVpu(ctx, c, volumeID, vpusPerGB)` | 块卷 OCID + VPU 值 | `*core.Volume` | 更新块卷性能级别。VPU 值：0=Lower Cost（仅稀疏卷）、10=Balanced、20=Higher Performance、30-120=Ultra High。 |

---

## 7. 备份管理

**源文件**: `internal/oci/backup.go`

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `CreateBootVolumeBackup(ctx, client, bootVolumeID, displayName)` | 启动卷 OCID + 显示名 | `string` | 创建启动卷备份。返回备份 OCID。 |
| `ListBootVolumeBackups(ctx, client, compartmentID, bootVolumeID)` | Compartment + 启动卷 OCID | `[]core.BootVolumeBackup` | 列出启动卷的所有备份。自动分页。 |
| `CopyBootVolumeBackup(ctx, client, backupID, targetRegion, displayName)` | 备份 OCID + 目标区域 | `string` | 跨区域复制备份。返回目标区域的备份 OCID。用于灾备和迁移。 |
| `DeleteBootVolumeBackup(ctx, client, backupID)` | 备份 OCID | `error` | 删除备份。 |

---

## 8. 监控与审计

**源文件**: `internal/oci/traffic.go`, `internal/oci/audit.go`, `internal/oci/usage.go`

### 8.1 实例流量监控

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `BuildMonitoringClient(prov)` | Provider | `*monitoring.MonitoringClient` | 构建监控客户端。 |
| `GetInstanceTrafficTotal(ctx, client, compartmentID, vnics, ingress, startTime, endTime, period)` | VNIC 列表 + 方向 + 时间范围 | `float64` | 查询 VNIC 集合的总入站/出站流量（字节）。使用 `oci_vcn` 命名空间的 `VnicFromNetworkBytes`/`VnicToNetworkBytes` 指标。每个 VNIC 失败不影响总计。 |
| `BytesToGB(bytes)` | 字节数 | `float64` | 字节转 GB（÷1024³）。 |

**TrafficPeriod 枚举**:

| 值 | 说明 | MQL 间隔 |
|----|------|----------|
| `1h` | 最近 1 小时 | 5 分钟 |
| `1d` | 最近 1 天 | 1 小时 |
| `1M` | 最近 1 月 | 1 天 |

### 8.2 审计日志

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ListAuditEvents(ctx, client, compartmentID, startTime, endTime, limit)` | Compartment + 时间范围 + 限制 | `[]audit.AuditEvent` | 查询审计事件。时间范围内的所有操作记录。 |
| `ListRecentAuditEvents(ctx, client, compartmentID, days)` | Compartment + 天数 | `[]audit.AuditEvent` | 查询最近 N 天的审计事件。 |
| `ListAuditEventsByDateRange(ctx, client, compartmentID, startDate, endDate)` | Compartment + 日期字符串 | `[]audit.AuditEvent` | 按日期范围查询审计事件。日期格式：`2006-01-02`。 |

### 8.3 费用查询

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `QueryCost(ctx, c, tenancyOCID, startUTC, endUTC, groupBy, granularity)` | 时间范围 + 分组 + 粒度 | `[]CostSummary` | 通用费用查询。groupBy 支持：`service`、`skuName`、`region` 等。granularity：`DAILY`、`MONTHLY`。 |
| `QueryTodayCost(ctx, c, tenancyOCID)` | 无 | `[]CostSummary` | 查询今日费用（DAILY 粒度）。 |
| `QueryYesterdayCost(ctx, c, tenancyOCID)` | 无 | `[]CostSummary` | 查询昨日费用。 |
| `QueryCurrentMonthCost(ctx, c, tenancyOCID)` | 无 | `[]CostSummary` | 查询当月费用（MONTHLY 粒度）。 |
| `QueryLastMonthCost(ctx, c, tenancyOCID)` | 无 | `[]CostSummary` | 查询上月费用。 |
| `QueryCustomCost(ctx, c, tenancyOCID, startStr, endStr)` | 日期字符串 | `[]CostSummary` | 自定义日期范围费用查询。日期格式：`2006-01-02`。DAILY 粒度。 |

**CostSummary 结构**:

```go
type CostSummary struct {
    TimeUsageStarted *common.SDKTime `json:"timeUsageStarted"`
    TimeUsageEnded   *common.SDKTime `json:"timeUsageEnded"`
    Service          string          `json:"service"`          // 服务名称
    ResourceName     string          `json:"resourceName"`     // 资源名称
    ComputedAmount   float32         `json:"computedAmount"`   // 费用金额
    ComputedQuantity float32         `json:"computedQuantity"` // 使用量
    Currency         string          `json:"currency"`         // 货币代码
    SkuName          string          `json:"skuName"`          // SKU 名称
    Region           string          `json:"region"`           // 区域
}
```

---

## 9. 配额与区域

**源文件**: `internal/oci/limits.go`, `internal/oci/region_sub.go`, `internal/oci/osp_gateway.go`

### 9.1 服务配额 (Limits)

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `GetServiceQuotasPaged(ctx, client, tenancyOCID, compartmentID, serviceName, page, limit)` | 服务名 + 分页 | `([]LimitValueSummary, string, int64, error)` | 分页获取服务配额。返回配额列表、下一页 token、总数。 |
| `ServiceHasLimits(ctx, client, tenancyOCID, compartmentID, serviceName)` | 服务名 | `bool` | 检查服务是否有限额配置。 |
| `ListLimitServices(ctx, client, tenancyOCID)` | 租户 OCID | `[]limits.ServiceSummary` | 列出所有有限额的服务。 |
| `HasEnoughResource(ctx, client, tenancyOCID, compartmentID, serviceName, limitName, required)` | 服务名 + 限额名 + 需要量 | `(bool, error)` | 检查资源是否足够。比较可用量与需要量。 |

### 9.2 区域订阅

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ListSubscribedRegions(ctx, c, tenancyOCID)` | 租户 OCID | `[]RegionSubInfo` | 列出已订阅区域。返回 RegionKey、RegionName、Status、IsHomeRegion。 |
| `ListAllRegions(ctx, c)` | 无 | `[]RegionInfo` | 列出所有可用区域。返回 Key、Name、中文名。 |
| `ListUnsubscribedRegions(ctx, c, tenancyOCID)` | 租户 OCID | `[]RegionInfo` | 列出未订阅区域（All - Subscribed）。 |
| `GetRegionSummary(ctx, c, tenancyOCID)` | 租户 OCID | `*RegionSummary` | 获取区域统计：总数、已订阅数、未订阅数。 |
| `SubscribeToRegion(ctx, c, tenancyOCID, regionKey)` | 区域 Key | `(bool, string, error)` | 订阅区域。先检查是否已订阅，再验证区域存在，最后创建订阅。不等待激活。 |
| `GetRegionSubscriptionStatus(ctx, c, tenancyOCID, regionKey)` | 区域 Key | `string` | 获取订阅状态：`READY`、`NOT_SUBSCRIBED`、`READY` 等。 |
| `WaitRegionActivation(ctx, c, tenancyOCID, regionKey, maxWaitMinutes)` | 区域 Key + 最大等待分钟 | `(bool, string, error)` | 等待区域激活。每 30 秒轮询，直到 READY/FAILED 或超时。默认最大 30 分钟。 |

### 9.3 OSP Gateway 订阅详情

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `GetHomeRegionName(ctx, c, tenancyOCID)` | 租户 OCID | `string` | 从已订阅区域中发现 Home Region 名称。 |
| `GetSubscriptionInfo(ctx, c, tenancyOCID)` | 租户 OCID | `*SubscriptionInfo` | 获取 OSP Gateway 订阅详情。需先发现 Home Region。 |

**SubscriptionInfo 结构**:

```go
type SubscriptionInfo struct {
    Id                     string          `json:"id"`
    PlanType               string          `json:"planType"`               // 计划类型
    AccountType            string          `json:"accountType"`            // 账户类型
    TimeStart              *common.SDKTime `json:"timeStart"`              // 订阅开始时间
    CurrencyCode           string          `json:"currencyCode"`           // 货币代码
    IsIntentToPay          bool            `json:"isIntentToPay"`          // 是否付费意向
    SubscriptionPlanNumber string          `json:"subscriptionPlanNumber"` // 订阅计划编号
    UpgradeState           string          `json:"upgradeState"`           // 升级状态
    EmailAddress           string          `json:"emailAddress"`           // 联系邮箱
    CompanyName            string          `json:"companyName"`            // 公司名称
    Country                string          `json:"country"`                // 国家
    LanguageCode           string          `json:"languageCode"`           // 语言代码
}
```

---

## 10. 控制台连接

**源文件**: `internal/oci/console.go`, `internal/oci/tunnel.go`

### 10.1 控制台连接管理

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `CreateConsoleConnection(ctx, c, instanceID, publicKey)` | 实例 OCID + SSH 公钥 | `*ConsoleConnection` | 创建 VNC 控制台连接。需要提供 SSH 公钥。 |
| `GetConsoleConnection(ctx, c, connID)` | 连接 ID | `*ConsoleConnection` | 获取控制台连接详情。 |
| `ListConsoleConnections(ctx, c, compartmentID, instanceID)` | Compartment + 实例 OCID | `[]ConsoleConnection` | 列出实例的所有控制台连接。 |
| `DeleteConsoleConnection(ctx, c, connID)` | 连接 ID | `error` | 删除控制台连接。 |
| `GenerateConsoleConnection(ctx, c, instanceID, publicKey)` | 实例 OCID + SSH 公钥 | `*ConsoleConnectionInfo` | 创建控制台连接并返回完整信息（含连接字符串）。 |
| `EnsureConsoleConnection(ctx, c, compartmentID, instanceID, publicKey, timeout)` | 实例 OCID + SSH 公钥 + 超时 | `*ConsoleConnectionInfo` | **推荐使用**。确保获得 ACTIVE 状态的控制台连接。自动处理：清理残留连接 → 等待删除完成 → 创建新连接 → 等待 ACTIVE。409 错误自动重试。 |
| `FindActiveConsoleConnection(ctx, c, compartmentID, instanceID)` | 实例 OCID | `*ConsoleConnectionInfo` | 查找实例的第一个 ACTIVE 控制台连接。无则返回 nil。 |
| `CleanupConsoleConnections(ctx, c, compartmentID, instanceID)` | 实例 OCID | `error` | 清理实例的所有非终端控制台连接。用于会话结束时。 |
| `WaitForConnectionsCleared(ctx, c, compartmentID, instanceID, timeout)` | 实例 OCID + 超时 | `error` | 等待所有控制台连接进入终端状态。 |

**ConsoleConnectionInfo 结构**:

```go
type ConsoleConnectionInfo struct {
    ID                  string `json:"id"`
    InstanceID          string `json:"instanceId"`
    ConnectionString    string `json:"connectionString"`    // SSH 连接命令
    VncConnectionString string `json:"vncConnectionString"` // VNC 连接字符串
    LifecycleState      string `json:"lifecycleState"`      // ACTIVE/CREATING/FAILED/DELETED
    PrivateKeyPEM       string `json:"privateKeyPem"`       // 私钥（如生成）
    PublicKeySSH        string `json:"publicKeySsh"`        // SSH 公钥
}
```

### 10.2 SSH 隧道构建

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ParseConnectionString(connectionString)` | OCI 连接字符串 | `*ParsedConnectionString` | 解析 OCI VncConnectionString。提取 ConnectionID、ProxyHost、TargetHost（实例 OCID）。 |
| `BuildSSHTunnelCommand(cfg)` | SSHTunnelConfig | `[]string` | 构建 VNC 隧道 SSH 命令参数。通过 OCI 控制台代理（端口 443）建立 -L 转发到实例的 5900 端口。 |
| `BuildSerialConsoleCommand(cfg)` | SSHTunnelConfig | `[]string` | 构建串口控制台 SSH 命令参数。交互式终端（-tt 强制 PTY），通过 -W 转发。 |

**SSHTunnelConfig 结构**:

```go
type SSHTunnelConfig struct {
    PrivateKeyPath string // SSH 私钥路径
    ConnectionID   string // 控制台连接 OCID
    ProxyHost      string // 代理主机: <connID>@instance-console.<region>
    TargetHost     string // 目标主机: 实例 OCID
    LocalPort      int    // 本地端口 (VNC: 5900)
    RemotePort     int    // 远程端口 (VNC: 5900)
}
```

---

## 11. 代理池

**源文件**: `internal/oci/proxy.go`

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `NewProxyPool(q)` | Queries 实例 | `*ProxyPool` | 创建代理池。从数据库读取代理记录。 |
| `ProxyPool.Pick(ctx)` | 无 | `(*repo.VpnProxyRecord, error)` | 随机选择一个可用代理。无代理返回 nil（直接连接）。 |
| `ProxyPool.Available(rec)` | 代理记录 | `bool` | 健康检查。通过代理发起 HTTPS GET 到 oracle.com，状态码 < 500 视为可用。超时 3 秒。 |
| `WithProxy(ctx, pool, creds, masterKey, fn)` | 代理池 + 凭据 + 回调 | `error` | **推荐使用**。代理装饰器。自动选择健康代理并构建代理客户端执行回调；无代理或代理不可用时回退到直接连接。支持 SOCKS5/HTTP/HTTPS 代理类型。 |

---

## 附录

### A. 数据结构汇总

```go
// VNIC 附件信息
type VnicAttachmentInfo struct {
    VnicID        string   `json:"vnicId"`
    InstanceID    string   `json:"instanceId"`
    InstanceName  string   `json:"instanceName"`
    PublicIP      string   `json:"publicIp"`
    PrivateIP     string   `json:"privateIp"`
    Ipv6Addresses []string `json:"ipv6Addresses"`
    SubnetID      string   `json:"subnetId"`
    VlanTag       *int     `json:"vlanTag"`
}

// 批量 VNIC 创建结果
type BatchVnicCreationResult struct {
    InstanceID                string               `json:"instanceId"`
    InstanceDisplayName       string               `json:"instanceDisplayName"`
    RequestedVnicCount        int                  `json:"requestedVnicCount"`
    RequestedIpv6CountPerVnic int                  `json:"requestedIpv6CountPerVnic"`
    SuccessfulVnicCount       int                  `json:"successfulVnicCount"`
    TotalIpv6Count            int                  `json:"totalIpv6Count"`
    VnicResults               []VnicCreationResult `json:"vnicResults"`
    AllSuccessful             bool                 `json:"allSuccessful"`
    Summary                   string               `json:"summary"`
    TotalExecutionTimeMs      int64                `json:"totalExecutionTimeMs"`
}

// IAM 用户
type OciUser struct {
    OCID                    string    `json:"ocid"`
    Name                    string    `json:"name"`
    Email                   string    `json:"email"`
    Domain                  string    `json:"domain"`
    LifecycleState          string    `json:"lifecycleState"`
    TimeCreated             time.Time `json:"timeCreated"`
    LastSuccessfulLoginTime time.Time `json:"lastSuccessfulLoginTime"`
}

// MFA 状态
type MfaStatus struct {
    TotpEnabled              bool `json:"totpEnabled"`
    EmailEnabled             bool `json:"emailEnabled"`
    SmsEnabled               bool `json:"smsEnabled"`
    PushEnabled              bool `json:"pushEnabled"`
    SecurityQuestionsEnabled bool `json:"securityQuestionsEnabled"`
    FidoAuthenticatorEnabled bool `json:"fidoAuthenticatorEnabled"`
    PhoneCallEnabled         bool `json:"phoneCallEnabled"`
}

// 密码策略
type PasswordPolicyDetail struct {
    Name                    string `json:"name"`
    IsPasswordExpiryEnabled bool   `json:"isPasswordExpiryEnabled"`
    PasswordExpiryDays      int    `json:"passwordExpiryDays"`
}
```

### B. 错误处理约定

- 所有函数返回 `error` 作为最后一个返回值
- OCI SDK 错误通过 `fmt.Errorf("context: %w", err)` 包装
- HTTP 状态码可通过 `err.(interface{ GetHTTPStatusCode() int })` 获取
- 404 NotFound 通过 `isNotFound(err)` 辅助函数检测
- 409 Conflict 通过 `isConsoleConflict(err)` 辅助函数检测

### C. 分页约定

- 所有列表函数自动处理分页（内部循环 `OpcNextPage`）
- 部分函数支持外部传入分页参数（如 `ListBucketsPaginated`）
- 默认每页大小：100（实例）、200（用户/Compartment）

### D. 超时约定

| 操作 | 默认超时 | 轮询间隔 |
|------|----------|----------|
| 实例状态变更 | 300 秒 | 3 秒 |
| VNIC 附件/卸载 | 300 秒 | 3 秒 |
| NLB 创建 | 300 秒 | 5 秒 |
| 控制台连接 | 可配置 | 3 秒 |
| 区域激活 | 30 分钟 | 30 秒 |
| 代理健康检查 | 3 秒 | - |
