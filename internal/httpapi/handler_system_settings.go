// Package httpapi — handler_system_settings.go: Unified system settings API.
// Provides a structured, categorized view of all system configuration.
package httpapi

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/sysconf"
)

// systemSettingsGet — GET /system/settings
// Returns all system settings organized by category.
func systemSettingsGet(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		settings := gin.H{
			"notification": getNotificationSettings(ctx, deps.SysConf),
			"security":     getSecuritySettings(ctx, deps.SysConf),
			"dns":          getDNSSettings(ctx, deps.SysConf),
			"ssl":          getSSLSettings(ctx, deps.SysConf),
			"proxy":        getProxySettings(ctx, deps.SysConf),
			"oauth":        getOAuthSettings(ctx, deps.SysConf),
		}

		response.OK(c, response.SuccessData(settings))
	}
}

// systemSettingsUpdate — PUT /system/settings
// Updates multiple system settings at once.
func systemSettingsUpdate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
			return
		}

		ctx := c.Request.Context()
		updated := make(map[string]bool)

		// Process each category
		if notif, ok := req["notification"].(map[string]interface{}); ok {
			updateNotificationSettings(ctx, deps.SysConf, notif, updated)
		}
		if sec, ok := req["security"].(map[string]interface{}); ok {
			updateSecuritySettings(ctx, deps.SysConf, sec, updated)
		}
		if dnsCfg, ok := req["dns"].(map[string]interface{}); ok {
			updateDNSSettings(ctx, deps.SysConf, dnsCfg, updated)
		}
		if sslCfg, ok := req["ssl"].(map[string]interface{}); ok {
			updateSSLSettings(ctx, deps.SysConf, sslCfg, updated)
		}
		if oauth, ok := req["oauth"].(map[string]interface{}); ok {
			updateOAuthSettings(ctx, deps.SysConf, oauth, updated)
		}

		response.OK(c, response.SuccessData(gin.H{
			"updated": updated,
			"count":   len(updated),
		}))
	}
}

// =========================================================================
// Notification settings
// =========================================================================

func getNotificationSettings(ctx context.Context, sc *sysconf.Service) gin.H {
	return gin.H{
		"telegram": gin.H{
			"enabled": sc.GetString(ctx, "telegram.bot.token") != "" && sc.GetString(ctx, "telegram.chat.id") != "",
			"botToken": maskSecret(sc.GetString(ctx, "telegram.bot.token")),
			"chatId":   sc.GetString(ctx, "telegram.chat.id"),
		},
		"dingtalk": gin.H{
			"enabled": sc.GetString(ctx, "dingtalk.webhook") != "",
			"webhook": maskSecret(sc.GetString(ctx, "dingtalk.webhook")),
			"secret":  maskSecret(sc.GetString(ctx, "dingtalk.secret")),
		},
		"bark": gin.H{
			"enabled": sc.GetString(ctx, "bark.key") != "",
			"server":  sc.GetString(ctx, "bark.server"),
			"key":     maskSecret(sc.GetString(ctx, "bark.key")),
		},
		"feishu": gin.H{
			"enabled": sc.GetString(ctx, "feishu.webhook") != "",
			"webhook": maskSecret(sc.GetString(ctx, "feishu.webhook")),
			"secret":  maskSecret(sc.GetString(ctx, "feishu.secret")),
		},
	}
}

