# 租户管理功能审计清单

对比 Java 原版（oci-start TenantController 1404 行 + EmailController + SocialTypeController + OciCostController + TrafficStatisticsController）与 Go 重写（oci-start-go）的后端 API + 前端实现。

**图例**：✅ Go 后端已有 + 前端已接 | 🔲 Go 后端已有 + 前端未接 | ❌ Go 后端缺失 | 🟡 部分/替代方案

---

## 1. 租户基础 CRUD

| 功能 | Java API | Go 后端 | Go 前端 |
|------|----------|---------|---------|
| 列表（全部租户） | `GET /tenants/listAll` | ✅ `GET /tenants/listAll` | ✅ TenantList |
| 列表（分页，HTML 页面） | `GET /tenants/list` | ❌ | —（SPA 无分页 HTML） |
| 列表（JSON API） | `GET /tenants/list/json` | ✅ (同 listAll) | ✅ |
| 新增（OCI Config 解析+表单） | `POST /tenants/save` | ✅ `POST /tenants/save` | ✅ TenantList 新增对话框 |
| 删除 | `GET /tenants/deleteApi` | ✅ `GET /tenants/deleteApi` | ✅ TenantList 下拉 |
| 获取单个 | `—`（从 listAll 过滤） | ✅ `GET /tenants/:id` | ✅ TenantDetail |
| 更新 | `—`（用各专用端点） | ✅ `PUT /tenants/:id` | ✅ TenantDetail 编辑 |
| 同步 OCI 实例 | `GET /tenants/syncOci` | ✅ `GET /tenants/syncOci` | ✅ TenantList/TenantDetail |
| 测试存活（check status） | `GET /tenants/checkStatus` | ✅ `GET /tenants/:id/check` | ✅ TenantDetail |
| 批量检测 | `GET /tenants/checkAccounts` | ✅ `POST /tenants/check-batch` | ✅ TenantList 批量检测 |
| 批量检测（SSE 流式） | `GET /tenants/checkAccountsStream` | ❌ | ❌ |

## 2. 自定义名称 / 成本 / 信息更新

| 功能 | Java API | Go 后端 | Go 前端 |
|------|----------|---------|---------|
| 更新自定义名称 | `POST /tenants/updateCustomName` | ✅ `PUT /tenants/:id`（合并） | ✅ TenantDetail 概况 |
| 更新账号成本 | `POST /tenants/updateAccountCost` | ✅ `PUT /tenants/:id`（合并） | ✅ TenantDetail 概况 |
| 从 OCI 获取租户信息 | `GET /tenants/updateTenant`（SSE） | ✅ `POST /tenants/:id/update-detail` | ✅ TenantDetail 设置 |
| 更新账号详情页面 | `GET /tenants/updateAccountDetail`（HTML） | —（SPA 无 HTML 页面） | ✅ TenantDetail 设置 |

## 3. 导出 / 导入

| 功能 | Java API | Go 后端 | Go 前端 |
|------|----------|---------|---------|
| 导出租户 | `GET /tenants/export` | ✅ `GET /tenants/:id/export` | ✅ TenantList/TenantDetail |
| 按租户导出 | `GET /tenants/exportByTenant` | ✅（同上，:id 路径） | ✅ |
| 发送导出验证码 | `POST /tenants/verify/sendExportCode` | ❌ | ❌ |
| 导入租户 | `POST /tenants/import` | 🟡 `POST /migration/import`（迁移模块，不同机制） | 🟡 迁移页面 |

## 4. 区域管理

