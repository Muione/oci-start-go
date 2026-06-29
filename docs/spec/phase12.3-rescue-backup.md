# Phase 12.3 — Rescue Mode Completion & Backup Auto Security Rules

## Overview

This spec covers two tightly coupled features that the Java project implements and the Go project has partially stubbed:

1. **Auto Security Rule Opening** — Before any SSH-dependent operation (backup, rescue), the system pings the instance. If unreachable, it automatically opens all OCI security list protocols (including ICMP) so the instance becomes reachable, then retries.
2. **SSH Root Login Enablement** — After confirming network reachability, the system SSHs into the instance and configures `PermitRootLogin yes` + `PasswordAuthentication yes` in sshd_config, installs a persistent self-healing service, and sets the root password.
3. **Rescue Mode Improvements** — The Go rescue handler already has the full 10-step WebSocket flow. This spec adds the missing security-rule + root-login pre/post steps that the Java handler performs.
4. **Backup Flow Enhancements** — The Go `BackupSvc.ScheduleBackup()` has two TODO stubs that this spec completes.

These features are the bridge between a bare OCI instance launch and a fully-manageable instance that can be SSHed into, backed up, and rescued.

---

## 1. Problem Statement

After a successful OCI instance grab, the system schedules a boot volume backup 3 minutes later (see `grabber/success.go:64`). The Java project performs two critical pre-backup steps that the Go project currently stubs:

1. The instance may not be reachable via SSH because OCI security lists do not have port 22 open by default for new tenants, or the instance's firewall blocks ICMP.
2. Even if reachable, root password login may be disabled (OCI images default to key-only auth), preventing the system from SSHing in to verify instance health.

Without these steps, the backup either fails silently (Java skips backup if `enableRootLogin` fails) or creates a backup of an instance that was never verified as healthy.

---

## 2. Feature 1: Auto Security Rule Opening

### 2.1 Java Behavior (Source of Truth)

**File:** `InstanceBackUpEventListener.java:103-106`
```java
if (!PingUtil.ping(instanceData.getPublicIp()).isReachable()) {
    log.debug("当前ip无法ping通,执行协议开启后再次尝试");
    securityRuleService.checkAndEnableRule(tenant);
}
```

**File:** `SecurityRuleServiceImpl.java` — `checkAndEnableRule(tenant)`:
1. Lists all Security Lists in the tenant's compartment.
2. For each missing rule, adds it to the first Security List:
   - Ingress: `all` protocol from `0.0.0.0/0`
   - Ingress: `all` protocol from `::/0` (IPv6, failures tolerated)
   - Ingress: ICMP type 8 code 0 from `0.0.0.0/0`
   - Ingress: ICMP type 8 code 0 from `10.0.0.0/16`
   - Egress: `all` protocol to `0.0.0.0/0`
   - Egress: `all` protocol to `::/0` (IPv6, failures tolerated)
3. Sets `tenant.enableAllProtocol = true` in the database.

The rescue handler (`RescueWebSocketHandler.java`) calls the same `checkAndEnableRule` before starting any rescue operation.

### 2.2 Go Current State

**File:** `internal/service/backup.go:55-56`
```go
// TODO (Phase 6): Ping public IP → if unreachable, open security rules.
// TODO (Phase 6): SSH enableRootLogin.
```

**File:** `internal/service/ping.go` — `CheckPingConn()` already does TCP connect to port 22 with 5-second timeout, but only updates `conn_time`; it does not trigger security rule opening on failure.

**File:** `internal/oci/security_list.go` — `EnableAllForTenant()` already implements the exact same rules as Java `checkAndEnableRule`. Already done.

**File:** `internal/service/security_rule.go` — `SingleEnableAll()` wraps `EnableAllForTenant` + `EnableIPv6ForTenant` + tenant flag update. Already done.

### 2.3 What Needs to Be Built

