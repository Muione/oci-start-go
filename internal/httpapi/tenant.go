// Package httpapi — tenant.go: Phase 3 tenant CRUD + instance sync endpoints.
// POST /tenants/save is multipart (parity with Java saveApi: Tenant fields +
// keyFileStr file part); the private key PEM is encrypted at rest by the
// service (plan D1). These are protected routes (SessionAuth + UserContext +
// TenantContext). /tenants/save is in TenantContext's IGNORE_PATHS (the
// tenant is not yet persisted when the header is sent).
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/service"
)

// tenantList — GET /tenants/listAll
func tenantList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := deps.Tenant.List(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "查询租户失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(list))
	}
}

// tenantSave — POST /tenants/save (multipart/form-data)
func tenantSave(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		in := service.SaveInput{
			Tenancy:      c.PostForm("tenancy"),
			TenantID:     c.PostForm("tenantId"),
			UserName:     c.PostForm("userName"),
			Fingerprint:  c.PostForm("fingerprint"),
			Region:       c.PostForm("region"),
			AccountType:  c.PostForm("accountType"),
			IsHomeRegion: c.PostForm("isHomeRegion") == "true" || c.PostForm("isHomeRegion") == "1",
		}
		if ct, err := strconv.ParseInt(c.PostForm("cloudType"), 10, 64); err == nil {
			in.CloudType = ct
		}
		fh, err := c.FormFile("keyFileStr")
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "缺少密钥文件 keyFileStr")
			return
		}
		f, err := fh.Open()
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "读取密钥文件失败")
			return
		}
		defer f.Close()
		buf := make([]byte, fh.Size)
		if _, err := f.Read(buf); err != nil && err.Error() != "EOF" {
			response.Fail(c, http.StatusBadRequest, "读取密钥文件失败")
			return
		}
		in.KeyPEM = buf
		if err := deps.Tenant.Save(c.Request.Context(), in); err != nil {
			response.Fail(c, http.StatusInternalServerError, "保存失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// tenantDelete — GET /tenants/deleteApi?tenantId=
func tenantDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 tenantId 无效")
			return
		}
		if err := deps.Tenant.Delete(c.Request.Context(), id); err != nil {
			response.Fail(c, http.StatusInternalServerError, "删除失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// tenantSyncOci — GET /tenants/syncOci?tenantId=
func tenantSyncOci(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 tenantId 无效")
			return
		}
		if err := deps.Tenant.SyncOci(c.Request.Context(), id); err != nil {
			response.Fail(c, http.StatusInternalServerError, "同步失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// tenantInstances — GET /tenants/:id/instances
func tenantInstances(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		list, err := deps.Tenant.ListInstances(c.Request.Context(), id)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "查询实例失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(list))
	}
}
