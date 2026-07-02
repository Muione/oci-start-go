// Package migration — Phase 8: H2 → SQLite data migration import.
// Imports plain SQL dumps and AES-256-CBC encrypted .enc files from the
// Java DatabaseExportService. Parity with Java DatabaseImportService.
package migration

import (
	"bufio"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"

	"github.com/rs/zerolog"
)

// Crypter handles AES-256-CBC decrypt and GZIP decompress matching the Java
// AesFileEncryptor + ZipUtils format.
type Crypter struct{}

// DecryptBytes decrypts base64-encoded AES-256-CBC ciphertext with the given
// base64-encoded key and raw IV. Returns the raw plaintext bytes.
func (c *Crypter) DecryptBytes(dataBase64 string, keyBase64 string, iv []byte) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid master key (base64): %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes (AES-256), got %d", len(key))
	}

	ciphertext, err := base64.StdEncoding.DecodeString(dataBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid ciphertext (base64): %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}

	if len(iv) != aes.BlockSize {
		return nil, fmt.Errorf("IV must be %d bytes, got %d", aes.BlockSize, len(iv))
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// PKCS5/7 unpad
	plaintext, err = pkcs7Unpad(plaintext)
	if err != nil {
		return nil, fmt.Errorf("invalid padding (possibly wrong master key): %w", err)
	}

	return plaintext, nil
}

// DecryptAndDecompress decrypts and then GZIP-decompresses, matching the Java
// exportEncryptedBackup pipeline: GZIP → AES256 encrypt.
func (c *Crypter) DecryptAndDecompress(dataBase64, keyBase64 string, iv []byte) (string, error) {
	compressed, err := c.DecryptBytes(dataBase64, keyBase64, iv)
	if err != nil {
		return "", err
	}

	reader, err := gzip.NewReader(strings.NewReader(string(compressed)))
	if err != nil {
		// Fallback: maybe it's uncompressed plain SQL (old format)
		return string(compressed), nil
	}
	defer reader.Close()

	var sb strings.Builder
	if _, err := io.Copy(&sb, reader); err != nil {
		return "", fmt.Errorf("gzip decompress: %w", err)
	}
	return sb.String(), nil
}

// GenerateMasterKey creates a random 32-byte AES-256 key, base64-encoded.
//
// The exported signature is kept (no error return) for backward compatibility;
// a CSPRNG failure is fatal and unrecoverable, so we panic rather than return a
// silently-zero key. ponytail: panic because the exported sig can't carry an
// error; if a caller needs the error, use internal/util/crypto.GenerateMasterKey.
func GenerateMasterKey() string {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("migration: crypto/rand Read failed: " + err.Error())
	}
	return base64.StdEncoding.EncodeToString(key)
}

// GenerateIV creates a random 16-byte IV. Returns an error if the CSPRNG fails
// so callers cannot silently end up with a zero IV.
func GenerateIV() ([]byte, error) {
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("generate IV: %w", err)
	}
	return iv, nil
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	paddingLen := int(data[len(data)-1])
	if paddingLen > aes.BlockSize || paddingLen == 0 {
		return nil, fmt.Errorf("invalid padding length: %d", paddingLen)
	}
	for i := 0; i < paddingLen; i++ {
		if data[len(data)-1-i] != byte(paddingLen) {
			return nil, fmt.Errorf("invalid padding byte at position %d", i)
		}
	}
	return data[:len(data)-paddingLen], nil
}

// SQLSplitter reads SQL lines from a Reader and yields complete INSERT
// statements (including multi-line PEM values).
type SQLSplitter struct {
	log      zerolog.Logger
	stats    ImportStats
}

// ImportStats tracks import progress.
type ImportStats struct {
	TotalLines    int64
	InsertLines   int64
	Inserted      int64
	Skipped       int64
	SkippedDups   int64
	SkippedUser   int64
	Errors        int64
	TablesFound   map[string]int64
}

// NewSQLSplitter creates a SQL splitter.
func NewSQLSplitter(log zerolog.Logger) *SQLSplitter {
	return &SQLSplitter{
		log: log,
		stats: ImportStats{
			TablesFound: make(map[string]int64),
		},
	}
}

// Stats returns a copy of the current import stats.
func (s *SQLSplitter) Stats() ImportStats {
	return ImportStats{
		TotalLines:    atomic.LoadInt64(&s.stats.TotalLines),
		InsertLines:   atomic.LoadInt64(&s.stats.InsertLines),
		Inserted:      atomic.LoadInt64(&s.stats.Inserted),
		Skipped:       atomic.LoadInt64(&s.stats.Skipped),
		SkippedDups:   atomic.LoadInt64(&s.stats.SkippedDups),
		SkippedUser:   atomic.LoadInt64(&s.stats.SkippedUser),
		Errors:        atomic.LoadInt64(&s.stats.Errors),
	}
}

// ParseEncryptedFile extracts IV and DATA from an .enc file and decrypts it.
//
// Deprecated: this entrypoint has no master key and therefore cannot actually
// decrypt. It returns a clear, actionable error directing callers to
// ParseEncryptedFileWithKey. Previously it silently forwarded an empty key and
// failed deep inside DecryptBytes with a misleading "master key must be 32
// bytes" message.
func (c *Crypter) ParseEncryptedFile(content string) (string, error) {
	return "", errors.New("masterKey required; use ParseEncryptedFileWithKey")
}

// ParseEncryptedFileWithKey is like ParseEncryptedFile but uses the provided
// master key for decryption.
func (c *Crypter) ParseEncryptedFileWithKey(content, masterKey string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var ivBase64, dataBase64 string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "IV:") {
			ivBase64 = strings.TrimSpace(strings.TrimPrefix(line, "IV:"))
		} else if strings.HasPrefix(line, "DATA:") {
			dataBase64 = strings.TrimSpace(strings.TrimPrefix(line, "DATA:"))
		}
	}

	if ivBase64 == "" || dataBase64 == "" {
		return "", fmt.Errorf("invalid encrypted file format: missing IV or DATA")
	}

	iv, err := base64.StdEncoding.DecodeString(ivBase64)
	if err != nil {
		return "", fmt.Errorf("invalid IV base64: %w", err)
	}

	return c.DecryptAndDecompress(dataBase64, masterKey, iv)
}
