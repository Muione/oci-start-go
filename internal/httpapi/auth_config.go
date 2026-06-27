package httpapi

import (
	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/response"
)

// GET /api/config/mfa-enabled — public.
func configMfaEnabled(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		response.OK(c, response.SuccessData(gin.H{"enabled": deps.SysConf.GetBool(c.Request.Context(), "mfa.enabled")}))
	}
}

// GET /api/config/turnstile — public. SiteKey only returned when enabled.
func configTurnstile(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		enabled := deps.SysConf.GetBool(ctx, "turnstile.enabled")
		siteKey := ""
		if enabled {
			siteKey = deps.SysConf.GetString(ctx, "turnstile.site.key")
		}
		response.OK(c, response.SuccessData(gin.H{"enabled": enabled, "siteKey": siteKey}))
	}
}

// GET /api/config/message-enabled — public. Returns true if any notification
// channel (Telegram/DingTalk/Bark/Feishu) is configured.
func configMessageEnabled(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		// Check all supported notification channels.
		tg := deps.SysConf.GetString(ctx, "telegram.bot.token") != "" &&
			deps.SysConf.GetString(ctx, "telegram.chat.id") != ""
		dingtalk := deps.SysConf.GetString(ctx, "dingtalk.webhook") != ""
		bark := deps.SysConf.GetString(ctx, "bark.key") != ""
		feishu := deps.SysConf.GetString(ctx, "feishu.webhook") != ""

		enabled := tg || dingtalk || bark || feishu
		response.OK(c, response.SuccessData(gin.H{
			"enabled":  enabled,
			"telegram": tg,
			"dingtalk": dingtalk,
			"bark":     bark,
			"feishu":   feishu,
		}))
	}
}
