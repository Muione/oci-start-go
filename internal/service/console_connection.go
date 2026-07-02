// Package service — console_connection.go: persists + lists + deletes OCI
// instance console connections for the VNC console feature. The private key
// for an app-created connection is encrypted at rest (AES-256-GCM, master key)
// so a session can be resumed across app restarts without re-creating the OCI
// connection (which would 409 if the old one lingers).
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/util/crypto"
)

// isOciNotFound reports whether err is an OCI 404 (NotAuthorizedOrNotFound).
// Used by Delete to treat "already deleted" as success — OCI's
// ListInstanceConsoleConnections includes lingering DELETED connections, so a
// user clicking delete on one gets a 404; that's the desired end state, not an
// error. Mirrors oci.isNotFound (vnic.go) + isConsoleConflict (console.go)
// with errors.As to see through %w wraps + a string fallback.
func isOciNotFound(err error) bool {
	if err == nil {
		return false
	}
	type httpStatus interface {
		GetHTTPStatusCode() int
	}
	var hs httpStatus
	if errors.As(err, &hs) {
		return hs.GetHTTPStatusCode() == 404
	}
	msg := err.Error()
	return strings.Contains(msg, "404") && (strings.Contains(msg, "NotFound") || strings.Contains(msg, "NotAuthorized"))
}

// ConsoleConnectionService persists app-created console connections and lists /
// deletes them. buildClients builds OCI clients for a tenant (same helper the
// ws ConsoleHandler uses); nil in tests for Persist/Load-only paths.
type ConsoleConnectionService struct {
	store        *db.Store
	masterKey    []byte
	buildClients func(ctx context.Context, tenantID int64) (oci.Clients, error)
}

// NewConsoleConnectionService constructs a ConsoleConnectionService.
func NewConsoleConnectionService(store *db.Store, masterKey []byte, buildClients func(context.Context, int64) (oci.Clients, error)) *ConsoleConnectionService {
	return &ConsoleConnectionService{store: store, masterKey: masterKey, buildClients: buildClients}
}

// ConsoleConnectionView is the API-facing representation of one OCI console
// connection, annotated with whether this app created it and whether it can be
// resumed (ours + ACTIVE + we hold the key).
type ConsoleConnectionView struct {
	ConnID         string `json:"connId"`
	LifecycleState string `json:"lifecycleState"`
	IsOurs         bool   `json:"isOurs"`
	CanResume      bool   `json:"canResume"`
}

// ConsoleConnectionLister is the subset of ConsoleConnectionService used by the
// httpapi handlers (list + delete). An interface so handlers can be tested
// with a fake instead of a real ComputeClient.
type ConsoleConnectionLister interface {
	List(ctx context.Context, tenantID int64, compartmentID, instanceID string) ([]ConsoleConnectionView, error)
	Delete(ctx context.Context, tenantID int64, compartmentID, instanceID, connID string) error
}

// OCI op seams. ponytail: package vars defaulting to the real oci functions;
// overridable in tests so List/Delete can be exercised without network or real
// credentials. Mirrors the httpapi ws seam-var pattern.
var (
	listConsoleConnectionsFn  = oci.ListConsoleConnections
	deleteConsoleConnectionFn = oci.DeleteConsoleConnection
)

// Persist encrypts the private key PEM with the master key and upserts the
// console connection row for the instance (one per instance; latest creation
// wins). Called after a fresh OCI connection is created so the session can be
// resumed later.
func (s *ConsoleConnectionService) Persist(ctx context.Context, instanceID, connID string, tenantID int64, privateKeyPEM, publicKeySSH string) error {
	enc, err := crypto.EncryptString(privateKeyPEM, s.masterKey)
	if err != nil {
		return fmt.Errorf("encrypt console private key: %w", err)
	}
	return repo.New(s.store.Write).UpsertConsoleConnection(ctx, instanceID, tenantID, connID, enc, publicKeySSH)
}

// LoadForResume returns the connID + decrypted private key PEM for the
// instance's app-created console connection. Returns sql.ErrNoRows if the app
// has no tracked connection for the instance (caller prompts "create new").
func (s *ConsoleConnectionService) LoadForResume(ctx context.Context, instanceID string) (connID, privateKeyPEM string, err error) {
	row, err := repo.New(s.store.Read).GetConsoleConnectionByInstance(ctx, instanceID)
	if err != nil {
		return "", "", err
	}
	pem := crypto.DecryptStringWithFallback(row.EncryptedPrivateKey.String, s.masterKey)
	return row.ConnectionID, pem, nil
}

// List returns all OCI console connections for the instance, marking the one
// this app created (if any) as IsOurs and resumable.
func (s *ConsoleConnectionService) List(ctx context.Context, tenantID int64, compartmentID, instanceID string) ([]ConsoleConnectionView, error) {
	clients, err := s.buildClients(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("build clients: %w", err)
	}
	conns, err := listConsoleConnectionsFn(ctx, clients, compartmentID, instanceID)
	if err != nil {
		return nil, fmt.Errorf("list console connections: %w", err)
	}
	// Drop terminal DELETED connections — OCI's list includes lingering DELETED
	// rows (they eventually purge); showing them is noise + they can't be
	// re-deleted (404), which made "delete" look unresponsive.
	var alive []oci.ConsoleConnection
	for _, c := range conns {
		if c.LifecycleState != "DELETED" {
			alive = append(alive, c)
		}
	}
	row, err := repo.New(s.store.Read).GetConsoleConnectionByInstance(ctx, instanceID)
	var our *repo.ConsoleConnectionRow
	if err == nil {
		our = &row
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("load our connection row: %w", err)
	}
	return joinOurs(alive, our), nil
}

// joinOurs annotates OCI console connections with IsOurs / CanResume based on
// the app's persisted row. Pure (no I/O) so the marking logic is testable
// without a live ComputeClient.
func joinOurs(conns []oci.ConsoleConnection, our *repo.ConsoleConnectionRow) []ConsoleConnectionView {
	views := make([]ConsoleConnectionView, 0, len(conns))
	for _, c := range conns {
		v := ConsoleConnectionView{ConnID: c.ID, LifecycleState: c.LifecycleState}
		if our != nil && our.ConnectionID == c.ID {
			v.IsOurs = true
			// Resumable only if ACTIVE and we actually hold the encrypted key
			// (a legacy private_key_path row is ours-in-spirit but not resumable).
			if c.LifecycleState == "ACTIVE" && our.EncryptedPrivateKey.Valid {
				v.CanResume = true
			}
		}
		views = append(views, v)
	}
	return views
}

// Delete deletes the OCI console connection and, if it was the app's tracked
// row for the instance, removes that row so it's no longer advertised as
// resumable.
func (s *ConsoleConnectionService) Delete(ctx context.Context, tenantID int64, compartmentID, instanceID, connID string) error {
	clients, err := s.buildClients(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("build clients: %w", err)
	}
	if err := deleteConsoleConnectionFn(ctx, clients, connID); err != nil {
		// 404 = already deleted (a lingering DELETED row the user clicked).
		// That's the desired end state — fall through to drop our DB row + succeed.
		if !isOciNotFound(err) {
			return fmt.Errorf("delete console connection: %w", err)
		}
	}
	row, err := repo.New(s.store.Read).GetConsoleConnectionByInstance(ctx, instanceID)
	if err == nil && row.ConnectionID == connID {
		return repo.New(s.store.Write).DeleteConsoleConnectionByInstance(ctx, instanceID)
	}
	return nil
}
