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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	// PersistConsoleConnection saves the connID + private key for an
	// app-created connection so it can be resumed later (encrypted at rest by
	// the implementation). Wired to service.ConsoleConnectionService.Persist.
	PersistConsoleConnection func(ctx context.Context, instanceID, connID string, tenantID int64, privateKeyPEM, publicKeySSH string) error

	// LoadConsoleConnection returns the connID + decrypted private key PEM for
	// the instance's app-created connection (sql.ErrNoRows if none). Wired to
	// service.ConsoleConnectionService.LoadForResume.
	LoadConsoleConnection func(ctx context.Context, instanceID string) (connID, privateKeyPEM string, err error)
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
	instanceID  string
	connID      string
	sshCmd      *exec.Cmd
	localPort   int
	keyFilePath string
	cancel      chan struct{}
	controlWS   *safeConn // the control WebSocket (/ws/console), serialized
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
	// ponytail: all control-conn writes go through safeConn so the
	// tunnel-wait goroutine and this read loop cannot race a gorilla write.
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
			sc.writeJSON(map[string]string{"type": "error", "message": "invalid JSON"})
			continue
		}

		switch req.Type {
		case "ping", "heartbeat":
			sc.writeJSON(map[string]string{"type": "pong"})

		case "create_connection":
			h.handleCreateConnection(sc, req.Data)

		case "resume_connection":
			h.handleResumeConnection(sc, req.Data)

		case "disconnect":
			h.handleDisconnect(sc, req.Data)
			return

		default:
			sc.writeJSON(map[string]string{"type": "error", "message": "unknown command: " + req.Type})
		}
	}
}

// handleCreateConnection creates an OCI console connection, starts an SSH
// tunnel, and notifies the frontend with the VNC WebSocket URL.
func (h *ConsoleHandler) handleCreateConnection(sc *safeConn, data json.RawMessage) {
	var d struct {
		InstanceID string `json:"instanceId"`
		TenantID   int64  `json:"tenantId"`
	}
	if err := json.Unmarshal(data, &d); err != nil || d.InstanceID == "" {
		sc.writeJSON(map[string]string{"type": "error", "message": "instanceId required"})
		return
	}

	h.mu.Lock()
	deps := h.deps
	h.mu.Unlock()

	if deps == nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "console handler not configured"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// 1. Get instance info for compartment ID.
	progress(sc, "查询实例信息...")
	info, err := deps.InstanceLookup(d.InstanceID)
	if err != nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "instance lookup failed: " + err.Error()})
		return
	}

	compartmentID := info.CompartmentID
	if compartmentID == "" {
		// Fallback: get compartment from tenant's tenancy.
		progress(sc, "获取区间 ID...")
		compartmentID, err = deps.GetCompartmentID(ctx, info.TenantID, d.InstanceID)
		if err != nil {
			sc.writeJSON(map[string]string{"type": "error", "message": "get compartment failed: " + err.Error()})
			return
		}
	}

	// 2. Build OCI clients.
	progress(sc, "构建 OCI 凭据...")
	clients, err := deps.BuildClients(ctx, info.TenantID)
	if err != nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "build OCI clients failed: " + err.Error()})
		return
	}

	// 3. Generate the SSH key pair bound to this connection at creation time.
	progress(sc, "生成 SSH 密钥对...")
	keyPair, err := sshkeygen.GenerateRSA2048()
	if err != nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "generate SSH key failed: " + err.Error()})
		return
	}

	// 4. Create a fresh console connection. EnsureConsoleConnection clears any
	//    lingering connection first, waits for the async delete to settle, and
	//    recovers from a 409 IncorrectState ("already exists or has not been
	//    terminated") by retrying once — the root-cause fix for the VNC 409.
	//    The old "reuse ACTIVE without key, then delete-and-race-create" flow
	//    was the bug: Delete is async, so the immediate Create hit 409.
	progress(sc, "清理旧连接并创建控制台连接（如有遗留可能需等待异步删除）...")
	connInfo, err := oci.EnsureConsoleConnection(ctx, clients, compartmentID, d.InstanceID, keyPair.PublicKeySSH, 90*time.Second)
	if err != nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "create console connection failed: " + friendlyConsoleErr(err)})
		return
	}
	progress(sc, "控制台连接已就绪: "+connInfo.ID)

	// 5. Persist the connID + private key so this connection can be resumed
	//    later (encrypted at rest by the implementation). Best-effort: resume
	//    is a nicety, a persist failure must not break the live session.
	if deps.PersistConsoleConnection != nil {
		if perr := deps.PersistConsoleConnection(ctx, d.InstanceID, connInfo.ID, info.TenantID, keyPair.PrivateKeyPEM, keyPair.PublicKeySSH); perr != nil {
			deps.Logger.Warn().Err(perr).Str("instanceId", d.InstanceID).
				Msg("persist console connection for resume failed (best-effort)")
		}
	}

	// 6. Build the SSH tunnel + notify the frontend.
	h.startTunnelAndNotify(sc, deps, d.InstanceID, info, connInfo, keyPair.PrivateKeyPEM)
}

