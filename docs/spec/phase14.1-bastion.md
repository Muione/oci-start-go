# Phase 14.1 -- OCI Bastion Service Integration

## 1. Overview

This phase ports the Java Bastion integration to Go. The Java implementation
consists of a single utility class (`BastionSshSessionUtils`) that creates
port-forwarding SSH sessions through the OCI Bastion service. The Java code
is a skeleton/work-in-progress -- it creates a session with hardcoded OCIDs
and prints the session ID, but has no controller, service layer, or DTOs.

The Go implementation will provide a complete Bastion management API:
listing bastions, creating managed SSH and port-forwarding sessions,
retrieving session details, and deleting sessions.

---

## 2. Database Schema

No new tables are required. Bastion sessions are ephemeral OCI resources and
do not need local persistence. If session tracking is desired in the future,
a `bastion_sessions` table can be added.

---

## 3. Existing Go Infrastructure

### 3.1 Already Exists

- **Provider pattern**: `internal/oci/provider.go` -- `Clients` struct,
  `NewClients(prov)`, `NewClientsWithHTTPClient(prov, hc)`
- **Proxy pattern**: `internal/oci/proxy.go` -- `WithProxy(ctx, pool, creds, masterKey, fn)`
- **Route registration**: `internal/httpapi/server.go` -- protected route groups
  with `auth.SessionAuth`, `auth.UserContext`, `auth.TenantContext`

### 3.2 NOT Yet Implemented

- `internal/oci/bastion.go` -- OCI Bastion SDK wrapper
- `internal/service/bastion.go` -- Bastion service layer
- `internal/httpapi/bastion.go` -- HTTP handlers
- `go.mod` dependency: `github.com/oracle/oci-go-sdk/v65/bastion`

---

## 4. OCI SDK Operations Required

The Go OCI SDK package `github.com/oracle/oci-go-sdk/v65/bastion` provides
the `BastionClient` with the following operations.

### 4.1 New Client Addition to `oci.Clients`

```go
type Clients struct {
    // ... existing fields ...

    // Phase 14.1: Bastion
    Bastion *bastion.BastionClient  // github.com/oracle/oci-go-sdk/v65/bastion
}
```

### 4.2 Bastion Operations

| Operation         | OCI SDK Call                          | Java Method / Reference             |
|-------------------|---------------------------------------|-------------------------------------|
| List bastions     | `BastionClient.ListBastions`          | (not in Java -- new)                |
| Get bastion       | `BastionClient.GetBastion`            | (not in Java -- new)                |
| Create session    | `BastionClient.CreateSession`         | `BastionSshSessionUtils.executeCommand` |
| Get session       | `BastionClient.GetSession`            | (not in Java -- new)                |
| List sessions     | `BastionClient.ListSessions`          | (not in Java -- new)                |
| Delete session    | `BastionClient.DeleteSession`         | (not in Java -- new)                |
| Get session (SSH) | `BastionClient.GetSession` + command  | (not in Java -- new)                |

### 4.3 CreateSession Model Details

The Java code creates a **port-forwarding session**:

```java
CreatePortForwardingSessionTargetResourceDetails targetResourceDetails =
    CreatePortForwardingSessionTargetResourceDetails.builder()
        .targetResourceId("ocid1.instance.oc1..xxxxx")
        .targetResourcePrivateIpAddress("10.0.0.23")
        .targetResourcePort(22)
        .build();

CreateSessionDetails build = CreateSessionDetails.builder()
    .targetResourceDetails(targetResourceDetails)
    .bastionId("ocid1.bastion.oc1.xxxx")
    .build();
```

The Go SDK equivalents:

**CreatePortForwardingSessionTargetResourceDetails** fields:
- `TargetResourcePrivateIpAddress` (string) -- private IP of the target
- `TargetResourcePort` (int) -- port on the target (e.g. 22)
- `TargetResourceId` (string) -- optional instance OCID

**CreateManagedSshSessionTargetResourceDetails** fields:
- `TargetResourceDetails` with `TargetResourceOperatingSystemUserName`
- `TargetResourceId` (string) -- instance OCID

**CreateSessionDetails** fields:
- `BastionId` (string) -- bastion OCID
- `TargetResourceDetails` -- one of the above target types
- `SessionTtlInSeconds` (int) -- session TTL, default 1800 (30 min)
- `DisplayName` (string) -- optional friendly name
- `KeyDetails` -- public key for SSH sessions (`PublicKeyContent`)

### 4.4 Session Lifecycle States

| State             | Meaning                          |
|-------------------|----------------------------------|
| CREATING          | Session is being provisioned     |
| ACTIVE            | Session is ready to use          |
| DELETING          | Session is being torn down       |
| DELETED           | Session is gone                  |
| FAILED            | Session creation failed          |
| NEEDS_ATTENTION   | Requires manual intervention     |

---

## 5. Go API Design

### 5.1 Routes

All routes are protected (SessionAuth + UserContext + TenantContext).

```
GET    /api/v1/tenants/:id/bastions                              -- List bastions
GET    /api/v1/tenants/:id/bastions/:bastionId                   -- Get bastion details
POST   /api/v1/tenants/:id/bastions/:bastionId/sessions          -- Create session
GET    /api/v1/tenants/:id/bastions/:bastionId/sessions          -- List sessions
GET    /api/v1/tenants/:id/bastions/:bastionId/sessions/:sessionId -- Get session
DELETE /api/v1/tenants/:id/bastions/:bastionId/sessions/:sessionId -- Delete session
```

