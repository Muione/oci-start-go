// Package notify — channels.go: DingTalk, Bark, and Feishu notification
// channels (Phase 8). All implement the Notifier interface for drop-in
// use by grabber, traffic alerts, check-live, and offline detection.
package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// ==========================================================================
// MultiNotifier fans out sends to all configured channels.
// ==========================================================================

// MultiNotifier sends to multiple notifiers in parallel.
type MultiNotifier struct {
	channels []Notifier
	Logger   zerolog.Logger
}

// NewMultiNotifier creates a fan-out notifier.
func NewMultiNotifier(logger zerolog.Logger, channels ...Notifier) *MultiNotifier {
	active := make([]Notifier, 0, len(channels))
	for _, ch := range channels {
		if ch != nil {
			active = append(active, ch)
		}
	}
	return &MultiNotifier{channels: active, Logger: logger}
}

// Send fans out to all channels. Errors are logged but not propagated.
func (m *MultiNotifier) Send(ctx context.Context, msg string) error {
	for _, ch := range m.channels {
		go func(n Notifier) {
			if err := n.Send(ctx, msg); err != nil {
				m.Logger.Warn().Err(err).Msg("multi-notify: channel send failed")
			}
		}(ch)
	}
	return nil
}

// SendTemplate formats and fans out.
func (m *MultiNotifier) SendTemplate(ctx context.Context, tmpl string, args ...interface{}) error {
	msg := fmt.Sprintf(tmpl, args...)
	return m.Send(ctx, msg)
}

var _ Notifier = (*MultiNotifier)(nil)

// ==========================================================================
// DingTalk (钉钉) notification — webhook robot.
// Config: dingtalk.webhook (full webhook URL), dingtalk.secret (optional HMAC sign key).
// ==========================================================================

// DingTalkNotifier sends messages to a DingTalk group via webhook robot.
// Supports text and markdown message types.
type DingTalkNotifier struct {
	Webhook string
	Secret  string // optional HMAC-SHA256 signing secret
	Client  *http.Client
	Logger  zerolog.Logger
}

// NewDingTalkNotifier creates a DingTalk notifier. If httpClient is nil,
// a default client with 10s timeout is used.
func NewDingTalkNotifier(webhook, secret string, logger zerolog.Logger, httpClient ...*http.Client) *DingTalkNotifier {
	client := &http.Client{Timeout: 10 * time.Second}
	if len(httpClient) > 0 && httpClient[0] != nil {
		client = httpClient[0]
	}
	return &DingTalkNotifier{
		Webhook: webhook,
		Secret:  secret,
		Client:  client,
		Logger:  logger,
	}
}

// Enabled returns true if webhook is configured.
func (d *DingTalkNotifier) Enabled() bool {
	return d.Webhook != ""
}

// Send sends a markdown message to DingTalk.
func (d *DingTalkNotifier) Send(ctx context.Context, msg string) error {
	if !d.Enabled() {
		d.Logger.Debug().Msg("notify: dingtalk not configured, log-only")
		return nil
	}
	return d.sendWebhook(ctx, msg)
}

// SendTemplate formats and sends.
func (d *DingTalkNotifier) SendTemplate(ctx context.Context, tmpl string, args ...interface{}) error {
	msg := fmt.Sprintf(tmpl, args...)
	return d.Send(ctx, msg)
}

func (d *DingTalkNotifier) sendWebhook(ctx context.Context, text string) error {
	body := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": "OCI-Start 通知",
			"text":  text,
		},
	}

	data, _ := json.Marshal(body)

	targetURL := d.Webhook
	if d.Secret != "" {
		timestamp := time.Now().UnixMilli()
		sign := d.sign(timestamp)
		if strings.Contains(targetURL, "?") {
			targetURL += "&"
		} else {
			targetURL += "?"
		}
		targetURL += fmt.Sprintf("timestamp=%d&sign=%s", timestamp, sign)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("dingtalk request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.Client.Do(req)
	if err != nil {
		return fmt.Errorf("dingtalk send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		rbody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dingtalk API error %d: %s", resp.StatusCode, string(rbody))
	}

	d.Logger.Debug().Int("len", len(text)).Msg("notify: dingtalk sent")
	return nil
}

func (d *DingTalkNotifier) sign(timestamp int64) string {
	str := fmt.Sprintf("%d\n%s", timestamp, d.Secret)
	h := hmac.New(sha256.New, []byte(d.Secret))
	h.Write([]byte(str))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

var _ Notifier = (*DingTalkNotifier)(nil)

// ==========================================================================
// Bark notification — iOS push notification via Bark server.
// Config: bark.server (Bark server URL), bark.key (device key).
// ==========================================================================

// BarkNotifier sends push notifications via Bark (iOS).
type BarkNotifier struct {
	Server string // Bark server URL, e.g. https://api.day.app
	Key    string // Device key
	Client *http.Client
	Logger zerolog.Logger
}

// NewBarkNotifier creates a Bark notifier. If httpClient is nil,
// a default client with 10s timeout is used.
func NewBarkNotifier(server, key string, logger zerolog.Logger, httpClient ...*http.Client) *BarkNotifier {
	if server == "" {
		server = "https://api.day.app"
	}
	client := &http.Client{Timeout: 10 * time.Second}
	if len(httpClient) > 0 && httpClient[0] != nil {
		client = httpClient[0]
	}
	return &BarkNotifier{
		Server: server,
		Key:    key,
		Client: client,
		Logger: logger,
	}
}

// Enabled returns true if key is configured.
func (b *BarkNotifier) Enabled() bool {
	return b.Key != ""
}

// Send sends a push notification via Bark.
func (b *BarkNotifier) Send(ctx context.Context, msg string) error {
	if !b.Enabled() {
		b.Logger.Debug().Msg("notify: bark not configured, log-only")
		return nil
	}

	// Extract first line as title, rest as body
	title := "OCI-Start"
	body := msg
	if idx := strings.Index(msg, "\n"); idx > 0 {
		title = msg[:idx]
		body = msg[idx+1:]
	}

	apiURL := fmt.Sprintf("%s/%s/%s/%s", strings.TrimRight(b.Server, "/"), b.Key,
		url.PathEscape(title), url.PathEscape(body))
	// Add URL params for sound and badge
	apiURL += "?sound=notification&badge=1"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("bark request: %w", err)
	}

	resp, err := b.Client.Do(req)
	if err != nil {
		return fmt.Errorf("bark send: %w", err)
	}
	defer resp.Body.Close()

	rbody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bark API error %d: %s", resp.StatusCode, string(rbody))
	}

	b.Logger.Debug().Msg("notify: bark sent")
	return nil
}

