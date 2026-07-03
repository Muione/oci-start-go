// Package httpapi — dns.go: DNS record management handlers (Phase 7/8).
// DNS records are managed directly against Cloudflare / EdgeOne APIs
// without local database storage. Cloudflare uses API Token auth (not
// the legacy Global API Key).
package httpapi

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/acme"
	"github.com/Muione/oci-start-go/internal/dns"
	"github.com/Muione/oci-start-go/internal/response"
)

// sslList returns configured SSL certificate status.
// GET /ssl/list
func sslList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		domain := deps.SysConf.GetString(ctx, "ssl.domain")
		email := deps.SysConf.GetString(ctx, "ssl.email")
		staging := deps.SysConf.GetBool(ctx, "ssl.staging")
		notAfter := deps.SysConf.GetString(ctx, "ssl.notAfter")

		certs := []gin.H{}
		if domain != "" {
			cert := gin.H{
				"domain":  domain,
				"email":   email,
				"staging": staging,
				"status":  "configured",
			}
			if notAfter != "" {
				cert["notAfter"] = notAfter
			}
			certs = append(certs, cert)
		}

		response.OK(c, response.SuccessData(gin.H{
			"certs":      certs,
			"configured": domain != "" && email != "",
			"staging":    staging,
			"notAfter":   notAfter,
		}))
	}
}

// certObtain is the seam for ACME certificate issuance; defaults to
// deps.CertManager.ObtainCertificate. Overridable in tests to avoid network.
// ponytail: test seam only.
var certObtain = func(deps *Deps, ctx context.Context, domain, email, cfToken string, staging bool) (*acme.CertResult, error) {
	return deps.CertManager.ObtainCertificate(ctx, domain, email, cfToken, staging)
}

// persistCert stores the issued cert material in system_config. Returns the
// first persistence error so the caller can surface a 500 after a successful
// issuance (otherwise a restart would silently lose HTTPS).
func persistCert(deps *Deps, ctx context.Context, domain, email string, result *acme.CertResult) error {
	sets := []struct{ key, val string }{
		{"ssl.domain", domain},
		{"ssl.email", email},
		{"ssl.certificate", result.Certificate},
		{"ssl.privateKey", result.PrivateKey},
		{"ssl.notAfter", result.NotAfter},
	}
	for _, s := range sets {
		if err := deps.SysConf.SetString(ctx, s.key, s.val); err != nil {
			return err
		}
	}
	return nil
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
		if in.Domain == "" || in.Email == "" {
			response.Fail(c, http.StatusBadRequest, "domain and email are required")
			return
		}

		ctx := c.Request.Context()
		cfToken := deps.SysConf.GetString(ctx, "cloudflare.api.token")
		staging := deps.SysConf.GetBool(ctx, "ssl.staging")

		if cfToken == "" {
			response.Fail(c, http.StatusBadRequest, "Cloudflare API Token not configured (cloudflare.api.token)")
			return
		}

		if deps.CertManager == nil {
			response.Fail(c, http.StatusServiceUnavailable, "certificate manager not configured")
			return
		}

		result, err := certObtain(deps, ctx, in.Domain, in.Email, cfToken, staging)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "SSL certificate issuance failed: "+err.Error())
			return
		}

		// E5: persist cert material; a failure here means a restart would lose
		// HTTPS even though issuance succeeded — surface as 500, not success.
		if err := persistCert(deps, ctx, in.Domain, in.Email, result); err != nil {
			deps.Logger.Error().Err(err).Str("domain", in.Domain).Msg("ssl: persist cert failed")
			response.Fail(c, http.StatusInternalServerError, "cert issued but persistence failed")
			return
		}

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
			"cloudflare.api.token",
			"edgeone.secretId",
			"edgeone.secretKey",
			"edgeone.zoneId",
			"ssl.domain",
			"ssl.email",
			"ssl.staging",
			"ssl.notAfter",
			"ssl.certificate",
			"turnstile.enabled",
			"turnstile.site.key",
			"turnstile.secret.key",
			"mfa.enabled",
			"github.client.id",
			"github.client.secret",
			"github.redirect.uri",
			"google.client.id",
			"google.client.secret",
			"google.redirect.uri",
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
			"github.enabled",
			"google.enabled",
		}
		bools := make(map[string]bool)
		for _, key := range boolKeys {
			bools[key] = deps.SysConf.GetBool(ctx, key)
		}

		response.OK(c, response.SuccessData(gin.H{
			"strings":    configs,
			"booleans":   bools,
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

// cloudflareZones returns all Cloudflare zones.
// GET /dns/cloudflare/zones
func cloudflareZones(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		cfToken := deps.SysConf.GetString(ctx, "cloudflare.api.token")
		if cfToken == "" {
			response.Fail(c, http.StatusBadRequest, "Cloudflare not configured (set cloudflare.api.token)")
			return
		}
		cache := dns.GetOrCreateCache(cfToken)
		zones, err := cache.ListZones(ctx)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list zones: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(zones))
	}
}

// cloudflareRecords returns paginated DNS records for a Cloudflare zone.
// GET /dns/cloudflare/zones/:zoneId/records?page=1&perPage=20&type=&name=&content=
func cloudflareRecords(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		zoneID := c.Param("zoneId")
		if zoneID == "" {
			response.Fail(c, http.StatusBadRequest, "zoneId required")
			return
		}
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		perPage, _ := strconv.Atoi(c.DefaultQuery("perPage", "20"))
		recordType := c.Query("type")
		name := c.Query("name")
		content := c.Query("content")

		ctx := c.Request.Context()
		cfToken := deps.SysConf.GetString(ctx, "cloudflare.api.token")
		if cfToken == "" {
			response.Fail(c, http.StatusBadRequest, "Cloudflare not configured (set cloudflare.api.token)")
			return
		}
		cache := dns.GetOrCreateCache(cfToken)
		records, resp, err := cache.RawClient().ListDnsRecordsPage(ctx, zoneID, page, perPage, recordType, name, content)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list records: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(gin.H{
			"records":    records,
			"page":       resp.ResultInfo.Page,
			"perPage":    resp.ResultInfo.PerPage,
			"totalPages": resp.ResultInfo.TotalPages,
			"totalCount": resp.ResultInfo.TotalCount,
		}))
	}
}