// getConsoleConnectionInfo fetches an OCI console connection's current state +
// VNC connection string by ID. Overridable in tests (Clients.Compute is
// concrete) so handleResumeConnection's decision logic can be exercised
// without a live ComputeClient. Mirrors oci.GetConsoleConnectionInfo.
var getConsoleConnectionInfo = oci.GetConsoleConnectionInfo

// handleResumeConnection resumes a VNC session to an existing app-created
// console connection: loads the persisted connID + private key, fetches the
// connection's current state from OCI, and — if still ACTIVE — starts the SSH
// tunnel with the persisted key (no new OCI connection is created, so no 409).
func (h *ConsoleHandler) handleResumeConnection(sc *safeConn, data json.RawMessage) {
	var d struct {
		InstanceID string `json:"instanceId"`
	}
	if err := json.Unmarshal(data, &d); err != nil || d.InstanceID == "" {
		sc.writeJSON(map[string]string{"type": "error", "message": "instanceId required"})
		return
	}

	h.mu.Lock()
	deps := h.deps
	h.mu.Unlock()
	if deps == nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "console handler not configured"})
		return
	}
	if deps.LoadConsoleConnection == nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "resume not configured"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	progress(sc, "查询实例信息...")
	info, err := deps.InstanceLookup(d.InstanceID)
	if err != nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "instance lookup failed: " + err.Error()})
		return
	}

	// Load the app-created connection's connID + decrypted private key.
	progress(sc, "加载已保存的控制台连接...")
	connID, privateKeyPEM, err := deps.LoadConsoleConnection(ctx, d.InstanceID)
	if err != nil {
		deps.Logger.Warn().Err(err).Str("instanceId", d.InstanceID).
			Msg("load console connection for resume failed")
		sc.writeJSON(map[string]string{"type": "error", "message": "无可恢复的连接（本应用未保存该实例的 console connection），请新建连接"})
		return
	}
	if privateKeyPEM == "" {
		sc.writeJSON(map[string]string{"type": "error", "message": "saved console key is missing; create a new connection instead"})
		return
	}

	progress(sc, "构建 OCI 凭据...")
	clients, err := deps.BuildClients(ctx, info.TenantID)
	if err != nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "build OCI clients failed: " + err.Error()})
		return
	}

	// Fetch the connection's current state + VNC connection string.
	progress(sc, "检查连接状态（"+connID+"）...")
	connInfo, err := getConsoleConnectionInfo(ctx, clients, connID)
	if err != nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "fetch console connection failed: " + err.Error()})
		return
	}
	if connInfo.LifecycleState != "ACTIVE" {
		sc.writeJSON(map[string]string{"type": "error", "message": fmt.Sprintf("保存的连接状态为 %s（非 ACTIVE），请删除后新建", connInfo.LifecycleState)})
		return
	}
	progress(sc, "连接处于 ACTIVE，开始复用隧道...")

	h.startTunnelAndNotify(sc, deps, d.InstanceID, info, connInfo, privateKeyPEM)
}

