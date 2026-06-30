package httpclient

import (
	"net/http"
	"testing"
	"time"

	"github.com/Muione/oci-start-go/internal/sysconf"
)

// ============================================================
// TE-203: Proxy-aware HTTP client creation
// ============================================================

func TestNewClient_NilSysConf(t *testing.T) {
	client := NewClient(nil, 10*time.Second)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 10*time.Second {
		t.Errorf("timeout = %v, want 10s", client.Timeout)
	}
	// Should be a standard client with no custom transport
	if client.Transport != nil {
		t.Error("expected nil transport for nil sysconf")
	}
}

func TestNewClientWithContext_NilSysConf(t *testing.T) {
	client := NewClientWithContext(nil, nil, 10*time.Second)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 10*time.Second {
		t.Errorf("timeout = %v, want 10s", client.Timeout)
	}
}

func TestNewProxyTransport_UnknownType(t *testing.T) {
	cfg := sysconf.ProxyConfig{
		Type: "UNKNOWN",
		Host: "proxy.example.com",
		Port: 8080,
	}
	transport := newProxyTransport(cfg)
	if transport != nil {
		t.Error("expected nil transport for unknown proxy type")
	}
}

func TestNewProxyTransport_HTTPType(t *testing.T) {
	cfg := sysconf.ProxyConfig{
		Type: "HTTP",
		Host: "proxy.example.com",
		Port: 8080,
	}
	transport := newProxyTransport(cfg)
	if transport == nil {
		t.Fatal("expected non-nil transport for HTTP type")
	}
	if transport.Proxy == nil {
		t.Error("expected Proxy function to be set for HTTP transport")
	}
	if transport.TLSHandshakeTimeout != 15*time.Second {
		t.Errorf("TLSHandshakeTimeout = %v, want 15s", transport.TLSHandshakeTimeout)
	}
	if transport.MaxIdleConns != 100 {
		t.Errorf("MaxIdleConns = %v, want 100", transport.MaxIdleConns)
	}
}

func TestNewProxyTransport_HTTPSType(t *testing.T) {
	cfg := sysconf.ProxyConfig{
		Type: "HTTPS",
		Host: "proxy.example.com",
		Port: 443,
	}
	transport := newProxyTransport(cfg)
	if transport == nil {
		t.Fatal("expected non-nil transport for HTTPS type")
	}
	if transport.Proxy == nil {
		t.Error("expected Proxy function to be set for HTTPS transport")
	}
}

func TestNewProxyTransport_HTTPWithAuth(t *testing.T) {
	cfg := sysconf.ProxyConfig{
		Type:     "HTTP",
		Host:     "proxy.example.com",
		Port:     8080,
		Username: "user",
		Password: "pass",
	}
	transport := newProxyTransport(cfg)
	if transport == nil {
		t.Fatal("expected non-nil transport for HTTP with auth")
	}
	if transport.Proxy == nil {
		t.Error("expected Proxy function to be set")
	}
}

func TestNewProxyTransport_HTTPWithUsernameOnly(t *testing.T) {
	cfg := sysconf.ProxyConfig{
		Type:     "HTTP",
		Host:     "proxy.example.com",
		Port:     8080,
		Username: "user",
	}
	transport := newProxyTransport(cfg)
	if transport == nil {
		t.Fatal("expected non-nil transport for HTTP with username only")
	}
	if transport.Proxy == nil {
		t.Error("expected Proxy function to be set")
	}
}

func TestNewProxyTransport_SOCKS5Type(t *testing.T) {
	cfg := sysconf.ProxyConfig{
		Type: "SOCKS5",
		Host: "127.0.0.1",
		Port: 1080,
	}
	transport := newProxyTransport(cfg)
	if transport == nil {
		t.Fatal("expected non-nil transport for SOCKS5 type")
	}
	if transport.DialContext == nil {
		t.Error("expected DialContext to be set for SOCKS5 transport")
	}
}

func TestNewProxyTransport_SOCKS5WithAuth(t *testing.T) {
	cfg := sysconf.ProxyConfig{
		Type:     "SOCKS5",
		Host:     "127.0.0.1",
		Port:     1080,
		Username: "user",
		Password: "pass",
	}
	transport := newProxyTransport(cfg)
	if transport == nil {
		t.Fatal("expected non-nil transport for SOCKS5 with auth")
	}
	if transport.DialContext == nil {
		t.Error("expected DialContext to be set for SOCKS5 with auth")
	}
}

func TestHTTPProxyTransport_Fields(t *testing.T) {
	cfg := sysconf.ProxyConfig{
		Type: "HTTP",
		Host: "proxy.example.com",
		Port: 3128,
	}
	transport := newHTTPProxyTransport(cfg)

	if transport.ResponseHeaderTimeout != 30*time.Second {
		t.Errorf("ResponseHeaderTimeout = %v, want 30s", transport.ResponseHeaderTimeout)
	}
	if transport.ExpectContinueTimeout != 1*time.Second {
		t.Errorf("ExpectContinueTimeout = %v, want 1s", transport.ExpectContinueTimeout)
	}
	if transport.MaxIdleConnsPerHost != 10 {
		t.Errorf("MaxIdleConnsPerHost = %v, want 10", transport.MaxIdleConnsPerHost)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Errorf("IdleConnTimeout = %v, want 90s", transport.IdleConnTimeout)
	}
}

func TestSOCKS5ProxyTransport_Fields(t *testing.T) {
	cfg := sysconf.ProxyConfig{
		Type: "SOCKS5",
		Host: "127.0.0.1",
		Port: 1080,
	}
	transport := newSOCKS5ProxyTransport(cfg)

	if transport.ResponseHeaderTimeout != 30*time.Second {
		t.Errorf("ResponseHeaderTimeout = %v, want 30s", transport.ResponseHeaderTimeout)
	}
	if transport.MaxIdleConns != 100 {
		t.Errorf("MaxIdleConns = %v, want 100", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 10 {
		t.Errorf("MaxIdleConnsPerHost = %v, want 10", transport.MaxIdleConnsPerHost)
	}
}

func TestNewClient_ReturnsStandardClient(t *testing.T) {
	// Verify that the returned client is a valid *http.Client
	// that can be used for standard HTTP operations.
	client := NewClient(nil, 5*time.Second)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if _, ok := interface{}(client).(*http.Client); !ok {
		t.Error("expected *http.Client type")
	}
}
