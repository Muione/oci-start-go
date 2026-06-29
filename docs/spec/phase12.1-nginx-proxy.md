# Phase 12.1: Nginx / Reverse Proxy Configuration -- API SPEC

## Overview

This phase ports the Nginx (OpenResty) reverse proxy management feature from the Java
project to Go. It covers:

- **ProxyConfig CRUD** -- per-domain reverse proxy rules (target host/port, SSL, WebSocket,
  rate limiting, caching, custom config blocks).
- **NginxConfig generation** -- server-block template engine that compiles all active
  ProxyConfig rows into a single `nginx.conf`-style string, versioned in the
  `nginx_config` table.
- **Config test / apply / reload** -- syntax-check via OpenResty API, push config to
  OpenResty, reload, and rollback on failure.
- **SSL certificate lifecycle** -- request (Let's Encrypt via ACME), bind to proxy config,
  auto-renew scheduled job, download as ZIP.
- **OpenResty service management** -- check installed/running/API-available, start service.
- **Config diff** -- LCS-based line-level diff between current and latest config versions.

All proxy config and nginx endpoints are protected (SessionAuth + UserContext + TenantContext).

## Database

Tables `proxy_config`, `nginx_config`, and `ssl_certificate` already exist in the Go
project's init migration (`migrations/0001_init_schema.up.sql`) and are registered in
`internal/repo/models.go`. No schema changes are needed.

### Table: `proxy_config`

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| id | INTEGER PK | no | Auto-increment |
| domain | TEXT UNIQUE NOT NULL | no | Domain name (e.g. `app.example.com`) |
| target_host | TEXT NOT NULL | no | Backend host |
| target_port | INTEGER NOT NULL | no | Backend port |
| protocol | TEXT | yes | `http` or `https` (default `http`) |
| enable_ssl | INTEGER | yes | 0/1 boolean |
| enable_websocket | INTEGER | yes | 0/1 boolean |
| ssl_certificate_id | BIGINT | yes | FK to `ssl_certificate.id` |
| config_status | TEXT | yes | `PENDING` / `APPLIED` / `ERROR` / `DISABLED` |
| ssl_status | TEXT | yes | `NOT_CONFIGURED` / `CONFIGURED` / `PENDING` / `ERROR` |
| custom_config | TEXT | yes | Raw nginx snippet injected into the `server {}` block |
| remark | TEXT | yes | User-facing note |
| load_balance_type | TEXT | yes | `round_robin` / `ip_hash` / `least_conn` |
| enable_health_check | INTEGER | yes | 0/1 |
| health_check_path | TEXT | yes | e.g. `/health` |
| health_check_interval | INTEGER | yes | Seconds |
| enable_rate_limit | INTEGER | yes | 0/1 |
| rate_limit | INTEGER | yes | Requests/sec |
| enable_cache | INTEGER | yes | 0/1 |
| cache_time | INTEGER | yes | Cache TTL in seconds |
| create_time | TEXT | yes | ISO-8601 |
| update_time | TEXT | yes | ISO-8601 |

### Table: `nginx_config`

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| id | INTEGER PK | no | Auto-increment |
| config_name | TEXT | yes | `auto-generated-{timestamp}` |
| config_content | TEXT | yes | Full nginx server-block content |
| is_current | INTEGER | yes | 0/1 -- marks the version currently applied to OpenResty |
| config_version | INTEGER | yes | Monotonically incrementing version number |
| config_status | TEXT | yes | `DRAFT` / `TESTING` / `APPLIED` / `ERROR` |
| create_time | TEXT | yes | ISO-8601 |
| update_time | TEXT | yes | ISO-8601 |

### Table: `ssl_certificate`

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| id | INTEGER PK | no | Auto-increment |
| domain | TEXT NOT NULL | no | Domain the cert covers |
| certificate_type | TEXT NOT NULL | no | `LETS_ENCRYPT` / `CLOUDFLARE` |
| email | TEXT | yes | Contact email (required for Let's Encrypt) |
| validation_method | TEXT | yes | `DNS` / `HTTP` |
| auto_renew | INTEGER | yes | 0/1 |
| certificate_status | TEXT | yes | `PENDING` / `VALID` / `EXPIRING_SOON` / `EXPIRED` / `ERROR` |
| issue_date | TEXT | yes | ISO-8601 |
| expire_date | TEXT | yes | ISO-8601 |
| certificate_path | TEXT | yes | Filesystem path to `fullchain.pem` |
| private_key_path | TEXT | yes | Filesystem path to `privkey.pem` |
| create_time | TEXT | yes | ISO-8601 |
| update_time | TEXT | yes | ISO-8601 |
| dns_provider | TEXT | yes | `CLOUDFLARE` / `ALIYUN` / etc. |

## API Endpoints

All routes are mounted under the protected group in `server.go`.

### 1. Proxy Config CRUD

#### 1.1 Create Proxy Config

```
POST /ssl/proxy/create
```

**Request Body:**

```json
{
  "domain": "app.example.com",
  "targetHost": "10.0.0.5",
  "targetPort": 3000,
  "protocol": "http",
  "enableSsl": false,
  "enableWebSocket": false,
  "sslCertificateId": null,
  "customConfig": "",
  "remark": "Production frontend",
  "loadBalanceType": "round_robin",
  "enableHealthCheck": false,
  "healthCheckPath": "/health",
  "healthCheckInterval": 30,
  "enableRateLimit": false,
  "rateLimit": 100,
  "enableCache": false,
  "cacheTime": 300
}
```

**Validation:**
- `domain` required, must be unique (check `EXISTS` before insert).
- `targetHost` and `targetPort` required.

**Side effects:**
1. Insert row into `proxy_config` with `config_status=PENDING`, `ssl_status=NOT_CONFIGURED`.
2. If Cloudflare DNS is configured, attempt to create a simple A record for `domain`
   (best-effort; log warning on failure, do not roll back the DB insert).
3. Call `generateNginxConfig()` to produce a new `nginx_config` draft.

**Response (200):**

```json
{ "success": true, "message": "Proxy config created" }
```

**Java reference:** `NginxController.createProxy()` (line 80-94),
`ProxyConfigServiceImpl.createProxyConfig()` (line 70-93).

---

#### 1.2 Update Proxy Config

```
PUT /ssl/proxy/{id}
```

**Request Body:** Same as create, plus `id` from path.

**Side effects:**
1. If `domain` changed and new domain already exists, return 400 error.
2. If domain changed: reset `ssl_certificate_id=NULL`, `enable_ssl=0`,
   `ssl_status=NOT_CONFIGURED` (old cert no longer matches new domain).
3. Set `config_status=PENDING`.
4. Call `generateNginxConfig()`.

**Java reference:** `NginxController.updateProxy()` (line 99-114),
`ProxyConfigServiceImpl.updateProxyConfig()` (line 97-118).

---

#### 1.3 Get Proxy Config Detail

```
GET /ssl/proxy/{id}
```

**Response:** Single `proxy_config` row as JSON object.

**Java reference:** `NginxController.getProxyDetail()` (line 119-129).

---

#### 1.4 Delete Proxy Config

```
DELETE /ssl/proxy/{id}
```

**Side effects:**
1. Delete row from `proxy_config`.
2. Call `generateNginxConfig()`.

**Java reference:** `NginxController.deleteProxy()` (line 134-148).

---

#### 1.5 List Proxy Configs (Paginated)

```
GET /ssl/proxy/list?page=0&size=20
```

**Response:** Paginated list ordered by `create_time DESC`.

**Java reference:** `NginxController.getProxyList()` (line 153-165).

---

#### 1.6 Batch Delete Proxy Configs

```
DELETE /ssl/proxy/batch
```

**Request Body:** JSON array of IDs: `[1, 2, 3]`.

**Side effects:** Delete each, then single `generateNginxConfig()` call.

**Java reference:** `NginxController.batchDeleteProxy()` (line 170-184).

---

#### 1.7 Toggle Proxy Config (Enable/Disable)

```
PUT /ssl/proxy/{id}/toggle
```

**Request Body:**

```json
{ "enabled": true }
```

**Logic:**
- `enabled=true` -> set `config_status=PENDING`
- `enabled=false` -> set `config_status=DISABLED`
- Then `generateNginxConfig()`.

**Java reference:** `NginxController.toggleProxyConfig()` (line 189-201),
`ProxyConfigServiceImpl.toggleProxyConfig()` (line 214-224).

---

#### 1.8 Test Proxy Connection

```
POST /ssl/proxy/{id}/test-connection
```

**Logic:** TCP dial to `targetHost:targetPort` with 5s timeout. Any TCP-level
response (including RST) counts as "reachable". This avoids false negatives on
API backends returning 4xx/5xx.

**Response:**

```json
{ "success": true, "data": { "connected": true } }
```

**Java reference:** `ProxyConfigServiceImpl.testConnection()` (line 227-243).

---

#### 1.9 Apply SSL to Proxy Config

```
POST /ssl/proxy/{id}/ssl?email=user@example.com
```

**Logic:**
1. Look up `proxy_config` by `id`.
2. Create `SslCertificateRequestDto` with domain and email.
3. Call `sslCertificateService.requestCertificate()`.
4. On success: set `ssl_certificate_id`, `ssl_status=PENDING`, `enable_ssl=1`.
5. On failure: set `ssl_status=ERROR`.

**Java reference:** `NginxController.applySslConfig()` (line 225-235),
`ProxyConfigServiceImpl.applySslConfig()` (line 140-163).

---

#### 1.10 Fix Proxy Config

```
POST /ssl/proxy/{id}/fix
```

**Logic:** Reset `config_status=PENDING` (allows re-generation after an error state).

**Java reference:** `NginxController.fixProxyConfig()` (line 240-250).

---

### 2. SSL Certificate Management

#### 2.1 Request Certificate

```
POST /ssl/certificates/request
```

**Request Body:**

```json
{
  "domain": "app.example.com",
  "email": "admin@example.com",
  "certificateType": "LETS_ENCRYPT",
  "dnsProvider": "CLOUDFLARE",
  "validationMethod": "dns",
  "autoRenew": true
}
```

**Logic:**
1. Check for existing valid/pending cert for this domain; return existing if found.
2. Acquire per-domain lock (map of `sync.Mutex` keyed by domain string).
3. Insert `ssl_certificate` row with `status=PENDING`.
4. Launch async goroutine for ACME flow:
   a. Create ACME session (Let's Encrypt production or staging based on `config.Ssl.Staging`).
   b. Generate/load account key pair (persisted at `{base_path}/account.key`).
   c. Create or retrieve ACME account.
   d. Create certificate order for the domain.
   e. Process DNS-01 challenge via Cloudflare API:
      - Add `_acme-challenge.{domain}` TXT record.
      - Wait for DNS propagation (poll 8.8.8.8 / 1.1.1.1 via `net.Resolver`, up to 24
        attempts at 15s intervals).
      - Trigger challenge, wait for completion (up to 5 min).
      - Clean up TXT record in `finally`.
   f. Generate domain key pair (RSA 2048), create CSR, submit.
   g. Wait for order completion.
   h. Save `fullchain.pem` and `privkey.pem` to `{base_path}/cert/{domain}/`.
   i. Update `ssl_certificate` with paths, `status=VALID`, real expiry date parsed from X.509.
   j. Upload cert to OpenResty via API (`POST /api/ssl/certs`).
   k. Sync `proxy_config.ssl_status` for all proxies referencing this cert.
5. On any failure: set `status=ERROR`, sync proxy statuses.

**Go implementation:** Use `github.com/go-acme/lego/v4` (already in `internal/acme/manager.go`)
instead of acme4j. The existing `CertManager.ObtainCertificate()` covers the lego setup;
this phase extends it with DNS propagation waiting, file persistence, and OpenResty upload.

**Per-domain locking:** Use `sync.Map` of `*sync.Mutex` keyed by domain string, with
a cleanup function to delete stale entries after unlock.

**Java reference:** `SslCertificateServiceImpl.requestCertificate()` (line 114-137),
`doRequestCertificate()` (line 139-170), `processAcme4jRequestAsync()` (line 192-228).

---

#### 2.2 Renew Certificate

```
POST /ssl/certificates/{id}/renew
```

**Logic:** Same as request but re-uses existing record. Set `status=PENDING`, launch
async ACME flow. Only supports `LETS_ENCRYPT` type.

**Java reference:** `SslCertificateServiceImpl.renewCertificate()` (line 533-556).

---

#### 2.3 Delete Certificate

```
DELETE /ssl/certificates/{id}
```

**Logic:**
1. Check for referencing `proxy_config` rows; reject if any exist (list the domains in
   the error message).
2. If `LETS_ENCRYPT` + `VALID`: attempt ACME revocation (best-effort).
3. Delete cert files from disk.
4. Delete DB row.

**Java reference:** `SslCertificateServiceImpl.deleteCertificate()` (line 558-600).

---

#### 2.4 Toggle Auto-Renew

```
PUT /ssl/certificates/{id}/auto-renew
```

**Request Body:**

```json
{ "enabled": true }
```

**Java reference:** `SslCertificateServiceImpl.toggleAutoRenew()` (line 674-684).

---

#### 2.5 List Certificates (Paginated)

```
GET /ssl/certificates/list?page=0&size=20
```

**Response:** Paginated list ordered by `create_time DESC`.

---

#### 2.6 Check Expiring Certificates

```
GET /ssl/certificates/expiring
```

**Logic:** Return all certs with `expire_date <= now + 30 days` AND `auto_renew=1`.

**Java reference:** `SslCertificateServiceImpl.checkExpiringCertificates()` (line 692-695).

---

#### 2.7 Download Certificate (ZIP)

```
GET /ssl/certificates/{id}/download
```

**Response:** `application/zip` containing:
- `fullchain.pem`
- `privkey.pem`
- `README.txt` (metadata + usage instructions)

**Java reference:** `SslCertificateServiceImpl.downloadCertificate()` (line 746-788).

---

#### 2.8 Match Certificates by Domain

```
GET /ssl/certificates/match?domain=api.example.com
```

**Logic:** Find all `VALID` certificates whose domain matches the query via:
1. Exact match.
2. Wildcard match: `*.example.com` matches `api.example.com` (single-level subdomain only).
3. Multi-domain cert: split on comma, match each.

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "name": "*.example.com",
      "domain": "*.example.com",
      "certPath": "/data/cert/example.com/fullchain.pem",
      "keyPath": "/data/cert/example.com/privkey.pem",
      "expiryDate": "2026-12-01T00:00:00"
    }
  ]
}
```

**Java reference:** `SslCertificateServiceImpl.findCertificatesByDomain()` (line 840-867),
`isDomainMatch()` (line 880-935).

---

### 3. Nginx Config Management

#### 3.1 Generate Nginx Config

```
POST /ssl/nginx/generate
```

**Logic:**
1. Query all `proxy_config` rows where `config_status != 'DISABLED'`.
2. For each config, generate an nginx `server {}` block via the template engine
   (see Section 5 below).
3. If SSL is enabled and the referenced `ssl_certificate_id` points to a deleted cert,
   skip that domain with a warning log.
4. Concatenate all server blocks.
5. Compare content with the latest `nginx_config` row; if identical, return the existing
   version (no-op).
6. Otherwise insert a new `nginx_config` row: `config_status=DRAFT`,
   `config_version = latest + 1`.

**Response:**

```json
{ "success": true, "data": { "id": 5, "configVersion": 3 } }
```

**Java reference:** `NginxConfigServiceImpl.generateNginxConfig()` (line 85-119).

---

#### 3.2 Apply Nginx Config

```
POST /ssl/nginx/{id}/apply
```

**Logic (with global `sync.Mutex` to prevent concurrent applies, 30s timeout):**
1. Load the `nginx_config` row by `id`.
2. Load the current `is_current=1` row (may be null on first apply).
3. **Step 1 -- Test:** Call OpenResty API `POST /api/config/test` with config content.
   If test fails, abort.
4. **Step 2 -- Push:** Call OpenResty API `PUT /api/config` with `{ "content": "..." }`.
5. **Step 3 -- Reload:** Call OpenResty API `POST /api/config/reload`.
6. **Step 4 -- DB update:** Set old current row `is_current=0`, new row `is_current=1`
   and `config_status=APPLIED`.
7. **Rollback on failure:** If step 2 succeeded but step 3 failed, push the old current
   config back to OpenResty and reload again. Log critical error if rollback also fails.

**Important:** Steps 3-6 are NOT wrapped in a single DB transaction. The OpenResty API
calls are external side effects that cannot be rolled back by a DB transaction abort.
The DB update (step 6) uses a separate transaction.

**Java reference:** `NginxConfigServiceImpl.applyConfig()` (line 217-272).

---

#### 3.3 Test Nginx Config

```
POST /ssl/nginx/{id}/test
```

**Logic:** Call OpenResty API `POST /api/config/test` with `{ "content": "..." }`.
Return success/failure based on HTTP status code. Read-only -- does not modify DB state.

**Java reference:** `NginxConfigServiceImpl.testConfig()` (line 306-326).

---

#### 3.4 Reload Nginx

```
POST /ssl/nginx/reload
```

**Logic:** Call OpenResty API `POST /api/config/reload`.

**Java reference:** `NginxConfigServiceImpl.reloadNginx()` (line 329-337).

---

#### 3.5 Get Config Diff

```
GET /ssl/nginx/diff
```

**Logic:**
1. Load `is_current=1` row (current) and latest by `config_version DESC`.
2. If latest is null: return "No generated config".
3. If current is null: return "First config, nothing applied yet".
4. If same ID: return "Config is up to date".
5. Otherwise: compute line-level LCS diff, return lines prefixed with `  ` (unchanged),
   `- ` (removed), `+ ` (added).

**Response:**

```json
{
  "success": true,
  "data": {
    "current": { "id": 3, "configVersion": 2 },
    "latest": { "id": 5, "configVersion": 3 },
    "diff": "  server {\n-     listen 80;\n+     listen 443 ssl http2;\n  }"
  }
}
```

**Java reference:** `NginxConfigServiceImpl.getConfigDiff()` (line 340-350),
`buildLineDiff()` (line 353-374).

---

#### 3.6 Get Nginx Status

```
GET /ssl/nginx/status
```

**Response:**

```json
{
  "success": true,
  "data": {
    "hasChanges": true,
    "currentVersion": 2,
    "latestVersion": 3
  }
}
```

**Java reference:** `NginxController.getNginxStatus()` (line 438-455).

---

#### 3.7 Get Latest Config

```
GET /ssl/nginx/latest
```

**Response:** Full `nginx_config` row (latest by version).

---

### 4. OpenResty Service Management

#### 4.1 Check OpenResty Status

```
GET /ssl/openresty/status
```

**Logic:**
1. Run `openresty -v` to check if installed (exit code 0 = installed).
2. Run `pgrep -f openresty` to check if running.
3. If running, call OpenResty API `GET /api/test` to check API availability.

**Response:**

```json
{
  "success": true,
  "data": {
    "installed": true,
    "running": true,
    "apiAvailable": true
  }
}
```

**Java reference:** `NginxConfigServiceImpl.checkOpenRestyStatus()` (line 377-394).

---

#### 4.2 Start OpenResty

```
POST /ssl/openresty/start
```

**Logic:**
1. Run `/usr/local/openresty/bin/openresty`.
2. Poll OpenResty API `GET /api/test` up to 10 times at 1s intervals.
3. Return success when API responds 2xx.

**Java reference:** `NginxConfigServiceImpl.startOpenRestyService()` (line 397-416).

---

## 5. Nginx Config Template Engine

The `generateServerBlock()` function produces an nginx `server {}` block for each
`ProxyConfig`. The template logic:

### HTTP-only config

```nginx
server {
    listen 80;
    server_name {domain};

    location / {
        proxy_pass {protocol}://{targetHost}:{targetPort};
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### SSL config (when `enable_ssl=1`)

Generates TWO server blocks:

**Block 1 -- HTTP redirect:**

```nginx
server {
    listen 80;
    server_name {domain};
    return 301 https://$server_name$request_uri;
}
```

**Block 2 -- HTTPS:**

```nginx
server {
    listen 443 ssl http2;
    server_name {domain};

    ssl_certificate /usr/local/openresty/nginx/ssl/{domain}/fullchain.pem;
    ssl_certificate_key /usr/local/openresty/nginx/ssl/{domain}/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    location / {
        proxy_pass {protocol}://{targetHost}:{targetPort};
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Optional directives (appended inside the `location /` block or server block)

| Condition | Directive |
|-----------|-----------|
| `enable_websocket=1` | `proxy_http_version 1.1;` + `proxy_set_header Upgrade $http_upgrade;` + `proxy_set_header Connection "upgrade";` |
| `enable_rate_limit=1` | `limit_req zone={domain_sanitized}_limit burst={rate*2} nodelay;` |
| `enable_cache=1` | `proxy_cache my_cache;` + `proxy_cache_valid 200 {cache_time}s;` + `proxy_cache_key $scheme$proxy_host$request_uri;` |
| `enable_health_check=1` | Comment: `# Health check endpoint: {healthCheckPath}` |
| `customConfig != ""` | Injected as indented raw text after the location block |

**Java reference:** `NginxConfigServiceImpl.generateServerBlock()` (line 121-204).

---

## 6. OpenResty API Client

The Java project communicates with an OpenResty management API (a sidecar service
running on the same host). The Go implementation needs an HTTP client for these endpoints:

| Method | Path | Body | Description |
|--------|------|------|-------------|
| GET | `/api/test` | -- | Health check |
| POST | `/api/config/test` | `{ "content": "..." }` | Syntax test |
| PUT | `/api/config` | `{ "content": "..." }` | Push config |
| POST | `/api/config/reload` | `{}` | Reload OpenResty |
| POST | `/api/ssl/certs` | `{ "domain", "cert", "key", "force_replace" }` | Upload SSL cert |
| GET | `/api/ssl/certs` | -- | List certs |
| GET | `/api/ssl/certs/{domain}` | -- | Get cert by domain |
| DELETE | `/api/ssl/certs/{domain}` | -- | Delete cert |

**Configuration** (already in `config.yaml`):

```yaml
openresty:
  api:
    base_url: http://127.0.0.1:8080/api
```

**Auth:** If configured, requests include `X-API-Token` header.

**Go implementation:** Create `internal/openresty/client.go` with a `Client` struct
wrapping `net/http.Client` with configurable base URL, token, and timeout.

---

## 7. Scheduled Jobs

### 7.1 SSL Auto-Renewal

A daily scheduled job (via `internal/scheduler`) that:
1. Queries all certs with `auto_renew=1`, `certificate_type='LETS_ENCRYPT'`,
   `expire_date <= now + 7 days`.
2. For each, attempts renewal (per-domain lock prevents duplicate concurrent runs).

**Java reference:** `SslCertJob` (Quartz `@DisallowConcurrentExecution`),
`SslCertificateServiceImpl.processAutoRenewal()` (line 698-728).

---

## 8. Go Implementation Guidance

### 8.1 New Files to Create

| Path | Purpose |
|------|---------|
| `internal/repo/queries/proxy_config.sql` | sqlc queries for proxy_config CRUD |
| `internal/repo/queries/nginx_config.sql` | sqlc queries for nginx_config CRUD |
| `internal/repo/queries/ssl_certificate.sql` | sqlc queries for ssl_certificate CRUD |
| `internal/openresty/client.go` | OpenResty management API HTTP client |
| `internal/service/nginx.go` | ProxyConfigService, NginxConfigService, SslCertificateService |
| `internal/httpapi/nginx.go` | HTTP handlers for all /ssl/* endpoints |
| `internal/service/nginx_template.go` | Server-block template engine |

### 8.2 sqlc Query Files

Create `proxy_config.sql`, `nginx_config.sql`, `ssl_certificate.sql` in
`internal/repo/queries/` with the following queries:

**proxy_config.sql:**
- `InsertProxyConfig` -- INSERT with all fields
- `UpdateProxyConfig` -- UPDATE all fields except id, create_time
- `DeleteProxyConfig` -- DELETE by id
- `FindProxyConfigById` -- SELECT by id
- `ListProxyConfigs` -- SELECT with LIMIT/OFFSET, ORDER BY create_time DESC
- `CountProxyConfigs` -- SELECT COUNT(*)
- `ListActiveProxyConfigs` -- SELECT WHERE config_status != 'DISABLED'
- `ExistsProxyConfigByDomain` -- SELECT COUNT(*) WHERE domain = ?
- `UpdateProxyConfigStatus` -- UPDATE config_status by id
- `UpdateProxyConfigSslFields` -- UPDATE ssl_certificate_id, enable_ssl, ssl_status by id
- `FindProxyConfigsBySslCertId` -- SELECT WHERE ssl_certificate_id = ?

**nginx_config.sql:**
- `InsertNginxConfig` -- INSERT
- `FindLatestNginxConfig` -- SELECT ORDER BY config_version DESC LIMIT 1
- `FindCurrentNginxConfig` -- SELECT WHERE is_current = 1
- `FindNginxConfigById` -- SELECT by id
- `MarkNginxConfigCurrent` -- UPDATE is_current=1, config_status='APPLIED' by id
- `ClearCurrentNginxConfig` -- UPDATE is_current=0 WHERE is_current=1

**ssl_certificate.sql:**
- `InsertSslCertificate` -- INSERT
- `UpdateSslCertificate` -- UPDATE mutable fields by id
- `DeleteSslCertificate` -- DELETE by id
- `FindSslCertificateById` -- SELECT by id
- `FindSslCertificateByDomain` -- SELECT WHERE domain = ? LIMIT 1
- `ListSslCertificates` -- SELECT with LIMIT/OFFSET, ORDER BY create_time DESC
- `CountSslCertificates` -- SELECT COUNT(*)
- `ListExpiringCertificates` -- SELECT WHERE expire_date <= ? AND auto_renew = 1
- `FindAllActiveSslCertificates` -- SELECT WHERE certificate_status = 'VALID'
- `ExistsSslCertificateById` -- SELECT COUNT(*) WHERE id = ?
- `ExistsProxyConfigBySslCertId` -- SELECT COUNT(*) FROM proxy_config WHERE ssl_certificate_id = ?

### 8.3 Service Layer

`internal/service/nginx.go` should contain:

```go
type NginxService struct {
    repo       *repo.Queries
    db         *sql.DB
    openresty  *openresty.Client
    acme       *acme.CertManager
    dns        *dns.DnsService
    cfg        *config.Config
    logger     zerolog.Logger
    applyLock  sync.Mutex
    domainLock sync.Map // map[string]*sync.Mutex
}
```

Methods:
- `CreateProxyConfig(ctx, dto) error`
- `UpdateProxyConfig(ctx, id, dto) error`
- `DeleteProxyConfig(ctx, id) error`
- `GetProxyConfig(ctx, id) (*ProxyConfig, error)`
- `ListProxyConfigs(ctx, page, size) ([]ProxyConfig, int64, error)`
- `BatchDeleteProxyConfigs(ctx, ids []int64) error`
- `ToggleProxyConfig(ctx, id int64, enabled bool) error`
- `TestProxyConnection(ctx, id int64) (bool, error)`
- `ApplySslToProxy(ctx, id int64, email string) error`
- `FixProxyConfig(ctx, id int64) error`
- `GenerateNginxConfig(ctx) (*NginxConfig, error)`
- `ApplyNginxConfig(ctx, id int64) error`
- `TestNginxConfig(ctx, id int64) (bool, error)`
- `ReloadNginx(ctx) error`
- `GetConfigDiff(ctx) (string, error)`
- `GetNginxStatus(ctx) (map[string]interface{}, error)`
- `GetLatestNginxConfig(ctx) (*NginxConfig, error)`
- `CheckOpenRestyStatus(ctx) (map[string]interface{}, error)`
- `StartOpenResty(ctx) error`
- `RequestCertificate(ctx, dto) (*SslCertificate, error)`
- `RenewCertificate(ctx, id int64) error`
- `DeleteCertificate(ctx, id int64) error`
- `ToggleAutoRenew(ctx, id int64, enabled bool) error`
- `ListCertificates(ctx, page, size) ([]SslCertificate, int64, error)`
- `CheckExpiringCertificates(ctx) ([]SslCertificate, error)`
- `DownloadCertificate(ctx, id int64) ([]byte, string, error)`
- `MatchCertificatesByDomain(ctx, domain string) ([]CertificateDTO, error)`
- `ProcessAutoRenewal(ctx) error`
- `SyncProxySslStatusByCertificate(ctx, certId int64) error`
- `UploadSslToOpenResty(ctx, cert *SslCertificate, reloadAfter bool) error`

### 8.4 HTTP Handlers

`internal/httpapi/nginx.go` -- handler functions that parse request, call service,
return `ApiResponse`. Each handler follows the same pattern as existing handlers in
`server.go`.

### 8.5 Route Registration

Add to the protected route group in `server.go`:

```go
// Phase 12.1: Nginx / Reverse Proxy management.
pro.POST("/ssl/proxy/create", nginxCreateProxy(deps))
pro.PUT("/ssl/proxy/:id", nginxUpdateProxy(deps))
pro.GET("/ssl/proxy/:id", nginxGetProxy(deps))
pro.DELETE("/ssl/proxy/:id", nginxDeleteProxy(deps))
pro.GET("/ssl/proxy/list", nginxListProxies(deps))
pro.DELETE("/ssl/proxy/batch", nginxBatchDeleteProxies(deps))
pro.PUT("/ssl/proxy/:id/toggle", nginxToggleProxy(deps))
pro.POST("/ssl/proxy/:id/test-connection", nginxTestProxyConnection(deps))
pro.POST("/ssl/proxy/:id/ssl", nginxApplySsl(deps))
pro.POST("/ssl/proxy/:id/fix", nginxFixProxy(deps))
pro.POST("/ssl/certificates/request", nginxRequestCert(deps))
pro.POST("/ssl/certificates/:id/renew", nginxRenewCert(deps))
pro.DELETE("/ssl/certificates/:id", nginxDeleteCert(deps))
pro.PUT("/ssl/certificates/:id/auto-renew", nginxToggleAutoRenew(deps))
pro.GET("/ssl/certificates/list", nginxListCerts(deps))
pro.GET("/ssl/certificates/expiring", nginxExpiringCerts(deps))
pro.GET("/ssl/certificates/:id/download", nginxDownloadCert(deps))
pro.GET("/ssl/certificates/match", nginxMatchCerts(deps))
pro.POST("/ssl/nginx/generate", nginxGenerateConfig(deps))
pro.POST("/ssl/nginx/:id/apply", nginxApplyConfig(deps))
pro.POST("/ssl/nginx/:id/test", nginxTestConfig(deps))
pro.POST("/ssl/nginx/reload", nginxReload(deps))
pro.GET("/ssl/nginx/diff", nginxConfigDiff(deps))
pro.GET("/ssl/nginx/status", nginxStatus(deps))
pro.GET("/ssl/nginx/latest", nginxLatestConfig(deps))
pro.GET("/ssl/openresty/status", openrestyStatus(deps))
pro.POST("/ssl/openresty/start", openrestyStart(deps))
```

### 8.6 Deps Additions

Add to `Deps` struct in `deps.go`:

```go
NginxSvc *service.NginxService
```

Wire in `main.go` with the openresty client, repo, acme manager, dns service, and config.

---

## 9. Edge Cases and Error Handling

| Scenario | Behavior |
|----------|----------|
| Domain already exists on create | Return 400: "domain already exists" |
| SSL enabled but no certificate_id | Throw during config generation; skip domain with warning |
| Referenced certificate deleted | Skip domain in config generation; log warning |
| Domain changed on update | Reset ssl_certificate_id, enable_ssl, ssl_status |
| Certificate deletion while referenced by proxy | Reject with list of referencing domains |
| Concurrent apply requests | Global mutex with 30s timeout; reject if locked |
| Apply fails after OpenResty push | Rollback: push previous config, reload |
| OpenResty not installed/running | Return status with `installed=false`/`running=false` |
| DNS propagation timeout | Warn and continue ACME flow (may fail on first attempt) |
| Per-domain cert lock contention | `tryLock` -- reject immediately with "already in progress" |
| Config content unchanged on generate | Return existing version (no new row) |
| TCP connection test to unreachable host | Return `connected=false` (not an error) |
| Batch delete with mixed valid/invalid IDs | Delete what exists; log warnings for missing |
| Auto-renew with expired lock | Skip domain, log info |
| Empty custom_config | Skip injection |
| SSL cert path in OpenResty differs from local | OpenResty upload copies cert to its managed path |

---

## 10. Security Considerations

- All endpoints require session authentication (protected route group).
- The OpenResty API token (`X-API-Token`) should be stored in config, never exposed
  to the frontend.
- Certificate private keys are stored on disk with filesystem permissions (0600).
- The ACME account key persists at `{base_path}/account.key`; protect this file.
- Domain lock map should have a cleanup mechanism to prevent unbounded memory growth
  (evict entries older than 1 hour using `sync.Map.Range` + timestamp check).
