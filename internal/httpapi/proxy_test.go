package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/db"
)

// setupProxyTestDeps opens an in-memory store with the vpn_proxy_record table.
func setupProxyTestDeps(t *testing.T) *Deps {
	t.Helper()
	store, err := db.Open("file::memory:?cache=shared", 1, 1)
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if _, err := store.Write.Exec(`
		CREATE TABLE IF NOT EXISTS vpn_proxy_record (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			proxy_type TEXT NOT NULL,
			proxy_host TEXT NOT NULL,
			proxy_port INTEGER NOT NULL,
			proxy_username TEXT,
			proxy_password TEXT,
			available_status INTEGER NOT NULL,
			update_time TEXT,
			create_time TEXT
		)`); err != nil {
		t.Fatalf("create vpn_proxy_record: %v", err)
	}
	return &Deps{Store: store}
}

// E6: a non-numeric id on the update path must be rejected with 400 (parity
// with proxyDelete), not silently coerced to id=0.
func TestProxySave_NonNumericID_400(t *testing.T) {
	deps := setupProxyTestDeps(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/proxies/save", proxySave(deps))

	form := url.Values{
		"proxyHost": {"proxy.example.com"},
		"proxyPort": {"8080"},
		"proxyType": {"HTTP"},
		"id":        {"abc"},
	}
	req := httptest.NewRequest("POST", "/proxies/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (non-numeric id)", w.Code, http.StatusBadRequest)
	}
}

// E6: when the DB update fails, the handler must surface 500 instead of
// reporting success.
func TestProxySave_UpdateFails_500(t *testing.T) {
	deps := setupProxyTestDeps(t)
	// Seed a row so the update path targets a real id, then break writes.
	res, err := deps.Store.Write.Exec(
		`INSERT INTO vpn_proxy_record (proxy_type, proxy_host, proxy_port, available_status) VALUES ('HTTP','h',1,1)`)
	if err != nil {
		t.Fatalf("seed proxy: %v", err)
	}
	id, _ := res.LastInsertId()
	if err := deps.Store.Write.Close(); err != nil {
		t.Fatalf("close write pool: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/proxies/save", proxySave(deps))

	form := url.Values{
		"proxyHost": {"proxy.example.com"},
		"proxyPort": {"8080"},
		"proxyType": {"HTTP"},
		"id":        {strconv.FormatInt(id, 10)},
	}
	req := httptest.NewRequest("POST", "/proxies/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (update failure must surface as 500)", w.Code, http.StatusInternalServerError)
	}
}