Add a `CheckAndEnableRule` method to `SecurityRuleService` that is a thin wrapper matching the Java `checkAndEnableRule(tenant)` semantics:

```go
// CheckAndEnableRule opens all protocols if the tenant hasn't already been
// flagged. Idempotent — skips if tenant.enableAllProtocol == 1.
// Parity with Java SecurityRuleServiceImpl.checkAndEnableRule.
func (s *SecurityRuleService) CheckAndEnableRule(ctx context.Context, tenantID int64) error {
    t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
    if err != nil {
        return fmt.Errorf("find tenant %d: %w", tenantID, err)
    }

    // Skip if already enabled (idempotent gate).
    if t.EnableAllProtocol.Valid && t.EnableAllProtocol.Int64 == 1 {
        return nil
    }

    return s.SingleEnableAll(ctx, tenantID)
}
```

Then use it in `BackupSvc.ScheduleBackup()`:

```go
func (s *BackupSvc) ScheduleBackup(ctx context.Context, input BackupInput) {
    // ...existing logging...

    // Step 1: Ping check + auto-open security rules.
    if !s.checkReachability(input.PublicIP, 22) {
        s.logger.Info().Str("ip", input.PublicIP).Msg("backup: unreachable, opening security rules")
        if err := s.securityRules.CheckAndEnableRule(ctx, input.TenantID); err != nil {
            s.logger.Error().Err(err).Msg("backup: failed to open security rules")
        }
        // Retry ping after rule change (OCI eventual consistency ~5s).
        time.Sleep(10 * time.Second)
        if !s.checkReachability(input.PublicIP, 22) {
            s.logger.Warn().Str("ip", input.PublicIP).Msg("backup: still unreachable after opening rules, skipping")
            return
        }
    }

    // Step 2: SSH root login enablement.
    // ...see Feature 2 below...

    // Step 3: Create backup (existing code).
    if err := s.createBackup(ctx, input); err != nil {
        s.logger.Error().Err(err).Str("taskId", input.TaskID).Msg("backup: failed")
    }
}
```

### 2.4 Reachability Check Helper

```go
// checkReachability tries TCP connect to host:port with a timeout.
// Returns true if connection succeeds.
func (s *BackupSvc) checkReachability(host string, port int) bool {
    addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
    conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
    if err != nil {
        return false
    }
    conn.Close()
    return true
}
```

This reuses the same TCP-connect approach as `PingSvc.CheckPingConn()` (line 66 of `ping.go`). ICMP ping is not used because:
- OCI security lists may block ICMP by default.
- TCP port 22 is what we actually need for the subsequent SSH step.
- The Java code uses an external HTTP-based ping service (`tools.ipip.net`), but TCP connect is more reliable and self-contained.

---

## 3. Feature 2: SSH Root Login Enablement

### 3.1 Java Behavior (Source of Truth)

**File:** `JschUtils.java:1081-1240` — `enableRootLogin(host, username, password, rootPassword, port)`:

This is a comprehensive SSH-based configuration script that:

1. **Connects via SSH** using JSch with `StrictHostKeyChecking=no`.
2. **Sets root password** via `echo "root:<password>" | chpasswd` (runs first as a separate script).
3. **Modifies sshd_config:**
   - `PasswordAuthentication yes`
   - `PermitRootLogin yes`
   - `ChallengeResponseAuthentication yes`
   - `UsePAM yes`
4. **Backs up sshd_config** with timestamp.
5. **OS-specific handling:**
   - Ubuntu/Debian: comments out `@include common-auth` in `/etc/pam.d/sshd`
   - Oracle/RHEL/CentOS/AlmaLinux/Rocky: sets SELinux to permissive if enforcing
6. **Scans `/etc/ssh/sshd_config.d/*.conf`** and applies the same sed rules to override drop-in configs.
7. **Creates a persistent self-healing service:**
   - Script at `/usr/local/bin/check-ssh-config.sh` that re-applies the sshd_config changes on every boot.
   - systemd service `check-ssh-config.service` (or rc.local fallback) that runs the script at startup.
