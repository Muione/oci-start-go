// Package config loads config.yaml + env overrides (viper).
// Mirrors the Java application.yml key set; see SPEC §17.
package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

//go:embed default_config.yaml
var defaultConfig []byte

type Config struct {
	Server     ServerCfg     `mapstructure:"server"`
	Datasource DatasourceCfg `mapstructure:"datasource"`
	BaseFile   BaseFileCfg   `mapstructure:"base_file"`
	ThirdApi   ThirdApiCfg   `mapstructure:"third_api"`
	Oci        OciCfg        `mapstructure:"oci"`
	Ssl        SslCfg        `mapstructure:"ssl"`
	Turnstile  TurnstileCfg  `mapstructure:"turnstile"`
	SaToken    SaTokenCfg    `mapstructure:"sa_token"`
	Cache      CacheCfg      `mapstructure:"cache"`
	Logging    LoggingCfg    `mapstructure:"logging"`
	Deploy     DeployCfg     `mapstructure:"deploy"`
	Migrate    MigrateCfg    `mapstructure:"migrate"`

	dataDir string // unexported: resolved from DATA_PATH env or "./data"
}

type ServerCfg struct {
	Port           int `mapstructure:"port"`
	MaxHeaderBytes int `mapstructure:"max_header_bytes"`
}

type DatasourceCfg struct {
	Driver           string `mapstructure:"driver"`
	URL              string `mapstructure:"url"`
	MaxOpenConns     int    `mapstructure:"max_open_conns"`
	ReadMaxOpenConns int    `mapstructure:"read_max_open_conns"`
}

type BaseFileCfg struct {
	Path string `mapstructure:"path"`
}

type ThirdApiCfg struct {
	SpeedURL string `mapstructure:"speed_url"`
}

type OciCfg struct {
	Version    string `mapstructure:"version"`
	SshVersion string `mapstructure:"ssh_version"`
}

type SslCfg struct {
	Staging bool `mapstructure:"staging"`
}

type TurnstileCfg struct {
	Local TurnstileLocalCfg `mapstructure:"local"`
}

type TurnstileLocalCfg struct {
	Bypass bool `mapstructure:"bypass"`
}

type SaTokenCfg struct {
	Name          string `mapstructure:"name"`
	Timeout       int    `mapstructure:"timeout"`
	ActiveTimeout int    `mapstructure:"active_timeout"`
	IsConcurrent  bool   `mapstructure:"is_concurrent"`
	Style         string `mapstructure:"style"`
}

type CacheCfg struct {
	Type string    `mapstructure:"type"`
	Spec CacheSpec `mapstructure:"spec"`
}

type CacheSpec struct {
	InitialCapacity   int    `mapstructure:"initial_capacity"`
	MaximumSize       int    `mapstructure:"maximum_size"`
	ExpireAfterAccess string `mapstructure:"expire_after_access"`
	ExpireAfterWrite  string `mapstructure:"expire_after_write"`
}

type LoggingCfg struct {
	Level               string `mapstructure:"level"`
	LogHome             string `mapstructure:"log_home"`
	File                string `mapstructure:"file"`
	MaxSizeMB           int    `mapstructure:"max_size_mb"`
	MaxAgeDays          int    `mapstructure:"max_age_days"`
	TotalSizeCapGB      int    `mapstructure:"total_size_cap_gb"`
	CleanHistoryOnStart bool   `mapstructure:"clean_history_on_start"`
	PrettyConsole       bool   `mapstructure:"pretty_console"`
}

type DeployCfg struct {
	Type string `mapstructure:"type"`
}

type MigrateCfg struct {
	Path       string `mapstructure:"path"`
	AutoOnBoot bool   `mapstructure:"auto_on_boot"`
}

// Load reads config.yaml (cwd or /etc/oci-start), applies env overrides, and
// resolves the data directory (DATA_PATH or ./data). If no config file is found,
// the embedded default is written to ./config.yaml on first run.
func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/oci-start")

	v.SetEnvPrefix("OCI_START")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	// Explicit unprefixed bindings (env names that don't follow OCI_START_* convention).
	_ = v.BindEnv("server.port", "SERVER_PORT")
	_ = v.BindEnv("logging.log_home", "LOG_HOME")
	_ = v.BindEnv("deploy.type", "DEPLOY_TYPE")

	v.SetDefault("server.port", 9856)
	v.SetDefault("server.max_header_bytes", 81920)
	v.SetDefault("migrate.auto_on_boot", true)

	if err := v.ReadInConfig(); err != nil {
		// If config file not found, write the embedded default.
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if werr := os.WriteFile("config.yaml", defaultConfig, 0644); werr != nil {
				return nil, fmt.Errorf("config not found and failed to write default: %w", werr)
			}
			fmt.Println("📝 首次运行 — 已生成默认配置文件 config.yaml，请按需修改后重新启动")
			// Re-read the newly written config.
			if rerr := v.ReadInConfig(); rerr != nil {
				return nil, fmt.Errorf("read newly generated config: %w", rerr)
			}
		} else {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	cfg.applyDataPath()
	return &cfg, nil
}

// applyDataPath remaps the "./data" prefix across datasource.url, base_file.path
// when DATA_PATH is set, and records the resolved data dir for master-key placement.
func (c *Config) applyDataPath() {
	dp := os.Getenv("DATA_PATH")
	if dp == "" {
		c.dataDir = "./data"
		return
	}
	c.dataDir = dp
	c.Datasource.URL = strings.Replace(c.Datasource.URL, "./data", dp, 1)
	c.BaseFile.Path = strings.Replace(c.BaseFile.Path, "./data", dp, 1)
}

// DataDir returns the resolved data directory (./data or DATA_PATH override).
func (c *Config) DataDir() string { return c.dataDir }

// MasterKeyPath returns the master-key file path under the data directory.
func (c *Config) MasterKeyPath() string {
	return filepath.Join(c.dataDir, "master.key")
}
