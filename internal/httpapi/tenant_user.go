// Package httpapi — tenant_user.go: IAM user management, password policy,
// MFA, notification recipients, and auto-fetch tenancy detail handlers.
// API surface follows the Java reference (TenantController.java):
//   - GET    /tenants/:id/users                          → tenantUsersList
//   - POST   /tenants/:id/users                          → tenantUserCreate
//   - DELETE /tenants/:id/users/:userOcid                → tenantUserDelete
//   - POST   /tenants/:id/users/:userOcid/reset-password → tenantUserResetPassword
//   - GET    /tenants/:id/groups                         → tenantGroupsList
//   - GET    /tenants/:id/password-policy                → tenantPasswordPolicyGet
//   - POST   /tenants/:id/password-policy                → tenantPasswordPolicyUpdate
//   - GET    /tenants/:id/mfa/status                     → tenantMfaStatus
//   - POST   /tenants/:id/mfa/toggle                     → tenantMfaToggle
//   - POST   /tenants/:id/mfa/reset                      → tenantMfaReset
//   - GET    /tenants/:id/notification-recipients        → tenantNotifRecipientsGet
//   - POST   /tenants/:id/notification-recipients/update → tenantNotifRecipientsUpdate
//   - POST   /tenants/:id/update-detail                  → tenantUpdateDetail
package httpapi

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/response"
)

// --- User CRUD ---

// tenantUsersList — GET /tenants/:id/users
func tenantUsersList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		users, err := deps.TenantUser.ListUsers(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "获取用户列表失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(users))
	}
}

// tenantUserCreate — POST /tenants/:id/users
func tenantUserCreate(deps *Deps) gin.HandlerFunc {
	type createReq struct {
		Username  string `json:"username"`
		Email     string `json:"email"`
		GroupName string `json:"groupName"`
		GroupOcid string `json:"groupOcid"`
	}
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		var req createReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "请求数据无效: "+err.Error())
			return
		}
		if req.Username == "" || req.Email == "" {
			response.Fail(c, http.StatusBadRequest, "用户名和邮箱不能为空")
			return
		}
		result, err := deps.TenantUser.CreateUser(c.Request.Context(), id, oci.CreateUserRequest{
			Username:  req.Username,
			Email:     req.Email,
			GroupName: req.GroupName,
			GroupOCID: req.GroupOcid,
		})
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "创建用户失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(result))
	}
}

// tenantUserDelete — DELETE /tenants/:id/users/:userOcid
func tenantUserDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		userOCID := c.Param("userOcid")
		if userOCID == "" {
			response.Fail(c, http.StatusBadRequest, "用户 OCID 不能为空")
			return
		}
		if err := deps.TenantUser.DeleteUser(c.Request.Context(), id, userOCID); err != nil {
			response.Fail(c, http.StatusInternalServerError, "删除用户失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// tenantUserResetPassword — POST /tenants/:id/users/:userOcid/reset-password
func tenantUserResetPassword(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		userOCID := c.Param("userOcid")
		if userOCID == "" {
			response.Fail(c, http.StatusBadRequest, "用户 OCID 不能为空")
			return
		}
		pw, err := deps.TenantUser.ResetPassword(c.Request.Context(), id, userOCID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "重置密码失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(map[string]string{"password": pw}))
	}
}

// --- Groups ---

// tenantGroupsList — GET /tenants/:id/groups
func tenantGroupsList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		groups, err := deps.TenantUser.ListGroups(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "获取用户组失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(groups))
	}
}

// --- Password Policy ---

// tenantPasswordPolicyGet — GET /tenants/:id/password-policy
func tenantPasswordPolicyGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		policy, err := deps.TenantUser.GetPasswordPolicy(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "获取密码策略失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(policy))
	}
}

// tenantPasswordPolicyUpdate — POST /tenants/:id/password-policy
func tenantPasswordPolicyUpdate(deps *Deps) gin.HandlerFunc {
	type policyReq struct {
		EnableExpiry bool `json:"enableExpiry"`
		ExpiryDays   int  `json:"expiryDays"`
	}
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		var req policyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "请求数据无效: "+err.Error())
			return
		}
		if err := deps.TenantUser.UpdatePasswordPolicy(c.Request.Context(), id, req.EnableExpiry, req.ExpiryDays); err != nil {
			response.Fail(c, http.StatusInternalServerError, "更新密码策略失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// --- MFA ---

// tenantMfaStatus — GET /tenants/:id/mfa/status
func tenantMfaStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		status, err := deps.TenantUser.GetMfaStatus(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "获取MFA状态失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(status))
	}
}

// tenantMfaToggle — POST /tenants/:id/mfa/toggle
func tenantMfaToggle(deps *Deps) gin.HandlerFunc {
	type mfaReq struct {
		Enable bool `json:"enable"`
	}
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		var req mfaReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "请求数据无效: "+err.Error())
			return
		}
		state, err := deps.TenantUser.ToggleEmailMFA(c.Request.Context(), id, req.Enable)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "切换MFA失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(map[string]bool{"emailEnabled": state}))
	}
}

// tenantMfaReset — POST /tenants/:id/mfa/reset
// Java reference: POST /tenants/resetAccountFactor
func tenantMfaReset(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		count, err := deps.TenantUser.ResetMfa(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "重置MFA失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(map[string]int{"deletedDevices": count}))
	}
}

// --- Notification Recipients ---

// tenantNotifRecipientsGet — GET /tenants/:id/notification-recipients
func tenantNotifRecipientsGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		recipients, err := deps.TenantUser.GetNotificationRecipients(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "获取通知邮箱失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(recipients))
	}
}

// tenantNotifRecipientsUpdate — POST /tenants/:id/notification-recipients/update
// Java reference: POST /tenants/notification/update
func tenantNotifRecipientsUpdate(deps *Deps) gin.HandlerFunc {
	type updateReq struct {
		Emails []string `json:"emails"`
	}
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		var req updateReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "请求数据无效: "+err.Error())
			return
		}
		if len(req.Emails) == 0 {
			response.Fail(c, http.StatusBadRequest, "邮箱列表不能为空")
			return
		}
		if err := deps.TenantUser.UpdateNotificationRecipients(c.Request.Context(), id, req.Emails); err != nil {
			response.Fail(c, http.StatusInternalServerError, "更新通知邮箱失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// --- Auto-fetch Tenancy Detail ---

// tenantUpdateDetail — POST /tenants/:id/update-detail
// Fetches the tenancy metadata from OCI and updates the local record.
func tenantUpdateDetail(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		detail, err := deps.TenantUser.UpdateAccountDetail(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, fmt.Sprintf("从OCI获取租户信息失败: %s", err.Error()))
			return
		}
		response.OK(c, response.SuccessData(detail))
	}
}

// --- Subscription Days (BE-001) ---

// tenantSubscriptionDays -- GET /tenants/:id/subscription-days
func tenantSubscriptionDays(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid tenant ID")
			return
		}
		info, err := deps.TenantUser.GetSubscriptionDays(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "Failed to get subscription days: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(info))
	}
}

// --- Domain Tenants (BE-003) ---

// tenantDomainTenants -- GET /tenants/:id/domains
func tenantDomainTenants(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid tenant ID")
			return
		}
		domains, err := deps.TenantUser.ListDomainTenants(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "Failed to list domain tenants: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(domains))
	}
}