func updateNotificationSettings(ctx context.Context, sc *sysconf.Service, data map[string]interface{}, updated map[string]bool) {
	if tg, ok := data["telegram"].(map[string]interface{}); ok {
		if v, ok := tg["botToken"].(string); ok {
			sc.SetString(ctx, "telegram.bot.token", v)
			updated["telegram.bot.token"] = true
		}
		if v, ok := tg["chatId"].(string); ok {
			sc.SetString(ctx, "telegram.chat.id", v)
			updated["telegram.chat.id"] = true
		}
	}
	if dt, ok := data["dingtalk"].(map[string]interface{}); ok {
		if v, ok := dt["webhook"].(string); ok {
			sc.SetString(ctx, "dingtalk.webhook", v)
			updated["dingtalk.webhook"] = true
		}
		if v, ok := dt["secret"].(string); ok {
			sc.SetString(ctx, "dingtalk.secret", v)
			updated["dingtalk.secret"] = true
		}
	}
	if bark, ok := data["bark"].(map[string]interface{}); ok {
		if v, ok := bark["server"].(string); ok {
			sc.SetString(ctx, "bark.server", v)
			updated["bark.server"] = true
		}
		if v, ok := bark["key"].(string); ok {
			sc.SetString(ctx, "bark.key", v)
			updated["bark.key"] = true
		}
	}
	if fs, ok := data["feishu"].(map[string]interface{}); ok {
		if v, ok := fs["webhook"].(string); ok {
			sc.SetString(ctx, "feishu.webhook", v)
			updated["feishu.webhook"] = true
		}
		if v, ok := fs["secret"].(string); ok {
			sc.SetString(ctx, "feishu.secret", v)
			updated["feishu.secret"] = true
		}
	}
}

// =========================================================================
// Security settings
// =========================================================================

func getSecuritySettings(ctx context.Context, sc *sysconf.Service) gin.H {
	return gin.H{
		"turnstile": gin.H{
			"enabled":   sc.GetBool(ctx, "turnstile.enabled"),
			"siteKey":   sc.GetString(ctx, "turnstile.site.key"),
			"secretKey": maskSecret(sc.GetString(ctx, "turnstile.secret.key")),
		},
		"mfa": gin.H{
			"enabled": sc.GetBool(ctx, "mfa.enabled"),
		},
	}
}

func updateSecuritySettings(ctx context.Context, sc *sysconf.Service, data map[string]interface{}, updated map[string]bool) {
	if ts, ok := data["turnstile"].(map[string]interface{}); ok {
		if v, ok := ts["enabled"].(bool); ok {
			sc.SetEnabled(ctx, "turnstile.enabled", v)
			updated["turnstile.enabled"] = true
		}
		if v, ok := ts["siteKey"].(string); ok {
			sc.SetString(ctx, "turnstile.site.key", v)
			updated["turnstile.site.key"] = true
		}
		if v, ok := ts["secretKey"].(string); ok {
			sc.SetString(ctx, "turnstile.secret.key", v)
			updated["turnstile.secret.key"] = true
		}
	}
	if mfa, ok := data["mfa"].(map[string]interface{}); ok {
		if v, ok := mfa["enabled"].(bool); ok {
			sc.SetEnabled(ctx, "mfa.enabled", v)
			updated["mfa.enabled"] = true
		}
	}
}

// =========================================================================
// DNS settings
// =========================================================================

func getDNSSettings(ctx context.Context, sc *sysconf.Service) gin.H {
	return gin.H{
		"cloudflare": gin.H{
			"configured": sc.GetString(ctx, "cloudflare.api.token") != "",
			"apiToken":   maskSecret(sc.GetString(ctx, "cloudflare.api.token")),
		},
		"edgeone": gin.H{
			"configured": sc.GetString(ctx, "edgeone.secretId") != "",
			"secretId":   maskSecret(sc.GetString(ctx, "edgeone.secretId")),
			"secretKey":  maskSecret(sc.GetString(ctx, "edgeone.secretKey")),
			"zoneId":     sc.GetString(ctx, "edgeone.zoneId"),
		},
	}
}

func updateDNSSettings(ctx context.Context, sc *sysconf.Service, data map[string]interface{}, updated map[string]bool) {
	if cf, ok := data["cloudflare"].(map[string]interface{}); ok {
		if v, ok := cf["apiToken"].(string); ok {
			sc.SetString(ctx, "cloudflare.api.token", v)
			updated["cloudflare.api.token"] = true
		}
	}
	if eo, ok := data["edgeone"].(map[string]interface{}); ok {
		if v, ok := eo["secretId"].(string); ok {
			sc.SetString(ctx, "edgeone.secretId", v)
			updated["edgeone.secretId"] = true
		}
		if v, ok := eo["secretKey"].(string); ok {
			sc.SetString(ctx, "edgeone.secretKey", v)
			updated["edgeone.secretKey"] = true
		}
		if v, ok := eo["zoneId"].(string); ok {
			sc.SetString(ctx, "edgeone.zoneId", v)
			updated["edgeone.zoneId"] = true
		}
	}
}