| 功能 | Java API | Go 后端 | Go 前端 |
|------|----------|---------|---------|
| 区域列表 | `GET /tenants/regionList` | ❌（前端硬编码） | 🟡 TenantList/TenantAdd 区域选择器（硬编码列表） |
| 区域订阅列表 | `GET /tenants/regionSubList` | ❌ | ❌ |
| 已订阅区域数据 | `GET /tenants/subscribed-regions-data` | ✅ `GET /tenants/:id/regions/subscribed` | ✅ TenantDetail 区域 |
| 区域摘要 | `GET /tenants/region-summary` | ✅ `GET /tenants/:id/regions/summary` | ✅ TenantDetail 区域 |
| 未订阅区域 | `GET /tenants/unsubscribed-regions` | ✅ `GET /tenants/:id/regions/unsubscribed` | ✅ TenantDetail 区域 |
| 订阅区域 | `POST /tenants/subscribe-regions` | ✅ `POST /tenants/:id/regions/subscribe` | ✅ TenantDetail 区域 |
| 订阅状态检查 | `GET /tenants/check-subscription-status` | ✅ `GET /tenants/:id/regions/subscription-status` | ❌ 前端未接（按需查询） |
| 列出父租户 | `GET /tenants/listParentTenants` | ❌ | ❌ |
| 按父租户列区域 | `GET /tenants/listRegions` | ❌ | ❌ |

## 5. IAM 用户管理

| 功能 | Java API | Go 后端 | Go 前端 |
|------|----------|---------|---------|
| 列出用户 | `GET /tenants/oracle-users` | ✅ `GET /tenants/:id/users` | ✅ TenantDetail 用户 |
| 分页列出用户 | `GET /tenants/oracle-users-page` | 🟡（同上，无分页） | 🟡（无分页） |
| 创建用户 | `POST /tenants/oracle-users` | ✅ `POST /tenants/:id/users` | ✅ TenantDetail 用户 |
| 删除用户 | `POST /tenants/oracle-users/deleteUser` | ✅ `DELETE /tenants/:id/users/:ocid` | ✅ TenantDetail 用户 |
| 重置密码 | `POST /tenants/oracle-users/resetPassword` | ✅ `POST /tenants/:id/users/:ocid/reset-password` | ✅ TenantDetail 用户 |
| 获取密码策略 | `POST /tenants/oracle-users/getPasspolicy` | ✅ `GET /tenants/:id/password-policy` | ✅ TenantDetail 用户 |
| 更新密码策略 | `POST /tenants/oracle-users/password-policy` | ✅ `POST /tenants/:id/password-policy` | ✅ TenantDetail 用户 |
| 列出用户组 | `POST /tenants/groups` | ✅ `GET /tenants/:id/groups` | ✅ TenantDetail 用户 |
| 重置 MFA 因子 | `POST /tenants/resetAccountFactor` | ✅ `POST /tenants/:id/mfa/reset` | ✅ TenantDetail 设置 |

## 6. 安全规则

| 功能 | Java API | Go 后端 | Go 前端 |
|------|----------|---------|---------|
| 列出安全规则 | `GET /tenants/security-rules` | ✅ `GET /tenants/security-rules` | ✅ TenantDetail 安全规则 |
| 添加安全规则 | `POST /tenants/security-rules` | ✅ `POST /tenants/security-rules` | ✅ TenantDetail 安全规则 |
| 删除安全规则 | `DELETE /tenants/security-rules/{id}` | ✅ `DELETE /tenants/security-rules/:id` | ✅ TenantDetail 安全规则 |
| 批量启用全部规则 | `POST /tenants/enableAll` | ✅ `POST /tenants/enableAll` | ✅ TenantDetail 安全规则 |

## 7. 邮件服务配置

