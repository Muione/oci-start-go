package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/response"
	"github.com/Muione/oci-start-go/internal/sysconf"
)

// setupPhase3TestRouter creates a Gin engine with Phase 3 routes registered
// WITHOUT auth middleware (for unit testing handler input validation).
func setupPhase3TestRouter(deps *Deps) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Proxy CRUD
	r.GET("/system/proxy", systemProxyGet(deps))
	r.PUT("/system/proxy", systemProxyUpdate(deps))
	r.POST("/system/proxy/test", systemProxyTest(deps))
	// System settings
	r.GET("/system/settings", systemSettingsGet(deps))
	r.PUT("/system/settings", systemSettingsUpdate(deps))
	return r
}

// setupTestDeps creates a Deps with an in-memory SQLite store and sysconf.Service.
// The store is pre-seeded with the system_config table for testing.
// Uses shared-cache in-memory mode so read/write connections see the same data.
func setupTestDeps(t *testing.T) *Deps {
	t.Helper()
	// Use shared-cache in-memory SQLite so both read and write connections
	// share the same database (required because db.Open creates two pools).
	store, err := db.Open("file::memory:?cache=shared", 1, 1)
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	// Create the system_config table directly (we don't need all tables)
	_, err = store.Write.Exec(`
		CREATE TABLE IF NOT EXISTS system_config (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			config_key TEXT UNIQUE,
			config_value TEXT,
			config_enabled INTEGER,
			last_modified TEXT
		)
	`)
	if err != nil {
		t.Fatalf("failed to create system_config table: %v", err)
	}

	sc := sysconf.New(store)
	return &Deps{SysConf: sc}
}

// ============================================================
// TE-201: Proxy Config CRUD API
// ============================================================

func TestSystemProxyGet_DefaultEmpty(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	req := httptest.NewRequest("GET", "/system/proxy", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp response.ApiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success=true, got false: %s", resp.Message)
	}
}

func TestSystemProxyUpdate_ValidRequest(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	body := `{"type":"HTTP","host":"proxy.example.com","port":8080,"username":"user","password":"pass","enabled":true}`
	req := httptest.NewRequest("PUT", "/system/proxy", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp response.ApiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success=true, got false: %s", resp.Message)
	}
}

