// Package httpapi — instance_detail.go: instance detail management + backup
// + traffic alert API handlers (Phase 5). Protected routes.
package httpapi

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/service"
	"github.com/Muione/oci-start-go/internal/util/crypto"
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
// GET /instances/traffic?tenantId=&startDate=&endDate=
// startDate and endDate are optional (format: 2006-01-02).
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
		startDate := c.Query("startDate")
		endDate := c.Query("endDate")
		stats := deps.TrafficSvc.QueryTenantTraffic(c.Request.Context(), tenant, startDate, endDate)
		// Frontend expects a flat array, not the wrapped TenantTrafficStats
		if stats.Instances == nil {
			response.OK(c, response.SuccessData([]any{}))
			return
		}
		response.OK(c, response.SuccessData(stats.Instances))
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
		// Q1: shared instance→tenant→OCI-clients resolution.
		clients, inst, err := ociClientsForInstance(c, deps, id)
		if err != nil {
			respondOciClientsErr(c, err)
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

// instanceStart starts a stopped OCI instance.
// POST /instances/:id/start
func instanceStart(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id")
			return
		}
		clients, inst, err := ociClientsForInstance(c, deps, id)
		if err != nil {
			respondOciClientsErr(c, err)
			return
		}
		if err := oci.StartInstance(c.Request.Context(), clients, inst.InstanceID); err != nil {
			response.Fail(c, http.StatusInternalServerError, "start instance: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("instance start request sent"))
	}
}

// instanceStop stops a running OCI instance.
// POST /instances/:id/stop
func instanceStop(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id")
			return
		}
		clients, inst, err := ociClientsForInstance(c, deps, id)
		if err != nil {
			respondOciClientsErr(c, err)
			return
		}
		if err := oci.StopInstance(c.Request.Context(), clients, inst.InstanceID); err != nil {
			response.Fail(c, http.StatusInternalServerError, "stop instance: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("instance stop request sent"))
	}
}

// instanceTerminate terminates an OCI instance and deletes the local record.
// POST /instances/:id/terminate
func instanceTerminate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id")
			return
		}
		var body struct {
			PreserveBootVolume bool `json:"preserveBootVolume"`
		}
		_ = c.ShouldBindJSON(&body) // optional

		clients, inst, err := ociClientsForInstance(c, deps, id)
		if err != nil {
			respondOciClientsErr(c, err)
			return
		}
		// Terminate via OCI API (preserveBootVolume=false by default)
		if err := ociTerminateInstance(c.Request.Context(), clients, inst.InstanceID, body.PreserveBootVolume); err != nil {
			response.Fail(c, http.StatusInternalServerError, "terminate instance: "+err.Error())
			return
		}
		// Delete local record
		if err := repo.New(deps.Store.Write).DeleteInstanceDetail(c.Request.Context(), id); err != nil {
			respondLocalSyncFailed(c, deps, "delete instance detail", err)
			return
		}
		response.OK(c, response.SuccessMsg("instance termination request sent"))
	}
}

// instanceDeleteRecord deletes only the local instance detail record (no cloud operation).
// DELETE /instances/:id
func instanceDeleteRecord(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id")
			return
		}
		if err := repo.New(deps.Store.Write).DeleteInstanceDetail(c.Request.Context(), id); err != nil {
			response.Fail(c, http.StatusInternalServerError, "delete instance record: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("instance record deleted"))
	}
}

// instanceChangeIP reassigns the public IP of an instance.
// POST /instances/:id/change-ip
func instanceChangeIP(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id")
			return
		}
		clients, inst, err := ociClientsForInstance(c, deps, id)
		if err != nil {
			respondOciClientsErr(c, err)
			return
		}
		oldIP := inst.PublicIps
		newIP, err := ociReassignPublicIP(c.Request.Context(), clients, inst.CompartmentID, inst.InstanceID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "change ip: "+err.Error())
			return
		}
		// Update local DB with new IP
		if err := repo.New(deps.Store.Write).UpdateInstanceDetailPublicIp(c.Request.Context(), repo.UpdateInstanceDetailPublicIpParams{
			PublicIps: sql.NullString{String: newIP, Valid: true},
			ID:        id,
		}); err != nil {
			respondLocalSyncFailed(c, deps, "update instance public ip", err)
			return
		}
		response.OK(c, response.SuccessData(gin.H{
			"oldIp": oldIP,
			"newIp": newIP,
		}))
	}
}

