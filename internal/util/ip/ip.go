// Package ip resolves the real client IP from proxy headers, mirroring Java
// IpUtils.getClientIpAddress. See SPEC §7.5.
//
// S9 — IP spoofing hardening: proxy headers (X-Forwarded-For et al.) are
// honored ONLY when the direct TCP peer is a trusted proxy. Loopback is
// always trusted (the server talking to itself is a trusted path); additional
// proxies may be declared via the OCI_START_TRUSTED_PROXIES env var
// (comma-separated IPs or CIDRs, e.g. "10.0.0.5,10.0.0.0/24"). When the env
// var is unset, only loopback peers' XFF is trusted — so a directly-exposed
// listener ignores spoofable headers and reports RemoteAddr.
package ip

import (
	"net"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// ClientIP returns the client IP. When the direct peer is a trusted proxy,
// the first non-empty/non-"unknown" value among the proxy headers is used
// (X-Forwarded-For first IP of the chain, then Proxy-Client-IP,
// WL-Proxy-Client-IP, HTTP_CLIENT_IP, HTTP_X_FORWARDED_FOR, X-Real-IP). For
// non-trusted peers, proxy headers are ignored and RemoteAddr is used.
// "::1"/"0:0:0:0:0:0:0:1" normalized to "127.0.0.1".
func ClientIP(c *gin.Context) string {
	remoteHost := remoteHostOf(c.Request.RemoteAddr)
	if isTrustedProxy(remoteHost) {
		if ip := fromProxyHeaders(c); ip != "" {
			return normalize(ip)
		}
	}
	return normalize(remoteHost)
}

// remoteHostOf extracts the host portion of an addr that may carry a port.
// If SplitHostPort fails (no port, malformed), the input is returned as-is
// so normalize() can still fold loopback text forms.
func remoteHostOf(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

// fromProxyHeaders returns the first usable client IP from the proxy header
// set, or "" if none are present/usable.
func fromProxyHeaders(c *gin.Context) string {
	for _, h := range []string{
		"X-Forwarded-For",
		"Proxy-Client-IP",
		"WL-Proxy-Client-IP",
		"HTTP_CLIENT_IP",
		"HTTP_X_FORWARDED_FOR",
		"X-Real-IP",
	} {
		v := c.GetHeader(h)
		if v == "" || strings.EqualFold(v, "unknown") {
			continue
		}
		ip := strings.TrimSpace(v)
		if i := strings.Index(ip, ","); i >= 0 {
			ip = strings.TrimSpace(ip[:i]) // first IP of X-Forwarded-For chain
		}
		return ip
	}
	return ""
}

// isTrustedProxy reports whether the direct peer is allowed to set trusted
// proxy headers. Loopback is always trusted (stdlib check); the env-declared
// list adds more. ponytail: env re-parsed per call — admin-panel traffic makes
// a sync.Once cache unnecessary; switch to one if a hot path emerges.
func isTrustedProxy(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	for _, n := range trustedProxyNets() {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// trustedProxyNets parses OCI_START_TRUSTED_PROXIES into CIDR nets. Bare IPs
// are widened to /32 (v4) or /128 (v6). Empty/unset env → nil (loopback is
// handled separately by net.IP.IsLoopback).
func trustedProxyNets() []*net.IPNet {
	raw := os.Getenv("OCI_START_TRUSTED_PROXIES")
	if raw == "" {
		return nil
	}
	var nets []*net.IPNet
	for _, c := range strings.Split(raw, ",") {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if !strings.Contains(c, "/") {
			if ip := net.ParseIP(c); ip != nil {
				if ip.To4() != nil {
					c += "/32"
				} else {
					c += "/128"
				}
			} else {
				continue
			}
		}
		if _, n, err := net.ParseCIDR(c); err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}

func normalize(ip string) string {
	if ip == "::1" || ip == "0:0:0:0:0:0:0:1" {
		return "127.0.0.1"
	}
	return ip
}
