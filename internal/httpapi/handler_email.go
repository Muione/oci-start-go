// Package httpapi -- handler_email.go: Phase 12.2 Email Delivery management
// handlers. Recipients, sending, bodies, send records, tenant configs, and
// OCI email provisioning (enable/disable).
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/service"
)

// --- Email Recipient Handlers ---

// emailReceiveList -- POST /api/email/receive/list
func emailReceiveList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Email    string `json:"email"`
			Name     string `json:"name"`
			Page     int64  `json:"page"`
			PageSize int64  `json:"pageSize"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if body.Page < 0 {
			body.Page = 0
		}
		if body.PageSize <= 0 {
			body.PageSize = 20
		}
		data, total, err := deps.EmailSvc.ListReceives(c.Request.Context(), body.Email, body.Name, body.Page, body.PageSize)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "Query failed: "+err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "success",
			"data":    data,
			"total":   total,
			"code":    200,
		})
	}
}

// emailReceiveAdd -- POST /api/email/receive/add
func emailReceiveAdd(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.EmailSvc.AddReceive(c.Request.Context(), body.Email, body.Name); err != nil {
			response.Fail(c, http.StatusInternalServerError, "Add failed: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// emailReceiveDelete -- POST /api/email/receive/delete
func emailReceiveDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			ID int64 `json:"id"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.EmailSvc.DeleteReceive(c.Request.Context(), body.ID); err != nil {
			response.Fail(c, http.StatusInternalServerError, "Delete failed: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// emailReceiveGet -- POST /api/email/receive/get
func emailReceiveGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			ID int64 `json:"id"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		data, err := deps.EmailSvc.GetReceive(c.Request.Context(), body.ID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "Query failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// --- Email Sending ---

// emailSend -- POST /api/email/send
func emailSend(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body service.SendEmailInput
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		result, err := deps.EmailSvc.Send(c.Request.Context(), body)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "Send failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(result))
	}
}

// --- Email Body Handlers ---

// emailBodyList -- POST /api/email/body/list
func emailBodyList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			EmailBodyID string `json:"emailBodyId"`
			Page        int64  `json:"page"`
			PageSize    int64  `json:"pageSize"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if body.Page < 0 {
			body.Page = 0
		}
		if body.PageSize <= 0 {
			body.PageSize = 20
		}
		data, total, err := deps.EmailSvc.ListBodies(c.Request.Context(), body.EmailBodyID, body.Page, body.PageSize)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "Query failed: "+err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "success",
			"data":    data,
			"total":   total,
			"code":    200,
		})
	}
}

// emailBodyDelete -- POST /api/email/body/delete
func emailBodyDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			EmailBodyID string `json:"emailBodyId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.EmailSvc.DeleteBody(c.Request.Context(), body.EmailBodyID); err != nil {
			response.Fail(c, http.StatusInternalServerError, "Delete failed: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// emailBodyBatchDelete -- POST /api/email/body/batchDelete
func emailBodyBatchDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantEmailConfigID int64 `json:"tenantEmailConfigId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.EmailSvc.BatchDeleteBodies(c.Request.Context(), body.TenantEmailConfigID); err != nil {
			response.Fail(c, http.StatusInternalServerError, "Batch delete failed: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// --- Email Send Record Handlers ---

// emailSendRecordList -- POST /api/email/send/list
func emailSendRecordList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			EmailBodyID string `json:"emailBodyId"`
			Page        int64  `json:"page"`
			PageSize    int64  `json:"pageSize"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if body.Page < 0 {
			body.Page = 0
		}
		if body.PageSize <= 0 {
			body.PageSize = 20
		}
		data, total, err := deps.EmailSvc.ListSendRecords(c.Request.Context(), body.EmailBodyID, body.Page, body.PageSize)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "Query failed: "+err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "success",
			"data":    data,
			"total":   total,
			"code":    200,
		})
	}
}

// --- Tenant Email Config Handlers ---

// tenantEmailConfigList -- POST /api/email/tenant/list
func tenantEmailConfigList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Page     int64 `json:"page"`
			PageSize int64 `json:"pageSize"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if body.Page < 0 {
			body.Page = 0
		}
		if body.PageSize <= 0 {
			body.PageSize = 20
		}
		data, total, err := deps.EmailSvc.ListTenantConfigs(c.Request.Context(), body.Page, body.PageSize)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "Query failed: "+err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "success",
			"data":    data,
			"total":   total,
			"code":    200,
		})
	}
}

// tenantEmailConfigGet -- POST /api/email/tenant/get
func tenantEmailConfigGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID int64 `json:"tenantId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		data, err := deps.EmailSvc.GetTenantConfig(c.Request.Context(), body.TenantID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "Query failed: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// --- Email Enable/Disable ---

// emailEnable -- POST /api/email/enable
func emailEnable(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body service.EnableEmailInput
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.EmailSvc.EnableEmail(c.Request.Context(), body); err != nil {
			response.Fail(c, http.StatusInternalServerError, "Enable failed: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// emailDisable -- POST /api/email/disable
func emailDisable(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body service.DisableEmailInput
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.EmailSvc.DisableEmail(c.Request.Context(), body); err != nil {
			response.Fail(c, http.StatusInternalServerError, "Disable failed: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// parsePageParams is a helper for query parameter parsing.
func parsePageParams(c *gin.Context) (page, pageSize int64) {
	page, _ = strconv.ParseInt(c.DefaultQuery("page", "0"), 10, 64)
	pageSize, _ = strconv.ParseInt(c.DefaultQuery("pageSize", "20"), 10, 64)
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return
}