// instanceEnableIPv6 enables IPv6 on an instance's VNIC.
// POST /instances/:id/enable-ipv6
func instanceEnableIPv6(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id")
			return
		}
		var body struct {
			ForceNew bool `json:"forceNew"`
		}
		_ = c.ShouldBindJSON(&body)

		clients, inst, err := ociClientsForInstance(c, deps, id)
		if err != nil {
			respondOciClientsErr(c, err)
			return
		}
		// Get the primary VNIC
		vnic, err := ociGetPrimaryVnic(c.Request.Context(), clients, inst.InstanceID, inst.CompartmentID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "get vnic: "+err.Error())
			return
		}
		if vnic.Id == nil {
			response.Fail(c, http.StatusInternalServerError, "vnic has no id")
			return
		}
		ipv6, err := ociAssignIpv6ToVnic(c.Request.Context(), clients.Vcn, *vnic.Id, body.ForceNew)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "enable ipv6: "+err.Error())
			return
		}
		// Update local DB with new IPv6
		if err := repo.New(deps.Store.Write).UpdateInstanceDetailIpv6(c.Request.Context(), repo.UpdateInstanceDetailIpv6Params{
			Ipv6Addresses: sql.NullString{String: ipv6, Valid: true},
			ID:            id,
		}); err != nil {
			respondLocalSyncFailed(c, deps, "update instance ipv6", err)
			return
		}
		response.OK(c, response.SuccessData(gin.H{
			"ipv6Address": ipv6,
		}))
	}
}

