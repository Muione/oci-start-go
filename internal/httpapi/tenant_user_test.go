package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/Muione/oci-start-go/internal/response"
)

// setupTenantUserTestRouter creates a Gin engine with tenant_user routes
// registered WITHOUT auth middleware for unit testing.
func setupTenantUserTestRouter(deps *Deps) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/tenants/:id/mfa/status", tenantMfaStatus(deps))
	r.POST("/tenants/:id/mfa/toggle", tenantMfaToggle(deps))
	r.POST("/tenants/:id/mfa/reset", tenantMfaReset(deps))
	r.GET("/tenants/:id/notification-recipients", tenantNotifRecipientsGet(deps))
	r.POST("/tenants/:id/notification-recipients/update", tenantNotifRecipientsUpdate(deps))
	r.GET("/tenants/:id/password-policy", tenantPasswordPolicyGet(deps))
	r.POST("/tenants/:id/password-policy", tenantPasswordPolicyUpdate(deps))
	r.GET("/tenants/:id/subscription-days", tenantSubscriptionDays(deps))
	r.GET("/tenants/:id/domains", tenantDomainTenants(deps))
	r.GET("/tenants/:id/users", tenantUsersList(deps))
	r.GET("/tenants/:id/groups", tenantGroupsList(deps))
	return r
}

// --- TE-002: MFA Settings ---

func TestMfaStatus_InvalidTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	req := httptest.NewRequest("GET", "/tenants/abc/mfa/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Success {
		t.Error("expected success=false for invalid tenant ID")
	}
}

func TestMfaToggle_InvalidTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	body := bytes.NewBufferString(`{"enable": true}`)
	req := httptest.NewRequest("POST", "/tenants/abc/mfa/toggle", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestMfaToggle_InvalidJSON(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("POST", "/tenants/1/mfa/toggle", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestMfaReset_InvalidTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	req := httptest.NewRequest("POST", "/tenants/abc/mfa/reset", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- TE-003: Domain Tenants ---

func TestDomainTenants_InvalidTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	req := httptest.NewRequest("GET", "/tenants/abc/domains", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDomainTenants_ValidIDFormat(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	req := httptest.NewRequest("GET", "/tenants/123/domains", nil)
	w := httptest.NewRecorder()

	// With nil TenantUser service, this will panic at service level.
	// We only verify the handler input validation passes (not 400 "参数 id 无效").
	panicked := false
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		r.ServeHTTP(w, req)
	}()

	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Message == "参数 id 无效" || resp.Message == "Invalid tenant ID" {
			t.Error("valid ID should pass input validation")
		}
	}
	if !panicked && w.Code == http.StatusOK {
		// If neither panic nor 200, something unexpected happened
		t.Errorf("expected panic or service error, got status %d", w.Code)
	}
}

// --- TE-005: Notification Recipients ---

func TestNotifRecipientsGet_InvalidTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	req := httptest.NewRequest("GET", "/tenants/abc/notification-recipients", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestNotifRecipientsUpdate_InvalidTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	body := bytes.NewBufferString(`{"emails": ["test@example.com"]}`)
	req := httptest.NewRequest("POST", "/tenants/abc/notification-recipients/update", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestNotifRecipientsUpdate_EmptyEmails(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	body := bytes.NewBufferString(`{"emails": []}`)
	req := httptest.NewRequest("POST", "/tenants/1/notification-recipients/update", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Success {
		t.Error("expected success=false for empty emails")
	}
}

func TestNotifRecipientsUpdate_InvalidJSON(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	body := bytes.NewBufferString(`{bad json}`)
	req := httptest.NewRequest("POST", "/tenants/1/notification-recipients/update", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- TE-006: Password Policy ---

func TestPasswordPolicyGet_InvalidTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	req := httptest.NewRequest("GET", "/tenants/abc/password-policy", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPasswordPolicyUpdate_InvalidTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	body := bytes.NewBufferString(`{"enableExpiry": true, "expiryDays": 90}`)
	req := httptest.NewRequest("POST", "/tenants/abc/password-policy", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPasswordPolicyUpdate_InvalidJSON(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	body := bytes.NewBufferString(`{invalid}`)
	req := httptest.NewRequest("POST", "/tenants/1/password-policy", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- TE-007: Subscription Days ---

func TestSubscriptionDays_InvalidTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	req := httptest.NewRequest("GET", "/tenants/abc/subscription-days", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSubscriptionDays_ValidIDFormat(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	req := httptest.NewRequest("GET", "/tenants/123/subscription-days", nil)
	w := httptest.NewRecorder()

	// With nil TenantUser service, this will panic at service level.
	// We only verify the handler input validation passes (not 400 "参数 id 无效").
	func() {
		defer func() { recover() }()
		r.ServeHTTP(w, req)
	}()

	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Message == "参数 id 无效" || resp.Message == "Invalid tenant ID" {
			t.Error("valid ID should pass input validation")
		}
	}
}

// --- Additional: Users and Groups list ---

func TestUsersList_InvalidTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	req := httptest.NewRequest("GET", "/tenants/abc/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGroupsList_InvalidTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupTenantUserTestRouter(deps)

	req := httptest.NewRequest("GET", "/tenants/abc/groups", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
