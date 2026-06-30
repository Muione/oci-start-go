// Package httpapi — shape_image.go: Shape/Image listing + boot volume VPU
// management API handlers. Protected routes.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/response"
)

// listShapes returns available compute shapes for a tenant, optionally
// filtered by architecture ("ARM" or "AMD").
// GET /oci/shapes?tenantId=1&architecture=ARM
func listShapes(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		if err != nil || tenantID <= 0 {
			response.Fail(c, http.StatusBadRequest, "valid tenantId required")
			return
		}
		architecture := c.Query("architecture")

		t, err := repo.New(deps.Store.Read).FindTenantByID(c.Request.Context(), tenantID)
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
		compartmentID := ns(t.Tenancy)
		shapes, err := oci.ListShapesFiltered(c.Request.Context(), clients, compartmentID, "", architecture)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list shapes: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(shapes))
	}
}

// listImages returns available compute images for a tenant, optionally
// filtered by architecture and shape compatibility.
// GET /oci/images?tenantId=1&architecture=ARM&shape=VM.Standard.A1.Flex
func listImages(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		if err != nil || tenantID <= 0 {
			response.Fail(c, http.StatusBadRequest, "valid tenantId required")
			return
		}
		architecture := c.Query("architecture")
		shape := c.Query("shape")

		t, err := repo.New(deps.Store.Read).FindTenantByID(c.Request.Context(), tenantID)
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
		compartmentID := ns(t.Tenancy)
		images, err := oci.ListImagesFiltered(c.Request.Context(), clients, compartmentID, shape, architecture)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "list images: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(images))
	}
}

// instanceUpdateVpu updates the VPU (performance level) of an instance's
// boot volume. The instance should be stopped for this operation.
// POST /instances/:id/vpu  {vpusPerGb: 20}
func instanceUpdateVpu(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid id")
			return
		}
		var body struct {
			VpusPerGb int64 `json:"vpusPerGb"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid body")
			return
		}
		if body.VpusPerGb < 0 || body.VpusPerGb > 120 {
			response.Fail(c, http.StatusBadRequest, "vpusPerGb must be 0-120")
			return
		}

		inst, err := deps.InstanceSvc.GetByID(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusNotFound, "instance not found")
			return
		}
		if inst.BootVolumeID == "" {
			response.Fail(c, http.StatusBadRequest, "instance has no boot volume ID")
			return
		}

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

		bv, err := oci.UpdateBootVolumeVpu(c.Request.Context(), clients, inst.BootVolumeID, body.VpusPerGb)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "update vpu: "+err.Error())
			return
		}

		// Update local DB
		_ = repo.New(deps.Store.Write).UpdateInstanceDetailVpusPerGb(c.Request.Context(), repo.UpdateInstanceDetailVpusPerGbParams{
			VpusPerGb: nullStr(strconv.FormatInt(body.VpusPerGb, 10)),
			ID:        id,
		})

		vpuVal := int64(0)
		if bv.VpusPerGB != nil {
			vpuVal = *bv.VpusPerGB
		}
		response.OK(c, response.SuccessData(map[string]any{
			"vpusPerGb": vpuVal,
		}))
	}
}

// instanceRestart stops then starts an instance (reset).
// POST /instances/:id/restart
func instanceRestart(deps *Deps) gin.HandlerFunc {
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
		if err := oci.ResetInstance(c.Request.Context(), clients, inst.InstanceID); err != nil {
			response.Fail(c, http.StatusInternalServerError, "restart instance: "+err.Error())
			return
		}
		response.OK(c, response.SuccessMsg("instance restarted"))
	}
}
