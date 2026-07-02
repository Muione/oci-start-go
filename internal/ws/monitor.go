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
	sessions map[*safeConn]struct{}
}

// NewMonitorHandler creates a MonitorHandler.
func NewMonitorHandler() *MonitorHandler {
	return &MonitorHandler{sessions: make(map[*safeConn]struct{})}
}

// HandleMonitor upgrades HTTP → WS for monitor dashboard connections.
func (h *MonitorHandler) HandleMonitor(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	// ponytail: per-conn write serialization; Broadcast and this read loop both
	// write through sc so they cannot race a gorilla write.
	sc := &safeConn{c: conn}
	defer h.remove(sc)

	h.add(sc)

	// Send welcome.
	_ = sc.writeMessage(websocket.TextMessage, []byte(`{"type":"system","message":"connected, waiting for data..."}`))

	// Read loop: respond to ping.
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		// Respond to ping.
		if string(msg) == "ping" || string(msg) == `"ping"` {
			_ = sc.writeMessage(websocket.TextMessage, []byte(`{"type":"pong","message":"pong"}`))
		}
	}
}

func (h *MonitorHandler) add(sc *safeConn) {
	h.mu.Lock()
	h.sessions[sc] = struct{}{}
	h.mu.Unlock()
}

func (h *MonitorHandler) remove(sc *safeConn) {
	h.mu.Lock()
	delete(h.sessions, sc)
	h.mu.Unlock()
}

// Broadcast sends a MonitorReportDTO to all connected dashboard sessions.
// It snapshots the session set under a brief read lock, then writes each conn
// outside the lock via safeConn (with a write deadline). A conn whose write
// fails is dropped so a dead peer cannot block or starve others.
func (h *MonitorHandler) Broadcast(report MonitorReportDTO) {
	data, err := json.Marshal(report)
	if err != nil {
		return
	}
	h.mu.RLock()
	conns := make([]*safeConn, 0, len(h.sessions))
	for sc := range h.sessions {
		conns = append(conns, sc)
	}
	h.mu.RUnlock()

	for _, sc := range conns {
		if err := sc.writeMessage(websocket.TextMessage, data); err != nil {
			h.remove(sc) // drop dead conn so it doesn't block future broadcasts
		}
	}
}

// OnlineCount returns the number of connected dashboard sessions.
func (h *MonitorHandler) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions)
}
