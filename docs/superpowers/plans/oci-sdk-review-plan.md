# OCI SDK 实现审查计划

**创建日期**: 2026-07-03
**目标**: 全面审查 `internal/oci/` 包的 OCI Go SDK 集成，确保代码质量、安全性、错误处理和测试覆盖。

---

## 审查范围

总计 **~136 个函数**，分布在 20+ 个文件中。

---

## Phase 1: 核心架构审查

### 1.1 认证与凭据 (`provider.go`, `credentials.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| 密钥管理 | AES-256-GCM 解密是否安全？master key 是否正确保护？ | 🔴 Critical |
| 凭据存储 | KeyFileBlob 是否避免明文存储？密钥是否泄露到日志？ | 🔴 Critical |
| Provider 构建 | 错误处理是否完整？空值检查是否充分？ | 🟡 High |
| 客户端初始化 | NewClients 是否正确处理部分失败？ | 🟡 High |
| API Key CRUD | SCIM 认证是否正确？错误信息是否有用？ | 🟡 High |
| Auth Token | 一次性 Token 是否安全返回？是否记录审计？ | 🟡 High |
| SMTP 凭据 | 一次性密码是否正确处理？ | 🟢 Medium |
| Customer Secret Key | 一次性密钥是否安全返回？ | 🟢 Medium |

### 1.2 代理池 (`proxy.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| 代理选择 | 随机选择是否有偏？是否支持权重？ | 🟢 Medium |
| 健康检查 | 超时是否合理？是否避免误判？ | 🟡 High |
| SOCKS5 认证 | 用户名密码是否安全传递？ | 🟡 High |
| HTTP/HTTPS 代理 | URL 构建是否正确？ | 🟢 Medium |
| WithProxy 模式 | 降级逻辑是否正确？资源是否释放？ | 🟡 High |

---

## Phase 2: 实例管理审查

### 2.1 实例生命周期 (`compute.go`, `instance_control.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| ListInstances | 分页是否完整？是否处理空结果？ | 🟡 High |
| GetInstance | 错误信息是否包含 instanceID？ | 🟢 Medium |
| TerminateInstance | preserveBootVolume 参数是否正确传递？ | 🟡 High |
| Start/StopInstance | 是否检查当前状态？重复操作是否幂等？ | 🟡 High |
| ResetInstance | 等待超时是否合理？状态轮询间隔？ | 🟡 High |
| UpdateInstanceShape | 是否验证 shape 兼容性？OCPU/内存范围检查？ | 🟡 High |
| waitForInstanceState | 超时和间隔是否可配置？上下文取消是否支持？ | 🟢 Medium |

### 2.2 实例同步 (`instance_sync.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| ListInstancesByTenant | 跨 compartment 遍历是否完整？ | 🟡 High |
| buildInstanceDetailRow | 字段映射是否完整？空值处理？ | 🟡 High |
| bootVolumeInfo | 多启动卷场景是否正确处理？ | 🟢 Medium |
| 错误容忍 | 单 compartment 失败是否影响其他？ | 🟡 High |

---

## Phase 3: VNIC 管理审查

### 3.1 VNIC 基础操作 (`vnic.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| ListVnicAttachments | 分页是否完整？过滤条件是否正确？ | 🟢 Medium |
| GetVnicInfo | IPv6 地址是否正确解析？ | 🟢 Medium |
| ListAllVnicsForInstance | 单 VNIC 错误是否影响其他？ | 🟡 High |
| IsPrimaryVnic | 判断逻辑是否与 OCI 文档一致？ | 🟡 High |

### 3.2 VNIC 批量操作 (`vnic.go` Phase 11.2)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| CreateMultipleVnicsWithIpv6 | 部分失败处理是否正确？ | 🔴 Critical |
| CreateSingleVnicWithIpv6 | 等待逻辑是否可靠？ | 🟡 High |
| DeleteVnicWithIpv6 | 主 VNIC 保护是否生效？ | 🔴 Critical |
| DeleteAllSecondaryVnics | 是否正确识别主 VNIC？ | 🟡 High |
| IPv6 创建/删除 | 失败是否容忍？是否影响主流程？ | 🟡 High |
| WaitForVnicAttachment | NotFound 处理是否正确？ | 🟢 Medium |

---

## Phase 4: 网络管理审查

### 4.1 VCN/VNIC 网络 (`network.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| GetPrimaryVnic | 回退逻辑是否正确？O(N) 优化是否生效？ | 🟡 High |
| ReassignPublicIP | 旧 IP 删除+新 IP 创建的原子性？ | 🟡 High |
| CreateOrGetNatGateway | 并发创建是否幂等？ | 🟢 Medium |
| UpdateInstanceVnicRouteTable | 路由表 ID 验证？ | 🟢 Medium |
| ResetVnicToDefaultRouteTable | "amd" 过滤逻辑是否正确？ | 🟡 High |

