package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/response"
)

func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// versionHandler returns the seeded app_version row (parity with
// VersionCheckTask.getVersion). Protected in Phase 2.
func versionHandler(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		ver, err := repo.New(store.Read).GetAppVersion(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, response.SuccessData(gin.H{
			"current_version": ver.CurrentVersion,
			"latest_version":  ver.LatestVersion,
			"deploy_type":     ver.DeployType,
			"create_time":     ver.CreateTime,
			"update_time":     ver.UpdateTime,
		}))
	}
}
