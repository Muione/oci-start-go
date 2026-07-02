// Package ws implements WebSocket endpoints (SPEC SS11-12). Each handler
// is a self-contained gorilla/websocket implementation. The Hub holds
// per-endpoint session maps for broadcast. Upgrader enforces same-origin
// by default (CSWSH defense; main.go may override Upgrader.CheckOrigin).
package ws

import (
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// writeTimeout caps how long a single conn write may block, so a slow/dead
// client cannot stall the writer goroutine. Overridable in tests.
var writeTimeout = 10 * time.Second

// safeConn serializes writes to a single gorilla/websocket.Conn. gorilla
// forbids concurrent writes to the same conn (panics / corrupts frames);
// every handler routes its writes through this wrapper. Each write also sets
// a fresh write deadline so a stalled peer cannot block the writer forever.
type safeConn struct {
	c  *websocket.Conn
	mu sync.Mutex
}

// writeMessage sends a framed message under the conn's write deadline.
func (s *safeConn) writeMessage(msgType int, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.c.SetWriteDeadline(time.Now().Add(writeTimeout))
	return s.c.WriteMessage(msgType, data)
}

// writeJSON marshals v and sends it as a text frame under the write deadline.
func (s *safeConn) writeJSON(v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.c.SetWriteDeadline(time.Now().Add(writeTimeout))
	return s.c.WriteJSON(v)
}

// checkOrigin is the default WebSocket origin check. main.go may override
// ws.Upgrader.CheckOrigin (the Upgrader is exported) for an explicit
// allowlist, or replace this var via a setter if one is added in batch3.
var checkOrigin = defaultCheckOrigin

// defaultCheckOrigin enforces same-origin: the Origin header's host must match
// the request Host. Requests with no Origin (non-browser clients) are allowed.
// ponytail: u.Host == r.Host covers typical cases (default ports are stripped
// by url.Parse; non-default ports appear in both). A port mismatch on default
// ports would false-negative toward rejection — the secure side to err on.
func defaultCheckOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // non-browser client; no Origin to spoof
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return u.Host == r.Host
}

// Upgrader configures the websocket upgrade. CheckOrigin defaults to
// same-origin enforcement; main.go may override it for an allowlist.
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return checkOrigin(r) },
}

// SetHostKeyVerify configures SSH-over-WS server host-key verification.
// enabled=true (default, secure) verifies against known_hosts and fails closed
// when the file is missing (MITM defense); enabled=false accepts any host key
// (insecure, for legacy deployments without a known_hosts file). Call once at
// boot before the HTTP server accepts SSH-WS connections — handleConnect reads
// the package callback fresh per connection, and goroutine-creation ordering
// publishes the write to connection goroutines.
func SetHostKeyVerify(enabled bool) {
	if enabled {
		hostKeyCallback = newHostKeyCallback(knownHostsPath)
	} else {
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}
}

// Hub holds all WebSocket handlers and shared session maps.
type Hub struct {
	SSH     *SSHHandler
	Log     *LogHandler
	Monitor *MonitorHandler
	Console *ConsoleHandler
	Rescue  *RescueHandler
}

// NewHub creates all WebSocket handlers.
func NewHub() *Hub {
	return &Hub{
		SSH:     &SSHHandler{},
		Log:     NewLogHandler(),
		Monitor: NewMonitorHandler(),
		Console: &ConsoleHandler{},
		Rescue:  &RescueHandler{},
	}
}

// Shutdown cleans up all handlers (closes SSH sessions, stops log tail, kills
// console tunnels + deletes PEM keys, cancels rescue flows, etc.).
func (h *Hub) Shutdown() {
	h.SSH.Shutdown()
	h.Log.Shutdown()
	if h.Console != nil {
		h.Console.Shutdown()
	}
	if h.Rescue != nil {
		h.Rescue.Shutdown()
	}
}