| 功能 | Java API | Go 后端 | Go 前端 |
|------|----------|---------|---------|
| 获取邮件配置 | `GET /tenants/:id/email`（Go 自定义） | ✅ `GET /tenants/:id/email` | ✅ TenantDetail 邮件 |
| 保存邮件配置 | `POST /tenants/:id/email` | ✅ `POST /tenants/:id/email` | ✅ TenantDetail 邮件 |
| 删除邮件配置 | `DELETE /tenants/:id/email` | ✅ `DELETE /tenants/:id/email` | ✅ TenantDetail 邮件 |
| 切换启用/禁用 | `POST /tenants/:id/email/toggle` | ✅ `POST /tenants/:id/email/toggle` | ✅ TenantDetail 邮件（switch + 保存） |
| 启用邮件服务 | `POST /tenants/email/enable` | ✅ `POST /api/email/enable` | ✅ TenantDetail 邮件 |
| 禁用邮件服务 | `POST /tenants/email/disable` | ✅ `POST /api/email/disable` | ✅ TenantDetail 邮件 |
| 邮件服务状态 | `GET /tenants/email/status` | ❌ | ❌ |
| 测试邮件服务 | `POST /tenants/email/test` | ❌ | ❌ |

## 8. 社交登录

| 功能 | Java API | Go 后端 | Go 前端 |
|------|----------|---------|---------|
| 列出社交配置 | `GET /social/list` | ✅ `GET /tenants/:id/social` | ✅ TenantDetail 社交 |
| 添加社交配置 | `POST /social/add` | ✅ `POST /tenants/:id/social` | ✅ TenantDetail 社交 |
| 更新社交配置 | `PUT /social/update` | 🟡（用 POST 覆盖） | ✅ TenantDetail 社交 |
| 删除社交配置 | `DELETE /social/...` | ✅ `DELETE /tenants/:id/social/:id` | ✅ TenantDetail 社交 |
| 启用/禁用 | `POST /social/enable` / `disable` | ✅ `PUT .../toggle` | ✅ TenantDetail 社交 |
| 可用登录类型 | `GET /social/availableLoginTypes` | ❌ | 🟡（前端硬编码 GITHUB/GOOGLE/WEIXIN） |

## 9. MFA

| 功能 | Java API | Go 后端 | Go 前端 |
|------|----------|---------|---------|
| MFA 状态 | `GET /tenants/mfa/status` | ✅ `GET /tenants/:id/mfa/status` | ✅ TenantDetail 设置 |
| 切换 MFA（邮件） | `POST /tenants/mfa/email` | ✅ `POST /tenants/:id/mfa/toggle` | ✅ TenantDetail 设置 |
| 重置 MFA | `POST /tenants/resetAccountFactor` | ✅ `POST /tenants/:id/mfa/reset` | ✅ TenantDetail 设置 |

## 10. 通知接收人

| 功能 | Java API | Go 后端 | Go 前端 |
|------|----------|---------|---------|
| 获取通知接收人 | `POST /tenants/notification/recipients` | ✅ `GET /tenants/:id/notification-recipients` | ✅ TenantDetail 设置 |
| 更新通知接收人 | `POST /tenants/notification/update` | ✅ `POST /tenants/:id/notification-recipients/update` | ✅ TenantDetail 设置 |

## 11. 流量告警

| 功能 | Java API | Go 后端 | Go 前端 |
|------|----------|---------|---------|
| 获取告警配置 | `GET /tenants/traffic-alert/{id}` | 🟡 `GET /api/traffic/alert`（不同路径） | ❌ 前端未接 |
| 保存告警配置 | `POST /tenants/traffic-alert` | ❌（直接路径） | ❌ |

## 12. 配额

| 功能 | Java API | Go 后端 | Go 前端 |
|------|----------|---------|---------|
| 获取配额 | `GET /tenants/quota` | ✅ `GET /tenants/:id/quota` | ✅ TenantDetail 设置 |

## 13. 费用 / 账单

| 功能 | Java API | Go 后端 | Go 前端 |
|------|----------|---------|---------|
| 费用查询 | `POST /cost/query`（OciCostController） | 🟡 `GET /tenants/:id/cost` | ✅ TenantDetail 费用 |
| 订阅详情 | `—` | ✅ `GET /tenants/:id/subscription` | ✅ TenantDetail 费用 |
| 订阅天数 | `—` | ✅ `GET /tenants/:id/subscription-days` | ✅ TenantDetail 费用 |
| 域内租户 | `—` | ✅ `GET /tenants/:id/domains` | ✅ TenantDetail 概况 |

