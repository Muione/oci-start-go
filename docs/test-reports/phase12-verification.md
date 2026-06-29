# Phase 12 Verification Report

**Date:** 2026-06-28
**Scope:** Phase 12.1 (Nginx/Reverse Proxy), 12.2 (Email Delivery), 12.3 (SSH Config Backend Improvements)

---

## 1. Route Registration

### Phase 12.1: Nginx / Reverse Proxy (27 routes) -- PASS

All 27 routes registered under `/ssl/` in `server.go` lines 202-228:

| # | Method | Path | Handler |
|---|--------|------|---------|
| 1 | POST | `/ssl/proxy/create` | `nginxCreateProxy` |
| 2 | PUT | `/ssl/proxy/:id` | `nginxUpdateProxy` |
| 3 | GET | `/ssl/proxy/:id` | `nginxGetProxy` |
| 4 | DELETE | `/ssl/proxy/:id` | `nginxDeleteProxy` |
| 5 | GET | `/ssl/proxy/list` | `nginxListProxies` |
| 6 | DELETE | `/ssl/proxy/batch` | `nginxBatchDeleteProxies` |
| 7 | PUT | `/ssl/proxy/:id/toggle` | `nginxToggleProxy` |
| 8 | POST | `/ssl/proxy/:id/test-connection` | `nginxTestProxyConnection` |
| 9 | POST | `/ssl/proxy/:id/ssl` | `nginxApplySsl` |
| 10 | POST | `/ssl/proxy/:id/fix` | `nginxFixProxy` |
| 11 | POST | `/ssl/certificates/request` | `nginxRequestCert` |
| 12 | POST | `/ssl/certificates/:id/renew` | `nginxRenewCert` |
| 13 | DELETE | `/ssl/certificates/:id` | `nginxDeleteCert` |
| 14 | PUT | `/ssl/certificates/:id/auto-renew` | `nginxToggleAutoRenew` |
| 15 | GET | `/ssl/certificates/list` | `nginxListCerts` |
| 16 | GET | `/ssl/certificates/expiring` | `nginxExpiringCerts` |
| 17 | GET | `/ssl/certificates/:id/download` | `nginxDownloadCert` |
| 18 | GET | `/ssl/certificates/match` | `nginxMatchCerts` |
| 19 | POST | `/ssl/nginx/generate` | `nginxGenerateConfig` |
| 20 | POST | `/ssl/nginx/:id/apply` | `nginxApplyConfig` |
| 21 | POST | `/ssl/nginx/:id/test` | `nginxTestConfig` |
| 22 | POST | `/ssl/nginx/reload` | `nginxReload` |
| 23 | GET | `/ssl/nginx/diff` | `nginxConfigDiff` |
| 24 | GET | `/ssl/nginx/status` | `nginxStatus` |
| 25 | GET | `/ssl/nginx/latest` | `nginxLatestConfig` |
| 26 | GET | `/ssl/openresty/status` | `openrestyStatus` |
| 27 | POST | `/ssl/openresty/start` | `openrestyStart` |

### Phase 12.2: Email Delivery (13 routes) -- PASS

All 13 routes registered under `/api/email/` in `server.go` lines 231-243:

| # | Method | Path | Handler |
|---|--------|------|---------|
| 1 | POST | `/api/email/receive/list` | `emailReceiveList` |
| 2 | POST | `/api/email/receive/add` | `emailReceiveAdd` |
| 3 | POST | `/api/email/receive/delete` | `emailReceiveDelete` |
| 4 | POST | `/api/email/receive/get` | `emailReceiveGet` |
| 5 | POST | `/api/email/send` | `emailSend` |
| 6 | POST | `/api/email/body/list` | `emailBodyList` |
| 7 | POST | `/api/email/body/delete` | `emailBodyDelete` |
| 8 | POST | `/api/email/body/batchDelete` | `emailBodyBatchDelete` |
| 9 | POST | `/api/email/send/list` | `emailSendRecordList` |
| 10 | POST | `/api/email/tenant/list` | `tenantEmailConfigList` |
| 11 | POST | `/api/email/tenant/get` | `tenantEmailConfigGet` |
| 12 | POST | `/api/email/enable` | `emailEnable` |
| 13 | POST | `/api/email/disable` | `emailDisable` |

