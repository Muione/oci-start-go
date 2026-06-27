package auth

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/response"
	iputil "github.com/Muione/oci-start-go/internal/util/ip"
)

// IpBan mirrors @CheckIpBan (on BaseController → all routes). Resolves the
// client IP and blocks if a ban_record with status=1 exists.
func IpBan(store *db.Store) gin.HandlerFunc {
	q := repo.New(store.Read)
	return func(c *gin.Context) {
		ip := iputil.ClientIP(c)
		if ip == "" {
			response.Fail(c, http.StatusForbidden, "无法识别IP来源，拒绝访问。")
			return
		}
		ban, err := q.FindTopBanByIpAddress(c.Request.Context(), ip)
		switch {
		case err == nil && ban.Status == 1:
			response.Fail(c, http.StatusForbidden, "您的IP已被封禁，无法访问此接口。")
			return
		case err != nil && !errors.Is(err, sql.ErrNoRows):
			response.Fail(c, http.StatusInternalServerError, "ip ban check failed")
			return
		}
		c.Next()
	}
}

// SessionAuth mirrors the Sa-Token login-check interceptor. Reads the satoken
// token (cookie or Bearer), validates the session, injects the username into
// context, and (throttled) touches last_active_at. Invalid → 401.
func SessionAuth(svc *SessionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := TokenFromRequest(c)
		if token == "" {
			response.Fail(c, http.StatusUnauthorized, "未登录或登录已过期")
			return
		}
		username, lastActive, ok := svc.Validate(c.Request.Context(), token)
		if !ok {
			response.Fail(c, http.StatusUnauthorized, "未登录或登录已过期")
			return
		}
		ctx := WithUsername(c.Request.Context(), username)
		c.Request = c.Request.WithContext(ctx)
		if time.Since(lastActive) > touchInterval {
			_ = svc.Touch(c.Request.Context(), token)
		}
		c.Next()
	}
}

// UserContext mirrors @CheckLoginUser (enrich-only; does not enforce login).
// SessionAuth already injected the username, so this is a chain-parity no-op.
func UserContext() gin.HandlerFunc {
	return func(c *gin.Context) { c.Next() }
}

// TenantContext mirrors RequestContextInterceptor: read X-Tenant-Id (fallback
// ?tenantId), store the parsed id in context. Phase 2 stores id only; the
// Tenant DB load + OCI Provider construction are deferred to Phase 3.
func TenantContext(store *db.Store) gin.HandlerFunc {
	ignore := map[string]bool{"/tenants/save": true}
	q := repo.New(store.Read)
	return func(c *gin.Context) {
		if ignore[c.Request.URL.Path] {
			c.Next()
			return
		}
		idStr := c.GetHeader("X-Tenant-Id")
		if idStr == "" {
			idStr = c.Query("tenantId")
		}
		if idStr == "" {
			c.Next()
			return
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.Next()
			return
		}
		ctx := WithTenantID(c.Request.Context(), id)
		// Load the Tenant row so downstream consumers can build an OCI provider
		// without re-querying (parity with RequestContextInterceptor loading the
		// Tenant into RequestContext). Missing tenant is non-fatal (skip), parity
		// with Java preHandle returning true on lookup miss.
		if t, err := q.FindTenantByID(ctx, id); err == nil {
			ctx = WithTenant(ctx, t)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
