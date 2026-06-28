// Package service — tenant_social.go: Phase 9 social login configuration
// per tenant (Google OAuth, GitHub OAuth, etc.).
package service

import (
	"context"
	"fmt"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/repo"
)

// TenantSocialService manages tenant_social CRUD.
type TenantSocialService struct {
	store *db.Store
}

func NewTenantSocialService(store *db.Store) *TenantSocialService {
	return &TenantSocialService{store: store}
}

// SocialConfigResp is the API-facing social config.
type SocialConfigResp struct {
	ID                int64  `json:"id"`
	TenantID          int64  `json:"tenantId"`
	Tenancy           string `json:"tenancy"`
	CloudType         int64  `json:"cloudType"`
	ClientID          string `json:"clientId"`
	ClientSecret      string `json:"clientSecret"`
	SocialTypeStr     string `json:"socialTypeStr"`
	ThirdLoginAddress string `json:"thirdLoginAddress"`
	RedirectUrl       string `json:"redirectUrl"`
	SocialStatus      string `json:"socialStatus"`
}

// SocialSaveInput carries the social config save payload.
type SocialSaveInput struct {
	ID                int64  `json:"id"`
	TenantID          int64  `json:"tenantId"`
	Tenancy           string `json:"tenancy"`
	CloudType         int64  `json:"cloudType"`
	ClientID          string `json:"clientId"`
	ClientSecret      string `json:"clientSecret"`
	SocialTypeStr     string `json:"socialTypeStr"`
	ThirdLoginAddress string `json:"thirdLoginAddress"`
	RedirectUrl       string `json:"redirectUrl"`
	SocialStatus      string `json:"socialStatus"`
}

// List returns all social configs for a tenant.
func (s *TenantSocialService) List(ctx context.Context, tenantID int64) ([]SocialConfigResp, error) {
	rows, err := repo.New(s.store.Read).ListTenantSocialByTenantId(ctx, nullInt64(tenantID))
	if err != nil {
		return nil, fmt.Errorf("list social configs: %w", err)
	}
	out := make([]SocialConfigResp, 0, len(rows))
	for _, r := range rows {
		out = append(out, SocialConfigResp{
			ID:                r.ID,
			TenantID:          ni(r.TenantID),
			Tenancy:           ns(r.Tenancy),
			CloudType:         ni(r.CloudType),
			ClientID:          ns(r.ClientID),
			ClientSecret:      ns(r.ClientSecret),
			SocialTypeStr:     ns(r.SocialTypeStr),
			ThirdLoginAddress: ns(r.ThirdLoginAddress),
			RedirectUrl:       ns(r.RedirectUrl),
			SocialStatus:      ns(r.SocialStatus),
		})
	}
	return out, nil
}

// Save creates or updates a social config.
func (s *TenantSocialService) Save(ctx context.Context, in SocialSaveInput) error {
	if in.ID > 0 {
		return repo.New(s.store.Write).UpdateTenantSocial(ctx, repo.UpdateTenantSocialParams{
			ID:                in.ID,
			ClientID:          nullStr(in.ClientID),
			ClientSecret:      nullStr(in.ClientSecret),
			RedirectUrl:       nullStr(in.RedirectUrl),
			ThirdLoginAddress: nullStr(in.ThirdLoginAddress),
		})
	}
	status := in.SocialStatus
	if status == "" {
		status = "enabled"
	}
	return repo.New(s.store.Write).InsertTenantSocial(ctx, repo.InsertTenantSocialParams{
		TenantID:          nullInt64(in.TenantID),
		Tenancy:           nullStr(in.Tenancy),
		CloudType:         nullInt64(in.CloudType),
		ClientID:          nullStr(in.ClientID),
		ClientSecret:      nullStr(in.ClientSecret),
		SocialTypeStr:     nullStr(in.SocialTypeStr),
		ThirdLoginAddress: nullStr(in.ThirdLoginAddress),
		RedirectUrl:       nullStr(in.RedirectUrl),
		SocialStatus:      nullStr(status),
	})
}

// SetStatus updates the status (enabled/disabled) of a social config.
func (s *TenantSocialService) SetStatus(ctx context.Context, id int64, status string) error {
	return repo.New(s.store.Write).UpdateTenantSocialStatus(ctx, repo.UpdateTenantSocialStatusParams{
		ID:           id,
		SocialStatus: nullStr(status),
	})
}

// Delete removes a social config.
func (s *TenantSocialService) Delete(ctx context.Context, id int64) error {
	return repo.New(s.store.Write).DeleteTenantSocial(ctx, id)
}
