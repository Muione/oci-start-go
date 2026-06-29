// Package service -- bastion.go: Phase 14.1 Bastion service.
// Orchestrates OCI bastion and session management via the proxy pool.
package service

import (
	"context"
	"fmt"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
)

// BastionService manages OCI bastion and session operations per tenant.
type BastionService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool
}

// NewBastionService constructs a BastionService.
func NewBastionService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *BastionService {
	return &BastionService{store: store, masterKey: masterKey, pool: pool}
}

// ListBastions returns all bastions in a compartment for the given tenant.
func (s *BastionService) ListBastions(ctx context.Context, tenantID int64, compartmentID string) ([]oci.BastionSummaryVO, error) {
	var result []oci.BastionSummaryVO
	err := s.withTenant(ctx, tenantID, func(c oci.Clients) error {
		var innerErr error
		result, innerErr = oci.ListBastions(ctx, c.Bastion, compartmentID)
		return innerErr
	})
	return result, err
}

// CreateSession creates a new bastion session (port-forwarding or managed SSH).
func (s *BastionService) CreateSession(ctx context.Context, tenantID int64, in oci.CreateSessionInput) (*oci.SessionVO, error) {
	var result *oci.SessionVO
	err := s.withTenant(ctx, tenantID, func(c oci.Clients) error {
		var innerErr error
		result, innerErr = oci.CreateSession(ctx, c.Bastion, in)
		return innerErr
	})
	return result, err
}

// ListSessions returns all sessions for a bastion.
func (s *BastionService) ListSessions(ctx context.Context, tenantID int64, bastionID string) ([]oci.SessionVO, error) {
	var result []oci.SessionVO
	err := s.withTenant(ctx, tenantID, func(c oci.Clients) error {
		var innerErr error
		result, innerErr = oci.ListSessions(ctx, c.Bastion, bastionID)
		return innerErr
	})
	return result, err
}

// GetSession returns a single bastion session by ID.
func (s *BastionService) GetSession(ctx context.Context, tenantID int64, sessionID string) (*oci.SessionVO, error) {
	var result *oci.SessionVO
	err := s.withTenant(ctx, tenantID, func(c oci.Clients) error {
		var innerErr error
		result, innerErr = oci.GetSession(ctx, c.Bastion, sessionID)
		return innerErr
	})
	return result, err
}

// DeleteSession deletes a bastion session by ID.
func (s *BastionService) DeleteSession(ctx context.Context, tenantID int64, sessionID string) error {
	return s.withTenant(ctx, tenantID, func(c oci.Clients) error {
		return oci.DeleteSession(ctx, c.Bastion, sessionID)
	})
}

// withTenant resolves tenant creds and runs fn inside the OCI proxy pool.
func (s *BastionService) withTenant(ctx context.Context, tenantID int64, fn func(oci.Clients) error) error {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("tenant %d not found: %w", tenantID, err)
	}
	creds := oci.Credentials{
		Tenancy:     nsStr(t.Tenancy),
		UserID:      nsStr(t.TenantID),
		Fingerprint: nsStr(t.Fingerprint),
		Region:      nsStr(t.Region),
		KeyFileBlob: nsStr(t.KeyFileBlob),
		KeyFile:     nsStr(t.KeyFile),
	}
	return oci.WithProxy(ctx, s.pool, creds, s.masterKey, fn)
}