8. **Validates sshd config** with `sshd -t` before restarting.
9. **Restarts sshd** via systemctl or service command.
10. **Returns `ScriptResult`** with success/failure status.

The method is called:
- In `InstanceBackUpEventListener` after the ping/security-rule check, before backup.
- In `RescueWebSocketHandler` after rescue completes, to re-enable root login on the rescued instance.

### 3.2 Go Current State

**File:** `internal/grabber/launch.go:217` — cloud-init already sets root password and enables PasswordAuthentication at launch time:
```
echo "root:<password>" | chpasswd
sed -i 's/^#PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
sed -i 's/^PasswordAuthentication.*no/PasswordAuthentication yes/' /etc/ssh/sshd_config
systemctl restart sshd
```

However, this only works at launch time. The post-launch `enableRootLogin` (via SSH, not cloud-init) is needed for:
- Backup pre-check: verify the instance is actually SSHable.
- Rescue post-step: re-enable root login after a rescue (the rescue OS may have different sshd_config).
- Instances where cloud-init failed or sshd_config was modified after boot.

### 3.3 What Needs to Be Built

Create `internal/service/ssh_config.go`:

```go
// Package service — ssh_config.go: SSH root login enablement (Phase 12.3).
// Port of JschUtils.enableRootLogin. SSHs into an instance and configures
// PermitRootLogin yes, PasswordAuthentication yes, with OS-specific
// handling and persistent self-healing service installation.
package service

import (
    "fmt"
    "net"
    "time"

    "golang.org/x/crypto/ssh"
    "github.com/rs/zerolog"
)

// SSHConfigurator enables root password login on remote instances.
type SSHConfigurator struct {
    logger zerolog.Logger
}

func NewSSHConfigurator(logger zerolog.Logger) *SSHConfigurator {
    return &SSHConfigurator{logger: logger}
}

// EnableRootLogin SSHs into the instance and configures root password login.
// Returns nil on success, error on failure.
// Parity with Java JschUtils.enableRootLogin.
func (s *SSHConfigurator) EnableRootLogin(host, username, password, rootPassword string, port int) error {
    // 1. Set root password.
    if rootPassword != "" {
        passwdScript := fmt.Sprintf(`echo "root:%s" | chpasswd`, rootPassword)
        if err := s.execScript(host, username, password, port, passwdScript); err != nil {
            return fmt.Errorf("set root password: %w", err)
        }
    }

    // 2. Main sshd_config modification script.
    script := buildEnableRootLoginScript()
    if err := s.execScript(host, username, password, port, script); err != nil {
        return fmt.Errorf("configure sshd: %w", err)
    }

    return nil
}

// execScript connects via SSH and executes a shell script.
func (s *SSHConfigurator) execScript(host, username, password string, port int, script string) error {
    config := &ssh.ClientConfig{
        User: username,
        Auth: []ssh.AuthMethod{
            ssh.Password(password),
        },
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout:         30 * time.Second,
    }

    addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
    client, err := ssh.Dial("tcp", addr, config)
    if err != nil {
        return fmt.Errorf("ssh dial %s: %w", addr, err)
    }
    defer client.Close()

    session, err := client.NewSession()
    if err != nil {
        return fmt.Errorf("ssh session: %w", err)
    }
    defer session.Close()

    session.Stderr = &stderrBuf{}
    if err := session.Run(script); err != nil {
        return fmt.Errorf("run script: %w", err)
    }
    return nil
}
```

The `buildEnableRootLoginScript()` function generates the same bash script as the Java code:

```go
func buildEnableRootLoginScript() string {
    return `#!/bin/bash
set -e

# Detect sudo need
if [ "$(id -u)" -ne 0 ]; then SUDO_CMD="sudo"; else SUDO_CMD=""; fi

