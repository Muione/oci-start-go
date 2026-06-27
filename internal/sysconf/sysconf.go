// Package sysconf wraps the system_config KV table with plain Go types,
// hiding sqlc's sql.Null* noise. Boolean configs use config_enabled (INTEGER 0/1);
// string configs use config_value. Mirrors Java SystemConfigService reads.
// See SPEC §7.2/§7.3 (mfa.*, turnstile.*, github.*, google.*).
package sysconf

import (
	"context"
	"database/sql"
	"time"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/repo"
)

const timeFmt = "2006-01-02 15:04:05"

type Service struct {
	store *db.Store
}

func New(store *db.Store) *Service { return &Service{store: store} }

// GetString returns config_value for key ("" if absent).
func (s *Service) GetString(ctx context.Context, key string) string {
	cfg, err := repo.New(s.store.Read).FindConfigByKey(ctx, sql.NullString{String: key, Valid: true})
	if err != nil {
		return ""
	}
	return cfg.ConfigValue.String
}

// GetBool returns the config_enabled flag for key (false if absent).
func (s *Service) GetBool(ctx context.Context, key string) bool {
	cfg, err := repo.New(s.store.Read).FindConfigByKey(ctx, sql.NullString{String: key, Valid: true})
	if err != nil {
		return false
	}
	return cfg.ConfigEnabled.Valid && cfg.ConfigEnabled.Int64 != 0
}

// SetEnabled upserts the config_enabled flag for key (turnstile.enabled etc.).
func (s *Service) SetEnabled(ctx context.Context, key string, enabled bool) error {
	now := time.Now().Format(timeFmt)
	return repo.New(s.store.Write).UpsertConfigEnabled(ctx, repo.UpsertConfigEnabledParams{
		ConfigKey:     sql.NullString{String: key, Valid: true},
		ConfigValue:   sql.NullString{String: "", Valid: true},
		ConfigEnabled: sql.NullInt64{Int64: boolToInt64(enabled), Valid: true},
		LastModified:  sql.NullString{String: now, Valid: true},
	})
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
