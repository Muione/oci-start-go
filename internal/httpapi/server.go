// Package httpapi wires the Gin engine, middleware, and routes.
// See SPEC §6 (route groups) — Phase 2 adds auth + base JSON API.
package httpapi

import (
	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/auth"
	logpkg "github.com/Muione/oci-start-go/internal/util/log"
	"github.com/Muione/oci-start-go/internal/web"
)

// NewServer builds the Gin engine with recovery + traceId + IpBan (all routes),
// public routes (whitelist), protected routes (SessionAuth/UserContext/TenantContext),
// and the SPA NoRoute fallback.
func NewServer(deps *Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(Recovery(), logpkg.TraceIDMiddleware())
	r.Use(auth.IpBan(deps.Store)) // @CheckIpBan parity: applies to all routes

	// public routes (Sa-Token exclude list)
	pub := r.Group("/")
	pub.GET("/healthz", healthz)
	pub.GET("/api/login/init", loginInit(deps))
	pub.POST("/api/login", login(deps))
	pub.POST("/api/logout", logout(deps))
	pub.POST("/api/register-first-user", registerFirstUser(deps))
	pub.GET("/api/disTurnstile", disTurnstile(deps))
	pub.GET("/api/config/mfa-enabled", configMfaEnabled(deps))
	pub.GET("/api/config/turnstile", configTurnstile(deps))
	pub.GET("/api/config/message-enabled", configMessageEnabled(deps))
	pub.GET("/api/github/login/url", githubLoginURL(deps))
	pub.GET("/api/github/callback", githubCallback(deps))
	pub.GET("/api/github/status", githubStatus(deps))
	pub.GET("/api/google/login/url", googleLoginURL(deps))
	pub.POST("/api/send-reset-code", sendResetCode(deps))
	pub.POST("/api/verify-reset-code", verifyResetCode(deps))
	pub.POST("/api/reset-password", resetPassword(deps))

	// protected routes
	pro := r.Group("/")
	pro.Use(auth.SessionAuth(deps.Session), auth.UserContext(), auth.TenantContext(deps.Store))
	pro.GET("/api/version", versionHandler(deps.Store))
	pro.GET("/api/userInfo", userInfo(deps))
	pro.GET("/tenants/listAll", tenantList(deps))
	pro.POST("/tenants/save", tenantSave(deps))
	pro.GET("/tenants/deleteApi", tenantDelete(deps))
	pro.GET("/tenants/syncOci", tenantSyncOci(deps))
	pro.GET("/tenants/:id/instances", tenantInstances(deps))
	pro.GET("/proxies/list", proxyList(deps))
	pro.POST("/proxies/save", proxySave(deps))
	pro.GET("/proxies/delete", proxyDelete(deps))
	pro.GET("/api/stats", dashboardStats(deps))

	// SPA static assets + NoRoute fallback to index.html
	web.Register(r)
	return r
}