// cloudflareCreateRecord creates a DNS record in Cloudflare.
// POST /dns/cloudflare/zones/:zoneId/records
func cloudflareCreateRecord(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		zoneID := c.Param("zoneId")
		if zoneID == "" {
			response.Fail(c, http.StatusBadRequest, "zoneId required")
			return
		}
		var body struct {
			Type    string `json:"type"`
			Name    string `json:"name"`
			Content string `json:"content"`
			TTL     int    `json:"ttl"`
			Proxied bool   `json:"proxied"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid body")
			return
		}
		if body.Type == "" || body.Name == "" || body.Content == "" {
			response.Fail(c, http.StatusBadRequest, "type, name, content are required")
			return
		}

		ctx := c.Request.Context()
		cfToken := deps.SysConf.GetString(ctx, "cloudflare.api.token")
		if cfToken == "" {
			response.Fail(c, http.StatusBadRequest, "Cloudflare not configured (set cloudflare.api.token)")
			return
		}
		cache := dns.GetOrCreateCache(cfToken)
		record, err := cache.CreateRecord(ctx, zoneID, dns.DnsRecord{
			Type:    body.Type,
			Name:    body.Name,
			Content: body.Content,
			TTL:     body.TTL,
			Proxied: body.Proxied,
		})
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "create record: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(record))
	}
}

// cloudflareUpdateRecord updates a DNS record in Cloudflare.
// PUT /dns/cloudflare/zones/:zoneId/records/:recordId
func cloudflareUpdateRecord(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		zoneID := c.Param("zoneId")
		recordID := c.Param("recordId")
		if zoneID == "" || recordID == "" {
			response.Fail(c, http.StatusBadRequest, "zoneId and recordId required")
			return
		}
		var body struct {
			Type    string `json:"type"`
			Name    string `json:"name"`
			Content string `json:"content"`
			TTL     int    `json:"ttl"`
			Proxied bool   `json:"proxied"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid body")
			return
		}
		if body.Content == "" {
			response.Fail(c, http.StatusBadRequest, "content required")
			return
		}

		ctx := c.Request.Context()
		cfToken := deps.SysConf.GetString(ctx, "cloudflare.api.token")
		if cfToken == "" {
			response.Fail(c, http.StatusBadRequest, "Cloudflare not configured (set cloudflare.api.token)")
			return
		}
		cache := dns.GetOrCreateCache(cfToken)
		record, err := cache.UpdateRecord(ctx, zoneID, recordID, dns.DnsRecord{
			Type:    body.Type,
			Name:    body.Name,
			Content: body.Content,
			TTL:     body.TTL,
			Proxied: body.Proxied,
		})
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "update record: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(record))
	}
}

// cloudflareDeleteRecord deletes a DNS record from Cloudflare.
// DELETE /dns/cloudflare/zones/:zoneId/records/:recordId
func cloudflareDeleteRecord(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		zoneID := c.Param("zoneId")
		recordID := c.Param("recordId")
		if zoneID == "" || recordID == "" {
			response.Fail(c, http.StatusBadRequest, "zoneId and recordId required")
			return
		}
		ctx := c.Request.Context()
		cfToken := deps.SysConf.GetString(ctx, "cloudflare.api.token")
		if cfToken == "" {
			response.Fail(c, http.StatusBadRequest, "Cloudflare not configured (set cloudflare.api.token)")
			return
		}
		cache := dns.GetOrCreateCache(cfToken)
		if err := cache.DeleteRecord(ctx, zoneID, recordID); err != nil {
			response.Fail(c, http.StatusInternalServerError, "delete record: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("DNS record deleted"))
	}
}

// edgeoneZones returns EdgeOne zone info.
// GET /dns/edgeone/zones
func edgeoneZones(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		secretID := deps.SysConf.GetString(ctx, "edgeone.secretId")
		secretKey := deps.SysConf.GetString(ctx, "edgeone.secretKey")
		zoneID := deps.SysConf.GetString(ctx, "edgeone.zoneId")
		if secretID == "" || secretKey == "" {
			response.Fail(c, http.StatusBadRequest, "EdgeOne not configured")
			return
		}
		response.OK(c, response.SuccessData([]gin.H{
			{"id": zoneID, "name": zoneID, "status": "active"},
		}))
	}
}

