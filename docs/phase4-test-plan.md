# Phase 4 测试方案 — 抢机引擎

> 本测试方案用于最终统一测试。测试不在此阶段执行，仅列出测试项、方法和预期结果。

---

## 1. 单元测试 (编译时验证)

| # | 测试项 | 方法 | 预期 |
|---|---|---|---|
| 1.1 | Go 编译通过 | `go build ./...` | 无编译错误 |
| 1.2 | 二进制构建 | `go build -tags dist -o oci-start ./cmd/oci-start/` | 生成可执行文件 |
| 1.3 | 前端构建 | `cd frontend && npm install && npm run build` | 生成 dist/ 产物 |
| 1.4 | 无 vet 警告 | `go vet ./...` | 无输出 |
| 1.5 | sqlc 生成一致 | `sqlc generate` | 无变更 (幂等) |

---

## 2. 抢机引擎逻辑测试

### 2.1 调度入口 (CheckAndExecuteTasksOnce)
| # | 场景 | 前置条件 | 操作 | 预期 |
|---|---|---|---|---|
| 2.1.1 | 无到期任务 | boot_instance 表无 status=1 的行 | 触发 CheckAndExecuteTasksOnce | 空返回，无日志错 |
| 2.1.2 | 有到期任务 | 插入一条 status=1, next_execution_time <= now | 触发一次 | FindDistinctTasksToExecute 返回该任务，提交到父池 |
| 2.1.3 | 租户不存在 | 任务的 tenant_id 无对应 tenant | 触发 | 跳过该任务，释放 key |
| 2.1.4 | 批量限制 | 50条到期任务，batchSize=30 | 触发 | 最多取30条 (SQL LIMIT) |
| 2.1.5 | 去重逻辑 | 同 tenant+arch 的2条任务 | 触发 | 只提交最早到期的那条 |

### 2.2 单飞去重 (Single-Flight)
| # | 场景 | 前置条件 | 操作 | 预期 |
|---|---|---|---|---|
| 2.2.1 | Key 被占用 | 任务A已持有 key "tenancy_reg_ARM" | 同 key 的任务B到达 | B被跳过，不占用线程 |
| 2.2.2 | Key 释放 | 任务A完成 | removeTaskKey | 下一轮B可以被调度 |
| 2.2.3 | 按 bootId 释放 | 两个任务 key 不同但同 bootId (不常见) | removeTaskKey | 只删除 value==bootId 的条目 |
| 2.2.4 | 异常路径释放 | processTask 中 status!=1 | removeTaskKey | key 被正确释放 |

### 2.3 双池隔离
| # | 场景 | 前置条件 | 操作 | 预期 |
|---|---|---|---|---|
| 2.3.1 | 父池满 | 所有父池 token 被占用 | 再提交任务 | submitParent 返回 false，key 被释放 |
| 2.3.2 | API池阻塞 | API 池正在执行长任务 | 新任务提交到 API 池 | runAPI 阻塞等待 token |
| 2.3.3 | 父子池独立 | 父池 goroutine 在 processTask 中 | processTask 提交后立即返回 | 父池 goroutine 释放，不占用 API pool |

### 2.4 非阻塞超时 (80s)
| # | 场景 | 前置条件 | 操作 | 预期 |
|---|---|---|---|---|
| 2.4.1 | OCI 响应超时 | Mock OCI 调用超过80s | executeGrabTaskAsync | 80s 后 context 到期，触发 onGrabFailure(Timeout) |
| 2.4.2 | 正常完成 | OCI 30s 返回 | executeGrabTaskAsync | select 收到 done，不触发超时 |
| 2.4.3 | 超时不阻塞线程 | 超时计时中 | 其他任务 | 其他任务不受影响，独立超时 |

### 2.5 幂等启动 (OpenBootLock)
| # | 场景 | 前置条件 | 操作 | 预期 |
|---|---|---|---|---|
| 2.5.1 | 首次启动 | open_boot_lock 无记录 | launchInstance | INSERT PROCESSING → 启动 → UPDATE SUCCESS |
| 2.5.2 | 已成功 (幂等快路径) | lock 记录 status=SUCCESS | launchInstance (重复调用) | 直接返回成功，不重复调用OCI |
| 2.5.3 | PROCESSING 并发 | 两个 goroutine 同时启动 | launchInstance | 第二个 INSERT OR IGNORE 不生效，FindLockByTaskID 返回第一条 → "task already in progress" |
| 2.5.4 | 失败后重试 | 第一次启动失败 | launchInstance 再调 | lock 被删除 → 重新 INSERT |

### 2.6 失败分类
| # | 场景 | 错误类型 | 操作 | 预期 |
|---|---|---|---|---|
| 2.6.1 | DNS 解析失败 | UnknownHostException | onGrabFailure | isNetworkTemporaryError=true，不增 failCount |
| 2.6.2 | 连接被拒 | ConnectException | onGrabFailure | isNetworkTemporaryError=true，不增 failCount |
| 2.6.3 | OCI API 错误 | BmcException | onGrabFailure | isNetworkTemporaryError=false，failCount+1，Telegram通知 |
| 2.6.4 | 超时 | TimeoutException | onGrabFailure | failCount 不增 (临时错误) |

