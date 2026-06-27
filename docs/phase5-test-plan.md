# Phase 5 测试方案 — 网络 / 流量 / 备份 / 实例管理

---

## 1. 编译验证
| # | 测试项 | 命令 | 预期 |
|---|---|---|---|
| 1.1 | Go 编译 | `go build ./...` | 无错误 |
| 1.2 | 二进制构建 | `go build -tags dist -o oci-start ./cmd/oci-start/` | 生成可执行文件 |
| 1.3 | 前端构建 | `npm run build` | 生成 dist/ |
| 1.4 | sqlc 幂等 | `sqlc generate` | 无变更 |

## 2. 调度器 Jobs
| # | Job | 验证方法 | 预期 |
|---|---|---|---|
| 2.1 | InstanceTrafficJob | 查看日志 | "traffic: check complete" |
| 2.2 | CheckLiveJob | 查看日志 (08:00-22:00) | "checklive" 相关日志 |
| 2.3 | PingConnTimeJob | 查看日志 | "ping: check complete" |
| 2.4 | CheckOfflineInstanceJob | 查看日志 | 离线检测日志 |

## 3. API 端点测试
| # | 方法 | 路径 | 预期 |
|---|---|---|---|
| 3.1 | GET | `/instances/list` | 200, items+total |
| 3.2 | GET | `/instances/:id` | 200, 实例详情 |
| 3.3 | POST | `/instances/:id/remark` | 200, 备注更新 |
| 3.4 | GET | `/instances/traffic?tenantId=1` | 200, 流量统计 |
| 3.5 | GET | `/backup/list?tenantId=1` | 200, 备份列表 |
| 3.6 | GET | `/backup/delete?id=1` | 200, 删除 |
| 3.7 | GET | `/traffic/alert/list` | 200, 告警列表 |
| 3.8 | POST | `/traffic/alert/save` | 200, 保存告警 |

## 4. 前端页面
| # | 页面 | 测试项 | 预期 |
|---|---|---|---|
| 4.1 | `/instances` | 页面加载 | 实例表格显示 |
| 4.2 | `/instances` | 详情对话框 | 显示完整实例信息+备注编辑 |
| 4.3 | `/instances` | 分页 | 翻页正常 |

## 5. OCI API 集成 (需要真实凭证)
| # | 场景 | 预期 |
|---|---|---|
| 5.1 | 流量查询 | MonitoringClient.SummarizeMetricsData 返回数据或权限错误 |
| 5.2 | 账号存活检查 | IdentityClient.ListCompartments 成功或 NotAuthenticated |
| 5.3 | 备份创建 | BlockstorageClient.CreateBootVolumeBackup 返回 backup OCID |

## 6. 已知限制
| 功能 | 状态 |
|---|---|
| SSH enableRoot (备份前置) | Phase 6 |
| Telegram 通知 | Phase 7 |
| VNIC 完整枚举 | Phase 5 stub |
| 自动关机 (流量超限) | Phase 5 stub |