// startTunnelAndNotify parses the VNC connection string, starts the local SSH
// tunnel to the OCI console proxy, tracks the session, and sends vnc_ready.
// Shared by handleCreateConnection (fresh connection) and handleResumeConnection
// (reusing an app-created connection + its persisted private key).
func (h *ConsoleHandler) startTunnelAndNotify(sc *safeConn, deps *ConsoleDeps, instanceID string, info *ConsoleInstanceInfo, connInfo *oci.ConsoleConnectionInfo, privateKeyPEM string) {
	// Use VncConnectionString, not ConnectionString: the SDK field doc states
	// VncConnectionString is "the SSH connection string for the SSH tunnel used
	// to connect to the console connection over VNC" — ConnectionString is the
	// serial-console string and targets the wrong endpoint for VNC.
	if connInfo.VncConnectionString == "" {
		sc.writeJSON(map[string]string{"type": "error", "message": "empty VNC connection string from console connection"})
		return
	}
	parsed, err := oci.ParseConnectionString(connInfo.VncConnectionString)
	if err != nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "parse connection string failed: " + err.Error()})
		return
	}
	// Log the raw strings + parsed parts + (later) the built ssh args so a
	// tunnel failure can be pinpointed: the OCI VncConnectionString format
	// evolves and a -W/-L target mismatch shows up here, not in a blind
	// "port not ready". connID/proxyHost are operational, not secret.
	deps.Logger.Info().
		Str("instanceId", instanceID).
		Str("vncConnectionString", connInfo.VncConnectionString).
		Str("connectionString", connInfo.ConnectionString).
		Str("connID", parsed.ConnectionID).
		Str("proxyHost", parsed.ProxyHost).
		Str("targetHost", parsed.TargetHost).
		Msg("VNC tunnel: parsed OCI connection string")

	// Save the private key (0600) for the SSH tunnel.
	progress(sc, "保存临时私钥文件...")
	keyFilePath, err := savePrivateKeyFile(instanceID, privateKeyPEM)
	if err != nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "save key file failed: " + err.Error()})
		return
	}

	// Allocate local port for VNC tunnel.
	progress(sc, "分配本地隧道端口...")
	localListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Remove(keyFilePath)
		sc.writeJSON(map[string]string{"type": "error", "message": "allocate local port failed: " + err.Error()})
		return
	}
	localPort := localListener.Addr().(*net.TCPAddr).Port
	localListener.Close() // Release immediately; SSH tunnel will bind.

	// Start SSH tunnel process.
	progress(sc, fmt.Sprintf("启动 SSH 隧道（本地端口 %d → OCI 控制台代理）...", localPort))
	tunnelCfg := oci.SSHTunnelConfig{
		PrivateKeyPath: keyFilePath,
		ConnectionID:   parsed.ConnectionID,
		ProxyHost:      parsed.ProxyHost,
		TargetHost:     parsed.TargetHost,
		LocalPort:      localPort,
		RemotePort:     5900, // VNC default
	}
	sshArgs := oci.BuildSSHTunnelCommand(tunnelCfg)
	deps.Logger.Info().Strs("sshArgs", sshArgs).Str("localPort", strconv.Itoa(localPort)).
		Msg("VNC tunnel: built ssh command")

	sshCmd := exec.Command(sshArgs[0], sshArgs[1:]...)
	// Capture combined ssh output so a failure surfaces the real cause
	// (Permission denied / Load key invalid format / Could not resolve host)
	// instead of a blind "port not ready". ssh never prints the private key.
	var sshOut bytes.Buffer
	sshCmd.Stdout = &sshOut
	sshCmd.Stderr = &sshOut
	if err := sshCmd.Start(); err != nil {
		os.Remove(keyFilePath)
		sc.writeJSON(map[string]string{"type": "error", "message": "start SSH tunnel failed: " + err.Error()})
		return
	}

	// Wait for tunnel port to be connectable.
	progress(sc, "等待隧道端口就绪...")
	if err := waitForPort(localPort, 15*time.Second); err != nil {
		sshCmd.Process.Kill()
		sshCmd.Wait()
		os.Remove(keyFilePath)
		sc.writeJSON(map[string]string{"type": "error", "message": formatTunnelError(err, sshOut.String())})
		return
	}

	// Track the session.
	session := &consoleSession{
		instanceID:  instanceID,
		connID:      connInfo.ID,
		sshCmd:      sshCmd,
		localPort:   localPort,
		keyFilePath: keyFilePath,
		cancel:      make(chan struct{}),
		controlWS:   sc,
	}

	h.mu.Lock()
	// Clean up any previous session for this instance.
	if old, ok := h.active[instanceID]; ok {
		close(old.cancel)
		if old.sshCmd != nil && old.sshCmd.Process != nil {
			old.sshCmd.Process.Kill()
			old.sshCmd.Wait()
		}
		if old.keyFilePath != "" {
			os.Remove(old.keyFilePath)
		}
	}
	h.active[instanceID] = session
	h.mu.Unlock()

	// Monitor SSH process in background.
	go func() {
		err := sshCmd.Wait()
		deps.Logger.Info().Err(err).Str("instanceId", instanceID).
			Str("sshOut", strings.TrimSpace(sshOut.String())).
			Msg("SSH tunnel exited")

		h.mu.Lock()
		if cur, ok := h.active[instanceID]; ok && cur == session {
			delete(h.active, instanceID)
		}
		h.mu.Unlock()

		// Notify frontend if the control WS is still open.
		if session.controlWS != nil {
			session.controlWS.writeJSON(map[string]interface{}{
				"type":       "tunnel_closed",
				"instanceId": instanceID,
				"message":    "SSH tunnel closed",
			})
		}
		os.Remove(keyFilePath)
	}()

	// Send vnc_ready with the binary VNC WebSocket URL.
	sc.writeJSON(map[string]interface{}{
		"type":       "vnc_ready",
		"instanceId": instanceID,
		"vncUrl":     fmt.Sprintf("/ws/vnc/%s", instanceID),
		"message":    fmt.Sprintf("VNC console ready for %s (%s)", info.DisplayName, info.Shape),
	})
}

