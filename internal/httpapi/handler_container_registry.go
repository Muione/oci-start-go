// Package httpapi -- handler_container_registry.go: Phase 14.2 Container Registry management handlers.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
)

// ctrRegListRepos -- GET /oci/container/repositories?tenantId=&compartmentId=
func ctrRegListRepos(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		compartmentID := c.Query("compartmentId")
		if tenantID == 0 || compartmentID == "" {
			response.Fail(c, http.StatusBadRequest, "tenantId and compartmentId required")
			return
		}
		data, err := deps.CtrRegSvc.ListRepositories(c.Request.Context(), tenantID, compartmentID)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// ctrRegListImages -- GET /oci/container/images?tenantId=&compartmentId=&repositoryName=
func ctrRegListImages(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, _ := strconv.ParseInt(c.Query("tenantId"), 10, 64)
		compartmentID := c.Query("compartmentId")
		repositoryName := c.Query("repositoryName")
		if tenantID == 0 || compartmentID == "" || repositoryName == "" {
			response.Fail(c, http.StatusBadRequest, "tenantId, compartmentId, repositoryName required")
			return
		}
		data, err := deps.CtrRegSvc.ListImages(c.Request.Context(), tenantID, compartmentID, repositoryName)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}

// ctrRegDeleteImage -- POST /oci/container/image/delete
func ctrRegDeleteImage(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID int64  `json:"tenantId"`
			ImageID  string `json:"imageId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.CtrRegSvc.DeleteImage(c.Request.Context(), body.TenantID, body.ImageID); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// ctrRegDeleteRepo -- POST /oci/container/repository/delete
func ctrRegDeleteRepo(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID    int64  `json:"tenantId"`
			RepositoryID string `json:"repositoryId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		if err := deps.CtrRegSvc.DeleteRepository(c.Request.Context(), body.TenantID, body.RepositoryID); err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// ctrRegCleanup -- POST /oci/container/cleanup
func ctrRegCleanup(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			TenantID      int64  `json:"tenantId"`
			CompartmentID string `json:"compartmentId"`
			RepositoryName string `json:"repositoryName"`
			KeepCount     int    `json:"keepCount"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
			return
		}
		data, err := deps.CtrRegSvc.CleanupOldImages(c.Request.Context(), body.TenantID, body.CompartmentID, body.RepositoryName, body.KeepCount)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(data))
	}
}
