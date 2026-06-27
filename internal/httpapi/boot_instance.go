// Package httpapi — boot_instance.go: boot task CRUD + grabber system status
// handlers (Phase 4). Protected routes under SessionAuth/UserContext.
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/service"
)

func errorStatus(err error) int {
	if err != nil {
		return http.StatusInternalServerError
	}
	return http.StatusOK
}

// bootList returns all boot tasks.
// GET /boot/list
func bootList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tasks, err := deps.Boot.List(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list boot tasks: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(tasks))
	}
}

// bootSave creates or updates a boot task.
// POST /boot/save  (JSON body)
func bootSave(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in service.BootSaveInput
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		if in.TenantID <= 0 {
			response.Fail(c, http.StatusBadRequest, "tenantId required")
			return
		}
		if in.Ocpu <= 0 || in.Memory <= 0 || in.Disk <= 0 {
			response.Fail(c, http.StatusBadRequest, "ocpu/memory/disk required")
			return
		}
		if err := deps.Boot.Save(c.Request.Context(), in); err != nil {
			response.Fail(c, http.StatusInternalServerError, "save boot task: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// bootDelete soft-deletes a boot task.
// GET /boot/delete?bootId=
func bootDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		bootID := c.Query("bootId")
		if bootID == "" {
			response.Fail(c, http.StatusBadRequest, "bootId required")
			return
		}
		if err := deps.Boot.Remove(c.Request.Context(), bootID); err != nil {
			response.Fail(c, http.StatusInternalServerError, "delete boot task: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// bootToggle enables or disables a boot task.
// GET /boot/toggle?bootId=&enable=1|0
func bootToggle(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		bootID := c.Query("bootId")
		if bootID == "" {
			response.Fail(c, http.StatusBadRequest, "bootId required")
			return
		}
		enable := c.Query("enable") != "0"
		if err := deps.Boot.Toggle(c.Request.Context(), bootID, enable); err != nil {
			response.Fail(c, http.StatusInternalServerError, "toggle boot task: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// bootSystemStatus returns grab engine pool metrics + task counts.
// GET /boot/systemStatus
func bootSystemStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Engine == nil {
			response.Fail(c, http.StatusServiceUnavailable, "grab engine not available")
			return
		}
		status := deps.Engine.SystemStatus(c.Request.Context())
		response.OK(c, response.SuccessData(status))
	}
}

// bootTenantList returns a lightweight tenant list for the boot task form dropdown.
// GET /boot/tenants
func bootTenantList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenants, err := deps.Tenant.List(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list tenants: "+err.Error())
			return
		}
		type item struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			Region   string `json:"region"`
			Tenancy  string `json:"tenancy"`
		}
		out := make([]item, 0, len(tenants))
		for _, t := range tenants {
			out = append(out, item{
				ID:      t.ID,
				Name:    t.UserName,
				Region:  t.RegionName,
				Tenancy: t.Tenancy,
			})
		}
		response.OK(c, response.SuccessData(out))
	}
}
