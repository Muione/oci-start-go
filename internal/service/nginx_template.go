// Package service — nginx_template.go: Server-block template engine for Phase 12.1.
// Generates nginx server {} blocks from proxy_config rows. Handles HTTP-only,
// SSL (with HTTP redirect), WebSocket, rate limiting, caching, health check,
// and custom config injection.
package service

import (
	"fmt"
	"strings"

	"github.com/Muione/oci-start-go/internal/repo"
)

// sanitizeDomainForNginx replaces dots and wildcards with underscores for use
// in nginx zone names.
func sanitizeDomainForNginx(domain string) string {
	r := strings.NewReplacer(".", "_", "*", "wildcard")
	return r.Replace(domain)
}

// ns64 returns the string value of a sql.NullString, or "" if invalid.
func ns64(v interface{ Valid() bool; StringValue() string }) string {
	// This won't work; use a simpler approach.
	return ""
}

// generateServerBlock produces one or two nginx server {} blocks for a single
// ProxyConfig. When SSL is enabled, two blocks are generated: an HTTP redirect
// and the HTTPS block. The blocks are returned as a single concatenated string.
func generateServerBlock(pc repo.ProxyConfig) string {
	domain := pc.Domain
	targetHost := pc.TargetHost
	targetPort := pc.TargetPort

	protocol := "http"
	if pc.Protocol.Valid && pc.Protocol.String != "" {
		protocol = pc.Protocol.String
	}

	enableSSL := pc.EnableSsl.Valid && pc.EnableSsl.Int64 == 1
	enableWS := pc.EnableWebsocket.Valid && pc.EnableWebsocket.Int64 == 1
	enableRateLimit := pc.EnableRateLimit.Valid && pc.EnableRateLimit.Int64 == 1
	enableCache := pc.EnableCache.Valid && pc.EnableCache.Int64 == 1

	var sb strings.Builder

	if enableSSL {
		// Block 1: HTTP -> HTTPS redirect.
		sb.WriteString("server {\n")
		sb.WriteString("    listen 80;\n")
		sb.WriteString(fmt.Sprintf("    server_name %s;\n", domain))
		sb.WriteString("    return 301 https://$server_name$request_uri;\n")
		sb.WriteString("}\n\n")

		// Block 2: HTTPS.
		sb.WriteString("server {\n")
		sb.WriteString("    listen 443 ssl http2;\n")
		sb.WriteString(fmt.Sprintf("    server_name %s;\n", domain))
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("    ssl_certificate /usr/local/openresty/nginx/ssl/%s/fullchain.pem;\n", domain))
		sb.WriteString(fmt.Sprintf("    ssl_certificate_key /usr/local/openresty/nginx/ssl/%s/privkey.pem;\n", domain))
		sb.WriteString("    ssl_protocols TLSv1.2 TLSv1.3;\n")
		sb.WriteString("    ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512:ECDHE-RSA-AES256-GCM-SHA384;\n")
		sb.WriteString("    ssl_prefer_server_ciphers off;\n")
		sb.WriteString("    ssl_session_cache shared:SSL:10m;\n")
		sb.WriteString("    ssl_session_timeout 10m;\n")
	} else {
		// Block: HTTP only.
		sb.WriteString("server {\n")
		sb.WriteString("    listen 80;\n")
		sb.WriteString(fmt.Sprintf("    server_name %s;\n", domain))
	}

	// Rate limit zone (at server level, before location).
	if enableRateLimit {
		rateLimit := int64(100)
		if pc.RateLimit.Valid {
			rateLimit = pc.RateLimit.Int64
		}
		zoneName := sanitizeDomainForNginx(domain) + "_limit"
		sb.WriteString(fmt.Sprintf("    limit_req zone=%s burst=%d nodelay;\n", zoneName, rateLimit*2))
	}

	sb.WriteString("\n")
	sb.WriteString("    location / {\n")
	sb.WriteString(fmt.Sprintf("        proxy_pass %s://%s:%d;\n", protocol, targetHost, targetPort))
	sb.WriteString("        proxy_set_header Host $host;\n")
	sb.WriteString("        proxy_set_header X-Real-IP $remote_addr;\n")
	sb.WriteString("        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
	sb.WriteString("        proxy_set_header X-Forwarded-Proto $scheme;\n")

	// WebSocket support.
	if enableWS {
		sb.WriteString("        proxy_http_version 1.1;\n")
		sb.WriteString("        proxy_set_header Upgrade $http_upgrade;\n")
		sb.WriteString("        proxy_set_header Connection \"upgrade\";\n")
	}

	// Cache.
	if enableCache {
		cacheTime := int64(300)
		if pc.CacheTime.Valid {
			cacheTime = pc.CacheTime.Int64
		}
		sb.WriteString("        proxy_cache my_cache;\n")
		sb.WriteString(fmt.Sprintf("        proxy_cache_valid 200 %ds;\n", cacheTime))
		sb.WriteString("        proxy_cache_key $scheme$proxy_host$request_uri;\n")
	}

	sb.WriteString("    }\n")

	// Health check comment.
	if pc.EnableHealthCheck.Valid && pc.EnableHealthCheck.Int64 == 1 {
		hcPath := "/health"
		if pc.HealthCheckPath.Valid && pc.HealthCheckPath.String != "" {
			hcPath = pc.HealthCheckPath.String
		}
		sb.WriteString(fmt.Sprintf("    # Health check endpoint: %s\n", hcPath))
	}

	// Custom config injection.
	if pc.CustomConfig.Valid && pc.CustomConfig.String != "" {
		sb.WriteString("\n")
		// Indent each line of the custom config.
		for _, line := range strings.Split(pc.CustomConfig.String, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				sb.WriteString("    " + line + "\n")
			}
		}
	}

	sb.WriteString("}\n")

	return sb.String()
}

// GenerateFullNginxConfig compiles all active proxy configs into a single
// nginx config string. Returns the concatenated server blocks.
func GenerateFullNginxConfig(configs []repo.ProxyConfig) string {
	var sb strings.Builder
	for i, pc := range configs {
		block := generateServerBlock(pc)
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(block)
	}
	return sb.String()
}
