// Package service — tenant_credentials.go: per-user IAM credential management
// (API keys, auth tokens, SMTP credentials, customer secret keys) plus
// domain-level sign-on policies and account recovery settings, scoped to a
// tenant. Mirrors TenantUserService: tenant creds → OCI provider → oci wrappers.
package service

import (
	"context"
	"fmt"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/oracle/oci-go-sdk/v65/common"
)

// TenantCredentialsService manages per-user IAM credentials and domain-level
// sign-on/recovery settings for a tenant.
type TenantCredentialsService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool
}

// NewTenantCredentialsService constructs a TenantCredentialsService.
func NewTenantCredentialsService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *TenantCredentialsService {
	return &TenantCredentialsService{store: store, masterKey: masterKey, pool: pool}
}

// provider resolves the tenant's OCI configuration provider and tenancy OCID.
// Collapses the getProvider + NewProvider dance repeated across the methods.
func (s *TenantCredentialsService) provider(ctx context.Context, tenantID int64) (common.ConfigurationProvider, string, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := tenantToCreds(t)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, "", fmt.Errorf("create provider: %w", err)
	}
	return prov, creds.Tenancy, nil
}

// ─── API Keys ───────────────────────────────────────────────────────────────

func (s *TenantCredentialsService) ListApiKeys(ctx context.Context, tenantID int64, userOCID string) ([]oci.ApiKeyInfo, error) {
	prov, tenancy, err := s.provider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return oci.ListApiKeys(ctx, prov, tenancy, userOCID)
}

func (s *TenantCredentialsService) CreateApiKey(ctx context.Context, tenantID int64, userOCID, pem string) (*oci.CreateApiKeyResult, error) {
	prov, tenancy, err := s.provider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return oci.CreateApiKey(ctx, prov, tenancy, userOCID, pem)
}

func (s *TenantCredentialsService) DeleteApiKey(ctx context.Context, tenantID int64, keyID string) error {
	prov, tenancy, err := s.provider(ctx, tenantID)
	if err != nil {
		return err
	}
	return oci.DeleteApiKey(ctx, prov, tenancy, keyID)
}

// ─── Auth Tokens ────────────────────────────────────────────────────────────

func (s *TenantCredentialsService) ListAuthTokens(ctx context.Context, tenantID int64, userOCID string) ([]oci.AuthTokenInfo, error) {
	prov, tenancy, err := s.provider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return oci.ListAuthTokens(ctx, prov, tenancy, userOCID)
}

func (s *TenantCredentialsService) CreateAuthToken(ctx context.Context, tenantID int64, userOCID, description string) (*oci.AuthTokenInfo, error) {
	prov, tenancy, err := s.provider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return oci.CreateAuthToken(ctx, prov, tenancy, userOCID, description)
}

func (s *TenantCredentialsService) DeleteAuthToken(ctx context.Context, tenantID int64, tokenID string) error {
	prov, tenancy, err := s.provider(ctx, tenantID)
	if err != nil {
		return err
	}
	return oci.DeleteAuthToken(ctx, prov, tenancy, tokenID)
}

// ─── SMTP Credentials ───────────────────────────────────────────────────────

func (s *TenantCredentialsService) ListSmtpCredentials(ctx context.Context, tenantID int64, userOCID string) ([]oci.SmtpCredentialInfo, error) {
	prov, tenancy, err := s.provider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return oci.ListSmtpCredentials(ctx, prov, tenancy, userOCID)
}

func (s *TenantCredentialsService) CreateSmtpCredential(ctx context.Context, tenantID int64, userOCID, description string) (*oci.SmtpCredentialInfo, error) {
	prov, tenancy, err := s.provider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return oci.CreateSmtpCredential(ctx, prov, tenancy, userOCID, description)
}

func (s *TenantCredentialsService) DeleteSmtpCredential(ctx context.Context, tenantID int64, credID string) error {
	prov, tenancy, err := s.provider(ctx, tenantID)
	if err != nil {
		return err
	}
	return oci.DeleteSmtpCredential(ctx, prov, tenancy, credID)
}

// ─── Customer Secret Keys ───────────────────────────────────────────────────

func (s *TenantCredentialsService) ListCustomerSecretKeys(ctx context.Context, tenantID int64, userOCID string) ([]oci.CustomerSecretKeyInfo, error) {
	prov, tenancy, err := s.provider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return oci.ListCustomerSecretKeys(ctx, prov, tenancy, userOCID)
}

func (s *TenantCredentialsService) CreateCustomerSecretKey(ctx context.Context, tenantID int64, userOCID, displayName string) (*oci.CustomerSecretKeyInfo, error) {
	prov, tenancy, err := s.provider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return oci.CreateCustomerSecretKey(ctx, prov, tenancy, userOCID, displayName)
}

func (s *TenantCredentialsService) DeleteCustomerSecretKey(ctx context.Context, tenantID int64, keyID string) error {
	prov, tenancy, err := s.provider(ctx, tenantID)
	if err != nil {
		return err
	}
	return oci.DeleteCustomerSecretKey(ctx, prov, tenancy, keyID)
}

// ─── Sign-on Policies & Account Recovery ────────────────────────────────────

func (s *TenantCredentialsService) ListSignOnPolicies(ctx context.Context, tenantID int64) ([]oci.SignOnPolicyInfo, error) {
	prov, tenancy, err := s.provider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return oci.ListSignOnPolicies(ctx, prov, tenancy)
}

func (s *TenantCredentialsService) GetAccountRecoverySetting(ctx context.Context, tenantID int64) (*oci.AccountRecoveryInfo, error) {
	prov, tenancy, err := s.provider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return oci.GetAccountRecoverySetting(ctx, prov, tenancy)
}

func (s *TenantCredentialsService) UpdateAccountRecoverySetting(ctx context.Context, tenantID int64, factors []string) (*oci.AccountRecoveryInfo, error) {
	prov, tenancy, err := s.provider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return oci.UpdateAccountRecoverySetting(ctx, prov, tenancy, factors)
}
