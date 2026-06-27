// Package ip resolves the real client IP from proxy headers, mirroring Java
// IpUtils.getClientIpAddress. See SPEC §7.5.
package ip

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// ClientIP returns the client IP. Header order: X-Forwarded-For (first IP of
// the chain), Proxy-Client-IP, WL-Proxy-Client-IP, HTTP_CLIENT_IP,
// HTTP_X_FORWARDED_FOR, X-Real-IP; falls back to RemoteAddr. "::1"/
// "0:0:0:0:0:0:0:1" normalized to "127.0.0.1".
func ClientIP(c *gin.Context) string {
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
		return normalize(ip)
	}
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return normalize(c.Request.RemoteAddr)
	}
	return normalize(host)
}

func normalize(ip string) string {
	if ip == "::1" || ip == "0:0:0:0:0:0:0:1" {
		return "127.0.0.1"
	}
	return ip
}
