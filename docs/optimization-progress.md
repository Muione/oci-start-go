# oci-start-go 优化实施进度

> 计划详见 [optimization-plan.md](./optimization-plan.md)。
> 状态图例：⏳ pending ｜ 🔄 in-progress ｜ ✅ done ｜ ⚠️ partial ｜ ❌ blocked
> 每批完成后跑 `go build ./... && go vet ./... && go test ./...`（并发项 `-race`）全量核验。
> 实施方式：按包 disjoint 分组派遣并行子 agent，分 3 批 + S4 专项。

## 批次总览
- **Batch 1（叶包，并行）**：util / notify / scheduler / oci / migration / auth+bootstrap+sysconf / config / repo — ✅ 8/8 完成，全量 build/vet/test 绿
- **Batch 2（中包，并行）**：service / ws — ✅ 完成，全量 -race 绿
- **Batch 3（上层，并行）**：httpapi / cmd — ✅ 完成，全量 -race 绿
- **Wave 2（S4 专项）**：root 密码加密跨包 — ✅ 完成（含 rescue 缺口补丁），最终全量 -race rc=0

## 项级进度

### P0 安全
| ID | 项 | 文件 | 状态 | agent | 测试 | 备注 |
|---|---|---|---|---|---|---|
| S1 | monitor 模板注入 | httpapi/monitor_api.go | ✅ | httpapi-agent | red→green | interval Atoi[1,3600]+token正则, 4 tests(恶意/坏token/越界→400, 合法→200) PASS -race |
| S3 | reset code 回显 | httpapi/auth_password_reset.go | ✅ | httpapi-agent | red→green | 移除响应 code 字段, 只回通用提示, 测试断言无 code PASS |
| S2 | WS 鉴权 + CSWSH | httpapi/server.go + ws/* | ✅ | httpapi+ws | red→green | S2-WS✅(CheckOrigin 同源) + S2-server✅(6 WS 路由入 pro 组, 无token→401 不升级); 测试 PASS |
| S4 | root 密码加密 | grabber+service+httpapi+migration | ✅ | S4-agent+主 | red→green | 写入侧加密(grabber success+httpapi instanceSaveSSHConfig) + 读侧解密(InstanceDetailSvc.SetMasterKey+GetRootPassword, fallback) + 导出[redacted]; 5 tests -race PASS; rescue 缺口已补(SSHConfigurator.SetMasterKey+decryptIfSet, main 装配, +1 test) |
| S5 | cookie Secure | auth/cookie.go | ✅ | auth-agent | red→green | Secure:true+SameSite=Lax, SetSession/Clear 两处, cookie_test PASS |
| S6 | turnstile token 脱敏+常量时间 | bootstrap/turnstile.go | ✅ | auth-agent | red→green | 日志改 token_hash(sha256前8字节)+URL去token, ConsumeAndRotate 用 subtle.ConstantTimeCompare, turnstile_test PASS |
| S7 | migration SQL 注入 | migration/importer.go | ✅ | migration-agent | red→green | identRe 白名单+validateIdentifiers 在 executeInsert 最早期; 与 B6 PRAGMA 字面化同根因一并修; 注入用例断言被拒 |
| S8 | ssh_config 命令注入 | service/ssh_config.go | ✅ | service-agent | red→green | EnableRootLogin 改 chpasswd+stdin 传密码(无插值), 元字符密码注入测试 PASS; execScript 加 stdin 参(未导出) |
| S9 | IP 伪造 | util/ip/ip.go | ✅ | util-agent | red→green | ClientIP 仅信可信代理(loopback+env OCI_START_TRUSTED_PROXIES) XFF, 4 调用方自动受益, 11 tests PASS -race |
| S10 | SSH host key MITM | ws/ssh.go | ✅ | ws+cmd | red→green | hostKeyCallback 默认安全(known_hosts fail-closed) + ws.SetHostKeyVerify setter + config.SSH.HostKeyVerify(默认true, 可false兼容) + main 装配; SSH-WS 不再默认 broken; ssh_hostkey_test PASS |

### P1 并发
| ID | 项 | 文件 | 状态 | agent | 测试 | 备注 |
|---|---|---|---|---|---|---|
| 0.1 | safeConn 封装 | ws/ws.go | ✅ | ws-agent | red→green | writeTimeout(10s)+mu+SetWriteDeadline, writeMessage/writeJSON, 共用; safeconn_test -race PASS |
| C6 | Hub.Shutdown 补 Console/Rescue | ws/ws.go | ✅ | ws-agent | red→green | 加 Console.Shutdown()+新增 RescueHandler.Shutdown(Cancel+done 2s best-effort), shutdown_test PASS |
| C1 | ssh stdout 竞态 | ws/ssh.go | ✅ | ws-agent | red→green | conn 包 safeConn, pumpStdout 抽出, sshSession done 通道, -race PASS |
| C2 | console tunnel 竞态 | ws/console.go | ✅ | ws-agent | red→green | controlWS *safeConn, HandleConsole/createConn/disconnect+tunnel-wait 全走 sc, -race PASS |
| C3 | monitor Broadcast 竞态 | ws/monitor.go | ✅ | ws-agent | red→green | sessions map[*safeConn], 快照+放锁+deadline, 死conn清理, pong走sc, -race PASS |
| C4 | rescue deps 竞态 | ws/rescue.go | ✅ | ws-agent | red→green | runRescueFlow 按值收 deps(不读 h.deps), rescueFlow.sc, send/handleStatus/handleCancel 走 sc, -race PASS |
| C5 | rescue init 覆盖 | ws/rescue.go | ✅ | ws-agent | red→green | handleInit 锁内先关旧 flow Cancel+delete 再存新(仿console), 测试旧goroutine退出 |
| C7 | log broadcast 队头阻塞 | ws/log.go | ✅ | ws-agent | red→green | sessions map[*safeConn], 快照+放锁+deadline, 死conn清理, sendRecentLogs走sc, -race PASS |

### P2 错误
| ID | 项 | 文件 | 状态 | agent | 测试 | 备注 |
|---|---|---|---|---|---|---|
| E3 | cron.AddFunc 错误 | scheduler/scheduler.go | ✅ | scheduler-agent | red→green | 13 处(非11) 全覆盖, addFunc helper, 2 tests PASS, go build ./... exit0 |
| E4 | sysconf 吞 DB 错误 | sysconf/sysconf.go | ✅ | auth-agent | red→green | Service 加 logger+SetLogger, 非 ErrNoRows 错误 warn(缺键不刷屏), sysconf_test PASS |
| E8 | rand.Read 错误 | migration/import.go + auth_password_reset.go | ✅ | migration+httpapi | 双侧 red→green | migration侧✅(GenerateIV→([]byte,error), GenerateMasterKey守铁律4 CSPRNG失败panic) + httpapi侧✅(randRead seam 检查→500) |
| E9 | reset persist 错误 | httpapi/auth_password_reset.go | ✅ | httpapi-agent | red→green | 检查 UpsertConfigValue 错误, 失败 500, close写池测试 PASS |
| E6 | proxySave 错误 | httpapi/proxy.go | ✅ | httpapi-agent | red→green | ParseInt失败400(仿proxyDelete) + Update失败500, proxy_test PASS |
| E5 | sslIssue persist 错误 | httpapi/dns.go | ✅ | httpapi-agent | red→green | persistCert helper 检查5个SetString, 任一失败500, certObtain seam 测试 PASS |
| E1 | email Send 吞错 | service/email_send.go | ✅ | service-agent | red→green | errors.Join 捕获 7 处持久化错误返非 nil(原返 nil), SMTP+ABORT 测试 PASS -race |
| E2 | instance 本地 DB 同步 | httpapi/instance_detail.go | ✅ | httpapi-agent | red→green | Terminate/ChangeIP/EnableIPv6 检查本地DB更新错, 失败 Logger.Error+syncFailed标记, 4 OCI op seam 测试 PASS |
| E7 | rescue CompleteRescue 吞错+tenantID | ws/rescue.go | ✅ | ws-agent | red→green | Stop/Detach/Attach/Start 逐步查错发 RescueStatus{Step:error}+中止; tenantID 从 flow.tenantID 取(修0 bug) |

### P3 性能
| ID | 项 | 文件 | 状态 | agent | 测试 | 备注 |
|---|---|---|---|---|---|---|
| P-1 | tenant List N+1 | service/tenant.go + repo | ✅ | service+repo | red→green | List 改单次 ListTenantsWithCounts, 16查询→1(计数测试证明); repo+service 双侧完成; 注:留旧 toTenantResp 未用待清理 |
| P-2 | tenantNameMap 缓存 | service/instance_detail.go | ✅ | service-agent | red→green | atomic.Pointer[tenantNameCache]+TTL(5min); GetByID/ListByTenant 改 FindTenantByID 单租户; 100租户行数测试+改名刷新 PASS -race |
| P-3 | GetPrimaryVnic 冗余 | oci/network.go | ✅ | oci-agent | red→green | 单遍缓存 2N→≤N, 注入 getVnic, 4 tests PASS -race |

### P4 bug+测试
| ID | 项 | 文件 | 状态 | agent | 测试 | 备注 |
|---|---|---|---|---|---|---|
| B1 | Feishu HMAC key | notify/channels.go | ✅ | notify-agent | red→green | 一行修复 key=f.Secret, 3 tests(Feishu/DingTalk/Telegram) PASS -race; 注:base64 sign 未 url.QueryEscape 同 DingTalk, 留待另项 |
| B2 | ParseEncryptedFile 空 key | migration/import.go | ✅ | migration-agent | red→green | 早返 "masterKey required" 清晰错误, ParseEncryptedFileWithKey 往返 PASS |
| B3 | sysconf 有损 upsert | sysconf/sysconf.go | ⚠️ | auth-agent | N/A(误报) | 审计前提不成立: 当前 sqlc ON CONFLICT DO UPDATE SET 只更新目标列保留另一列, 非有损; 留 2 回归测试防退化, 未改代码 |
| B4 | crypto 测试 | util/crypto/aesgcm_test.go | ✅ | util-agent | PASS | 表驱动往返+篡改/短blob/错key/非法len/非法base64; 未改 aesgcm.go; +fallback.go(DecryptStringWithFallback 供 S4) |
| B5 | session/middleware 测试 | auth/* | ✅ | auth-agent | red→green | session_test+middleware_test; 额外修复 TZ bug(ParseInLocation time.Local→UTC, Create/Touch→UTC); SessionAuth/IpBan/TenantContext 覆盖 |
| B6 | importer 解析测试 | migration/importer.go | ✅ | migration-agent | red→green | 表驱动 splitValues/parseInsertSQL + ImportSQLText 集成; 额外修 2 真 bug: PRAGMA table_info(?)占位符无效(每行被skip)+PEM行缓冲丢末行; splitValues ''转义实测无问题 |

### P5 质量
| ID | 项 | 文件 | 状态 | agent | 测试 | 备注 |
|---|---|---|---|---|---|---|
| Q1 | oci-clients helper | httpapi/instance_detail.go | ✅ | httpapi-agent | refactor绿 | ociClientsForInstance(seam)+respondOciClientsErr, 7 handler 去重样板; 未碰密码字段(留S4) |
| Q2 | zerolog hack | cmd/oci-start/main.go | ✅ | cmd-agent | build OK | 删 hack 行+zerolog import, grep 确认零引用, logger 经 := 推断 |
| Q3 | main SQL 下沉 | cmd/oci-start/main.go + repo | ✅ | cmd+repo | build OK | 3 处内联SQL→repo FindConsole/RescueInstanceInfo/FindCompartmentID, WithTimeout(30s)包裹, 未改 ConsoleDeps/RescueDeps 签名, +nis(NullInt64)helper |
| Q4 | build/setup | scripts/* | ✅ | config-agent | bash -n OK | build.sh CGO 1→0(静态), setup.sh GO_MIN 1.22→1.25 |
| Q5 | config print/replace | config/config.go | ✅ | config-agent | red→green | remapPrefix 显式替换+未命中告警, L140 Println→stderrf, 4 tests PASS |

## 变更日志
- 2026-07-01 init: CLAUDE.md + .claude/settings.json 已写；绿基线确认；计划文件 docs/optimization-plan.md 已写。
- 2026-07-01 dispatch Batch 1: 8 background agents launched — util(B4,S9,+DecryptStringWithFallback), notify(B1 Feishu HMAC), scheduler(E3), oci(P-3 GetPrimaryVnic), migration(B2,B6,S7,E8), auth+bootstrap+sysconf(S5,S6,B5,E4,B3), config(Q4,Q5), repo(P-1 tenant aggregate query, Q3 console/rescue repo methods). 各 agent 严格 TDD、只改/只测自己的包（避免跨包编译竞态）。完成后逐个汇报 → 更新本文件 → 跑全量构建 → 进 Batch 2。
- 2026-07-01 ✅ Batch1 [1/8] scheduler-agent E3 done: addFunc helper 覆盖 13 处 AddFunc, scheduler_test.go 2 tests red→green, `go build ./...` exit 0.
- 2026-07-01 ✅ Batch1 [2/8] oci-agent P-3 done: getPrimaryVnicFromAttachments 单遍缓存 2N→≤N, 4 tests red→green, `go test ./internal/oci/ -race` 绿。
- 2026-07-01 ✅ Batch1 [3/8] notify-agent B1 done: Feishu HMAC key 一行修复(str→f.Secret), channels_test.go 3 tests red→green, -race+vet 干净。
- 2026-07-01 ✅ Batch1 [4/8] util-agent done: B4 crypto 信任根测试(未改 aesgcm.go) + 新增 DecryptStringWithFallback(fallback.go, 供 S4) + S9 ClientIP 仅信可信代理 XFF(11 tests red→green, 4 调用方自动受益)。`go test ./internal/util/{crypto,ip}/... -race` 全绿。
- 2026-07-01 ✅ Batch1 [5/8] config-agent done: Q4 build.sh CGO→0 + setup.sh GO_MIN→1.25 (bash -n OK); Q5 applyDataPath remapPrefix+告警, L140→stderrf, config_test.go 4 tests red→green, 公开 API 未动。
- 2026-07-01 ✅ Batch1 [6/8] auth-agent done: S5 cookie Secure+SameSite; S6 turnstile token 脱敏(hash前8字节)+ConstantTimeCompare; B5 session/middleware 测试 + 额外修复 TZ bug(Local→UTC); E4 sysconf 加 logger 非 ErrNoRows warn。B3 经核实为审计误报(当前 SQL 非有损), 留回归测试未改代码。三包 -race+vet 全绿。
- 2026-07-01 ✅ Batch1 [7/8] migration-agent done: B2 ParseEncryptedFile 早返清晰错误; B6 importer 测试 + 额外修 2 真 bug(PRAGMA table_info(?)占位符无效致每行被skip + PEM行缓冲丢末行; splitValues ''转义实测无问题); S7 identRe 白名单+validateIdentifiers(与B6同根因); E8 migration侧 GenerateIV→([]byte,error), GenerateMasterKey 守铁律4保持(string)CSPRNG失败panic。`go test ./internal/migration/ -race` 全绿。E8 的 auth_password_reset 侧留 Batch3。
- 2026-07-01 ✅ Batch1 [8/8] repo-agent done: P-1 repo侧 ListTenantsWithCounts 聚合(3相关子查询避LEFT JOIN行乘积, parity测试); Q3 repo侧 FindConsoleInstanceInfo/FindRescueInstanceInfo/FindCompartmentID(真实列类型, 测试PASS)。未碰 main.go/querier.go。`go build/test ./internal/repo/ -race` 全绿。Batch 1 全部完成 → 跑全量构建核验。
- 2026-07-01 ✅ Batch 1 全量核验: `go build/vet/test ./...` 全绿（auth/bootstrap/config/httpapi/migration/notify/oci/repo/scheduler/service/sysconf/util/{crypto,ip,httpclient} 全 ok）。无回归。
- 2026-07-01 dispatch Batch 2: service-agent(E1 email吞错, P-1 service侧用 ListTenantsWithCounts, P-2 tenantNameMap缓存, S8 ssh_config命令注入) + ws-agent(0.1 safeConn, C6 Hub.Shutdown补Console/Rescue, C1 ssh, C2 console, C3 monitor, C4 rescue deps, C5 rescue init, C7 log, E7 rescue CompleteRescue+tenantID, S10 SSH host key, S2-WS CheckOrigin)。ws 不导入 service，并行安全。
- 2026-07-01 ✅ Batch2 [1/2] service-agent done: E1 Send 用 errors.Join 捕获持久化错误返非 nil; P-1 List 16查询→1(ListTenantsWithCounts, 计数测试); P-2 atomic.Pointer+TTL 缓存+FindTenantByID 单租户(100租户行数测试); S8 chpasswd+stdin 传密码(元字符注入测试)。`go test ./internal/service/ -race`+vet+build 全绿。注:留旧 toTenantResp 未用待清理。
- 2026-07-01 ✅ Batch2 [2/2] ws-agent done (11 项全完, 17 tests -race PASS): 0.1 safeConn; C6 Hub.Shutdown补Console+新RescueHandler.Shutdown; C1-C5/C7 各 handler conn 包 safeConn+快照放锁+死conn清理; E7 CompleteRescue 逐步查错发Error中止+tenantID从flow取(修0bug); S10 hostKeyCallback 默认安全(需batch3接config); S2-WS CheckOrigin 同源。⚠ backlog: S10 需 cmd 接 config 否则 SSH-WS 默认拒连; rescueFlow Status/Step 字段竞态建议 send() 加锁; CompleteRescue 不响应 Cancel。
- 2026-07-01 ✅ Batch 2 全量 -race 核验: `go build/vet/test -race ./...` 全绿（ws 17 tests + service + oci + repo 等，无竞态无回归）。
- 2026-07-01 dispatch Batch 3: httpapi-agent(S1 monitor注入, S3 reset回显, E9 reset persist, E8reset rand, E6 proxySave, E5 sslIssue, E2 instance本地同步, Q1 oci-clients helper, S2-server WS路由入pro组) 后台运行。cmd-agent(Q2 zerolog hack, Q3 main SQL→repo方法, S10 host_key_config 接线) 待 httpapi 完成后派（cmd 导入 httpapi）。
- 2026-07-01 ✅ Batch3 [1/2] httpapi-agent done (9 项全完, 14 新测试 -race PASS, 公开API零改动): S1 interval/token校验; S3 移除code回显; E9/E8reset/E6/E5 错误检查+500; E2 三handler检查本地同步错(syncFailed标记); Q1 ociClientsForInstance helper 去7处样板; S2-server 6 WS路由入pro组(无token→401)。包级测试缝(randRead/certObtain/ociClientsForInstance/4 OCI op), 未碰密码(留S4)。`go build ./...`+vet+`go test ./internal/httpapi/ -race -count=1` 全绿。
- 2026-07-01 dispatch cmd-agent: Q2(删 zerolog hack) + Q3(main.go 3处内联SQL→repo FindConsoleInstanceInfo/FindRescueInstanceInfo/FindCompartmentID + WithTimeout包裹) + S10接线(ws 加 SetHostKeyVerify setter + config 加 SSH.HostKeyVerify 字段 + main 按config 装配, 默认安全但可配置关闭)。S4-agent 待 cmd 完成后派（cmd 导入 S4 要改的包）。
- 2026-07-01 ✅ Batch3 [2/2] cmd-agent done: Q2 删 zerolog hack+import; Q3 3处内联SQL→repo方法+WithTimeout(30s)+nis(NullInt64)helper, 未改 ConsoleDeps/RescueDeps 签名; S10 ws.SetHostKeyVerify setter+config.SSH.HostKeyVerify(默认true安全,可false兼容)+main装配+ssh_hostkey_test。build/vet/test 全绿, -race ws 通过。SSH-WS 不再默认 broken。Batch 3 全完成。
- 2026-07-01 dispatch Wave 2 S4-agent: root 密码 AES-256-GCM 加密入库(grabber/success 写入侧) + 读时解密(DecryptStringWithFallback 兼容旧明文, service/httpapi 读取侧) + 导出脱敏(httpapi instanceExport/migration export) + InstanceDetailSvc.SetMasterKey setter(main 装配). TDD: 写入断言密文≠明文; 读密文→明文; 旧明文→明文(降级); 导出不含明文.
- 2026-07-01 ✅ Wave 2 S4-agent done (5 tests -race PASS, build/vet/test rc=0, 无导出签名变更): 写入侧 grabber success saveTemInstance + httpapi instanceSaveSSHConfig 加密(EngineDeps/deps.MasterKey); 读侧 InstanceDetailSvc.SetMasterKey+GetRootPassword(DecryptStringWithFallback); 导出 instanceExport [redacted]; migration 无 password 引用; console Password 未用于SSH认证无影响; 向后兼容旧明文. ⚠ 已知缺口: rescue 流程 EnableRootLogin 读新密文→SSH拨号失败(非致命, 实例已救援, 仅自动root-login降级) — 待补 SSHConfigurator.SetMasterKey + EnableRootLogin 解密。
- 2026-07-01 ✅ S4 rescue 缺口补丁(主内联 TDD): SSHConfigurator 加 masterKey+SetMasterKey+decryptIfSet, EnableRootLogin 解密 password/rootPassword(ciphertext→解密/plaintext→降级), main.go 装配 sshConfig.SetMasterKey(masterKey); ssh_config_test +1 red→green。最终全量 `go build/vet/test -race ./...` rc=0 全绿。**全部优化项完成。**

---

## VNC 控制台 409 IncorrectState 修复（2026-07-02）

**用户报错**：VNC 连接 OCI 实例返回 `409 IncorrectState ... Console connection ... already exists or has not been terminated`。

**根因**：OCI 每实例同时只允许 1 个 console connection，且 `DeleteInstanceConsoleConnection` 异步（`DELETING→DELETED`），未到 DELETED 前 `Create` 必返 409。代码三个缺口叠加：
1. `handleDisconnect`/tunnel 退出都**不删 OCI 连接** → 连接常驻 ACTIVE，永不回收。
2. `FindActiveConsoleConnection` 复用遗留 ACTIVE 连接但无私钥（私钥每会话生成不持久化）→ 必落入"无 key 重建"分支：`CleanupConsoleConnections`(Delete 不等) → 立即 `GenerateConsoleConnection`(Create) → 旧连还在 DELETING → **409**。
3. Create 409 无兜底，原文抛用户。

**修复（TDD，先红后绿）**：
- `internal/oci/console.go` 新增：`EnsureConsoleConnection`(清理→等清理→建→等ACTIVE，409 重试一次)、`WaitForConnectionsCleared`、`GetConsoleConnectionInfo`、`isConsoleConflict`(409 识别，errors.As 穿透 %w 包装，仿 `isNotFound`)、`consoleOps` 测试缝 + `waitForCleared`/`clearAll`/`waitForActive`/`ensureConsoleConnection` 可测核心。`console_ensure_test.go` 11 tests -race PASS。
- `internal/ws/console.go`：重写 `handleCreateConnection` 砍掉复用/重建分支，改用 `EnsureConsoleConnection`；`consoleSession` 加 `tenantID`/`compartmentID`/`deleteOnce sync.Once`；新增 `deleteOciConn(deps)`(best-effort 删 OCI 连接, Once 防止 disconnect+tunnel 退出双删)；`handleDisconnect`+tunnel 退出 goroutine+旧 session 覆盖 均调 `deleteOciConn`；`friendlyConsoleErr`(持续 409 给中文提示)。`deleteConsoleConn` 包级 seam 变量供单测。`console_disconnect_test.go` 4 tests -race PASS。
- **前端** `frontend/src/views/Console.vue`：① 关键 bug——后端发 `vncUrl` 前端读 `vncWsUrl`，VNC 拿不到 URL 永远连不上；改为读 `vncUrl`。② RFB `disconnect` 事件回发 `disconnect` 让后端删 OCI 连接(闭环 409 根因)。③ `onerror` 补 `cleanupNoVNC`。`npm run typecheck` 通过。

**保留为导出 API（应用内已无调用方，但不改公开签名）**：`FindActiveConsoleConnection`/`WaitForConnectionActive`/`CleanupConsoleConnections`/`CreateConsoleConnection`。

**核验**：`go build/vet/test -race ./...` rc=0 全绿；前端 typecheck 通过。

**临时缓解（不改代码即可连）**：`oci compute console-connection list/delete` 手动删遗留连接后重连。

---

## VNC 控制台 SSH 隧道修复 + 连接管理功能（2026-07-02）

### SSH 隧道 port not ready 修复
**报错**：`SSH tunnel port not ready: port 43495 not ready after 15s`（409 已修，连接能建，但 SSH 隧道起不来）。

**根因 1（字段用错）**：SDK 字段文档明确 `VncConnectionString` = "用于通过 VNC 连接的 SSH 隧道字符串"，`ConnectionString` 是串口控制台的。代码解析的是 `ConnectionString`（串口）→ VNC 目标错误。改 `handleCreateConnection`/`startTunnelAndNotify` 解析 `connInfo.VncConnectionString`。`tunnel_test.go` 3 tests 锁定真实 OCI VNC 格式。
**根因 2（诊断缺失）**：ssh stderr 被丢到 `os.Stderr`，前端只看到 "port not ready"。`formatTunnelError` 把 ssh 合并输出尾部拼进错误消息（`console_tunnel_test.go` 1 test），真实原因（Permission denied / Load key invalid format / Could not resolve host）现在直显前端。

### VNC 控制台连接管理功能（用户需求：列出/恢复/删除/新建）
方案 A（持久化恢复）：新建 DB 表 `console_connections`（已存在，0001 建表；0009 加 `encrypted_private_key`+`public_key_ssh`+instance_id 唯一索引），AES-256-GCM 加密私钥入库，断开时**保留** OCI 连接（回退上轮"断开即删"——409 由 `EnsureConsoleConnection` 创建时清理兜底，仍有效），跨会话可恢复。

**分层 TDD 实现**：
- **Phase 1 repo**（`console_connection_extra.sql.go` + 迁移 0009）：`UpsertConsoleConnection`/`GetConsoleConnectionByInstance`/`DeleteConsoleConnectionByInstance` + `ConsoleConnectionRow`。`console_connection_extra_test.go` 3 tests（insert/replace、NotFound、delete+no-op）-race 绿。
- **Phase 2 service**（`console_connection.go`）：`ConsoleConnectionService`（Persist 加密 / LoadForResume 解密 / List OCI+DB join / Delete）；`joinOurs` 纯函数标记 IsOurs+CanResume；`ConsoleConnectionLister` 接口供 httpapi 注入；oci.List/Delete seam 变量。`console_connection_test.go` 8 tests（往返+密文校验、joinOurs 4 场景、List join、NoRow、Delete 删行/非本应用留行）-race 绿。
- **Phase 3 ws**（`console.go`）：回退 `deleteOciConn`/`deleteConsoleConn` seam/`deleteOnce`/`tenantID`/`compartmentID` 字段及三处调用；抽出 `startTunnelAndNotify`（create+resume 共用隧道启动）；`create_connection` 后 `Persist`（best-effort）；新增 `resume_connection` 消息（Load→GetConsoleConnectionInfo→ACTIVE 则复用隧道，非 ACTIVE 提示删建）；`getConsoleConnectionInfo` seam。`console_resume_test.go` 4 tests（无保存/非ACTIVE/Get错/空key）-race 绿。删旧 `console_disconnect_test.go`（测的是已移除断开即删）。
- **Phase 4 httpapi**（`console_connection.go`）：`GET /instances/:id/console-connections`（list）+ `DELETE /instances/:id/console-connections/:connId`（delete），`:id`=实例 OCID。`ConsoleConnSvc` 接口字段入 Deps。`console_connection_test.go` 7 tests（404/200/500 × list+delete + 全路由注册无 Gin 冲突冒烟）-race 绿。
- **Phase 5 main.go**：抽 `buildClients` 共享闭包；`service.NewConsoleConnectionService(store, masterKey, buildClients)`；ConsoleDeps 装配 `Persist`/`LoadForResume`；httpapi Deps 装配 `ConsoleConnSvc`。
- **Phase 6 前端**（`Console.vue` 重写）：选中实例自动 `GET` 连接列表 → el-table 展示（connId 截断+tooltip、状态、本应用/其他 tag、恢复/删除按钮）；"新建连接"按钮=create_connection；恢复=resume_connection；删除=DELETE+刷新；切实例自动断旧 VNC+重载；RFB 断开回发 disconnect。`npm run typecheck` 通过。

### 兼容性
- 不改公开 API 签名（新加导出：`EnsureConsoleConnection`/`WaitForConnectionsCleared`/`GetConsoleConnectionInfo`/`ConsoleConnectionLister`/`ConsoleConnectionService`/`ConsoleConnectionView`/repo 三方法）。
- `console_connections` 表已存在（0001），0009 只加列+索引；表此前无查询无数据，加唯一索引安全。
- 断开保留 OCI 连接是新行为（支持恢复）；创建时 `EnsureConsoleConnection` 清理遗留，409 不复发。

### 核验
`go build/vet/test -race ./...` rc=0 全绿；前端 `vue-tsc --noEmit` 通过。新测试 ~26 个跨 repo/service/ws/httpapi/oci 五层。

---

## SSH 隧道 "invalid quotes" 修复 + 连接过程进度更新（2026-07-02）

### "invalid quotes" 根因
对照 Java 原版 `parseConnectionString`（`oci-start/.../ConsoleWebSocketHandler.java:714`）确认真实 OCI 字符串格式：`ssh -o ProxyCommand='... -W %h:%p -p 443 <connID>@<proxyHost>' <connID>@<targetHost>`——闭合单引号粘在 `<proxyHost>'` 后。Go 的 `strings.Fields` 不识别引号，把 `<connID>@<proxyHost>'`（带尾引号）整个当 ProxyHost → 拼进 `-o ProxyCommand=...` → ssh 见未闭合引号 → `command-line line 0: invalid quotes`。原 `tunnel_test.go` 用了错误格式（引号在 `%h:%p'` 后）所以没捕获。

### 修复
- `internal/oci/tunnel.go` `ParseConnectionString`：对每个提取的 token `strings.Trim(s, "'\"")`，杜绝引号泄漏。注释说明真实格式 + 内层 ssh 目标用 connID 作用户。
- `tunnel_test.go` 改用真实 OCI 格式（引号在 host 后）+ 新增 `TestBuildSSHTunnelCommand_NoQuoteLeak`（断言重建命令所有 arg 无引号字符）-race 绿。
- `ssh -G` 干净值冒烟确认多词 ProxyCommand 经 `-o` 直传（无 shell 引号）合法。

### 连接过程进度更新（用户需求）
后端 `handleCreateConnection`/`handleResumeConnection`/`startTunnelAndNotify` 各步骤发 `{"type":"output","data":"<step>\n"}` 到控制 WS：查询实例 / 构建 OCI 凭据 / 生成密钥 / 清理旧连接并创建控制台连接 / 控制台连接已就绪 / 加载已保存连接 / 检查连接状态 / 保存私钥 / 分配端口 / 启动 SSH 隧道 / 等待端口就绪。前端 `Console.vue` 已对 `output` 追加 statusLog；新增 `statusLogEl` ref + `watch(statusLog)` 自动滚到底。`console_resume_test.go` 改 `readUntilType`（单读者、跳过 progress 帧读到目标 type）替代 `drain`+`readMsg`（多读者竞争）。

### 核验
`go build/vet/test -race ./...` rc=0 全绿；前端 `vue-tsc --noEmit` 通过。

---

## SSH 隧道 -L/-W 目标错误修复（2026-07-02，基于诊断日志定位）

### 根因（诊断日志揭示）
对比用户日志里 OCI 原始 `vncConnectionString`：
```
ssh -o ProxyCommand='ssh -W %h:%p -p 443 <CONN-OCID>@instance-console.<r>' -N -L localhost:5900:<INSTANCE-OCID>:5900 <INSTANCE-OCID>
```
OCI 控制台代理**按实例 OCID 路由**：`-L` 转发目标 + 外层 ssh 目标都是 `<INSTANCE-OCID>`（不是代理本机/connID@代理）。我的 `BuildSSHTunnelCommand` 两处错：
1. `-L` 写 `<localPort>:127.0.0.1:5900`（代理本机 5900，无 VNC）→ `channel 0: open failed: connect failed: Unable to establish connection`。
2. 外层目标写 `<CONN-OCID>@<proxy>` → `-W %h:%p` 转发到 `<proxy>:22`（代理自己:22）→ `stdio forwarding failed`。

### 修复（`internal/oci/tunnel.go`）
- `ParseConnectionString`：新增 `instanceIDRegex` (`ocid1\.instance\.[a-z0-9]+\.[a-z0-9.-]+`，`.instance.` 两边点号避免误匹配 `instanceconsoleconnection`)，提取实例 OCID 为 `TargetHost`；保留 `ConnectionID`(conn-ocid) + `ProxyHost`(connID@proxyhost)；找不到任一即报错。
- `BuildSSHTunnelCommand`：`-L` 改 `<localPort>:<TargetHost>:<remotePort>`（实例 OCID:5900）；外层目标改 `cfg.TargetHost`（实例 OCID，去掉 `connID@` 前缀）。
- `tunnel_test.go`：改用真实 OCI VncConnectionString 格式（含 `-L localhost:5900:<INSTANCE-OCID>:5900` + 外层 `<INSTANCE-OCID>`）；新增 `TestBuildSSHTunnelCommand_TargetsInstanceID` 断言 `-L` 远端=实例OCID:5900 + 外层=实例OCID + 无引号泄漏。

### 预期
重连后 ssh 隧道应能正确转发到实例 5900。**若仍报 `Unable to establish connection`**：则实例本身无 VNC server（headless Linux；oci-start 不在实例上装 VNC）→ 需在实例装 VNC（x11vnc+桌面）或用串口/SSH。日志仍会显示真实 ssh stderr 供定位。

### 核验
`go build/vet/test -race ./...` rc=0 全绿。

---

## VNC 桥接 panic + 短写修复（2026-07-02）

### panic: close of closed channel（切页面闪退）
`HandleVNCBridge` 两个 copy goroutine 都 `defer close(done)`：一个先退出关 done→handler 返回关 conn→另一个 Read 返回后 defer 又 close(done)→重复关闭 panic。抽 `bridgeVNC` 用 `sync.Once` 守护 close（`signal := func(){ once.Do(func(){ close(done) }) }`）。`console_bridge_test.go` 1 test（双向字节流 + 关闭两端不 panic）-race 绿。

### VNC 画面乱码：WS→TCP 短写丢字节
`bridgeVNC` 的 WS→TCP 用 `tcpConn.Write(msg)` 忽略返回长度——`net.Conn.Write` 可短写，丢字节使 RFB 字节流错位→全屏乱码。改 write-all 循环（`for len(msg)>0 { n,_ := Write(msg); msg=msg[n:] }`）。TCP→WS 方向 gorilla `WriteMessage` 内部已 write-all，无需改。

### 判定残留乱码是远程还是前端
若修完仍乱码：用桌面 VNC 客户端连 SSH 隧道本地端口（进度日志"本地端口 XXXXX"），`vncviewer localhost:XXXXX`。桌面客户端也乱→远程 VNC server 编码/像素格式问题；桌面正常→前端 noVNC 设置（可试 `resizeSession=false`）。

### 核验
`go build/vet/test -race ./...` rc=0 全绿。

---

## 串口控制台文本终端（2026-07-02）

### 背景
实例是 headless（无桌面），noVNC 图形 VNC 不适用（5900 无 VNC server，OCI "VNC" 路由到串口 banner→乱码/断开）。改用 OCI ConnectionString（串口）建交互式 SSH，浏览器 xterm.js 显示文本终端（启动日志/登录/救援），无需桌面。

### 实现
- **oci 层** `BuildSerialConsoleCommand(cfg)`：`ssh -tt`（强制远端 PTY，即使 stdin 是 pipe）+ ProxyCommand（-i key -W %h:%p -p 443 connID@proxy）+ 外层目标=实例 OCID；无 -L/-N（交互 shell，非隧道）。`tunnel_test.go` +1 test（断言 -tt、无 -L/-N、外层=实例OCID、无引号泄漏）。
- **ws 层** `HandleSerialConsole`（`/ws/console/serial?instanceId=`）：升级 WS→`ensureSerialConn`（优先 Load 持久化连接恢复，否则 EnsureConsoleConnection 新建+Persist，复用 getConsoleConnectionInfo seam）→解析 `ConnectionString`（串口）→存 key→起 `ssh -tt`→双向桥（stdout→WS binary；WS JSON `{"type":"input","data"}`→ssh stdin write-all；`resize` 接受但忽略，exec ssh -tt 无运行时 resize，需本地 PTY 才支持，暂不引入 pty 库）。`<-done` 后 Kill+Wait+Close+Remove 一次（sync.Once 防 double-close）。`console_serial_test.go` 2 tests（lookup 失败→error JSON、无 instanceId→400）。
- **httpapi** 路由 `pro.GET("/ws/console/serial", ...)`（cookie 同源鉴权）。
- **前端** `Console.vue` 重写：xterm.js（@xterm/xterm + addon-fit）连 `/ws/console/serial?instanceId=`；onData→`{"type":"input"}`，onResize→`{"type":"resize"}`，onmessage→term.write（text=控制 JSON output/error，binary=终端字节）；保留连接管理列表（list/delete，串口 WS 内部建连，无需 resume/create 按钮）；keep-alive（Default.vue）保留——切页面串口不断。typecheck 通过。

### 已知限制
- 终端大小固定（ssh -tt 默认 80x24，运行时 resize 不生效）。需要可调大小可引入 `creack/pty` 给 ssh 本地 PTY + TIOCSWINSZ——留作后续。

### 核验
`go build/vet/test -race ./...` rc=0 全绿；前端 `vue-tsc --noEmit` 通过。

---

## 串口卡住 + keep-alive + SSH 终端重构（2026-07-02）

### 串口卡住修复
**根因**：OCI 串口 MOTD + 交互输出走 ssh 的 **stderr**，原代码 `Stderr=&sshOut` 只捕获不转发，stdout 空→前端无输出卡住。**修**：`serialWSWriter` 把 ssh stdout+stderr 合并转发到 WS（`Stdout=wsWriter`、`Stderr=io.MultiWriter(wsWriter,&sshOut)`），exec 内部 copy，safeConn 序列化 WS 写。ssh 退出检测改 `Wait`-goroutine→signal（避免双 Wait）。

### keep-alive 不生效
`<keep-alive :include="['console']">` 按组件 **name** 匹配，`<script setup>` 默认无 name→匹配不上→切页卸载→WS 关→ssh 被 kill。**修**：Console.vue + SSHTerminal.vue 加 `defineOptions({name:'console'/'terminal'})`；Default.vue include 改 `['console','terminal']`。

### SSH 终端全面重构（SSHTerminal.vue）
原：手动 host/port/user/pass、单会话、无 keep-alive。重构为：
- **多标签页会话**：自定义 tab 栏 + `v-show` 保活所有终端（el-tabs 只渲染活动 pane 会卸载 xterm，故用 v-show）。
- **实例选择自动填充**：选实例→填 host（publicIps/privateIps 首个）+ username + port。
- **最近连接**（localStorage，最多 8，点击套用）。
- **控制按钮**：重连/清屏/复制选中/全屏/断开。
- **正确 resize**：FitAddon + ResizeObserver，连接时 + 切 tab 时（onActivated）fit。
- **keep-alive**（name='terminal'）切页不断。
- 状态指示灯（已连接绿/连接中黄脉冲/未连灰）。

### 核验
`go build/vet/test -race ./...` rc=0 全绿；前端 `vue-tsc --noEmit` 通过。

---

## 串口白框修复 + SSH 密钥登录（2026-07-02）

### 串口开头白色输入框
**根因**：xterm.js 的 helper textarea（输入/IME 用）需 `@xterm/xterm/css/xterm.css` 隐藏，项目未导入 → 裸 textarea 显示为顶部白框。**修**：`main.ts` 加 `import '@xterm/xterm/css/xterm.css'`。vite build 确认 xterm chunk 构建。

### SSH 终端密钥登录 + 密钥存储
- **后端** `internal/ws/ssh.go`：connect 消息加 `privateKey`+`passphrase` 字段；新增 `buildAuth(password, privateKey, passphrase)`（密钥优先 → `ssh.PublicKeys(signer)`，`ParsePrivateKeyWithPassphrase` 处理口令；否则 `ssh.Password`）。`ssh_auth_test.go` 4 tests（密码/坏key/有效key/密钥优先）-race 绿。
- **前端** `SSHTerminal.vue`：认证方式 radio（密码/密钥）；密钥模式=已存密钥下拉 + 粘贴 PEM textarea + 口令；"管理密钥"对话框（el-dialog，标签+私钥+口令，列表删除）；密钥存 localStorage（`ssh-keys`），连接时按 authType 发 privateKey/passphrase 或 password。注明 localStorage 仅适合自托管（与后端 DB 加密模型不一致，留作后续升级）。

### 核验
`go build/vet/test -race ./...` rc=0 全绿；前端 `vue-tsc --noEmit` + `vite build` 通过。

---

## 控制台连接列表始终空（2026-07-02）

**根因**：`request` 拦截器已解包 ApiResponse（`return b.data`），`await request.get(...)` 返回的就是 `data` 字段（数组）。`Console.vue` 的 `loadConnections` 又写 `res.data`（再解一层）→ `undefined` → 列表恒空。**修**：`connections.value = Array.isArray(res) ? res : (res?.data || [])`。另加串口 WS onopen 后 5s 自动刷新列表（让建连期间也能看到连接）。

### 核验
前端 `vue-tsc --noEmit` 通过（仅前端改动，Go 未动）。

---

## SSH 密钥 DB 加密存储（2026-07-02）

将 SSH 终端的私钥存储从 localStorage 升级为 DB 加密（master key + AES-256-GCM，与租户凭据/root 密码一致模型）。私钥内容只在保存时发到服务端加密存储，连接时服务端按 id 解密使用，**内容永不回前端**。

### 实现（分层 TDD）
- **迁移 0010** `ssh_keys` 表（id, label, encrypted_key, encrypted_passphrase, fingerprint, created_at）。
- **repo** `ssh_keys_extra.sql.go`：`CreateSSHKey`/`ListSSHKeys`/`GetSSHKey`/`DeleteSSHKey` + `SSHKeyRow`。3 tests -race 绿。
- **service** `ssh_key.go` `SSHKeyService`：`Create`（验证+`fingerprint`+加密 key/pass）、`List`（返回 `SSHKeyView`，**无 content 字段**）、`Delete`、`Resolve`（按 id 解密返回 content+pass 给 WS）。`fingerprint` 用 `ssh.ParsePrivateKey[WithPassphrase]`→`ssh.FingerprintSHA256`。6 tests -race 绿。
- **httpapi** `ssh_key.go`：`GET/POST/DELETE /api/ssh-keys`，`SSHKeySvc service.SSHKeyLister` 接口字段入 Deps。5 tests -race 绿（含路由注册冒烟）。
- **ws** `ssh.go`：`SSHDeps{ResolveSSHKey}` + `SetDeps`；connect 消息加 `keyId`；`resolveKeyAuth`（keyId→ResolveSSHKey→buildAuth，否则 ad-hoc privateKey→buildAuth）。3 tests -race 绿。
- **main.go**：`sshKeySvc` 装配 + `wsHub.SSH.SetDeps`（ResolveSSHKey=sshKeySvc.Resolve）+ httpapi Deps。
- **前端** `SSHTerminal.vue`：savedKeys 改为 `GET /api/ssh-keys`（id/label/fingerprint，无 content）；新增 `POST /ssh-keys`（label+content+passphrase，仅保存时发送）；`DELETE /ssh-keys/:id`；连接时选了已存密钥发 `keyId`，否则发 `privateKey`+`passphrase`；管理对话框显示标签+指纹+删除，注明 DB 加密。移除 localStorage 密钥存储（recents 仍 localStorage，非敏感）。

### 核验
`go build/vet/test -race ./...` rc=0 全绿；前端 `vue-tsc --noEmit` + `vite build` 通过。新增 ~17 测试跨 repo/service/httpapi/ws 四层。

---

## 串口/SSH 终端输入无反应（2026-07-02）

**根因**：前端发 `{"type":"input","data":"<键击>"}`，`data` 是 **JSON 字符串**（如 "a"、"\\r"、箭头键转义）。后端却用 `var d struct{Data string}; json.Unmarshal(req.Data, &d)` 解析——把字符串往结构体塞，**静默失败**（error 被忽略）→ `d.Data` 空 → 写空字节到 ssh stdin → 键击丢失，typing 无反应。输出通路正常（login: 能显示）正因如此。

**修**：新增 `parseInputData(raw json.RawMessage) (string, error)`（直接 unmarshal 成 string），用于 `ssh.go` handleInput + `console.go` 串口 WS→stdin 两处。`input_parse_test.go` 7 tests（plain/CR/newline/tab/空/多字符+CR/拒绝对象）-race 绿。**SSH 终端 + 串口控制台两处同时修**（同一 bug）。

### 核验
`go build/vet/test -race ./...` rc=0 全绿。

---

## 控制台连接列表多 + 删除无反应（2026-07-02）

**根因**：OCI `ListInstanceConsoleConnections` 返回**含终态 DELETED 残留**的连接（OCI 异步清理慢），列表显示一堆 DELETED。点删除时：DELETED 连接再删 OCI 返回 404 → service 报错 500 → 前端 toast 一闪而过 + 刷新后 DELETED 还在（没过滤）→ 看似"没反应"。

**修**：
- `service.List`：过滤掉 `DELETED` 终态连接（残留噪声，不可再删）。
- `service.Delete`：404（已删除）当成功（`isOciNotFound` 用 errors.As 穿透 %w，仿 isConsoleConflict/isNotFound）+ 仍删 DB 行。
- 前端 `deleteConnection`：乐观移除行（即时反馈）+ 延时 3s 刷新（让 OCI 过渡到 DELETED 被过滤）。
- 测试：`List_JoinsOurs` 改断言 DELETED 被过滤；新增 `Delete_NotFoundIsSuccess` + `isOciNotFound`；-race 绿。

### 核验
`go build/vet/test -race ./...` rc=0 全绿；前端 typecheck 通过。
