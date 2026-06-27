// Package rsakey provides the per-session RSA keypair store used for login
// password encryption (replaces Java's HTTP-session keypair). PKCS1v15 to match
// the jsencrypt client. See SPEC §7.2 / plan §3.
package rsakey

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type entry struct {
	priv     *rsa.PrivateKey
	pubB64   string
	expireAt time.Time
}

// KeypairStore holds short-lived per-session RSA keypairs keyed by a pre-login
// token (UUID, ~10min TTL). A background goroutine prunes expired entries.
type KeypairStore struct {
	m    sync.Map // token -> *entry
	ttl  time.Duration
	stop chan struct{}
	once sync.Once
}

func NewKeypairStore() *KeypairStore {
	s := &KeypairStore{ttl: 10 * time.Minute, stop: make(chan struct{})}
	go s.sweep()
	return s
}

func (s *KeypairStore) sweep() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			now := time.Now()
			s.m.Range(func(k, v any) bool {
				if e, ok := v.(*entry); ok && now.After(e.expireAt) {
					s.m.Delete(k)
				}
				return true
			})
		case <-s.stop:
			return
		}
	}
}

// Stop terminates the sweep goroutine (idempotent). Called on graceful shutdown.
func (s *KeypairStore) Stop() {
	s.once.Do(func() { close(s.stop) })
}

// Issue generates a 2048-bit keypair, stores it under a fresh UUID token, and
// returns the token + base64-encoded X.509 SPKI public key (what jsencrypt
// consumes via setPublicKey).
func (s *KeypairStore) Issue() (token, pubKeyBase64 string, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return "", "", err
	}
	pub := base64.StdEncoding.EncodeToString(der)
	token = uuid.NewString()
	s.m.Store(token, &entry{priv: priv, pubB64: pub, expireAt: time.Now().Add(s.ttl)})
	return token, pub, nil
}

// Decrypt looks up the keypair by token, base64-decodes the ciphertext (handling
// the Java " "→"+" quirk), RSA-PKCS1v15 decrypts, and returns the plaintext.
// Single-use: the entry is deleted on lookup (matches Java clearing RSA keys).
func (s *KeypairStore) Decrypt(token, cipherBase64 string) (string, error) {
	v, ok := s.m.LoadAndDelete(token)
	if !ok {
		return "", errors.New("invalid or expired pre-login token")
	}
	e := v.(*entry)
	if time.Now().After(e.expireAt) {
		return "", errors.New("expired pre-login token")
	}
	cipherBase64 = strings.ReplaceAll(cipherBase64, " ", "+")
	ct, err := base64.StdEncoding.DecodeString(cipherBase64)
	if err != nil {
		return "", err
	}
	pt, err := rsa.DecryptPKCS1v15(rand.Reader, e.priv, ct)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
