// Package httpapi — console_connection.go: list + delete OCI instance console
// connections for the VNC console feature. The handler resolves the instance's
// tenant + compartment from the DB, delegates to ConsoleConnectionSvc (which
// talks to OCI + the persisted-key store), and maps errors. Resume + create
// go through the /ws/console control WebSocket, not these REST endpoints.
package httpapi

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/response"
)

// instanceConsoleConnectionsList lists an instance's OCI console connections
// (with IsOurs/CanResume annotations) so the frontend can offer resume/delete.
// :id is the instance OCID.
func instanceConsoleConnectionsList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		instanceID := c.Param("id")
		row, err := repo.New(deps.Store.Read).FindConsoleInstanceInfo(c.Request.Context(), sql.NullString{String: instanceID, Valid: true})
		if err != nil {
			respondOciClientsErr(c, err) // 404 when instance absent, 500 otherwise
			return
		}
		if deps.ConsoleConnSvc == nil {
			response.Fail(c, http.StatusInternalServerError, "console connection service not configured")
			return
		}
		views, err := deps.ConsoleConnSvc.List(c.Request.Context(), row.TenantID.Int64, row.CompartmentID.String, instanceID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(views))
	}
}

// instanceConsoleConnectionDelete deletes one OCI console connection (and the
// app's persisted row if it was ours). :id is the instance OCID, :connId the
// console connection OCID.
func instanceConsoleConnectionDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		instanceID := c.Param("id")
		connID := c.Param("connId")
		row, err := repo.New(deps.Store.Read).FindConsoleInstanceInfo(c.Request.Context(), sql.NullString{String: instanceID, Valid: true})
		if err != nil {
			respondOciClientsErr(c, err)
			return
		}
		if deps.ConsoleConnSvc == nil {
			response.Fail(c, http.StatusInternalServerError, "console connection service not configured")
			return
		}
		if err := deps.ConsoleConnSvc.Delete(c.Request.Context(), row.TenantID.Int64, row.CompartmentID.String, instanceID, connID); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}
