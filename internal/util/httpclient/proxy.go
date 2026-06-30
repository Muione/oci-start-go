// Package httpclient provides proxy-aware HTTP client creation.
// When a system proxy is configured and enabled, the returned client will
// route outbound traffic through the specified proxy.
package httpclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"

	"github.com/Muione/oci-start-go/internal/sysconf"
)

// NewClient creates an *http.Client that routes through the system proxy
// if configured and enabled. Returns a standard client otherwise.
func NewClient(sysConf *sysconf.Service, timeout time.Duration) *http.Client {
	return NewClientWithContext(context.Background(), sysConf, timeout)
}

// NewClientWithContext creates an *http.Client with context for proxy config lookup.
func NewClientWithContext(ctx context.Context, sysConf *sysconf.Service, timeout time.Duration) *http.Client {
	if sysConf == nil || !sysConf.IsProxyEnabled(ctx) {
		return &http.Client{Timeout: timeout}
	}

	cfg := sysConf.GetProxyConfig(ctx)
	transport := newProxyTransport(cfg)
	if transport == nil {
		return &http.Client{Timeout: timeout}
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// newProxyTransport creates an http.Transport configured with the given proxy.
func newProxyTransport(cfg sysconf.ProxyConfig) *http.Transport {
	switch cfg.Type {
	case "HTTP", "HTTPS":
		return newHTTPProxyTransport(cfg)
	case "SOCKS5":
		return newSOCKS5ProxyTransport(cfg)
	default:
		return nil
	}
}

// newHTTPProxyTransport creates a transport for HTTP/HTTPS proxies.
func newHTTPProxyTransport(cfg sysconf.ProxyConfig) *http.Transport {
	proxyURL := &url.URL{
		Scheme: cfg.Type,
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
	}
	if cfg.Username != "" {
		if cfg.Password != "" {
			proxyURL.User = url.UserPassword(cfg.Username, cfg.Password)
		} else {
			proxyURL.User = url.User(cfg.Username)
		}
	}

	return &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout:  30 * time.Second,
		ExpectContinueTimeout:  1 * time.Second,
		MaxIdleConns:           100,
		MaxIdleConnsPerHost:    10,
		IdleConnTimeout:        90 * time.Second,
	}
}

// newSOCKS5ProxyTransport creates a transport for SOCKS5 proxies.
func newSOCKS5ProxyTransport(cfg sysconf.ProxyConfig) *http.Transport {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	var auth *proxy.Auth
	if cfg.Username != "" {
		auth = &proxy.Auth{
			User:     cfg.Username,
			Password: cfg.Password,
		}
	}

	dialer, err := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
	if err != nil {
		return nil
	}

	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.(proxy.ContextDialer).DialContext(ctx, network, addr)
		},
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout:  30 * time.Second,
		ExpectContinueTimeout:  1 * time.Second,
		MaxIdleConns:           100,
		MaxIdleConnsPerHost:    10,
		IdleConnTimeout:        90 * time.Second,
	}
}
