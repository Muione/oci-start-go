// Package service — offline.go: offline instance detection (Phase 5).
// Port of CheckOfflineInstanceJob. Finds instances whose last heartbeat
// exceeds a threshold and marks them as offline. Notifies on state changes.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/notify"
	"github.com/Muione/oci-start-go/internal/repo"
)

// OfflineSvc detects and handles offline instances.
type OfflineSvc struct {
	store    *db.Store
	logger   zerolog.Logger
	notifier notify.Notifier
}

func NewOfflineSvc(store *db.Store, logger zerolog.Logger, notifier notify.Notifier) *OfflineSvc {
	return &OfflineSvc{store: store, logger: logger, notifier: notifier}
}

const offlineThresholdMinutes = 5 // minutes without heartbeat → offline

// CheckOfflineInstances finds instances that haven't reported a heartbeat
// within the threshold and marks them as offline. Runs via
// CheckOfflineInstanceJob (every 1 min).
func (s *OfflineSvc) CheckOfflineInstances(ctx context.Context) {
	cutoff := time.Now().Add(-offlineThresholdMinutes * time.Minute)
	offline, err := repo.New(s.store.Read).FindOfflineInstances(ctx,
		sql.NullString{String: cutoff.Format("2006-01-02 15:04:05"), Valid: true})
	if err != nil {
		s.logger.Error().Err(err).Msg("offline: find offline instances failed")
		return
	}

	q := repo.New(s.store.Write)
	for _, inst := range offline {
		if err := q.UpdateInstanceOffline(ctx, repo.UpdateInstanceOfflineParams{
			OnLineEnable:  0,
			OfflineNotify: 1,
			ID:            inst.ID,
		}); err != nil {
			s.logger.Error().Err(err).Int64("id", inst.ID).Msg("offline: mark offline failed")
			continue
		}

		s.logger.Info().
			Int64("id", inst.ID).
			Str("displayName", ns(inst.DisplayName)).
			Str("publicIp", ns(inst.PublicIps)).
			Msg("offline: instance marked offline")

		msg := fmt.Sprintf("[实例离线]\n名称: %s\nIP: %s",
			ns(inst.DisplayName), ns(inst.PublicIps))
		if s.notifier != nil {
			if sendErr := s.notifier.Send(ctx, msg); sendErr != nil {
				s.logger.Warn().Err(sendErr).Msg("offline: notify failed")
			}
		} else {
			s.logger.Debug().Str("telegramMsg", msg).Msg("offline: alert (notifier not configured)")
		}
	}

	if len(offline) > 0 {
		s.logger.Warn().Int("count", len(offline)).Msg("offline: instances marked offline")
	}
}
