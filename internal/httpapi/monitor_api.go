// Package httpapi — monitor_api.go: monitor agent download + report endpoints
// (Phase 6, upgraded Phase 8). Port of MonitorApiController.java. These are
// PUBLIC routes (no auth required) — the agent runs on remote VPS instances.
//
// Uses embedded monitor_agent.sh with {{SERVER_URL}}, {{TOKEN}}, {{INTERVAL}}
// template substitution at download time.
package httpapi

import (
	_ "embed"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/ws"
)

//go:embed monitor_agent.sh
var monitorAgentScript string

// monitorTokenRe restricts the agent token to a safe charset so it cannot
// escape its bash variable assignment. ponytail: ceiling 128 chars, no shell
// metacharacters; widen the charset only if a real token format requires it.
var monitorTokenRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// monitorDownload serves the monitor agent bash script with dynamic config.
// GET /api/monitor/download?token=<token>&interval=<seconds>
func monitorDownload(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		intervalStr := c.DefaultQuery("interval", "10")

		// S1: validate inputs before template substitution. The script renders
		// INTERVAL={{INTERVAL}} unquoted, so any non-integer could inject shell.
		n, err := strconv.Atoi(intervalStr)
		if err != nil || n < 1 || n > 3600 {
			response.Fail(c, http.StatusBadRequest, "invalid interval (1-3600 seconds)")
			return
		}
		if !monitorTokenRe.MatchString(token) {
			response.Fail(c, http.StatusBadRequest, "invalid token")
			return
		}

		// Build server URL from request context.
		scheme := "http"
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		serverURL := fmt.Sprintf("%s://%s/api/monitor/report", scheme, c.Request.Host)

		script := monitorAgentScript
		script = strings.ReplaceAll(script, "{{SERVER_URL}}", serverURL)
		script = strings.ReplaceAll(script, "{{TOKEN}}", token)
		script = strings.ReplaceAll(script, "{{INTERVAL}}", strconv.Itoa(n))

		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Header("Content-Disposition", `attachment; filename="monitor_agent.sh"`)
		c.String(http.StatusOK, script)
	}
}

// monitorReport receives metrics from the agent and broadcasts to dashboard.
// POST /api/monitor/report
func monitorReport(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var report ws.MonitorReportDTO
		if err := c.ShouldBindJSON(&report); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid report: "+err.Error())
			return
		}
		if deps.WsHub != nil && deps.WsHub.Monitor != nil {
			deps.WsHub.Monitor.Broadcast(report)
		}
		response.OK(c, response.Success())
	}
}
