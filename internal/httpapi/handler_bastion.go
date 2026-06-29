// Package httpapi -- handler_bastion.go: Phase 14.1 Bastion management handlers.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/response"
)

// bastionList -- GET /oci/bastion/list?tenantId=&compartmentId=
func bastionList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		compartmentID := c.Query("compartmentId")
		if tenantID == 0 || compartmentID == "" {
			response.Fail(c, http.StatusBadRequest, "tenantId and compartmentId required")
			return
		}
		data, err := deps.BastionSvc.ListBastions(c.Request.Context(), tenantID, compartmentID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// bastionSessionCreate -- POST /oci/bastion/session/create
func bastionSessionCreate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID int64  `json:"tenantId"`
			BastionID string `json:"bastionId"`
			SessionType string `json:"sessionType"`
			DisplayName string `json:"displayName"`
			TargetResourcePrivateIPAddress string `json:"targetResourcePrivateIpAddress"`
			TargetResourcePort int `json:"targetResourcePort"`
			TargetResourceID string `json:"targetResourceId"`
			TargetResourceOSUserName string `json:"targetResourceOperatingSystemUserName"`
			SessionTTLInSeconds int `json:"sessionTtlInSeconds"`
			PublicKeyContent string `json:"publicKeyContent"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		data, err := deps.BastionSvc.CreateSession(c.Request.Context(), body.TenantID, CreateSessionInputFromJSON(body))
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// CreateSessionInputFromJSON converts handler body to service input.
func CreateSessionInputFromJSON(body struct {
	TenantID int64  `json:"tenantId"`
	BastionID string `json:"bastionId"`
	SessionType string `json:"sessionType"`
	DisplayName string `json:"displayName"`
	TargetResourcePrivateIPAddress string `json:"targetResourcePrivateIpAddress"`
	TargetResourcePort int `json:"targetResourcePort"`
	TargetResourceID string `json:"targetResourceId"`
	TargetResourceOSUserName string `json:"targetResourceOperatingSystemUserName"`
	SessionTTLInSeconds int `json:"sessionTtlInSeconds"`
	PublicKeyContent string `json:"publicKeyContent"`
}) oci.CreateSessionInput {
	return oci.CreateSessionInput{
		BastionID: body.BastionID,
		SessionType: body.SessionType,
		DisplayName: body.DisplayName,
		TargetResourcePrivateIPAddress: body.TargetResourcePrivateIPAddress,
		TargetResourcePort: body.TargetResourcePort,
		TargetResourceID: body.TargetResourceID,
		TargetResourceOSUserName: body.TargetResourceOSUserName,
		SessionTTLInSeconds: body.SessionTTLInSeconds,
		PublicKeyContent: body.PublicKeyContent,
	}
}

// bastionSessionList -- GET /oci/bastion/session/list?tenantId=&bastionId=
func bastionSessionList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		bastionID := c.Query("bastionId")
		if tenantID == 0 || bastionID == "" {
			response.Fail(c, http.StatusBadRequest, "tenantId and bastionId required")
			return
		}
		data, err := deps.BastionSvc.ListSessions(c.Request.Context(), tenantID, bastionID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// bastionSessionGet -- GET /oci/bastion/session/get?tenantId=&sessionId=
func bastionSessionGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		sessionID := c.Query("sessionId")
		if tenantID == 0 || sessionID == "" {
			response.Fail(c, http.StatusBadRequest, "tenantId and sessionId required")
			return
		}
		data, err := deps.BastionSvc.GetSession(c.Request.Context(), tenantID, sessionID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// bastionSessionDelete -- POST /oci/bastion/session/delete
func bastionSessionDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID  int64  `json:"tenantId"`
			SessionID string `json:"sessionId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.BastionSvc.DeleteSession(c.Request.Context(), body.TenantID, body.SessionID); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}
