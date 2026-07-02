package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/acme"
	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/sysconf"
)

// setupSslTestDeps opens an in-memory store with system_config, seeds a
// Cloudflare API token, and wires Deps with a real (unused) CertManager.
func setupSslTestDeps(t *testing.T) *Deps {
	t.Helper()
	store, err := db.Open("file::memory:?cache=shared", 1, 1)
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if _, err := store.Write.Exec(`
		CREATE TABLE IF NOT EXISTS system_config (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			config_key TEXT UNIQUE,
			config_value TEXT,
			config_enabled INTEGER,
			last_modified TEXT
		)`); err != nil {
		t.Fatalf("create system_config: %v", err)
	}
	sc := sysconf.New(store)
	if err := sc.SetString(context.Background(), "cloudflare.api.token", "cf-token-123"); err != nil {
		t.Fatalf("seed cf token: %v", err)
	}
	return &Deps{Store: store, SysConf: sc, CertManager: acme.NewCertManager(zerolog.Nop()), Logger: zerolog.Nop()}
}

// E5: when ACME issuance succeeds but persisting the cert to system_config
// fails, the handler must return 500 (not report success).
func TestSslIssue_PersistFails_500(t *testing.T) {
	deps := setupSslTestDeps(t)

	// Swap the obtain seam so we don't hit the network; issuance "succeeds".
	orig := certObtain
	defer func() { certObtain = orig }()
	certObtain = func(d *Deps, ctx context.Context, domain, email, cfToken string, staging bool) (*acme.CertResult, error) {
		return &acme.CertResult{Domain: domain, Certificate: "CERT", PrivateKey: "KEY", NotAfter: "2026-12-01"}, nil
	}

	// Break writes so SetString (persist) fails. Reads (cf token lookup) still
	// go through the read pool.
	if err := deps.Store.Write.Close(); err != nil {
		t.Fatalf("close write pool: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/ssl/issue", sslIssue(deps))

	body := bytes.NewBufferString(`{"domain":"example.com","email":"admin@example.com"}`)
	req := httptest.NewRequest("POST", "/ssl/issue", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (persist failure must surface as 500)", w.Code, http.StatusInternalServerError)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("persistence")) {
		t.Errorf("response should mention persistence failure, got: %s", w.Body.String())
	}
}
