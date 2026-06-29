// Package httpapi -- handler_quick_dd.go: Phase 13.2 Quick DD One-Click
// Reinstall HTTP handlers. Provides SSE (Server-Sent Events) streaming for
// real-time DD progress, plus a synchronous endpoint for simple use cases.
package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/service"
)

// POST /quick-dd/start
// Starts a quick DD operation with SSE progress streaming.
// The response is streamed as Server-Sent Events.
func quickDDStart(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req service.DDRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		if err := deps.QuickDDSvc.ValidateDDRequest(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, err.Error())
			return
		}

		// Set SSE headers.
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		// Create progress channel.
		progressCh := make(chan service.DDProgress, 64)

		// Run DD in background goroutine.
		go func() {
			defer close(progressCh)
			_ = deps.QuickDDSvc.RunDDWithProgress(c.Request.Context(), req, progressCh)
		}()

		// Stream progress events as SSE.
		c.Stream(func(w io.Writer) bool {
			progress, ok := <-progressCh
			if !ok {
				// Channel closed, send final event.
				c.SSEvent("done", map[string]string{"message": "stream ended"})
				return false
			}

			data, err := json.Marshal(progress)
			if err != nil {
				c.SSEvent("error", map[string]string{"error": err.Error()})
				return false
			}

			c.SSEvent("progress", string(data))

			// Keep streaming until completed or error.
			return progress.Status != "completed" && progress.Status != "error"
		})
	}
}

// POST /quick-dd/execute
// Executes a quick DD operation synchronously (no SSE).
// Returns the final result after the DD completes.
func quickDDExecute(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req service.DDRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		result, err := deps.QuickDDSvc.RunDD(c.Request.Context(), req)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "DD failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(result))
	}
}

// POST /quick-dd/reboot
// Reboots an instance via SSH after DD completion.
func quickDDReboot(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			InstanceID int64 `json:"instanceId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.InstanceID <= 0 {
			response.Fail(c, http.StatusBadRequest, "instanceId is required")
			return
		}

		if err := deps.QuickDDSvc.RebootInstance(c.Request.Context(), body.InstanceID); err != nil {
			response.Fail(c, http.StatusInternalServerError, "reboot failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("reboot command sent"))
	}
}

// GET /quick-dd/ssh-config/:id
// Returns the SSH configuration for an instance (for DD setup).
func quickDDSSHConfig(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id")
			return
		}

		host, port, username, _, err := deps.QuickDDSvc.GetInstanceSSHConfig(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "get SSH config: "+err.Error())
			return
		}

		response.OK(c, response.SuccessData(map[string]any{
			"host":     host,
			"port":     port,
			"username": username,
		}))
	}
}
