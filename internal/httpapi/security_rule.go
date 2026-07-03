// Package httpapi -- security_rule.go: Phase 11.3 security list rule management.
// GET/POST/DELETE /tenants/security-rules, POST /tenants/enableAll.
// Parity with Java SecurityRuleController.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/response"
)

// getSecurityRules -- GET /tenants/security-rules?tenantId={id}&type={ingress|egress}
func getSecurityRules(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 tenantId 无效")
			return
		}
		ruleType := c.Query("type")
		if ruleType != "ingress" && ruleType != "egress" {
			response.Fail(c, http.StatusBadRequest, "参数 type 必须为 ingress 或 egress")
			return
		}

		rules, err := deps.SecurityRule.GetRules(c.Request.Context(), tenantID, ruleType)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "查询安全规则失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(rules))
	}
}

// addSecurityRule -- POST /tenants/security-rules
func addSecurityRule(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var rule oci.SecurityRuleDTO
		if err := c.ShouldBindJSON(&rule); err != nil {
			response.Fail(c, http.StatusBadRequest, "请求体格式错误: "+err.Error())
			return
		}
		if rule.TenantID == nil {
			response.Fail(c, http.StatusBadRequest, "缺少 tenantId")
			return
		}
		if rule.Type != "ingress" && rule.Type != "egress" {
			response.Fail(c, http.StatusBadRequest, "type 必须为 ingress 或 egress")
			return
		}

		if err := deps.SecurityRule.AddRule(c.Request.Context(), *rule.TenantID, rule); err != nil {
			response.Fail(c, http.StatusInternalServerError, "添加安全规则失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(rule))
	}
}

// deleteSecurityRule -- DELETE /tenants/security-rules/:id
func deleteSecurityRule(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		compositeID := c.Param("id")
		if compositeID == "" {
			response.Fail(c, http.StatusBadRequest, "缺少规则 ID")
			return
		}

		if err := deps.SecurityRule.DeleteRule(c.Request.Context(), compositeID); err != nil {
			response.Fail(c, http.StatusInternalServerError, "删除安全规则失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// batchEnableAll -- POST /tenants/enableAll
func batchEnableAll(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		successCount, failCount, err := deps.SecurityRule.BatchEnableAll(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "批量启用失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsgData("success", map[string]int{
			"success": successCount,
			"fail":    failCount,
		}))
	}
}

// enableIpv6 -- POST /tenants/enableIpv6
// Enables IPv6 security rules for all tenants (batch operation).
func enableIpv6(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.SecurityRule.BatchEnableIPv6(c.Request.Context()); err != nil {
			response.Fail(c, http.StatusInternalServerError, "enable IPv6 failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("IPv6 rules enabled"))
	}
}
