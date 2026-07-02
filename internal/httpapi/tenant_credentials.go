// Package httpapi — tenant_credentials.go: per-user IAM credential and
// domain-level sign-on/recovery handlers. Follows tenant_user.go's pattern.
// Routes are registered in server.go under the protected group:
//
//	GET    /tenants/:id/users/:userOcid/api-keys
//	POST   /tenants/:id/users/:userOcid/api-keys
//	DELETE /tenants/:id/users/:userOcid/api-keys/:keyId
//	GET    /tenants/:id/users/:userOcid/auth-tokens
//	POST   /tenants/:id/users/:userOcid/auth-tokens
//	DELETE /tenants/:id/users/:userOcid/auth-tokens/:tokenId
//	GET    /tenants/:id/users/:userOcid/smtp-credentials
//	POST   /tenants/:id/users/:userOcid/smtp-credentials
//	DELETE /tenants/:id/users/:userOcid/smtp-credentials/:credId
//	GET    /tenants/:id/users/:userOcid/customer-secret-keys
//	POST   /tenants/:id/users/:userOcid/customer-secret-keys
//	DELETE /tenants/:id/users/:userOcid/customer-secret-keys/:keyId
//	GET    /tenants/:id/signon-policies
//	GET    /tenants/:id/account-recovery
//	PUT    /tenants/:id/account-recovery
//
// Create endpoints surface the one-time secret (token/password/secret key) in
// the response body. Route param :userOcid captures the full user OCID (dots are
// fine — params match until the next "/").
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
)

// parseTenantID extracts and validates the :id route param. On failure it
// writes the error response and returns ok=false.
func parseTenantID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "Invalid tenant ID")
		return 0, false
	}
	return id, true
}

// --- API Keys ---

func tenantListApiKeys(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseTenantID(c)
		if !ok {
			return
		}
		userOCID := c.Param("userOcid")
		keys, err := deps.TenantCredentials.ListApiKeys(c.Request.Context(), id, userOCID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "List API keys failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(keys))
	}
}

func tenantCreateApiKey(deps *Deps) gin.HandlerFunc {
	type req struct {
		Key string `json:"key"`
	}
	return func(c *gin.Context) {
		id, ok := parseTenantID(c)
		if !ok {
			return
		}
		var r req
		if err := c.ShouldBindJSON(&r); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
			return
		}
		if r.Key == "" {
			response.Fail(c, http.StatusBadRequest, "Public key PEM is required")
			return
		}
		userOCID := c.Param("userOcid")
		res, err := deps.TenantCredentials.CreateApiKey(c.Request.Context(), id, userOCID, r.Key)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "Create API key failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(res))
	}
}

