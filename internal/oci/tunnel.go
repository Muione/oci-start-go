// Package oci — tunnel.go: SSH tunnel construction for OCI console connections.
// Parses OCI console connection strings and builds SSH tunnel commands that
// route through the OCI console proxy on port 443 (not port 22).
package oci

import (
	"fmt"
	"regexp"
	"strings"
)

// ParsedConnectionString holds the components extracted from an OCI console
// connection string.
type ParsedConnectionString struct {
	ConnectionID string // console-connection OCID (the ProxyCommand's SSH user)
	ProxyHost    string // "<connID>@instance-console.<region>" — the inner SSH target
	TargetHost   string // instance OCID — the outer SSH destination + -L remote host (the console proxy routes by it)
}

// connectionIDRegex matches the console-connection OCID.
var connectionIDRegex = regexp.MustCompile(`ocid1\.instanceconsoleconnection\.[a-z0-9.-]+`)

// instanceIDRegex matches the INSTANCE OCID (ocid1.instance.<realm>.<...>).
// The literal ".instance." (dot on both sides) avoids matching the
// console-connection OCID, which is "ocid1.instanceconsoleconnection.".
var instanceIDRegex = regexp.MustCompile(`ocid1\.instance\.[a-z0-9]+\.[a-z0-9.-]+`)

// ParseConnectionString extracts the connection ID, proxy host, and instance
// OCID from an OCI VncConnectionString. The real format is:
//
//	ssh -o ProxyCommand='ssh -W %h:%p -p 443 <connID>@instance-console.<r>' \
//	    -N -L localhost:5900:<instanceID>:5900 <instanceID>
//
// The console proxy routes both the -W (serial) and -L (VNC) by INSTANCE OCID,
// so the outer ssh destination and the -L remote host are the instance OCID
// (NOT 127.0.0.1 / the proxy host). strings.Fields ignores the single quotes
// around the ProxyCommand value, so we Trim quotes off extracted tokens to
// avoid leaking a quote into the rebuilt ssh command ("invalid quotes").
func ParseConnectionString(connectionString string) (*ParsedConnectionString, error) {
	if connectionString == "" {
		return nil, fmt.Errorf("empty connection string")
	}

	// Console-connection OCID (the ProxyCommand's SSH user).
	connID := connectionIDRegex.FindString(connectionString)
	if connID == "" {
		return nil, fmt.Errorf("no console connection ID found in: %s", connectionString)
	}

	// Instance OCID (outer ssh destination + -L remote host).
	instanceID := instanceIDRegex.FindString(connectionString)
	if instanceID == "" {
		return nil, fmt.Errorf("no instance ID found in: %s", connectionString)
	}

	trimQuotes := func(s string) string { return strings.Trim(s, "'\"") }

	// ProxyHost: the token after "-p 443" in the ProxyCommand —
	// "<connID>@instance-console.<region>", possibly with a trailing quote.
	proxyHost := ""
	parts := strings.Fields(connectionString)
	for i, part := range parts {
		if part == "-p" && i+1 < len(parts) && parts[i+1] == "443" {
			if i+2 < len(parts) {
				proxyHost = trimQuotes(parts[i+2])
			}
			break
		}
	}
	if proxyHost == "" {
		return nil, fmt.Errorf("no proxy host (-p 443 <host>) found in: %s", connectionString)
	}

	return &ParsedConnectionString{
		ConnectionID: connID,
		ProxyHost:    proxyHost,
		TargetHost:   instanceID,
	}, nil
}

// SSHTunnelConfig holds the configuration for building an SSH tunnel command.
type SSHTunnelConfig struct {
	PrivateKeyPath string
	ConnectionID   string
	ProxyHost      string
	TargetHost     string
	LocalPort      int
	RemotePort     int
}

// BuildSerialConsoleCommand returns the SSH command arguments for an
// INTERACTIVE serial-console session (text terminal), as opposed to the VNC
// tunnel (which is -L/-N). It connects to the OCI console proxy on port 443
// using the connection OCID as the SSH user, -W forwards to the instance OCID
// (the proxy routes by it), and -tt forces a remote PTY so the serial console
// is an interactive byte stream. The caller bridges a WebSocket to the ssh
// process's stdin/stdout.
func BuildSerialConsoleCommand(cfg SSHTunnelConfig) []string {
	proxyCmd := fmt.Sprintf(
		"ssh -i %s -o StrictHostKeyChecking=no -o PubkeyAcceptedKeyTypes=+ssh-rsa -o HostKeyAlgorithms=+ssh-rsa -p 443 %s -W %%h:%%p",
		cfg.PrivateKeyPath,
		cfg.ProxyHost,
	)

	return []string{
		"ssh",
		"-tt", // force remote PTY even though stdin is a pipe (WS bridge)
		"-i", cfg.PrivateKeyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "PubkeyAcceptedKeyTypes=+ssh-rsa",
		"-o", "HostKeyAlgorithms=+ssh-rsa",
		"-o", fmt.Sprintf("ProxyCommand=%s", proxyCmd),
		cfg.TargetHost, // outer ssh "host" = instance OCID; ProxyCommand -W routes via the console proxy
	}
}

// BuildSSHTunnelCommand returns the SSH command arguments for establishing a
// VNC tunnel through the OCI console proxy. The local port is forwarded to
// <instanceID>:5900 on the remote — the console proxy routes by INSTANCE OCID
// to the instance's VNC port (NOT 127.0.0.1:5900; the proxy itself doesn't
// serve VNC). The outer ssh destination is the instance OCID; the ProxyCommand
// connects to the console proxy on port 443 using the connection OCID as the
// SSH user, and -W forwards to %h:%p (the instance OCID) which the proxy also
// routes by instance OCID. Use -N (no shell) + -L (forward only).
func BuildSSHTunnelCommand(cfg SSHTunnelConfig) []string {
	if cfg.RemotePort == 0 {
		cfg.RemotePort = 5900
	}

	proxyCmd := fmt.Sprintf(
		"ssh -i %s -o StrictHostKeyChecking=no -o PubkeyAcceptedKeyTypes=+ssh-rsa -o HostKeyAlgorithms=+ssh-rsa -p 443 %s -W %%h:%%p",
		cfg.PrivateKeyPath,
		cfg.ProxyHost,
	)

	// -L forwards localPort -> <instanceID>:5900 (the proxy routes by instance
	// OCID to the instance's VNC port).
	localForward := fmt.Sprintf("%d:%s:%d", cfg.LocalPort, cfg.TargetHost, cfg.RemotePort)

	return []string{
		"ssh",
		"-i", cfg.PrivateKeyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "PubkeyAcceptedKeyTypes=+ssh-rsa",
		"-o", "HostKeyAlgorithms=+ssh-rsa",
		"-o", fmt.Sprintf("ProxyCommand=%s", proxyCmd),
		"-L", localForward,
		"-N",
		cfg.TargetHost, // outer ssh "host" = instance OCID; ProxyCommand -W routes via the console proxy
	}
}