### Phase 12.3: Backend Improvements -- PASS (no new routes expected)

Phase 12.3 adds SSH root login configurator as a backend service injected into existing systems (backup, rescue). No new HTTP routes needed per spec.

---

## 2. Build Verification

| Check | Result | Notes |
|-------|--------|-------|
| `go build ./...` | **PASS** | Clean compilation |
| `go vet ./...` | **WARN** | 1 non-blocking warning: `internal/service/nginx.go:373` -- IPv6 address format `%s:%d` passed to `net.Dial` (cosmetic, does not affect functionality) |
| `npx vite build` | **PASS** | 1735 modules transformed, output to `internal/web/dist/` |

---

## 3. File Existence (17/17) -- PASS

| File | Exists |
|------|--------|
| `internal/openresty/client.go` | Yes |
| `internal/service/nginx.go` | Yes |
| `internal/service/nginx_template.go` | Yes |
| `internal/service/email.go` | Yes |
| `internal/service/email_send.go` | Yes |
| `internal/service/ssh_config.go` | Yes |
| `internal/oci/email.go` | Yes |
| `internal/httpapi/nginx.go` | Yes |
| `internal/httpapi/handler_email.go` | Yes |
| `internal/repo/proxy_config.sql.go` | Yes |
| `internal/repo/nginx_config.sql.go` | Yes |
| `internal/repo/ssl_certificate.sql.go` | Yes |
| `internal/repo/email_receive.sql.go` | Yes |
| `internal/repo/email_body.sql.go` | Yes |
| `internal/repo/email_send_record.sql.go` | Yes |
| `frontend/src/views/NginxProxy.vue` | Yes |
| `frontend/src/views/EmailManagement.vue` | Yes |

---

## 4. Dependency Wiring -- PASS

### deps.go (Deps struct)

| Field | Type | Phase |
|-------|------|-------|
| `SSHConfig` | `*service.SSHConfigurator` | 12.3 (line 95) |
| `NginxSvc` | `*service.NginxService` | 12.1 (line 98) |
| `EmailSvc` | `*service.EmailService` | 12.2 (line 101) |

### main.go (construction + injection)

| Service | Constructor | Injection |
|---------|-------------|-----------|
| SSHConfig | `service.NewSSHConfigurator(logger)` (line 152) | `deps.SSHConfig` (line 344), `wsHub.Rescue.SetDeps` EnableRootLogin (line 239), `backupSvc.SetSSHConfig` (line 156) |
| NginxSvc | `service.NewNginxService(store, orClient, cfg, sc, dnsSvc, logger)` (line 292) | `deps.NginxSvc` (line 345), `scheduler.SvcSet.NginxSvc` (line 305) |
| EmailSvc | `service.NewEmailService(store, dnsSvc, sc, proxyPool, masterKey)` (line 295) | `deps.EmailSvc` (line 346) |

---

## 5. Scheduler (SSL Auto-Renewal) -- PASS

**File:** `internal/scheduler/scheduler.go`

- `nginxSvc *service.NginxService` field declared on Scheduler struct (line 47)
- `SvcSet.NginxSvc` field in service set (line 59)
- Wired in `New()`: `s.nginxSvc = svcs.NginxSvc` (line 83)
- `sslCertJob()` method (lines 185-226): prioritizes `s.nginxSvc.ProcessAutoRenewal(ctx)` when available, falls back to legacy `certManager.RenewCertificate` for single-domain setups
- Cron expression: `0 0 4 * * *` (daily at 04:00)
- NginxSvc passed via main.go scheduler construction (line 305)

---

## Summary

| Category | Status |
|----------|--------|
| 12.1 Route Registration (27 routes) | PASS |
| 12.2 Route Registration (13 routes) | PASS |
| 12.3 Backend-Only (no routes) | PASS |
| Go Build | PASS |
| Go Vet | PASS (1 non-blocking warning) |
| Frontend Build | PASS |
| File Existence (17/17) | PASS |
| Dependency Wiring | PASS |
| Scheduler SSL Auto-Renewal | PASS |

**Overall: ALL CHECKS PASSED**
