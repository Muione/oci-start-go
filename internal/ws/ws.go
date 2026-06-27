// Package ws implements WebSocket endpoints (SPEC SS11-12). Each handler
// is a self-contained gorilla/websocket implementation. The Hub holds
// per-endpoint session maps for broadcast. Upgrader allows all origins
// (parity with Java AllowedOrigins("*")).
package ws

import (
	"net/http"

	"github.com/gorilla/websocket"
)

// Upgrader configures the websocket upgrade. AllowedOrigins("*") matches
// the Java WebSocket config (AllowedOrigins("*")).
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
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

// Shutdown cleans up all handlers (closes SSH sessions, stops log tail, etc.).
func (h *Hub) Shutdown() {
	h.SSH.Shutdown()
	h.Log.Shutdown()
}
