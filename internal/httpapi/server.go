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
	pro.POST("/system/config/save", systemConfigSave(deps))
	pro.GET("/ssl/list", sslList(deps))
	pro.POST("/ssl/issue", sslIssue(deps))
	pro.GET("/system/config", systemConfigGet(deps))

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

	// Phase 12.1: Nginx / Reverse Proxy management.
	pro.POST("/ssl/proxy/create", nginxCreateProxy(deps))
	pro.PUT("/ssl/proxy/:id", nginxUpdateProxy(deps))
	pro.GET("/ssl/proxy/:id", nginxGetProxy(deps))
	pro.DELETE("/ssl/proxy/:id", nginxDeleteProxy(deps))
	pro.GET("/ssl/proxy/list", nginxListProxies(deps))
	pro.DELETE("/ssl/proxy/batch", nginxBatchDeleteProxies(deps))
	pro.PUT("/ssl/proxy/:id/toggle", nginxToggleProxy(deps))
	pro.POST("/ssl/proxy/:id/test-connection", nginxTestProxyConnection(deps))
	pro.POST("/ssl/proxy/:id/ssl", nginxApplySsl(deps))
	pro.POST("/ssl/proxy/:id/fix", nginxFixProxy(deps))
	pro.POST("/ssl/certificates/request", nginxRequestCert(deps))
	pro.POST("/ssl/certificates/:id/renew", nginxRenewCert(deps))
	pro.DELETE("/ssl/certificates/:id", nginxDeleteCert(deps))
	pro.PUT("/ssl/certificates/:id/auto-renew", nginxToggleAutoRenew(deps))
	pro.GET("/ssl/certificates/list", nginxListCerts(deps))
	pro.GET("/ssl/certificates/expiring", nginxExpiringCerts(deps))
	pro.GET("/ssl/certificates/:id/download", nginxDownloadCert(deps))
	pro.GET("/ssl/certificates/match", nginxMatchCerts(deps))
	pro.POST("/ssl/nginx/generate", nginxGenerateConfig(deps))
	pro.POST("/ssl/nginx/:id/apply", nginxApplyConfig(deps))
	pro.POST("/ssl/nginx/:id/test", nginxTestConfig(deps))
	pro.POST("/ssl/nginx/reload", nginxReload(deps))
	pro.GET("/ssl/nginx/diff", nginxConfigDiff(deps))
	pro.GET("/ssl/nginx/status", nginxStatus(deps))
	pro.GET("/ssl/nginx/latest", nginxLatestConfig(deps))
	pro.GET("/ssl/openresty/status", openrestyStatus(deps))
	pro.POST("/ssl/openresty/start", openrestyStart(deps))

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

	// Phase 13.3: NoSQL Database management.
	pro.GET("/oci/nosql/tables", nosqlTableList(deps))
	pro.GET("/oci/nosql/table/get", nosqlTableGet(deps))
	pro.POST("/oci/nosql/table/create", nosqlTableCreate(deps))
	pro.POST("/oci/nosql/table/delete", nosqlTableDelete(deps))
	pro.POST("/oci/nosql/row/get", nosqlRowGet(deps))
	pro.POST("/oci/nosql/row/put", nosqlRowPut(deps))
	pro.POST("/oci/nosql/row/delete", nosqlRowDelete(deps))
	pro.POST("/oci/nosql/query", nosqlQuery(deps))

	// Phase 13.3: MySQL Database Service management.
	pro.GET("/oci/mysql/db-systems", mysqlDbSystemList(deps))
	pro.GET("/oci/mysql/db-system/get", mysqlDbSystemGet(deps))
	pro.POST("/oci/mysql/db-system/create", mysqlDbSystemCreate(deps))
	pro.POST("/oci/mysql/db-system/delete", mysqlDbSystemDelete(deps))
	pro.POST("/oci/mysql/db-system/start", mysqlDbSystemStart(deps))
	pro.POST("/oci/mysql/db-system/stop", mysqlDbSystemStop(deps))
	pro.POST("/oci/mysql/db-system/restart", mysqlDbSystemRestart(deps))
	pro.GET("/oci/mysql/backups", mysqlBackupList(deps))
	pro.POST("/oci/mysql/backup/create", mysqlBackupCreate(deps))
	pro.POST("/oci/mysql/backup/delete", mysqlBackupDelete(deps))
	pro.GET("/oci/mysql/channels", mysqlChannelList(deps))
	pro.POST("/oci/mysql/channel/delete", mysqlChannelDelete(deps))

	// Phase 13.3: Resource Manager management.
	pro.GET("/oci/resourcemgr/stacks", resMgrStackList(deps))
	pro.GET("/oci/resourcemgr/stack/get", resMgrStackGet(deps))
	pro.POST("/oci/resourcemgr/stack/delete", resMgrStackDelete(deps))
	pro.POST("/oci/resourcemgr/job/create", resMgrJobCreate(deps))
	pro.GET("/oci/resourcemgr/job/get", resMgrJobGet(deps))
	pro.GET("/oci/resourcemgr/jobs", resMgrJobList(deps))
	pro.GET("/oci/resourcemgr/job/logs", resMgrJobLogs(deps))
	pro.POST("/oci/resourcemgr/job/cancel", resMgrJobCancel(deps))

	// Phase 14.1: Bastion management.
	pro.GET("/oci/bastion/list", bastionList(deps))
	pro.POST("/oci/bastion/session/create", bastionSessionCreate(deps))
	pro.GET("/oci/bastion/session/list", bastionSessionList(deps))
	pro.GET("/oci/bastion/session/get", bastionSessionGet(deps))
	pro.POST("/oci/bastion/session/delete", bastionSessionDelete(deps))

	// Phase 14.2: Container Registry management.
	pro.GET("/oci/container/repositories", ctrRegListRepos(deps))
	pro.GET("/oci/container/images", ctrRegListImages(deps))
	pro.POST("/oci/container/image/delete", ctrRegDeleteImage(deps))
	pro.POST("/oci/container/repository/delete", ctrRegDeleteRepo(deps))
	pro.POST("/oci/container/cleanup", ctrRegCleanup(deps))

	// Phase 14.3: AI Vision management.
	pro.POST("/oci/aivision/image/analyze", aiVisionAnalyzeImage(deps))
	pro.POST("/oci/aivision/document/analyze", aiVisionAnalyzeDocument(deps))
	pro.POST("/oci/aivision/video/create", aiVisionCreateVideoJob(deps))
	pro.GET("/oci/aivision/video/status", aiVisionGetVideoJob(deps))
	pro.POST("/oci/aivision/video/cancel", aiVisionCancelVideoJob(deps))

	// SPA static assets + NoRoute fallback to index.html
	web.Register(r)
	return r
}
