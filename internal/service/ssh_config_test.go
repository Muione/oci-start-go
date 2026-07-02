// Package service -- ssh_config_test.go: S8 regression. The root password must
// not be interpolated into a shell command string (command injection); it must
// travel via stdin so shell metacharacters in the password cannot escape.
package service

import (
	"strings"
	"testing"

	"github.com/Muione/oci-start-go/internal/util/crypto"
)

// TestBuildRootPasswordInput_NoShellInjection feeds a password full of shell
// metacharacters and asserts the generated command/script carries none of them
// unescaped and does not embed the raw password. The password must be carried
// by stdin instead.
func TestBuildRootPasswordInput_NoShellInjection(t *testing.T) {
	pw := "a\";rm -rf / #$(whoami)`id`"
	script, stdin := buildRootPasswordInput(pw)

	for _, bad := range []string{";", "$", "`", "\"", "rm -rf", "whoami"} {
		if strings.Contains(script, bad) {
			t.Errorf("script contains %q (injection risk): %q", bad, script)
		}
	}
	if strings.Contains(script, pw) {
		t.Errorf("script must not embed the raw password: %q", script)
	}
	if !strings.Contains(stdin, "root:"+pw) {
		t.Errorf("stdin must carry the password verbatim, got %q", stdin)
	}
}

// TestSSHConfigurator_DecryptIfSet: rescue flow passes instance_detail.password
// (AES-encrypted for new rows per S4) into EnableRootLogin. The configurator
// must decrypt before SSH dial; nil masterKey → legacy plaintext passthrough.
func TestSSHConfigurator_DecryptIfSet(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	s := &SSHConfigurator{masterKey: key}

	// plaintext passes through (fallback)
	plain := "hunter2"
	if got := s.decryptIfSet(plain); got != plain {
		t.Fatalf("plaintext fallback: got %q want %q", got, plain)
	}
	// ciphertext decrypts back to plain
	enc, err := crypto.EncryptString(plain, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if enc == plain {
		t.Fatal("ciphertext should differ from plaintext")
	}
	if got := s.decryptIfSet(enc); got != plain {
		t.Fatalf("ciphertext decrypt: got %q want %q", got, plain)
	}
	// nil masterKey → as-is (unwired/legacy)
	s2 := &SSHConfigurator{}
	if got := s2.decryptIfSet(enc); got != enc {
		t.Fatalf("nil key passthrough: got %q want %q", got, enc)
	}
}