# Backup sshd_config
$SUDO_CMD cp /etc/ssh/sshd_config /etc/ssh/sshd_config.bak.$(date +%Y%m%d%H%M%S)

# Modify sshd_config
$SUDO_CMD sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
$SUDO_CMD sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
$SUDO_CMD sed -i 's/^#\?ChallengeResponseAuthentication.*/ChallengeResponseAuthentication yes/' /etc/ssh/sshd_config
$SUDO_CMD sed -i 's/^UsePAM no/UsePAM yes/' /etc/ssh/sshd_config

# OS-specific handling
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$(echo "$ID" | tr '[:upper:]' '[:lower:]')
    case $OS in
        ubuntu|debian)
            if [ -f /etc/pam.d/sshd ]; then
                $SUDO_CMD cp /etc/pam.d/sshd /etc/pam.d/sshd.bak
                $SUDO_CMD sed -i 's/@include common-auth/#@include common-auth/' /etc/pam.d/sshd
            fi
            ;;
        ol|rhel|centos|almalinux|rocky)
            if command -v getenforce >/dev/null 2>&1; then
                if [ "$(getenforce)" = "Enforcing" ]; then
                    $SUDO_CMD setenforce 0
                    $SUDO_CMD sed -i 's/^SELINUX=enforcing/SELINUX=permissive/' /etc/selinux/config
                fi
            fi
            ;;
    esac
fi