### 4.2 安全列表 (`security_list.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| ListSecurityRules | 跨 Security List 合并是否正确？ | 🟡 High |
| AddSecurityRule | 去重逻辑是否正确？ | 🟡 High |
| DeleteSecurityRule | 全局索引计算是否正确？ | 🔴 Critical |
| EnableAllForTenant | 规则完整性？IPv6 失败容忍？ | 🟡 High |
| ConfigureIPv6SecurityRules | ICMPv6 类型覆盖是否完整？ | 🟢 Medium |

### 4.3 NLB 负载均衡 (`nlb.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| CreateOrGetNetworkLoadBalancer | 后端集/监听器配置是否正确？ | 🟡 High |
| WaitForNLBCreation | FAILED 状态是否正确处理？ | 🟡 High |
| findNLBByDisplayName | 分页是否完整？ | 🟢 Medium |

---

## Phase 5: IAM 管理审查

### 5.1 用户/组管理 (`identity.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| ListUsers | 分页是否完整？域名解析是否正确？ | 🟢 Medium |
| CreateUser | 组查找是否正确？一次性密码是否安全返回？ | 🟡 High |
| DeleteUser | 是否检查用户存在？关联资源清理？ | 🟡 High |
| ResetUserPassword | 一次性密码是否正确返回？ | 🟢 Medium |
| ResetMfaForAllUsers | 大量用户时性能？错误容忍？ | 🟡 High |

### 5.2 Identity Domain SCIM (`identity.go`, `signon_policy.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| getDomainURL | 多域名场景是否正确？ | 🟢 Medium |
| getIdDomainsClient | 认证是否正确（IDCS token）？ | 🔴 Critical |
| GetPasswordPolicy | Custom 策略过滤是否正确？ | 🟢 Medium |
| UpdatePasswordPolicy | PUT 全对象模式是否正确？ | 🟡 High |
| GetMfaStatus | 字段映射是否完整？ | 🟢 Medium |
| ToggleEmailMFA | GET→modify→PUT 模式是否正确？ | 🟡 High |
| NotificationRecipients | 邮箱验证状态是否正确？ | 🟢 Medium |
| AccountRecoverySetting | 因子枚举是否正确？ | 🟢 Medium |

---

## Phase 6: 存储管理审查

### 6.1 对象存储 (`objectstorage.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| ListBucketsPaginated | 分页 token 是否正确传递？ | 🟢 Medium |
| CreateBucket | 访问类型映射是否正确？ | 🟢 Medium |
| PutObject | Content-Length/Type 是否正确？ | 🟢 Medium |
| GetObject | 大文件流式读取？ | 🟡 High |
| CreatePresignedURL | 过期时间计算？URL 拼接？ | 🟡 High |
| MultipartUpload | 分片大小限制？ETag 收集？ | 🟡 High |

### 6.2 块存储 (`block_volume.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| UpdateBootVolumeVpu | VPU 值范围验证？ | 🟢 Medium |
| UpdateVolumeVpu | VPU 值范围验证？ | 🟢 Medium |

### 6.3 备份 (`backup.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| CreateBootVolumeBackup | 显示名称是否必填？ | 🟢 Medium |
| CopyBootVolumeBackup | 跨区域复制错误处理？ | 🟡 High |

---

## Phase 7: 监控与审计审查

### 7.1 流量监控 (`traffic.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| GetInstanceTrafficTotal | MQL 查询语法是否正确？ | 🟡 High |
| queryVnicTraffic | 聚合逻辑是否正确？ | 🟡 High |
| BytesToGB | 精度是否足够？ | 🟢 Medium |

### 7.2 审计日志 (`audit.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| ListAuditEvents | 时间范围限制（90天）是否正确？ | 🟡 High |
| ListRecentAuditEvents | UTC 时间计算是否正确？ | 🟡 High |
| extractAuditEvent | 空值处理是否完整？ | 🟢 Medium |
| truncateUserName | 截断长度是否合理？ | 🟢 Medium |

### 7.3 费用查询 (`usage.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| QueryCost | 时间范围是否正确？ | 🟡 High |
| QueryTodayCost | UTC 日期边界是否正确？ | 🟡 High |
| QueryCurrentMonthCost | 月份边界计算是否正确？ | 🟢 Medium |

---

## Phase 8: 配额与区域审查

### 8.1 配额管理 (`limits.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| GetServiceQuotasPaged | 两遍遍历逻辑是否正确？ | 🟡 High |
| collectLimitNames | AD 级别聚合是否正确？ | 🟡 High |
| getAggregatedAvailability | 跨 AD 求和是否正确？ | 🟡 High |
| ServiceHasLimits | 提前终止逻辑是否正确？ | 🟢 Medium |
| HasEnoughResource | 可用资源计算是否正确？ | 🟡 High |

