// Package crypto provides master-key bootstrap + AES-256-GCM envelope encryption.
// Deliberate upgrade from Java AesFileEncryptor (AES/CBC/PKCS5Padding, IV passed
// separately) to AES-256-GCM with IV prepended to ciphertext. See SPEC §5.3.
// CBC-compat reader is deferred to Phase 8 (data migration re-encrypts CBC→GCM).
package crypto

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"crypto/rand"
)

const KeyLen = 32 // AES-256

// LoadMasterKey resolves the 32-byte master key.
// Precedence: env OCI_START_MASTER_KEY (base64 of 32 raw bytes) > keyPath file
// (32 raw bytes, 0600, auto-generated on first boot).
func LoadMasterKey(keyPath string) ([]byte, error) {
	if env := os.Getenv("OCI_START_MASTER_KEY"); env != "" {
		key, err := base64.StdEncoding.DecodeString(env)
		if err != nil {
			return nil, fmt.Errorf("decode OCI_START_MASTER_KEY: %w", err)
		}
		if len(key) != KeyLen {
			return nil, fmt.Errorf("master key must be %d bytes, got %d", KeyLen, len(key))
		}
		return key, nil
	}

	if key, err := os.ReadFile(keyPath); err == nil {
		if len(key) != KeyLen {
			return nil, fmt.Errorf("master key file must be %d bytes, got %d", KeyLen, len(key))
		}
		return key, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read master key: %w", err)
	}

	// Not present: generate and persist.
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return nil, fmt.Errorf("mkdir master key dir: %w", err)
	}
	key := make([]byte, KeyLen)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		return nil, fmt.Errorf("write master key: %w", err)
	}
	return key, nil
}

// GenerateMasterKey returns a base64-encoded 32-byte key (parity with
// AesFileEncryptor.generateMasterKey) — used by tooling/tests, not runtime boot.
func GenerateMasterKey() (string, error) {
	key := make([]byte, KeyLen)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
