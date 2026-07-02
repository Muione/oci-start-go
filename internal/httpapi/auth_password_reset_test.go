package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/sysconf"
)

// setupResetTestDeps opens an in-memory SQLite store with the login_user +
// system_config tables and one seeded user, returning wired Deps.
func setupResetTestDeps(t *testing.T) *Deps {
	t.Helper()
	store, err := db.Open("file::memory:?cache=shared", 1, 1)
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS login_user (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			is_first_user INTEGER,
			login_type TEXT NOT NULL,
			external_id TEXT,
			last_login_at TEXT,
			role TEXT NOT NULL DEFAULT 'USER'
		)`,
		`CREATE TABLE IF NOT EXISTS system_config (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			config_key TEXT UNIQUE,
			config_value TEXT,
			config_enabled INTEGER,
			last_modified TEXT
		)`,
	} {
		if _, err := store.Write.Exec(ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	if _, err := store.Write.Exec(
		`INSERT INTO login_user (username, password, is_first_user, login_type, role) VALUES (?, ?, 1, 'PASSWORD', 'USER')`,
		"resetuser", "hashed-secret",
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return &Deps{Store: store, SysConf: sysconf.New(store)}
}

// S3: the reset code must NOT be echoed back in the response JSON.
func TestSendResetCode_NoCodeEchoed(t *testing.T) {
	deps := setupResetTestDeps(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/send-reset-code", sendResetCode(deps))

	body := bytes.NewBufferString(`{"username":"resetuser"}`)
	req := httptest.NewRequest("POST", "/api/send-reset-code", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Code    *string `json:"code"`
			Message string  `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.Code != nil {
		t.Errorf("response data must not contain a 'code' field; got: %s", w.Body.String())
	}
	if resp.Data.Message == "" {
		t.Errorf("expected a non-empty generic message, got: %s", w.Body.String())
	}
}

// E9: when persisting the reset code fails, the handler must return 500 instead
// of silently reporting success.
func TestSendResetCode_PersistFails_500(t *testing.T) {
	deps := setupResetTestDeps(t)
	// Break the write pool so UpsertConfigValue fails. Reads (login_user lookup)
	// still go through the read pool, which stays open.
	if err := deps.Store.Write.Close(); err != nil {
		t.Fatalf("close write pool: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/send-reset-code", sendResetCode(deps))

	body := bytes.NewBufferString(`{"username":"resetuser"}`)
	req := httptest.NewRequest("POST", "/api/send-reset-code", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (persist failure must surface as 500)", w.Code, http.StatusInternalServerError)
	}
}

// E8reset: when the CSPRNG read fails, the handler must return 500 instead of
// emitting a zero code.
func TestSendResetCode_RandFails_500(t *testing.T) {
	deps := setupResetTestDeps(t)

	orig := randRead
	defer func() { randRead = orig }()
	randRead = func(b []byte) (int, error) { return 0, errors.New("csprng unavailable") }

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/send-reset-code", sendResetCode(deps))

	body := bytes.NewBufferString(`{"username":"resetuser"}`)
	req := httptest.NewRequest("POST", "/api/send-reset-code", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (rand failure must surface as 500)", w.Code, http.StatusInternalServerError)
	}
}