### 2.7 成功事件链
| # | 场景 | 前置条件 | 操作 | 预期 |
|---|---|---|---|---|
| 2.7.1 | 抢机成功 | OCI 返回 Running 实例 | onGrabSuccess | status=2, public_ip 更新, successCount+1 |
| 2.7.2 | 幂等通知 | notifyFlag=NO | MarkNotificationAsSent | 行更新 (notifyFlag → YES)，触发通知 |
| 2.7.3 | 重复通知 | notifyFlag=YES (已通知过) | MarkNotificationAsSent | 0行更新，不触发通知 |
| 2.7.4 | 3分钟备份 | 抢机成功 | time.AfterFunc | 3分钟后 scheduleBackup 被调用 |
| 2.7.5 | TemInstance 记录 | 抢机成功 | saveTemInstance | tem_instance 表插入一行 |

---

## 3. API 集成测试 (HTTP)

### 3.1 Boot 任务 CRUD
| # | 方法 | 路径 | 请求 | 预期 |
|---|---|---|---|---|
| 3.1.1 | GET | `/boot/list` | — | 200, 返回 BootTask[] |
| 3.1.2 | POST | `/boot/save` | `{tenantId:1, ocpu:4, memory:24, disk:100, architecture:"ARM"}` | 200, 任务创建 |
| 3.1.3 | POST | `/boot/save` | 缺少 tenantId | 400, "tenantId required" |
| 3.1.4 | POST | `/boot/save` | `{bootId:"xxx", ...}` | 200, 任务更新 |
| 3.1.5 | GET | `/boot/delete?bootId=xxx` | — | 200, 任务 status=0 |
| 3.1.6 | GET | `/boot/delete?bootId=` | — | 400, "bootId required" |
| 3.1.7 | GET | `/boot/toggle?bootId=xxx&enable=1` | — | 200, 任务启用 |
| 3.1.8 | GET | `/boot/toggle?bootId=xxx&enable=0` | — | 200, 任务暂停 |

### 3.2 引擎状态
| # | 方法 | 路径 | 请求 | 预期 |
|---|---|---|---|---|
| 3.2.1 | GET | `/boot/systemStatus` | — | 200, 包含 pool 指标 |
| 3.2.2 | GET | `/boot/tenants` | — | 200, 返回租户下拉列表 |

### 3.3 认证 (回归)
| # | 方法 | 路径 | 请求 | 预期 |
|---|---|---|---|---|
| 3.3.1 | GET | `/boot/list` | 无 Cookie | 401 |
| 3.3.2 | GET | `/boot/list` | 过期 Cookie | 401 |

---

## 4. 调度器测试

| # | 测试项 | 方法 | 预期 |
|---|---|---|---|
| 4.1 | CreateInstanceJob 注册 | 查看日志 | 每6秒输出 "grabber: found expired tasks" (或 "没有到期任务") |
| 4.2 | BootInstanceRefreshJob | 系统时间在 00:00 | 日志输出 "scheduler: BootInstanceRefreshJob completed" |
| 4.3 | Graceful Shutdown | SIGTERM | 日志: "scheduler: stopped" → "grabber: shutting down" → "grabber: stopped" → "stopped" |
| 4.4 | Stale Lock 清理 | 启动前 open_boot_lock 有 PROCESSING 行 | 启动后 PROCESSING 行被删除 |

---

## 5. 前端测试

| # | 页面 | 测试项 | 预期 |
|---|---|---|---|
| 5.1 | `/boot` | 页面加载 | 显示引擎状态卡片 + 任务表格 |
| 5.2 | `/boot` | 新建按钮 | 打开对话框，租户下拉加载，表单可填写 |
| 5.3 | `/boot` | 保存任务 | 创建成功，表格刷新 |
| 5.4 | `/boot` | 编辑任务 | 点击编辑，表单预填当前值，保存更新 |
| 5.5 | `/boot` | 删除任务 | 确认对话框，删除后表格更新 |
| 5.6 | `/boot` | 启用/暂停 | 切换按钮文字和颜色 |
| 5.7 | 侧边栏 | "抢机任务"菜单项 | 当前页面高亮，文字显示正确 |

---

## 6. 性能测试 (建议)

| # | 场景 | 指标 | 目标 |
|---|---|---|---|
| 6.1 | 空任务轮询 | CPU/内存 | <1% CPU, <20MB 额外内存 |
| 6.2 | 100任务批次 | 调度耗时 | <100ms per tick |
| 6.3 | 并发单飞 | activeKeyCount | ≤ API池大小 |

---

## 7. 已知限制 / 待后续阶段完成

| 功能 | 状态 | 计划 |
|---|---|---|
| Telegram 通知 | Stub (日志记录) | Phase 7 |
| 启动卷备份 | Stub (日志记录) | Phase 5/6 |
| SSH enableRoot | 未实现 | Phase 6 |
| OciComputerInfo 缓存路径 | TODO 注释 | Phase 4 后续 |
| InstanceTrafficJob | Stub | Phase 5 |
| CheckLiveJob | Stub | Phase 5 |
| VNIC/IPv6/IP切换 | 未实现 | Phase 5 |
| GCP 抢机 | 未实现 | Phase 5 |
