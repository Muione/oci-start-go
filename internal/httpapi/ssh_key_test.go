// Package httpapi — ssh_key_test.go: tests the SSH key CRUD handlers with a
// fake SSHKeyLister (no DB / no encryption). The handlers validate input +
// map service errors to HTTP codes.
package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/service"
)

type fakeSSHKeySvc struct {
	listOut  []service.SSHKeyView
	listErr  error
	createID int64
	createEr error
	deleteEr error
	gotLabel string
	gotContent string
	gotDelID  int64
}

func (f *fakeSSHKeySvc) List(context.Context) ([]service.SSHKeyView, error) {
	return f.listOut, f.listErr
}
func (f *fakeSSHKeySvc) Create(_ context.Context, label, content, passphrase string) (int64, error) {
	f.gotLabel = label
	f.gotContent = content
	_ = passphrase
	return f.createID, f.createEr
}
func (f *fakeSSHKeySvc) Delete(_ context.Context, id int64) error {
	f.gotDelID = id
	return f.deleteEr
}

func sshKeyDeps(svc *fakeSSHKeySvc) *Deps {
	return &Deps{Logger: zerolog.Nop(), SSHKeySvc: svc}
}

func TestSSHKeys_List_OK(t *testing.T) {
	deps := sshKeyDeps(&fakeSSHKeySvc{listOut: []service.SSHKeyView{{ID: 1, Label: "k", Fingerprint: "SHA256:x"}}})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ssh-keys", sshKeysList(deps))
	req := httptest.NewRequest("GET", "/ssh-keys", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"label":"k"`)) {
		t.Errorf("body missing label: %s", w.Body.String())
	}
}

func TestSSHKeyCreate_Validation(t *testing.T) {
	deps := sshKeyDeps(&fakeSSHKeySvc{})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/ssh-keys", sshKeyCreate(deps))
	req := httptest.NewRequest("POST", "/ssh-keys", bytes.NewBufferString(`{"label":"","content":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d want 400", w.Code)
	}
}

func TestSSHKeyCreate_OK(t *testing.T) {
	fake := &fakeSSHKeySvc{createID: 7}
	deps := sshKeyDeps(fake)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/ssh-keys", sshKeyCreate(deps))
	req := httptest.NewRequest("POST", "/ssh-keys", bytes.NewBufferString(`{"label":"mykey","content":"PEM","passphrase":"pw"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if fake.gotLabel != "mykey" || fake.gotContent != "PEM" {
		t.Errorf("create args label=%q content=%q", fake.gotLabel, fake.gotContent)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"id":7`)) {
		t.Errorf("body missing id=7: %s", w.Body.String())
	}
}

func TestSSHKeyCreate_ServiceError(t *testing.T) {
	deps := sshKeyDeps(&fakeSSHKeySvc{createEr: errors.New("invalid private key: bad pem")})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/ssh-keys", sshKeyCreate(deps))
	req := httptest.NewRequest("POST", "/ssh-keys", bytes.NewBufferString(`{"label":"k","content":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d want 500", w.Code)
	}
}

func TestSSHKeyDelete(t *testing.T) {
	fake := &fakeSSHKeySvc{}
	deps := sshKeyDeps(fake)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.DELETE("/ssh-keys/:id", sshKeyDelete(deps))
	req := httptest.NewRequest("DELETE", "/ssh-keys/3", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if fake.gotDelID != 3 {
		t.Errorf("deleted id=%d want 3", fake.gotDelID)
	}
	// invalid id → 400
	req2 := httptest.NewRequest("DELETE", "/ssh-keys/abc", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("bad id: status=%d want 400", w2.Code)
	}
	// prevent unused-import on strconv if not used elsewhere
}
