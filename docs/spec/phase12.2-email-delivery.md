# Phase 12.2 -- OCI Email Delivery Service Integration

## 1. Overview

This phase ports the Java Email Delivery integration to Go. The Java implementation
(`EmailController`, `EmailServiceImpl`, `OciEmailUtils`) provides a complete email
lifecycle: provisioning OCI email domains + senders + DKIM + SMTP credentials,
sending emails via SMTP, and tracking delivery results. The Go project already has
partial infrastructure (tenant_email_config CRUD, email_receive/email_body/
email_send_record tables), but lacks the OCI provisioning flow, SMTP sending,
and the full management API.

---

## 2. Database Schema (Already Exists)

All four tables already exist in `migrations/0001_init_schema.up.sql` and
`internal/repo/models.go`. No schema changes needed.

### 2.1 `tenant_email_config`

| Column              | Type   | Notes                                        |
|---------------------|--------|----------------------------------------------|
| id                  | BIGINT | PK auto-increment                           |
| tenant_id           | BIGINT | FK to tenant.id                             |
| domain_id           | TEXT   | OCI email domain OCID                       |
| domain_name         | TEXT   | e.g. "example.com"                          |
| sender_id           | TEXT   | OCI sender OCID                             |
| credential_id       | TEXT   | OCI SMTP credential OCID                    |
| smtp_username       | TEXT   | OCI SMTP username                           |
| smtp_password       | TEXT   | OCI SMTP password (plaintext in Java)       |
| smtp_host           | TEXT   | e.g. "smtp.email.ap-tokyo-1.oci.oraclecloud.com" |
| smtp_port           | TEXT   | "587"                                       |
| sender_email        | TEXT   | e.g. "noreply@example.com"                  |
| dkim_id             | TEXT   | OCI DKIM OCID                               |
| cname_record_value  | TEXT   | DKIM CNAME value from OCI                   |
| active              | INT    | 0=disabled, 1=enabled                       |
| created_time        | TEXT   | ISO-8601 timestamp                          |
| daily_email_limit   | BIGINT | Default 200                                 |
| today_sent_count    | BIGINT | Resets daily                                |
| last_reset_date     | TEXT   | ISO-8601 date                               |
| dbs_record_ids_str  | TEXT   | Comma-separated Cloudflare DNS record IDs   |

### 2.2 `email_receive`

| Column     | Type   | Notes                    |
|------------|--------|--------------------------|
| id         | BIGINT | PK auto-increment       |
| email      | TEXT   | NOT NULL, unique         |
| name       | TEXT   | NOT NULL                 |
| create_time| TEXT   |                          |
| update_time| TEXT   |                          |

### 2.3 `email_body`

| Column                  | Type   | Notes                          |
|-------------------------|--------|--------------------------------|
| id                      | BIGINT | PK auto-increment             |
| email_body_id           | TEXT   | NOT NULL UNIQUE (snowflake)   |
| current_version         | BIGINT | tenant_id (misnamed column)   |
| tenant_name             | TEXT   |                                |
| tenant_email_config_id  | BIGINT | FK to tenant_email_config.id  |
| sender_email            | TEXT   |                                |
| title                   | TEXT   | Email subject                 |
| content                 | TEXT   | Email body                    |
| receive_total           | BIGINT | Total recipients              |
| receive_success_total   | BIGINT | Successful sends              |
| receive_fail_total      | BIGINT | Failed sends                  |
| create_time             | TEXT   |                                |

### 2.4 `email_send_record`

| Column                 | Type   | Notes                          |
|------------------------|--------|--------------------------------|
| id                     | BIGINT | PK auto-increment             |
| email_send_record_id   | TEXT   | Snowflake ID                  |
| email_body_id          | TEXT   | FK to email_body.email_body_id|
| email_send_address     | TEXT   | Sender address                |
| current_version        | BIGINT | tenant_id (misnamed column)   |
| tenant_name            | TEXT   |                                |
| email_receive_id       | BIGINT | FK to email_receive.id        |
| receive_email_address  | TEXT   | Recipient address             |
| send_state             | INT    | 0=fail, 1=success             |
| create_time            | TEXT   |                                |

---

## 3. Existing Go Infrastructure

### 3.1 Already Implemented (Phase 9)

- **Routes** in `server.go`:
  - `GET  /tenants/:id/email`       -- tenantEmailGet
  - `POST /tenants/:id/email`       -- tenantEmailSave
  - `POST /tenants/:id/email/toggle`-- tenantEmailToggle
  - `DELETE /tenants/:id/email`     -- tenantEmailDelete

