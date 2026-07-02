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
	pro.GET("/api/mfa/status", mfaStatus(deps))
	pro.POST("/api/mfa/totp/setup", mfaTotpSetup(deps))
	pro.POST("/api/mfa/totp/verify", mfaTotpVerify(deps))
	pro.POST("/api/mfa/disable", mfaDisable(deps))
	pro.GET("/tenants/listAll", tenantList(deps))
	pro.POST("/tenants/save", tenantSave(deps))
	pro.GET("/tenants/deleteApi", tenantDelete(deps))
	pro.GET("/tenants/syncOci", tenantSyncOci(deps))
	pro.GET("/tenants/:id/instances", tenantInstances(deps))
	pro.GET("/tenants/:id", tenantGet(deps))
	pro.PUT("/tenants/:id", tenantUpdate(deps))
	pro.GET("/tenants/:id/check", tenantCheck(deps))
	pro.GET("/tenants/:id/export", tenantExport(deps))
	pro.POST("/tenants/check-batch", tenantCheckBatch(deps))
	pro.GET("/tenants/:id/email", tenantEmailGet(deps))
	pro.POST("/tenants/:id/email", tenantEmailSave(deps))
	pro.POST("/tenants/:id/email/toggle", tenantEmailToggle(deps))
	pro.DELETE("/tenants/:id/email", tenantEmailDelete(deps))
	pro.GET("/tenants/:id/social", tenantSocialList(deps))
	pro.POST("/tenants/:id/social", tenantSocialSave(deps))
	pro.PUT("/tenants/:id/social/:socialId/toggle", tenantSocialToggle(deps))
	pro.DELETE("/tenants/:id/social/:socialId", tenantSocialDelete(deps))

	// Phase 10: tenant IAM user management.
	pro.GET("/tenants/:id/users", tenantUsersList(deps))
	pro.POST("/tenants/:id/users", tenantUserCreate(deps))
	pro.DELETE("/tenants/:id/users/:userOcid", tenantUserDelete(deps))
	pro.POST("/tenants/:id/users/:userOcid/reset-password", tenantUserResetPassword(deps))
	pro.GET("/tenants/:id/groups", tenantGroupsList(deps))
	pro.GET("/tenants/:id/password-policy", tenantPasswordPolicyGet(deps))
	pro.POST("/tenants/:id/password-policy", tenantPasswordPolicyUpdate(deps))
	pro.GET("/tenants/:id/mfa/status", tenantMfaStatus(deps))
	pro.POST("/tenants/:id/mfa/toggle", tenantMfaToggle(deps))
	pro.POST("/tenants/:id/mfa/reset", tenantMfaReset(deps))
	pro.GET("/tenants/:id/notification-recipients", tenantNotifRecipientsGet(deps))
	pro.POST("/tenants/:id/notification-recipients/update", tenantNotifRecipientsUpdate(deps))
	pro.POST("/tenants/:id/update-detail", tenantUpdateDetail(deps))
	pro.GET("/tenants/:id/subscription-days", tenantSubscriptionDays(deps))
	pro.GET("/tenants/:id/domains", tenantDomainTenants(deps))
	pro.GET("/proxies/list", proxyList(deps))
	pro.POST("/proxies/save", proxySave(deps))
	pro.GET("/proxies/delete", proxyDelete(deps))
	pro.GET("/api/stats", dashboardStats(deps))

	// Phase 11.3: Security List rule management.
	pro.GET("/tenants/security-rules", getSecurityRules(deps))
	pro.POST("/tenants/security-rules", addSecurityRule(deps))
	pro.DELETE("/tenants/security-rules/:id", deleteSecurityRule(deps))
	pro.POST("/tenants/enableAll", batchEnableAll(deps))

	// Phase 11.4: Quota, Region Subscription, Audit Log.
	pro.GET("/tenants/:id/quota", tenantQuota(deps))
	pro.GET("/tenants/:id/regions/summary", tenantRegionSummary(deps))
	pro.GET("/tenants/:id/regions/subscribed", tenantRegionsSubscribed(deps))
	pro.GET("/tenants/:id/regions/unsubscribed", tenantRegionsUnsubscribed(deps))
	pro.POST("/tenants/:id/regions/subscribe", tenantRegionsSubscribe(deps))
	pro.GET("/tenants/:id/regions/subscription-status", tenantRegionSubStatus(deps))
	pro.POST("/tenants/:id/audit-log", tenantAuditLog(deps))

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
	pro.POST("/instances/:id/restart", instanceRestart(deps))
	pro.POST("/instances/:id/vpu", instanceUpdateVpu(deps))
	pro.GET("/instances/:id/ssh-config", instanceGetSSHConfig(deps))
	pro.POST("/instances/:id/ssh-config", instanceSaveSSHConfig(deps))
	pro.GET("/instances/:id/console-connections", instanceConsoleConnectionsList(deps))
	pro.DELETE("/instances/:id/console-connections/:connId", instanceConsoleConnectionDelete(deps))
	pro.GET("/ssh-keys", sshKeysList(deps))
	pro.POST("/ssh-keys", sshKeyCreate(deps))
	pro.DELETE("/ssh-keys/:id", sshKeyDelete(deps))
	pro.GET("/backup/list", backupList(deps))
	pro.GET("/backup/delete", backupDelete(deps))
	pro.GET("/traffic/alert/list", trafficAlertList(deps))
	pro.GET("/traffic/alert/get", trafficAlertGet(deps))
	pro.POST("/traffic/alert/save", trafficAlertSave(deps))

	// Phase 6: WebSocket endpoints. Registered under the protected group so
	// SessionAuth runs on the upgrade handshake (previously on the root router,
	// which bypassed auth). Handler closures are unchanged; CheckOrigin is
	// already handled by the ws-agent.
	pro.GET("/ws/ssh", func(c *gin.Context) { deps.WsHub.SSH.HandleSSH(c.Writer, c.Request) })
	pro.GET("/log/ws", func(c *gin.Context) { deps.WsHub.Log.HandleLog(c.Writer, c.Request) })
	pro.GET("/ws/monitor", func(c *gin.Context) { deps.WsHub.Monitor.HandleMonitor(c.Writer, c.Request) })
	pro.GET("/ws/console", func(c *gin.Context) { deps.WsHub.Console.HandleConsole(c.Writer, c.Request) })
	pro.GET("/ws/console/serial", func(c *gin.Context) { deps.WsHub.Console.HandleSerialConsole(c.Writer, c.Request) })
	pro.GET("/ws/vnc/:instanceId", func(c *gin.Context) {
		// Pass instanceId via query param so the handler can read it from standard request.
		q := c.Request.URL.Query()
		q.Set("instanceId", c.Param("instanceId"))
		c.Request.URL.RawQuery = q.Encode()
		deps.WsHub.Console.HandleVNCBridge(c.Writer, c.Request)
	})
	pro.GET("/ws/rescue", func(c *gin.Context) { deps.WsHub.Rescue.HandleRescue(c.Writer, c.Request) })

	// Phase 6: monitor agent endpoints (public — agent runs on remote VPS).
	pub.GET("/api/monitor/download", monitorDownload(deps))
	pub.POST("/api/monitor/report", monitorReport(deps))

	// Phase 7: DNS, SSL, system config.
	pro.POST("/system/config/save", systemConfigSave(deps))
	pro.GET("/ssl/list", sslList(deps))
	pro.POST("/ssl/issue", sslIssue(deps))
	pro.GET("/system/config", systemConfigGet(deps))

	// Phase 3: System outbound proxy configuration.
	pro.GET("/system/proxy", systemProxyGet(deps))
	pro.PUT("/system/proxy", systemProxyUpdate(deps))
	pro.POST("/system/proxy/test", systemProxyTest(deps))

	// Phase 3: Unified system settings API.
	pro.GET("/system/settings", systemSettingsGet(deps))
	pro.PUT("/system/settings", systemSettingsUpdate(deps))

	// Cloudflare DNS management (Phase 7).
	pro.GET("/dns/cloudflare/zones", cloudflareZones(deps))
	pro.GET("/dns/cloudflare/zones/:zoneId/records", cloudflareRecords(deps))
	pro.POST("/dns/cloudflare/zones/:zoneId/records", cloudflareCreateRecord(deps))
	pro.PUT("/dns/cloudflare/zones/:zoneId/records/:recordId", cloudflareUpdateRecord(deps))
	pro.DELETE("/dns/cloudflare/zones/:zoneId/records/:recordId", cloudflareDeleteRecord(deps))

	// EdgeOne DNS management (Phase 7/8).
	pro.GET("/dns/edgeone/zones", edgeoneZones(deps))
	pro.GET("/dns/edgeone/records", edgeoneRecords(deps))
	pro.POST("/dns/edgeone/records", edgeoneCreateRecord(deps))
	pro.PUT("/dns/edgeone/records/:recordId", edgeoneUpdateRecord(deps))
	pro.DELETE("/dns/edgeone/records/:recordId", edgeoneDeleteRecord(deps))

	// Phase 8: data migration (import H2 exports into SQLite).
	pro.POST("/migration/import", deps.Migration.ImportPlain)
	pro.POST("/migration/import-encrypted", deps.Migration.ImportEncrypted)
	pro.GET("/migration/export", deps.Migration.ExportPlain)

	// Phase 11.1: Object Storage.
	pro.GET("/oci/storage/namespace", objectStorageNamespace(deps))
	pro.GET("/oci/storage/buckets", objectStorageListBuckets(deps))
	pro.POST("/oci/storage/bucket/create", objectStorageCreateBucket(deps))
	pro.POST("/oci/storage/bucket/delete", objectStorageDeleteBucket(deps))
	pro.GET("/oci/storage/objects", objectStorageListObjects(deps))
	pro.POST("/oci/storage/object/delete", objectStorageDeleteObject(deps))
	pro.POST("/oci/storage/object/upload", objectStorageUpload(deps))
	pro.GET("/oci/storage/object/download", objectStorageDownload(deps))
	pro.GET("/oci/storage/object/preview", objectStoragePreview(deps))
	pro.POST("/oci/storage/object/presigned", objectStoragePresigned(deps))
	pro.POST("/oci/storage/object/multipart/initiate", objectStorageMultipartInitiate(deps))
	pro.POST("/oci/storage/object/multipart/part", objectStorageMultipartPart(deps))
	pro.POST("/oci/storage/object/multipart/commit", objectStorageMultipartCommit(deps))
	pro.POST("/oci/storage/object/multipart/abort", objectStorageMultipartAbort(deps))
	pro.GET("/oci/storage/object/multipart/resumeable", objectStorageMultipartResumable(deps))

	// Phase 2: Shape and Image listing.
	pro.GET("/oci/shapes", listShapes(deps))
	pro.GET("/oci/images", listImages(deps))

	// Phase 11.2: VNIC batch management.
	pro.GET("/oci/vnic/loadData", vnicLoadData(deps))
	pro.POST("/oci/vnic/create", vnicCreate(deps))
	pro.POST("/oci/vnic/delete", vnicDelete(deps))
	pro.POST("/oci/vnic/createIpv6", vnicCreateIpv6(deps))
	pro.POST("/oci/vnic/deleteIpv6", vnicDeleteIpv6(deps))
	pro.POST("/oci/vnic/deleteAllSecondary", vnicDeleteAllSecondary(deps))
	pro.GET("/oci/vnic/refresh", vnicRefresh(deps))
	pro.POST("/oci/vnic/changeSpecIp", vnicChangeSpecIp(deps))
	pro.POST("/oci/vnic/network/configureLoadBalancer", vnicConfigureLB(deps))
	pro.POST("/oci/vnic/network/restoreNetwork", vnicRestoreNetwork(deps))

	// Phase 12.2: Email Delivery management.
	pro.POST("/api/email/receive/list", emailReceiveList(deps))
	pro.POST("/api/email/receive/add", emailReceiveAdd(deps))
	pro.POST("/api/email/receive/delete", emailReceiveDelete(deps))
	pro.POST("/api/email/receive/get", emailReceiveGet(deps))
	pro.POST("/api/email/send", emailSend(deps))
	pro.POST("/api/email/body/list", emailBodyList(deps))
	pro.POST("/api/email/body/delete", emailBodyDelete(deps))
	pro.POST("/api/email/body/batchDelete", emailBodyBatchDelete(deps))
	pro.POST("/api/email/send/list", emailSendRecordList(deps))
	pro.POST("/api/email/tenant/list", tenantEmailConfigList(deps))
	pro.POST("/api/email/tenant/get", tenantEmailConfigGet(deps))
	pro.POST("/api/email/enable", emailEnable(deps))
	pro.POST("/api/email/disable", emailDisable(deps))

	// Phase B: Billing (subscription + usage/cost).
	pro.GET("/tenants/:id/subscription", billingSubscription(deps))
	pro.GET("/tenants/:id/cost", billingCost(deps))

	// SPA static assets + NoRoute fallback to index.html
	web.Register(r)
	return r
}
