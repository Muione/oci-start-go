// Package httpapi — gcp_boot.go: GCP Compute Engine boot instance handlers.
// Provides REST endpoints for creating, listing, and deleting GCP instances
// using the service account JWT OAuth2 flow from cloud/gcp.
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/cloud/gcp"
	"github.com/Muione/oci-start-go/internal/response"
)

// gcpBootLaunch launches a GCP Compute Engine instance.
// POST /boot-instance/gcp/launch
func gcpBootLaunch(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.GcpSvc == nil {
			response.Fail(c, http.StatusServiceUnavailable, "GCP service not configured (set gcp.serviceAccountJson + gcp.projectId)")
			return
		}

		var in struct {
			ProjectID    string `json:"projectId"`
			Zone         string `json:"zone"`
			MachineType  string `json:"machineType"`
			SourceImage  string `json:"sourceImage"`
			DiskSizeGb   int64  `json:"diskSizeGb"`
			Preemptible  bool   `json:"preemptible"`
			Architecture string `json:"architecture"`
			CloudInit    string `json:"cloudInit"`
			TenantID     int64  `json:"tenantId"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid body")
			return
		}
		if in.Zone == "" || in.MachineType == "" || in.SourceImage == "" {
			response.Fail(c, http.StatusBadRequest, "zone, machineType, and sourceImage are required")
			return
		}

		task := gcp.BootTask{
			TenantID:      in.TenantID,
			ProjectID:     in.ProjectID,
			Zone:          in.Zone,
			MachineType:   in.MachineType,
			SourceImage:   in.SourceImage,
			DiskSizeGb:    in.DiskSizeGb,
			Preemptible:   in.Preemptible,
			Architecture:  in.Architecture,
			CloudInit:     in.CloudInit,
			InstanceCount: 1,
		}

		info, err := deps.GcpSvc.LaunchGcpInstance(c.Request.Context(), task)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "GCP launch failed: "+err.Error())
			return
		}

		response.OK(c, response.SuccessData(gin.H{
			"name":        info.Name,
			"zone":        info.Zone,
			"status":      info.Status,
			"machineType": info.MachineType,
		}))
	}
}

// gcpBootList returns all GCP boot tasks.
// GET /boot-instance/gcp/list
func gcpBootList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.GcpSvc == nil {
			response.Fail(c, http.StatusServiceUnavailable, "GCP service not configured")
			return
		}
		tasks, err := deps.GcpSvc.ListBootTasks(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list GCP tasks: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(tasks))
	}
}

// gcpBootDelete deletes a GCP boot task.
// GET /boot-instance/gcp/delete?id=...
func gcpBootDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.GcpSvc == nil {
			response.Fail(c, http.StatusServiceUnavailable, "GCP service not configured")
			return
		}
		id := c.Query("id")
		if id == "" {
			response.Fail(c, http.StatusBadRequest, "id is required")
			return
		}
		if err := deps.GcpSvc.DeleteBootTask(c.Request.Context(), id); err != nil {
			response.Fail(c, http.StatusInternalServerError, "delete GCP task: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(gin.H{"deleted": true}))
	}
}

// gcpBootStatus returns whether GCP is configured.
// GET /boot-instance/gcp/status
func gcpBootStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		configured := deps.GcpSvc != nil && deps.GcpSvc.IsConfigured()
		response.OK(c, response.SuccessData(gin.H{"configured": configured}))
	}
}
