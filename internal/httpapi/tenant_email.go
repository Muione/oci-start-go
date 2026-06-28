// Package httpapi — tenant_email.go: Phase 9 email service configuration
// handlers (per-tenant SES/SMTP settings).
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/service"
)

// tenantEmailGet — GET /tenants/:id/email
func tenantEmailGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		cfg, err := deps.TenantEmail.Get(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "查询邮箱配置失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(cfg))
	}
}

// tenantEmailSave — POST /tenants/:id/email
func tenantEmailSave(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		var in service.EmailSaveInput
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Fail(c, http.StatusBadRequest, "请求数据无效: "+err.Error())
			return
		}
		in.TenantID = id
		if err := deps.TenantEmail.Save(c.Request.Context(), in); err != nil {
			response.Fail(c, http.StatusInternalServerError, "保存邮箱配置失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// tenantEmailToggle — POST /tenants/:id/email/toggle
// Toggles the active state of the email service.
func tenantEmailToggle(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		var body struct {
			Active bool `json:"active"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "请求数据无效: "+err.Error())
			return
		}
		if err := deps.TenantEmail.SetActive(c.Request.Context(), id, body.Active); err != nil {
			response.Fail(c, http.StatusInternalServerError, "切换邮箱状态失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// tenantEmailDelete — DELETE /tenants/:id/email
func tenantEmailDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		if err := deps.TenantEmail.Delete(c.Request.Context(), id); err != nil {
			response.Fail(c, http.StatusInternalServerError, "删除邮箱配置失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}
