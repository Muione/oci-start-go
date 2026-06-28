// Package httpapi — tenant_ext.go: Phase 9 tenant extended operations
// (update, detail, check connectivity, export).
package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/service"
)

// tenantGet — GET /tenants/:id
func tenantGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		detail, err := deps.Tenant.GetFull(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "查询租户详情失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(detail))
	}
}

// tenantUpdate — PUT /tenants/:id
func tenantUpdate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		var in service.UpdateInput
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Fail(c, http.StatusBadRequest, "请求数据无效: "+err.Error())
			return
		}
		in.ID = id
		if err := deps.Tenant.Update(c.Request.Context(), in); err != nil {
			response.Fail(c, http.StatusInternalServerError, "更新租户失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// tenantCheck — GET /tenants/:id/check
func tenantCheck(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		result := deps.Tenant.Check(c.Request.Context(), id)
		response.OK(c, response.SuccessData(result))
	}
}

// tenantExport — GET /tenants/:id/export
func tenantExport(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		data, err := deps.Tenant.Export(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "导出失败: "+err.Error())
			return
		}
		bytes, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "序列化失败: "+err.Error())
			return
		}
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", "attachment; filename=tenant_"+strconv.FormatInt(id, 10)+".json")
		c.Data(http.StatusOK, "application/json", bytes)
	}
}

// tenantCheckBatch — POST /tenants/check-batch
// Checks connectivity for multiple tenant IDs at once.
func tenantCheckBatch(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var ids []int64
		if err := c.ShouldBindJSON(&ids); err != nil {
			response.Fail(c, http.StatusBadRequest, "请求数据无效: "+err.Error())
			return
		}
		results := make([]service.CheckResult, 0, len(ids))
		for _, id := range ids {
			results = append(results, deps.Tenant.Check(c.Request.Context(), id))
		}
		response.OK(c, response.SuccessData(results))
	}
}
