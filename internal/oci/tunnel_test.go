// Package oci — tunnel_test.go: locks ParseConnectionString against the real
// OCI InstanceConsoleConnection.VncConnectionString format (the SSH tunnel
// string for VNC, not the serial-console ConnectionString).
package oci

import (
	"strings"
	"testing"
)

// Realistic OCI VncConnectionString (from a live ap-chuncheon-1 connection).
// The -L forward targets <instanceID>:5900 (the console proxy routes by
// INSTANCE OCID), and the outer ssh destination is the instance OCID. The
// closing single quote of the ProxyCommand sticks to the token after "-p 443";
// the parser must strip it.
func TestParseConnectionString_VncFormat(t *testing.T) {
	const connID = "ocid1.instanceconsoleconnection.oc1.ap-chuncheon-1.an4w4ljr3csr7nqcnvngzikd422xu6g373ay2iqz4w5uvo2pvbdholwwkjdq"
	const instanceID = "ocid1.instance.oc1.ap-chuncheon-1.an4w4ljr3csr7nqcnvngzikd422xu6g373ay2iqz4w5uvo2pvbdholwwkjdq"
	const host = "instance-console.ap-chuncheon-1.oci.oraclecloud.com"
	const s = "ssh -o ProxyCommand='ssh -W %h:%p -p 443 " + connID + "@" + host + "' -N -L localhost:5900:" + instanceID + ":5900 " + instanceID

	p, err := ParseConnectionString(s)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.ConnectionID != connID {
		t.Errorf("ConnectionID=%q want %q", p.ConnectionID, connID)
	}
	if p.ProxyHost != connID+"@"+host {
		t.Errorf("ProxyHost=%q want %q", p.ProxyHost, connID+"@"+host)
	}
	if p.TargetHost != instanceID {
		t.Errorf("TargetHost=%q want instanceID %q", p.TargetHost, instanceID)
	}
}

// TestBuildSerialConsoleCommand: the interactive serial-console ssh command
// must force a remote PTY (-tt), have NO -L/-N (it's an interactive shell, not
// a tunnel), target the instance OCID, and include the key in both the outer
// ssh and the ProxyCommand. No quote may leak.
func TestBuildSerialConsoleCommand(t *testing.T) {
	const connID = "ocid1.instanceconsoleconnection.oc1.ap-chuncheon-1.x"
	const instanceID = "ocid1.instance.oc1.ap-chuncheon-1.x"
	const host = "instance-console.ap-chuncheon-1.oci.oraclecloud.com"
	const s = "ssh -o ProxyCommand='ssh -W %h:%p -p 443 " + connID + "@" + host + "' " + instanceID
	p, err := ParseConnectionString(s)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	args := BuildSerialConsoleCommand(SSHTunnelConfig{
		PrivateKeyPath: "/tmp/key.pem",
		ProxyHost:      p.ProxyHost,
		TargetHost:     p.TargetHost,
	})

	has := func(flag string) bool {
		for _, a := range args {
			if a == flag {
				return true
			}
		}
		return false
	}
	if !has("-tt") {
		t.Error("missing -tt (force remote PTY for interactive serial console)")
	}
	if has("-N") {
		t.Error("must NOT have -N (serial console is an interactive shell, not a tunnel)")
	}
	if has("-L") {
		t.Error("must NOT have -L (serial console is interactive, no port forward)")
	}
	if args[len(args)-1] != instanceID {
		t.Errorf("outer target=%q, want instanceID %q", args[len(args)-1], instanceID)
	}
	for _, a := range args {
		if strings.ContainsAny(a, "'\"") {
			t.Errorf("arg contains a quote (ssh 'invalid quotes'): %q", a)
		}
	}
}
func TestBuildSSHTunnelCommand_TargetsInstanceID(t *testing.T) {
	const connID = "ocid1.instanceconsoleconnection.oc1.ap-chuncheon-1.x"
	const instanceID = "ocid1.instance.oc1.ap-chuncheon-1.x"
	const host = "instance-console.ap-chuncheon-1.oci.oraclecloud.com"
	const s = "ssh -o ProxyCommand='ssh -W %h:%p -p 443 " + connID + "@" + host + "' -N -L localhost:5900:" + instanceID + ":5900 " + instanceID
	p, err := ParseConnectionString(s)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	args := BuildSSHTunnelCommand(SSHTunnelConfig{
		PrivateKeyPath: "/tmp/key.pem",
		ConnectionID:   p.ConnectionID,
		ProxyHost:      p.ProxyHost,
		TargetHost:     p.TargetHost,
		LocalPort:      36909,
	})

	// Find -L and check its remote target is the instance OCID.
	var localForward string
	var outerTarget string
	for i, a := range args {
		if a == "-L" && i+1 < len(args) {
			localForward = args[i+1]
		}
	}
	// outer target is the last arg.
	outerTarget = args[len(args)-1]

	if localForward != "36909:"+instanceID+":5900" {
		t.Errorf("-L forward=%q, want 36909:%s:5900 (instance OCID, not 127.0.0.1)", localForward, instanceID)
	}
	if outerTarget != instanceID {
		t.Errorf("outer ssh target=%q, want instanceID %q (not <connID>@<proxy>)", outerTarget, instanceID)
	}
	for _, a := range args {
		if strings.ContainsAny(a, "'\"") {
			t.Errorf("arg contains a quote (ssh 'invalid quotes'): %q", a)
		}
	}
}

func TestParseConnectionString_Empty(t *testing.T) {
	if _, err := ParseConnectionString(""); err == nil {
		t.Fatal("want error on empty")
	}
}

func TestParseConnectionString_NoConnID(t *testing.T) {
	if _, err := ParseConnectionString("ssh -p 443 somehost -W %h:%p user@host"); err == nil {
		t.Fatal("want error when no connection OCID present")
	}
}