// instanceExport exports all instances as a plaintext file (includes root passwords).
// GET /instances/export?tenantId=
func instanceExport(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantIDStr := c.Query("tenantId")

		// Get all tenants
		tenants, err := repo.New(deps.Store.Read).ListTenants(ctx)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list tenants: "+err.Error())
			return
		}
		tenantMap := make(map[int64]repo.ListTenantsRow)
		for _, t := range tenants {
			tenantMap[t.ID] = t
		}

		// Get all instances or filter by tenant
		var instances []repo.InstanceDetail
		if tenantIDStr != "" {
			tid, err := strconv.ParseInt(tenantIDStr, 10, 64)
			if err != nil {
				response.Fail(c, http.StatusBadRequest, "invalid tenantId")
				return
			}
			rows, err := repo.New(deps.Store.Read).FindInstancesByTenantId(ctx, sql.NullInt64{Int64: tid, Valid: true})
			if err != nil {
				response.Fail(c, http.StatusInternalServerError, "list instances: "+err.Error())
				return
			}
			for _, r := range rows {
				instances = append(instances, repo.InstanceDetail{
					ID:                  r.ID,
					TenantID:            r.TenantID,
					InstanceID:          r.InstanceID,
					DisplayName:         r.DisplayName,
					Shape:               r.Shape,
					State:               r.State,
					Ocpus:               r.Ocpus,
					MemoryInGbs:         r.MemoryInGbs,
					BootVolumeSizeInGbs: r.BootVolumeSizeInGbs,
					PublicIps:           r.PublicIps,
					PrivateIps:          r.PrivateIps,
					AvailabilityDomain:  r.AvailabilityDomain,
					Ipv6Addresses:       r.Ipv6Addresses,
					Username:            r.Username,
					Port:                r.Port,
					Password:            r.Password,
					Architecture:        r.Architecture,
					VpusPerGb:           r.VpusPerGb,
					CreateTime:          r.CreateTime,
				})
			}
		} else {
			// List all - use ListAllInstanceDetails with large limit
			rows, err := repo.New(deps.Store.Read).ListAllInstanceDetails(ctx, repo.ListAllInstanceDetailsParams{
				Limit:  99999,
				Offset: 0,
			})
			if err != nil {
				response.Fail(c, http.StatusInternalServerError, "list instances: "+err.Error())
				return
			}
			for _, r := range rows {
				instances = append(instances, repo.InstanceDetail{
					ID:                  r.ID,
					TenantID:            r.TenantID,
					InstanceID:          r.InstanceID,
					DisplayName:         r.DisplayName,
					Shape:               r.Shape,
					State:               r.State,
					Ocpus:               r.Ocpus,
					MemoryInGbs:         r.MemoryInGbs,
					BootVolumeSizeInGbs: r.BootVolumeSizeInGbs,
					PublicIps:           r.PublicIps,
					PrivateIps:          r.PrivateIps,
					AvailabilityDomain:  r.AvailabilityDomain,
					Ipv6Addresses:       r.Ipv6Addresses,
					Username:            r.Username,
					Port:                r.Port,
					Password:            r.Password,
					Architecture:        r.Architecture,
					VpusPerGb:           r.VpusPerGb,
					CreateTime:          r.CreateTime,
				})
			}
		}

		// Build plaintext export
		var sb strings.Builder
		sb.WriteString("# OCI Instance Export\n")
		sb.WriteString(fmt.Sprintf("# Export Time: %s\n", time.Now().Format("2006-01-02 15:04:05")))
		sb.WriteString(fmt.Sprintf("# Total Instances: %d\n\n", len(instances)))

		// Group by tenant
		grouped := make(map[int64][]repo.InstanceDetail)
		for _, inst := range instances {
			tid := ni(inst.TenantID)
			grouped[tid] = append(grouped[tid], inst)
		}

		tIdx := 1
		for tid, insts := range grouped {
			t := tenantMap[tid]
			sb.WriteString("================================================================\n")
			sb.WriteString(fmt.Sprintf("Tenant #%d: %s", tIdx, ns(t.TenancyName)))
			if ns(t.UserName) != "" {
				sb.WriteString(fmt.Sprintf(" / User: %s", ns(t.UserName)))
			}
			if ns(t.Region) != "" {
				sb.WriteString(fmt.Sprintf(" / Region: %s", ns(t.Region)))
			}
			sb.WriteString(fmt.Sprintf(" (Total: %d)\n", len(insts)))
			sb.WriteString("================================================================\n")
			tIdx++

			iIdx := 1
			for _, inst := range insts {
				sb.WriteString(fmt.Sprintf("\n[%d] %s\n", iIdx, dash(ns(inst.DisplayName))))
				if ns(inst.Remark) != "" {
					sb.WriteString(fmt.Sprintf("  Remark:     %s\n", ns(inst.Remark)))
				}
				sb.WriteString(fmt.Sprintf("  State:      %s\n", dash(ns(inst.State))))
				sb.WriteString(fmt.Sprintf("  Arch:       %s\n", dash(ns(inst.Architecture))))
				sb.WriteString(fmt.Sprintf("  CPU/MEM:    %dC/%dG\n", ni(inst.Ocpus), ni(inst.MemoryInGbs)))
				sb.WriteString(fmt.Sprintf("  Disk/VPU:   %dGB / %s\n", ni(inst.BootVolumeSizeInGbs), dash(ns(inst.VpusPerGb))))
				sb.WriteString(fmt.Sprintf("  IPv4:       %s\n", dash(ns(inst.PublicIps))))
				sb.WriteString(fmt.Sprintf("  Private:    %s\n", dash(ns(inst.PrivateIps))))
				if ns(inst.Ipv6Addresses) != "" {
					sb.WriteString(fmt.Sprintf("  IPv6:       %s\n", ns(inst.Ipv6Addresses)))
				}
				sb.WriteString(fmt.Sprintf("  AD:         %s\n", dash(ns(inst.AvailabilityDomain))))
				usr := ns(inst.Username)
				if usr == "" {
					usr = "root"
				}
				sb.WriteString(fmt.Sprintf("  SSH User:   %s\n", usr))
				port := ni(inst.Port)
				if port == 0 {
					port = 22
				}
				sb.WriteString(fmt.Sprintf("  SSH Port:   %d\n", port))
				sb.WriteString(fmt.Sprintf("  Root Pass:  %s\n", redactPassword(inst.Password)))
				if ns(inst.CreateTime) != "" {
					sb.WriteString(fmt.Sprintf("  Created:    %s\n", ns(inst.CreateTime)))
				}
				iIdx++
			}
			sb.WriteString("\n")
		}

		filename := fmt.Sprintf("oci-instances-%s.txt", time.Now().Format("20060102-150405"))
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		c.String(http.StatusOK, sb.String())
	}
}

