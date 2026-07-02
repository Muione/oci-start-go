// Package httpapi — proxy.go: VpnProxyRecord CRUD handlers (Phase 4 UI).
// Protected routes (SessionAuth + UserContext). The proxy pool is used by the
// grab engine (oci.WithProxy); these endpoints allow the admin to manage the
// proxy list in the SPA.
package httpapi

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/response"
)

// proxyList — GET /proxies/list
func proxyList(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := repo.New(deps.Store.Read).ListVpnProxyRecords(c.Request.Context())
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "查询代理列表失败: "+err.Error())
			return
		}
		// Map to plain DTOs (no sql.Null wrappers).
		type proxyResp struct {
			ID              int64  `json:"id"`
			ProxyType       string `json:"proxyType"`
			ProxyHost       string `json:"proxyHost"`
			ProxyPort       int64  `json:"proxyPort"`
			ProxyUsername   string `json:"proxyUsername"`
			ProxyPassword   string `json:"proxyPassword"`
			AvailableStatus int64  `json:"availableStatus"`
		}
		out := make([]proxyResp, 0, len(list))
		for _, r := range list {
			out = append(out, proxyResp{
				ID:              r.ID,
				ProxyType:       r.ProxyType,
				ProxyHost:       r.ProxyHost,
				ProxyPort:       r.ProxyPort,
				ProxyUsername:   ns(r.ProxyUsername),
				ProxyPassword:   ns(r.ProxyPassword),
				AvailableStatus: r.AvailableStatus,
			})
		}
		response.OK(c, response.SuccessData(out))
	}
}

// proxySave — POST /proxies/save
func proxySave(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.PostForm("proxyHost")
		portStr := c.PostForm("proxyPort")
		ptype := c.PostForm("proxyType")
		if host == "" || portStr == "" {
			response.Fail(c, http.StatusBadRequest, "代理地址和端口必填")
			return
		}
		port, err := strconv.ParseInt(portStr, 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "端口格式错误")
			return
		}
		if ptype == "" {
			ptype = "SOCKS5"
		}
		now := time.Now().Format("2006-01-02 15:04:05")
		params := repo.InsertVpnProxyRecordParams{
			ProxyType:       ptype,
			ProxyHost:       host,
			ProxyPort:       port,
			ProxyUsername:   nullStr(c.PostForm("proxyUsername")),
			ProxyPassword:   nullStr(c.PostForm("proxyPassword")),
			AvailableStatus: 1,
			UpdateTime:      sql.NullString{String: now, Valid: true},
			CreateTime:      sql.NullString{String: now, Valid: true},
		}
		// Existing record? Update instead.
		if idStr := c.PostForm("id"); idStr != "" {
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				response.Fail(c, http.StatusBadRequest, "参数 id 无效")
				return
			}
			if err := repo.New(deps.Store.Write).UpdateVpnProxyRecord(c.Request.Context(), repo.UpdateVpnProxyRecordParams{
				ID:              id,
				ProxyType:       params.ProxyType,
				ProxyHost:       params.ProxyHost,
				ProxyPort:       params.ProxyPort,
				ProxyUsername:   params.ProxyUsername,
				ProxyPassword:   params.ProxyPassword,
				AvailableStatus: params.AvailableStatus,
				UpdateTime:      params.UpdateTime,
			}); err != nil {
				response.Fail(c, http.StatusInternalServerError, "更新代理失败: "+err.Error())
				return
			}
			response.OK(c, response.Success())
			return
		}
		if err := repo.New(deps.Store.Write).InsertVpnProxyRecord(c.Request.Context(), params); err != nil {
			response.Fail(c, http.StatusInternalServerError, "保存代理失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// proxyDelete — GET /proxies/delete?id=
func proxyDelete(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Query("id"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "参数 id 无效")
			return
		}
		if err := repo.New(deps.Store.Write).DeleteVpnProxyRecord(c.Request.Context(), id); err != nil {
			response.Fail(c, http.StatusInternalServerError, "删除代理失败: "+err.Error())
			return
		}
		response.OK(c, response.Success())
	}
}

// dashboardStats — GET /api/stats
func dashboardStats(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		q := repo.New(deps.Store.Read)

		tenants, _ := q.ListTenants(ctx)
		proxies, _ := q.ListVpnProxyRecords(ctx)

		var instanceCount, backupCount, onlineCount int64
		deps.Store.Read.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM instance_detail`).Scan(&instanceCount)
		deps.Store.Read.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM instance_backup_detail`).Scan(&backupCount)
		deps.Store.Read.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM instance_detail WHERE on_line_enable = 1`).Scan(&onlineCount)

		type statsResp struct {
			TenantCount   int   `json:"tenantCount"`
			ProxyCount    int   `json:"proxyCount"`
			InstanceCount int64 `json:"instanceCount"`
			BackupCount   int64 `json:"backupCount"`
			OnlineCount   int64 `json:"onlineCount"`
		}
		response.OK(c, response.SuccessData(statsResp{
			TenantCount:   len(tenants),
			ProxyCount:    len(proxies),
			InstanceCount: instanceCount,
			BackupCount:   backupCount,
			OnlineCount:   onlineCount,
		}))
	}
}
