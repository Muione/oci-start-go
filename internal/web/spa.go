// Package web serves the embedded SPA assets with a NoRoute fallback to
// index.html (so client-side routing works). Stub build serves a placeholder.
package web

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Register wires SPA static-asset serving on the engine's NoRoute handler.
func Register(r *gin.Engine) {
	sub, err := fs.Sub(Assets, "dist")
	if err != nil {
		// Stub build (no dist/): serve the placeholder at /.
		r.NoRoute(func(c *gin.Context) {
			if c.Request.Method == http.MethodGet && (c.Request.URL.Path == "/" || c.Request.URL.Path == "/index.html") {
				b, _ := Assets.ReadFile("stub.html")
				c.Data(http.StatusOK, "text/html; charset=utf-8", b)
				return
			}
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "not found"})
		})
		return
	}
	r.NoRoute(func(c *gin.Context) {
		p := strings.TrimPrefix(c.Request.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if b, rerr := fs.ReadFile(sub, p); rerr == nil {
			c.Data(http.StatusOK, contentType(p), b)
			return
		}
		if idx, rerr := fs.ReadFile(sub, "index.html"); rerr == nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8", idx)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "not found"})
	})
}

func contentType(p string) string {
	switch {
	case strings.HasSuffix(p, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(p, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(p, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(p, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(p, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(p, ".png"):
		return "image/png"
	case strings.HasSuffix(p, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(p, ".woff2"):
		return "font/woff2"
	}
	return "application/octet-stream"
}
