// Package httpapi — nginx.go: Phase 12.1 HTTP handlers for Nginx / Reverse Proxy
// management. All endpoints are in the protected route group (SessionAuth +
// UserContext + TenantContext). Follows the same handler-factory pattern as tenant.go.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/service"
)

// ---------------------------------------------------------------------------
// Proxy Config Handlers
// ---------------------------------------------------------------------------

// nginxCreateProxy — POST /ssl/proxy/create
func nginxCreateProxy(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in service.ProxyConfigInput
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := deps.NginxSvc.CreateProxyConfig(c.Request.Context(), in); err != nil {
			if err.Error() == "domain already exists: "+in.Domain {
				response.Fail(c, http.StatusBadRequest, err.Error())
				return
			}
			response.Fail(c, http.StatusInternalServerError, "create proxy config failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("Proxy config created"))
	}
}

// nginxUpdateProxy — PUT /ssl/proxy/:id
func nginxUpdateProxy(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id parameter")
			return
		}
		var in service.ProxyConfigInput
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := deps.NginxSvc.UpdateProxyConfig(c.Request.Context(), id, in); err != nil {
			response.Fail(c, http.StatusInternalServerError, "update proxy config failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("Proxy config updated"))
	}
}

// nginxGetProxy — GET /ssl/proxy/:id
func nginxGetProxy(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id parameter")
			return
		}
		pc, err := deps.NginxSvc.GetProxyConfig(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "get proxy config failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(pc))
	}
}

// nginxDeleteProxy — DELETE /ssl/proxy/:id
func nginxDeleteProxy(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id parameter")
			return
		}
		if err := deps.NginxSvc.DeleteProxyConfig(c.Request.Context(), id); err != nil {
			response.Fail(c, http.StatusInternalServerError, "delete proxy config failed: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// nginxListProxies — GET /ssl/proxy/list?page=0&size=20
func nginxListProxies(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.ParseInt(c.DefaultQuery("page", "0"), 10, 64)
		size, _ := strconv.ParseInt(c.DefaultQuery("size", "20"), 10, 64)
		list, total, err := deps.NginxSvc.ListProxyConfigs(c.Request.Context(), page, size)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list proxy configs failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(map[string]any{
			"list":  list,
			"total": total,
		}))
	}
}

