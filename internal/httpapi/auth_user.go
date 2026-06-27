package httpapi

import (
	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/auth"
	"github.com/Muione/oci-start-go/internal/response"
)

// GET /api/userInfo — protected. Returns the current session username.
func userInfo(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		username, _ := auth.UsernameFromContext(c.Request.Context())
		response.OK(c, response.SuccessData(gin.H{"username": username}))
	}
}