// friendlyConsoleErr turns a console-connection create error into a user-facing
// message. A persistent 409 IncorrectState (retry exhausted) gets a clear hint
// to wait for OCI's async termination or delete manually; other errors pass
// through verbatim.
func friendlyConsoleErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.Contains(msg, "409") || strings.Contains(msg, "IncorrectState") || strings.Contains(msg, "retry after 409") {
		return "OCI 侧仍存在未终止的控制台连接，请约 1 分钟后重试；若持续失败，可在 OCI 控制台或用 CLI 手动删除该实例的 console connection 后重连（详情: " + msg + "）"
	}
	return msg
}

// formatTunnelError builds the message sent to the frontend when the local SSH
// tunnel port fails to come up. The real cause lives in the ssh process's
// combined stdout/stderr (e.g. "Permission denied (publickey)",
// "Load key: invalid format", "Could not resolve host") — appending a bounded
// tail of that output turns a blind "port not ready" into a diagnosable error.
// ssh never prints the private key itself, so this is safe to surface.
func formatTunnelError(portErr error, sshOut string) string {
	msg := "SSH tunnel port not ready: " + portErr.Error()
	tail := strings.TrimSpace(sshOut)
	if tail == "" {
		return msg
	}
	const max = 600
	if len(tail) > max {
		tail = "..." + tail[len(tail)-max:]
	}
	return msg + " | ssh: " + tail
}

// progress sends a step message to the frontend control WebSocket so the user
// sees what the connection flow is doing (查询实例 / 生成密钥 / 启动隧道 ...).
// The frontend appends msg.data to the status log on "output" messages.
func progress(sc *safeConn, msg string) {
	sc.writeJSON(map[string]string{"type": "output", "data": msg + "\n"})
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

	// Bidirectional binary bridge. Blocks until one side closes; the conns
	// are then closed by HandleVNCBridge's defers, which unblocks the other
	// copy goroutine. sync.Once guards the done-channel close so the second
	// goroutine's defer doesn't panic on "close of closed channel".
	bridgeVNC(wsConn, tcpConn)
}