// nginxBatchDeleteProxies — DELETE /ssl/proxy/batch
func nginxBatchDeleteProxies(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var ids []int64
		if err := c.ShouldBindJSON(&ids); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := deps.NginxSvc.BatchDeleteProxyConfigs(c.Request.Context(), ids); err != nil {
			response.Fail(c, http.StatusInternalServerError, "batch delete failed: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// nginxToggleProxy — PUT /ssl/proxy/:id/toggle
func nginxToggleProxy(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id parameter")
			return
		}
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := deps.NginxSvc.ToggleProxyConfig(c.Request.Context(), id, body.Enabled); err != nil {
			response.Fail(c, http.StatusInternalServerError, "toggle failed: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// nginxTestProxyConnection — POST /ssl/proxy/:id/test-connection
func nginxTestProxyConnection(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id parameter")
			return
		}
		connected, err := deps.NginxSvc.TestProxyConnection(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "test connection failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(map[string]bool{"connected": connected}))
	}
}

// nginxApplySsl — POST /ssl/proxy/:id/ssl?email=user@example.com
func nginxApplySsl(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id parameter")
			return
		}
		email := c.Query("email")
		if email == "" {
			response.Fail(c, http.StatusBadRequest, "email query parameter is required")
			return
		}
		if err := deps.NginxSvc.ApplySslToProxy(c.Request.Context(), id, email); err != nil {
			response.Fail(c, http.StatusInternalServerError, "apply SSL failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("SSL certificate request initiated"))
	}
}

// nginxFixProxy — POST /ssl/proxy/:id/fix
func nginxFixProxy(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id parameter")
			return
		}
		if err := deps.NginxSvc.FixProxyConfig(c.Request.Context(), id); err != nil {
			response.Fail(c, http.StatusInternalServerError, "fix proxy config failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("Proxy config reset to PENDING"))
	}
}

// ---------------------------------------------------------------------------
// SSL Certificate Handlers
// ---------------------------------------------------------------------------

// nginxRequestCert — POST /ssl/certificates/request
func nginxRequestCert(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in service.CertificateRequestInput
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		cert, err := deps.NginxSvc.RequestCertificate(c.Request.Context(), in)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "request certificate failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(cert))
	}
}

// nginxRenewCert — POST /ssl/certificates/:id/renew
func nginxRenewCert(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id parameter")
			return
		}
		if err := deps.NginxSvc.RenewCertificate(c.Request.Context(), id); err != nil {
			response.Fail(c, http.StatusInternalServerError, "renew certificate failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("Certificate renewal initiated"))
	}
}

// nginxDeleteCert — DELETE /ssl/certificates/:id
func nginxDeleteCert(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id parameter")
			return
		}
		if err := deps.NginxSvc.DeleteCertificate(c.Request.Context(), id); err != nil {
			response.Fail(c, http.StatusInternalServerError, "delete certificate failed: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// nginxToggleAutoRenew — PUT /ssl/certificates/:id/auto-renew
func nginxToggleAutoRenew(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id parameter")
			return
		}
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := deps.NginxSvc.ToggleAutoRenew(c.Request.Context(), id, body.Enabled); err != nil {
			response.Fail(c, http.StatusInternalServerError, "toggle auto-renew failed: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// nginxListCerts — GET /ssl/certificates/list?page=0&size=20
func nginxListCerts(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.ParseInt(c.DefaultQuery("page", "0"), 10, 64)
		size, _ := strconv.ParseInt(c.DefaultQuery("size", "20"), 10, 64)
		list, total, err := deps.NginxSvc.ListCertificates(c.Request.Context(), page, size)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list certificates failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(map[string]any{
			"list":  list,
			"total": total,
		}))
	}
}

// nginxExpiringCerts — GET /ssl/certificates/expiring
func nginxExpiringCerts(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := deps.NginxSvc.CheckExpiringCertificates(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "check expiring certificates failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(list))
	}
}

// nginxDownloadCert — GET /ssl/certificates/:id/download
func nginxDownloadCert(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id parameter")
			return
		}
		data, filename, err := deps.NginxSvc.DownloadCertificate(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "download certificate failed: "+err.Error())
			return
		}
		c.Header("Content-Disposition", "attachment; filename="+filename)
		c.Data(http.StatusOK, "application/zip", data)
	}
}

// nginxMatchCerts — GET /ssl/certificates/match?domain=api.example.com
func nginxMatchCerts(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Query("domain")
		if domain == "" {
			response.Fail(c, http.StatusBadRequest, "domain query parameter is required")
			return
		}
		matches, err := deps.NginxSvc.MatchCertificatesByDomain(c.Request.Context(), domain)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "match certificates failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(matches))
	}
}

// ---------------------------------------------------------------------------
// Nginx Config Handlers
// ---------------------------------------------------------------------------

// nginxGenerateConfig — POST /ssl/nginx/generate
func nginxGenerateConfig(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := deps.NginxSvc.GenerateNginxConfig(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "generate config failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(map[string]any{
			"id":            cfg.ID,
			"configVersion": cfg.ConfigVersion,
		}))
	}
}

// nginxApplyConfig — POST /ssl/nginx/:id/apply
func nginxApplyConfig(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id parameter")
			return
		}
		if err := deps.NginxSvc.ApplyNginxConfig(c.Request.Context(), id); err != nil {
			response.Fail(c, http.StatusInternalServerError, "apply config failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("Config applied successfully"))
	}
}

// nginxTestConfig — POST /ssl/nginx/:id/test
func nginxTestConfig(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id parameter")
			return
		}
		valid, err := deps.NginxSvc.TestNginxConfig(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "test config failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(map[string]bool{"valid": valid}))
	}
}

// nginxReload — POST /ssl/nginx/reload
func nginxReload(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.NginxSvc.ReloadNginx(c.Request.Context()); err != nil {
			response.Fail(c, http.StatusInternalServerError, "reload failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("Nginx reloaded"))
	}
}

// nginxConfigDiff — GET /ssl/nginx/diff
func nginxConfigDiff(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := deps.NginxSvc.GetConfigDiff(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "get config diff failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// nginxStatus — GET /ssl/nginx/status
func nginxStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := deps.NginxSvc.GetNginxStatus(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "get nginx status failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// nginxLatestConfig — GET /ssl/nginx/latest
func nginxLatestConfig(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := deps.NginxSvc.GetLatestNginxConfig(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "get latest config failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(cfg))
	}
}

// ---------------------------------------------------------------------------
// OpenResty Service Handlers
// ---------------------------------------------------------------------------

// openrestyStatus — GET /ssl/openresty/status
func openrestyStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := deps.NginxSvc.CheckOpenRestyStatus(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "check openresty status failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// openrestyStart — POST /ssl/openresty/start
func openrestyStart(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.NginxSvc.StartOpenResty(c.Request.Context()); err != nil {
			response.Fail(c, http.StatusInternalServerError, "start openresty failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("OpenResty started"))
	}
}
