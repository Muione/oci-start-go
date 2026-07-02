package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// setupMonitorTestRouter wires only the monitor download route (no auth).
func setupMonitorTestRouter(deps *Deps) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/monitor/download", monitorDownload(deps))
	return r
}

// S1: a malicious interval must be rejected with 400 and must not leak into the
// rendered bash script (template-injection RCE via INTERVAL={{INTERVAL}}).
func TestMonitorDownload_MaliciousInterval_Rejected(t *testing.T) {
	deps := &Deps{}
	r := setupMonitorTestRouter(deps)

	// %3B = ';' so it stays inside the interval value instead of acting as a
	// query separator. Decodes to "5;curl evil".
	req := httptest.NewRequest("GET", "/api/monitor/download?token=abc123&interval=5%3Bcurl+evil", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (malicious interval must be rejected)", w.Code, http.StatusBadRequest)
	}
	if strings.Contains(w.Body.String(), "curl") {
		t.Errorf("response body must not contain the injected payload, got: %s", w.Body.String())
	}
}

// S1: a token with disallowed characters must be rejected with 400.
func TestMonitorDownload_BadToken_Rejected(t *testing.T) {
	deps := &Deps{}
	r := setupMonitorTestRouter(deps)

	req := httptest.NewRequest("GET", "/api/monitor/download?token=a;b&interval=15", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (bad token must be rejected)", w.Code, http.StatusBadRequest)
	}
}

// S1: a valid interval is rendered as a bare integer in the script.
func TestMonitorDownload_ValidInterval_OK(t *testing.T) {
	deps := &Deps{}
	r := setupMonitorTestRouter(deps)

	req := httptest.NewRequest("GET", "/api/monitor/download?token=abc123._-XYZ&interval=15", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "INTERVAL=15") {
		t.Errorf("expected script to contain INTERVAL=15, got: %s", w.Body.String())
	}
}

// S1: interval out of range is rejected.
func TestMonitorDownload_IntervalOutOfRange_Rejected(t *testing.T) {
	deps := &Deps{}
	r := setupMonitorTestRouter(deps)

	for _, v := range []string{"0", "3601", "-1"} {
		req := httptest.NewRequest("GET", "/api/monitor/download?token=abc123&interval="+v, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("interval=%s: status = %d, want %d", v, w.Code, http.StatusBadRequest)
		}
	}
}
