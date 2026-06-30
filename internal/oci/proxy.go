// Package oci — proxy.go ports the Java SOCKS proxy pool (@UseSocksProxy +
// ProxyAspect + SocksProxyUtils). A ProxyPool picks a random available
// VpnProxyRecord, health-checks it (TCP/HTTPS reachability to oracle.com:443),
// and builds an *http.Transport whose DialContext/Proxy routes through it. The
// transport is injected into OCI clients by overriding each client's embedded
// BaseClient.HTTPClient (plan D2). WithProxy is the method-level decorator
// replacement for the Java AOP aspect: pick → health-check → build proxied
// clients → run fn; no proxy or unhealthy proxy falls back to direct.
package oci

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/oracle/oci-go-sdk/v65/common"
	"golang.org/x/net/proxy"
)

const (
	proxyTestURL   = "https://www.oracle.com"
	proxyTimeout   = 3 * time.Second // parity with SocksProxyUtils.TIMEOUT_MS
)

// ProxyPool selects a random enabled VpnProxyRecord from the DB.
type ProxyPool struct {
	q *repo.Queries
}

func NewProxyPool(q *repo.Queries) *ProxyPool { return &ProxyPool{q: q} }

// Pick returns a random available proxy, or (nil, nil) when none is configured
// (caller falls back to direct). Parity with findRandomAvailableRecord.
func (p *ProxyPool) Pick(ctx context.Context) (*repo.VpnProxyRecord, error) {
	r, err := p.q.FindRandomAvailableProxy(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

// Available health-checks the proxy by issuing a short HTTPS GET through it.
// Parity with SocksProxyUtils.isProxyAvailable (connect to oracle.com:443).
func (p *ProxyPool) Available(rec *repo.VpnProxyRecord) bool {
	if rec == nil || rec.ProxyHost == "" || rec.ProxyPort <= 0 {
		return false
	}
	tr, err := transportFor(rec)
	if err != nil {
		return false
	}
	c := &http.Client{Transport: tr, Timeout: proxyTimeout}
	resp, err := c.Get(proxyTestURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}

// transportFor builds an *http.Transport routed through the proxy. HTTP/HTTPS
// proxies use http.ProxyURL (auth from URL userinfo); SOCKS5 (default) uses
// golang.org/x/net/proxy.SOCKS5 so username/password auth is honoured.
func transportFor(rec *repo.VpnProxyRecord) (*http.Transport, error) {
	host := rec.ProxyHost
	port := rec.ProxyPort
	addr := net.JoinHostPort(host, strconv.FormatInt(port, 10))
	typ := strings.ToUpper(strings.TrimSpace(rec.ProxyType))

	hasAuth := rec.ProxyUsername.Valid && rec.ProxyUsername.String != "" &&
		rec.ProxyPassword.Valid

	switch typ {
	case "HTTP", "HTTPS":
		u := &url.URL{Scheme: strings.ToLower(typ), Host: addr}
		if hasAuth {
			u.User = url.UserPassword(rec.ProxyUsername.String, rec.ProxyPassword.String)
		}
		return &http.Transport{Proxy: http.ProxyURL(u)}, nil
	default: // SOCKS5
		var auth *proxy.Auth
		if hasAuth {
			auth = &proxy.Auth{User: rec.ProxyUsername.String, Password: rec.ProxyPassword.String}
		}
		dialer, err := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
		if err != nil {
			return nil, err
		}
		return &http.Transport{
			DialContext: func(ctx context.Context, network, a string) (net.Conn, error) {
				return dialer.Dial(network, a)
			},
		}, nil
	}
}

// NewClientsWithHTTPClient builds Clients (via NewClients) then overrides each
// client's embedded BaseClient.HTTPClient with hc, routing OCI calls through a
// proxy transport. *http.Client satisfies common.HTTPRequestDispatcher.
func NewClientsWithHTTPClient(prov common.ConfigurationProvider, hc *http.Client) (Clients, error) {
	c, err := NewClients(prov)
	if err != nil {
		return Clients{}, err
	}
	c.Compute.HTTPClient = hc
	c.Vcn.HTTPClient = hc
	c.Identity.HTTPClient = hc
	c.ObjectStorage.HTTPClient = hc
	c.Blockstorage.HTTPClient = hc
	c.Limits.HTTPClient = hc
	c.Audit.HTTPClient = hc
	c.NLB.HTTPClient = hc
	c.Email.HTTPClient = hc
	c.OspGateway.HTTPClient = hc
	c.UsageApi.HTTPClient = hc
	return c, nil
}

// WithProxy is the @UseSocksProxy decorator replacement. It picks a healthy
// proxy and runs fn with proxy-routed clients; if no proxy is configured or the
// picked proxy is unhealthy, it falls back to direct clients. fn receives the
// constructed Clients; the transport is released when fn returns (no ThreadLocal
// to clear, unlike Java ProxyContext).
func WithProxy(ctx context.Context, pool *ProxyPool, creds Credentials, masterKey []byte, fn func(Clients) error) error {
	prov, err := NewProvider(creds, masterKey)
	if err != nil {
		return err
	}
	rec, err := pool.Pick(ctx)
	if err != nil {
		return err
	}
	if rec != nil && pool.Available(rec) {
		tr, err := transportFor(rec)
		if err == nil {
			hc := &http.Client{Transport: tr}
			clients, err := NewClientsWithHTTPClient(prov, hc)
			if err != nil {
				return err
			}
			return fn(clients)
		}
		// transport build failed → fall through to direct
	}
	clients, err := NewClients(prov)
	if err != nil {
		return err
	}
	return fn(clients)
}
