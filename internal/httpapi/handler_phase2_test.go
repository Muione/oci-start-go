package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/Muione/oci-start-go/internal/response"
)

// setupPhase2TestRouter creates a Gin engine with Phase 2 routes registered
// WITHOUT auth middleware (for unit testing handler input validation).
func setupPhase2TestRouter(deps *Deps) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// TE-101: Instance modify
	r.POST("/instances/:id/modify", instanceModify(deps))
	// TE-102: VPU update
	r.POST("/instances/:id/vpu", instanceUpdateVpu(deps))
	// TE-105: Instance operations
	r.POST("/instances/:id/start", instanceStart(deps))
	r.POST("/instances/:id/stop", instanceStop(deps))
	r.POST("/instances/:id/terminate", instanceTerminate(deps))
	r.POST("/instances/:id/restart", instanceRestart(deps))
	// TE-106: Shape/Image listing
	r.GET("/oci/shapes", listShapes(deps))
	r.GET("/oci/images", listImages(deps))
	// TE-103: VNIC management
	r.GET("/oci/vnic/loadData", vnicLoadData(deps))
	r.POST("/oci/vnic/create", vnicCreate(deps))
	r.POST("/oci/vnic/delete", vnicDelete(deps))
	r.POST("/oci/vnic/createIpv6", vnicCreateIpv6(deps))
	r.POST("/oci/vnic/deleteIpv6", vnicDeleteIpv6(deps))
	r.POST("/oci/vnic/deleteAllSecondary", vnicDeleteAllSecondary(deps))
	r.GET("/oci/vnic/refresh", vnicRefresh(deps))
	r.POST("/oci/vnic/changeSpecIp", vnicChangeSpecIp(deps))
	r.POST("/oci/vnic/network/configureLoadBalancer", vnicConfigureLB(deps))
	r.POST("/oci/vnic/network/restoreNetwork", vnicRestoreNetwork(deps))
	return r
}

// ============================================================
// TE-101: Instance Modify API
// ============================================================

func TestInstanceModify_InvalidID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"shape":"VM.Standard.A1.Flex"}`)
	req := httptest.NewRequest("POST", "/instances/abc/modify", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Success {
		t.Error("expected success=false for invalid id")
	}
}

func TestInstanceModify_InvalidBody(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("POST", "/instances/1/modify", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInstanceModify_EmptyShapeAndDisplayName(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest("POST", "/instances/1/modify", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	var resp response.ApiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Success {
		t.Error("expected success=false for empty shape and displayName")
	}
}

func TestInstanceModify_PassesValidation_WithShape(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"shape":"VM.Standard.A1.Flex","ocpus":4,"memoryInGbs":24}`)
	req := httptest.NewRequest("POST", "/instances/1/modify", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	// Should pass input validation (may panic at service level due to nil InstanceSvc)
	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid shape should pass handler validation, got: %s", resp.Message)
	}
}

