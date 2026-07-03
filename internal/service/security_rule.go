// Package service -- security_rule.go: Phase 11.3 security list rule management.
// Service layer above the OCI wrapper (internal/oci/security_list.go).
// Resolves tenant from DB, builds OCI clients via oci.WithProxy, delegates to
// OCI wrapper functions. Parity with Java SecurityRuleService.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
)

// SecurityRuleService manages OCI security list rules for tenants.
type SecurityRuleService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool
}

// NewSecurityRuleService constructs a SecurityRuleService.
func NewSecurityRuleService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *SecurityRuleService {
	return &SecurityRuleService{store: store, masterKey: masterKey, pool: pool}
}

// GetRules lists security rules for a tenant. Looks up tenant by ID, builds
// OCI clients, calls oci.ListSecurityRules.
func (s *SecurityRuleService) GetRules(ctx context.Context, tenantID int64, ruleType string) ([]oci.SecurityRuleDTO, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := tenantToCreds(t)

	var rules []oci.SecurityRuleDTO
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		var innerErr error
		rules, innerErr = oci.ListSecurityRules(ctx, c, creds.Tenancy, ruleType)
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	return rules, nil
}

// AddRule adds/replaces a security rule for a tenant. Looks up tenant,
// builds OCI clients, calls oci.AddSecurityRule.
func (s *SecurityRuleService) AddRule(ctx context.Context, tenantID int64, rule oci.SecurityRuleDTO) error {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := tenantToCreds(t)

	return oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		return oci.AddSecurityRule(ctx, c, creds.Tenancy, rule)
	})
}

// DeleteRule deletes a security rule by composite ID. The composite ID format
// is "{tenantId}_{ruleIndex}_{type}". Looks up tenant by the embedded tenantId,
// builds OCI clients, calls oci.DeleteSecurityRule.
func (s *SecurityRuleService) DeleteRule(ctx context.Context, compositeID string) error {
	// Parse tenantId from composite ID.
	parts := strings.Split(compositeID, "_")
	if len(parts) < 3 {
		return fmt.Errorf("invalid composite ID: %s", compositeID)
	}
	tenantID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid tenant ID in composite ID: %s", parts[0])
	}

	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := tenantToCreds(t)

	return oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		return oci.DeleteSecurityRule(ctx, c, creds.Tenancy, compositeID)
	})
}

// BatchEnableAll iterates ALL tenants and calls SingleEnableAll for each.
// Returns success and fail counts. Parity with Java
// SecurityRuleService.batchAllSecurityRule.
func (s *SecurityRuleService) BatchEnableAll(ctx context.Context) (successCount, failCount int, err error) {
	rows, err := repo.New(s.store.Read).ListTenants(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("list tenants: %w", err)
	}

	for _, r := range rows {
		if err := s.SingleEnableAll(ctx, r.ID); err != nil {
			failCount++
			continue
		}
		successCount++
	}
	return successCount, failCount, nil
}

// SingleEnableAll calls oci.EnableAllForTenant + oci.EnableIPv6ForTenant
// for a single tenant, then updates tenant flags. Parity with Java
// SecurityRuleService.singleSecurityAllRule + singleIpv6Rule.
func (s *SecurityRuleService) SingleEnableAll(ctx context.Context, tenantID int64) error {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := tenantToCreds(t)

	return oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		// Enable all protocols (IPv4 + IPv6 + ICMP).
		if _, err := oci.EnableAllForTenant(ctx, c, creds.Tenancy); err != nil {
			return fmt.Errorf("enable all: %w", err)
		}

		// Also enable IPv6 (idempotent -- rules may already exist from EnableAllForTenant).
		_ = oci.EnableIPv6ForTenant(ctx, c, creds.Tenancy)

		// Update tenant flags.
		return repo.New(s.store.Write).UpdateTenantEnableFlags(ctx, repo.UpdateTenantEnableFlagsParams{
			EnableIcmp:        sql.NullInt64{Int64: 1, Valid: true},
			EnableAllProtocol: sql.NullInt64{Int64: 1, Valid: true},
			ID:                tenantID,
		})
	})
}

// CheckAndEnableRule opens all protocols if the tenant hasn't already been
// flagged. Idempotent -- skips if tenant.enableAllProtocol == 1.
// Parity with Java SecurityRuleServiceImpl.checkAndEnableRule.
func (s *SecurityRuleService) CheckAndEnableRule(ctx context.Context, tenantID int64) error {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}

	// Skip if already enabled (idempotent gate).
	if t.EnableAllProtocol.Valid && t.EnableAllProtocol.Int64 == 1 {
		return nil
	}

	return s.SingleEnableAll(ctx, tenantID)
}

// BatchEnableIPv6 iterates ALL tenants and enables IPv6 rules for each.
func (s *SecurityRuleService) BatchEnableIPv6(ctx context.Context) error {
	rows, err := repo.New(s.store.Read).ListTenants(ctx)
	if err != nil {
		return fmt.Errorf("list tenants: %w", err)
	}

	for _, r := range rows {
		if err := s.SingleEnableIPv6(ctx, r.ID); err != nil {
			// Log and continue with other tenants.
			continue
		}
	}
	return nil
}

// SingleEnableIPv6 enables IPv6 rules for a single tenant.
func (s *SecurityRuleService) SingleEnableIPv6(ctx context.Context, tenantID int64) error {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := tenantToCreds(t)

	return oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		return oci.EnableIPv6ForTenant(ctx, c, creds.Tenancy)
	})
}
