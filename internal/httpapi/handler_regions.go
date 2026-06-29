package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
)

// tenantRegionSummary — GET /tenants/:id/regions/summary
func tenantRegionSummary(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		result, err := deps.RegionSub.Summary(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// tenantRegionsSubscribed — GET /tenants/:id/regions/subscribed
func tenantRegionsSubscribed(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		result, err := deps.RegionSub.Subscribed(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// tenantRegionsUnsubscribed — GET /tenants/:id/regions/unsubscribed
func tenantRegionsUnsubscribed(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		result, err := deps.RegionSub.Unsubscribed(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// tenantRegionsSubscribe — POST /tenants/:id/regions/subscribe
func tenantRegionsSubscribe(deps *Deps) gin.HandlerFunc {
	type subscribeReq struct {
		RegionKeys []string `json:"regionKeys"`
	}
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		var req subscribeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "请求参数无效")
			return
		}
		if len(req.RegionKeys) == 0 {
			response.Fail(c, http.StatusBadRequest, "regionKeys 不能为空")
			return
		}
		result, err := deps.RegionSub.Subscribe(c.Request.Context(), id, req.RegionKeys)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// tenantRegionSubStatus — GET /tenants/:id/regions/subscription-status?regionKey=ap-tokyo-1
func tenantRegionSubStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		regionKey := c.Query("regionKey")
		if regionKey == "" {
			response.Fail(c, http.StatusBadRequest, "regionKey 不能为空")
			return
		}
		result, err := deps.RegionSub.SubscriptionStatus(c.Request.Context(), id, regionKey)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
