// Package notify provides the notification channel abstraction (SPEC S12).
// Phase 7 implementation: Telegram via Bot API HTTP GET. The Notifier
// interface is consumed by grabber (success/failure), traffic alerts,
// check-live, and offline detection. All previously stubbed log calls now
// route through here.
package notify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Notifier sends notification messages. Currently only Telegram; additional
// channels (DingTalk, Bark, Feishu) can be added by implementing this interface.
type Notifier interface {
	Send(ctx context.Context, msg string) error
	SendTemplate(ctx context.Context, tmpl string, args ...interface{}) error
}

// TelegramNotifier sends messages via the Telegram Bot API.
// Config is loaded from SystemConfig KV (telegram.bot.token, telegram.chat.id).
type TelegramNotifier struct {
	Token  string
	ChatID string
	Client *http.Client
	Logger zerolog.Logger
}

// NewTelegramNotifier creates a notifier. If token or chatID are empty,
// Send operations are no-ops (log-only).
func NewTelegramNotifier(token, chatID string, logger zerolog.Logger) *TelegramNotifier {
	return &TelegramNotifier{
		Token:  token,
		ChatID: chatID,
		Client: &http.Client{Timeout: 10 * time.Second},
		Logger: logger,
	}
}

// Enabled returns true if the notifier is configured.
func (n *TelegramNotifier) Enabled() bool {
	return n.Token != "" && n.ChatID != ""
}

// Send sends a plain-text message to the configured Telegram chat.
func (n *TelegramNotifier) Send(ctx context.Context, msg string) error {
	if !n.Enabled() {
		n.Logger.Debug().Str("msg", msg).Msg("notify: telegram not configured, log-only")
		return nil
	}
	return n.sendAPI(ctx, msg, "Markdown")
}

// SendTemplate formats a message and sends it.
func (n *TelegramNotifier) SendTemplate(ctx context.Context, tmpl string, args ...interface{}) error {
	msg := fmt.Sprintf(tmpl, args...)
	return n.Send(ctx, msg)
}

func (n *TelegramNotifier) sendAPI(ctx context.Context, text, parseMode string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.Token)
	form := url.Values{}
	form.Set("chat_id", n.ChatID)
	form.Set("text", text)
	if parseMode != "" {
		form.Set("parse_mode", parseMode)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := n.Client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error %d: %s", resp.StatusCode, string(body))
	}

	n.Logger.Debug().Int("len", len(text)).Msg("notify: telegram sent")
	return nil
}

var _ Notifier = (*TelegramNotifier)(nil)
