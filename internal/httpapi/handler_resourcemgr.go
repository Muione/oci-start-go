// Package httpapi -- handler_resourcemgr.go: Phase 13.3 Resource Manager management handlers.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
)

// resMgrStackList -- GET /oci/resourcemgr/stacks?tenantId=&compartmentId=&displayName=&limit=&page=
func resMgrStackList(deps *Deps) gin.HandlerFunc {
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
		data, nextPage, err := deps.ResourceMgrSvc.ListStacks(c.Request.Context(), tenantID, compartmentID, displayName, limit, page)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "success", "data": data, "nextPage": nextPage, "code": 200})
	}
}

// resMgrStackGet -- GET /oci/resourcemgr/stack/get?tenantId=&stackId=
func resMgrStackGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		stackID := c.Query("stackId")
		if tenantID == 0 || stackID == "" {
			response.Fail(c, http.StatusBadRequest, "tenantId and stackId required")
			return
		}
		data, err := deps.ResourceMgrSvc.GetStack(c.Request.Context(), tenantID, stackID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// resMgrStackDelete -- POST /oci/resourcemgr/stack/delete
func resMgrStackDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID int64  `json:"tenantId"`
			StackID  string `json:"stackId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.ResourceMgrSvc.DeleteStack(c.Request.Context(), body.TenantID, body.StackID); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// resMgrJobCreate -- POST /oci/resourcemgr/job/create
func resMgrJobCreate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID  int64  `json:"tenantId"`
			StackID   string `json:"stackId"`
			Operation string `json:"operation"` // PLAN, APPLY, DESTROY
			PlanJobID string `json:"planJobId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		switch body.Operation {
		case "PLAN":
			data, err := deps.ResourceMgrSvc.CreatePlanJob(c.Request.Context(), body.TenantID, body.StackID)
			if err != nil {
				response.Fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			response.OK(c, response.SuccessData(data))
		case "APPLY":
			var planID *string
			if body.PlanJobID != "" {
				planID = &body.PlanJobID
			}
			data, err := deps.ResourceMgrSvc.CreateApplyJob(c.Request.Context(), body.TenantID, body.StackID, planID)
			if err != nil {
				response.Fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			response.OK(c, response.SuccessData(data))
		case "DESTROY":
			data, err := deps.ResourceMgrSvc.CreateDestroyJob(c.Request.Context(), body.TenantID, body.StackID)
			if err != nil {
				response.Fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			response.OK(c, response.SuccessData(data))
		default:
			response.Fail(c, http.StatusBadRequest, "operation must be PLAN, APPLY, or DESTROY")
		}
	}
}

// resMgrJobGet -- GET /oci/resourcemgr/job/get?tenantId=&jobId=
func resMgrJobGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		jobID := c.Query("jobId")
		if tenantID == 0 || jobID == "" {
			response.Fail(c, http.StatusBadRequest, "tenantId and jobId required")
			return
		}
		data, err := deps.ResourceMgrSvc.GetJob(c.Request.Context(), tenantID, jobID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// resMgrJobList -- GET /oci/resourcemgr/jobs?tenantId=&compartmentId=&stackId=&limit=&page=
func resMgrJobList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		compartmentID := c.Query("compartmentId")
		stackID := c.Query("stackId")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		page := c.Query("page")
		if tenantID == 0 || compartmentID == "" {
			response.Fail(c, http.StatusBadRequest, "tenantId and compartmentId required")
			return
		}
		data, nextPage, err := deps.ResourceMgrSvc.ListJobs(c.Request.Context(), tenantID, compartmentID, stackID, limit, page)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "success", "data": data, "nextPage": nextPage, "code": 200})
	}
}

// resMgrJobLogs -- GET /oci/resourcemgr/job/logs?tenantId=&jobId=&limit=&page=
func resMgrJobLogs(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		jobID := c.Query("jobId")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
		page := c.Query("page")
		if tenantID == 0 || jobID == "" {
			response.Fail(c, http.StatusBadRequest, "tenantId and jobId required")
			return
		}
		data, nextPage, err := deps.ResourceMgrSvc.GetJobLogs(c.Request.Context(), tenantID, jobID, limit, page)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "success", "data": data, "nextPage": nextPage, "code": 200})
	}
}

// resMgrJobCancel -- POST /oci/resourcemgr/job/cancel
func resMgrJobCancel(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID int64  `json:"tenantId"`
			JobID    string `json:"jobId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.ResourceMgrSvc.CancelJob(c.Request.Context(), body.TenantID, body.JobID); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}