- **Service**: `internal/service/tenant_email.go` -- basic CRUD for tenant_email_config
  (Get, Save/Upsert, SetActive, Delete)

- **Repo**: `internal/repo/tenant_email_config.sql.go` -- sqlc-generated queries
  (FindTenantEmailConfigByTenantId, UpsertTenantEmailConfig, UpdateTenantEmailActive,
  DeleteTenantEmailConfig)

- **Models**: `internal/repo/models.go` -- Go structs for all four tables

- **SQL queries**: `internal/repo/queries/tenant_email_config.sql`

### 3.2 NOT Yet Implemented

- Email recipient (email_receive) CRUD service + repo + handlers
- Email body + send record repo queries
- OCI Email Domain provisioning (create/list/delete)
- OCI Sender provisioning (create/list/delete)
- OCI DKIM provisioning (create/list/delete)
- OCI SMTP credential generation/deletion
- SMTP email sending (parallel)
- Full "enable email for tenant" orchestration flow
- Full "disable email for tenant" orchestration flow (teardown OCI resources)
- Cloudflare DNS record creation for SPF/DKIM during enable
- Email body list / send record list / delete APIs
- Daily sent count tracking and limit enforcement

---

## 4. OCI SDK Operations Required

The Go OCI SDK (`github.com/oracle/oci-go-sdk/v65`) must be extended in
`internal/oci/provider.go` to include the Email and Identity (SMTP) clients.

### 4.1 New Client Additions to `oci.Clients`

```go
type Clients struct {
    // ... existing fields ...

    // Phase 12.2: Email Delivery
    Email    *email.EmailClient        // github.com/oracle/oci-go-sdk/v65/email
    Identity *identity.IdentityClient  // already exists, used for SMTP creds
}
```

The `email` package provides:
- `email.EmailClient` -- email domain, sender, DKIM management
- `identity.IdentityClient` -- SMTP credential management (already in Clients)

### 4.2 Email Domain Operations

| Operation              | OCI SDK Call                              | Java Method                    |
|------------------------|-------------------------------------------|--------------------------------|
| Create domain          | `EmailClient.CreateEmailDomain`           | `createEmailDomain`            |
| List domains           | `EmailClient.ListEmailDomains`            | `findEmailDomainByName`        |
| Get domain             | `EmailClient.GetEmailDomain`              | (inside findEmailDomainByName) |
| Delete domain          | `EmailClient.DeleteEmailDomain`           | `deleteEmailDomain`            |

**CreateEmailDomainDetails** fields:
- `CompartmentId` = tenant tenancy OCID
- `Name` = domain name (e.g. "example.com")

### 4.3 Sender Operations

| Operation              | OCI SDK Call                              | Java Method                    |
|------------------------|-------------------------------------------|--------------------------------|
| Create sender          | `EmailClient.CreateSender`                | `createSender`                 |
| List senders           | `EmailClient.ListSenders`                 | `findSenderByEmail`            |
| Get sender             | `EmailClient.GetSender`                   | (inside findSenderByEmail)     |
| Delete sender          | `EmailClient.DeleteSender`                | `deleteApprovedSender`         |

**CreateSenderDetails** fields:
- `CompartmentId` = tenant tenancy OCID
- `EmailAddress` = e.g. "noreply@example.com"

### 4.4 DKIM Operations

| Operation              | OCI SDK Call                              | Java Method                    |
|------------------------|-------------------------------------------|--------------------------------|
| Create DKIM            | `EmailClient.CreateDkim`                  | `createDkim`                   |
| List DKIMs             | `EmailClient.ListDkims`                   | `findDkimByDomainId`           |
| Get DKIM               | `EmailClient.GetDkim`                     | (inside findDkimByDomainId)    |
| Delete DKIM            | `EmailClient.DeleteDkim`                  | `deleteDkim`                   |

**CreateDkimDetails** fields:
- `EmailDomainId` = email domain OCID

### 4.5 SMTP Credential Operations (via IdentityClient, already exists)

| Operation              | OCI SDK Call                                      | Java Method                        |
|------------------------|---------------------------------------------------|------------------------------------|
| Create SMTP creds      | `IdentityClient.CreateSmtpCredential`             | `generateSmtpCredentials`          |
| Delete SMTP creds      | `IdentityClient.DeleteSmtpCredential`             | `deleteSmtpCredentials`            |

