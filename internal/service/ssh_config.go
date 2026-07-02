// Package service -- ssh_config.go: SSH root login enablement (Phase 12.3).
// Port of JschUtils.enableRootLogin. SSHs into an instance and configures
// PermitRootLogin yes, PasswordAuthentication yes, with OS-specific
// handling and persistent self-healing service installation.
package service

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/Muione/oci-start-go/internal/util/crypto"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
)

// SSHConfigurator enables root password login on remote instances.
type SSHConfigurator struct {
	logger    zerolog.Logger
	masterKey []byte // S4: decrypt instance_detail.password (encrypted at rest)
}

// NewSSHConfigurator constructs an SSHConfigurator.
func NewSSHConfigurator(logger zerolog.Logger) *SSHConfigurator {
	return &SSHConfigurator{logger: logger}
}

// SetMasterKey arms the configurator to decrypt instance passwords stored as
// AES-256-GCM ciphertext (S4). Unset → passwords treated as plaintext (legacy).
func (s *SSHConfigurator) SetMasterKey(key []byte) { s.masterKey = key }

// decryptIfSet decrypts a password when masterKey is armed; otherwise returns
// the value as-is. Ciphertext that fails GCM auth also falls back to raw
// (legacy plaintext rows), via DecryptStringWithFallback.
func (s *SSHConfigurator) decryptIfSet(pw string) string {
	if s.masterKey == nil {
		return pw
	}
	return crypto.DecryptStringWithFallback(pw, s.masterKey)
}

// EnableRootLogin SSHs into the instance and configures root password login.
// Returns nil on success, error on failure.
// Parity with Java JschUtils.enableRootLogin.
func (s *SSHConfigurator) EnableRootLogin(host, username, password, rootPassword string, port int) error {
	// S4: instance_detail.password is AES-encrypted for new rows; decrypt
	// before SSH dial. nil masterKey → passthrough (legacy/unwired).
	password = s.decryptIfSet(password)
	rootPassword = s.decryptIfSet(rootPassword)
	// 1. Set root password. The password is piped to chpasswd over SSH stdin,
	// never interpolated into the shell command, so shell metacharacters in the
	// password cannot break out into command execution (S8).
	if rootPassword != "" {
		script, stdin := buildRootPasswordInput(rootPassword)
		if err := s.execScript(host, username, password, port, script, strings.NewReader(stdin)); err != nil {
			return fmt.Errorf("set root password: %w", err)
		}
	}

	// 2. Main sshd_config modification script.
	script := buildEnableRootLoginScript()
	if err := s.execScript(host, username, password, port, script, nil); err != nil {
		return fmt.Errorf("configure sshd: %w", err)
	}

	return nil
}

// buildRootPasswordInput returns the chpasswd command and the stdin payload that
// sets the root password. The password travels in stdin (read by chpasswd), not
// in the command string, eliminating shell-injection risk from a password
// containing ;, $, backticks, quotes, etc.
func buildRootPasswordInput(rootPassword string) (script, stdin string) {
	return "chpasswd", "root:" + rootPassword + "\n"
}

// execScript connects via SSH and executes a shell script. When stdin is
// non-nil it is piped to the remote command (used to feed chpasswd the password
// without interpolating it into the command string).
func (s *SSHConfigurator) execScript(host, username, password string, port int, script string, stdin io.Reader) error {
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

	var stderr bytes.Buffer
	session.Stderr = &stderr
	if stdin != nil {
		session.Stdin = stdin
	}

	if err := session.Run(script); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("run script: %w (stderr: %s)", err, stderr.String())
		}
		return fmt.Errorf("run script: %w", err)
	}
	return nil
}

// buildEnableRootLoginScript generates the bash script that configures
// PermitRootLogin, PasswordAuthentication, and installs a self-healing
// systemd service. Handles Ubuntu/Debian PAM and RHEL SELinux differences.
// Parity with Java JschUtils.enableRootLogin.
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
