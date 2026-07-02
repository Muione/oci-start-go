package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/Muione/oci-start-go/internal/repo"
)

func init() { gin.SetMode(gin.TestMode) }

// B5: SessionAuth rejects with 401 when no token is present.
func TestSessionAuth_NoToken_401(t *testing.T) {
	store := newAuthStore(t)
	svc := NewSessionService(store)
	r := gin.New()
	r.GET("/x", SessionAuth(svc), func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("no-token status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// B5: SessionAuth rejects an invalid (unknown) token with 401.
func TestSessionAuth_InvalidToken_401(t *testing.T) {
	store := newAuthStore(t)
	svc := NewSessionService(store)
	r := gin.New()
	r.GET("/x", SessionAuth(svc), func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "not-a-real-token"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("invalid-token status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// B5: SessionAuth validates the token and injects the username into context.
func TestSessionAuth_ValidToken_InjectsUsername(t *testing.T) {
	store := newAuthStore(t)
	svc := NewSessionService(store)
	ctx := context.Background()
	tok, err := svc.Create(ctx, "alice", "1.2.3.4", "ua")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	var gotUser string
	var gotOK bool
	r := gin.New()
	r.GET("/x", SessionAuth(svc), func(c *gin.Context) {
		gotUser, gotOK = UsernameFromContext(c.Request.Context())
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: tok})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("valid-token status = %d, want %d", w.Code, http.StatusOK)
	}
	if !gotOK || gotUser != "alice" {
		t.Errorf("username in context = (%q,%v), want (\"alice\",true)", gotUser, gotOK)
	}
}

// B5: SessionAuth throttles last_active_at writes — a session idle <60s is
// NOT touched (last_active unchanged), one idle >60s IS touched.
func TestSessionAuth_TouchThrottle(t *testing.T) {
	store := newAuthStore(t)
	svc := NewSessionService(store)
	ctx := context.Background()

	// idle 10s: within touchInterval (60s) → must NOT be touched
	const tokFresh = "tok-fresh-10s"
	insertSessionRow(t, store, tokFresh, "u1", futureStr(30*24*time.Hour), pastStr(10*time.Second))
	// idle 2m: beyond touchInterval → MUST be touched (last_active → ~now)
	const tokIdle = "tok-idle-2m"
	insertSessionRow(t, store, tokIdle, "u2", futureStr(30*24*time.Hour), pastStr(2*time.Minute))

	r := gin.New()
	r.GET("/x", SessionAuth(svc), func(c *gin.Context) { c.Status(200) })

	for _, tok := range []string{tokFresh, tokIdle} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.AddCookie(&http.Cookie{Name: CookieName, Value: tok})
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("token %s: status = %d, want 200", tok, w.Code)
		}
	}

	// tokFresh last_active should still be ~10s ago (untouched)
	_, lastFresh, ok := svc.Validate(ctx, tokFresh)
	if !ok {
		t.Fatal("tokFresh should still validate")
	}
	if d := time.Since(lastFresh); d < 5*time.Second {
		t.Errorf("tokFresh was touched (last_active ~now, off %v); throttle should have skipped it", d)
	}
	// tokIdle last_active should be ~now (touched)
	_, lastIdle, ok := svc.Validate(ctx, tokIdle)
	if !ok {
		t.Fatal("tokIdle should still validate")
	}
	if d := time.Since(lastIdle); d > 5*time.Second {
		t.Errorf("tokIdle not touched (last_active off by %v); should have been touched", d)
	}
}

// B5: IpBan returns 403 when a ban_record with status=1 exists for the
// client IP. The banned client connects directly (RemoteAddr); iputil only
// trusts proxy headers from trusted proxies, so we model a direct peer.
func TestIpBan_BannedIP_403(t *testing.T) {
	store := newAuthStore(t)
	const ip = "203.0.113.7"
	if _, err := store.Write.ExecContext(context.Background(),
		`INSERT INTO ban_record (ip_address, status, create_time) VALUES (?, 1, ?)`,
		ip, nowStr()); err != nil {
		t.Fatalf("insert ban: %v", err)
	}
	r := gin.New()
	r.Use(IpBan(store))
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = ip + ":54321"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("banned IP status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

// B5: IpBan passes through when the resolved ban has status=0 (unbanned).
func TestIpBan_NotBanned_Passes(t *testing.T) {
	store := newAuthStore(t)
	const ip = "203.0.113.8"
	if _, err := store.Write.ExecContext(context.Background(),
		`INSERT INTO ban_record (ip_address, status, create_time) VALUES (?, 0, ?)`,
		ip, nowStr()); err != nil {
		t.Fatalf("insert ban: %v", err)
	}
	r := gin.New()
	r.Use(IpBan(store))
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = ip + ":54321"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("unbanned IP status = %d, want %d", w.Code, http.StatusOK)
	}
}

// B5: IpBan rejects with 403 when the client IP cannot be resolved.
func TestIpBan_EmptyIP_403(t *testing.T) {
	store := newAuthStore(t)
	r := gin.New()
	r.Use(IpBan(store))
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	// No headers and an empty RemoteAddr → ClientIP returns "" → 403.
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = ""
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("empty-IP status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

// B5: TenantContext reads X-Tenant-Id, parses it, loads the Tenant row, and
// stores both the id and the Tenant in context.
func TestTenantContext_ReadsXTenantId(t *testing.T) {
	store := newAuthStore(t)
	const id int64 = 42
	if _, err := store.Write.ExecContext(context.Background(),
		`INSERT INTO tenant (id) VALUES (?)`, id); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}

	var gotID int64
	var gotIDOK bool
	var gotTenant repo.Tenant
	var gotTenantOK bool
	r := gin.New()
	r.GET("/x", TenantContext(store), func(c *gin.Context) {
		gotID, gotIDOK = TenantIDFromContext(c.Request.Context())
		gotTenant, gotTenantOK = TenantFromContext(c.Request.Context())
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Tenant-Id", "42")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !gotIDOK || gotID != id {
		t.Errorf("tenant id in context = (%d,%v), want (%d,true)", gotID, gotIDOK, id)
	}
	if !gotTenantOK || gotTenant.ID != id {
		t.Errorf("tenant row in context = (id=%d, ok=%v), want (id=%d,true)", gotTenant.ID, gotTenantOK, id)
	}
}

// B5: TenantContext is non-fatal on a missing/invalid X-Tenant-Id (passes
// through without setting a tenant).
func TestTenantContext_MissingHeader_PassesThrough(t *testing.T) {
	store := newAuthStore(t)
	called := false
	r := gin.New()
	r.GET("/x", TenantContext(store), func(c *gin.Context) { called = true; c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !called {
		t.Error("next handler not called for missing X-Tenant-Id")
	}
}