// bridgeVNC copies binary data bidirectionally between a WebSocket and a TCP
// conn until one side closes (or errors), then returns. The surviving copy
// goroutine exits shortly after, when the caller closes the conn it's blocked
// on. sync.Once makes the done-close idempotent — both goroutines defer
// signal(), but the channel is closed only once (the old code did close(done)
// in both defers and panicked).
func bridgeVNC(wsConn *websocket.Conn, tcpConn net.Conn) {
	done := make(chan struct{})
	var once sync.Once
	signal := func() { once.Do(func() { close(done) }) }

	// TCP -> WebSocket (binary).
	go func() {
		defer signal()
		buf := make([]byte, 32*1024)
		for {
			n, err := tcpConn.Read(buf)
			if n > 0 {
				if wsErr := wsConn.WriteMessage(websocket.BinaryMessage, buf[:n]); wsErr != nil {
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
		defer signal()
		for {
			msgType, msg, err := wsConn.ReadMessage()
			if err != nil {
				return
			}
			if msgType == websocket.BinaryMessage {
				// Write-all: net.Conn.Write may short-write, and dropping
				// bytes misaligns the RFB byte stream -> a garbled VNC screen.
				for len(msg) > 0 {
					n, writeErr := tcpConn.Write(msg)
					if writeErr != nil {
						return
					}
					msg = msg[n:]
				}
			}
		}
	}()

	<-done
}

// HandleSerialConsole upgrades HTTP → WebSocket and runs an interactive
// serial-console terminal: it ensures the OCI console connection exists,
// starts an `ssh -tt` process to the console proxy (which routes to the
// instance's serial console), and bridges the WebSocket to the ssh process's
// stdin/stdout so a browser xterm.js can interact with the serial console.
// This is the headless-instance counterpart to VNC: no desktop required, just
// a text terminal (boot logs / login / rescue).
func (h *ConsoleHandler) HandleSerialConsole(w http.ResponseWriter, r *http.Request) {
	instanceID := r.URL.Query().Get("instanceId")
	if instanceID == "" {
		http.Error(w, "instanceId required", http.StatusBadRequest)
		return
	}
	h.mu.Lock()
	deps := h.deps
	h.mu.Unlock()
	if deps == nil {
		http.Error(w, "console handler not configured", http.StatusInternalServerError)
		return
	}

	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	sc := &safeConn{c: conn}

	progress(sc, "查询实例信息...")
	info, err := deps.InstanceLookup(instanceID)
	if err != nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "instance lookup failed: " + err.Error()})
		return
	}

	progress(sc, "建立/恢复控制台连接...")
	connInfo, privateKeyPEM, err := h.ensureSerialConn(sc, deps, info, instanceID)
	if err != nil {
		sc.writeJSON(map[string]string{"type": "error", "message": err.Error()})
		return
	}

	progress(sc, "解析串口连接字符串...")
	parsed, err := oci.ParseConnectionString(connInfo.ConnectionString)
	if err != nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "parse connection string failed: " + err.Error()})
		return
	}

	progress(sc, "保存临时私钥...")
	keyFilePath, err := savePrivateKeyFile(instanceID, privateKeyPEM)
	if err != nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "save key file failed: " + err.Error()})
		return
	}

	progress(sc, "启动串口控制台 SSH...")
	sshArgs := oci.BuildSerialConsoleCommand(oci.SSHTunnelConfig{
		PrivateKeyPath: keyFilePath,
		ProxyHost:      parsed.ProxyHost,
		TargetHost:     parsed.TargetHost,
	})
	deps.Logger.Info().Strs("sshArgs", sshArgs).Str("instanceId", instanceID).
		Msg("serial console: built ssh command")
	sshCmd := exec.Command(sshArgs[0], sshArgs[1:]...)
	var sshOut bytes.Buffer
	// Merge ssh stdout + stderr → WebSocket so the user sees the MOTD/banner
	// (OCI prints it on stderr) AND the interactive console. stderr is also
	// captured to sshOut for the exit log. exec copies both via its own
	// goroutines; safeConn serializes the WS writes.
	wsWriter := &serialWSWriter{sc: sc}
	sshCmd.Stdout = wsWriter
	sshCmd.Stderr = io.MultiWriter(wsWriter, &sshOut)
	stdin, _ := sshCmd.StdinPipe()
	if err := sshCmd.Start(); err != nil {
		os.Remove(keyFilePath)
		sc.writeJSON(map[string]string{"type": "error", "message": "start serial ssh failed: " + err.Error()})
		return
	}

	// done closes when ssh exits OR the WS closes; sync.Once prevents a
	// double-close.
	done := make(chan struct{})
	var once sync.Once
	signal := func() { once.Do(func() { close(done) }) }

	// ssh exit → signal. Wait reaps the process exactly once.
	go func() {
		defer signal()
		_ = sshCmd.Wait()
	}()

	// WebSocket → ssh stdin. xterm.js sends {"type":"input","data":"..."}.
	// {"type":"resize",...} is accepted but ignored (exec ssh -tt uses a
	// fixed remote PTY; runtime resize needs a local PTY, not worth a dep here).
	go func() {
		defer signal()
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if msgType != websocket.TextMessage && msgType != websocket.BinaryMessage {
				continue
			}
			var m struct {
				Type string          `json:"type"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(msg, &m); err != nil || m.Type != "input" {
				continue // ignore resize / non-JSON / unknown
			}
			s, perr := parseInputData(m.Data)
			if perr != nil {
				continue
			}
			b := []byte(s)
			for len(b) > 0 { // write-all (short writes would corrupt the terminal stream)
				n, werr := stdin.Write(b)
				if werr != nil {
					return
				}
				b = b[n:]
			}
		}
	}()

	<-done
	// One side ended. If the WS closed first, kill ssh; the Wait goroutine
	// reaps it. Close the WS to unblock the stdin loop if ssh exited first.
	if sshCmd.Process != nil {
		_ = sshCmd.Process.Kill()
	}
	conn.Close()
	os.Remove(keyFilePath)
	<-done // wait for the Wait goroutine to finish reaping (both sides now done)
	deps.Logger.Info().Str("instanceId", instanceID).
		Str("sshOut", strings.TrimSpace(sshOut.String())).
		Msg("serial console ssh exited")
}

// serialWSWriter forwards ssh stdout/stderr bytes to the WebSocket as binary
// frames. Used as exec.Cmd.Stdout/Stderr so the OCI MOTD (on stderr) + the
// interactive console (on stdout) both reach the xterm.js client.
type serialWSWriter struct{ sc *safeConn }

func (w *serialWSWriter) Write(p []byte) (int, error) {
	if err := w.sc.writeMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// ensureSerialConn returns a console connection (with its ConnectionString) +
// the private key to use for the serial ssh. It prefers resuming a previously
// persisted app-created connection (so the VNC + serial consoles share it);
// otherwise it creates a fresh one and persists it.
func (h *ConsoleHandler) ensureSerialConn(sc *safeConn, deps *ConsoleDeps, info *ConsoleInstanceInfo, instanceID string) (*oci.ConsoleConnectionInfo, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Try resume: persisted connID + decrypted key.
	if deps.LoadConsoleConnection != nil {
		connID, pem, err := deps.LoadConsoleConnection(ctx, instanceID)
		if err == nil && pem != "" {
			clients, cerr := deps.BuildClients(ctx, info.TenantID)
			if cerr == nil {
				if connInfo, gerr := getConsoleConnectionInfo(ctx, clients, connID); gerr == nil &&
					connInfo.LifecycleState == "ACTIVE" && connInfo.ConnectionString != "" {
					progress(sc, "复用已保存的控制台连接")
					return connInfo, pem, nil
				}
			}
		}
	}

	// Create fresh.
	progress(sc, "构建 OCI 凭据...")
	compartmentID := info.CompartmentID
	if compartmentID == "" {
		var cerr error
		compartmentID, cerr = deps.GetCompartmentID(ctx, info.TenantID, instanceID)
		if cerr != nil {
			return nil, "", fmt.Errorf("get compartment: %w", cerr)
		}
	}
	clients, err := deps.BuildClients(ctx, info.TenantID)
	if err != nil {
		return nil, "", fmt.Errorf("build OCI clients: %w", err)
	}
	progress(sc, "生成 SSH 密钥对...")
	keyPair, err := sshkeygen.GenerateRSA2048()
	if err != nil {
		return nil, "", fmt.Errorf("generate SSH key: %w", err)
	}
	progress(sc, "创建控制台连接（清理遗留）...")
	connInfo, err := oci.EnsureConsoleConnection(ctx, clients, compartmentID, instanceID, keyPair.PublicKeySSH, 90*time.Second)
	if err != nil {
		return nil, "", fmt.Errorf("create console connection: %s", friendlyConsoleErr(err))
	}
	if deps.PersistConsoleConnection != nil {
		if perr := deps.PersistConsoleConnection(ctx, instanceID, connInfo.ID, info.TenantID, keyPair.PrivateKeyPEM, keyPair.PublicKeySSH); perr != nil {
			deps.Logger.Warn().Err(perr).Str("instanceId", instanceID).Msg("persist console connection failed (best-effort)")
		}
	}
	return connInfo, keyPair.PrivateKeyPEM, nil
}

// handleDisconnect tears down the SSH tunnel and cleans up resources. The OCI
// console connection is intentionally KEPT so the session can be resumed later;
// EnsureConsoleConnection clears any lingering connection on the next create,
// so keeping it does not bring back the 409.
func (h *ConsoleHandler) handleDisconnect(sc *safeConn, data json.RawMessage) {
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

	sc.writeJSON(map[string]string{
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