**CreateSmtpCredentialDetails** fields:
- `Description` = "Email credentials for tenant {id}"

The response contains `SmtpCredential` with `Username`, `Password`, `Id`.

### 4.6 SMTP Host Construction

```
smtpHost = "smtp.email." + region.CodeByName(tenant.Region) + ".oci.oraclecloud.com"
smtpPort = 587
```

The region code is already resolved by `internal/oci/region.CodeByName()`.

---

## 5. API Endpoints to Implement

### 5.1 Email Recipient Management

All under `/api/email/` prefix (new route group).

| Method | Path                   | Handler              | Description                  |
|--------|------------------------|----------------------|------------------------------|
| POST   | /api/email/receive/list    | emailReceiveList     | Paginated list, filter by email/name |
| POST   | /api/email/receive/add     | emailReceiveAdd      | Add recipient (email+name)   |
| POST   | /api/email/receive/delete  | emailReceiveDelete   | Delete by ID                 |
| POST   | /api/email/receive/get     | emailReceiveGet      | Get by ID                    |

### 5.2 Email Sending

| Method | Path                   | Handler              | Description                  |
|--------|------------------------|----------------------|------------------------------|
| POST   | /api/email/send            | emailSend            | Send email to recipients     |

### 5.3 Email Body (Batch) Management

| Method | Path                   | Handler              | Description                  |
|--------|------------------------|----------------------|------------------------------|
| POST   | /api/email/body/list       | emailBodyList        | Paginated email body list    |
| POST   | /api/email/body/delete     | emailBodyDelete      | Delete body + its records    |
| POST   | /api/email/body/batchDelete| emailBodyBatchDelete | Delete all bodies + records  |

### 5.4 Email Send Record Management

| Method | Path                   | Handler              | Description                  |
|--------|------------------------|----------------------|------------------------------|
| POST   | /api/email/send/list       | emailSendRecordList  | Paginated records, filter by emailBodyId |

### 5.5 Tenant Email Service Management

| Method | Path                   | Handler              | Description                  |
|--------|------------------------|----------------------|------------------------------|
| POST   | /api/email/tenant/list     | tenantEmailConfigList| List enabled tenants (paginated) |
| POST   | /api/email/tenant/get      | tenantEmailConfigGet | Get config by tenant ID      |
| POST   | /api/email/enable          | emailEnable          | Full OCI provisioning flow   |
| POST   | /api/email/disable         | emailDisable         | Full OCI teardown flow       |

---

## 6. Service Layer Design

### 6.1 New Package: `internal/service/email.go`

```go
type EmailService struct {
    store       *db.Store
    dnsSvc      *dns.DnsService
    cfCache     *dns.CfCache    // for Cloudflare DNS record creation
    masterKey   []byte
}
```

### 6.2 New Package: `internal/oci/email.go`

Encapsulates all OCI Email Delivery SDK calls, parallel to `oci/compute.go`, etc.

```go
type EmailOps struct{}

func (e *EmailOps) CreateEmailDomain(provider common.ConfigurationProvider, domainName string) (*EmailDomainResult, error)
func (e *EmailOps) FindEmailDomainByName(provider common.ConfigurationProvider, domainName string) (*EmailDomain, error)
func (e *EmailOps) DeleteEmailDomain(provider common.ConfigurationProvider, domainId string) error

func (e *EmailOps) CreateSender(provider common.ConfigurationProvider, emailAddress string) (*SenderResult, error)
func (e *EmailOps) FindSenderByEmail(provider common.ConfigurationProvider, email string) (*Sender, error)
func (e *EmailOps) DeleteSender(provider common.ConfigurationProvider, senderId string) error

func (e *EmailOps) CreateDkim(provider common.ConfigurationProvider, domainId string) (*DkimResult, error)
func (e *EmailOps) FindDkimByDomainId(provider common.ConfigurationProvider, domainId string) (*Dkim, error)
func (e *EmailOps) DeleteDkim(provider common.ConfigurationProvider, dkimId string) error

func (e *EmailOps) GenerateSmtpCredentials(provider common.ConfigurationProvider, description string) (*SmtpCredsResult, error)
func (e *EmailOps) DeleteSmtpCredentials(provider common.ConfigurationProvider, userId, credentialId string) error
```

### 6.3 SMTP Sending: `internal/service/email_send.go`

