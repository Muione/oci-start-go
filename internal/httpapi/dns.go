// Package httpapi — dns.go: DNS record management handlers (Phase 7/8).
package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/dns"
	"github.com/Muione/oci-start-go/internal/response"
)

// dnsList returns local DNS records.
// GET /dns/list
func dnsList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.DnsSvc == nil {
			response.Fail(c, http.StatusServiceUnavailable, "DNS service not available")
			return
		}
		records, err := deps.DnsSvc.List(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list dns: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(records))
	}
}

// dnsSync syncs DNS records from Cloudflare.
// POST /dns/sync  {zoneId: "..."}
func dnsSync(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct{ ZoneID string `json:"zoneId"` }
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid body")
			return
		}
		// Get Cloudflare config from system config.
		cfEmail := deps.SysConf.GetString(c.Request.Context(), "cloudflare.email")
		cfKey := deps.SysConf.GetString(c.Request.Context(), "cloudflare.api.key")
		if cfEmail == "" || cfKey == "" {
			response.Fail(c, http.StatusBadRequest, "Cloudflare not configured (set cloudflare.email + cloudflare.api.key)")
			return
		}
		if deps.DnsSvc == nil {
			response.Fail(c, http.StatusServiceUnavailable, "DNS service not available")
			return
		}

		client := dns.NewCfClient(cfEmail, cfKey)
		ctx := c.Request.Context()

		// Sync specific zone if provided, otherwise sync all zones.
		if in.ZoneID != "" {
			count, err := deps.DnsSvc.SyncFromCloudflare(ctx, client, in.ZoneID)
			if err != nil {
				response.Fail(c, http.StatusInternalServerError, "sync zone failed: "+err.Error())
				return
			}
			response.OK(c, response.SuccessData(gin.H{
				"synced": count,
				"zoneId": in.ZoneID,
			}))
			return
		}

		zones, err := client.ListZones(ctx)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list zones failed: "+err.Error())
			return
		}

		totalSynced := 0
		for _, zone := range zones {
			count, err := deps.DnsSvc.SyncFromCloudflare(ctx, client, zone.ID)
			if err != nil {
				continue
			}
			totalSynced += count
		}

		response.OK(c, response.SuccessData(gin.H{
			"synced": totalSynced,
			"zones":  len(zones),
		}))
	}
}

