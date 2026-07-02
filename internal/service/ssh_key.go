// Package service — ssh_key.go: stores SSH private keys AES-256-GCM encrypted
// at rest (master key) + resolves them by id for the WS SSH handler. The key
// content never leaves the backend; the UI only sees id/label/fingerprint.
// Consistent with the tenant-credential + root-password encryption model.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/util/crypto"
	"golang.org/x/crypto/ssh"
)

// SSHKeyService encrypts/decrypts SSH private keys against the master key and
// persists them via repo. The WS handler resolves a key by id at connect time.
type SSHKeyService struct {
	store     *db.Store
	masterKey []byte
}

// NewSSHKeyService constructs an SSHKeyService.
func NewSSHKeyService(store *db.Store, masterKey []byte) *SSHKeyService {
	return &SSHKeyService{store: store, masterKey: masterKey}
}

// SSHKeyView is the API-facing representation. It deliberately has NO key
// content field so List cannot leak the material to the frontend.
type SSHKeyView struct {
	ID          int64  `json:"id"`
	Label       string `json:"label"`
	Fingerprint string `json:"fingerprint"`
	CreatedAt   string `json:"createdAt,omitempty"`
}

// SSHKeyLister is the subset used by the httpapi handlers (create/list/delete).
type SSHKeyLister interface {
	Create(ctx context.Context, label, content, passphrase string) (int64, error)
	List(ctx context.Context) ([]SSHKeyView, error)
	Delete(ctx context.Context, id int64) error
}

// Create validates the key (parsing it to derive a fingerprint), encrypts the
// key + passphrase with the master key, and stores it. Returns the new id.
func (s *SSHKeyService) Create(ctx context.Context, label, content, passphrase string) (int64, error) {
	if label == "" {
		return 0, fmt.Errorf("label required")
	}
	if content == "" {
		return 0, fmt.Errorf("private key required")
	}
	fp, err := fingerprint(content, passphrase)
	if err != nil {
		return 0, fmt.Errorf("invalid private key: %w", err)
	}
	encKey, err := crypto.EncryptString(content, s.masterKey)
	if err != nil {
		return 0, fmt.Errorf("encrypt private key: %w", err)
	}
	encPass := ""
	if passphrase != "" {
		encPass, err = crypto.EncryptString(passphrase, s.masterKey)
		if err != nil {
			return 0, fmt.Errorf("encrypt passphrase: %w", err)
		}
	}
	return repo.New(s.store.Write).CreateSSHKey(ctx, label, encKey, encPass, fp)
}

// List returns all stored keys (id/label/fingerprint/createdAt) — no content.
func (s *SSHKeyService) List(ctx context.Context) ([]SSHKeyView, error) {
	rows, err := repo.New(s.store.Read).ListSSHKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("list ssh keys: %w", err)
	}
	views := make([]SSHKeyView, len(rows))
	for i, r := range rows {
		views[i] = SSHKeyView{
			ID:          r.ID,
			Label:       r.Label,
			Fingerprint: r.Fingerprint.String,
			CreatedAt:   timeToStr(r.CreatedAt),
		}
	}
	return views, nil
}

// Delete removes a stored key by id.
func (s *SSHKeyService) Delete(ctx context.Context, id int64) error {
	return repo.New(s.store.Write).DeleteSSHKey(ctx, id)
}

// Resolve returns the decrypted private key + passphrase for the WS SSH handler
// to use at connect time. The content stays server-side.
func (s *SSHKeyService) Resolve(ctx context.Context, id int64) (content, passphrase string, err error) {
	row, err := repo.New(s.store.Read).GetSSHKey(ctx, id)
	if err != nil {
		return "", "", fmt.Errorf("get ssh key %d: %w", id, err)
	}
	content = crypto.DecryptStringWithFallback(row.EncryptedKey.String, s.masterKey)
	passphrase = crypto.DecryptStringWithFallback(row.EncryptedPassphrase.String, s.masterKey)
	return content, passphrase, nil
}

// fingerprint parses the (possibly passphrase-protected) private key + returns
// the SSH public key's SHA256 fingerprint. Also validates the key (returns an
// error for malformed keys). Pure function — testable without DB.
func fingerprint(content, passphrase string) (string, error) {
	var signer ssh.Signer
	var err error
	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(content), []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey([]byte(content))
	}
	if err != nil {
		return "", err
	}
	return ssh.FingerprintSHA256(signer.PublicKey()), nil
}

// timeToStr formats a sql.NullTime for the API view (empty if NULL).
func timeToStr(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}