```go
type EmailSendResult struct {
    EmailSendRecordID string
    Email             string
    Success           bool
    Message           string
}

func (s *EmailService) SendEmailsParallel(
    smtpHost string, smtpPort int,
    smtpUsername, smtpPassword, senderEmail string,
    records []repo.EmailSendRecord,
    subject, content string,
) []EmailSendResult
```

Uses `net/smtp` + `crypto/tls` (STARTTLS on port 587). Goroutines with
`errgroup` for parallel sending (parity with Java CompletableFuture pattern).

---

## 7. Enable Email Flow (Orchestration)

`POST /api/email/enable` triggers the following sequence (port of
`EmailServiceImpl.enableEmailForTenant`):

1. **Validate Cloudflare config** -- system_config must have Cloudflare enabled
   (check `system_config` table for `cloudflare_api_token` key).

2. **Create OCI Email Domain** -- `EmailClient.CreateEmailDomain(compartmentId, domainName)`.
   If already exists, reuse existing domain ID.

3. **Create OCI Sender** -- `EmailClient.CreateSender(compartmentId, "noreply@"+domainName)`.
   If already exists, reuse.

4. **Create OCI DKIM** -- `EmailClient.CreateDkim(domainId)`.
   Returns `cnameRecordValue` for DNS CNAME record.

5. **Generate SMTP Credentials** -- `IdentityClient.CreateSmtpCredential(userId, description)`.
   Returns `username`, `password`, `credentialId`.

6. **Construct SMTP Host** -- `"smtp.email." + region.CodeByName(region) + ".oci.oraclecloud.com"`

7. **Create Cloudflare DNS Records**:
   - SPF TXT record: `v=spf1 include:emaildelivery.oraclecloud.com ~all` on `@`
   - DKIM CNAME record: parsed from `cnameRecordValue` (name + target)

8. **Save to tenant_email_config** -- upsert with all OCI resource IDs and SMTP details.

9. **Set tenant.email_enable = 1**.

### 7.1 Cloudflare DNS Record Creation

The Java `CloudflareService.addOracleEmailDnsRecords` creates:
- **SPF**: type=TXT, name=domain, content=`v=spf1 include:emaildelivery.oraclecloud.com ~all`, ttl=600
- **DKIM CNAME**: type=CNAME, name=parsed from cnameRecordValue, content=parsed target, ttl=600

The Go implementation should use `dns.CfCache.CreateRecord()` which already exists.
Store returned record IDs in `dbs_record_ids_str` (comma-separated).

### 7.2 cnameRecordValue Parsing

The OCI DKIM `cnameRecordValue` has the format:
```
<selector>._domainkey.<domain> <target>
```
Example: `abc123._domainkey.example.com abc123.dkim.emaildelivery.oraclecloud.com`

Parse into:
- name: `abc123._domainkey.example.com`
- target: `abc123.dkim.emaildelivery.oraclecloud.com`

---

## 8. Disable Email Flow (Teardown)

`POST /api/email/disable` triggers (port of `EmailServiceImpl.disableEmailForTenant`):

1. Look up tenant_email_config by ID.
2. Look up tenant by tenant_id.
3. **Delete OCI resources** (in order):
   - Delete DKIM: `EmailClient.DeleteDkim(dkimId)`
   - Delete SMTP credentials: `IdentityClient.DeleteSmtpCredential(userId, credentialId)`
   - Delete sender: `EmailClient.DeleteSender(senderId)`
   - Delete email domain: `EmailClient.DeleteEmailDomain(domainId)`
4. **Delete Cloudflare DNS records** -- iterate `dbs_record_ids_str`, call
   `CfCache.DeleteRecord()` for each.
5. **Set tenant.email_enable = 0**.
6. **Delete tenant_email_config row**.
7. **Cascade delete** email_body + email_send_record for this config.

---

## 9. Email Sending Flow

`POST /api/email/send` triggers (port of `EmailServiceImpl.send`):

### Request Body

```json
{
    "tenantEmailConfigId": 1,
    "title": "Subject Line",
    "content": "Email body text",
    "emailReceiveIds": [1, 2, 3]
}
```

### Flow

1. Load tenant_email_config by ID. Validate exists and active.
2. Load tenant by config's tenant_id.
3. Load email_receive rows by IDs. Validate non-empty.
4. Generate snowflake ID for emailBodyId.
5. Create email_body record with totals initialized to 0.
6. Create email_send_record rows (one per recipient, state=0).
7. Send emails in parallel via SMTP (goroutines).
8. Update each email_send_record with result (state 0 or 1).
9. Update email_body with success/fail totals.
10. Update tenant_email_config today_sent_count (reset if new day).

