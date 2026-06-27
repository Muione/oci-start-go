// Package httpapi — instance_detail.go: instance detail management + backup
// + traffic alert API handlers (Phase 5). Protected routes.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/response"
)

// instanceList returns paginated instance details.
// GET /instances/list?limit=20&offset=0
func instanceList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "20"), 10, 64)
		offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
		rows, total, err := deps.InstanceSvc.List(c.Request.Context(), limit, offset)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list instances: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(map[string]any{
			"items": rows,
			"total": total,
		}))
	}
}

// instanceGet returns a single instance detail.
// GET /instances/:id
func instanceGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id")
			return
		}
		inst, err := deps.InstanceSvc.GetByID(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "get instance: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(inst))
	}
}

// instanceUpdateRemark updates the remark for an instance.
// POST /instances/:id/remark  {remark: "..."}
func instanceUpdateRemark(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id")
			return
		}
		var body struct{ Remark string `json:"remark"` }
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid body")
			return
		}
		if err := deps.InstanceSvc.UpdateRemark(c.Request.Context(), id, body.Remark); err != nil {
			response.Fail(c, http.StatusInternalServerError, "update remark: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// instanceTraffic returns traffic stats for a specific tenant.
// GET /instances/traffic?tenantId=
func instanceTraffic(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		if err != nil || tenantID <= 0 {
			response.Fail(c, http.StatusBadRequest, "valid tenantId required")
			return
		}
		tenant, err := repo.New(deps.Store.Read).FindTenantByID(c.Request.Context(), tenantID)
		if err != nil {
			response.Fail(c, http.StatusNotFound, "tenant not found")
			return
		}
		stats := deps.TrafficSvc.QueryTenantTraffic(c.Request.Context(), tenant)
		response.OK(c, response.SuccessData(stats))
	}
}

// backupList returns backup records for a tenant.
// GET /backup/list?tenantId=
func backupList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		if err != nil || tenantID <= 0 {
			response.Fail(c, http.StatusBadRequest, "valid tenantId required")
			return
		}
		rows, err := deps.InstanceSvc.ListBackups(c.Request.Context(), tenantID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list backups: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(rows))
	}
}

// backupDelete deletes a backup record.
// GET /backup/delete?id=
func backupDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Query("id"), 10, 64)
		if err != nil || id <= 0 {
			response.Fail(c, http.StatusBadRequest, "valid id required")
			return
		}
		if err := deps.InstanceSvc.DeleteBackup(c.Request.Context(), id); err != nil {
			response.Fail(c, http.StatusInternalServerError, "delete backup: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// trafficAlertSave creates or updates a traffic alert config.
// POST /traffic/alert/save
func trafficAlertSave(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			TenantID           int64   `json:"tenantId"`
			Threshold          float64 `json:"threshold"`
			AutoShutdown       bool    `json:"autoShutdown"`
			Enabled            bool    `json:"enabled"`
			StatisticsEnabled  bool    `json:"statisticsEnabled"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid body")
			return
		}
		if err := deps.TrafficSvc.SaveTrafficAlert(c.Request.Context(),
			in.TenantID, in.Threshold, in.AutoShutdown, in.Enabled, in.StatisticsEnabled); err != nil {
			response.Fail(c, http.StatusInternalServerError, "save traffic alert: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// trafficAlertList returns all traffic alert configs.
// GET /traffic/alert/list
func trafficAlertList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := deps.TrafficSvc.ListTrafficAlerts(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list traffic alerts: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(rows))
	}
}

// trafficAlertGet returns a single traffic alert config.
// GET /traffic/alert/get?tenantId=
func trafficAlertGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		if err != nil || tenantID <= 0 {
			response.Fail(c, http.StatusBadRequest, "valid tenantId required")
			return
		}
		alert, err := deps.TrafficSvc.GetTrafficAlert(c.Request.Context(), tenantID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "get traffic alert: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(alert))
	}
}

// instanceModify modifies an instance's shape, OCPU, memory, or display name
// via the OCI API and updates the local DB record.
// POST /instances/:id/modify  {shape?, ocpus?, memoryInGbs?, displayName?}
func instanceModify(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id")
			return
		}
		var body struct {
			Shape       string   `json:"shape"`
			Ocpus       *float32 `json:"ocpus"`
			MemoryInGbs *float32 `json:"memoryInGbs"`
			DisplayName string  `json:"displayName"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid body")
			return
		}
		if body.Shape == "" && body.DisplayName == "" {
			response.Fail(c, http.StatusBadRequest, "shape or displayName required")
			return
		}
		// Get the instance to find its tenant
		inst, err := deps.InstanceSvc.GetByID(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "find instance: "+err.Error())
			return
		}
		// Get tenant credentials
		t, err := repo.New(deps.Store.Read).FindTenantByID(c.Request.Context(), inst.TenantID)
		if err != nil {
			response.Fail(c, http.StatusNotFound, "tenant not found")
			return
		}
		creds := tenantToCreds(t)
		prov, err := oci.NewProvider(creds, deps.MasterKey)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "oci provider: "+err.Error())
			return
		}
		clients, err := oci.NewClients(prov)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "oci clients: "+err.Error())
			return
		}
		updated, err := oci.UpdateInstanceShape(c.Request.Context(), clients, inst.InstanceID, body.Shape, body.Ocpus, body.MemoryInGbs, body.DisplayName)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "update instance: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(map[string]any{
			"id":          id,
			"state":       string(updated.LifecycleState),
			"shape":       body.Shape,
			"ocpus":       body.Ocpus,
			"memoryInGbs": body.MemoryInGbs,
				"displayName": body.DisplayName,
		}))
	}
}

// tenantToCreds converts a repo.Tenant to oci.Credentials (used by instanceModify).
func tenantToCreds(t repo.Tenant) oci.Credentials {
	return oci.Credentials{
		Tenancy:     ns(t.Tenancy),
		UserID:      ns(t.TenantID),
		Fingerprint: ns(t.Fingerprint),
		Region:      ns(t.Region),
		KeyFileBlob: ns(t.KeyFileBlob),
		KeyFile:     ns(t.KeyFile),
	}
}
