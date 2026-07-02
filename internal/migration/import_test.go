package migration

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

// pkcs7Pad mirrors the encrypt-side of pkcs7Unpad (PKCS5/7), used only by test
// helpers to produce ciphertext the production DecryptBytes can decrypt.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - len(data)%blockSize
	pad := make([]byte, padLen)
	for i := range pad {
		pad[i] = byte(padLen)
	}
	return append(data, pad...)
}

// encryptForTest produces an .enc-style payload (IV: + DATA: lines) for the
// given plaintext SQL and base64 key, matching the Java exportEncryptedBackup
// pipeline: gzip → AES-256-CBC encrypt → base64. Returns the full content.
func encryptForTest(t *testing.T, plainSQL, keyBase64 string) string {
	t.Helper()
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("key must be 32 bytes, got %d", len(key))
	}

	var gz bytes.Buffer
	gw := gzip.NewWriter(&gz)
	if _, err := gw.Write([]byte(plainSQL)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes cipher: %v", err)
	}
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		t.Fatalf("rand iv: %v", err)
	}

	padded := pkcs7Pad(gz.Bytes(), aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)

	ivB64 := base64.StdEncoding.EncodeToString(iv)
	dataB64 := base64.StdEncoding.EncodeToString(ciphertext)
	return "-----BEGIN OCI-START MIGRATION-----\nIV: " + ivB64 + "\nDATA: " + dataB64 + "\n-----END OCI-START MIGRATION-----"
}

// --- B2: ParseEncryptedFile with empty key must give a clear, actionable error ---

// TestParseEncryptedFile_RequiresKey asserts that ParseEncryptedFile (the no-key
// variant) returns a clear "masterKey required" message pointing the caller at
// ParseEncryptedFileWithKey, instead of a misleading base64/length error from
// deep inside DecryptBytes.
func TestParseEncryptedFile_RequiresKey(t *testing.T) {
	c := &Crypter{}
	// Content has both IV and DATA present, so the "missing IV or DATA" guard
	// is bypassed and the empty-key path is exercised.
	content := "-----BEGIN OCI-START MIGRATION-----\n" +
		"IV: AAAAAAAAAAAAAAAAAAAAAA==\n" +
		"DATA: AAAAAAAAAA==\n" +
		"-----END OCI-START MIGRATION-----"

	_, err := c.ParseEncryptedFile(content)
	if err == nil {
		t.Fatal("expected an error from ParseEncryptedFile with no key, got nil")
	}
	if !strings.Contains(err.Error(), "masterKey required") {
		t.Fatalf("expected error mentioning 'masterKey required; use ParseEncryptedFileWithKey', got: %v", err)
	}
}

// TestParseEncryptedFileWithKey_Roundtrip asserts the keyed variant actually
// decrypts a sample export end-to-end (encrypt → parse → decrypt → plain SQL).
func TestParseEncryptedFileWithKey_Roundtrip(t *testing.T) {
	key := GenerateMasterKey()
	plain := "INSERT INTO tenant (id, name) VALUES (1, 'acme');\n"
	enc := encryptForTest(t, plain, key)

	c := &Crypter{}
	got, err := c.ParseEncryptedFileWithKey(enc, key)
	if err != nil {
		t.Fatalf("decrypt roundtrip failed: %v", err)
	}
	if got != plain {
		t.Fatalf("roundtrip mismatch:\nwant: %q\ngot:  %q", plain, got)
	}
}

// --- E8: crypto/rand.Read errors must not be swallowed ---

// TestGenerateMasterKey_NormalPath verifies the happy path: a base64-decodable
// 32-byte AES-256 key. The swallowed-error bug (rand.Read's err ignored,
// leaving a zero key) is not directly observable on the success path —
// crypto/rand.Read on Linux uses the getrandom syscall and cannot be reliably
// redirected via rand.Reader, so the failure path is verified by code
// inspection (see the panic-on-error in GenerateMasterKey).
func TestGenerateMasterKey_NormalPath(t *testing.T) {
	key := GenerateMasterKey()
	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatalf("master key not base64-decodable: %v", err)
	}
	if len(raw) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(raw))
	}
	// two draws must differ (CSPRNG sanity)
	k2 := GenerateMasterKey()
	if k2 == key {
		t.Fatal("two GenerateMasterKey calls returned identical keys")
	}
}

// TestGenerateIV_NormalPath asserts GenerateIV returns a 16-byte IV and no
// error. Against the old `func GenerateIV() []byte` signature this test does
// not even compile (the desired contract — return err — is absent), which is
// the red; after the signature change to ([]byte, error) it goes green.
func TestGenerateIV_NormalPath(t *testing.T) {
	iv, err := GenerateIV()
	if err != nil {
		t.Fatalf("GenerateIV returned error: %v", err)
	}
	if len(iv) != aes.BlockSize {
		t.Fatalf("expected IV of %d bytes, got %d", aes.BlockSize, len(iv))
	}
	// two draws must differ (CSPRNG sanity)
	iv2, err := GenerateIV()
	if err != nil {
		t.Fatalf("second GenerateIV returned error: %v", err)
	}
	if bytes.Equal(iv, iv2) {
		t.Fatal("two GenerateIV calls produced identical IVs")
	}
}
