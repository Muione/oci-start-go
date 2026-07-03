# Fix IDCS 401 — MFA / Notification SDK Migration

**Status:** DONE

## Problem
`GetMfaStatus`, `ToggleEmailMFA`, `GetNotificationRecipients`, and
`UpdateNotificationRecipients` used the raw HTTP helper `doIdDomainCall`, which
signs requests with `common.DefaultRequestSigner` (OCI API-key signing). IDCS
admin SCIM endpoints (`/admin/v1/...`) reject OCI-signed requests with HTTP 401
— they require the token auth that the OCI SDK
`identitydomains.IdentityDomainsClient` provides internally.

## Root cause
Wrong auth mechanism for IDCS `/admin/v1` endpoints. The Java reference
(`SignOnPolicyUtils.initIdentityDomainsClient`) builds an
`IdentityDomainsClient` from the same API-key provider and it works; the Go
equivalent is `identitydomains.NewIdentityDomainsClientWithConfigurationProvider(prov, domainURL)`.

## What changed
File: `internal/oci/identity.go`

1. Added import `github.com/oracle/oci-go-sdk/v65/identitydomains`.
2. Added `getIdDomainsClient(ctx, prov, tenancyOCID)` helper (after `getDomainURL`)
   that builds an `IdentityDomainsClient` pointed at the tenant's domain URL.
3. Replaced `GetMfaStatus` → `client.GetAuthenticationFactorSetting`.
4. Replaced `ToggleEmailMFA` → GET → set `EmailEnabled` → PUT full object
   (PUT requires all mandatory fields, so the full object is sent, not just the
   changed field).
5. Replaced `GetNotificationRecipients` → `client.GetNotificationSetting`,
   reading `TestRecipients`.
6. Replaced `UpdateNotificationRecipients` → GET → set `TestModeEnabled` +
   `TestRecipients` → PUT full object.
7. Left `doIdDomainCall` in place — `GetPasswordPolicy` / `UpdatePasswordPolicy`
   still use it (separate fix if they also 401; not reported here).

## Cleanup (beyond the literal task)
- Removed four now-dead types that were only used by the replaced functions:
  `idDomainAuthFactorSetting`, `idDomainAuthFactorPutBody`,
  `idDomainNotificationSetting`, `notifPutBody`. The SDK provides its own
  equivalents; leaving them would be dead code.
- Updated the stale file-header comment that falsely claimed "The Go SDK v65
  does not include the identitydomains subpackage" — it does (v65.118.1), and
  the comment now accurately describes the split (MFA/notification via SDK
  client, password policy still via raw SCIM HTTP).

## SDK facts verified against the module cache
Package `github.com/oracle/oci-go-sdk/v65/identitydomains` (v65.118.1) confirmed
to contain:
- `NewIdentityDomainsClientWithConfigurationProvider(prov, endpoint string)`
- `GetAuthenticationFactorSetting` / `PutAuthenticationFactorSetting`
- `GetNotificationSetting` / `PutNotificationSetting`
- `AuthenticationFactorSetting` fields: `Schemas`, `SmsEnabled`, `TotpEnabled`,
  `PushEnabled`, `SecurityQuestionsEnabled`, `EmailEnabled`, `PhoneCallEnabled`,
  `FidoAuthenticatorEnabled` (all `*bool` except `Schemas []string`)
- `NotificationSetting` fields: `Schemas []string`, `NotificationEnabled *bool`,
  `TestModeEnabled *bool`, `TestRecipients []string`
- Request structs embed the settings by value (not pointer); response structs
  embed by value too — matches the task spec exactly.

## Commits
- `43fc4cb` — `fix(oci): use IdentityDomainsClient SDK for MFA/notification to fix IDCS 401`
  (1 file changed, +70 / -102)

## Build / test / vet results
- `go build ./cmd/oci-start` — clean, no errors.
- `go test ./internal/oci/... -v -count=1 -timeout 60s` — PASS
  (`internal/oci` ok 0.433s, `internal/oci/region` ok 0.007s).
- `go vet ./...` — clean, no output.

## Concerns
None blocking. Two notes:
- `doIdDomainCall` and its `httpClient` remain for password-policy use. If
  `GetPasswordPolicy` / `UpdatePasswordPolicy` also start returning 401, the
  same SDK-client migration applies (the SDK has a `PasswordPolicy` resource
  too) — out of scope for this fix, which only covered MFA + notifications as
  reported.
- Function signatures are unchanged, so all callers are unaffected.
