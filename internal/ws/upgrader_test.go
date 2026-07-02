// Package ws — upgrader_test.go: tests for Upgrader.CheckOrigin same-origin (S2-WS).
package ws

import (
	"net/http"
	"testing"
)

func newCheckReq(t *testing.T, origin, host string) *http.Request {
	t.Helper()
	r, err := http.NewRequest(http.MethodGet, "http://"+host+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Host = host
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	return r
}

// TestUpgrader_CheckOrigin verifies the default CheckOrigin enforces same-origin
// (CSWSH defense): same-origin and no-Origin requests pass; cross-origin is
// rejected.
func TestUpgrader_CheckOrigin(t *testing.T) {
	// Same origin (Origin host == Host).
	if !Upgrader.CheckOrigin(newCheckReq(t, "http://example.com", "example.com")) {
		t.Fatal("same-origin request rejected")
	}
	// Same origin with matching non-default port.
	if !Upgrader.CheckOrigin(newCheckReq(t, "http://example.com:8080", "example.com:8080")) {
		t.Fatal("same-origin (with port) request rejected")
	}
	// Cross origin → must be rejected.
	if Upgrader.CheckOrigin(newCheckReq(t, "http://evil.example.net", "example.com")) {
		t.Fatal("cross-origin request accepted (CSWSH risk)")
	}
	// No Origin header (non-browser client) → allowed.
	if !Upgrader.CheckOrigin(newCheckReq(t, "", "example.com")) {
		t.Fatal("request without Origin header rejected")
	}
	// Malformed Origin → rejected.
	if Upgrader.CheckOrigin(newCheckReq(t, "://bad", "example.com")) {
		t.Fatal("malformed Origin accepted")
	}
}