// edgeoneRecords returns DNS records from EdgeOne.
// GET /dns/edgeone/records
func edgeoneRecords(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		secretID := deps.SysConf.GetString(ctx, "edgeone.secretId")
		secretKey := deps.SysConf.GetString(ctx, "edgeone.secretKey")
		zoneID := deps.SysConf.GetString(ctx, "edgeone.zoneId")
		if secretID == "" || secretKey == "" {
			response.Fail(c, http.StatusBadRequest, "EdgeOne not configured")
			return
		}
		client := dns.NewEdgeOneClient(secretID, secretKey, zoneID, deps.Logger)
		records, err := client.ListRecords()
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list edgeone records: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(records))
	}
}

// edgeoneCreateRecord creates a DNS record in EdgeOne.
// POST /dns/edgeone/records
func edgeoneCreateRecord(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Type     string `json:"type"`
			Name     string `json:"name"`
			Content  string `json:"content"`
			TTL      int    `json:"ttl"`
			Priority int    `json:"priority"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid body")
			return
		}
		if body.Type == "" || body.Name == "" || body.Content == "" {
			response.Fail(c, http.StatusBadRequest, "type, name, content required")
			return
		}

		ctx := c.Request.Context()
		secretID := deps.SysConf.GetString(ctx, "edgeone.secretId")
		secretKey := deps.SysConf.GetString(ctx, "edgeone.secretKey")
		zoneID := deps.SysConf.GetString(ctx, "edgeone.zoneId")
		if secretID == "" || secretKey == "" {
			response.Fail(c, http.StatusBadRequest, "EdgeOne not configured")
			return
		}
		client := dns.NewEdgeOneClient(secretID, secretKey, zoneID, deps.Logger)
		record, err := client.CreateRecord(body.Name, body.Type, body.Content, body.TTL, body.Priority)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "create edgeone record: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(record))
	}
}

// edgeoneUpdateRecord updates a DNS record in EdgeOne.
// PUT /dns/edgeone/records/:recordId
func edgeoneUpdateRecord(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		recordID := c.Param("recordId")
		if recordID == "" {
			response.Fail(c, http.StatusBadRequest, "recordId required")
			return
		}
		var body struct {
			Type     string `json:"type"`
			Name     string `json:"name"`
			Content  string `json:"content"`
			TTL      int    `json:"ttl"`
			Priority int    `json:"priority"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid body")
			return
		}
		if body.Content == "" {
			response.Fail(c, http.StatusBadRequest, "content required")
			return
		}

		ctx := c.Request.Context()
		secretID := deps.SysConf.GetString(ctx, "edgeone.secretId")
		secretKey := deps.SysConf.GetString(ctx, "edgeone.secretKey")
		zoneID := deps.SysConf.GetString(ctx, "edgeone.zoneId")
		if secretID == "" || secretKey == "" {
			response.Fail(c, http.StatusBadRequest, "EdgeOne not configured")
			return
		}
		client := dns.NewEdgeOneClient(secretID, secretKey, zoneID, deps.Logger)
		if err := client.UpdateRecord(recordID, body.Name, body.Type, body.Content, body.TTL, body.Priority); err != nil {
			response.Fail(c, http.StatusInternalServerError, "update edgeone record: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("record updated"))
	}
}

// edgeoneDeleteRecord deletes a DNS record from EdgeOne.
// DELETE /dns/edgeone/records/:recordId
func edgeoneDeleteRecord(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		recordID := c.Param("recordId")
		if recordID == "" {
			response.Fail(c, http.StatusBadRequest, "recordId required")
			return
		}
		ctx := c.Request.Context()
		secretID := deps.SysConf.GetString(ctx, "edgeone.secretId")
		secretKey := deps.SysConf.GetString(ctx, "edgeone.secretKey")
		zoneID := deps.SysConf.GetString(ctx, "edgeone.zoneId")
		if secretID == "" || secretKey == "" {
			response.Fail(c, http.StatusBadRequest, "EdgeOne not configured")
			return
		}
		client := dns.NewEdgeOneClient(secretID, secretKey, zoneID, deps.Logger)
		if err := client.DeleteRecord(recordID); err != nil {
			response.Fail(c, http.StatusInternalServerError, "delete edgeone record: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("record deleted"))
	}
}
