package httpapi

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
	logpkg "github.com/Muione/oci-start-go/internal/util/log"
)

// Recovery mirrors GlobalExceptionHandler: recovers panics and returns a 500
// ApiResponse JSON (instead of gin's plain-text 500). SPA gets JSON everywhere.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				l := logpkg.FromContext(c.Request.Context())
				l.Error().
					Interface("panic", r).
					Bytes("stack", debug.Stack()).
					Msg("recovered panic")
				c.AbortWithStatusJSON(http.StatusInternalServerError, response.ApiResponse{
					Success: false,
					Message: "服务器内部错误",
					Code:    500,
				})
			}
		}()
		c.Next()
	}
}
