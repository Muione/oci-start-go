package httpapi

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/notify"
	"github.com/Muione/oci-start-go/internal/response"
)

// POST /system/notification/test — protected. Test send notification.
func notificationTest(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Channel string `json:"channel"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, http.StatusBadRequest, "参数无效")
			return
		}

		ctx := c.Request.Context()
		message := "🔔 这是一条测试通知\n时间: " + nowStr()

		var notifier notify.Notifier
		switch req.Channel {
		case "telegram":
			token := deps.SysConf.GetString(ctx, "telegram.bot.token")
			chatID := deps.SysConf.GetString(ctx, "telegram.chat.id")
			if token == "" || chatID == "" {
				recordNotification(deps, ctx, req.Channel, message, false, "未配置")
				response.Fail(c, http.StatusBadRequest, "Telegram 未配置")
				return
			}
			notifier = notify.NewTelegramNotifier(token, chatID, zerolog.Nop())
		case "dingtalk":
			webhook := deps.SysConf.GetString(ctx, "dingtalk.webhook")
			secret := deps.SysConf.GetString(ctx, "dingtalk.secret")
			if webhook == "" {
				recordNotification(deps, ctx, req.Channel, message, false, "未配置")
				response.Fail(c, http.StatusBadRequest, "钉钉未配置")
				return
			}
			notifier = notify.NewDingTalkNotifier(webhook, secret, zerolog.Nop())
		case "bark":
			key := deps.SysConf.GetString(ctx, "bark.key")
			if key == "" {
				recordNotification(deps, ctx, req.Channel, message, false, "未配置")
				response.Fail(c, http.StatusBadRequest, "Bark 未配置")
				return
			}
			server := deps.SysConf.GetString(ctx, "bark.server")
			if server == "" {
				server = "https://api.day.app"
			}
			notifier = notify.NewBarkNotifier(server, key, zerolog.Nop())
		case "feishu":
			webhook := deps.SysConf.GetString(ctx, "feishu.webhook")
			secret := deps.SysConf.GetString(ctx, "feishu.secret")
			if webhook == "" {
				recordNotification(deps, ctx, req.Channel, message, false, "未配置")
				response.Fail(c, http.StatusBadRequest, "飞书未配置")
				return
			}
			notifier = notify.NewFeishuNotifier(webhook, secret, zerolog.Nop())
		default:
			response.Fail(c, http.StatusBadRequest, "不支持的渠道")
			return
		}

		if err := notifier.Send(ctx, message); err != nil {
			recordNotification(deps, ctx, req.Channel, message, false, err.Error())
			response.Fail(c, http.StatusInternalServerError, "发送失败: "+err.Error())
			return
		}

		recordNotification(deps, ctx, req.Channel, message, true, "")
		response.OK(c, response.SuccessData(gin.H{
			"success": true,
			"message": "测试通知已发送",
		}))
	}
}

// notificationHistoryResp represents a notification history entry.
type notificationHistoryResp struct {
	ID           int64  `json:"id"`
	Channel      string `json:"channel"`
	Message      string `json:"message"`
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
	CreatedAt    string `json:"created_at"`
}

// GET /system/notification/history — protected. Returns notification history.
func notificationHistory(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		channel := c.Query("channel")
		limit := 50
		if l, err := strconv.Atoi(c.DefaultQuery("limit", "50")); err == nil && l > 0 && l <= 200 {
			limit = l
		}

		query := "SELECT id, channel, message, success, error_message, created_at FROM notification_history WHERE 1=1"
		args := []interface{}{}

		if channel != "" {
			query += " AND channel = ?"
			args = append(args, channel)
		}

		query += " ORDER BY created_at DESC LIMIT ?"
		args = append(args, limit)

		rows, err := deps.Store.Read.QueryContext(ctx, query, args...)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, "查询失败")
			return
		}
		defer rows.Close()

		history := []notificationHistoryResp{}
		for rows.Next() {
			var item notificationHistoryResp
			var success int
			if err := rows.Scan(&item.ID, &item.Channel, &item.Message, &success, &item.ErrorMessage, &item.CreatedAt); err != nil {
				continue
			}
			item.Success = success == 1
			history = append(history, item)
		}

		response.OK(c, response.SuccessData(gin.H{"history": history}))
	}
}

// recordNotification inserts a notification record into the database.
func recordNotification(deps *Deps, ctx context.Context, channel, message string, success bool, errMsg string) {
	successInt := 0
	if success {
		successInt = 1
	}
	_, _ = deps.Store.Write.ExecContext(ctx,
		`INSERT INTO notification_history (channel, message, success, error_message) VALUES (?, ?, ?, ?)`,
		channel, message, successInt, errMsg)
}
