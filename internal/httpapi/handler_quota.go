package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
)

// tenantQuota — GET /tenants/:id/quota?serviceName=compute&page=0&pageSize=20
func tenantQuota(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}

		serviceName := c.DefaultQuery("serviceName", "compute")
		page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
		if page < 0 {
			page = 0
		}
		if pageSize <= 0 {
			pageSize = 20
		}

		result, err := deps.Quota.GetQuota(c.Request.Context(), id, serviceName, page, pageSize)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
