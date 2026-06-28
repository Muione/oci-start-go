// Package httpapi wires the Gin engine, middleware, and routes.
// See SPEC §6 (route groups) — Phase 2 adds auth + base JSON API.
package httpapi

import (
	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/auth"
	logpkg "github.com/Muione/oci-start-go/internal/util/log"
	"github.com/Muione/oci-start-go/internal/web"
)

// NewServer builds the Gin engine with recovery + traceId + IpBan (all routes),
// public routes (whitelist), protected routes (SessionAuth/UserContext/TenantContext),
// and the SPA NoRoute fallback.
func NewServer(deps *Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(Recovery(), logpkg.TraceIDMiddleware())
	r.Use(auth.IpBan(deps.Store)) // @CheckIpBan parity: applies to all routes

	// public routes (Sa-Token exclude list)
	pub := r.Group("/")
	pub.GET("/healthz", healthz)
	pub.GET("/api/login/init", loginInit(deps))
	pub.POST("/api/login", login(deps))
	pub.POST("/api/logout", logout(deps))
	pub.GET("/api/config/initialized", isInitialized(deps))
	pub.POST("/api/register-first-user", registerFirstUser(deps))
	pub.GET("/api/disTurnstile", disTurnstile(deps))
	pub.GET("/api/config/mfa-enabled", configMfaEnabled(deps))
	pub.GET("/api/config/turnstile", configTurnstile(deps))
	pub.GET("/api/config/message-enabled", configMessageEnabled(deps))
	pub.GET("/api/github/login/url", githubLoginURL(deps))
	pub.GET("/api/github/callback", githubCallback(deps))
	pub.GET("/api/github/status", githubStatus(deps))
	pub.GET("/api/google/login/url", googleLoginURL(deps))
	pub.GET("/api/google/callback", googleCallback(deps))
	pub.GET("/api/google/status", googleStatus(deps))
	pub.POST("/api/send-reset-code", sendResetCode(deps))
	pub.POST("/api/verify-reset-code", verifyResetCode(deps))
	pub.POST("/api/reset-password", resetPassword(deps))

	// protected routes
	pro := r.Group("/")
	pro.Use(auth.SessionAuth(deps.Session), auth.UserContext(), auth.TenantContext(deps.Store))
	pro.GET("/api/version", versionHandler(deps.Store))
	pro.GET("/api/userInfo", userInfo(deps))
	pro.POST("/api/change-password", changePassword(deps))
	pro.GET("/tenants/listAll", tenantList(deps))
	pro.POST("/tenants/save", tenantSave(deps))
	pro.GET("/tenants/deleteApi", tenantDelete(deps))
	pro.GET("/tenants/syncOci", tenantSyncOci(deps))
	pro.GET("/tenants/:id/instances", tenantInstances(deps))
	pro.GET("/proxies/list", proxyList(deps))
	pro.POST("/proxies/save", proxySave(deps))
	pro.GET("/proxies/delete", proxyDelete(deps))
	pro.GET("/api/stats", dashboardStats(deps))

	// Phase 4: boot task CRUD + grabber system status.
	pro.GET("/boot/list", bootList(deps))
	pro.POST("/boot/save", bootSave(deps))
	pro.GET("/boot/delete", bootDelete(deps))
	pro.GET("/boot/toggle", bootToggle(deps))
	pro.GET("/boot/systemStatus", bootSystemStatus(deps))
	pro.GET("/boot/tenants", bootTenantList(deps))

	// Phase 5: instance details + traffic + backups.
	pro.GET("/instances/list", instanceList(deps))
	pro.GET("/instances/export", instanceExport(deps))
	pro.GET("/instances/traffic", instanceTraffic(deps))
	pro.GET("/instances/:id", instanceGet(deps))
	pro.POST("/instances/:id/remark", instanceUpdateRemark(deps))
	pro.POST("/instances/:id/modify", instanceModify(deps))
	pro.POST("/instances/:id/start", instanceStart(deps))
	pro.POST("/instances/:id/stop", instanceStop(deps))
	pro.POST("/instances/:id/terminate", instanceTerminate(deps))
	pro.DELETE("/instances/:id", instanceDeleteRecord(deps))
	pro.POST("/instances/:id/change-ip", instanceChangeIP(deps))
	pro.POST("/instances/:id/enable-ipv6", instanceEnableIPv6(deps))
	pro.GET("/instances/:id/ssh-config", instanceGetSSHConfig(deps))
	pro.POST("/instances/:id/ssh-config", instanceSaveSSHConfig(deps))
	pro.GET("/backup/list", backupList(deps))
	pro.GET("/backup/delete", backupDelete(deps))
	pro.POST("/boot-instance/gcp/launch", gcpBootLaunch(deps))
	pro.GET("/boot-instance/gcp/list", gcpBootList(deps))
	pro.GET("/boot-instance/gcp/delete", gcpBootDelete(deps))
	pro.GET("/boot-instance/gcp/status", gcpBootStatus(deps))
	pro.GET("/traffic/alert/list", trafficAlertList(deps))
	pro.GET("/traffic/alert/get", trafficAlertGet(deps))
	pro.POST("/traffic/alert/save", trafficAlertSave(deps))

	// Phase 6: WebSocket endpoints (no auth — WS upgrade handshake is separate).
	r.GET("/ws/ssh", func(c *gin.Context) { deps.WsHub.SSH.HandleSSH(c.Writer, c.Request) })
	r.GET("/log/ws", func(c *gin.Context) { deps.WsHub.Log.HandleLog(c.Writer, c.Request) })
	r.GET("/ws/monitor", func(c *gin.Context) { deps.WsHub.Monitor.HandleMonitor(c.Writer, c.Request) })
	r.GET("/ws/console", func(c *gin.Context) { deps.WsHub.Console.HandleConsole(c.Writer, c.Request) })
	r.GET("/ws/rescue", func(c *gin.Context) { deps.WsHub.Rescue.HandleRescue(c.Writer, c.Request) })

	// Phase 6: monitor agent endpoints (public — agent runs on remote VPS).
	pub.GET("/api/monitor/download", monitorDownload(deps))
	pub.POST("/api/monitor/report", monitorReport(deps))

	// Phase 7: DNS, SSL, system config.
	pro.GET("/dns/list", dnsList(deps))
	pro.POST("/dns/save", dnsSave(deps))
	pro.GET("/dns/delete", dnsDelete(deps))
	pro.POST("/system/config/save", systemConfigSave(deps))
	pro.POST("/dns/sync", dnsSync(deps))
	pro.GET("/ssl/list", sslList(deps))
	pro.POST("/ssl/issue", sslIssue(deps))
	pro.GET("/system/config", systemConfigGet(deps))

	// Cloudflare DNS management (Phase 7).
	pro.GET("/dns/cloudflare/zones", cloudflareZones(deps))
	pro.GET("/dns/cloudflare/zones/:zoneId/records", cloudflareRecords(deps))
	pro.POST("/dns/cloudflare/zones/:zoneId/records", cloudflareCreateRecord(deps))
	pro.PUT("/dns/cloudflare/zones/:zoneId/records/:recordId", cloudflareUpdateRecord(deps))
	pro.DELETE("/dns/cloudflare/zones/:zoneId/records/:recordId", cloudflareDeleteRecord(deps))
	pro.POST("/dns/cloudflare/zones/:zoneId/sync", cloudflareSyncZone(deps))

	// EdgeOne DNS management (Phase 7/8).
	pro.GET("/dns/edgeone/zones", edgeoneZones(deps))
	pro.GET("/dns/edgeone/records", edgeoneRecords(deps))
	pro.POST("/dns/edgeone/records", edgeoneCreateRecord(deps))
	pro.PUT("/dns/edgeone/records/:recordId", edgeoneUpdateRecord(deps))
	pro.DELETE("/dns/edgeone/records/:recordId", edgeoneDeleteRecord(deps))
	pro.POST("/dns/edgeone/sync", edgeoneSync(deps))

	// Phase 8: data migration (import H2 exports into SQLite).
	pro.POST("/migration/import", deps.Migration.ImportPlain)
	pro.POST("/migration/import-encrypted", deps.Migration.ImportEncrypted)
	pro.GET("/migration/export", deps.Migration.ExportPlain)

	// SPA static assets + NoRoute fallback to index.html
	web.Register(r)
	return r
}