### SMTP Connection Details

- Host: from tenant_email_config.smtp_host
- Port: 587 (STARTTLS)
- Auth: PLAIN via smtp_username/smtp_password
- From: sender_email (e.g. "noreply@domain.com")
- Per-recipient: one message per goroutine (parity with Java parallel pattern)

### Daily Limit Enforcement

Before sending, check `today_sent_count < daily_email_limit`. If limit reached,
reject with error. Reset logic: if `last_reset_date < today`, reset count to 0.

---

## 10. File Structure

```
internal/
  oci/
    email.go              -- NEW: OCI Email SDK operations (domain/sender/dkim/smtp)
    provider.go           -- MODIFIED: add EmailClient to Clients struct
  service/
    email.go              -- NEW: EmailService (recipients, bodies, records, orchestration)
    email_send.go         -- NEW: SMTP sending logic
    tenant_email.go       -- EXISTING: keep as-is for basic config CRUD
  httpapi/
    email.go              -- NEW: all email-related handlers
    deps.go               -- MODIFIED: add EmailSvc *service.EmailService
    server.go             -- MODIFIED: add email route group
  repo/
    queries/
      email_receive.sql   -- NEW: sqlc queries for email_receive
      email_body.sql      -- NEW: sqlc queries for email_body
      email_send_record.sql -- NEW: sqlc queries for email_send_record
    email_receive.sql.go  -- NEW: generated
    email_body.sql.go     -- NEW: generated
    email_send_record.sql.go -- NEW: generated
```

---

## 11. Edge Cases

1. **Domain already exists in OCI**: `createEmailDomain` must check
   `ListEmailDomains` first and return existing domain ID if found. Same for
   sender and DKIM.

2. **Cloudflare not configured**: Enable flow must fail early with clear error
   if system_config has no Cloudflare credentials or Cloudflare is disabled.

3. **DNS record already exists**: SPF or DKIM CNAME may already exist in
   Cloudflare. Check before creating; skip if found (Java does this).

4. **SMTP credential leak**: The OCI SMTP password is shown only once at
   creation time. Store it securely in tenant_email_config. Consider
   encrypting it with masterKey (parity with tenant key_file_blob).

5. **Daily email limit**: The Java default is 200/day per tenant config.
   Enforce in send flow; reset count when `last_reset_date < today`.

6. **Partial failure in parallel send**: Some recipients succeed, some fail.
   Each record tracks its own state. Email body aggregates totals.

7. **Tenant deletion cascade**: When a tenant is deleted, must also clean up
   OCI email resources (domain, sender, DKIM, SMTP creds) and DNS records.
   Java does this in `EmailServiceImpl.deleteEmailConfig`.

8. **OCI API rate limiting**: Email domain/sender/DKIM creation may be
   rate-limited. Add retry with exponential backoff.

9. **SMTP connection timeout**: Java uses javax.mail with default timeouts.
   Go `net/smtp` needs explicit timeout handling (5s connect, 30s send).

10. **cnameRecordValue parsing**: The OCI DKIM CNAME value format must be
    parsed correctly to extract the DNS record name and target. Malformed
    values should be logged and the enable flow should fail gracefully.

11. **Idempotent enable**: If enable is called again for the same domain,
    each OCI resource check (domain, sender, DKIM) must be idempotent --
    find existing before creating.

12. **Disable with missing OCI resources**: If any OCI resource was already
    deleted externally, the disable flow should continue (log warning, skip
    that deletion step). Java does this with try-catch per step.

---

## 12. Go Implementation Guidance

### 12.1 OCI Email SDK Import

```go
import "github.com/oracle/oci-go-sdk/v65/email"
```

This provides: `email.EmailClient`, `email.CreateEmailDomainDetails`,
`email.CreateSenderDetails`, `email.CreateDkimDetails`, etc.

### 12.2 Provider Reuse

All OCI email operations use the same `common.ConfigurationProvider` built
by `oci.NewProvider(creds, masterKey)`. The compartment ID is the tenancy OCID
(`creds.Tenancy`). The user ID for SMTP credential operations is `creds.UserID`.

### 12.3 SMTP Sending with net/smtp