# Override drop-in configs in sshd_config.d/
if [ -d /etc/ssh/sshd_config.d/ ]; then
    for file in /etc/ssh/sshd_config.d/*.conf; do
        if [ -f "$file" ]; then
            $SUDO_CMD sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication yes/' "$file"
            $SUDO_CMD sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin yes/' "$file"
        fi
    done
fi

# Install persistent self-healing script
$SUDO_CMD cat > /usr/local/bin/check-ssh-config.sh << 'SCRIPT'
#!/bin/bash
if ! grep -q "^PermitRootLogin yes" /etc/ssh/sshd_config || \
   ! grep -q "^PasswordAuthentication yes" /etc/ssh/sshd_config; then
    sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
    sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
    if [ -d /etc/ssh/sshd_config.d/ ]; then
        for f in /etc/ssh/sshd_config.d/*.conf; do
            [ -f "$f" ] && sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication yes/' "$f" && \
                           sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin yes/' "$f"
        done
    fi
    systemctl restart sshd 2>/dev/null || service sshd restart 2>/dev/null
fi
SCRIPT
$SUDO_CMD chmod +x /usr/local/bin/check-ssh-config.sh

# Create systemd service (or rc.local fallback)
if command -v systemctl >/dev/null 2>&1; then
    $SUDO_CMD cat > /etc/systemd/system/check-ssh-config.service << 'UNIT'
[Unit]
Description=Check SSH Configuration for Root Login
After=network.target sshd.service

[Service]
Type=oneshot
ExecStart=/usr/local/bin/check-ssh-config.sh

[Install]
WantedBy=multi-user.target
UNIT
    $SUDO_CMD systemctl daemon-reload
    $SUDO_CMD systemctl enable check-ssh-config.service
else
    if [ -f /etc/rc.local ]; then
        grep -q "check-ssh-config.sh" /etc/rc.local || \
            $SUDO_CMD sed -i '/^exit 0/i /usr/local/bin/check-ssh-config.sh' /etc/rc.local
    fi
fi

# Validate and restart sshd
$SUDO_CMD sshd -t && $SUDO_CMD systemctl restart sshd 2>/dev/null || $SUDO_CMD service sshd restart
`
}
```

---

## 4. Feature 3: Rescue Mode Improvements

### 4.1 Java Rescue Flow (Key Differences from Go)

The Java `RescueWebSocketHandler.java` (1261 lines) has these steps that the Go `rescue.go` does NOT currently implement:

| Step | Java Behavior | Go Current State |
|------|--------------|-----------------|
| Pre-rescue | `securityRuleService.checkAndEnableRule(tenant)` — opens all protocols | Missing |
| Helper ping | `PingUtil.ping(helperHost).isReachable()` — verifies helper instance SSH | Missing |
| Rescue commands | `executeOciRescueCommands2()` — SSH into helper, write rescue image to ARM boot volume via dd | Uses `AttachRescueVolume` (different approach — attaches a pre-built rescue boot volume instead of dd-writing) |
| Post-rescue | `enableRootLogin()` — re-enable root password login on rescued instance | Missing |
| SSH persistence | `OciSshConnService.saveOrUpdate()` — saves SSH credentials to DB | Missing |
| Fallback | `doHelpFromBootVolumeBackup()` — restore from backup if rescue fails | Missing |
| Notification | Telegram success/failure notification | Missing |

### 4.2 Go Rescue Improvements

The Go rescue handler uses a **rescue boot volume** approach (attach a pre-built rescue image as a boot volume) rather than Java's **dd-write** approach (SSH into a helper instance and dd a rescue image onto the original boot volume). Both are valid; the Go approach is simpler but requires a pre-existing rescue image.

**Additions needed in `internal/ws/rescue.go`:**

#### 4.2.1 Pre-Rescue Security Rule Opening

Add to `runRescueFlow()`, before Step 1:

```go
// Pre-rescue: ensure security rules are open.
if err := h.securityRules.CheckAndEnableRule(ctx, tenantID); err != nil {
    send(RescueStatus{Step: "error", Message: "开放安全规则失败", Error: err.Error(), Progress: 0})
    return
}
```

This requires adding a `SecurityRules *SecurityRuleService` field to `RescueDeps`.

#### 4.2.2 Post-Rescue Root Login Enablement

Add to `CompleteRescue()`, after Step 10 (start instance) and before the "complete" status:

```go
// Step 10.5: Wait for instance to be reachable, then enable root login.
send(RescueStatus{Step: "enable_root", Message: "等待实例启动并配置SSH...", Progress: 99})
time.Sleep(30 * time.Second) // Give sshd time to start

info, err := deps.GetInstance(flow.InstanceID, tenantID)
if err == nil && info.PublicIP != "" {
    if err := h.sshConfig.EnableRootLogin(info.PublicIP, "root",
        info.SSHPassword, info.SSHPassword, 22); err != nil {
        send(RescueStatus{Step: "warning", Message: "SSH配置失败（实例可能需要手动配置）",
            Error: err.Error(), Progress: 100})
    } else {
        // Save SSH credentials to DB.
        h.saveSSHConfig(flow.InstanceID, info)
    }
}
```

This requires adding `SSHConfig *SSHConfigurator` to `RescueDeps`.

#### 4.2.3 SSH Credential Persistence

```go
// saveSSHConfig stores SSH connection details in the instance_ssh_conn table.
func (h *RescueHandler) saveSSHConfig(instanceID string, info *RescueInstanceInfo) {
    // Parity with Java OciSshConnService.saveOrUpdate.
    // INSERT OR REPLACE into instance_ssh_conn:
    //   instance_id, host (publicIP), port (22), username ("root"), password
}
```

#### 4.2.4 Backup Fallback

When the rescue flow fails (e.g., helper instance cannot be created, rescue image unavailable), fall back to restoring from the most recent boot volume backup. This is the Java `doHelpFromBootVolumeBackup` path.

```go
// Fallback to backup restoration if rescue fails.
func (h *RescueHandler) fallbackToBackupRestore(conn *websocket.Conn, flow *rescueFlow, tenantID int64) {
    send(RescueStatus{Step: "fallback", Message: "急救失败，尝试从备份恢复...", Progress: 10})
    // 1. Find most recent backup for this instance's boot volume.
    // 2. If backup is in a different region, copy it first.
    // 3. Create a new boot volume from the backup.
    // 4. Attach the restored boot volume.
    // 5. Start the instance.
}
```

---

## 5. Feature 4: Backup Flow Enhancements

### 5.1 Current Go Backup Flow

```
grabber/success.go: onGrabSuccess()
  → time.AfterFunc(3min, scheduleBackup)
    → BackupSvc.ScheduleBackup()
      → [TODO] ping + open security rules
      → [TODO] SSH enableRootLogin
      → createBackup() → oci.CreateBootVolumeBackup()
```

### 5.2 Enhanced Go Backup Flow

```
grabber/success.go: onGrabSuccess()
  → time.AfterFunc(3min, scheduleBackup)
    → BackupSvc.ScheduleBackup()
      → checkReachability(ip, 22)
        → if unreachable: securityRules.CheckAndEnableRule(tenantID)
        → sleep 10s (OCI eventual consistency)
        → retry checkReachability(ip, 22)
          → if still unreachable: log warning, SKIP backup, return
      → sshConfig.EnableRootLogin(ip, "root", rootPass, rootPass, 22)
        → if failed: log warning, SKIP backup, return
      → createBackup() → oci.CreateBootVolumeBackup()
```

**Key design decision:** The Java code skips backup entirely if `enableRootLogin` fails. This is intentional — the backup should only be created for instances that are confirmed healthy and SSH-accessible. The Go implementation should follow the same pattern.

### 5.3 BackupSvc Dependencies Update

```go
type BackupSvc struct {
    store         *db.Store
    masterKey     []byte
    logger        zerolog.Logger
    securityRules *SecurityRuleService  // NEW
    sshConfig     *SSHConfigurator      // NEW
}
```

---

## 6. Database Changes

### 6.1 No Schema Changes Required

All necessary tables and columns already exist:

| Table | Column | Purpose | Status |
|-------|--------|---------|--------|
| `tenant` | `enable_all_protocol` | Flag: security rules already opened | Exists |
| `tenant` | `enable_icmp` | Flag: ICMP rules opened | Exists |
| `instance_detail` | `public_ip` | Instance public IP for ping/SSH | Exists |
| `instance_detail` | `port` | SSH port (default 22) | Exists |
| `instance_detail` | `enable_ping` | Flag: participate in ping checks | Exists |
| `instance_detail` | `conn_time` | Last successful TCP connect timestamp | Exists |
| `instance_detail` | `last_heartbeat` | Last heartbeat timestamp | Exists |
| `instance_backup_detail` | (all columns) | Backup tracking | Exists |

### 6.2 Optional New Table: `instance_ssh_conn`

The Java project stores SSH connection details in a separate table (`CloudSshConnRepository`). The Go project should consider adding this for the rescue flow to persist SSH credentials:

```sql
CREATE TABLE IF NOT EXISTS instance_ssh_conn (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id TEXT NOT NULL,
    host        TEXT NOT NULL,
    port        INTEGER DEFAULT 22,
    username    TEXT DEFAULT 'root',
    password    TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(instance_id)
);
```

This is optional — the rescue handler can pass SSH details via WebSocket without DB persistence. But persisting them allows the frontend to auto-populate the SSH terminal.

---

## 7. Ping Check Integration with PingSvc

### 7.1 Current State

`PingSvc.CheckPingConn()` runs every 5 minutes via cron. It only updates `conn_time` on success. It does NOT trigger any action on failure.

### 7.2 Enhancement: Auto-Recovery on Ping Failure

When `CheckPingConn` detects an instance that was previously reachable (has a non-zero `conn_time`) but is now unreachable, it should trigger the security rule opening flow:

```go
// In CheckPingConn, after the TCP connect fails:
if connErr != nil {
    // Instance was previously reachable but now isn't.
    if inst.ConnTime > 0 {
        s.logger.Warn().Str("ip", ip).Int64("instanceId", inst.ID).
            Msg("ping: instance went offline, attempting auto-recovery")
        // Trigger security rule opening (async, don't block the ping loop).
        go s.attemptAutoRecovery(ctx, inst)
    }
    continue
}
```

```go
func (s *PingSvc) attemptAutoRecovery(ctx context.Context, inst repo.InstanceDetail) {
    if !inst.TenantID.Valid {
        return
    }
    // Open all protocols.
    if err := s.securityRules.CheckAndEnableRule(ctx, inst.TenantID.Int64); err != nil {
        s.logger.Error().Err(err).Msg("ping: auto-recovery: failed to open security rules")
        return
    }
    // Wait for OCI eventual consistency.
    time.Sleep(15 * time.Second)
    // Re-check.
    addr := net.JoinHostPort(inst.PublicIps.String, "22")
    conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
    if err != nil {
        s.logger.Warn().Str("ip", inst.PublicIps.String).Msg("ping: auto-recovery: still unreachable")
        return
    }
    conn.Close()
    s.logger.Info().Str("ip", inst.PublicIps.String).Msg("ping: auto-recovery: instance is reachable again")
}
```

This requires adding `securityRules *SecurityRuleService` to `PingSvc`.

---

## 8. Edge Cases

### 8.1 OCI Eventual Consistency

After modifying security lists, the change takes 5-15 seconds to propagate. The backup and rescue flows must include a sleep/retry loop after `CheckAndEnableRule`. The Java code does NOT explicitly sleep (it relies on the SSH connection timeout as implicit delay), but the Go code should be explicit:

```go
// After CheckAndEnableRule:
time.Sleep(10 * time.Second)
```

### 8.2 Instance Not Yet Booted

The 3-minute delay after grab may not be enough for the instance to fully boot and start sshd. If the TCP connect fails but the instance state is RUNNING, the system should retry with exponential backoff (30s, 60s, 120s) before giving up.

### 8.3 SSH Host Key Mismatch

`ssh.InsecureIgnoreHostKey()` is used because the system manages instances it created. This matches the Java JSch `StrictHostKeyChecking=no` behavior. No change needed.

### 8.4 OCI Image Variations

Different OCI images (Ubuntu, Oracle Linux, CentOS, Debian) have different sshd_config layouts:
- Some use `sshd_config.d/` drop-in directory.
- Some have `PasswordAuthentication no` as default.
- Some have `PermitRootLogin prohibit-password` as default.
- SELinux is only relevant for RHEL-based distros.

The `buildEnableRootLoginScript()` handles all of these cases (same as Java).

### 8.5 Race Condition: Concurrent Backups

Multiple instances may trigger `ScheduleBackup` simultaneously. Each call creates its own OCI client and SSH connection — no shared state. The `CheckAndEnableRule` call is idempotent (skips if `enableAllProtocol == 1`), so concurrent calls for the same tenant are safe.

### 8.6 Security Rule Accumulation

`EnableAllForTenant` uses `AddSecurityRule` which has duplicate detection (removes matching rules before adding). This prevents rule accumulation over repeated calls. No change needed.

### 8.7 IPv6-Only Instances

If an instance only has an IPv6 address, the TCP connect check and SSH connection must use IPv6. The current Go code uses `net.DialTimeout("tcp", addr, ...)` which handles IPv6 addresses correctly when formatted as `[::1]:22`.

### 8.8 Rescue with Backup Fallback

If the rescue flow fails at any step (helper instance creation fails, rescue image unavailable, SSH timeout), the system should attempt to restore from the most recent backup. The Java code implements this as `doHelpFromBootVolumeBackup`. The Go implementation should:
1. Query `instance_backup_detail` for the most recent backup of the instance's boot volume.
2. If the backup is in a different region, copy it first (OCI cross-region backup copy).
3. Create a new boot volume from the backup.
4. Attach it to the instance and start.

---

## 9. Go Implementation Checklist

### 9.1 New File: `internal/service/ssh_config.go`

- [ ] `SSHConfigurator` struct with logger
- [ ] `EnableRootLogin(host, username, password, rootPassword string, port int) error`
- [ ] `execScript(host, username, password string, port int, script string) error`
- [ ] `buildEnableRootLoginScript() string` — full bash script matching Java behavior

### 9.2 Modify: `internal/service/security_rule.go`

- [ ] Add `CheckAndEnableRule(ctx context.Context, tenantID int64) error` method
- [ ] Idempotent gate: skip if `tenant.enableAllProtocol == 1`

### 9.3 Modify: `internal/service/backup.go`

- [ ] Add `securityRules *SecurityRuleService` and `sshConfig *SSHConfigurator` fields to `BackupSvc`
- [ ] Update `NewBackupSvc` constructor
- [ ] Implement `checkReachability(host string, port int) bool`
- [ ] Fill TODO at line 55: ping → open security rules → retry
- [ ] Fill TODO at line 56: SSH enableRootLogin → skip backup on failure

### 9.4 Modify: `internal/ws/rescue.go`

- [ ] Add `SecurityRules *SecurityRuleService` and `SSHConfig *SSHConfigurator` to `RescueDeps`
- [ ] Add pre-rescue `CheckAndEnableRule` call in `runRescueFlow()`
- [ ] Add post-rescue `EnableRootLogin` call in `CompleteRescue()`
- [ ] Add `saveSSHConfig()` for DB persistence
- [ ] Add `fallbackToBackupRestore()` for rescue failure recovery

### 9.5 Modify: `internal/service/ping.go`

- [ ] Add `securityRules *SecurityRuleService` field to `PingSvc`
- [ ] Add `attemptAutoRecovery()` goroutine on ping failure for previously-reachable instances

### 9.6 Optional: `internal/repo/ssh_conn.go`

- [ ] SQL migration for `instance_ssh_conn` table
- [ ] `InsertOrUpdateSSHConn` query
- [ ] `GetSSHConnByInstanceID` query

### 9.7 Wiring: `cmd/server/main.go` or equivalent

- [ ] Create `SSHConfigurator` instance
- [ ] Inject into `BackupSvc`, `RescueHandler`, `PingSvc`

---

## 10. Security Considerations

### 10.1 "Enable All Protocols" Is a Firewall Kill Switch

`CheckAndEnableRule` opens protocol `all` from `0.0.0.0/0` — this effectively disables the OCI security list firewall for the tenant. This is the same behavior as the Java code. It is a deliberate design choice: the system needs unrestricted access to manage instances. However:

- The `enable_all_protocol` flag prevents repeated application.
- The flag should be exposed in the UI so operators can manually re-close protocols.
- Consider adding a scheduled job to re-close protocols after a timeout (e.g., 24 hours).

### 10.2 Root Password in Memory

The root password is passed through `BackupInput`, `RescueInstanceInfo`, and SSH sessions. It is never logged. The `SSHConfigurator.execScript` method should ensure the password is not included in error messages.

### 10.3 SSH InsecureIgnoreHostKey

Used because the system manages instances it created. This is standard for infrastructure management tools. Do NOT use in production for connecting to untrusted hosts.

---

## 11. Testing Strategy

### 11.1 Unit Tests

- `checkReachability`: mock TCP listener, verify timeout behavior.
- `buildEnableRootLoginScript`: snapshot test — verify script content matches expected.
- `CheckAndEnableRule`: mock OCI client, verify idempotent gate, verify rule additions.

### 11.2 Integration Tests

- Create a real OCI instance, run `ScheduleBackup`, verify backup is created.
- Verify `EnableRootLogin` actually configures sshd_config on a real instance.
- Verify rescue flow with security rule pre-check.

### 11.3 Manual Testing

- Create an OCI tenant with default security rules (no port 22).
- Launch an instance, wait 3 minutes.
- Verify: backup succeeds (security rules auto-opened, root login enabled).
- Verify: SSH terminal works with root password.