// SendTemplate formats and sends.
func (b *BarkNotifier) SendTemplate(ctx context.Context, tmpl string, args ...interface{}) error {
	msg := fmt.Sprintf(tmpl, args...)
	return b.Send(ctx, msg)
}

var _ Notifier = (*BarkNotifier)(nil)

// ==========================================================================
// Feishu (飞书) notification — webhook robot.
// Config: feishu.webhook (full webhook URL), feishu.secret (optional sign key).
// ==========================================================================

// FeishuNotifier sends messages to a Feishu (Lark) group via webhook robot.
type FeishuNotifier struct {
	Webhook string
	Secret  string // optional HMAC-SHA256 signing secret
	Client  *http.Client
	Logger  zerolog.Logger
}

// NewFeishuNotifier creates a Feishu notifier. If httpClient is nil,
// a default client with 10s timeout is used.
func NewFeishuNotifier(webhook, secret string, logger zerolog.Logger, httpClient ...*http.Client) *FeishuNotifier {
	client := &http.Client{Timeout: 10 * time.Second}
	if len(httpClient) > 0 && httpClient[0] != nil {
		client = httpClient[0]
	}
	return &FeishuNotifier{
		Webhook: webhook,
		Secret:  secret,
		Client:  client,
		Logger:  logger,
	}
}

// Enabled returns true if webhook is configured.
func (f *FeishuNotifier) Enabled() bool {
	return f.Webhook != ""
}

// Send sends a rich text message to Feishu.
func (f *FeishuNotifier) Send(ctx context.Context, msg string) error {
	if !f.Enabled() {
		f.Logger.Debug().Msg("notify: feishu not configured, log-only")
		return nil
	}
	return f.sendWebhook(ctx, msg)
}

// SendTemplate formats and sends.
func (f *FeishuNotifier) SendTemplate(ctx context.Context, tmpl string, args ...interface{}) error {
	msg := fmt.Sprintf(tmpl, args...)
	return f.Send(ctx, msg)
}

func (f *FeishuNotifier) sendWebhook(ctx context.Context, text string) error {
	body := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]string{
					"tag":     "plain_text",
					"content": "OCI-Start 通知",
				},
				"template": "blue",
			},
			"elements": []map[string]interface{}{
				{
					"tag":  "markdown",
					"content": text,
				},
			},
		},
	}

	data, _ := json.Marshal(body)

	targetURL := f.Webhook
	if f.Secret != "" {
		timestamp := time.Now().Unix()
		sign := f.sign(timestamp)
		if strings.Contains(targetURL, "?") {
			targetURL += "&"
		} else {
			targetURL += "?"
		}
		targetURL += fmt.Sprintf("timestamp=%d&sign=%s", timestamp, sign)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("feishu request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.Client.Do(req)
	if err != nil {
		return fmt.Errorf("feishu send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		rbody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("feishu API error %d: %s", resp.StatusCode, string(rbody))
	}

	f.Logger.Debug().Int("len", len(text)).Msg("notify: feishu sent")
	return nil
}

func (f *FeishuNotifier) sign(timestamp int64) string {
	str := fmt.Sprintf("%d\n%s", timestamp, f.Secret)
	h := hmac.New(sha256.New, []byte(f.Secret))
	h.Write([]byte(str))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

var _ Notifier = (*FeishuNotifier)(nil)
