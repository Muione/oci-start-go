// Package ws — ssh_hostkey_test.go: S10 config-driven host-key toggle.
package ws

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

// TestSetHostKeyVerify (S10) asserts the config-driven toggle swaps the package
// callback: false → any host key accepted (legacy compat); true → unknown host
// rejected (fail-closed when known_hosts is missing, as in CI).
func TestSetHostKeyVerify(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hostPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	hostname := "127.0.0.1:22"
	remote := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}

	// Save & restore package state (path + callback). Tests in this package run
	// sequentially (no t.Parallel), so mutating package vars is safe.
	origPath := knownHostsPath
	origCB := hostKeyCallback
	defer func() {
		knownHostsPath = origPath
		hostKeyCallback = origCB
	}()

	// Point at a known-missing file so secure mode fails closed deterministically
	// (doesn't depend on ./data/known_hosts existing in the test CWD).
	knownHostsPath = filepath.Join(t.TempDir(), "missing-known_hosts")

	SetHostKeyVerify(true)
	if err := hostKeyCallback(hostname, remote, hostPub); err == nil {
		t.Fatal("SetHostKeyVerify(true) accepted an unknown host key (should fail-closed)")
	}

	SetHostKeyVerify(false)
	if err := hostKeyCallback(hostname, remote, hostPub); err != nil {
		t.Fatalf("SetHostKeyVerify(false) rejected a host key: %v", err)
	}
}