### 5.2 Request/Response DTOs

**ListBastions Response:**
```json
{
  "bastions": [
    {
      "id": "ocid1.bastion.oc1...",
      "name": "my-bastion",
      "bastionType": "STANDARD",
      "lifecycleState": "ACTIVE",
      "compartmentId": "ocid1.compartment.oc1...",
      "targetVcnId": "ocid1.vcn.oc1...",
      "maxSessionTtlInSeconds": 10800,
      "timeCreated": "2025-01-01T00:00:00Z",
      "timeUpdated": "2025-01-01T00:00:00Z"
    }
  ]
}
```

**CreateSession Request (Port Forwarding):**
```json
{
  "bastionId": "ocid1.bastion.oc1...",
  "sessionType": "PORT_FORWARDING",
  "displayName": "ssh-to-web-server",
  "targetResourcePrivateIpAddress": "10.0.0.23",
  "targetResourcePort": 22,
  "targetResourceId": "ocid1.instance.oc1...",
  "sessionTtlInSeconds": 1800,
  "keyDetails": {
    "publicKeyContent": "ssh-rsa AAAA..."
  }
}
```

**CreateSession Request (Managed SSH):**
```json
{
  "bastionId": "ocid1.bastion.oc1...",
  "sessionType": "MANAGED_SSH",
  "displayName": "ssh-session",
  "targetResourceId": "ocid1.instance.oc1...",
  "targetResourceOperatingSystemUserName": "opc",
  "sessionTtlInSeconds": 1800,
  "keyDetails": {
    "publicKeyContent": "ssh-rsa AAAA..."
  }
}
```

**Session Response:**
```json
{
  "id": "ocid1.bastionsession.oc1...",
  "bastionId": "ocid1.bastion.oc1...",
  "sessionType": "PORT_FORWARDING",
  "displayName": "ssh-to-web-server",
  "lifecycleState": "ACTIVE",
  "targetResourceDetails": {
    "targetResourcePrivateIpAddress": "10.0.0.23",
    "targetResourcePort": 22
  },
  "sshMetadata": {
    "command": "ssh -o ProxyCommand=\"ssh -i key -p 22 ocid1.bastionsession.oc1...@host.bastion.oc1.oraclecloud.com\" -i key -p 22 opc@10.0.0.23"
  },
  "timeCreated": "2025-01-01T00:00:00Z",
  "timeUpdated": "2025-01-01T00:00:00Z"
}
```

---

## 6. File Structure

```
internal/
  oci/
    bastion.go          # OCI Bastion SDK wrapper functions
  service/
    bastion.go          # Bastion service layer
  httpapi/
    bastion.go          # HTTP handlers for bastion endpoints
```

---

## 7. Implementation Notes

### 7.1 `oci/bastion.go` Functions

```
ListBastions(ctx, client *bastion.BastionClient, compartmentID string) ([]BastionSummary, error)
GetBastion(ctx, client *bastion.BastionClient, bastionID string) (*Bastion, error)
CreateSession(ctx, client *bastion.BastionClient, req CreateSessionRequest) (*Session, error)
GetSession(ctx, client *bastion.BastionClient, sessionID string) (*Session, error)
ListSessions(ctx, client *bastion.BastionClient, bastionID string) ([]SessionSummary, error)
DeleteSession(ctx, client *bastion.BastionClient, sessionID string) error
```

### 7.2 Proxy Integration

Bastion operations should go through the existing `WithProxy` decorator:

```go
WithProxy(ctx, pool, creds, masterKey, func(c Clients) error {
    sessions, err := oci.ListSessions(ctx, c.Bastion, bastionID)
    // ...
})
```

### 7.3 Parity with Java

| Java Class / Method                  | Go Equivalent                    |
|--------------------------------------|----------------------------------|
| `BastionSshSessionUtils.executeCommand` | `oci.CreateSession` + `oci.GetSession` |
| (no controller)                      | `httpapi.BastionCreateSession`   |
| (no controller)                      | `httpapi.BastionListBastions`    |
| (no controller)                      | `httpapi.BastionListSessions`    |
| (no controller)                      | `httpapi.BastionDeleteSession`   |

### 7.4 Key Considerations

- **Session TTL**: Default 1800 seconds (30 min). Max is bastion's
  `maxSessionTtlInSeconds`. The handler should clamp user-provided TTL.
- **SSH Command Generation**: The `sshMetadata` field in the session response
  contains the proxy command. The Go handler should extract and return this
  for the frontend to display.
- **Session States**: Only `ACTIVE` sessions can be used. The frontend should
  poll `GetSession` until state transitions from `CREATING` to `ACTIVE`.
- **Public Key**: Required for SSH sessions. The frontend can generate a
  keypair client-side or the user can paste their existing public key.
- **Pagination**: `ListBastions` and `ListSessions` support `Limit` and
  `Page` parameters. Use standard pagination loop with `OpcNextPage`.

### 7.5 Error Handling

- 404 on GetSession/DeleteSession: session was already deleted or expired
- 409 on CreateSession: conflict (duplicate session for same target)
- 400 on CreateSession: invalid bastion ID or target configuration
- Wrap all errors with `fmt.Errorf("bastion: list: %w", err)` pattern
