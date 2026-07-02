package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// SetSessionCookie must mark the satoken cookie Secure (HTTPS-only) so it is
// never transmitted in cleartext over HTTP. HttpOnly + SameSite=Lax are parity
// baseline. S5.
func TestSetSessionCookie_SecureFlags(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	SetSessionCookie(c, "tok-123")

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 Set-Cookie, got %d", len(cookies))
	}
	ck := cookies[0]
	if ck.Name != CookieName {
		t.Errorf("cookie name = %q, want %q", ck.Name, CookieName)
	}
	if ck.Value != "tok-123" {
		t.Errorf("cookie value = %q, want %q", ck.Value, "tok-123")
	}
	if !ck.Secure {
		t.Error("cookie missing Secure flag (would leak over cleartext HTTP)")
	}
	if !ck.HttpOnly {
		t.Error("cookie missing HttpOnly flag")
	}
	if ck.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", ck.SameSite)
	}
	if ck.MaxAge != cookieMax {
		t.Errorf("MaxAge = %d, want %d", ck.MaxAge, cookieMax)
	}
	if ck.Path != "/" {
		t.Errorf("Path = %q, want %q", ck.Path, "/")
	}
}

// ClearSessionCookie must also carry Secure so the expiry travels the same
// path as the original cookie (otherwise some clients ignore the clear).
func TestClearSessionCookie_SecureFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	ClearSessionCookie(c)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 Set-Cookie, got %d", len(cookies))
	}
	ck := cookies[0]
	if !ck.Secure {
		t.Error("clear cookie missing Secure flag")
	}
	if ck.MaxAge >= 0 {
		t.Errorf("clear cookie MaxAge = %d, want < 0 (expire)", ck.MaxAge)
	}
}