// instanceSaveSSHConfig saves SSH connection configuration for an instance.
// POST /instances/:id/ssh-config  {username: "...", port: 22, password: "..."}
func instanceSaveSSHConfig(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id")
			return
		}
		var body struct {
			Username string `json:"username"`
			Port     int64  `json:"port"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid body")
			return
		}
		if body.Username == "" {
			response.Fail(c, http.StatusBadRequest, "username required")
			return
		}
		if body.Port < 1 || body.Port > 65535 {
			response.Fail(c, http.StatusBadRequest, "port must be 1-65535")
			return
		}
		if err := repo.New(deps.Store.Write).UpdateInstanceSSHConfig(c.Request.Context(), repo.UpdateInstanceSSHConfigParams{
			Username: sql.NullString{String: body.Username, Valid: true},
			Port:     sql.NullInt64{Int64: body.Port, Valid: true},
			Password: encryptPasswordField(body.Password, deps.MasterKey),
			ID:       id,
		}); err != nil {
			response.Fail(c, http.StatusInternalServerError, "save ssh config: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("SSH config saved"))
	}
}

// instanceGetSSHConfig returns SSH connection configuration for an instance.
// GET /instances/:id/ssh-config
func instanceGetSSHConfig(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id")
			return
		}
		inst, err := deps.InstanceSvc.GetByID(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusNotFound, "instance not found")
			return
		}
		// Need full detail for SSH fields - query directly
		detail, err := repo.New(deps.Store.Read).FindInstanceDetailByID(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusNotFound, "instance not found")
			return
		}
		username := ns(detail.Username)
		if username == "" {
			username = "root"
		}
		port := ni(detail.Port)
		if port == 0 {
			port = 22
		}
		response.OK(c, response.SuccessData(gin.H{
			"instanceId": inst.InstanceID,
			"username":   username,
			"port":       port,
			"publicIp":   inst.PublicIps,
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

// ociClientsForInstance loads an instance detail by id, resolves its tenant,
// and builds OCI clients from the tenant credentials. It deduplicates the
// GetByID→FindTenantByID→NewProvider→NewClients preamble shared by the
// instance action handlers. ponytail: package var so tests can swap it out
// (avoids real OCI client construction / network in unit tests); production
// uses the default implementation.
var ociClientsForInstance = func(c *gin.Context, deps *Deps, id int64) (oci.Clients, *service.InstanceDetailResp, error) {
	inst, err := deps.InstanceSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		return oci.Clients{}, nil, fmt.Errorf("find instance %d: %w", id, err)
	}
	t, err := repo.New(deps.Store.Read).FindTenantByID(c.Request.Context(), inst.TenantID)
	if err != nil {
		return oci.Clients{}, nil, fmt.Errorf("find tenant for instance %d: %w", id, err)
	}
	creds := tenantToCreds(t)
	prov, err := oci.NewProvider(creds, deps.MasterKey)
	if err != nil {
		return oci.Clients{}, nil, fmt.Errorf("oci provider: %w", err)
	}
	clients, err := oci.NewClients(prov)
	if err != nil {
		return oci.Clients{}, nil, fmt.Errorf("oci clients: %w", err)
	}
	return clients, inst, nil
}

// respondOciClientsErr maps an ociClientsForInstance failure to an HTTP
// response: 404 when the instance or tenant is absent, 500 otherwise.
func respondOciClientsErr(c *gin.Context, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		response.Fail(c, http.StatusNotFound, "instance or tenant not found")
		return
	}
	response.Fail(c, http.StatusInternalServerError, err.Error())
}

// OCI operation seams. ponytail: package vars defaulting to the real oci
// functions; overridable in tests so the "cloud op succeeded" path can be
// exercised without network or real credentials.
var (
	ociTerminateInstance = oci.TerminateInstance
	ociReassignPublicIP  = oci.ReassignPublicIP
	ociGetPrimaryVnic    = oci.GetPrimaryVnic
	ociAssignIpv6ToVnic  = oci.AssignIpv6ToVnic
)

// respondLocalSyncFailed logs and returns a 200 carrying a syncFailed marker:
// the cloud operation already succeeded, so a 500 would misrepresent state,
// but the operator must know the local DB is now stale.
func respondLocalSyncFailed(c *gin.Context, deps *Deps, op string, err error) {
	deps.Logger.Error().Err(err).Str("op", op).Msg("instance local sync failed")
	response.OK(c, response.SuccessData(gin.H{
		"message":    "cloud operation succeeded but local sync failed: " + err.Error(),
		"syncFailed": true,
	}))
}

// dash returns s if non-blank, otherwise "-".
func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// redactPassword masks the root password in exports (S4): "-" when no password
// is stored, "[redacted]" when one is. Never emits the stored value — which may
// now be ciphertext, but plaintext must not leak regardless.
func redactPassword(p sql.NullString) string {
	if !p.Valid || p.String == "" {
		return "-"
	}
	return "[redacted]"
}

// encryptPasswordField AES-256-GCM encrypts a plaintext password for at-rest
// storage in instance_detail (S4). With no master key wired the plaintext is
// stored verbatim (bootstrap path); DecryptStringWithFallback on read returns
// it verbatim, so the round-trip stays correct. Encryption failure falls back
// to plaintext rather than failing the save — the row stays usable.
func encryptPasswordField(plain string, masterKey []byte) sql.NullString {
	if plain == "" {
		return sql.NullString{}
	}
	if len(masterKey) == 0 {
		return sql.NullString{String: plain, Valid: true}
	}
	enc, err := crypto.EncryptString(plain, masterKey)
	if err != nil {
		return sql.NullString{String: plain, Valid: true}
	}
	return sql.NullString{String: enc, Valid: true}
}
