package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
)

// Password-reset endpoints are whitelisted (public) but their implementation
// depends on Phase 7 messaging (VerifyService). Phase 2 returns 501.

func sendResetCode(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		response.Fail(c, http.StatusNotImplemented, "not implemented (Phase 7)")
	}
}

func verifyResetCode(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		response.Fail(c, http.StatusNotImplemented, "not implemented (Phase 7)")
	}
}

func resetPassword(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		response.Fail(c, http.StatusNotImplemented, "not implemented (Phase 7)")
	}
}
