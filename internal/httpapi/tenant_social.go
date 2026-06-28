// Package httpapi — tenant_social.go: Phase 9 social login configuration
// handlers (per-tenant OAuth settings for Google, GitHub, etc.).
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/service"
)

// tenantSocialList — GET /tenants/:id/social
func tenantSocialList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		list, err := deps.TenantSocial.List(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "查询社媒配置失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(list))
	}
}

// tenantSocialSave — POST /tenants/:id/social
func tenantSocialSave(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		var in service.SocialSaveInput
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Fail(c, http.StatusBadRequest, "请求数据无效: "+err.Error())
			return
		}
		in.TenantID = id
		if err := deps.TenantSocial.Save(c.Request.Context(), in); err != nil {
			response.Fail(c, http.StatusInternalServerError, "保存社媒配置失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// tenantSocialToggle — PUT /tenants/:id/social/:socialId/toggle
func tenantSocialToggle(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		socialID, err := strconv.ParseInt(c.Param("socialId"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 socialId 无效")
			return
		}
		var body struct {
			Status string `json:"socialStatus"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "请求数据无效: "+err.Error())
			return
		}
		if err := deps.TenantSocial.SetStatus(c.Request.Context(), socialID, body.Status); err != nil {
			response.Fail(c, http.StatusInternalServerError, "切换社媒状态失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// tenantSocialDelete — DELETE /tenants/:id/social/:socialId
func tenantSocialDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		socialID, err := strconv.ParseInt(c.Param("socialId"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 socialId 无效")
			return
		}
		if err := deps.TenantSocial.Delete(c.Request.Context(), socialID); err != nil {
			response.Fail(c, http.StatusInternalServerError, "删除社媒配置失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}
