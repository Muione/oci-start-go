package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/Muione/oci-start-go/internal/response"
)

// setupBillingTestRouter creates a Gin engine with the billing routes registered
// but WITHOUT auth middleware (for unit testing handler input validation).
func setupBillingTestRouter(deps *Deps) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/tenants/:id/subscription", billingSubscription(deps))
	r.GET("/tenants/:id/cost", billingCost(deps))
	return r
}

// --- TE-001: Subscription Time Query ---

func TestBillingSubscription_InvalidTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupBillingTestRouter(deps)

	req := httptest.NewRequest("GET", "/tenants/abc/subscription", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	var resp response.ApiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false for invalid tenant ID")
	}
}

func TestBillingSubscription_NegativeID(t *testing.T) {
	deps := &Deps{}
	r := setupBillingTestRouter(deps)

	req := httptest.NewRequest("GET", "/tenants/-1/subscription", nil)
	w := httptest.NewRecorder()

	// -1 is a valid int64, so it passes parsing but panics at service level
	// (nil BillingSvc). Verify the handler panics (or returns non-200 if recovered).
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		r.ServeHTTP(w, req)
	}()

	if !panicked && w.Code == http.StatusOK {
		t.Error("expected panic or non-200 for negative tenant ID with nil service")
	}
}

// --- TE-004: Cost Query ---

func TestBillingCost_InvalidTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupBillingTestRouter(deps)

	req := httptest.NewRequest("GET", "/tenants/abc/cost", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	var resp response.ApiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false for invalid tenant ID")
	}
}

func TestBillingCost_CustomRange_MissingStart(t *testing.T) {
	deps := &Deps{}
	r := setupBillingTestRouter(deps)

	req := httptest.NewRequest("GET", "/tenants/1/cost?type=custom&end=2024-01-31", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	var resp response.ApiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false for missing start date")
	}
}

func TestBillingCost_CustomRange_MissingEnd(t *testing.T) {
	deps := &Deps{}
	r := setupBillingTestRouter(deps)

	req := httptest.NewRequest("GET", "/tenants/1/cost?type=custom&start=2024-01-01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestBillingCost_CustomRange_MissingBoth(t *testing.T) {
	deps := &Deps{}
	r := setupBillingTestRouter(deps)

	req := httptest.NewRequest("GET", "/tenants/1/cost?type=custom", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// serveWithRecover runs r.ServeHTTP and returns whether it panicked.
// Used for tests that reach the nil BillingSvc service call.
func serveWithRecover(r *gin.Engine, w *httptest.ResponseRecorder, req *http.Request) (panicked bool) {
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		r.ServeHTTP(w, req)
	}()
	return panicked
}

func TestBillingCost_DefaultType(t *testing.T) {
	// When no type is specified, it defaults to "current_month"
	deps := &Deps{}
	r := setupBillingTestRouter(deps)

	req := httptest.NewRequest("GET", "/tenants/1/cost", nil)
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	// With nil BillingSvc this will panic at service level (not 400).
	// The handler passes validation — verify it didn't reject at input level.
	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("default type should pass validation, got: %s", resp.Message)
	}
}

func TestBillingCost_AllQueryTypes(t *testing.T) {
	deps := &Deps{}
	r := setupBillingTestRouter(deps)

	queryTypes := []string{"yesterday", "today", "current_month", "last_month"}
	for _, qt := range queryTypes {
		req := httptest.NewRequest("GET", "/tenants/1/cost?type="+qt, nil)
		w := httptest.NewRecorder()
		serveWithRecover(r, w, req)

		// All valid types should pass handler validation (may panic at service)
		if w.Code == http.StatusBadRequest {
			var resp response.ApiResponse
			json.Unmarshal(w.Body.Bytes(), &resp)
			t.Errorf("query type %q should pass validation, got: %s", qt, resp.Message)
		}
	}
}
