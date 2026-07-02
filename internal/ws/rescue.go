// Package ws — rescue.go: instance rescue/reinstall over WebSocket (SPEC S12.3).
// Full multi-step rescue flow: stop → detach → attach rescue → start →
// wait for SSH → user reinstall → stop → detach rescue → reattach original →
// start. Progress reports streamed over the WebSocket.
//
// Parity with Java RescueWebSocketHandler.
package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// RescueDeps holds dependencies for the rescue handler.
type RescueDeps struct {
	Logger    zerolog.Logger
	MasterKey []byte

	// OCI operations (can be nil for stubs).
	StopInstance     func(instanceID string, tenantID int64) error
	StartInstance    func(instanceID string, tenantID int64) error
	GetInstance      func(instanceID string, tenantID int64) (*RescueInstanceInfo, error)
	DetachBootVolume func(instanceID string, tenantID int64) (string, error) // returns bootVolumeID
	AttachBootVolume func(instanceID string, tenantID int64, bootVolumeID string) error
	AttachRescueVolume func(instanceID string, tenantID int64, rescueImageID string) (string, error) // returns rescue bootVolumeID

	// Phase 12.3: security rule + SSH root login (optional, nil = skip).
	CheckAndEnableRule func(ctx context.Context, tenantID int64) error
	EnableRootLogin    func(host, username, password, rootPassword string, port int) error
}

// RescueInstanceInfo holds instance state for rescue operations.
type RescueInstanceInfo struct {
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	State            string `json:"state"` // RUNNING, STOPPED, STARTING, STOPPING
	BootVolumeID     string `json:"bootVolumeId"`
	Shape            string `json:"shape"`
	AvailabilityDomain string `json:"availabilityDomain"`
	CompartmentID    string `json:"compartmentId"`
	ImageID          string `json:"imageId"`
	PublicIP         string `json:"publicIp"`
	SSHUsername      string `json:"sshUsername"`
	SSHPassword      string `json:"sshPassword"`
}

// RescueHandler manages the multi-step rescue/reinstall flow over WebSocket.
type RescueHandler struct {
	mu      sync.Mutex
	deps    *RescueDeps
	active  map[string]*rescueFlow // instanceID → active rescue flow
}

// rescueFlow tracks a single rescue operation's state.
type rescueFlow struct {
	InstanceID     string
	Step           string // current step
	OriginalBootID string
	RescueBootID   string
	Cancel         chan struct{}
	done           chan struct{}  // closed when runRescueFlow returns; lets Shutdown await exit
	sc             *safeConn      // shared write-serialized conn for this flow
	tenantID       int64          // tenant whose instance is being rescued (E7: was hardcoded 0 in CompleteRescue)
	Status         RescueStatus
}

// RescueStatus is sent as progress updates over WebSocket.
type RescueStatus struct {
	Step       string `json:"step"`
	Message    string `json:"message"`
	Progress   int    `json:"progress"` // 0-100
	Error      string `json:"error,omitempty"`
	InstanceID string `json:"instanceId"`
}

// SetDeps injects runtime dependencies.
func (h *RescueHandler) SetDeps(deps *RescueDeps) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.active == nil {
		h.active = make(map[string]*rescueFlow)
	}
	h.deps = deps
}

// Shutdown cancels every active rescue flow and best-effort waits for each
// flow's goroutine to exit. Called by Hub.Shutdown. A flow's Cancel may be
// closed already by handleCancel/handleInit (they delete from active first),
// so only flows still in the map are touched here.
func (h *RescueHandler) Shutdown() {
	h.mu.Lock()
	flows := make([]*rescueFlow, 0, len(h.active))
	for _, f := range h.active {
		flows = append(flows, f)
	}
	h.active = make(map[string]*rescueFlow)
	h.mu.Unlock()
	for _, f := range flows {
		close(f.Cancel)
		if f.done != nil {
			select {
			case <-f.done:
			case <-time.After(2 * time.Second):
				// ponytail: best-effort wait; a flow mid-OCI-poll may take a
				// poll cycle to notice cancel. Don't block graceful shutdown.
			}
		}
	}
}

