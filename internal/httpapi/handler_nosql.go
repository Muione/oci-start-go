// Package httpapi -- handler_nosql.go: Phase 13.3 NoSQL Database management handlers.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
)

// nosqlTableList -- GET /oci/nosql/tables?tenantId=&compartmentId=&limit=&page=
func nosqlTableList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		compartmentID := c.Query("compartmentId")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		page := c.Query("page")
		if tenantID == 0 || compartmentID == "" {
			response.Fail(c, http.StatusBadRequest, "tenantId and compartmentId required")
			return
		}
		data, nextPage, err := deps.NoSQLSvc.ListTables(c.Request.Context(), tenantID, compartmentID, limit, page)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "success", "data": data, "nextPage": nextPage, "code": 200})
	}
}

// nosqlTableGet -- GET /oci/nosql/table/get?tenantId=&tableNameOrId=&compartmentId=
func nosqlTableGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		tableName := c.Query("tableNameOrId")
		compartmentID := c.Query("compartmentId")
		if tenantID == 0 || tableName == "" || compartmentID == "" {
			response.Fail(c, http.StatusBadRequest, "tenantId, tableNameOrId, compartmentId required")
			return
		}
		data, err := deps.NoSQLSvc.GetTable(c.Request.Context(), tenantID, tableName, compartmentID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// nosqlTableCreate -- POST /oci/nosql/table/create
func nosqlTableCreate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID      int64  `json:"tenantId"`
			CompartmentID string `json:"compartmentId"`
			TableName     string `json:"tableName"`
			DdlStatement  string `json:"ddlStatement"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		data, err := deps.NoSQLSvc.CreateTable(c.Request.Context(), body.TenantID, body.CompartmentID, body.TableName, body.DdlStatement, nil)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// nosqlTableDelete -- POST /oci/nosql/table/delete
func nosqlTableDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID      int64  `json:"tenantId"`
			TableNameOrId string `json:"tableNameOrId"`
			CompartmentID string `json:"compartmentId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.NoSQLSvc.DeleteTable(c.Request.Context(), body.TenantID, body.TableNameOrId, body.CompartmentID); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// nosqlRowGet -- POST /oci/nosql/row/get
func nosqlRowGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID      int64    `json:"tenantId"`
			TableNameOrId string   `json:"tableNameOrId"`
			CompartmentID string   `json:"compartmentId"`
			Key           []string `json:"key"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		data, err := deps.NoSQLSvc.GetRow(c.Request.Context(), body.TenantID, body.TableNameOrId, body.CompartmentID, body.Key)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// nosqlRowPut -- POST /oci/nosql/row/put
func nosqlRowPut(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID      int64                  `json:"tenantId"`
			TableNameOrId string                 `json:"tableNameOrId"`
			CompartmentID string                 `json:"compartmentId"`
			Value         map[string]interface{} `json:"value"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.NoSQLSvc.UpdateRow(c.Request.Context(), body.TenantID, body.TableNameOrId, body.CompartmentID, body.Value); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// nosqlRowDelete -- POST /oci/nosql/row/delete
func nosqlRowDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID      int64    `json:"tenantId"`
			TableNameOrId string   `json:"tableNameOrId"`
			CompartmentID string   `json:"compartmentId"`
			Key           []string `json:"key"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.NoSQLSvc.DeleteRow(c.Request.Context(), body.TenantID, body.TableNameOrId, body.CompartmentID, body.Key); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// nosqlQuery -- POST /oci/nosql/query
func nosqlQuery(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID      int64  `json:"tenantId"`
			CompartmentID string `json:"compartmentId"`
			Statement     string `json:"statement"`
			Limit         int    `json:"limit"`
			Page          string `json:"page"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if body.Limit <= 0 {
			body.Limit = 50
		}
		data, nextPage, err := deps.NoSQLSvc.QueryRows(c.Request.Context(), body.TenantID, body.CompartmentID, body.Statement, body.Limit, body.Page)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "success", "data": data, "nextPage": nextPage, "code": 200})
	}
}
