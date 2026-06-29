package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/service"
)

// tenantAuditLog — POST /tenants/:id/audit-log
func tenantAuditLog(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Tenant not found",
			})
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
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    result,
		})
	}
}
