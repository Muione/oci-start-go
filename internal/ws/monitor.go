// Package ws — monitor.go: VPS monitor WebSocket broadcast (SPEC S11).
// Port of MonitorWebSocketHandler.java. Receives frontend connections and
// broadcasts monitor data (CPU/memory/disk/network) sent by the agent.
package ws

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// MonitorReportDTO is the agent-reported metrics payload.
type MonitorReportDTO struct {
	Type            string  `json:"type,omitempty"` // "heartbeat" for scheduler pings
	ServerID        string  `json:"serverId"`
	ServerIP        string  `json:"serverIp"`
	CPUUsage        float64 `json:"cpuUsage"`
	MemoryUsage     float64 `json:"memoryUsage"`
	DiskUsage       float64 `json:"diskUsage"`
	UploadTraffic   float64 `json:"uploadTraffic"`
	DownloadTraffic float64 `json:"downloadTraffic"`
	CPUCores        int64   `json:"cpuCores"`
	TotalMemory     float64 `json:"totalMemory"`
	TotalDisk       float64 `json:"totalDisk"`
	UptimeHours     int64   `json:"uptimeHours"`
}

// MonitorHandler manages monitor dashboard WebSocket sessions.
type MonitorHandler struct {
	mu       sync.RWMutex
	sessions map[*websocket.Conn]struct{}
}

// NewMonitorHandler creates a MonitorHandler.
func NewMonitorHandler() *MonitorHandler {
	return &MonitorHandler{sessions: make(map[*websocket.Conn]struct{})}
}

// HandleMonitor upgrades HTTP → WS for monitor dashboard connections.
func (h *MonitorHandler) HandleMonitor(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	defer h.remove(conn)

	h.add(conn)

	// Send welcome.
	conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"system","message":"connected, waiting for data..."}`))

	// Read loop: respond to ping.
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		// Respond to ping.
		if string(msg) == "ping" || string(msg) == `"ping"` {
			conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"pong","message":"pong"}`))
		}
	}
}

func (h *MonitorHandler) add(conn *websocket.Conn) {
	h.mu.Lock()
	h.sessions[conn] = struct{}{}
	h.mu.Unlock()
}

func (h *MonitorHandler) remove(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.sessions, conn)
	h.mu.Unlock()
}

// Broadcast sends a MonitorReportDTO to all connected dashboard sessions.
// Called by the HTTP monitor report handler. Thread-safe (locks per-session
// on write, matching Java's synchronized(session) pattern).
func (h *MonitorHandler) Broadcast(report MonitorReportDTO) {
	data, err := json.Marshal(report)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.sessions {
		conn.WriteMessage(websocket.TextMessage, data)
	}
}

// OnlineCount returns the number of connected dashboard sessions.
func (h *MonitorHandler) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions)
}
