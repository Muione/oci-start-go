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
	ConnectionID string
	ProxyHost    string
	TargetHost   string
}

// connectionIDRegex matches OCI console connection OCIDs.
var connectionIDRegex = regexp.MustCompile(`ocid1\.instanceconsoleconnection\.[a-z0-9.-]+`)

// ParseConnectionString extracts the connection ID, proxy host, and target host
// from an OCI console connection string. The format is typically:
//
//	ssh -o ProxyCommand='ssh -i <key> -p 443 <proxyHost> -W %h:%p' <connID>@<targetHost>
func ParseConnectionString(connectionString string) (*ParsedConnectionString, error) {
	if connectionString == "" {
		return nil, fmt.Errorf("empty connection string")
	}

	// Extract connection ID (ocid1.instanceconsoleconnection.xxx).
	match := connectionIDRegex.FindString(connectionString)
	if match == "" {
		return nil, fmt.Errorf("no connection ID found in: %s", connectionString)
	}

	// Extract proxy host: the token after "@" in the ProxyCommand section.
	// Typically: ssh -i <key> -p 443 <proxyHost> -W %h:%p
	proxyHost := ""
	parts := strings.Fields(connectionString)
	for i, part := range parts {
		if part == "-p" && i+1 < len(parts) && parts[i+1] == "443" {
			if i+2 < len(parts) {
				proxyHost = parts[i+2]
			}
			break
		}
	}

	// Extract target host: the last token after the final "@" (or last field).
	targetHost := ""
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.Contains(parts[i], "@") {
			atParts := strings.SplitN(parts[i], "@", 2)
			if len(atParts) == 2 {
				targetHost = atParts[1]
			}
			break
		}
	}
	if targetHost == "" && len(parts) > 0 {
		targetHost = parts[len(parts)-1]
	}

	return &ParsedConnectionString{
		ConnectionID: match,
		ProxyHost:    proxyHost,
		TargetHost:   targetHost,
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

// BuildSSHTunnelCommand returns the SSH command arguments for establishing a
// tunnel through the OCI console proxy. The tunnel connects localPort on
// 127.0.0.1 to remotePort on the target instance, routing through the OCI
// console proxy on port 443.
func BuildSSHTunnelCommand(cfg SSHTunnelConfig) []string {
	if cfg.RemotePort == 0 {
		cfg.RemotePort = 5900
	}

	proxyCmd := fmt.Sprintf(
		"ssh -i %s -o StrictHostKeyChecking=no -o PubkeyAcceptedKeyTypes=+ssh-rsa -o HostKeyAlgorithms=+ssh-rsa -p 443 %s -W %%h:%%p",
		cfg.PrivateKeyPath,
		cfg.ProxyHost,
	)

	localForward := fmt.Sprintf("%d:127.0.0.1:%d", cfg.LocalPort, cfg.RemotePort)

	return []string{
		"ssh",
		"-i", cfg.PrivateKeyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "PubkeyAcceptedKeyTypes=+ssh-rsa",
		"-o", "HostKeyAlgorithms=+ssh-rsa",
		"-o", fmt.Sprintf("ProxyCommand=%s", proxyCmd),
		"-L", localForward,
		"-N",
		fmt.Sprintf("%s@%s", cfg.ConnectionID, cfg.TargetHost),
	}
}
