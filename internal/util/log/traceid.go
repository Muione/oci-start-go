package log

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TraceIDMiddleware mirrors Java TraceIdFilter: read X-Trace-Id (or generate a
// dashless UUID), inject into the request context, and echo back the response
// header. traceId propagates across goroutines via context (replaces Java MDC).
func TraceIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := c.GetHeader("X-Trace-Id")
		if tid == "" {
			tid = strings.ReplaceAll(uuid.NewString(), "-", "")
		}
		c.Request = c.Request.WithContext(WithTraceID(c.Request.Context(), tid))
		c.Header("X-Trace-Id", tid)
		c.Next()
	}
}
