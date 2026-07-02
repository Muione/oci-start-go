package ip

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.ReleaseMode) // silence gin debug banner in test output
	os.Exit(m.Run())
}

func newCtx(t *testing.T, remoteAddr string, headers map[string]string) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c.Request = req
	return c
}

func TestClientIP_NonTrustedRemoteIgnoresXFF(t *testing.T) {
	// A public, non-loopback peer must not have its X-Forwarded-For trusted.
	// Spoofed XFF from an external client should be discarded in favour of
	// the direct RemoteAddr.
	c := newCtx(t, "203.0.113.7:54321", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
		"X-Real-IP":       "5.6.7.8",
	})
	got := ClientIP(c)
	if got != "203.0.113.7" {
		t.Errorf("got %q, want %q (XFF must be ignored for non-trusted peer)", got, "203.0.113.7")
	}
}

func TestClientIP_LoopbackRemoteTrustsXFF(t *testing.T) {
	c := newCtx(t, "127.0.0.1:54321", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
	})
	if got := ClientIP(c); got != "1.2.3.4" {
		t.Errorf("got %q, want %q (loopback is a trusted proxy)", got, "1.2.3.4")
	}
}

func TestClientIP_IPv6LoopbackTrustsXFF(t *testing.T) {
	c := newCtx(t, "[::1]:54321", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
	})
	if got := ClientIP(c); got != "1.2.3.4" {
		t.Errorf("got %q, want %q (::1 is a trusted proxy)", got, "1.2.3.4")
	}
}

func TestClientIP_LoopbackNoHeadersFallsBackToRemoteAddr(t *testing.T) {
	c := newCtx(t, "127.0.0.1:54321", nil)
	if got := ClientIP(c); got != "127.0.0.1" {
		t.Errorf("got %q, want %q", got, "127.0.0.1")
	}
}

func TestClientIP_NonTrustedNoHeadersReturnsRemote(t *testing.T) {
	c := newCtx(t, "198.51.100.20:1234", nil)
	if got := ClientIP(c); got != "198.51.100.20" {
		t.Errorf("got %q, want %q", got, "198.51.100.20")
	}
}

func TestClientIP_NonTrustedIgnoresAllProxyHeaders(t *testing.T) {
	c := newCtx(t, "203.0.113.7:54321", map[string]string{
		"X-Forwarded-For":      "1.2.3.4",
		"Proxy-Client-IP":      "9.9.9.9",
		"WL-Proxy-Client-IP":   "8.8.8.8",
		"HTTP_CLIENT_IP":       "7.7.7.7",
		"HTTP_X_FORWARDED_FOR": "6.6.6.6",
		"X-Real-IP":            "5.6.7.8",
	})
	if got := ClientIP(c); got != "203.0.113.7" {
		t.Errorf("got %q, want %q (all proxy headers ignored for non-trusted peer)", got, "203.0.113.7")
	}
}

func TestClientIP_TrustedProxyFromEnv(t *testing.T) {
	// An operator-declared trusted proxy (non-loopback) should have its XFF
	// honored.
	t.Setenv("OCI_START_TRUSTED_PROXIES", "10.0.0.5")
	c := newCtx(t, "10.0.0.5:54321", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
	})
	if got := ClientIP(c); got != "1.2.3.4" {
		t.Errorf("got %q, want %q (env-trusted proxy)", got, "1.2.3.4")
	}
}

func TestClientIP_TrustedProxyCIDRFromEnv(t *testing.T) {
	t.Setenv("OCI_START_TRUSTED_PROXIES", "10.0.0.0/24")
	c := newCtx(t, "10.0.0.42:54321", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
	})
	if got := ClientIP(c); got != "1.2.3.4" {
		t.Errorf("got %q, want %q (CIDR-trusted proxy)", got, "1.2.3.4")
	}
}

func TestClientIP_NonTrustedEvenWithEnvSet(t *testing.T) {
	// Env trusts 10.0.0.5 only; a different IP must still be untrusted.
	t.Setenv("OCI_START_TRUSTED_PROXIES", "10.0.0.5")
	c := newCtx(t, "203.0.113.7:54321", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
	})
	if got := ClientIP(c); got != "203.0.113.7" {
		t.Errorf("got %q, want %q (not in env trust list)", got, "203.0.113.7")
	}
}

func TestClientIP_XFFChainPicksFirst(t *testing.T) {
	c := newCtx(t, "127.0.0.1:54321", map[string]string{
		"X-Forwarded-For": "1.2.3.4, 10.0.0.1, 10.0.0.2",
	})
	if got := ClientIP(c); got != "1.2.3.4" {
		t.Errorf("got %q, want first hop %q", got, "1.2.3.4")
	}
}

func TestClientIP_UnknownHeaderValueSkipped(t *testing.T) {
	// "unknown" XFF is skipped, falling back to the next header / remote.
	c := newCtx(t, "127.0.0.1:54321", map[string]string{
		"X-Forwarded-For": "unknown",
		"X-Real-IP":       "5.6.7.8",
	})
	if got := ClientIP(c); got != "5.6.7.8" {
		t.Errorf("got %q, want %q", got, "5.6.7.8")
	}
}
