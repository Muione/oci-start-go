// Package web serves the embedded SPA assets with a NoRoute fallback to
// index.html (so client-side routing works). Stub build serves a placeholder.
package web

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	distFS    fs.FS
	indexHTML []byte
)

func init() {
	if sub, err := fs.Sub(Assets, "dist"); err == nil {
		distFS = sub
		indexHTML, _ = fs.ReadFile(sub, "index.html")
	}
}

// SPAHTMLFallback serves index.html for browser-navigation requests so that
// client-side routes (e.g. /tenants/:id) work on refresh/direct-load. Without
// this, a refresh on /tenants/123 matches the API route GET /tenants/:id and
// returns JSON. Browser navigation sends Accept: text/html; API calls (axios)
// send Accept: application/json, so the two are distinguished. Asset requests
// (paths with a file extension) pass through to the NoRoute asset handler.
func SPAHTMLFallback() gin.HandlerFunc {
	if len(indexHTML) == 0 {
		return func(c *gin.Context) { c.Next() } // stub build: no SPA
	}
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}
		if !strings.Contains(c.Request.Header.Get("Accept"), "text/html") {
			c.Next()
			return
		}
		if path.Ext(c.Request.URL.Path) != "" {
			c.Next()
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
		c.Abort()
	}
}

// Register wires SPA static-asset serving on the engine's NoRoute handler.
func Register(r *gin.Engine) {
	if distFS == nil {
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
		if b, rerr := fs.ReadFile(distFS, p); rerr == nil {
			c.Data(http.StatusOK, contentType(p), b)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
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
