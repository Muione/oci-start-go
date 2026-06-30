// Package httpapi — handler_system_proxy.go: System outbound proxy configuration.
// These endpoints allow the admin to configure an application-level proxy for
// outbound HTTP traffic (Telegram bot, external API calls, etc.).
package httpapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/sysconf"
)

// systemProxyGet — GET /system/proxy
// Returns the current proxy configuration.
func systemProxyGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := deps.SysConf.GetProxyConfig(c.Request.Context())
		response.OK(c, response.SuccessData(cfg))
	}
}

type systemProxyUpdateReq struct {
	Type     string `json:"type" binding:"required,oneof=HTTP HTTPS SOCKS5"`
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port" binding:"required,min=1,max=65535"`
	Username string `json:"username"`
	Password string `json:"password"`
	Enabled  bool   `json:"enabled"`
}

// systemProxyUpdate — PUT /system/proxy
// Updates the proxy configuration.
func systemProxyUpdate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req systemProxyUpdateReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
			return
		}
		cfg := sysconf.ProxyConfig{
			Type:     req.Type,
			Host:     req.Host,
			Port:     req.Port,
			Username: req.Username,
			Password: req.Password,
			Enabled:  req.Enabled,
		}
		if err := deps.SysConf.SetProxyConfig(c.Request.Context(), cfg); err != nil {
			response.Fail(c, http.StatusInternalServerError, "保存代理配置失败: "+err.Error())
			return
		}
		response.OK(c, response.SuccessData(cfg))
	}
}

type systemProxyTestReq struct {
	Type     string `json:"type" binding:"required,oneof=HTTP HTTPS SOCKS5"`
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port" binding:"required,min=1,max=65535"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// systemProxyTest — POST /system/proxy/test
// Tests proxy connectivity by attempting a TCP dial through the proxy.
func systemProxyTest(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req systemProxyTestReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
			return
		}

		addr := fmt.Sprintf("%s:%d", req.Host, req.Port)
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			response.Fail(c, http.StatusBadGateway, fmt.Sprintf("无法连接到代理 %s: %s", addr, err.Error()))
			return
		}
		conn.Close()

		response.OK(c, response.SuccessData(gin.H{
			"reachable": true,
			"message":   fmt.Sprintf("代理 %s 连接成功", addr),
		}))
	}
}

// systemProxyGetSaved — GET /system/proxy/saved
// Returns the saved proxy config from DB (used by internal consumers).
func systemProxyGetSaved(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := deps.SysConf.GetProxyConfig(c.Request.Context())
		response.OK(c, response.SuccessData(cfg))
	}
}
