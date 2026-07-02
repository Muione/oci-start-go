package crypto

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestDecryptStringWithFallback_Ciphertext(t *testing.T) {
	ct, err := EncryptString("tenant-secret", testKey)
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}
	got := DecryptStringWithFallback(ct, testKey)
	if got != "tenant-secret" {
		t.Errorf("got %q, want %q", got, "tenant-secret")
	}
}

func TestDecryptStringWithFallback_PlaintextPassthrough(t *testing.T) {
	// A plain (un-encrypted) string is not valid envelope output; the helper
	// must return it verbatim rather than erroring.
	in := "not-encrypted-plaintext"
	if got := DecryptStringWithFallback(in, testKey); got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

func TestDecryptStringWithFallback_ValidBase64ButNotEnvelope(t *testing.T) {
	// "dGVzdA==" is base64("test") — decodes fine but GCM auth fails; must
	// fall back to the raw input.
	in := base64.StdEncoding.EncodeToString([]byte("test"))
	if got := DecryptStringWithFallback(in, testKey); got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

func TestDecryptStringWithFallback_Empty(t *testing.T) {
	if got := DecryptStringWithFallback("", testKey); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestDecryptStringWithFallback_WrongKeyReturnsRaw(t *testing.T) {
	ct, _ := EncryptString("secret", testKey)
	wrong := bytes.Repeat([]byte{0x99}, 32)
	// Wrong key → GCM auth fail → degrade to raw ciphertext (not plaintext).
	if got := DecryptStringWithFallback(ct, wrong); got != ct {
		t.Errorf("got %q, want raw ciphertext %q", got, ct)
	}
}
