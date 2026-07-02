package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Muione/oci-start-go/internal/auth"
	"github.com/Muione/oci-start-go/internal/db"
)

// S2-server: the WebSocket endpoints must be gated by SessionAuth. A request
// to /ws/ssh with no session token must be rejected with 401 (not upgraded,
// not handed to the WS handler).
func TestNewServer_WSRouteRequiresAuth(t *testing.T) {
	store, err := db.Open("file::memory:?cache=shared", 1, 1)
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	// IpBan (applied to all routes) reads ban_record; create it so the ban
	// check passes and SessionAuth gets to run.
	if _, err := store.Write.Exec(`
		CREATE TABLE IF NOT EXISTS ban_record (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ip_address TEXT NOT NULL,
			source TEXT,
			operator_name TEXT,
			reason TEXT,
			status INTEGER NOT NULL,
			create_time TEXT NOT NULL,
			unban_time TEXT,
			remark TEXT
		)`); err != nil {
		t.Fatalf("create ban_record: %v", err)
	}

	// WsHub is intentionally nil: SessionAuth must reject before the WS
	// handler closure ever runs.
	deps := &Deps{Store: store, Session: auth.NewSessionService(store)}
	r := NewServer(deps)

	req := httptest.NewRequest("GET", "/ws/ssh", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (WS route must require auth)", w.Code, http.StatusUnauthorized)
	}
}
