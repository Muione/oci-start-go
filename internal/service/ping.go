// Package service — ping.go: SSH/TCP connectivity check (Phase 5).
// Port of PingConnTimeTask.java. For each instance with enable_ping=1,
// try a TCP connect to port 22 and update the conn_time field.
// Single-flight guard prevents overlapping checks.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/repo"
)

// PingSvc checks instance connectivity.
type PingSvc struct {
	store         *db.Store
	logger        zerolog.Logger
	running       atomic.Bool
	securityRules *SecurityRuleService
}

func NewPingSvc(store *db.Store, logger zerolog.Logger) *PingSvc {
	return &PingSvc{store: store, logger: logger}
}

// SetSecurityRules injects the SecurityRuleService dependency for auto-recovery.
func (s *PingSvc) SetSecurityRules(sr *SecurityRuleService) {
	s.securityRules = sr
}

// CheckPingConn tests TCP connectivity to each instance's port 22 (SSH).
// Runs via PingConnTimeJob (every 5 min). Parity with PingConnTimeTask.
func (s *PingSvc) CheckPingConn(ctx context.Context) {
	if !s.running.CompareAndSwap(false, true) {
		s.logger.Debug().Msg("ping: previous check still running, skipping")
		return
	}
	defer s.running.Store(false)

	allRows, err := repo.New(s.store.Read).ListAllInstanceDetails(ctx, repo.ListAllInstanceDetailsParams{
		Limit:  500,
		Offset: 0,
	})
	if err != nil {
		s.logger.Error().Err(err).Msg("ping: list instances failed")
		return
	}

	now := time.Now()
	for _, inst := range allRows {
		if inst.EnablePing != 1 {
			continue
		}
		if !inst.PublicIps.Valid || inst.PublicIps.String == "" {
			continue
		}

		ip := inst.PublicIps.String
		port := int64(22)
		if inst.Port.Valid && inst.Port.Int64 > 0 {
			port = inst.Port.Int64
		}

		addr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			// Instance unreachable — if it was previously reachable, attempt auto-recovery.
			if inst.ConnTime > 0 && s.securityRules != nil {
				s.logger.Warn().Str("ip", ip).Int64("instanceId", inst.ID).
					Msg("ping: instance went offline, attempting auto-recovery")
				go s.attemptAutoRecovery(ctx, inst)
			}
			continue
		}
		conn.Close()

		_ = repo.New(s.store.Write).UpdateInstanceConnTime(ctx, repo.UpdateInstanceConnTimeParams{
			ConnTime:      now.Unix(),
			LastHeartbeat: sql.NullString{String: now.Format("2006-01-02 15:04:05"), Valid: true},
			ID:            inst.ID,
		})
	}

	s.logger.Debug().Int("instances", len(allRows)).Dur("elapsed", time.Since(now)).Msg("ping: check complete")
}

// attemptAutoRecovery tries to restore connectivity to a previously-reachable
// instance by opening all security list protocols and re-checking.
// Runs in a goroutine — does not block the ping loop.
// Parity with Java SecurityRuleServiceImpl.checkAndEnableRule recovery path.
func (s *PingSvc) attemptAutoRecovery(ctx context.Context, inst repo.ListAllInstanceDetailsRow) {
	if !inst.TenantID.Valid {
		return
	}

	// Open all protocols.
	if err := s.securityRules.CheckAndEnableRule(ctx, inst.TenantID.Int64); err != nil {
		s.logger.Error().Err(err).Msg("ping: auto-recovery: failed to open security rules")
		return
	}

	// Wait for OCI eventual consistency.
	time.Sleep(15 * time.Second)

	// Re-check connectivity.
	ip := inst.PublicIps.String
	port := int64(22)
	if inst.Port.Valid && inst.Port.Int64 > 0 {
		port = inst.Port.Int64
	}
	addr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		s.logger.Warn().Str("ip", ip).Msg("ping: auto-recovery: still unreachable")
		return
	}
	conn.Close()
	s.logger.Info().Str("ip", ip).Msg("ping: auto-recovery: instance is reachable again")
}