// HandleRescue upgrades HTTP → WS for rescue operations.
func (h *RescueHandler) HandleRescue(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	// ponytail: all writes go through sc so the runRescueFlow goroutine and
	// this read loop (handleStatus/handleCancel) cannot race a gorilla write.
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
			continue
		}
		switch req.Type {
		case "init":
			h.handleInit(sc, req.Data)
		case "status":
			h.handleStatus(sc, req.Data)
		case "cancel":
			h.handleCancel(sc, req.Data)
		case "complete":
			h.handleComplete(sc, req.Data)
		default:
			sc.writeJSON(map[string]string{"type": "error", "message": "unknown command: " + req.Type})
		}
	}
}

func (h *RescueHandler) handleInit(sc *safeConn, data json.RawMessage) {
	var d struct {
		InstanceID    string `json:"instanceId"`
		RescueType    int    `json:"rescueType"`    // 0=DD, 1=netboot
		RescueImageID string `json:"rescueImageId"` // for netboot
		TenantID      int64  `json:"tenantId"`
	}
	if err := json.Unmarshal(data, &d); err != nil || d.InstanceID == "" {
		sc.writeJSON(map[string]string{"type": "error", "message": "instanceId required"})
		return
	}

	h.mu.Lock()
	deps := h.deps
	h.mu.Unlock()

	if deps == nil || deps.StopInstance == nil {
		sc.writeJSON(map[string]string{
			"type":    "info",
			"message": "rescue handler running in stub mode — OCI deps not wired",
		})
		return
	}

	flow := &rescueFlow{
		InstanceID: d.InstanceID,
		Step:       "init",
		Cancel:     make(chan struct{}),
		done:       make(chan struct{}),
		sc:         sc,
		tenantID:   d.TenantID,
	}

	h.mu.Lock()
	// C5: close any previous flow for this instance so its goroutine stops
	// calling OCI and doesn't leak; mirrors console.go's old-session cleanup.
	if old, ok := h.active[d.InstanceID]; ok {
		close(old.Cancel)
		delete(h.active, d.InstanceID)
	}
	h.active[d.InstanceID] = flow
	h.mu.Unlock()

	// Run the rescue flow in a goroutine, sending updates over WS. deps is
	// captured by value here so concurrent SetDeps cannot race the read.
	go h.runRescueFlow(sc, flow, deps, d.TenantID, d.RescueType, d.RescueImageID)
}

func (h *RescueHandler) handleStatus(sc *safeConn, data json.RawMessage) {
	var d struct {
		InstanceID string `json:"instanceId"`
	}
	json.Unmarshal(data, &d)

	h.mu.Lock()
	flow := h.active[d.InstanceID]
	h.mu.Unlock()

	if flow == nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "no active rescue for " + d.InstanceID})
		return
	}

	sc.writeJSON(RescueStatus{
		Step:       flow.Step,
		Message:    flow.Status.Message,
		Progress:   flow.Status.Progress,
		InstanceID: d.InstanceID,
	})
}

func (h *RescueHandler) handleCancel(sc *safeConn, data json.RawMessage) {
	var d struct {
		InstanceID string `json:"instanceId"`
	}
	json.Unmarshal(data, &d)

	h.mu.Lock()
	flow := h.active[d.InstanceID]
	if flow != nil {
		close(flow.Cancel)
		delete(h.active, d.InstanceID)
	}
	h.mu.Unlock()

	sc.writeJSON(map[string]string{
		"type":       "cancelled",
		"instanceId": d.InstanceID,
	})
}

// handleComplete resumes the rescue flow after the user finishes repair work.
func (h *RescueHandler) handleComplete(sc *safeConn, data json.RawMessage) {
	var d struct {
		InstanceID string `json:"instanceId"`
	}
	json.Unmarshal(data, &d)

	h.mu.Lock()
	flow := h.active[d.InstanceID]
	h.mu.Unlock()

	if flow == nil {
		sc.writeJSON(map[string]string{"type": "error", "message": "no active rescue for " + d.InstanceID})
		return
	}

	// Continue the rescue flow from step 7 onwards (restore original boot volume).
	go h.CompleteRescue(sc.c, d.InstanceID)
}

