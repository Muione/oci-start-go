package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/service"
)

// tenantAuditLog — POST /tenants/:id/audit-log
func tenantAuditLog(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}

		var req service.AuditLogRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			req.Days = 1 // default
		}
		if req.Days <= 0 {
			req.Days = 1
		}
		if req.Days > 90 {
			req.Days = 90
		}

		result, err := deps.Audit.Query(c.Request.Context(), id, req)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "查询审计日志失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(result))
	}
}