### 8.2 区域订阅 (`region_sub.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| ListSubscribedRegions | 分页是否完整？ | 🟢 Medium |
| SubscribeToRegion | 去重检查是否正确？ | 🟢 Medium |
| WaitRegionActivation | 轮询间隔是否合理？ | 🟢 Medium |

### 8.3 OSP Gateway (`osp_gateway.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| GetHomeRegionName | Home Region 识别是否正确？ | 🟡 High |
| GetSubscriptionInfo | 字段映射是否完整？ | 🟢 Medium |

---

## Phase 9: 控制台连接审查

### 9.1 控制台管理 (`console.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| EnsureConsoleConnection | 409 重试逻辑是否正确？ | 🔴 Critical |
| clearAll | 终止状态判断是否正确？ | 🟡 High |
| waitForCleared | 超时处理是否正确？ | 🟡 High |
| waitForActive | 状态机是否完整？ | 🟡 High |

### 9.2 SSH 隧道 (`tunnel.go`)

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| ParseConnectionString | 正则表达式是否正确？ | 🟡 High |
| BuildSSHTunnelCommand | SSH 参数是否安全？ | 🟡 High |
| BuildSerialConsoleCommand | PTY 强制是否正确？ | 🟢 Medium |

---

## Phase 10: 测试覆盖审查

| 文件 | 现有测试 | 需要补充 | 优先级 |
|------|----------|----------|--------|
| `provider_test.go` | ✅ | 增加边界用例 | 🟡 High |
| `identity_test.go` | ✅ | 增加 SCIM 测试 | 🟡 High |
| `network_test.go` | ✅ | 增加安全列表测试 | 🟡 High |
| `shape_image_test.go` | ✅ | - | 🟢 Medium |
| `console_ensure_test.go` | ✅ | 增加 409 重试测试 | 🟡 High |
| `compute.go` | ❌ | 需要添加 | 🔴 Critical |
| `vnic.go` | ❌ | 需要添加 | 🔴 Critical |
| `credentials.go` | ❌ | 需要添加 | 🟡 High |
| `security_list.go` | ❌ | 需要添加 | 🟡 High |
| `backup.go` | ❌ | 需要添加 | 🟢 Medium |
| `traffic.go` | ❌ | 需要添加 | 🟢 Medium |
| `audit.go` | ❌ | 需要添加 | 🟢 Medium |
| `usage.go` | ❌ | 需要添加 | 🟢 Medium |
| `limits.go` | ❌ | 需要添加 | 🟡 High |
| `region_sub.go` | ❌ | 需要添加 | 🟢 Medium |

---

## Phase 11: 安全审查

| 审查项 | 检查点 | 优先级 |
|--------|--------|--------|
| 密钥泄露 | 日志/错误信息是否包含密文？ | 🔴 Critical |
| 凭据传输 | HTTPS 是否强制？证书验证？ | 🔴 Critical |
| 权限检查 | 最小权限原则是否遵循？ | 🔴 Critical |
| 输入验证 | OCID 格式验证？参数范围检查？ | 🟡 High |
| 重放攻击 | 一次性凭据是否正确失效？ | 🟡 High |
| 并发安全 | 共享状态是否有竞态条件？ | 🟡 High |

---

## 审查执行顺序

```
Phase 1 (认证核心) → Phase 2 (实例管理) → Phase 3 (VNIC)
    ↓
Phase 4 (网络) → Phase 5 (IAM) → Phase 6 (存储)
    ↓
Phase 7 (监控审计) → Phase 8 (配额区域) → Phase 9 (控制台)
    ↓
Phase 10 (测试覆盖) → Phase 11 (安全审查)
```

---

## 审查清单模板

每个函数审查时检查：

- [ ] **错误处理**: 所有错误是否被捕获并正确包装？
- [ ] **空值检查**: 指针解引用前是否检查 nil？
- [ ] **分页逻辑**: 是否完整遍历所有页面？
- [ ] **上下文传播**: ctx 是否正确传递并支持取消？
- [ ] **资源释放**: 是否有需要关闭的资源？
- [ ] **日志记录**: 关键操作是否有日志？
- [ ] **测试覆盖**: 是否有对应的单元测试？
- [ ] **文档注释**: 函数注释是否完整？

---

## 预期产出

1. **审查报告**: 每个 Phase 的发现和建议
2. **修复计划**: 按优先级排序的修复任务
3. **测试补充**: 需要添加的测试用例清单
4. **文档更新**: 需要补充的 API 文档
