package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// capturedReq captures the inbound request shape for test assertions without
// holding onto the recycled *http.Request after the handler returns.
type capturedReq struct {
	method    string
	path      string
	query     url.Values
	rawQuery  string
	body      []byte
}

func newCaptureServer(got *capturedReq) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got.method = r.Method
		got.path = r.URL.Path
		got.query = r.URL.Query()
		got.rawQuery = r.URL.RawQuery
		got.body = body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

// rawQueryParam returns the value of name from a raw query string without
// form-decoding. The notifiers write base64 signs into the URL unescaped
// (same pattern DingTalk uses), so url.Values.Get would corrupt '+' to space.
// Reading the raw query compares the exact bytes on the wire.
// ponytail: not a general query parser — base64 alphabet has no '&'/'=', so
// naive split is safe here; do not reuse for arbitrary query values.
func rawQueryParam(rawQuery, name string) string {
	for _, pair := range strings.Split(rawQuery, "&") {
		if eq := strings.IndexByte(pair, '='); eq >= 0 && pair[:eq] == name {
			return pair[eq+1:]
		}
	}
	return ""
}

// TestFeishuNotifier_SignUsesSecretAsHMACKey verifies the Feishu webhook
// signature is HMAC-SHA256 keyed by f.Secret over "<timestamp>\n<secret>".
// Regression guard: sign previously used the to-be-signed string as the key,
// so every signed request was rejected by Feishu.
func TestFeishuNotifier_SignUsesSecretAsHMACKey(t *testing.T) {
	var got capturedReq
	srv := newCaptureServer(&got)
	defer srv.Close()

	const secret = "test-secret-123"
	notifier := NewFeishuNotifier(srv.URL, secret, zerolog.Nop(), srv.Client())
	if err := notifier.Send(context.Background(), "hello feishu"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	tsStr := got.query.Get("timestamp")
	if tsStr == "" {
		t.Fatal("missing timestamp query param")
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		t.Fatalf("parse timestamp %q: %v", tsStr, err)
	}

	// Expected: HMAC-SHA256 with f.Secret as the key.
	str := fmt.Sprintf("%d\n%s", ts, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(str))
	wantSign := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if gotSign := rawQueryParam(got.rawQuery, "sign"); gotSign != wantSign {
		t.Fatalf("feishu signature mismatch:\n got  %s\n want %s", gotSign, wantSign)
	}
}

// TestDingTalkNotifier_SignAndBody verifies DingTalk signs correctly (key =
// d.Secret, the canonical reference Feishu previously deviated from) and
// posts a markdown body. Locks in the correct signing so it is not "fixed"
// to match the broken Feishu path.
func TestDingTalkNotifier_SignAndBody(t *testing.T) {
	var got capturedReq
	srv := newCaptureServer(&got)
	defer srv.Close()

	const secret = "ding-secret-xyz"
	notifier := NewDingTalkNotifier(srv.URL, secret, zerolog.Nop(), srv.Client())
	if err := notifier.Send(context.Background(), "hello dingtalk"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	if got.method != http.MethodPost {
		t.Fatalf("method = %s, want POST", got.method)
	}

	tsStr := got.query.Get("timestamp")
	if tsStr == "" {
		t.Fatal("missing timestamp query param")
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		t.Fatalf("parse timestamp %q: %v", tsStr, err)
	}
	str := fmt.Sprintf("%d\n%s", ts, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(str))
	wantSign := base64.StdEncoding.EncodeToString(h.Sum(nil))
	if gotSign := rawQueryParam(got.rawQuery, "sign"); gotSign != wantSign {
		t.Fatalf("dingtalk signature mismatch:\n got  %s\n want %s", gotSign, wantSign)
	}

	var body struct {
		MsgType  string `json:"msgtype"`
		Markdown struct {
			Title string `json:"title"`
			Text  string `json:"text"`
		} `json:"markdown"`
	}
	if err := json.Unmarshal(got.body, &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.MsgType != "markdown" {
		t.Errorf("msgtype = %q, want markdown", body.MsgType)
	}
	if !strings.Contains(body.Markdown.Text, "hello dingtalk") {
		t.Errorf("markdown.text = %q, want to contain message", body.Markdown.Text)
	}
}

// TestTelegramNotifier_Smoke verifies the Telegram Bot API request shape:
// POST /bot<token>/sendMessage with chat_id, text, parse_mode in the form.
func TestTelegramNotifier_Smoke(t *testing.T) {
	var got capturedReq
	srv := newCaptureServer(&got)
	defer srv.Close()

	tu, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse srv url: %v", err)
	}
	// Telegram hardcodes api.telegram.org; rewrite host to the test server.
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &hostRewriter{base: http.DefaultTransport, target: tu},
	}
	const token, chatID = "FAKETOKEN", "987654321"
	notifier := NewTelegramNotifier(token, chatID, zerolog.Nop(), client)
	if err := notifier.Send(context.Background(), "hello tg"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	wantPath := fmt.Sprintf("/bot%s/sendMessage", token)
	if got.path != wantPath {
		t.Errorf("path = %q, want %q", got.path, wantPath)
	}
	if got.method != http.MethodPost {
		t.Errorf("method = %s, want POST", got.method)
	}
	form, err := url.ParseQuery(string(got.body))
	if err != nil {
		t.Fatalf("parse form body: %v", err)
	}
	if form.Get("chat_id") != chatID {
		t.Errorf("chat_id = %q, want %q", form.Get("chat_id"), chatID)
	}
	if !strings.Contains(form.Get("text"), "hello tg") {
		t.Errorf("text = %q, want message", form.Get("text"))
	}
	if form.Get("parse_mode") != "Markdown" {
		t.Errorf("parse_mode = %q, want Markdown", form.Get("parse_mode"))
	}
}

// hostRewriter redirects all requests to target (scheme+host), preserving
// path and query. Lets Telegram's hardcoded api.telegram.org hit a test
// server.
type hostRewriter struct {
	base   http.RoundTripper
	target *url.URL
}

func (h *hostRewriter) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.URL.Scheme = h.target.Scheme
	r.URL.Host = h.target.Host
	r.Host = h.target.Host
	return h.base.RoundTrip(r)
}