func tenantDeleteApiKey(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseTenantID(c)
		if !ok {
			return
		}
		if err := deps.TenantCredentials.DeleteApiKey(c.Request.Context(), id, c.Param("keyId")); err != nil {
			response.Fail(c, http.StatusInternalServerError, "Delete API key failed: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// --- Auth Tokens ---

func tenantListAuthTokens(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseTenantID(c)
		if !ok {
			return
		}
		userOCID := c.Param("userOcid")
		tokens, err := deps.TenantCredentials.ListAuthTokens(c.Request.Context(), id, userOCID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "List auth tokens failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(tokens))
	}
}

func tenantCreateAuthToken(deps *Deps) gin.HandlerFunc {
	type req struct {
		Description string `json:"description"`
	}
	return func(c *gin.Context) {
		id, ok := parseTenantID(c)
		if !ok {
			return
		}
		var r req
		if err := c.ShouldBindJSON(&r); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
			return
		}
		if r.Description == "" {
			response.Fail(c, http.StatusBadRequest, "Description is required")
			return
		}
		userOCID := c.Param("userOcid")
		res, err := deps.TenantCredentials.CreateAuthToken(c.Request.Context(), id, userOCID, r.Description)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "Create auth token failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(res))
	}
}

func tenantDeleteAuthToken(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseTenantID(c)
		if !ok {
			return
		}
		if err := deps.TenantCredentials.DeleteAuthToken(c.Request.Context(), id, c.Param("tokenId")); err != nil {
			response.Fail(c, http.StatusInternalServerError, "Delete auth token failed: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// --- SMTP Credentials ---

func tenantListSmtpCredentials(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseTenantID(c)
		if !ok {
			return
		}
		userOCID := c.Param("userOcid")
		creds, err := deps.TenantCredentials.ListSmtpCredentials(c.Request.Context(), id, userOCID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "List SMTP credentials failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(creds))
	}
}

func tenantCreateSmtpCredential(deps *Deps) gin.HandlerFunc {
	type req struct {
		Description string `json:"description"`
	}
	return func(c *gin.Context) {
		id, ok := parseTenantID(c)
		if !ok {
			return
		}
		var r req
		if err := c.ShouldBindJSON(&r); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
			return
		}
		if r.Description == "" {
			response.Fail(c, http.StatusBadRequest, "Description is required")
			return
		}
		userOCID := c.Param("userOcid")
		res, err := deps.TenantCredentials.CreateSmtpCredential(c.Request.Context(), id, userOCID, r.Description)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "Create SMTP credential failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(res))
	}
}

func tenantDeleteSmtpCredential(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseTenantID(c)
		if !ok {
			return
		}
		if err := deps.TenantCredentials.DeleteSmtpCredential(c.Request.Context(), id, c.Param("credId")); err != nil {
			response.Fail(c, http.StatusInternalServerError, "Delete SMTP credential failed: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// --- Customer Secret Keys ---

func tenantListCustomerSecretKeys(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseTenantID(c)
		if !ok {
			return
		}
		userOCID := c.Param("userOcid")
		keys, err := deps.TenantCredentials.ListCustomerSecretKeys(c.Request.Context(), id, userOCID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "List customer secret keys failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(keys))
	}
}

func tenantCreateCustomerSecretKey(deps *Deps) gin.HandlerFunc {
	type req struct {
		DisplayName string `json:"displayName"`
	}
	return func(c *gin.Context) {
		id, ok := parseTenantID(c)
		if !ok {
			return
		}
		var r req
		if err := c.ShouldBindJSON(&r); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
			return
		}
		if r.DisplayName == "" {
			response.Fail(c, http.StatusBadRequest, "Display name is required")
			return
		}
		userOCID := c.Param("userOcid")
		res, err := deps.TenantCredentials.CreateCustomerSecretKey(c.Request.Context(), id, userOCID, r.DisplayName)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "Create customer secret key failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(res))
	}
}

func tenantDeleteCustomerSecretKey(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseTenantID(c)
		if !ok {
			return
		}
		if err := deps.TenantCredentials.DeleteCustomerSecretKey(c.Request.Context(), id, c.Param("keyId")); err != nil {
			response.Fail(c, http.StatusInternalServerError, "Delete customer secret key failed: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// --- Sign-on Policies & Account Recovery ---

func tenantListSignOnPolicies(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseTenantID(c)
		if !ok {
			return
		}
		policies, err := deps.TenantCredentials.ListSignOnPolicies(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "List sign-on policies failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(policies))
	}
}

func tenantGetAccountRecovery(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseTenantID(c)
		if !ok {
			return
		}
		info, err := deps.TenantCredentials.GetAccountRecoverySetting(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "Get account recovery settings failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(info))
	}
}

func tenantUpdateAccountRecovery(deps *Deps) gin.HandlerFunc {
	type req struct {
		Factors []string `json:"factors"`
	}
	return func(c *gin.Context) {
		id, ok := parseTenantID(c)
		if !ok {
			return
		}
		var r req
		if err := c.ShouldBindJSON(&r); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
			return
		}
		info, err := deps.TenantCredentials.UpdateAccountRecoverySetting(c.Request.Context(), id, r.Factors)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "Update account recovery settings failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(info))
	}
}
