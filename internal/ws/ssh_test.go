// Package ws — ssh_test.go: tests for the SSH stdout pump race (C1) and
// host-key verification hardening (S10).
package ws

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// fakeStdout yields data on every Read without blocking or EOF, so the pump
// loops fast and the test can observe cancellation via the done channel.
type fakeStdout struct{ data []byte }

func (f *fakeStdout) Read(p []byte) (int, error) {
	return copy(p, f.data), nil
}

// TestSSH_PumpStdout_NoRace exercises -race: the stdout pump writes binary
// frames via safeConn while concurrent "main loop" writers write text frames
// on the same conn. Without per-conn write serialization this races; with
// safeConn it must not panic or report a race.
func TestSSH_PumpStdout_NoRace(t *testing.T) {
	client, server := newConnPair(t)
	// Drain so the server's send buffer never fills and writes never block.
	go func() {
		for {
			if _, _, err := client.ReadMessage(); err != nil {
				return
			}
		}
	}()
	sc := &safeConn{c: server}
	stdout := &fakeStdout{data: []byte("stdout!")}
	done := make(chan struct{})

	var pumpExited sync.WaitGroup
	pumpExited.Add(1)
	go func() {
		defer pumpExited.Done()
		pumpStdout(sc, stdout, done)
	}()

	// Concurrent "main loop" writers on the same conn.
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_ = sc.writeMessage(1, []byte("main")) // TextMessage
			}
		}()
	}
	wg.Wait()

	// Close done → pump must exit.
	close(done)
	doneCh := make(chan struct{})
	go func() { pumpExited.Wait(); close(doneCh) }()
	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("stdout pump did not exit after done closed")
	}
}

// TestSSH_PumpStdout_ExitsOnStdoutError verifies the pump returns when stdout
// errors (the production exit path: session close unblocks stdout.Read).
func TestSSH_PumpStdout_ExitsOnStdoutError(t *testing.T) {
	client, server := newConnPair(t)
	go func() {
		for {
			if _, _, err := client.ReadMessage(); err != nil {
				return
			}
		}
	}()
	sc := &safeConn{c: server}
	stdout := io.NopCloser(errReader{})
	done := make(chan struct{})

	exited := make(chan struct{})
	go func() { pumpStdout(sc, stdout, done); close(exited) }()
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("pump did not exit on stdout error")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.EOF }

// TestSSH_HostKeyVerification (S10) verifies the default secure host-key
// callback: a known host key is accepted, an unknown one is rejected (MITM
// defense), a missing known_hosts file fails closed, and the legacy insecure
// callback accepts anything.
func TestSSH_HostKeyVerification(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hostPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	pub2, _, _ := ed25519.GenerateKey(rand.Reader)
	hostPub2, _ := ssh.NewPublicKey(pub2)

	hostname := "127.0.0.1:22"
	remote := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}

	// Write a known_hosts entry for the known key.
	tmp := t.TempDir()
	knownHosts := filepath.Join(tmp, "known_hosts")
	line := knownhosts.Line([]string{hostname}, hostPub)
	if err := os.WriteFile(knownHosts, []byte(line+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cb := newHostKeyCallback(knownHosts)
	if err := cb(hostname, remote, hostPub); err != nil {
		t.Fatalf("default secure callback rejected a KNOWN host key: %v", err)
	}
	if err := cb(hostname, remote, hostPub2); err == nil {
		t.Fatal("default secure callback accepted an UNKNOWN host key (MITM risk)")
	}

	// Missing known_hosts → fail-closed (default secure).
	cbFail := newHostKeyCallback(filepath.Join(tmp, "missing-known_hosts"))
	if err := cbFail(hostname, remote, hostPub); err == nil {
		t.Fatal("fail-closed default did not reject when known_hosts is missing")
	}

	// Verification disabled (legacy deployments) → any key accepted.
	if err := ssh.InsecureIgnoreHostKey()(hostname, remote, hostPub2); err != nil {
		t.Fatal("insecure callback rejected a key")
	}
}