```go
import (
    "crypto/tls"
    "net"
    "net/smtp"
)

func sendOneEmail(host string, port int, username, password, from, to, subject, body string) error {
    addr := fmt.Sprintf("%s:%d", host, port)
    auth := smtp.PlainAuth("", username, password, host)
    // Build RFC 822 message
    msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n"+
        "MIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"UTF-8\"\r\n\r\n%s",
        from, to, subject, body))
    return smtp.SendMail(addr, auth, from, []string{to}, msg)
}
```

Note: Go's `net/smtp.SendMail` already handles STARTTLS automatically when
the server supports it (port 587).

### 12.4 Snowflake ID Generation

Java uses `IdUtil.getSnowflakeNextIdStr()` (Hutool). In Go, use
`github.com/bwmarrin/snowflake` or a simple timestamp-based ID generator.
The existing Go codebase may already have an ID generation utility.

### 12.5 Parallel Sending Pattern

```go
func (s *EmailService) SendEmailsParallel(host string, port int,
    user, pass, from string,
    records []repo.EmailSendRecord,
    subject, content string,
) []EmailSendResult {

    results := make([]EmailSendResult, len(records))
    var wg sync.WaitGroup
    for i, rec := range records {
        wg.Add(1)
        go func(idx int, r repo.EmailSendRecord) {
            defer wg.Done()
            err := sendOneEmail(host, port, user, pass, from,
                r.ReceiveEmailAddress.String, subject, content)
            results[idx] = EmailSendRecordResult{
                EmailSendRecordID: r.EmailSendRecordID.String,
                Email:             r.ReceiveEmailAddress.String,
                Success:           err == nil,
                Message:           errMsg(err),
            }
        }(i, rec)
    }
    wg.Wait()
    return results
}
```

### 12.6 Deps Wiring

Add to `deps.go`:

```go
type Deps struct {
    // ... existing ...
    EmailSvc *service.EmailService  // Phase 12.2
}
```

Wire in `main.go`:

```go
emailSvc := service.NewEmailService(store, dnsSvc, cfCache, masterKey)
deps.EmailSvc = emailSvc
```

### 12.7 Route Registration

Add to `server.go` in the protected group:

```go
// Phase 12.2: Email Delivery management.
pro.POST("/api/email/receive/list", emailReceiveList(deps))
pro.POST("/api/email/receive/add", emailReceiveAdd(deps))
pro.POST("/api/email/receive/delete", emailReceiveDelete(deps))
pro.POST("/api/email/receive/get", emailReceiveGet(deps))
pro.POST("/api/email/send", emailSend(deps))
pro.POST("/api/email/body/list", emailBodyList(deps))
pro.POST("/api/email/body/delete", emailBodyDelete(deps))
pro.POST("/api/email/body/batchDelete", emailBodyBatchDelete(deps))
pro.POST("/api/email/send/list", emailSendRecordList(deps))
pro.POST("/api/email/tenant/list", tenantEmailConfigList(deps))
pro.POST("/api/email/tenant/get", tenantEmailConfigGet(deps))
pro.POST("/api/email/enable", emailEnable(deps))
pro.POST("/api/email/disable", emailDisable(deps))
```

---

## 13. Security Considerations

1. **SMTP password storage**: Java stores plaintext. Go should encrypt with
   masterKey (AES-256-GCM, same as tenant key_file_blob) before storing in DB.
   Decrypt on use.

2. **SMTP credentials are OCI-generated**: The password is only visible at
   creation time. If lost, a new credential must be generated (old one deleted).

3. **Email content sanitization**: No HTML rendering in current implementation
   (plain text only). If HTML support is added later, sanitize to prevent XSS.

4. **Rate limiting**: Enforce daily_sent_count per tenant config. Consider
   a global rate limit to prevent abuse.

---

## 14. Testing Checklist

- [ ] Enable email for a tenant (full OCI provisioning + Cloudflare DNS)
- [ ] Disable email for a tenant (full OCI teardown + Cloudflare DNS cleanup)
- [ ] Idempotent re-enable (no duplicate OCI resources)
- [ ] Add/list/delete email recipients
- [ ] Send email to multiple recipients (parallel)
- [ ] Verify send records track per-recipient success/failure
- [ ] Verify email_body aggregates correct totals
- [ ] Verify today_sent_count increments and resets daily
- [ ] Verify daily limit enforcement rejects sends when exceeded
- [ ] Disable cleans up OCI resources even if some are already deleted
- [ ] Tenant deletion cascades to email config cleanup
