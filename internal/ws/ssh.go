// Package ws — ssh.go: SSH-over-WebSocket (SPEC S12.1).
// Port of SshWebSocketHandler.java. Uses x/crypto/ssh (replaces JSch).
// Browser sends JSON: {"type":"connect","data":{host,port,username,password}}
// {"type":"input","data":"..."}, {"type":"resize","data":{cols,rows}}.
package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// knownHostsPath is the OpenSSH known_hosts file used to verify SSH server
// host keys. Overrideable (tests/main.go) before hostKeyCallback is consumed.
var knownHostsPath = filepath.Join("data", "known_hosts")

// hostKeyCallback verifies the SSH server's host key against known_hosts.
// Default is fail-closed (secure): if the file is missing/unreadable, every
// host is refused rather than silently accepting a possible MITM. main.go
// (batch3) may override this — e.g. ssh.InsecureIgnoreHostKey when config
// ssh.host_key_verify=false for legacy deployments without a known_hosts file.
var hostKeyCallback ssh.HostKeyCallback = newHostKeyCallback(knownHostsPath)

// newHostKeyCallback builds a known_hosts-verifying callback for path. On any
// error (missing file, parse error) it returns a fail-closed callback so the
// connection is refused instead of running unverified.
func newHostKeyCallback(path string) ssh.HostKeyCallback {
	cb, err := knownhosts.New(path)
	if err != nil {
		return func(_ string, _ net.Addr, _ ssh.PublicKey) error {
			return fmt.Errorf("ssh: known_hosts %q unavailable, refusing connection (set ssh.host_key_verify=false to allow insecure)", path)
		}
	}
	return cb
}

// SSHHandler manages SSH sessions tunneled over WebSocket.
type SSHHandler struct {
	mu     sync.Mutex
	deps   *SSHDeps
	sessions map[string]*sshSession
}

// SSHDeps holds runtime dependencies. ResolveSSHKey looks up a stored private
// key (by DB id) + its passphrase, decrypted, so the WS handler can auth with
// a saved key without the key material ever reaching the frontend. nil/zero
// falls back to an ad-hoc privateKey in the connect message.
type SSHDeps struct {
	Logger         zerolog.Logger
	ResolveSSHKey func(ctx context.Context, keyID int64) (content, passphrase string, err error)
}

// SetDeps injects runtime dependencies.
func (h *SSHHandler) SetDeps(deps *SSHDeps) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.deps = deps
}

type sshSession struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	done    chan struct{} // closed by cleanup to unblock the stdout pump
}

// HandleSSH upgrades HTTP → WS and runs the SSH-over-WS loop.
func (h *SSHHandler) HandleSSH(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	defer h.cleanup(conn)

	// ponytail: every write to this conn goes through safeConn so the stdout
	// pump goroutine and this read loop cannot race a gorilla write.
	sc := &safeConn{c: conn}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var req struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(msg, &req); err != nil {
			sc.writeMessage(websocket.TextMessage, []byte("invalid JSON"))
			continue
		}
		switch req.Type {
		case "connect":
			var d struct {
				Host       string `json:"host"`
				Port       int    `json:"port"`
				Username   string `json:"username"`
				Password   string `json:"password"`
				PrivateKey string `json:"privateKey"`
				Passphrase string `json:"passphrase"`
				KeyID      int64  `json:"keyId"`
			}
			if err := json.Unmarshal(req.Data, &d); err != nil {
				sc.writeMessage(websocket.TextMessage, []byte("invalid connect data"))
				continue
			}
			if d.Port == 0 {
				d.Port = 22
			}
			h.handleConnect(sc, d)
		case "input":
			s, err := parseInputData(req.Data)
			if err != nil {
				continue
			}
			h.handleInput(conn, s)
		case "resize":
			var d struct {
				Cols int `json:"cols"`
				Rows int `json:"rows"`
			}
			json.Unmarshal(req.Data, &d)
			h.handleResize(conn, d.Cols, d.Rows)
		}
	}
}

// buildAuth builds the SSH auth methods for a connect request: private-key
// (PublicKeys) when a key is provided (key takes precedence over password),
// otherwise password. Passphrase is used only with a key (ParsePrivateKeyWith
// Passphrase). Extracted from handleConnect so the selection + parse-error
// handling is unit-testable without a real SSH dial.
func buildAuth(password, privateKey, passphrase string) ([]ssh.AuthMethod, error) {
	if privateKey == "" {
		return []ssh.AuthMethod{ssh.Password(password)}, nil
	}
	var signer ssh.Signer
	var err error
	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(privateKey), []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey([]byte(privateKey))
	}
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
}

