// Package httpapi -- handler_ip_quality.go: Phase 13.1 IP Quality Detection
// HTTP handlers. Single/batch IP quality tests, direct IP test, auto-switch.
// All endpoints are protected (require auth).
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/service"
)

// GET /instances/:id/ip-quality
// Tests the network quality of a single instance's public IP.
func ipQualityTest(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id")
			return
		}
		result, err := deps.IpQualitySvc.TestSingleIP(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "test IP quality: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(result))
	}
}

// POST /ip-quality/test-ip
// Tests the quality of a specific IP address directly.
func ipQualityTestIP(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			IP string `json:"ip"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.IP == "" {
			response.Fail(c, http.StatusBadRequest, "ip is required")
			return
		}
		result, err := deps.IpQualitySvc.TestIPByAddress(c.Request.Context(), body.IP)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "test IP: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(result))
	}
}

// GET /ip-quality/batch/:tenantId
// Tests all instance IPs for a given tenant.
func ipQualityBatchTenant(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := strconv.ParseInt(c.Param("tenantId"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid tenantId")
			return
		}
		result, err := deps.IpQualitySvc.BatchTestByTenant(c.Request.Context(), tenantID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "batch test: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(result))
	}
}

// GET /ip-quality/batch-all
// Tests all instance IPs across all tenants.
func ipQualityBatchAll(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := deps.IpQualitySvc.BatchTestAll(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "batch test all: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(result))
	}
}

// POST /ip-quality/auto-switch
// Auto-switches an instance's IP if quality is below threshold.
func ipQualityAutoSwitch(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input service.AutoSwitchInput
		if err := c.ShouldBindJSON(&input); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid request body")
			return
		}
		if input.InstanceID <= 0 {
			response.Fail(c, http.StatusBadRequest, "instanceId is required")
			return
		}
		result, err := deps.IpQualitySvc.AutoSwitchIP(c.Request.Context(), input)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "auto-switch: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(result))
	}
}
