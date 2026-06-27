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
	sessions map[*websocket.Conn]struct{}
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
	defer h.removeSession(conn)

	h.addSession(conn)

	// Send last 100 lines.
	h.sendRecentLogs(conn)

	// Read loop (client may send ping, but mostly we just push).
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (h *LogHandler) addSession(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sessions == nil {
		h.sessions = make(map[*websocket.Conn]struct{})
	}
	h.sessions[conn] = struct{}{}
	if len(h.sessions) == 1 && !h.running {
		h.running = true
		h.stopCh = make(chan struct{})
		go h.tailLoop()
	}
}

func (h *LogHandler) removeSession(conn *websocket.Conn) {
	h.mu.Lock()
	if h.sessions != nil {
		delete(h.sessions, conn)
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

func (h *LogHandler) sendRecentLogs(conn *websocket.Conn) {
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
		conn.WriteMessage(websocket.TextMessage, scanner.Bytes())
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

func (h *LogHandler) broadcast(data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.sessions {
		conn.WriteMessage(websocket.TextMessage, data)
	}
}

// Shutdown stops the log tail and closes all sessions.
func (h *LogHandler) Shutdown() {
	h.mu.Lock()
	if h.running {
		h.running = false
		close(h.stopCh)
	}
	for conn := range h.sessions {
		conn.Close()
	}
	h.sessions = nil
	h.mu.Unlock()
}
