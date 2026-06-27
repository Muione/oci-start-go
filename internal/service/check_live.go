// Package service — check_live.go: account live check (Phase 5).
// Port of DynamicDailyTask.java. Verifies that tenant OCI accounts are still
// active by checking the Identity service. Runs via CheckLiveJob (hourly).
package service

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/notify"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/oci/region"
	"github.com/Muione/oci-start-go/internal/repo"
)

// CheckLiveSvc checks account liveness for active tenants.
type CheckLiveSvc struct {
	store     *db.Store
	masterKey []byte
	logger    zerolog.Logger
	notifier  notify.Notifier
	running   atomic.Bool
}

func NewCheckLiveSvc(store *db.Store, masterKey []byte, logger zerolog.Logger, notifier notify.Notifier) *CheckLiveSvc {
	return &CheckLiveSvc{store: store, masterKey: masterKey, logger: logger, notifier: notifier}
}

// CheckAccountLive is the main entry point (hourly cron). It iterates all
// active tenants and verifies their account status by listing compartments
// (a lightweight OCI API call that fails if the account is suspended).
// Has a single-flight guard to prevent overlapping runs.
func (s *CheckLiveSvc) CheckAccountLive(ctx context.Context) {
	if !s.running.CompareAndSwap(false, true) {
		s.logger.Debug().Msg("checklive: previous check still running, skipping")
		return
	}
	defer s.running.Store(false)

	tenants, err := repo.New(s.store.Read).ListTenants(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("checklive: list tenants failed")
		return
	}

	now := time.Now()
	for _, t := range tenants {
		if !t.IsActive.Valid || t.IsActive.Int64 == 0 {
			continue
		}
		// Time-window gate: only check during configured hours (default 08:00-22:00).
		if !isInTimeWindow(now, 8, 22) {
			continue
		}

		// Load the full tenant to get key_file_blob for OCI provider.
		fullTenant, err := repo.New(s.store.Read).FindTenantByID(ctx, t.ID)
		if err != nil {
			continue
		}
		tenant := fullTenant
		s.checkTenant(ctx, tenant)
	}
}

func (s *CheckLiveSvc) checkTenant(ctx context.Context, tenant repo.Tenant) {
	creds := oci.Credentials{
		Tenancy:     ns(tenant.Tenancy),
		UserID:      ns(tenant.TenantID),
		Fingerprint: ns(tenant.Fingerprint),
		Region:      ns(tenant.Region),
		KeyFileBlob: ns(tenant.KeyFileBlob),
		KeyFile:     ns(tenant.KeyFile),
	}

	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		s.logger.Warn().Err(err).Str("tenancy", ns(tenant.Tenancy)).Msg("checklive: provider error")
		return
	}

	clients, err := oci.NewClients(prov)
	if err != nil {
		s.logger.Warn().Err(err).Str("tenancy", ns(tenant.Tenancy)).Msg("checklive: clients error")
		return
	}

	// List compartments as a lightweight liveness check.
	_, err = oci.ListCompartments(ctx, clients, ns(tenant.Tenancy))
	if err != nil {
		regionName := region.NameByCode(region.CodeByName(ns(tenant.Region)))
		s.logger.Warn().
			Err(err).
			Str("tenancy", ns(tenant.Tenancy)).
			Str("region", regionName).
			Msg("checklive: account may be suspended or unreachable")

		msg := fmt.Sprintf("[账号存活检查]\n用户: %s\n区域: %s\n状态: 可能已失效\n原因: %v",
			ns(tenant.UserName), regionName, err)
		if s.notifier != nil {
			if sendErr := s.notifier.Send(ctx, msg); sendErr != nil {
				s.logger.Warn().Err(sendErr).Msg("checklive: notify failed")
			}
		} else {
			s.logger.Debug().Str("telegramMsg", msg).Msg("checklive: alert (notifier not configured)")
		}
	}
}

// isInTimeWindow checks if the current hour is within [startHour, endHour].
func isInTimeWindow(t time.Time, startHour, endHour int) bool {
	h := t.Hour()
	return h >= startHour && h <= endHour
}
