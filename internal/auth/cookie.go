package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	CookieName = "satoken"
	cookieMax  = 2592000 // 30d, parity with sa-token.timeout
)

// SetSessionCookie sets the satoken HttpOnly cookie (30d, SameSite=Lax).
func SetSessionCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   cookieMax,
	})
}

// ClearSessionCookie expires the satoken cookie.
func ClearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// TokenFromRequest reads the satoken cookie, falling back to Authorization:
// Bearer (for future OpenAPI token use).
func TokenFromRequest(c *gin.Context) string {
	if ck, err := c.Request.Cookie(CookieName); err == nil && ck.Value != "" {
		return ck.Value
	}
	h := c.GetHeader("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}
