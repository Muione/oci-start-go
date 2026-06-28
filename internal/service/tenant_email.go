// Package service — tenant_email.go: Phase 9 email service configuration
// per tenant (SES/SMTP settings).
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/repo"
)

// TenantEmailService manages tenant_email_config CRUD.
type TenantEmailService struct {
	store *db.Store
}

func NewTenantEmailService(store *db.Store) *TenantEmailService {
	return &TenantEmailService{store: store}
}

// EmailConfigResp is the API-facing email configuration.
type EmailConfigResp struct {
	ID           int64  `json:"id"`
	TenantID     int64  `json:"tenantId"`
	DomainName   string `json:"domainName"`
	SmtpUsername string `json:"smtpUsername"`
	SmtpPassword string `json:"smtpPassword"`
	SmtpHost     string `json:"smtpHost"`
	SmtpPort     string `json:"smtpPort"`
	SenderEmail  string `json:"senderEmail"`
	Active       bool   `json:"active"`
}

// EmailSaveInput carries the email config save payload.
type EmailSaveInput struct {
	TenantID     int64  `json:"tenantId"`
	DomainName   string `json:"domainName"`
	SmtpUsername string `json:"smtpUsername"`
	SmtpPassword string `json:"smtpPassword"`
	SmtpHost     string `json:"smtpHost"`
	SmtpPort     string `json:"smtpPort"`
	SenderEmail  string `json:"senderEmail"`
	Active       bool   `json:"active"`
}

// Get returns the email config for a tenant.
func (s *TenantEmailService) Get(ctx context.Context, tenantID int64) (*EmailConfigResp, error) {
	cfg, err := repo.New(s.store.Read).FindTenantEmailConfigByTenantId(ctx, nullInt64(tenantID))
	if err != nil {
		return nil, fmt.Errorf("find email config: %w", err)
	}
	return &EmailConfigResp{
		ID:           cfg.ID,
		TenantID:     ni(cfg.TenantID),
		DomainName:   ns(cfg.DomainName),
		SmtpUsername: ns(cfg.SmtpUsername),
		SmtpPassword: ns(cfg.SmtpPassword),
		SmtpHost:     ns(cfg.SmtpHost),
		SmtpPort:     ns(cfg.SmtpPort),
		SenderEmail:  ns(cfg.SenderEmail),
		Active:       ni(cfg.Active) != 0,
	}, nil
}

// Save creates or updates the email config for a tenant.
func (s *TenantEmailService) Save(ctx context.Context, in EmailSaveInput) error {
	now := time.Now().Format(httpTimeFmt)
	return repo.New(s.store.Write).UpsertTenantEmailConfig(ctx, repo.UpsertTenantEmailConfigParams{
		TenantID:     nullInt64(in.TenantID),
		DomainName:   nullStr(in.DomainName),
		SmtpUsername: nullStr(in.SmtpUsername),
		SmtpPassword: nullStr(in.SmtpPassword),
		SmtpHost:     nullStr(in.SmtpHost),
		SmtpPort:     nullStr(in.SmtpPort),
		SenderEmail:  nullStr(in.SenderEmail),
		Active:       nullInt64(boolToInt(in.Active)),
		CreatedTime:  nullStr(now),
	})
}

// SetActive enables or disables the email service for a tenant.
func (s *TenantEmailService) SetActive(ctx context.Context, tenantID int64, active bool) error {
	return repo.New(s.store.Write).UpdateTenantEmailActive(ctx, repo.UpdateTenantEmailActiveParams{
		TenantID: nullInt64(tenantID),
		Active:   nullInt64(boolToInt(active)),
	})
}

// Delete removes the email config for a tenant.
func (s *TenantEmailService) Delete(ctx context.Context, tenantID int64) error {
	return repo.New(s.store.Write).DeleteTenantEmailConfig(ctx, nullInt64(tenantID))
}
