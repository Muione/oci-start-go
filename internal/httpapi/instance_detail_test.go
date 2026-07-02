package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/oracle/oci-go-sdk/v65/core"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/service"
	_ "modernc.org/sqlite" // register "sqlite" driver
)

// setupInstanceSyncDeps opens an in-memory store for instance sync tests. The
// ociClientsForInstance and OCI-op seams are swapped by each test, so no real
// tenant/instance data is required — only a write pool to break.
func setupInstanceSyncDeps(t *testing.T) *Deps {
	t.Helper()
	store, err := db.Open("file::memory:?cache=shared", 1, 1)
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return &Deps{Store: store, Logger: zerolog.Nop()}
}

// swapClientsForInstance replaces the client-construction seam with one that
// returns a dummy instance detail, restoring it on test cleanup.
func swapClientsForInstance(t *testing.T) {
	t.Helper()
	orig := ociClientsForInstance
	t.Cleanup(func() { ociClientsForInstance = orig })
	ociClientsForInstance = func(c *gin.Context, deps *Deps, id int64) (oci.Clients, *service.InstanceDetailResp, error) {
		return oci.Clients{}, &service.InstanceDetailResp{
			ID:           id,
			InstanceID:   "ocid1.instance.x",
			CompartmentID: "ocid1.tenancy.x",
			PublicIps:    "1.2.3.4",
		}, nil
	}
}

func strPtr(s string) *string { return &s }