// resolveKeyAuth resolves a saved key by id (via deps, decrypted server-side)
// or falls back to an ad-hoc privateKey, then builds the auth methods. Key id
// takes precedence over privateKey. Extracted from handleConnect so the
// keyId-resolution path is unit-testable without a real SSH dial.
func resolveKeyAuth(deps *SSHDeps, password, privateKey, passphrase string, keyID int64) ([]ssh.AuthMethod, error) {
	if keyID != 0 && deps != nil && deps.ResolveSSHKey != nil {
		content, pass, err := deps.ResolveSSHKey(context.Background(), keyID)
		if err != nil {
			return nil, fmt.Errorf("resolve ssh key %d: %w", keyID, err)
		}
		return buildAuth(password, content, pass)
	}
	return buildAuth(password, privateKey, passphrase)
}

// parseInputData extracts the keystroke string from the `data` field of an
// {"type":"input","data":"..."} message. The data field is a JSON STRING (the
// keystrokes — including escape sequences like "\r" or "\x1b[A" for arrows),
// NOT an object. Unmarshaling into struct{Data string} (the old code) silently
// failed on a raw string → empty input → typing did nothing.
func parseInputData(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}

func (h *SSHHandler) handleConnect(sc *safeConn, d struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	PrivateKey string `json:"privateKey"`
	Passphrase string `json:"passphrase"`
	KeyID      int64  `json:"keyId"`
}) {
	addr := net.JoinHostPort(d.Host, fmt.Sprintf("%d", d.Port))
	auth, err := resolveKeyAuth(h.deps, d.Password, d.PrivateKey, d.Passphrase, d.KeyID)
	if err != nil {
		sc.writeMessage(websocket.TextMessage, []byte("\r\n"+err.Error()+"\r\n"))
		return
	}
	config := &ssh.ClientConfig{
		User:            d.Username,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
	}
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		sc.writeMessage(websocket.TextMessage, []byte("\r\nSSH conn error: "+err.Error()+"\r\n"))
		return
	}
	session, err := client.NewSession()
	if err != nil {
		client.Close()
		sc.writeMessage(websocket.TextMessage, []byte("\r\nSSH session error: "+err.Error()+"\r\n"))
		return
	}
	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
	if err := session.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		session.Close()
		client.Close()
		sc.writeMessage(websocket.TextMessage, []byte("\r\nPTY error: "+err.Error()+"\r\n"))
		return
	}
	stdin, _ := session.StdinPipe()
	stdout, _ := session.StdoutPipe()
	if err := session.Shell(); err != nil {
		session.Close()
		client.Close()
		return
	}
	key := sc.c.RemoteAddr().String()
	done := make(chan struct{})
	h.mu.Lock()
	if h.sessions == nil {
		h.sessions = make(map[string]*sshSession)
	}
	h.sessions[key] = &sshSession{client: client, session: session, stdin: stdin, done: done}
	h.mu.Unlock()
	sc.writeMessage(websocket.TextMessage, []byte("\r\nSSH conn success\r\n"))
	// Read stdout → WebSocket in background; exits on session close or done.
	go pumpStdout(sc, stdout, done)
}

// pumpStdout forwards SSH stdout to the websocket as binary frames. Writes are
// serialized through safeConn so they cannot race with the main read loop.
// Exits when stdout errors (session closed) or done is closed.
func pumpStdout(sc *safeConn, stdout io.Reader, done <-chan struct{}) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-done:
			return
		default:
		}
		n, err := stdout.Read(buf)
		if n > 0 {
			_ = sc.writeMessage(websocket.BinaryMessage, buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func (h *SSHHandler) handleInput(conn *websocket.Conn, input string) {
	h.mu.Lock()
	ss, ok := h.sessions[conn.RemoteAddr().String()]
	h.mu.Unlock()
	if ok && ss.stdin != nil {
		ss.stdin.Write([]byte(input))
	}
}

func (h *SSHHandler) handleResize(conn *websocket.Conn, cols, rows int) {
	h.mu.Lock()
	ss, ok := h.sessions[conn.RemoteAddr().String()]
	h.mu.Unlock()
	if ok && ss.session != nil {
		ss.session.WindowChange(rows, cols)
	}
}

func (h *SSHHandler) cleanup(conn *websocket.Conn) {
	key := conn.RemoteAddr().String()
	h.mu.Lock()
	ss, ok := h.sessions[key]
	if ok {
		delete(h.sessions, key)
	}
	h.mu.Unlock()
	if ok {
		if ss.done != nil {
			close(ss.done) // unblock the stdout pump
		}
		if ss.session != nil {
			ss.session.Close()
		}
		if ss.client != nil {
			ss.client.Close()
		}
	}
}

// Shutdown closes all active SSH sessions.
func (h *SSHHandler) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ss := range h.sessions {
		ss.session.Close()
		ss.client.Close()
	}
	h.sessions = nil
}