func (h *RescueHandler) runRescueFlow(sc *safeConn, flow *rescueFlow, deps *RescueDeps, tenantID int64, rescueType int, rescueImageID string) {
	defer close(flow.done) // signal exit so Shutdown can await cancellation
	if deps == nil {
		return
	}

	send := func(s RescueStatus) {
		flow.Status = s
		s.InstanceID = flow.InstanceID
		_ = sc.writeJSON(s)
	}

	isCancelled := func() bool {
		select {
		case <-flow.Cancel:
			return true
		default:
			return false
		}
	}

	// Pre-rescue: ensure security rules are open (Phase 12.3).
	if deps.CheckAndEnableRule != nil {
		if err := deps.CheckAndEnableRule(context.Background(), tenantID); err != nil {
			send(RescueStatus{Step: "error", Message: "开放安全规则失败", Error: err.Error(), Progress: 0})
			return
		}
	}

	// Step 1: Get instance info.
	send(RescueStatus{Step: "get_instance", Message: "获取实例信息...", Progress: 5})
	info, err := deps.GetInstance(flow.InstanceID, tenantID)
	if err != nil {
		send(RescueStatus{Step: "error", Message: "获取实例信息失败", Error: err.Error(), Progress: 0})
		return
	}
	flow.OriginalBootID = info.BootVolumeID

	// Step 2: Stop the instance.
	send(RescueStatus{Step: "stop", Message: fmt.Sprintf("停止实例 %s（原状态: %s）...", info.DisplayName, info.State), Progress: 15})
	if info.State == "RUNNING" {
		if err := deps.StopInstance(flow.InstanceID, tenantID); err != nil {
			send(RescueStatus{Step: "error", Message: "停止实例失败", Error: err.Error(), Progress: 0})
			return
		}

		// Wait for stopped state (poll every 5s, max 5 min).
		for i := 0; i < 60; i++ {
			if isCancelled() {
				send(RescueStatus{Step: "cancelled", Message: "操作已取消", Progress: 0})
				return
			}
			time.Sleep(5 * time.Second)
			inst, err := deps.GetInstance(flow.InstanceID, tenantID)
			if err != nil {
				continue
			}
			if inst.State == "STOPPED" {
				break
			}
			send(RescueStatus{Step: "stop", Message: fmt.Sprintf("等待实例停止... 当前状态: %s", inst.State), Progress: 20})
		}
	}
	send(RescueStatus{Step: "stop", Message: "实例已停止", Progress: 25})

	// Step 3: Detach original boot volume.
	send(RescueStatus{Step: "detach_original", Message: "卸载原始引导卷...", Progress: 30})
	if flow.OriginalBootID != "" {
		_, err := deps.DetachBootVolume(flow.InstanceID, tenantID)
		if err != nil {
			send(RescueStatus{Step: "error", Message: "卸载引导卷失败", Error: err.Error(), Progress: 0})
			return
		}
	}
	send(RescueStatus{Step: "detach_original", Message: "原始引导卷已卸载", Progress: 40})

	if isCancelled() {
		send(RescueStatus{Step: "cancelled", Message: "操作已取消", Progress: 0})
		return
	}

	// Step 4: Attach rescue boot volume.
	send(RescueStatus{Step: "attach_rescue", Message: "挂载急救引导卷...", Progress: 50})
	rescueBootID, err := deps.AttachRescueVolume(flow.InstanceID, tenantID, rescueImageID)
	if err != nil {
		send(RescueStatus{Step: "error", Message: "挂载急救引导卷失败", Error: err.Error(), Progress: 0})
		return
	}
	flow.RescueBootID = rescueBootID
	send(RescueStatus{Step: "attach_rescue", Message: "急救引导卷已挂载", Progress: 60})

	// Step 5: Start the instance with rescue volume.
	send(RescueStatus{Step: "start_rescue", Message: "启动急救系统...", Progress: 70})
	if err := deps.StartInstance(flow.InstanceID, tenantID); err != nil {
		send(RescueStatus{Step: "error", Message: "启动急救系统失败", Error: err.Error(), Progress: 0})
		return
	}

	// Wait for running state (poll every 5s, max 3 min).
	for i := 0; i < 36; i++ {
		if isCancelled() {
			send(RescueStatus{Step: "cancelled", Message: "操作已取消", Progress: 0})
			return
		}
		time.Sleep(5 * time.Second)
		inst, err := deps.GetInstance(flow.InstanceID, tenantID)
		if err != nil {
			continue
		}
		if inst.State == "RUNNING" {
			info = inst
			break
		}
		send(RescueStatus{Step: "start_rescue", Message: fmt.Sprintf("等待急救系统启动... 状态: %s", inst.State), Progress: 75})
	}

	flow.Step = "user_action"
	send(RescueStatus{
		Step:     "user_action",
		Message:  fmt.Sprintf("急救系统已启动！请通过 SSH 连接到实例执行救援操作（DD 重建等）。IP: %s, 用户: %s", info.PublicIP, info.SSHUsername),
		Progress: 80,
	})

	// Step 6: Wait for user to complete their work.
	// The user sends a "complete" message to finalize, or "cancel" to abort.
	// The flow remains at "user_action" until the client sends the next command.
}

