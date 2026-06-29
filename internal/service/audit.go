// Package service — audit.go: Audit Log service (Phase 11.4).
// Looks up tenant credentials, builds OCI clients via proxy, and delegates
// to oci audit log wrappers. Supports both "recent days" and "date range" modes.
package service

import (
	"context"
	"fmt"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
)

// AuditLogRequest is the request body for the audit log endpoint.
type AuditLogRequest struct {
	Days      int    `json:"days"`       // 1-90, default 1
	StartDate string `json:"startDate"`  // "yyyy-MM-dd"
	EndDate   string `json:"endDate"`    // "yyyy-MM-dd"
	PageToken string `json:"pageToken"`
}

// AuditService provides audit log querying per tenant.
type AuditService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool
}

// NewAuditService constructs an AuditService.
func NewAuditService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *AuditService {
	return &AuditService{store: store, masterKey: masterKey, pool: pool}
}

// Query queries audit logs for a tenant. Supports both "recent days" and
// "date range" modes based on the request body.
func (s *AuditService) Query(ctx context.Context, tenantID int64, req AuditLogRequest) (*oci.AuditLogPage, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	creds := tenantToCreds(t)

	var result *oci.AuditLogPage
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(clients oci.Clients) error {
		var qErr error
		if req.StartDate != "" {
			endDate := req.EndDate
			if endDate == "" {
				endDate = req.StartDate
			}
			result, qErr = oci.ListAuditEventsByDateRange(ctx, clients, creds.Tenancy, req.StartDate, endDate, req.PageToken)
		} else {
			result, qErr = oci.ListRecentAuditEvents(ctx, clients, creds.Tenancy, req.Days, req.PageToken)
		}
		return qErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