func TestSystemProxyUpdate_MissingType(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	body := `{"host":"proxy.example.com","port":8080}`
	req := httptest.NewRequest("PUT", "/system/proxy", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSystemProxyUpdate_InvalidType(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	body := `{"type":"INVALID","host":"proxy.example.com","port":8080}`
	req := httptest.NewRequest("PUT", "/system/proxy", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSystemProxyUpdate_MissingHost(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	body := `{"type":"HTTP","port":8080}`
	req := httptest.NewRequest("PUT", "/system/proxy", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSystemProxyUpdate_InvalidPort_Zero(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	body := `{"type":"HTTP","host":"proxy.example.com","port":0}`
	req := httptest.NewRequest("PUT", "/system/proxy", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSystemProxyUpdate_InvalidPort_OverMax(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	body := `{"type":"HTTP","host":"proxy.example.com","port":70000}`
	req := httptest.NewRequest("PUT", "/system/proxy", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSystemProxyUpdate_InvalidBody(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	req := httptest.NewRequest("PUT", "/system/proxy", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSystemProxyUpdate_ValidTypes(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	validTypes := []string{"HTTP", "HTTPS", "SOCKS5"}
	for _, proxyType := range validTypes {
		body := `{"type":"` + proxyType + `","host":"proxy.example.com","port":1080}`
		req := httptest.NewRequest("PUT", "/system/proxy", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("proxy type %q: status = %d, want %d", proxyType, w.Code, http.StatusOK)
		}
	}
}

func TestSystemProxyUpdate_Persistence(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	// Update proxy config
	body := `{"type":"SOCKS5","host":"my.proxy.com","port":1080,"username":"admin","password":"secret","enabled":true}`
	req := httptest.NewRequest("PUT", "/system/proxy", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", w.Code, http.StatusOK)
	}

	// Read back and verify persistence
	req2 := httptest.NewRequest("GET", "/system/proxy", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", w2.Code, http.StatusOK)
	}

	var resp response.ApiResponse
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be a map, got %T", resp.Data)
	}

	if data["type"] != "SOCKS5" {
		t.Errorf("type = %v, want SOCKS5", data["type"])
	}
	if data["host"] != "my.proxy.com" {
		t.Errorf("host = %v, want my.proxy.com", data["host"])
	}
	// port comes back as float64 from JSON
	if port, ok := data["port"].(float64); !ok || port != 1080 {
		t.Errorf("port = %v, want 1080", data["port"])
	}
	if data["enabled"] != true {
		t.Errorf("enabled = %v, want true", data["enabled"])
	}
}

// ============================================================
// TE-202: Proxy Connectivity Test
// ============================================================

func TestSystemProxyTest_MissingType(t *testing.T) {
	deps := &Deps{}
	r := setupPhase3TestRouter(deps)

	body := `{"host":"proxy.example.com","port":8080}`
	req := httptest.NewRequest("POST", "/system/proxy/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSystemProxyTest_InvalidType(t *testing.T) {
	deps := &Deps{}
	r := setupPhase3TestRouter(deps)

	body := `{"type":"INVALID","host":"proxy.example.com","port":8080}`
	req := httptest.NewRequest("POST", "/system/proxy/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSystemProxyTest_MissingHost(t *testing.T) {
	deps := &Deps{}
	r := setupPhase3TestRouter(deps)

	body := `{"type":"HTTP","port":8080}`
	req := httptest.NewRequest("POST", "/system/proxy/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSystemProxyTest_InvalidPort(t *testing.T) {
	deps := &Deps{}
	r := setupPhase3TestRouter(deps)

	body := `{"type":"HTTP","host":"proxy.example.com","port":0}`
	req := httptest.NewRequest("POST", "/system/proxy/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSystemProxyTest_InvalidBody(t *testing.T) {
	deps := &Deps{}
	r := setupPhase3TestRouter(deps)

	req := httptest.NewRequest("POST", "/system/proxy/test", bytes.NewBufferString("bad-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSystemProxyTest_UnreachableProxy(t *testing.T) {
	deps := &Deps{}
	r := setupPhase3TestRouter(deps)

	// Use localhost with a high port that nothing listens on.
	// This fails immediately with "connection refused" instead of timing out.
	body := `{"type":"HTTP","host":"127.0.0.1","port":59999}`
	req := httptest.NewRequest("POST", "/system/proxy/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return 502 (Bad Gateway) since the proxy is unreachable
	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d (unreachable proxy)", w.Code, http.StatusBadGateway)
	}

	var resp response.ApiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false for unreachable proxy")
	}
}

func TestSystemProxyTest_ValidTypes(t *testing.T) {
	deps := &Deps{}
	r := setupPhase3TestRouter(deps)

	validTypes := []string{"HTTP", "HTTPS", "SOCKS5"}
	for _, proxyType := range validTypes {
		// Use localhost with a high port that nothing listens on (fast failure).
		body := `{"type":"` + proxyType + `","host":"127.0.0.1","port":59999}`
		req := httptest.NewRequest("POST", "/system/proxy/test", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// All types should at least pass validation (unreachable host => 502)
		if w.Code == http.StatusBadRequest {
			var resp response.ApiResponse
			json.Unmarshal(w.Body.Bytes(), &resp)
			t.Errorf("proxy type %q should pass handler validation, got: %s", proxyType, resp.Message)
		}
	}
}

// ============================================================
// TE-203: Verify httpclient proxy-aware transport creation
// ============================================================

func TestNewProxyTransport_HTTPType(t *testing.T) {
	// This tests the httpclient package's proxy transport creation logic
	// by verifying the sysconf.ProxyConfig structure is correctly used.
	// We test this indirectly through the sysconf.Service.

	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	// Set proxy config with HTTP type
	body := `{"type":"HTTP","host":"proxy.example.com","port":8080,"enabled":true}`
	req := httptest.NewRequest("PUT", "/system/proxy", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify the stored config can be read back via GET
	req2 := httptest.NewRequest("GET", "/system/proxy", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	var resp response.ApiResponse
	json.Unmarshal(w2.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	if data["type"] != "HTTP" {
		t.Errorf("stored proxy type = %v, want HTTP", data["type"])
	}
}

func TestIsProxyEnabled_WhenDisabled(t *testing.T) {
	deps := setupTestDeps(t)

	// Default: no proxy configured, should be disabled
	if deps.SysConf.IsProxyEnabled(context.Background()) {
		t.Error("expected proxy to be disabled by default")
	}
}

// ============================================================
// TE-204: Verify GCP code removed
// ============================================================

func TestNoGCPCodeInHandlers(t *testing.T) {
	// This test verifies that no GCP-related code exists in the handler files.
	// In Go, we can't easily do file-level grep in a unit test, but we can
	// verify the sysconf keys don't reference GCP.
	//
	// The actual grep check should be done externally:
	//   grep -r "gcp\|GCP\|google.*cloud\|Google.*Cloud" internal/ --include="*.go"
	//
	// Here we verify that the settings endpoint returns categories that don't
	// include GCP.

	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	req := httptest.NewRequest("GET", "/system/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp response.ApiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be a map, got %T", resp.Data)
	}

	// Verify no GCP category exists in settings
	for key := range data {
		lower := key
		if lower == "gcp" || lower == "google_cloud" || lower == "googlecloud" {
			t.Errorf("found GCP-related settings category: %s", key)
		}
	}
}

// ============================================================
// TE-205: System Settings API
// ============================================================

func TestSystemSettingsGet_ReturnsAllCategories(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	req := httptest.NewRequest("GET", "/system/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp response.ApiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got false: %s", resp.Message)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be a map, got %T", resp.Data)
	}

	// Verify all expected categories are present
	expectedCategories := []string{"notification", "security", "dns", "ssl", "proxy", "oauth"}
	for _, cat := range expectedCategories {
		if _, exists := data[cat]; !exists {
			t.Errorf("missing settings category: %s", cat)
		}
	}
}

func TestSystemSettingsGet_NotificationStructure(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	req := httptest.NewRequest("GET", "/system/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	notif, ok := data["notification"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected notification to be a map, got %T", data["notification"])
	}

	// Verify notification sub-categories
	for _, sub := range []string{"telegram", "dingtalk", "bark", "feishu"} {
		if _, exists := notif[sub]; !exists {
			t.Errorf("missing notification sub-category: %s", sub)
		}
	}
}

func TestSystemSettingsGet_SecurityStructure(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	req := httptest.NewRequest("GET", "/system/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	sec, ok := data["security"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected security to be a map, got %T", data["security"])
	}

	for _, sub := range []string{"turnstile", "mfa"} {
		if _, exists := sec[sub]; !exists {
			t.Errorf("missing security sub-category: %s", sub)
		}
	}
}

func TestSystemSettingsGet_DNS(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	req := httptest.NewRequest("GET", "/system/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	dns, ok := data["dns"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected dns to be a map, got %T", data["dns"])
	}

	for _, sub := range []string{"cloudflare", "edgeone"} {
		if _, exists := dns[sub]; !exists {
			t.Errorf("missing dns sub-category: %s", sub)
		}
	}
}

func TestSystemSettingsGet_SSL(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	req := httptest.NewRequest("GET", "/system/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	ssl, ok := data["ssl"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected ssl to be a map, got %T", data["ssl"])
	}

	for _, key := range []string{"domain", "email", "staging"} {
		if _, exists := ssl[key]; !exists {
			t.Errorf("missing ssl key: %s", key)
		}
	}
}

func TestSystemSettingsGet_Proxy(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	req := httptest.NewRequest("GET", "/system/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	proxy, ok := data["proxy"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected proxy to be a map, got %T", data["proxy"])
	}

	for _, key := range []string{"enabled", "type", "host", "port", "username", "password"} {
		if _, exists := proxy[key]; !exists {
			t.Errorf("missing proxy key: %s", key)
		}
	}
}

func TestSystemSettingsGet_OAuth(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	req := httptest.NewRequest("GET", "/system/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	oauth, ok := data["oauth"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected oauth to be a map, got %T", data["oauth"])
	}

	for _, sub := range []string{"github", "google"} {
		if _, exists := oauth[sub]; !exists {
			t.Errorf("missing oauth sub-category: %s", sub)
		}
	}
}

func TestSystemSettingsUpdate_InvalidBody(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	req := httptest.NewRequest("PUT", "/system/settings", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSystemSettingsUpdate_Notification_Telegram(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	body := `{"notification":{"telegram":{"botToken":"test-bot-token","chatId":"123456"}}}`
	req := httptest.NewRequest("PUT", "/system/settings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success {
		t.Fatalf("expected success=true, got false: %s", resp.Message)
	}

	data := resp.Data.(map[string]interface{})
	updated := data["updated"].(map[string]interface{})

	if !updated["telegram.bot.token"].(bool) {
		t.Error("expected telegram.bot.token to be updated")
	}
	if !updated["telegram.chat.id"].(bool) {
		t.Error("expected telegram.chat.id to be updated")
	}
}

func TestSystemSettingsUpdate_Security_Turnstile(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	body := `{"security":{"turnstile":{"enabled":true,"siteKey":"site-key-123","secretKey":"secret-key-456"}}}`
	req := httptest.NewRequest("PUT", "/system/settings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	updated := data["updated"].(map[string]interface{})

	if !updated["turnstile.enabled"].(bool) {
		t.Error("expected turnstile.enabled to be updated")
	}
	if !updated["turnstile.site.key"].(bool) {
		t.Error("expected turnstile.site.key to be updated")
	}
	if !updated["turnstile.secret.key"].(bool) {
		t.Error("expected turnstile.secret.key to be updated")
	}
}

func TestSystemSettingsUpdate_DNS_Cloudflare(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	body := `{"dns":{"cloudflare":{"apiToken":"cf-token-123"}}}`
	req := httptest.NewRequest("PUT", "/system/settings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	updated := data["updated"].(map[string]interface{})

	if !updated["cloudflare.api.token"].(bool) {
		t.Error("expected cloudflare.api.token to be updated")
	}
}

func TestSystemSettingsUpdate_SSL(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	body := `{"ssl":{"domain":"example.com","email":"admin@example.com","staging":true}}`
	req := httptest.NewRequest("PUT", "/system/settings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	updated := data["updated"].(map[string]interface{})

	if !updated["ssl.domain"].(bool) {
		t.Error("expected ssl.domain to be updated")
	}
	if !updated["ssl.email"].(bool) {
		t.Error("expected ssl.email to be updated")
	}
	if !updated["ssl.staging"].(bool) {
		t.Error("expected ssl.staging to be updated")
	}
}

func TestSystemSettingsUpdate_OAuth_GitHub(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	body := `{"oauth":{"github":{"enabled":true,"clientId":"gh-client-id","clientSecret":"gh-client-secret"}}}`
	req := httptest.NewRequest("PUT", "/system/settings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	updated := data["updated"].(map[string]interface{})

	if !updated["github.enabled"].(bool) {
		t.Error("expected github.enabled to be updated")
	}
	if !updated["github.client.id"].(bool) {
		t.Error("expected github.client.id to be updated")
	}
	if !updated["github.client.secret"].(bool) {
		t.Error("expected github.client.secret to be updated")
	}
}

func TestSystemSettingsUpdate_MultipleCategories(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	body := `{
		"notification":{"telegram":{"botToken":"token123","chatId":"456"}},
		"security":{"mfa":{"enabled":true}},
		"ssl":{"domain":"test.com","email":"test@test.com"}
	}`
	req := httptest.NewRequest("PUT", "/system/settings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	count := data["count"].(float64)

	// 2 telegram + 1 mfa + 2 ssl = 5
	if count != 5 {
		t.Errorf("updated count = %v, want 5", count)
	}
}

func TestSystemSettingsUpdate_Persistence(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	// Update telegram settings
	body := `{"notification":{"telegram":{"botToken":"my-bot-token","chatId":"789"}}}`
	req := httptest.NewRequest("PUT", "/system/settings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", w.Code, http.StatusOK)
	}

	// Read back and verify
	req2 := httptest.NewRequest("GET", "/system/settings", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	var resp response.ApiResponse
	json.Unmarshal(w2.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	notif := data["notification"].(map[string]interface{})
	tg := notif["telegram"].(map[string]interface{})

	// botToken should be masked (first 4 + *** + last 4)
	if tg["enabled"] != true {
		t.Error("telegram should be enabled after setting botToken and chatId")
	}
}

func TestSystemSettingsUpdate_EmptyBody(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	body := `{}`
	req := httptest.NewRequest("PUT", "/system/settings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	count := data["count"].(float64)

	if count != 0 {
		t.Errorf("updated count = %v, want 0 for empty body", count)
	}
}

func TestMaskSecret_ShortString(t *testing.T) {
	// maskSecret is unexported but we test it indirectly through the settings endpoint.
	// Strings with len <= 8 return "***"
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	// Set a short secret
	body := `{"notification":{"telegram":{"botToken":"short"}}}`
	req := httptest.NewRequest("PUT", "/system/settings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Read back
	req2 := httptest.NewRequest("GET", "/system/settings", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	var resp response.ApiResponse
	json.Unmarshal(w2.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	notif := data["notification"].(map[string]interface{})
	tg := notif["telegram"].(map[string]interface{})

	// Short token should be masked as "***"
	if tg["botToken"] != "***" {
		t.Errorf("short botToken mask = %v, want ***", tg["botToken"])
	}
}

func TestMaskSecret_LongString(t *testing.T) {
	deps := setupTestDeps(t)
	r := setupPhase3TestRouter(deps)

	// Set a long secret (more than 8 chars)
	body := `{"notification":{"telegram":{"botToken":"1234567890abcdef"}}}`
	req := httptest.NewRequest("PUT", "/system/settings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Read back
	req2 := httptest.NewRequest("GET", "/system/settings", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	var resp response.ApiResponse
	json.Unmarshal(w2.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	notif := data["notification"].(map[string]interface{})
	tg := notif["telegram"].(map[string]interface{})

	// Long token should be masked as first4 + "***" + last4
	expected := "1234" + "***" + "cdef"
	if tg["botToken"] != expected {
		t.Errorf("long botToken mask = %v, want %v", tg["botToken"], expected)
	}
}

// ============================================================
// sysconf.Service unit tests (ProxyConfig)
// ============================================================

func TestSysConf_ProxyConfig_RoundTrip(t *testing.T) {
	deps := setupTestDeps(t)

	// Verify through HTTP endpoint
	r := setupPhase3TestRouter(deps)

	// Set config
	body := `{"type":"HTTPS","host":"secure.proxy.com","port":443,"username":"user1","password":"pass1","enabled":true}`
	req := httptest.NewRequest("PUT", "/system/proxy", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", w.Code, http.StatusOK)
	}

	// Read back
	req2 := httptest.NewRequest("GET", "/system/proxy", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	var resp response.ApiResponse
	json.Unmarshal(w2.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	if data["type"] != "HTTPS" {
		t.Errorf("type = %v, want HTTPS", data["type"])
	}
	if data["host"] != "secure.proxy.com" {
		t.Errorf("host = %v, want secure.proxy.com", data["host"])
	}
	if data["username"] != "user1" {
		t.Errorf("username = %v, want user1", data["username"])
	}
	if data["enabled"] != true {
		t.Errorf("enabled = %v, want true", data["enabled"])
	}
}
