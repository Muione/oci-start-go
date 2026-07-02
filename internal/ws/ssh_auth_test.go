// Package ws — ssh_auth_test.go: tests buildAuth's selection between password
// and private-key (PublicKeys) auth, including passphrase handling + parse
// errors. Extracted from handleConnect so the selection logic is unit-testable
// without a real SSH dial.
package ws

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Muione/oci-start-go/internal/util/sshkeygen"
)

func TestBuildAuth_Password(t *testing.T) {
	auth, err := buildAuth("hunter2", "", "")
	if err != nil {
		t.Fatalf("password: err=%v", err)
	}
	if len(auth) != 1 {
		t.Errorf("password: len=%d want 1", len(auth))
	}
}

func TestBuildAuth_BadKey(t *testing.T) {
	_, err := buildAuth("", "not a private key", "")
	if err == nil {
		t.Error("bad key: want parse error")
	}
}

func TestBuildAuth_ValidKey(t *testing.T) {
	kp, err := sshkeygen.GenerateRSA2048()
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	auth, err := buildAuth("", kp.PrivateKeyPEM, "")
	if err != nil {
		t.Fatalf("valid key: err=%v", err)
	}
	if len(auth) != 1 {
		t.Errorf("valid key: len=%d want 1", len(auth))
	}
}

// When a key is provided, password is ignored (key takes precedence).
func TestBuildAuth_KeyPrecedence(t *testing.T) {
	kp, _ := sshkeygen.GenerateRSA2048()
	auth, err := buildAuth("ignored", kp.PrivateKeyPEM, "")
	if err != nil || len(auth) != 1 {
		t.Errorf("key precedence: auth=%v err=%v", auth, err)
	}
}

// --- resolveKeyAuth: keyId (DB-stored) takes precedence over ad-hoc key. ---

func TestResolveKeyAuth_KeyID(t *testing.T) {
	kp, _ := sshkeygen.GenerateRSA2048()
	deps := &SSHDeps{
		ResolveSSHKey: func(_ context.Context, id int64) (string, string, error) {
			if id != 5 {
				t.Errorf("resolve id=%d want 5", id)
			}
			return kp.PrivateKeyPEM, "", nil
		},
	}
	// keyId=5 → resolved key is used; the ad-hoc privateKey "ignored" + password
	// are NOT used (buildAuth: key takes precedence over password).
	auth, err := resolveKeyAuth(deps, "pw", "ignored", "", 5)
	if err != nil || len(auth) != 1 {
		t.Errorf("keyId: auth=%v err=%v", auth, err)
	}
}

func TestResolveKeyAuth_KeyIDError(t *testing.T) {
	deps := &SSHDeps{
		ResolveSSHKey: func(context.Context, int64) (string, string, error) {
			return "", "", errors.New("not found")
		},
	}
	_, err := resolveKeyAuth(deps, "pw", "", "", 9)
	if err == nil || !strings.Contains(err.Error(), "resolve ssh key 9") {
		t.Errorf("got %v, want 'resolve ssh key 9' error", err)
	}
}

// keyId=0 falls back to ad-hoc privateKey (no deps call).
func TestResolveKeyAuth_FallbackNoKeyID(t *testing.T) {
	called := false
	deps := &SSHDeps{ResolveSSHKey: func(context.Context, int64) (string, string, error) {
		called = true
		return "", "", nil
	}}
	kp, _ := sshkeygen.GenerateRSA2048()
	_, err := resolveKeyAuth(deps, "", kp.PrivateKeyPEM, "", 0)
	if err != nil {
		t.Fatalf("fallback: %v", err)
	}
	if called {
		t.Error("ResolveSSHKey should not be called when keyId=0")
	}
}
