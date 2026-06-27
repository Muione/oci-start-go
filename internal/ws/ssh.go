// Package ws — ssh.go: SSH-over-WebSocket (SPEC S12.1).
// Port of SshWebSocketHandler.java. Uses x/crypto/ssh (replaces JSch).
// Browser sends JSON: {"type":"connect","data":{host,port,username,password}}
// {"type":"input","data":"..."}, {"type":"resize","data":{cols,rows}}.
package ws

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// SSHHandler manages SSH sessions tunneled over WebSocket.
type SSHHandler struct {
	mu       sync.Mutex
	sessions map[string]*sshSession
}

type sshSession struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
}

// HandleSSH upgrades HTTP → WS and runs the SSH-over-WS loop.
func (h *SSHHandler) HandleSSH(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	defer h.cleanup(conn)

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
			continue
		}
		switch req.Type {
		case "connect":
			var d struct {
				Host     string `json:"host"`
				Port     int    `json:"port"`
				Username string `json:"username"`
				Password string `json:"password"`
			}
			if err := json.Unmarshal(req.Data, &d); err != nil {
				conn.WriteMessage(websocket.TextMessage, []byte("invalid connect data"))
				continue
			}
			if d.Port == 0 {
				d.Port = 22
			}
			h.handleConnect(conn, d)
		case "input":
			var d struct{ Data string `json:"data"` }
			json.Unmarshal(req.Data, &d)
			h.handleInput(conn, d.Data)
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

func (h *SSHHandler) handleConnect(conn *websocket.Conn, d struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}) {
	addr := net.JoinHostPort(d.Host, fmt.Sprintf("%d", d.Port))
	config := &ssh.ClientConfig{
		User:            d.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(d.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("\r\nSSH conn error: "+err.Error()+"\r\n"))
		return
	}
	session, err := client.NewSession()
	if err != nil {
		client.Close()
		conn.WriteMessage(websocket.TextMessage, []byte("\r\nSSH session error: "+err.Error()+"\r\n"))
		return
	}
	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
	if err := session.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		session.Close()
		client.Close()
		conn.WriteMessage(websocket.TextMessage, []byte("\r\nPTY error: "+err.Error()+"\r\n"))
		return
	}
	stdin, _ := session.StdinPipe()
	stdout, _ := session.StdoutPipe()
	if err := session.Shell(); err != nil {
		session.Close()
		client.Close()
		return
	}
	key := conn.RemoteAddr().String()
	h.mu.Lock()
	if h.sessions == nil {
		h.sessions = make(map[string]*sshSession)
	}
	h.sessions[key] = &sshSession{client: client, session: session, stdin: stdin}
	h.mu.Unlock()
	conn.WriteMessage(websocket.TextMessage, []byte("\r\nSSH conn success\r\n"))
	// Read stdout → WebSocket in background.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				conn.WriteMessage(websocket.BinaryMessage, buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()
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