// E2 (Terminate): when the cloud op succeeds but the local DB delete fails, the
// response must carry a syncFailed marker instead of a plain success.
func TestInstanceTerminate_LocalSyncFails_SyncFailed(t *testing.T) {
	deps := setupInstanceSyncDeps(t)
	swapClientsForInstance(t)

	orig := ociTerminateInstance
	t.Cleanup(func() { ociTerminateInstance = orig })
	ociTerminateInstance = func(ctx context.Context, c oci.Clients, instanceID string, preserve bool) error { return nil }

	if err := deps.Store.Write.Close(); err != nil {
		t.Fatalf("close write pool: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/instances/:id/terminate", instanceTerminate(deps))

	req := httptest.NewRequest("POST", "/instances/1/terminate", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (cloud op succeeded)", w.Code, http.StatusOK)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("syncFailed")) {
		t.Errorf("response must carry syncFailed marker, got: %s", w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("local sync failed")) {
		t.Errorf("response must mention local sync failure, got: %s", w.Body.String())
	}
}

// E2 (ChangeIP): ReassignPublicIP succeeds but the local IP update fails →
// syncFailed marker.
func TestInstanceChangeIP_LocalSyncFails_SyncFailed(t *testing.T) {
	deps := setupInstanceSyncDeps(t)
	swapClientsForInstance(t)

	orig := ociReassignPublicIP
	t.Cleanup(func() { ociReassignPublicIP = orig })
	ociReassignPublicIP = func(ctx context.Context, c oci.Clients, compartmentID, instanceID string) (string, error) {
		return "5.6.7.8", nil
	}

	if err := deps.Store.Write.Close(); err != nil {
		t.Fatalf("close write pool: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/instances/:id/change-ip", instanceChangeIP(deps))

	req := httptest.NewRequest("POST", "/instances/1/change-ip", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("syncFailed")) {
		t.Errorf("response must carry syncFailed marker, got: %s", w.Body.String())
	}
}

// exportTestSchema carries instance_detail + tenant with exactly the columns
// ListAllInstanceDetails and ListTenants scan, so instanceExport runs against an
// in-memory DB without the full migration set.
const exportTestSchema = `
CREATE TABLE tenant (
    id INTEGER PRIMARY KEY, tenant_id TEXT, user_name TEXT, fingerprint TEXT, tenancy TEXT, region TEXT,
    created_at TEXT, api_synced INTEGER, enable_icmp INTEGER, enable_all_protocol INTEGER,
    is_home_region INTEGER, paren_id BIGINT, tenancy_name TEXT, tenancy_des TEXT, account_type TEXT,
    cloud_type INTEGER DEFAULT 1, region_en TEXT, id_str TEXT, email_address TEXT, email_enable INTEGER,
    transfer_status INTEGER DEFAULT 0, transfer_amount TEXT, is_active INTEGER DEFAULT 1
);
CREATE TABLE instance_detail (
    id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id BIGINT, instance_id TEXT, display_name TEXT,
    shape TEXT, state TEXT, ocpus INTEGER, memory_in_gbs INTEGER, boot_volume_size_in_gbs BIGINT,
    public_ips TEXT, private_ips TEXT, availability_domain TEXT, compartment_id TEXT,
    boot_volume_id TEXT, remark TEXT, boot_volume_name TEXT, vpus_per_gb TEXT,
    ipv6_addresses TEXT, vnic_ids TEXT, username TEXT, port INTEGER, password TEXT,
    processor_description TEXT, architecture TEXT, cloud_type INTEGER DEFAULT 1,
    sys_image_backup INTEGER DEFAULT 0, conn_time BIGINT NOT NULL DEFAULT 0,
    enable_ping INTEGER NOT NULL DEFAULT 0, on_line_enable INTEGER NOT NULL DEFAULT 1,
    last_on_line_enable INTEGER NOT NULL DEFAULT 1, offline_notify INTEGER NOT NULL DEFAULT 0,
    resume_notify INTEGER NOT NULL DEFAULT 0, monitor_installed INTEGER, last_heartbeat TEXT,
    create_time TEXT
);
`

// S4: instanceExport must never leak the root password (plaintext or
// ciphertext) — it is redacted to "[redacted]".
func TestInstanceExport_RedactsPassword(t *testing.T) {
	store, err := db.Open("file::memory:?cache=shared", 1, 1)
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if _, err := store.Write.Exec(exportTestSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	plain := "super-secret-root-pw"
	if _, err := store.Write.Exec(`INSERT INTO tenant (id, tenant_id, user_name, tenancy, region, created_at, is_active)
		VALUES (1, 'ocid1.tenancy.x', 'user1', 'Tenancy X', 'us-phoenix-1', '2026-01-01', 1)`); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := store.Write.Exec(`INSERT INTO instance_detail (tenant_id, instance_id, display_name, state, password, conn_time, enable_ping, on_line_enable, create_time)
		VALUES (1, 'ocid1.instance.x', 'inst-1', 'RUNNING', ?, 0, 0, 1, '2026-01-01 00:00:00')`, plain); err != nil {
		t.Fatalf("seed instance: %v", err)
	}

	deps := &Deps{Store: store, Logger: zerolog.Nop(), InstanceSvc: service.NewInstanceDetailSvc(store)}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/instances/export", instanceExport(deps))

	req := httptest.NewRequest("GET", "/instances/export", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, plain) {
		t.Errorf("export leaks plaintext password %q; body:\n%s", plain, body)
	}
	if !strings.Contains(body, "[redacted]") {
		t.Errorf("export must mark password as [redacted]; body:\n%s", body)
	}
}

// E2 (EnableIPv6): IPv6 assignment succeeds but the local DB update fails →
// syncFailed marker.
func TestInstanceEnableIPv6_LocalSyncFails_SyncFailed(t *testing.T) {
	deps := setupInstanceSyncDeps(t)
	swapClientsForInstance(t)

	origV := ociGetPrimaryVnic
	t.Cleanup(func() { ociGetPrimaryVnic = origV })
	ociGetPrimaryVnic = func(ctx context.Context, c oci.Clients, instanceID, compartmentID string) (core.Vnic, error) {
		return core.Vnic{Id: strPtr("ocid1.vnic.x")}, nil
	}
	origA := ociAssignIpv6ToVnic
	t.Cleanup(func() { ociAssignIpv6ToVnic = origA })
	ociAssignIpv6ToVnic = func(ctx context.Context, vcnClient *core.VirtualNetworkClient, vnicID string, forceNew bool) (string, error) {
		return "2001:db8::1", nil
	}

	if err := deps.Store.Write.Close(); err != nil {
		t.Fatalf("close write pool: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/instances/:id/enable-ipv6", instanceEnableIPv6(deps))

	req := httptest.NewRequest("POST", "/instances/1/enable-ipv6", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("syncFailed")) {
		t.Errorf("response must carry syncFailed marker, got: %s", w.Body.String())
	}
}
