# CLAUDE.md — oci-start-go

## 项目背景
oci-start 的 Go 重写（原 Java），模块 `github.com/Muione/oci-start-go`，Go 1.25。
单一二进制：Gin HTTP + WebSocket + robfig/cron 调度 + 内嵌 Vue3 SPA（build tag `dist` 切换 embed.FS）。
存储：SQLite（modernc.org/sqlite，纯 Go 无 cgo），WAL 模式，读/写双连接池。
多租户 OCI 实例管理：租户 OCI 凭据 AES-256-GCM 加密入库（master key），代用户调用 OCI API。

## 架构与模块（分层 `cmd → httpapi → service → repo/oci`）
- `cmd/oci-start/main.go` — 装配入口：配置→开库→迁移→bootstrap→装 Deps→HTTP+cron→优雅关闭
- `cmd/migrate/main.go` — 独立 CLI：把 Java H2 导出（.sql/.enc）导入 SQLite
- `internal/config` — viper 配置 + 内嵌 `default_config.yaml` 兜底，首次写 `./config.yaml`
- `internal/db` — `Store{Write,Read}` 双池 + `WithTx` 事务助手 + `Migrate`（golang-migrate）
- `internal/repo` — sqlc 生成（`sqlc generate`，配置 `sqlc.yaml`；queries 在 `internal/repo/queries/*.sql`）
- `internal/oci` — OCI SDK 封装：provider/clients/proxy池 + compute/network/storage/identity…
- `internal/service` — 业务逻辑层（tenant/instance/backup/email/billing…）
- `internal/httpapi` — Gin 路由+handler+中间件；`Deps` 结构体集中注入；`server.go` 注册路由
- `internal/auth` — session 服务 + `IpBan`/`SessionAuth`/`UserContext`/`TenantContext` 中间件
- `internal/grabber` — 抢购引擎（launch/pools/success/failure）
- `internal/ws` — WebSocket hub：ssh/console/rescue/monitor/log
- `internal/scheduler` — cron 定时（流量/巡检/证书/清理）
- `internal/{dns,acme,notify,migration,sysconf,response,web,bootstrap,util/*}` — 各见包注释

## 构建与常用命令
| 目的 | 命令 |
|---|---|
| 开发构建（stub SPA） | `go build -o oci-start ./cmd/oci-start` |
| 生产构建（含 SPA） | `cd frontend && npm ci && npm run build && cd .. && CGO_ENABLED=0 go build -tags dist -o oci-start ./cmd/oci-start` |
| 一键构建 | `./scripts/build.sh` |
| 环境准备 | `./scripts/setup.sh` |
| Docker | `docker build -t oci-start .` |
| 运行 | `go run ./cmd/oci-start` |
| 迁移 CLI | `go run ./cmd/migrate -db ./data/vps.db -file <export> [-key <master-key>]` |
| sqlc 重生成 | `sqlc generate` |
| vet / fmt | `go vet ./...` / `go fmt ./...` |
| 测试 | `go test ./...` |
| 竞态测试 | `go test -race ./...` |
| 单包测试 | `go test ./internal/ws/... -run TestX -race` |

注：`scripts/build.sh` 现用 `CGO_ENABLED=1`，但 modernc 为纯 Go，应改 `0`（已在优化清单 Q4）。

## 测试纪律（先写测试再改）
- 任何逻辑改动前先写/补测试；非平凡分支（解析、加密、并发、生命周期）必须有可运行检查。
- 并发改动用 `go test -race ./...` 验证。
- 修 bug 时先写一个会失败的测试复现，再改实现。
- 现状：172 源文件仅 12 测试文件，`repo`/`grabber`/`ws`/`auth`/`dns`/`acme`/`notify`/`migration` 零测试——补测是重点之一。

## 关键约束
- 不改公开 API 签名（导出函数/类型/方法）。
- 向后兼容：数据格式变更需读时降级迁移。
- SQLite-only，不引入新 DB 引擎。
- `data/master.key`（32 字节 AES）为全系统信任根，勿泄；勿打印密文/凭据到日志。

## 常用命令已加入 `.claude/settings.json` 的 `permissions.allow`，无需逐次确认。
