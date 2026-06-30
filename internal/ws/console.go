// Package ws — console.go: OCI instance VNC console over WebSocket.
// Uses OCI Console Connections (proxy tunnel) instead of direct SSH, so VNC
// works for ALL instances regardless of public/private IP assignment.
//
// Flow:
//  1. Frontend sends "create_connection" over /ws/console (control WS).
//  2. Backend creates/finds OCI console connection, starts SSH tunnel.
//  3. Backend sends "vnc_ready" with the binary VNC WebSocket URL.
//  4. Frontend opens /ws/vnc/:instanceId for raw VNC binary frames.
//  5. "disconnect" tears down tunnel + cleans up.
package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/util/sshkeygen"
)

// ConsoleDeps holds the dependencies needed by the console handler.
type ConsoleDeps struct {
	Logger    zerolog.Logger
	MasterKey []byte

	// BuildClients creates OCI clients for a given tenant ID.
	BuildClients func(ctx context.Context, tenantID int64) (oci.Clients, error)

	// GetCompartmentID returns the compartment ID for an instance.
	GetCompartmentID func(ctx context.Context, tenantID int64, instanceID string) (string, error)

	// InstanceLookup fetches instance metadata from the DB.
	InstanceLookup func(instanceID string) (*ConsoleInstanceInfo, error)
}

// ConsoleInstanceInfo holds the minimum instance info needed for VNC console.
type ConsoleInstanceInfo struct {
	InstanceID         string
	DisplayName        string
	PublicIPs          string
	PrivateIPs         string
	Shape              string
	Username           string
	Port               string
	Password           string
	TenantID           int64
	CompartmentID      string
	AvailabilityDomain string
	ImageID            string
}

// ConsoleHandler handles OCI instance VNC console over WebSocket.
type ConsoleHandler struct {
	mu     sync.Mutex
	deps   *ConsoleDeps
	active map[string]*consoleSession // instanceID -> active session
}

// consoleSession tracks an active VNC console session.
type consoleSession struct {
	instanceID     string
	connID         string
	sshCmd         *exec.Cmd
	localPort      int
	keyFilePath    string
	cancel         chan struct{}
	controlWS      *websocket.Conn // the control WebSocket (/ws/console)
}

// SetDeps injects runtime dependencies.
func (h *ConsoleHandler) SetDeps(deps *ConsoleDeps) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.active == nil {
		h.active = make(map[string]*consoleSession)
	}
	h.deps = deps
}

// HandleConsole upgrades HTTP -> WS for VNC console control messages.
func (h *ConsoleHandler) HandleConsole(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

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
			conn.WriteJSON(map[string]string{"type": "error", "message": "invalid JSON"})
			continue
		}

		switch req.Type {
		case "ping", "heartbeat":
			conn.WriteJSON(map[string]string{"type": "pong"})

		case "create_connection":
			h.handleCreateConnection(conn, req.Data)

		case "disconnect":
			h.handleDisconnect(conn, req.Data)
			return

		default:
			conn.WriteJSON(map[string]string{"type": "error", "message": "unknown command: " + req.Type})
		}
	}
}