## 14. Boot 卷管理

| 功能 | Java API | Go 后端 | Go 前端 |
|------|----------|---------|---------|
| 列出 Boot 卷 | `GET /tenants/boot-volumes` | ❌ | ❌ |
| 更新 Boot 卷 VPU | `PUT /tenants/update-volumes/{id}` | ❌ | ❌ |
| 删除 Boot 卷 | `DELETE /tenants/delete-volume/{id}` | ❌ | ❌ |

## 15. 审计 / 分析 / 转移

| 功能 | Java API | Go 后端 | Go 前端 |
|------|----------|---------|---------|
| 审计日志 | `POST /tenants/audit/log` | ✅ `POST /tenants/:id/audit-log` | ✅ TenantDetail 设置 |
| 审计分析（SSE 流） | `GET /tenants/analyze`（SSE） | ❌ | ❌ |
| 资产分析 | `GET /tenants/asset/analysis` | ❌ | ❌ |
| 转移租户 | `POST /tenants/transfer` | ❌ | ❌ |

## 16. 其他

| 功能 | Java API | Go 后端 | Go 前端 |
|------|----------|---------|---------|
| 查询系统镜像 | `POST /tenants/querySystemImages` | ❌ | ❌ |
| Boot 实例页面 | `GET /tenants/bootPage`（HTML） | —（SPA） | ✅ /boot 路由 |
| GCP Boot 页面 | `GET /tenants/gcpBootPage`（HTML） | —（非 OCI 功能） | — |
| 保存 Boot 实例 | `POST /tenants/boot/save` | ✅ `POST /boot/save`（不同路由） | ✅ /boot 页面 |
| 测试实例 | `POST /tenants/test-instances` | ❌ | ❌ |
| 加速页面 | `GET /tenants/addSpeed`（HTML） | —（SPA） | — |

---

## 汇总

### 已实现（✅ 后端 + 前端均到位）

密码策略、MFA 状态/切换/重置、通知接收人、配额、域内租户、订阅天数、审计日志、安全规则删除/批量启用、用户组、邮件切换、区域订阅管理（摘要+已订阅+可订阅+订阅操作）、邮件服务启用/禁用 — 均已在 TenantDetail 对应 tab 实现。

### Go 后端已有但前端未接（🔲）

| 功能 | API 路由 | 说明 |
|------|----------|------|
| 订阅状态检查（按区域查询） | `GET /tenants/:id/regions/subscription-status` | 按需查询，低优先级 |

### Go 后端完全缺失（❌ = 需要后端 + 前端）

| 功能 | Java API | 优先级 |
|------|----------|--------|
| Boot 卷管理（列表/更新 VPU/删除） | `/tenants/boot-volumes/*` | 🟡 中 |
| 区域列表（动态，非硬编码） | `GET /tenants/regionList` | 🟢 低 |
| 流量告警保存 | `POST /tenants/traffic-alert` | 🟢 低 |
| 邮件服务状态/测试 | `GET /tenants/email/status` `POST .../test` | 🟡 中 |
| 资产分析 | `GET /tenants/asset/analysis` | 🟡 中 |
| 租户转移 | `POST /tenants/transfer` | 🟢 低 |
| 可用社交登录类型 | `GET /social/availableLoginTypes` | 🟢 低 |
| 导出验证码 | `POST /tenants/verify/sendExportCode` | 🟢 低 |
| 父租户/区域列表 | `GET /tenants/listParentTenants` `/listRegions` | 🟢 低 |
| 批量检测 SSE 流式 | `GET /tenants/checkAccountsStream` | 🟢 低 |
| 审计分析（SSE 流） | `GET /tenants/analyze` | 🟢 低 |
| 查询系统镜像 | `POST /tenants/querySystemImages` | 🟢 低 |
| 测试实例 | `POST /tenants/test-instances` | 🟢 低 |