// =========================================================================
// SSL settings
// =========================================================================

func getSSLSettings(ctx context.Context, sc *sysconf.Service) gin.H {
	return gin.H{
		"domain":  sc.GetString(ctx, "ssl.domain"),
		"email":   sc.GetString(ctx, "ssl.email"),
		"staging": sc.GetBool(ctx, "ssl.staging"),
	}
}

func updateSSLSettings(ctx context.Context, sc *sysconf.Service, data map[string]interface{}, updated map[string]bool) {
	if v, ok := data["domain"].(string); ok {
		sc.SetString(ctx, "ssl.domain", v)
		updated["ssl.domain"] = true
	}
	if v, ok := data["email"].(string); ok {
		sc.SetString(ctx, "ssl.email", v)
		updated["ssl.email"] = true
	}
	if v, ok := data["staging"].(bool); ok {
		sc.SetEnabled(ctx, "ssl.staging", v)
		updated["ssl.staging"] = true
	}
}

// =========================================================================
// Proxy settings
// =========================================================================

func getProxySettings(ctx context.Context, sc *sysconf.Service) gin.H {
	cfg := sc.GetProxyConfig(ctx)
	return gin.H{
		"enabled":  cfg.Enabled,
		"type":     cfg.Type,
		"host":     cfg.Host,
		"port":     cfg.Port,
		"username": cfg.Username,
		"password": maskSecret(cfg.Password),
	}
}

// =========================================================================
// OAuth settings
// =========================================================================

func getOAuthSettings(ctx context.Context, sc *sysconf.Service) gin.H {
	return gin.H{
		"github": gin.H{
			"enabled":      sc.GetBool(ctx, "github.enabled"),
			"clientId":     maskSecret(sc.GetString(ctx, "github.client.id")),
			"clientSecret": maskSecret(sc.GetString(ctx, "github.client.secret")),
			"redirectUri":  sc.GetString(ctx, "github.redirect.uri"),
		},
		"google": gin.H{
			"enabled":      sc.GetBool(ctx, "google.enabled"),
			"clientId":     maskSecret(sc.GetString(ctx, "google.client.id")),
			"clientSecret": maskSecret(sc.GetString(ctx, "google.client.secret")),
			"redirectUri":  sc.GetString(ctx, "google.redirect.uri"),
		},
	}
}

func updateOAuthSettings(ctx context.Context, sc *sysconf.Service, data map[string]interface{}, updated map[string]bool) {
	if gh, ok := data["github"].(map[string]interface{}); ok {
		if v, ok := gh["enabled"].(bool); ok {
			sc.SetEnabled(ctx, "github.enabled", v)
			updated["github.enabled"] = true
		}
		if v, ok := gh["clientId"].(string); ok {
			sc.SetString(ctx, "github.client.id", v)
			updated["github.client.id"] = true
		}
		if v, ok := gh["clientSecret"].(string); ok {
			sc.SetString(ctx, "github.client.secret", v)
			updated["github.client.secret"] = true
		}
		if v, ok := gh["redirectUri"].(string); ok {
			sc.SetString(ctx, "github.redirect.uri", v)
			updated["github.redirect.uri"] = true
		}
	}
	if gg, ok := data["google"].(map[string]interface{}); ok {
		if v, ok := gg["enabled"].(bool); ok {
			sc.SetEnabled(ctx, "google.enabled", v)
			updated["google.enabled"] = true
		}
		if v, ok := gg["clientId"].(string); ok {
			sc.SetString(ctx, "google.client.id", v)
			updated["google.client.id"] = true
		}
		if v, ok := gg["clientSecret"].(string); ok {
			sc.SetString(ctx, "google.client.secret", v)
			updated["google.client.secret"] = true
		}
		if v, ok := gg["redirectUri"].(string); ok {
			sc.SetString(ctx, "google.redirect.uri", v)
			updated["google.redirect.uri"] = true
		}
	}
}

// =========================================================================
// Helpers
// =========================================================================

// maskSecret masks a secret string, showing only first 4 and last 4 characters.
func maskSecret(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "***" + s[len(s)-4:]
}
