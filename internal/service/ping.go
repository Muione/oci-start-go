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
	store   *db.Store
	logger  zerolog.Logger
	running atomic.Bool
}

func NewPingSvc(store *db.Store, logger zerolog.Logger) *PingSvc {
	return &PingSvc{store: store, logger: logger}
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