// handleCreateConnection creates an OCI console connection, starts an SSH
// tunnel, and notifies the frontend with the VNC WebSocket URL.
func (h *ConsoleHandler) handleCreateConnection(conn *websocket.Conn, data json.RawMessage) {
	var d struct {
		InstanceID string `json:"instanceId"`
		TenantID   int64  `json:"tenantId"`
	}
	if err := json.Unmarshal(data, &d); err != nil || d.InstanceID == "" {
		conn.WriteJSON(map[string]string{"type": "error", "message": "instanceId required"})
		return
	}

	h.mu.Lock()
	deps := h.deps
	h.mu.Unlock()

	if deps == nil {
		conn.WriteJSON(map[string]string{"type": "error", "message": "console handler not configured"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 1. Get instance info for compartment ID.
	info, err := deps.InstanceLookup(d.InstanceID)
	if err != nil {
		conn.WriteJSON(map[string]string{"type": "error", "message": "instance lookup failed: " + err.Error()})
		return
	}

	compartmentID := info.CompartmentID
	if compartmentID == "" {
		// Fallback: get compartment from tenant's tenancy.
		compartmentID, err = deps.GetCompartmentID(ctx, info.TenantID, d.InstanceID)
		if err != nil {
			conn.WriteJSON(map[string]string{"type": "error", "message": "get compartment failed: " + err.Error()})
			return
		}
	}

	// 2. Build OCI clients.
	clients, err := deps.BuildClients(ctx, info.TenantID)
	if err != nil {
		conn.WriteJSON(map[string]string{"type": "error", "message": "build OCI clients failed: " + err.Error()})
		return
	}

	// 3. Check for existing active console connection.
	existing, err := oci.FindActiveConsoleConnection(ctx, clients, compartmentID, d.InstanceID)
	if err != nil {
		deps.Logger.Warn().Err(err).Msg("failed to check existing console connections")
	}

	var connInfo *oci.ConsoleConnectionInfo
	var keyPair *sshkeygen.KeyPair

	if existing != nil {
		// Reuse existing connection.
		connInfo = existing
		deps.Logger.Info().Str("instanceId", d.InstanceID).Str("connId", existing.ID).
			Msg("reusing existing console connection")
	} else {
		// 4. Generate SSH key pair.
		keyPair, err = sshkeygen.GenerateRSA2048()
		if err != nil {
			conn.WriteJSON(map[string]string{"type": "error", "message": "generate SSH key failed: " + err.Error()})
			return
		}

		// 5. Create console connection.
		connInfo, err = oci.GenerateConsoleConnection(ctx, clients, d.InstanceID, keyPair.PublicKeySSH)
		if err != nil {
			conn.WriteJSON(map[string]string{"type": "error", "message": "create console connection failed: " + err.Error()})
			return
		}

		// 6. Wait for connection to become ACTIVE.
		connInfo, err = oci.WaitForConnectionActive(ctx, clients, connInfo.ID, d.InstanceID, 90*time.Second)
		if err != nil {
			conn.WriteJSON(map[string]string{"type": "error", "message": "console connection activation failed: " + err.Error()})
			return
		}
	}

	// 7. Parse connection string.
	if connInfo.ConnectionString == "" {
		conn.WriteJSON(map[string]string{"type": "error", "message": "empty connection string from console connection"})
		return
	}

	parsed, err := oci.ParseConnectionString(connInfo.ConnectionString)
	if err != nil {
		conn.WriteJSON(map[string]string{"type": "error", "message": "parse connection string failed: " + err.Error()})
		return
	}

	// 8. Save private key to file (needed for existing connections too —
	//    generate a fresh key pair if we reused a connection without keys).
	privateKeyPEM := ""
	if keyPair != nil {
		privateKeyPEM = keyPair.PrivateKeyPEM
	}
	if privateKeyPEM == "" {
		// For reused connections we need a fresh key — but OCI console
		// connections are bound to the key at creation time. If we don't
		// have the key, we must create a new connection.
		keyPair, err = sshkeygen.GenerateRSA2048()
		if err != nil {
			conn.WriteJSON(map[string]string{"type": "error", "message": "generate SSH key failed: " + err.Error()})
			return
		}
		// Delete old connection and create new one.
		_ = oci.CleanupConsoleConnections(ctx, clients, compartmentID, d.InstanceID)
		connInfo, err = oci.GenerateConsoleConnection(ctx, clients, d.InstanceID, keyPair.PublicKeySSH)
		if err != nil {
			conn.WriteJSON(map[string]string{"type": "error", "message": "create console connection failed: " + err.Error()})
			return
		}
		connInfo, err = oci.WaitForConnectionActive(ctx, clients, connInfo.ID, d.InstanceID, 90*time.Second)
		if err != nil {
			conn.WriteJSON(map[string]string{"type": "error", "message": "console connection activation failed: " + err.Error()})
			return
		}
		parsed, err = oci.ParseConnectionString(connInfo.ConnectionString)
		if err != nil {
			conn.WriteJSON(map[string]string{"type": "error", "message": "parse connection string failed: " + err.Error()})
			return
		}
		privateKeyPEM = keyPair.PrivateKeyPEM
	}

	keyFilePath, err := savePrivateKeyFile(d.InstanceID, privateKeyPEM)
	if err != nil {
		conn.WriteJSON(map[string]string{"type": "error", "message": "save key file failed: " + err.Error()})
		return
	}

	// 9. Allocate local port for VNC tunnel.
	localListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Remove(keyFilePath)
		conn.WriteJSON(map[string]string{"type": "error", "message": "allocate local port failed: " + err.Error()})
		return
	}
	localPort := localListener.Addr().(*net.TCPAddr).Port
	localListener.Close() // Release immediately; SSH tunnel will bind.

	// 10. Start SSH tunnel process.
	tunnelCfg := oci.SSHTunnelConfig{
		PrivateKeyPath: keyFilePath,
		ConnectionID:   parsed.ConnectionID,
		ProxyHost:      parsed.ProxyHost,
		TargetHost:     parsed.TargetHost,
		LocalPort:      localPort,
		RemotePort:     5900, // VNC default
	}
	sshArgs := oci.BuildSSHTunnelCommand(tunnelCfg)

	sshCmd := exec.Command(sshArgs[0], sshArgs[1:]...)
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	if err := sshCmd.Start(); err != nil {
		os.Remove(keyFilePath)
		conn.WriteJSON(map[string]string{"type": "error", "message": "start SSH tunnel failed: " + err.Error()})
		return
	}

	// 11. Wait for tunnel port to be connectable.
	if err := waitForPort(localPort, 15*time.Second); err != nil {
		sshCmd.Process.Kill()
		sshCmd.Wait()
		os.Remove(keyFilePath)
		conn.WriteJSON(map[string]string{"type": "error", "message": "SSH tunnel port not ready: " + err.Error()})
		return
	}

	// 12. Track the session.
	session := &consoleSession{
		instanceID: d.InstanceID,
		connID:     connInfo.ID,
		sshCmd:     sshCmd,
		localPort:  localPort,
		keyFilePath: keyFilePath,
		cancel:     make(chan struct{}),
		controlWS:  conn,
	}

	h.mu.Lock()
	// Clean up any previous session for this instance.
	if old, ok := h.active[d.InstanceID]; ok {
		close(old.cancel)
		if old.sshCmd != nil && old.sshCmd.Process != nil {
			old.sshCmd.Process.Kill()
			old.sshCmd.Wait()
		}
		if old.keyFilePath != "" {
			os.Remove(old.keyFilePath)
		}
	}
	h.active[d.InstanceID] = session
	h.mu.Unlock()

	// Monitor SSH process in background.
	go func() {
		err := sshCmd.Wait()
		deps.Logger.Info().Err(err).Str("instanceId", d.InstanceID).Msg("SSH tunnel exited")

		h.mu.Lock()
		if cur, ok := h.active[d.InstanceID]; ok && cur == session {
			delete(h.active, d.InstanceID)
		}
		h.mu.Unlock()

		// Notify frontend if the control WS is still open.
		if session.controlWS != nil {
			session.controlWS.WriteJSON(map[string]interface{}{
				"type":       "tunnel_closed",
				"instanceId": d.InstanceID,
				"message":    "SSH tunnel closed",
			})
		}
		os.Remove(keyFilePath)
	}()

	// 13. Send vnc_ready with the binary VNC WebSocket URL.
	conn.WriteJSON(map[string]interface{}{
		"type":       "vnc_ready",
		"instanceId": d.InstanceID,
		"vncUrl":     fmt.Sprintf("/ws/vnc/%s", d.InstanceID),
		"message":    fmt.Sprintf("VNC console ready for %s (%s)", info.DisplayName, info.Shape),
	})
}

// HandleVNCBridge upgrades HTTP -> WebSocket and bridges binary VNC traffic
// to the local SSH tunnel port.
func (h *ConsoleHandler) HandleVNCBridge(w http.ResponseWriter, r *http.Request) {
	instanceID := r.URL.Query().Get("instanceId")
	if instanceID == "" {
		http.Error(w, "instanceId required", http.StatusBadRequest)
		return
	}

	// Verify session exists.
	h.mu.Lock()
	session := h.active[instanceID]
	h.mu.Unlock()

	if session == nil {
		http.Error(w, "no active VNC session for instance", http.StatusNotFound)
		return
	}

	// Upgrade to WebSocket.
	wsConn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer wsConn.Close()

	// Connect to local VNC port.
	tcpConn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(session.localPort)), 5*time.Second)
	if err != nil {
		wsConn.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","message":"failed to connect to VNC tunnel"}`))
		return
	}
	defer tcpConn.Close()

	// Bidirectional bridge.
	done := make(chan struct{})

	// TCP -> WebSocket (binary).
	go func() {
		defer func() { close(done) }()
		buf := make([]byte, 32*1024)
		for {
			n, err := tcpConn.Read(buf)
			if n > 0 {
				wsErr := wsConn.WriteMessage(websocket.BinaryMessage, buf[:n])
				if wsErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// WebSocket -> TCP (binary).
	go func() {
		defer func() { close(done) }()
		for {
			msgType, msg, err := wsConn.ReadMessage()
			if err != nil {
				return
			}
			if msgType == websocket.BinaryMessage {
				_, writeErr := tcpConn.Write(msg)
				if writeErr != nil {
					return
				}
			}
		}
	}()

	<-done
}

// handleDisconnect tears down the SSH tunnel and cleans up resources.
func (h *ConsoleHandler) handleDisconnect(conn *websocket.Conn, data json.RawMessage) {
	var d struct {
		InstanceID string `json:"instanceId"`
	}
	json.Unmarshal(data, &d)

	h.mu.Lock()
	session := h.active[d.InstanceID]
	if session != nil {
		delete(h.active, d.InstanceID)
	}
	h.mu.Unlock()

	if session != nil {
		close(session.cancel)
		if session.sshCmd != nil && session.sshCmd.Process != nil {
			session.sshCmd.Process.Kill()
			session.sshCmd.Wait()
		}
		if session.keyFilePath != "" {
			os.Remove(session.keyFilePath)
		}
	}

	conn.WriteJSON(map[string]string{
		"type":       "disconnected",
		"instanceId": d.InstanceID,
	})
}

// Shutdown terminates all active sessions.
func (h *ConsoleHandler) Shutdown() {
	h.mu.Lock()
	sessions := make(map[string]*consoleSession)
	for k, v := range h.active {
		sessions[k] = v
	}
	h.active = make(map[string]*consoleSession)
	h.mu.Unlock()

	for _, session := range sessions {
		close(session.cancel)
		if session.sshCmd != nil && session.sshCmd.Process != nil {
			session.sshCmd.Process.Kill()
			session.sshCmd.Wait()
		}
		if session.keyFilePath != "" {
			os.Remove(session.keyFilePath)
		}
	}
}

// savePrivateKeyFile writes the PEM private key to the console-keys directory
// with 0600 permissions (required by SSH).
func savePrivateKeyFile(instanceID, privateKeyPEM string) (string, error) {
	dir := filepath.Join("data", "console-keys", instanceID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create key dir: %w", err)
	}

	keyPath := filepath.Join(dir, "console-key.pem")
	if err := os.WriteFile(keyPath, []byte(privateKeyPEM), 0600); err != nil {
		return "", fmt.Errorf("write key file: %w", err)
	}
	return keyPath, nil
}

// waitForPort polls a TCP port until it accepts connections or times out.
func waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("port %d not ready after %s", port, timeout)
}

// extractFirstIP pulls the first IP from a JSON array or plain string.
func extractFirstIP(raw string) string {
	if raw == "" {
		return ""
	}
	var ips []string
	if err := json.Unmarshal([]byte(raw), &ips); err == nil && len(ips) > 0 {
		return ips[0]
	}
	if strings.Contains(raw, ",") {
		parts := strings.Split(raw, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	return strings.TrimSpace(raw)
}