// CompleteRescue finishes the rescue flow after user action is done.
func (h *RescueHandler) CompleteRescue(conn *websocket.Conn, instanceID string) {
	h.mu.Lock()
	flow := h.active[instanceID]
	deps := h.deps
	h.mu.Unlock()

	if flow == nil || deps == nil {
		return
	}

	// Use the flow's shared safeConn so writes here cannot race the main
	// read loop (handleStatus/handleCancel). Fallback wraps conn if absent.
	sc := flow.sc
	if sc == nil {
		sc = &safeConn{c: conn}
	}

	send := func(s RescueStatus) {
		flow.Status = s
		s.InstanceID = flow.InstanceID
		_ = sc.writeJSON(s)
	}

	// Step 7: Stop instance.
	send(RescueStatus{Step: "stop_rescue", Message: "停止急救系统...", Progress: 82})
	if err := deps.StopInstance(flow.InstanceID, flow.tenantID); err != nil {
		send(RescueStatus{Step: "error", Message: "停止急救系统失败", Error: err.Error(), Progress: 82})
		return
	}
	time.Sleep(5 * time.Second)

	// Wait for stopped (max 5 min).
	for i := 0; i < 60; i++ {
		inst, err := deps.GetInstance(flow.InstanceID, flow.tenantID)
		if err != nil || inst.State == "STOPPED" {
			break
		}
		time.Sleep(5 * time.Second)
	}
	send(RescueStatus{Step: "stop_rescue", Message: "急救系统已停止", Progress: 85})

	// Step 8: Detach rescue boot volume.
	send(RescueStatus{Step: "detach_rescue", Message: "卸载急救引导卷...", Progress: 88})
	if flow.RescueBootID != "" {
		if _, err := deps.DetachBootVolume(flow.InstanceID, flow.tenantID); err != nil {
			send(RescueStatus{Step: "error", Message: "卸载急救引导卷失败", Error: err.Error(), Progress: 88})
			return
		}
	}
	send(RescueStatus{Step: "detach_rescue", Message: "急救引导卷已卸载", Progress: 92})

	// Step 9: Reattach original boot volume.
	send(RescueStatus{Step: "reattach_original", Message: "重新挂载原始引导卷...", Progress: 95})
	if flow.OriginalBootID != "" {
		if err := deps.AttachBootVolume(flow.InstanceID, flow.tenantID, flow.OriginalBootID); err != nil {
			send(RescueStatus{Step: "error", Message: "重新挂载原始引导卷失败", Error: err.Error(), Progress: 95})
			return
		}
	}
	send(RescueStatus{Step: "reattach_original", Message: "原始引导卷已挂载", Progress: 98})

	// Step 10: Start instance.
	send(RescueStatus{Step: "start_final", Message: "启动实例...", Progress: 99})
	if err := deps.StartInstance(flow.InstanceID, flow.tenantID); err != nil {
		send(RescueStatus{Step: "error", Message: "启动实例失败", Error: err.Error(), Progress: 99})
		return
	}

	// Step 10.5: Wait for instance to be reachable, then enable root login (Phase 12.3).
	if deps.EnableRootLogin != nil {
		send(RescueStatus{Step: "enable_root", Message: "等待实例启动并配置SSH...", Progress: 99})
		time.Sleep(30 * time.Second) // Give sshd time to start.

		info, err := deps.GetInstance(flow.InstanceID, flow.tenantID)
		if err == nil && info.PublicIP != "" {
			if err := deps.EnableRootLogin(info.PublicIP, "root", info.SSHPassword, info.SSHPassword, 22); err != nil {
				send(RescueStatus{Step: "warning", Message: "SSH配置失败（实例可能需要手动配置）",
					Error: err.Error(), Progress: 100})
			}
		}
	}

	send(RescueStatus{Step: "complete", Message: "救援流程完成！实例已恢复启动。", Progress: 100})

	h.mu.Lock()
	delete(h.active, instanceID)
	h.mu.Unlock()
}