func TestInstanceModify_PassesValidation_WithDisplayName(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"displayName":"my-instance"}`)
	req := httptest.NewRequest("POST", "/instances/1/modify", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid displayName should pass handler validation, got: %s", resp.Message)
	}
}

// ============================================================
// TE-102: VPU Update API
// ============================================================

func TestInstanceVpu_InvalidID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"vpusPerGb":20}`)
	req := httptest.NewRequest("POST", "/instances/abc/vpu", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInstanceVpu_InvalidBody(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("POST", "/instances/1/vpu", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInstanceVpu_NegativeValue(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"vpusPerGb":-1}`)
	req := httptest.NewRequest("POST", "/instances/1/vpu", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInstanceVpu_ExceedsMaxValue(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"vpusPerGb":121}`)
	req := httptest.NewRequest("POST", "/instances/1/vpu", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInstanceVpu_ValidValues(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	validValues := []int{0, 10, 20, 30, 60, 120}
	for _, vpu := range validValues {
		body := bytes.NewBufferString(`{"vpusPerGb":` + strconv.Itoa(vpu) + `}`)
		req := httptest.NewRequest("POST", "/instances/1/vpu", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		serveWithRecover(r, w, req)

		// Should pass input validation (may panic at service level)
		if w.Code == http.StatusBadRequest {
			var resp response.ApiResponse
			json.Unmarshal(w.Body.Bytes(), &resp)
			t.Errorf("vpusPerGb=%d should pass handler validation, got: %s", vpu, resp.Message)
		}
	}
}

func TestInstanceVpu_ValidValuesSimple(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	// Test a few valid VPU values that should pass input validation
	testCases := []struct {
		name     string
		vpuValue string
	}{
		{"zero", "0"},
		{"balanced", "10"},
		{"higher_perf", "20"},
		{"max", "120"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := bytes.NewBufferString(`{"vpusPerGb":` + tc.vpuValue + `}`)
			req := httptest.NewRequest("POST", "/instances/1/vpu", body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			serveWithRecover(r, w, req)

			// Should pass input validation (may panic at service level)
			if w.Code == http.StatusBadRequest {
				var resp response.ApiResponse
				json.Unmarshal(w.Body.Bytes(), &resp)
				t.Errorf("vpusPerGb=%s should pass handler validation, got: %s", tc.vpuValue, resp.Message)
			}
		})
	}
}

// ============================================================
// TE-105: Instance Operation API (start/stop/terminate/restart)
// ============================================================

func TestInstanceStart_InvalidID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("POST", "/instances/abc/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInstanceStop_InvalidID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("POST", "/instances/abc/stop", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInstanceTerminate_InvalidID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("POST", "/instances/abc/terminate", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInstanceRestart_InvalidID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("POST", "/instances/abc/restart", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInstanceStart_ValidID_PassesValidation(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("POST", "/instances/1/start", nil)
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	// Should pass input validation (id=1 is valid)
	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid id should pass handler validation, got: %s", resp.Message)
	}
}

func TestInstanceStop_ValidID_PassesValidation(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("POST", "/instances/1/stop", nil)
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid id should pass handler validation, got: %s", resp.Message)
	}
}

func TestInstanceTerminate_ValidID_PassesValidation(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("POST", "/instances/1/terminate", nil)
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid id should pass handler validation, got: %s", resp.Message)
	}
}

func TestInstanceTerminate_WithPreserveBootVolume(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"preserveBootVolume":true}`)
	req := httptest.NewRequest("POST", "/instances/1/terminate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	// Should pass input validation (body is optional and valid)
	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid terminate request should pass handler validation, got: %s", resp.Message)
	}
}

func TestInstanceRestart_ValidID_PassesValidation(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("POST", "/instances/1/restart", nil)
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid id should pass handler validation, got: %s", resp.Message)
	}
}

// ============================================================
// TE-106: Shape/Image Listing API
// ============================================================

func TestListShapes_MissingTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("GET", "/oci/shapes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestListShapes_InvalidTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("GET", "/oci/shapes?tenantId=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestListImages_MissingTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("GET", "/oci/images", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestListImages_InvalidTenantID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("GET", "/oci/images?tenantId=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestListShapes_ValidTenantID_PassesValidation(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("GET", "/oci/shapes?tenantId=1", nil)
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	// Should pass input validation (may panic at service level)
	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid tenantId should pass handler validation, got: %s", resp.Message)
	}
}

func TestListImages_ValidTenantID_PassesValidation(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("GET", "/oci/images?tenantId=1", nil)
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	// Should pass input validation (may panic at service level)
	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid tenantId should pass handler validation, got: %s", resp.Message)
	}
}

func TestListShapes_WithArchitectureFilter(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("GET", "/oci/shapes?tenantId=1&architecture=ARM", nil)
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	// Should pass input validation
	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("architecture filter should pass handler validation, got: %s", resp.Message)
	}
}

func TestListImages_WithFilters(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("GET", "/oci/images?tenantId=1&architecture=ARM&shape=VM.Standard.A1.Flex", nil)
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	// Should pass input validation
	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("architecture+shape filters should pass handler validation, got: %s", resp.Message)
	}
}

// ============================================================
// TE-103: VNIC Management API
// ============================================================

func TestVnicLoadData_MissingInstanceID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("GET", "/oci/vnic/loadData", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicCreate_MissingFields(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	// Empty body
	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest("POST", "/oci/vnic/create", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicCreate_MissingInstanceID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"subnetId":"ocid1.subnet.oc1..aaaa","vnicCount":1,"ipv6CountPerVnic":0}`)
	req := httptest.NewRequest("POST", "/oci/vnic/create", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicCreate_MissingSubnetID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"instanceId":"ocid1.instance.oc1..aaaa","vnicCount":1,"ipv6CountPerVnic":0}`)
	req := httptest.NewRequest("POST", "/oci/vnic/create", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicCreate_InvalidBody(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("POST", "/oci/vnic/create", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicDelete_MissingFields(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest("POST", "/oci/vnic/delete", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicDelete_MissingVnicID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"instanceId":"ocid1.instance.oc1..aaaa"}`)
	req := httptest.NewRequest("POST", "/oci/vnic/delete", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicDelete_MissingInstanceID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"vnicId":"ocid1.vnic.oc1..aaaa"}`)
	req := httptest.NewRequest("POST", "/oci/vnic/delete", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicCreateIpv6_MissingFields(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest("POST", "/oci/vnic/createIpv6", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicCreateIpv6_MissingVnicID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"instanceId":"ocid1.instance.oc1..aaaa","ipv6Count":5}`)
	req := httptest.NewRequest("POST", "/oci/vnic/createIpv6", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicCreateIpv6_MissingInstanceID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"vnicId":"ocid1.vnic.oc1..aaaa","ipv6Count":5}`)
	req := httptest.NewRequest("POST", "/oci/vnic/createIpv6", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicDeleteIpv6_MissingFields(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest("POST", "/oci/vnic/deleteIpv6", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicDeleteIpv6_MissingIpv6Address(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"vnicId":"ocid1.vnic.oc1..aaaa","instanceId":"ocid1.instance.oc1..aaaa"}`)
	req := httptest.NewRequest("POST", "/oci/vnic/deleteIpv6", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicDeleteAllSecondary_MissingInstanceID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest("POST", "/oci/vnic/deleteAllSecondary", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicRefresh_MissingInstanceID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("GET", "/oci/vnic/refresh", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicChangeSpecIp_MissingFields(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest("POST", "/oci/vnic/changeSpecIp", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicChangeSpecIp_MissingCidrRanges(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"instanceId":"ocid1.instance.oc1..aaaa","vnicId":"ocid1.vnic.oc1..aaaa","cidrRanges":[]}`)
	req := httptest.NewRequest("POST", "/oci/vnic/changeSpecIp", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicConfigureLB_MissingInstanceID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest("POST", "/oci/vnic/network/configureLoadBalancer", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVnicRestoreNetwork_MissingInstanceID(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest("POST", "/oci/vnic/network/restoreNetwork", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ============================================================
// TE-103: VNIC Management API - Valid requests pass validation
// ============================================================

func TestVnicCreate_ValidRequest_PassesValidation(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"instanceId":"ocid1.instance.oc1..aaaa","subnetId":"ocid1.subnet.oc1..aaaa","vnicCount":2,"ipv6CountPerVnic":1}`)
	req := httptest.NewRequest("POST", "/oci/vnic/create", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	// Should pass input validation
	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid vnic create request should pass handler validation, got: %s", resp.Message)
	}
}

func TestVnicDelete_ValidRequest_PassesValidation(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"instanceId":"ocid1.instance.oc1..aaaa","vnicId":"ocid1.vnic.oc1..aaaa"}`)
	req := httptest.NewRequest("POST", "/oci/vnic/delete", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid vnic delete request should pass handler validation, got: %s", resp.Message)
	}
}

func TestVnicCreateIpv6_ValidRequest_PassesValidation(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"instanceId":"ocid1.instance.oc1..aaaa","vnicId":"ocid1.vnic.oc1..aaaa","ipv6Count":3}`)
	req := httptest.NewRequest("POST", "/oci/vnic/createIpv6", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid ipv6 create request should pass handler validation, got: %s", resp.Message)
	}
}

func TestVnicDeleteAllSecondary_ValidRequest_PassesValidation(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"instanceId":"ocid1.instance.oc1..aaaa"}`)
	req := httptest.NewRequest("POST", "/oci/vnic/deleteAllSecondary", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid deleteAllSecondary request should pass handler validation, got: %s", resp.Message)
	}
}

func TestVnicRefresh_ValidRequest_PassesValidation(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("GET", "/oci/vnic/refresh?instanceId=ocid1.instance.oc1..aaaa", nil)
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid refresh request should pass handler validation, got: %s", resp.Message)
	}
}

func TestVnicLoadData_ValidRequest_PassesValidation(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	req := httptest.NewRequest("GET", "/oci/vnic/loadData?instanceId=ocid1.instance.oc1..aaaa", nil)
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid loadData request should pass handler validation, got: %s", resp.Message)
	}
}

func TestVnicChangeSpecIp_ValidRequest_PassesValidation(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"instanceId":"ocid1.instance.oc1..aaaa","vnicId":"ocid1.vnic.oc1..aaaa","cidrRanges":["10.0.0.0/24","10.0.1.0/24"]}`)
	req := httptest.NewRequest("POST", "/oci/vnic/changeSpecIp", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid changeSpecIp request should pass handler validation, got: %s", resp.Message)
	}
}

func TestVnicConfigureLB_ValidRequest_PassesValidation(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"instanceId":"ocid1.instance.oc1..aaaa"}`)
	req := httptest.NewRequest("POST", "/oci/vnic/network/configureLoadBalancer", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid configureLoadBalancer request should pass handler validation, got: %s", resp.Message)
	}
}

func TestVnicRestoreNetwork_ValidRequest_PassesValidation(t *testing.T) {
	deps := &Deps{}
	r := setupPhase2TestRouter(deps)

	body := bytes.NewBufferString(`{"instanceId":"ocid1.instance.oc1..aaaa"}`)
	req := httptest.NewRequest("POST", "/oci/vnic/network/restoreNetwork", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	serveWithRecover(r, w, req)

	if w.Code == http.StatusBadRequest {
		var resp response.ApiResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Errorf("valid restoreNetwork request should pass handler validation, got: %s", resp.Message)
	}
}
