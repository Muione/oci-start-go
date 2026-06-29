// Package httpapi -- handler_mysql.go: Phase 13.3 MySQL Database Service management handlers.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
)

// mysqlDbSystemList -- GET /oci/mysql/db-systems?tenantId=&compartmentId=&displayName=&limit=&page=
func mysqlDbSystemList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		compartmentID := c.Query("compartmentId")
		displayName := c.Query("displayName")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		page := c.Query("page")
		if tenantID == 0 || compartmentID == "" {
			response.Fail(c, http.StatusBadRequest, "tenantId and compartmentId required")
			return
		}
		data, nextPage, err := deps.MySQLSvc.ListDbSystems(c.Request.Context(), tenantID, compartmentID, displayName, limit, page)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "success", "data": data, "nextPage": nextPage, "code": 200})
	}
}

// mysqlDbSystemGet -- GET /oci/mysql/db-system/get?tenantId=&dbSystemId=
func mysqlDbSystemGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		dbSystemID := c.Query("dbSystemId")
		if tenantID == 0 || dbSystemID == "" {
			response.Fail(c, http.StatusBadRequest, "tenantId and dbSystemId required")
			return
		}
		data, err := deps.MySQLSvc.GetDbSystem(c.Request.Context(), tenantID, dbSystemID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// mysqlDbSystemCreate -- POST /oci/mysql/db-system/create
func mysqlDbSystemCreate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID          int64  `json:"tenantId"`
			CompartmentID     string `json:"compartmentId"`
			DisplayName       string `json:"displayName"`
			ShapeName         string `json:"shapeName"`
			AdminUsername     string `json:"adminUsername"`
			AdminPassword     string `json:"adminPassword"`
			SubnetID          string `json:"subnetId"`
			AvailabilityDomain string `json:"availabilityDomain"`
			MysqlVersion      string `json:"mysqlVersion"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		data, err := deps.MySQLSvc.CreateDbSystem(c.Request.Context(), body.TenantID, body.CompartmentID, body.DisplayName, body.ShapeName, body.AdminUsername, body.AdminPassword, body.SubnetID, body.AvailabilityDomain, body.MysqlVersion)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// mysqlDbSystemDelete -- POST /oci/mysql/db-system/delete
func mysqlDbSystemDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID   int64  `json:"tenantId"`
			DbSystemID string `json:"dbSystemId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.MySQLSvc.DeleteDbSystem(c.Request.Context(), body.TenantID, body.DbSystemID); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// mysqlDbSystemStart -- POST /oci/mysql/db-system/start
func mysqlDbSystemStart(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID   int64  `json:"tenantId"`
			DbSystemID string `json:"dbSystemId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.MySQLSvc.Start(c.Request.Context(), body.TenantID, body.DbSystemID); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// mysqlDbSystemStop -- POST /oci/mysql/db-system/stop
func mysqlDbSystemStop(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID   int64  `json:"tenantId"`
			DbSystemID string `json:"dbSystemId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.MySQLSvc.Stop(c.Request.Context(), body.TenantID, body.DbSystemID); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// mysqlDbSystemRestart -- POST /oci/mysql/db-system/restart
func mysqlDbSystemRestart(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID   int64  `json:"tenantId"`
			DbSystemID string `json:"dbSystemId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.MySQLSvc.Restart(c.Request.Context(), body.TenantID, body.DbSystemID); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// mysqlBackupList -- GET /oci/mysql/backups?tenantId=&compartmentId=&dbSystemId=&limit=&page=
func mysqlBackupList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		compartmentID := c.Query("compartmentId")
		dbSystemID := c.Query("dbSystemId")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		page := c.Query("page")
		if tenantID == 0 || compartmentID == "" {
			response.Fail(c, http.StatusBadRequest, "tenantId and compartmentId required")
			return
		}
		data, nextPage, err := deps.MySQLSvc.ListBackups(c.Request.Context(), tenantID, compartmentID, dbSystemID, limit, page)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "success", "data": data, "nextPage": nextPage, "code": 200})
	}
}

// mysqlBackupCreate -- POST /oci/mysql/backup/create
func mysqlBackupCreate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID      int64  `json:"tenantId"`
			DbSystemID    string `json:"dbSystemId"`
			DisplayName   string `json:"displayName"`
			CompartmentID string `json:"compartmentId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		data, err := deps.MySQLSvc.CreateBackup(c.Request.Context(), body.TenantID, body.DbSystemID, body.DisplayName, body.CompartmentID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// mysqlBackupDelete -- POST /oci/mysql/backup/delete
func mysqlBackupDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID int64  `json:"tenantId"`
			BackupID string `json:"backupId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.MySQLSvc.DeleteBackup(c.Request.Context(), body.TenantID, body.BackupID); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// mysqlChannelList -- GET /oci/mysql/channels?tenantId=&compartmentId=&dbSystemId=&limit=&page=
func mysqlChannelList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		compartmentID := c.Query("compartmentId")
		dbSystemID := c.Query("dbSystemId")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		page := c.Query("page")
		if tenantID == 0 || compartmentID == "" {
			response.Fail(c, http.StatusBadRequest, "tenantId and compartmentId required")
			return
		}
		data, nextPage, err := deps.MySQLSvc.ListChannels(c.Request.Context(), tenantID, compartmentID, dbSystemID, limit, page)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "success", "data": data, "nextPage": nextPage, "code": 200})
	}
}

// mysqlChannelDelete -- POST /oci/mysql/channel/delete
func mysqlChannelDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID  int64  `json:"tenantId"`
			ChannelID string `json:"channelId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.MySQLSvc.DeleteChannel(c.Request.Context(), body.TenantID, body.ChannelID); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}
