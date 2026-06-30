// Package sysconf wraps the system_config KV table with plain Go types,
// hiding sqlc's sql.Null* noise. Boolean configs use config_enabled (INTEGER 0/1);
// string configs use config_value. Mirrors Java SystemConfigService reads.
// See SPEC §7.2/§7.3 (mfa.*, turnstile.*, github.*, google.*).
package sysconf

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/repo"
)

// ProxyConfig represents the application-level outbound proxy configuration.
type ProxyConfig struct {
	Type     string `json:"type"`     // HTTP, HTTPS, SOCKS5
	Host     string `json:"host"`     // proxy host
	Port     int    `json:"port"`     // proxy port
	Username string `json:"username"` // optional auth username
	Password string `json:"password"` // optional auth password
	Enabled  bool   `json:"enabled"`  // whether proxy is active
}

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

// SetString upserts the config_value for key.
func (s *Service) SetString(ctx context.Context, key, value string) error {
	now := time.Now().Format(timeFmt)
	return repo.New(s.store.Write).UpsertConfigValue(ctx, repo.UpsertConfigValueParams{
		ConfigKey:     sql.NullString{String: key, Valid: true},
		ConfigValue:   sql.NullString{String: value, Valid: true},
		ConfigEnabled: sql.NullInt64{},
		LastModified:  sql.NullString{String: now, Valid: true},
	})
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

// GetProxyConfig reads the proxy configuration from system_config.
func (s *Service) GetProxyConfig(ctx context.Context) ProxyConfig {
	raw := s.GetString(ctx, "app.proxy.config")
	if raw == "" {
		return ProxyConfig{}
	}
	var cfg ProxyConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return ProxyConfig{}
	}
	return cfg
}

// SetProxyConfig writes the proxy configuration to system_config.
func (s *Service) SetProxyConfig(ctx context.Context, cfg ProxyConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.SetString(ctx, "app.proxy.config", string(data))
}

// IsProxyEnabled returns whether the application proxy is enabled.
func (s *Service) IsProxyEnabled(ctx context.Context) bool {
	cfg := s.GetProxyConfig(ctx)
	return cfg.Enabled && cfg.Host != "" && cfg.Port > 0
}
