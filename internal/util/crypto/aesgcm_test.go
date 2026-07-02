package crypto

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// testKey is a deterministic 32-byte AES-256 key for envelope tests.
var testKey = bytes.Repeat([]byte{0x42}, 32)

func TestEncryptDecryptString_RoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"ascii", "hello world"},
		{"pem", "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDx...\n-----END PRIVATE KEY-----"},
		{"unicode", "你好，世界🌍 — ñ ü é"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct, err := EncryptString(tc.in, testKey)
			if err != nil {
				t.Fatalf("EncryptString: %v", err)
			}
			if ct == tc.in && tc.in != "" {
				t.Error("ciphertext equals plaintext — envelope did not transform")
			}
			pt, err := DecryptString(ct, testKey)
			if err != nil {
				t.Fatalf("DecryptString: %v", err)
			}
			if pt != tc.in {
				t.Errorf("roundtrip mismatch: got %q want %q", pt, tc.in)
			}
		})
	}
}

func TestEncryptDecrypt_BytesRoundTrip(t *testing.T) {
	plain := []byte("raw bytes payload \x00\x01\x02")
	ct, err := Encrypt(plain, testKey)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	pt, err := Decrypt(ct, testKey)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(pt, plain) {
		t.Errorf("bytes mismatch: got %x want %x", pt, plain)
	}
}

func TestDecryptString_TamperedCiphertextFails(t *testing.T) {
	ct, err := EncryptString("sensitive", testKey)
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}
	// Flip one character in the base64 blob. Re-encode any valid base64
	// alteration so DecodeString still succeeds but GCM auth tag mismatches.
	raw, err := base64.StdEncoding.DecodeString(ct)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(raw) < 1 {
		t.Fatal("ciphertext too short to tamper")
	}
	raw[len(raw)-1] ^= 0xFF // flip last byte (part of GCM tag)
	tampered := base64.StdEncoding.EncodeToString(raw)

	if _, err := DecryptString(tampered, testKey); err == nil {
		t.Error("expected auth failure for tampered ciphertext, got nil")
	}
}

func TestDecrypt_ShortBlobFails(t *testing.T) {
	short := []byte{1, 2} // < nonce size (12)
	if _, err := Decrypt(short, testKey); err == nil {
		t.Error("expected error for short blob, got nil")
	}
}

func TestDecryptString_WrongKeyFails(t *testing.T) {
	ct, _ := EncryptString("secret", testKey)
	wrongKey := bytes.Repeat([]byte{0x99}, 32)
	if _, err := DecryptString(ct, wrongKey); err == nil {
		t.Error("expected auth failure for wrong key, got nil")
	}
}

func TestEncrypt_InvalidKeyLengthFails(t *testing.T) {
	// aes.NewCipher accepts only 16/24/32 bytes; 15 is invalid for any AES variant.
	if _, err := Encrypt([]byte("x"), bytes.Repeat([]byte{0x1}, 15)); err == nil {
		t.Error("expected error for 15-byte key, got nil")
	}
}

func TestDecryptString_InvalidBase64Fails(t *testing.T) {
	if _, err := DecryptString("!!!not base64!!!", testKey); err == nil {
		t.Error("expected base64 decode error, got nil")
	}
}

// ensure EncryptString output is base64 (text-safe) — guards TEXT storage path.
func TestEncryptString_OutputIsBase64(t *testing.T) {
	ct, err := EncryptString("x", testKey)
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}
	if _, err := base64.StdEncoding.DecodeString(ct); err != nil {
		t.Errorf("ciphertext not valid base64: %v", err)
	}
	if strings.ContainsAny(ct, "\x00\n\r") {
		t.Error("ciphertext contains control chars — unsafe for TEXT storage")
	}
}