// dnsSave creates or updates a local DNS record.
// POST /dns/save
func dnsSave(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in dns.DnsRecordResp
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid body")
			return
		}
		if deps.DnsSvc == nil {
			response.Fail(c, http.StatusServiceUnavailable, "DNS service not available")
			return
		}
		if err := deps.DnsSvc.Save(c.Request.Context(), in); err != nil {
			response.Fail(c, http.StatusInternalServerError, "save dns: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// dnsDelete removes a local DNS record.
// GET /dns/delete?id=
func dnsDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Query("id"), 10, 64)
		if err != nil || id <= 0 {
			response.Fail(c, http.StatusBadRequest, "valid id required")
			return
		}
		if deps.DnsSvc == nil {
			response.Fail(c, http.StatusServiceUnavailable, "DNS service not available")
			return
		}
		if err := deps.DnsSvc.Delete(c.Request.Context(), id); err != nil {
			response.Fail(c, http.StatusInternalServerError, "delete dns: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// sslList returns configured SSL certificate status.
// GET /ssl/list
func sslList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		domain := deps.SysConf.GetString(ctx, "ssl.domain")
		email := deps.SysConf.GetString(ctx, "ssl.email")
		staging := deps.SysConf.GetBool(ctx, "ssl.staging")

		certs := []gin.H{}
		if domain != "" {
			certs = append(certs, gin.H{
				"domain":  domain,
				"email":   email,
				"staging": staging,
				"status":  "configured",
			})
		}

		response.OK(c, response.SuccessData(gin.H{
			"certs":      certs,
			"configured": domain != "" && email != "",
			"staging":    staging,
		}))
	}
}

// sslIssue obtains a new SSL certificate via Let's Encrypt.
// POST /ssl/issue  {domain: "...", email: "..."}
func sslIssue(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			Domain string `json:"domain"`
			Email  string `json:"email"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid body: domain and email required")
			return
		}
		if strings.TrimSpace(in.Domain) == "" || strings.TrimSpace(in.Email) == "" {
			response.Fail(c, http.StatusBadRequest, "domain and email are required")
			return
		}

		ctx := c.Request.Context()
		cfEmail := deps.SysConf.GetString(ctx, "cloudflare.email")
		cfKey := deps.SysConf.GetString(ctx, "cloudflare.api.key")
		staging := deps.SysConf.GetBool(ctx, "ssl.staging")

		if cfEmail == "" || cfKey == "" {
			response.Fail(c, http.StatusBadRequest, "Cloudflare API credentials not configured (cloudflare.email + cloudflare.api.key)")
			return
		}

		if deps.CertManager == nil {
			response.Fail(c, http.StatusServiceUnavailable, "certificate manager not configured")
			return
		}

		// Actually obtain the certificate via ACME.
		result, err := deps.CertManager.ObtainCertificate(ctx,
			in.Domain, in.Email, cfEmail, cfKey, staging)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "SSL certificate issuance failed: "+err.Error())
			return
		}

		// Store the certificate and key in system config.
		_ = deps.SysConf.SetString(ctx, "ssl.domain", in.Domain)
		_ = deps.SysConf.SetString(ctx, "ssl.email", in.Email)
		_ = deps.SysConf.SetString(ctx, "ssl.certificate", result.Certificate)
		_ = deps.SysConf.SetString(ctx, "ssl.privateKey", result.PrivateKey)
		_ = deps.SysConf.SetString(ctx, "ssl.notAfter", result.NotAfter)

		response.OK(c, response.SuccessData(gin.H{
			"domain":   result.Domain,
			"notAfter": result.NotAfter,
			"staging":  staging,
			"message":  "SSL certificate issued successfully. Restart the server to apply.",
		}))
	}
}

// systemConfigGet returns all system configuration KV pairs.
// GET /system/config
func systemConfigGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// List commonly-used config keys.
		keys := []string{
			"app.version",
			"telegram.bot.token",
			"telegram.chat.id",
			"dingtalk.webhook",
			"dingtalk.secret",
			"bark.server",
			"bark.key",
			"feishu.webhook",
			"feishu.secret",
			"cloudflare.email",
			"cloudflare.api.key",
			"edgeone.secretId",
			"edgeone.secretKey",
			"edgeone.zoneId",
			"ssl.domain",
			"ssl.email",
			"ssl.staging",
			"gcp.serviceAccountJson",
			"gcp.projectId",
			"turnstile.enabled",
			"turnstile.siteKey",
			"turnstile.secretKey",
			"mfa.enabled",
			"github.clientId",
			"github.clientSecret",
			"google.clientId",
			"google.clientSecret",
			"message.enabled",
		}

		configs := make(map[string]interface{})
		for _, key := range keys {
			val := deps.SysConf.GetString(ctx, key)
			if val != "" {
				configs[key] = val
			}
		}

		// Also check boolean configs.
		boolKeys := []string{
			"turnstile.enabled",
			"mfa.enabled",
			"message.enabled",
			"ssl.staging",
		}
		bools := make(map[string]bool)
		for _, key := range boolKeys {
			bools[key] = deps.SysConf.GetBool(ctx, key)
		}

		response.OK(c, response.SuccessData(gin.H{
			"strings": configs,
			"booleans": bools,
			"appVersion": deps.SysConf.GetString(ctx, "app.version"),
		}))
	}
}

// systemConfigSave updates a single system config key-value pair.
// POST /system/config/save  {key: "...", value: "..."}
func systemConfigSave(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := c.ShouldBindJSON(&in); err != nil || in.Key == "" {
			response.Fail(c, http.StatusBadRequest, "key and value are required")
			return
		}
		if err := deps.SysConf.SetString(c.Request.Context(), in.Key, in.Value); err != nil {
			response.Fail(c, http.StatusInternalServerError, "save config failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(gin.H{"key": in.Key, "saved": true}))
	}
}
