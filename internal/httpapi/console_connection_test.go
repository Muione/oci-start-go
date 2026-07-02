// Package httpapi — console_connection_test.go: tests the list/delete console
// connection handlers. A fake ConsoleConnSvc (satisfying
// service.ConsoleConnectionLister) is injected so no real OCI client is
// needed; the handler's job is to resolve the instance (tenant+compartment)
// from the DB, call the service, and map errors.
package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	_ "modernc.org/sqlite"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/service"
)

type fakeConsoleSvc struct {
	listOut   []service.ConsoleConnectionView
	listErr   error
	deleteErr error

	listTenant int64
	listComp   string
	listInst   string

	delTenant int64
	delComp   string
	delInst   string
	delConn   string
}

func (f *fakeConsoleSvc) List(_ context.Context, tenant int64, comp, inst string) ([]service.ConsoleConnectionView, error) {
	f.listTenant, f.listComp, f.listInst = tenant, comp, inst
	return f.listOut, f.listErr
}

func (f *fakeConsoleSvc) Delete(_ context.Context, tenant int64, comp, inst, conn string) error {
	f.delTenant, f.delComp, f.delInst, f.delConn = tenant, comp, inst, conn
	return f.deleteErr
}

func setupConsoleHandlerDeps(t *testing.T) *Deps {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	d.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.Exec(`CREATE TABLE instance_detail (
    id INTEGER PRIMARY KEY, instance_id TEXT, display_name TEXT, public_ips TEXT,
    private_ips TEXT, shape TEXT, username TEXT, port INTEGER, password TEXT,
    tenant_id BIGINT, compartment_id TEXT, availability_domain TEXT
)`); err != nil {
		t.Fatalf("create instance_detail: %v", err)
	}
	return &Deps{Store: &db.Store{Write: d, Read: d}, Logger: zerolog.Nop()}
}

func insertInstanceRow(t *testing.T, deps *Deps, instanceID string, tenantID int64, comp string) {
	t.Helper()
	_, err := deps.Store.Write.Exec(
		"INSERT INTO instance_detail (instance_id, tenant_id, compartment_id) VALUES (?, ?, ?)",
		instanceID, tenantID, comp)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
}

// TestNewServer_ConsoleRoutesNoConflict: registering the full router must not
// panic — in particular /instances/:id (int64 leaf) and
// /instances/:id/console-connections (OCID child) share the :id param node.
func TestNewServer_ConsoleRoutesNoConflict(t *testing.T) {
	store, err := db.Open("file::memory:?cache=shared", 1, 1)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	gin.SetMode(gin.TestMode)
	s := NewServer(&Deps{Store: store, Logger: zerolog.Nop()})
	if s == nil {
		t.Fatal("nil server")
	}
}

// TestInstanceConsoleConnectionsList_NotFound: unknown instance -> 404.
func TestInstanceConsoleConnectionsList_NotFound(t *testing.T) {
	deps := setupConsoleHandlerDeps(t)
	deps.ConsoleConnSvc = &fakeConsoleSvc{}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/instances/:id/console-connections", instanceConsoleConnectionsList(deps))

	req := httptest.NewRequest("GET", "/instances/ocid1.inst.unknown/console-connections", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d want 404", w.Code)
	}
}

// TestInstanceConsoleConnectionsList_OK: resolves tenant+compartment from the
// DB, calls the service with them, returns the views.
func TestInstanceConsoleConnectionsList_OK(t *testing.T) {
	deps := setupConsoleHandlerDeps(t)
	insertInstanceRow(t, deps, "ocid1.inst.x", 7, "comp-x")
	fake := &fakeConsoleSvc{listOut: []service.ConsoleConnectionView{
		{ConnID: "ocid1.conn.a", LifecycleState: "ACTIVE", IsOurs: true, CanResume: true},
	}}
	deps.ConsoleConnSvc = fake

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/instances/:id/console-connections", instanceConsoleConnectionsList(deps))

	req := httptest.NewRequest("GET", "/instances/ocid1.inst.x/console-connections", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", w.Code, w.Body.String())
	}
	if fake.listTenant != 7 || fake.listComp != "comp-x" || fake.listInst != "ocid1.inst.x" {
		t.Errorf("service.List args = tenant=%d comp=%q inst=%q, want 7/comp-x/ocid1.inst.x",
			fake.listTenant, fake.listComp, fake.listInst)
	}
	var resp struct {
		Data []service.ConsoleConnectionView `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v (body: %s)", err, w.Body.String())
	}
	if len(resp.Data) != 1 || resp.Data[0].ConnID != "ocid1.conn.a" {
		t.Errorf("data=%+v, want one conn ocid1.conn.a", resp.Data)
	}
}

// TestInstanceConsoleConnectionsList_ServiceError -> 500.
func TestInstanceConsoleConnectionsList_ServiceError(t *testing.T) {
	deps := setupConsoleHandlerDeps(t)
	insertInstanceRow(t, deps, "ocid1.inst.x", 7, "comp-x")
	deps.ConsoleConnSvc = &fakeConsoleSvc{listErr: errors.New("boom")}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/instances/:id/console-connections", instanceConsoleConnectionsList(deps))

	req := httptest.NewRequest("GET", "/instances/ocid1.inst.x/console-connections", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d want 500", w.Code)
	}
}

// TestInstanceConsoleConnectionDelete_OK: passes connId through, calls Delete.
func TestInstanceConsoleConnectionDelete_OK(t *testing.T) {
	deps := setupConsoleHandlerDeps(t)
	insertInstanceRow(t, deps, "ocid1.inst.x", 7, "comp-x")
	fake := &fakeConsoleSvc{}
	deps.ConsoleConnSvc = fake

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.DELETE("/instances/:id/console-connections/:connId", instanceConsoleConnectionDelete(deps))

	req := httptest.NewRequest("DELETE", "/instances/ocid1.inst.x/console-connections/ocid1.conn.a", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", w.Code, w.Body.String())
	}
	if fake.delConn != "ocid1.conn.a" || fake.delInst != "ocid1.inst.x" || fake.delTenant != 7 || fake.delComp != "comp-x" {
		t.Errorf("service.Delete args = tenant=%d comp=%q inst=%q conn=%q, want 7/comp-x/ocid1.inst.x/ocid1.conn.a",
			fake.delTenant, fake.delComp, fake.delInst, fake.delConn)
	}
}

// TestInstanceConsoleConnectionDelete_NotFound -> 404 (instance unknown).
func TestInstanceConsoleConnectionDelete_NotFound(t *testing.T) {
	deps := setupConsoleHandlerDeps(t)
	deps.ConsoleConnSvc = &fakeConsoleSvc{}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.DELETE("/instances/:id/console-connections/:connId", instanceConsoleConnectionDelete(deps))

	req := httptest.NewRequest("DELETE", "/instances/ocid1.inst.unknown/console-connections/ocid1.conn.a", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d want 404", w.Code)
	}
}

// TestInstanceConsoleConnectionDelete_ServiceError -> 500.
func TestInstanceConsoleConnectionDelete_ServiceError(t *testing.T) {
	deps := setupConsoleHandlerDeps(t)
	insertInstanceRow(t, deps, "ocid1.inst.x", 7, "comp-x")
	deps.ConsoleConnSvc = &fakeConsoleSvc{deleteErr: errors.New("nope")}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.DELETE("/instances/:id/console-connections/:connId", instanceConsoleConnectionDelete(deps))

	req := httptest.NewRequest("DELETE", "/instances/ocid1.inst.x/console-connections/ocid1.conn.a", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d want 500", w.Code)
	}
}
