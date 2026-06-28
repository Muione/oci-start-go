// Package service — tenant_user.go: IAM user management per tenant.
// Lists/create/deletes OCI IAM users, manages password policies, MFA,
// and notification recipients for a given tenancy.
package service

import (
	"context"
	"fmt"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
)

// TenantUserService manages OCI IAM users within a tenant.
type TenantUserService struct {
	store     *db.Store
	masterKey []byte
}

func NewTenantUserService(store *db.Store, masterKey []byte) *TenantUserService {
	return &TenantUserService{store: store, masterKey: masterKey}
}

func (s *TenantUserService) getProvider(ctx context.Context, tenantID int64) (oci.Credentials, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return oci.Credentials{}, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	return tenantToCreds(t), nil
}

// ─── User CRUD ──────────────────────────────────────────────────────────

// ListUsers returns all IAM users for the tenant.
func (s *TenantUserService) ListUsers(ctx context.Context, tenantID int64) ([]oci.OciUser, error) {
	creds, err := s.getProvider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}
	return oci.ListUsers(ctx, prov, creds.Tenancy)
}

// CreateUser creates a new IAM user and returns the one-time password.
func (s *TenantUserService) CreateUser(ctx context.Context, tenantID int64, req oci.CreateUserRequest) (*oci.CreateUserResult, error) {
	creds, err := s.getProvider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}
	return oci.CreateUser(ctx, prov, creds.Tenancy, req)
}

// DeleteUser deletes an IAM user by OCID.
func (s *TenantUserService) DeleteUser(ctx context.Context, tenantID int64, userOCID string) error {
	creds, err := s.getProvider(ctx, tenantID)
	if err != nil {
		return err
	}
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}
	return oci.DeleteUser(ctx, prov, userOCID)
}

// ResetPassword resets the console password for an IAM user.
func (s *TenantUserService) ResetPassword(ctx context.Context, tenantID int64, userOCID string) (string, error) {
	creds, err := s.getProvider(ctx, tenantID)
	if err != nil {
		return "", err
	}
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return "", fmt.Errorf("create provider: %w", err)
	}
	return oci.ResetUserPassword(ctx, prov, userOCID)
}

// ListGroups returns all IAM groups for the tenant.
func (s *TenantUserService) ListGroups(ctx context.Context, tenantID int64) ([]oci.OciGroup, error) {
	creds, err := s.getProvider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}
	return oci.ListGroups(ctx, prov, creds.Tenancy)
}

// ─── Password Policy ────────────────────────────────────────────────────

// GetPasswordPolicy returns the authentication policy for the tenant.
func (s *TenantUserService) GetPasswordPolicy(ctx context.Context, tenantID int64) (*oci.PasswordPolicyDetail, error) {
	creds, err := s.getProvider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}
	return oci.GetPasswordPolicy(ctx, prov, creds.Tenancy)
}

// UpdatePasswordPolicy updates the authentication policy.
func (s *TenantUserService) UpdatePasswordPolicy(ctx context.Context, tenantID int64, enableExpiry bool, expiryDays int) error {
	creds, err := s.getProvider(ctx, tenantID)
	if err != nil {
		return err
	}
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}
	return oci.UpdatePasswordPolicy(ctx, prov, creds.Tenancy, enableExpiry, expiryDays)
}

// ─── MFA ────────────────────────────────────────────────────────────────

// GetMfaStatus returns the MFA configuration state.
func (s *TenantUserService) GetMfaStatus(ctx context.Context, tenantID int64) (*oci.MfaStatus, error) {
	creds, err := s.getProvider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}
	return oci.GetMfaStatus(ctx, prov, creds.Tenancy)
}

// ToggleEmailMFA enables or disables email MFA.
func (s *TenantUserService) ToggleEmailMFA(ctx context.Context, tenantID int64, enable bool) (bool, error) {
	creds, err := s.getProvider(ctx, tenantID)
	if err != nil {
		return false, err
	}
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return false, fmt.Errorf("create provider: %w", err)
	}
	return oci.ToggleEmailMFA(ctx, prov, creds.Tenancy, enable)
}

// ─── Notification Recipients ────────────────────────────────────────────

// GetNotificationRecipients returns current notification emails.
func (s *TenantUserService) GetNotificationRecipients(ctx context.Context, tenantID int64) ([]oci.NotifRecipient, error) {
	creds, err := s.getProvider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}
	return oci.GetNotificationRecipients(ctx, prov, creds.Tenancy)
}

// ─── Tenancy Auto-fetch ─────────────────────────────────────────────────

// UpdateAccountDetail fetches tenancy details from OCI and updates the local record.
func (s *TenantUserService) UpdateAccountDetail(ctx context.Context, tenantID int64) (*oci.TenancyDetail, error) {
	creds, err := s.getProvider(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}
	detail, err := oci.GetTenancyDetail(ctx, prov, creds.Tenancy)
	if err != nil {
		return nil, err
	}
	// Update the local tenant record with fetched data
	err = repo.New(s.store.Write).UpdateTenantFields(ctx, repo.UpdateTenantFieldsParams{
		TenancyName:  nullStr(detail.TenancyName),
		AccountType:  nullStr(detail.AccountType),
		EmailAddress: nullStr(detail.EmailAddress),
		ID:           tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("update tenant fields: %w", err)
	}
	return detail, nil
}
