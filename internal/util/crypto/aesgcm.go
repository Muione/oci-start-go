package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// Encrypt produces iv(12) || ciphertext || tag(16) for the given plaintext.
// Callers Base64-encode the result for TEXT storage via EncryptString.
func Encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ct...), nil
}

// Decrypt reverses Encrypt (expects iv-prepended blob).
func Decrypt(blob, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("ciphertext too short")
	}
	return gcm.Open(nil, blob[:ns], blob[ns:], nil)
}

// EncryptString returns a Base64(iv||ct||tag) string for TEXT storage,
// mirroring AesFileEncryptor.encrypt's signature.
func EncryptString(s string, key []byte) (string, error) {
	b, err := Encrypt([]byte(s), key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// DecryptString reverses EncryptString.
func DecryptString(b64 string, key []byte) (string, error) {
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	pt, err := Decrypt(b, key)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
