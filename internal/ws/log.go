// Package ws — log.go: real-time log tail over WebSocket (SPEC S11).
// Port of LogWebSocketHandler.java. On connect, sends last 100 lines via
// "tail -n 100". Then polls the log file every 1s and broadcasts new lines
// to all connected sessions. Stops when the last session disconnects.
package ws

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/gorilla/websocket"
)

// LogHandler tails the application log file and streams to WS clients.
type LogHandler struct {
	mu       sync.Mutex
	sessions map[*safeConn]struct{}
	running  bool
	stopCh   chan struct{}
	logPath  string
}

// NewLogHandler creates a LogHandler. logPath defaults to "logs/application.log".
func NewLogHandler() *LogHandler {
	path := "logs/application.log"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = "application.log" // fallback
	}
	return &LogHandler{logPath: path}
}

// HandleLog upgrades the HTTP connection and streams log tail.
func (h *LogHandler) HandleLog(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	// ponytail: per-conn write serialization; broadcast (tailLoop) and this
	// goroutine (sendRecentLogs) both write through sc.
	sc := &safeConn{c: conn}
	defer h.removeSession(sc)

	h.addSession(sc)

	// Send last 100 lines.
	h.sendRecentLogs(sc)

	// Read loop (client may send ping, but mostly we just push).
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (h *LogHandler) addSession(sc *safeConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sessions == nil {
		h.sessions = make(map[*safeConn]struct{})
	}
	h.sessions[sc] = struct{}{}
	if len(h.sessions) == 1 && !h.running {
		h.running = true
		h.stopCh = make(chan struct{})
		go h.tailLoop()
	}
}

func (h *LogHandler) removeSession(sc *safeConn) {
	h.mu.Lock()
	if h.sessions != nil {
		delete(h.sessions, sc)
	}
	remaining := len(h.sessions)
	h.mu.Unlock()
	if remaining == 0 {
		h.mu.Lock()
		if h.running {
			h.running = false
			close(h.stopCh)
		}
		h.mu.Unlock()
	}
}

func (h *LogHandler) sendRecentLogs(sc *safeConn) {
	cmd := exec.Command("tail", "-n", "100", h.logPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		_ = sc.writeMessage(websocket.TextMessage, scanner.Bytes())
	}
	cmd.Wait()
}

func (h *LogHandler) tailLoop() {
	file, err := os.Open(h.logPath)
	if err != nil {
		return
	}
	defer file.Close()

	// Seek to end.
	file.Seek(0, io.SeekEnd)

	reader := bufio.NewReader(file)
	for {
		select {
		case <-h.stopCh:
			return
		default:
		}
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			// Check if file was truncated.
			if fi, statErr := os.Stat(h.logPath); statErr == nil {
				if pos, _ := file.Seek(0, io.SeekCurrent); pos > fi.Size() {
					file.Seek(0, io.SeekStart)
				}
			}
			// Sleep briefly, then retry.
			select {
			case <-h.stopCh:
				return
			default:
			}
			continue
		}
		if err != nil {
			return
		}
		if line != "" {
			h.broadcast([]byte(line))
		}
	}
}

// broadcast sends a log line to all connected sessions. It snapshots the
// session set under a brief lock, then writes each conn outside the lock via
// safeConn (with a write deadline). A conn whose write fails is dropped so a
// dead/slow peer cannot block or starve others.
func (h *LogHandler) broadcast(data []byte) {
	h.mu.Lock()
	conns := make([]*safeConn, 0, len(h.sessions))
	for sc := range h.sessions {
		conns = append(conns, sc)
	}
	h.mu.Unlock()

	for _, sc := range conns {
		if err := sc.writeMessage(websocket.TextMessage, data); err != nil {
			h.removeSession(sc) // drop dead conn so it doesn't block future broadcasts
		}
	}
}

// Shutdown stops the log tail and closes all sessions.
func (h *LogHandler) Shutdown() {
	h.mu.Lock()
	if h.running {
		h.running = false
		close(h.stopCh)
	}
	for sc := range h.sessions {
		_ = sc.c.Close()
	}
	h.sessions = nil
	h.mu.Unlock()
}
