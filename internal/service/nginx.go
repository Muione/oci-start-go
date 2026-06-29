// Package service — nginx.go: Phase 12.1 NginxService. Combines proxy config
// CRUD, nginx config generation/apply with rollback, SSL certificate lifecycle
// (ACME via lego), OpenResty service management, and per-domain locking.
package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/registration"
	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/config"
	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/dns"
	"github.com/Muione/oci-start-go/internal/openresty"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/sysconf"
)

const timeFmt = "2006-01-02 15:04:05"

// NginxService manages proxy configs, nginx configs, SSL certificates,
// and OpenResty service lifecycle.
type NginxService struct {
	store     *db.Store
	openresty *openresty.Client
	cfg       *config.Config
	sc        *sysconf.Service
	dnsSvc    *dns.DnsService
	logger    zerolog.Logger

	// applyLock prevents concurrent nginx config applies.
	applyLock sync.Mutex

	// domainLock provides per-domain locking for SSL cert operations.
	// Map of domain -> *domainLockEntry.
	domainLock sync.Map

	// certBasePath is the base directory for storing certificates.
	certBasePath string
}

// domainLockEntry holds a mutex and timestamp for cleanup.
type domainLockEntry struct {
	mu       sync.Mutex
	acquired time.Time
}

// NewNginxService creates a NginxService.
func NewNginxService(store *db.Store, orClient *openresty.Client, cfg *config.Config, sc *sysconf.Service, dnsSvc *dns.DnsService, logger zerolog.Logger) *NginxService {
	certPath := filepath.Join(cfg.DataDir(), "cert")
	return &NginxService{
		store:        store,
		openresty:    orClient,
		cfg:          cfg,
		sc:           sc,
		dnsSvc:       dnsSvc,
		logger:       logger,
		certBasePath: certPath,
	}
}

// acquireDomainLock tries to acquire the per-domain lock. Returns false if
// already locked (another operation in progress).
func (s *NginxService) acquireDomainLock(domain string) bool {
	val, _ := s.domainLock.LoadOrStore(domain, &domainLockEntry{acquired: time.Now()})
	entry := val.(*domainLockEntry)
	// TryLock is Go 1.18+; use a channel-based approach for compatibility.
	locked := make(chan struct{}, 1)
	go func() {
		entry.mu.Lock()
		locked <- struct{}{}
	}()
	select {
	case <-locked:
		entry.acquired = time.Now()
		return true
	case <-time.After(100 * time.Millisecond):
		return false
	}
}

// releaseDomainLock releases the per-domain lock.
func (s *NginxService) releaseDomainLock(domain string) {
	if val, ok := s.domainLock.Load(domain); ok {
		entry := val.(*domainLockEntry)
		entry.mu.Unlock()
	}
}

// cleanupStaleDomainLocks removes lock entries older than 1 hour.
func (s *NginxService) cleanupStaleDomainLocks() {
	cutoff := time.Now().Add(-1 * time.Hour)
	s.domainLock.Range(func(key, value any) bool {
		entry := value.(*domainLockEntry)
		if entry.acquired.Before(cutoff) {
			s.domainLock.Delete(key)
		}
		return true
	})
}

// ---------------------------------------------------------------------------
// Proxy Config CRUD
// ---------------------------------------------------------------------------

// ProxyConfigInput is the DTO for creating/updating a proxy config.
type ProxyConfigInput struct {
	Domain              string `json:"domain"`
	TargetHost          string `json:"targetHost"`
	TargetPort          int64  `json:"targetPort"`
	Protocol            string `json:"protocol"`
	EnableSSL           bool   `json:"enableSsl"`
	EnableWebSocket     bool   `json:"enableWebSocket"`
	SSLCertificateID    *int64 `json:"sslCertificateId"`
	CustomConfig        string `json:"customConfig"`
	Remark              string `json:"remark"`
	LoadBalanceType     string `json:"loadBalanceType"`
	EnableHealthCheck   bool   `json:"enableHealthCheck"`
	HealthCheckPath     string `json:"healthCheckPath"`
	HealthCheckInterval int64  `json:"healthCheckInterval"`
	EnableRateLimit     bool   `json:"enableRateLimit"`
	RateLimit           int64  `json:"rateLimit"`
	EnableCache         bool   `json:"enableCache"`
	CacheTime           int64  `json:"cacheTime"`
}

// CreateProxyConfig creates a new proxy config, optionally creates a Cloudflare
// DNS record (best-effort), and generates a new nginx config draft.
func (s *NginxService) CreateProxyConfig(ctx context.Context, in ProxyConfigInput) error {
	if in.Domain == "" || in.TargetHost == "" || in.TargetPort == 0 {
		return fmt.Errorf("domain, targetHost, and targetPort are required")
	}

	// Check uniqueness.
	q := repo.New(s.store.Read)
	count, err := q.ExistsProxyConfigByDomain(ctx, in.Domain)
	if err != nil {
		return fmt.Errorf("check domain exists: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("domain already exists: %s", in.Domain)
	}

	now := time.Now().Format(timeFmt)
	protocol := in.Protocol
	if protocol == "" {
		protocol = "http"
	}
	lbt := in.LoadBalanceType
	if lbt == "" {
		lbt = "round_robin"
	}

	params := repo.InsertProxyConfigParams{
		Domain:              in.Domain,
		TargetHost:          in.TargetHost,
		TargetPort:          in.TargetPort,
		Protocol:            nullStr(protocol),
		EnableSsl:           nullInt64FromBool(in.EnableSSL),
		EnableWebsocket:     nullInt64FromBool(in.EnableWebSocket),
		SslCertificateID:    nullInt64Ptr(in.SSLCertificateID),
		ConfigStatus:        nullStr("PENDING"),
		SslStatus:           nullStr("NOT_CONFIGURED"),
		CustomConfig:        nullStr(in.CustomConfig),
		Remark:              nullStr(in.Remark),
		LoadBalanceType:     nullStr(lbt),
		EnableHealthCheck:   nullInt64FromBool(in.EnableHealthCheck),
		HealthCheckPath:     nullStr(in.HealthCheckPath),
		HealthCheckInterval: nullInt64(in.HealthCheckInterval),
		EnableRateLimit:     nullInt64FromBool(in.EnableRateLimit),
		RateLimit:           nullInt64(in.RateLimit),
		EnableCache:         nullInt64FromBool(in.EnableCache),
		CacheTime:           nullInt64(in.CacheTime),
		CreateTime:          nullStr(now),
		UpdateTime:          nullStr(now),
	}

	if err := repo.New(s.store.Write).InsertProxyConfig(ctx, params); err != nil {
		return fmt.Errorf("insert proxy config: %w", err)
	}

	// Best-effort Cloudflare DNS A record creation.
	go s.tryCreateCfDNSRecord(in.Domain)

	// Generate new nginx config draft.
	if _, err := s.GenerateNginxConfig(ctx); err != nil {
		s.logger.Warn().Err(err).Msg("nginx: auto-generate config after proxy create failed")
	}

	return nil
}

// UpdateProxyConfig updates an existing proxy config.
func (s *NginxService) UpdateProxyConfig(ctx context.Context, id int64, in ProxyConfigInput) error {
	if in.Domain == "" || in.TargetHost == "" || in.TargetPort == 0 {
		return fmt.Errorf("domain, targetHost, and targetPort are required")
	}

	qr := repo.New(s.store.Read)
	existing, err := qr.FindProxyConfigById(ctx, id)
	if err != nil {
		return fmt.Errorf("find proxy config: %w", err)
	}

	// If domain changed, check uniqueness and reset SSL.
	if existing.Domain != in.Domain {
		count, err := qr.ExistsProxyConfigByDomain(ctx, in.Domain)
		if err != nil {
			return fmt.Errorf("check domain exists: %w", err)
		}
		if count > 0 {
			return fmt.Errorf("domain already exists: %s", in.Domain)
		}
	}

	now := time.Now().Format(timeFmt)
	protocol := in.Protocol
	if protocol == "" {
		protocol = "http"
	}
	lbt := in.LoadBalanceType
	if lbt == "" {
		lbt = "round_robin"
	}

	sslCertID := existing.SslCertificateID
	enableSSL := existing.EnableSsl
	sslStatus := existing.SslStatus

	// If domain changed, reset SSL fields.
	if existing.Domain != in.Domain {
		sslCertID = sql.NullInt64{}
		enableSSL = sql.NullInt64{}
		sslStatus = nullStr("NOT_CONFIGURED")
	}

	params := repo.UpdateProxyConfigParams{
		Domain:              in.Domain,
		TargetHost:          in.TargetHost,
		TargetPort:          in.TargetPort,
		Protocol:            nullStr(protocol),
		EnableSsl:           enableSSL,
		EnableWebsocket:     nullInt64FromBool(in.EnableWebSocket),
		SslCertificateID:    sslCertID,
		ConfigStatus:        nullStr("PENDING"),
		SslStatus:           sslStatus,
		CustomConfig:        nullStr(in.CustomConfig),
		Remark:              nullStr(in.Remark),
		LoadBalanceType:     nullStr(lbt),
		EnableHealthCheck:   nullInt64FromBool(in.EnableHealthCheck),
		HealthCheckPath:     nullStr(in.HealthCheckPath),
		HealthCheckInterval: nullInt64(in.HealthCheckInterval),
		EnableRateLimit:     nullInt64FromBool(in.EnableRateLimit),
		RateLimit:           nullInt64(in.RateLimit),
		EnableCache:         nullInt64FromBool(in.EnableCache),
		CacheTime:           nullInt64(in.CacheTime),
		UpdateTime:          nullStr(now),
		ID:                  id,
	}

	if err := repo.New(s.store.Write).UpdateProxyConfig(ctx, params); err != nil {
		return fmt.Errorf("update proxy config: %w", err)
	}

	if _, err := s.GenerateNginxConfig(ctx); err != nil {
		s.logger.Warn().Err(err).Msg("nginx: auto-generate config after proxy update failed")
	}

	return nil
}

// DeleteProxyConfig deletes a proxy config by ID.
func (s *NginxService) DeleteProxyConfig(ctx context.Context, id int64) error {
	if err := repo.New(s.store.Write).DeleteProxyConfig(ctx, id); err != nil {
		return fmt.Errorf("delete proxy config: %w", err)
	}
	if _, err := s.GenerateNginxConfig(ctx); err != nil {
		s.logger.Warn().Err(err).Msg("nginx: auto-generate config after proxy delete failed")
	}
	return nil
}

// GetProxyConfig returns a proxy config by ID.
func (s *NginxService) GetProxyConfig(ctx context.Context, id int64) (repo.ProxyConfig, error) {
	return repo.New(s.store.Read).FindProxyConfigById(ctx, id)
}

// ListProxyConfigs returns a paginated list of proxy configs.
func (s *NginxService) ListProxyConfigs(ctx context.Context, page, size int64) ([]repo.ProxyConfig, int64, error) {
	if page < 0 {
		page = 0
	}
	if size <= 0 {
		size = 20
	}
	qr := repo.New(s.store.Read)
	total, err := qr.CountProxyConfigs(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count proxy configs: %w", err)
	}
	rows, err := qr.ListProxyConfigs(ctx, repo.ListProxyConfigsParams{
		Limit:  size,
		Offset: page * size,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list proxy configs: %w", err)
	}
	return rows, total, nil
}

// BatchDeleteProxyConfigs deletes multiple proxy configs by ID.
func (s *NginxService) BatchDeleteProxyConfigs(ctx context.Context, ids []int64) error {
	w := repo.New(s.store.Write)
	for _, id := range ids {
		if err := w.DeleteProxyConfig(ctx, id); err != nil {
			s.logger.Warn().Int64("id", id).Err(err).Msg("nginx: batch delete proxy config failed")
		}
	}
	if _, err := s.GenerateNginxConfig(ctx); err != nil {
		s.logger.Warn().Err(err).Msg("nginx: auto-generate config after batch delete failed")
	}
	return nil
}

// ToggleProxyConfig enables or disables a proxy config.
func (s *NginxService) ToggleProxyConfig(ctx context.Context, id int64, enabled bool) error {
	status := "DISABLED"
	if enabled {
		status = "PENDING"
	}
	now := time.Now().Format(timeFmt)
	if err := repo.New(s.store.Write).UpdateProxyConfigStatus(ctx, repo.UpdateProxyConfigStatusParams{
		ConfigStatus: nullStr(status),
		UpdateTime:   nullStr(now),
		ID:           id,
	}); err != nil {
		return fmt.Errorf("toggle proxy config: %w", err)
	}
	if _, err := s.GenerateNginxConfig(ctx); err != nil {
		s.logger.Warn().Err(err).Msg("nginx: auto-generate config after toggle failed")
	}
	return nil
}

// TestProxyConnection tests TCP connectivity to a proxy config's target.
func (s *NginxService) TestProxyConnection(ctx context.Context, id int64) (bool, error) {
	pc, err := repo.New(s.store.Read).FindProxyConfigById(ctx, id)
	if err != nil {
		return false, fmt.Errorf("find proxy config: %w", err)
	}
	addr := fmt.Sprintf("%s:%d", pc.TargetHost, pc.TargetPort)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return false, nil // unreachable, but not an error
	}
	conn.Close()
	return true, nil
}

// ApplySslToProxy requests an SSL certificate for a proxy config's domain
// and binds it to the proxy.
func (s *NginxService) ApplySslToProxy(ctx context.Context, id int64, email string) error {
	pc, err := repo.New(s.store.Read).FindProxyConfigById(ctx, id)
	if err != nil {
		return fmt.Errorf("find proxy config: %w", err)
	}

	// Request certificate.
	cert, err := s.RequestCertificate(ctx, CertificateRequestInput{
		Domain:          pc.Domain,
		Email:           email,
		CertificateType: "LETS_ENCRYPT",
		DNSProvider:     "CLOUDFLARE",
		ValidationMethod: "dns",
		AutoRenew:       true,
	})
	if err != nil {
		// Mark SSL status as ERROR on the proxy.
		now := time.Now().Format(timeFmt)
		_ = repo.New(s.store.Write).UpdateProxyConfigSslFields(ctx, repo.UpdateProxyConfigSslFieldsParams{
			SslCertificateID: sql.NullInt64{},
			EnableSsl:        sql.NullInt64{},
			SslStatus:        nullStr("ERROR"),
			UpdateTime:       nullStr(now),
			ID:               id,
		})
		return fmt.Errorf("request certificate: %w", err)
	}

	// Bind cert to proxy.
	now := time.Now().Format(timeFmt)
	return repo.New(s.store.Write).UpdateProxyConfigSslFields(ctx, repo.UpdateProxyConfigSslFieldsParams{
		SslCertificateID: sql.NullInt64{Int64: cert.ID, Valid: true},
		EnableSsl:        sql.NullInt64{Int64: 1, Valid: true},
		SslStatus:        nullStr("PENDING"),
		UpdateTime:       nullStr(now),
		ID:               id,
	})
}

// FixProxyConfig resets a proxy config's status to PENDING for re-generation.
func (s *NginxService) FixProxyConfig(ctx context.Context, id int64) error {
	now := time.Now().Format(timeFmt)
	return repo.New(s.store.Write).UpdateProxyConfigStatus(ctx, repo.UpdateProxyConfigStatusParams{
		ConfigStatus: nullStr("PENDING"),
		UpdateTime:   nullStr(now),
		ID:           id,
	})
}

// ---------------------------------------------------------------------------
// Nginx Config Management
// ---------------------------------------------------------------------------

// NginxConfigDTO is the API representation of an nginx_config row.
type NginxConfigDTO struct {
	ID            int64  `json:"id"`
	ConfigName    string `json:"configName"`
	ConfigContent string `json:"configContent"`
	IsCurrent     bool   `json:"isCurrent"`
	ConfigVersion int64  `json:"configVersion"`
	ConfigStatus  string `json:"configStatus"`
	CreateTime    string `json:"createTime"`
	UpdateTime    string `json:"updateTime"`
}

// GenerateNginxConfig compiles all active proxy configs into a new nginx_config
// draft. If the content matches the latest version, returns the existing one.
func (s *NginxService) GenerateNginxConfig(ctx context.Context) (*NginxConfigDTO, error) {
	qr := repo.New(s.store.Read)
	qw := repo.New(s.store.Write)

	// Get all active proxy configs.
	configs, err := qr.ListActiveProxyConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active proxy configs: %w", err)
	}

	// Generate config content.
	content := GenerateFullNginxConfig(configs)

	// Check if content matches the latest version.
	latest, err := qr.FindLatestNginxConfig(ctx)
	if err == nil && latest.ConfigContent.Valid && latest.ConfigContent.String == content {
		// Content unchanged, return existing.
		return nginxConfigToDTO(latest), nil
	}

	// Determine next version number.
	var nextVersion int64 = 1
	if err == nil && latest.ConfigVersion.Valid {
		nextVersion = latest.ConfigVersion.Int64 + 1
	}

	now := time.Now().Format(timeFmt)
	configName := fmt.Sprintf("auto-generated-%s", time.Now().Format("20060102150405"))

	id, err := qw.InsertNginxConfig(ctx, repo.InsertNginxConfigParams{
		ConfigName:    nullStr(configName),
		ConfigContent: nullStr(content),
		IsCurrent:     nullInt64(0),
		ConfigVersion: nullInt64(nextVersion),
		ConfigStatus:  nullStr("DRAFT"),
		CreateTime:    nullStr(now),
		UpdateTime:    nullStr(now),
	})
	if err != nil {
		return nil, fmt.Errorf("insert nginx config: %w", err)
	}

	return &NginxConfigDTO{
		ID:            id,
		ConfigName:    configName,
		ConfigContent: content,
		IsCurrent:     false,
		ConfigVersion: nextVersion,
		ConfigStatus:  "DRAFT",
		CreateTime:    now,
		UpdateTime:    now,
	}, nil
}

// ApplyNginxConfig applies a nginx config to OpenResty with rollback on failure.
// Uses a global mutex to prevent concurrent applies.
func (s *NginxService) ApplyNginxConfig(ctx context.Context, id int64) error {
	// Try to acquire apply lock with 30s timeout.
	locked := make(chan struct{}, 1)
	go func() {
		s.applyLock.Lock()
		locked <- struct{}{}
	}()
	select {
	case <-locked:
		defer s.applyLock.Unlock()
	case <-time.After(30 * time.Second):
		return fmt.Errorf("config apply in progress, please try again later")
	}

	qr := repo.New(s.store.Read)
	qw := repo.New(s.store.Write)

	// Load the target config.
	target, err := qr.FindNginxConfigById(ctx, id)
	if err != nil {
		return fmt.Errorf("find nginx config: %w", err)
	}
	if !target.ConfigContent.Valid || target.ConfigContent.String == "" {
		return fmt.Errorf("config content is empty")
	}

	// Load current config (may not exist on first apply).
	var oldConfig *repo.NginxConfig
	current, err := qr.FindCurrentNginxConfig(ctx)
	if err == nil {
		oldConfig = &current
	}

	// Step 1: Test config syntax.
	valid, msg, err := s.openresty.TestConfig(ctx, target.ConfigContent.String)
	if err != nil {
		return fmt.Errorf("test config: %w", err)
	}
	if !valid {
		return fmt.Errorf("config syntax test failed: %s", msg)
	}

	// Step 2: Push config to OpenResty.
	if err := s.openresty.PushConfig(ctx, target.ConfigContent.String); err != nil {
		return fmt.Errorf("push config: %w", err)
	}

	// Step 3: Reload OpenResty.
	if err := s.openresty.Reload(ctx); err != nil {
		// Rollback: push old config and reload.
		s.logger.Error().Err(err).Msg("nginx: reload failed, attempting rollback")
		if oldConfig != nil && oldConfig.ConfigContent.Valid {
			if rbErr := s.openresty.PushConfig(ctx, oldConfig.ConfigContent.String); rbErr != nil {
				s.logger.Error().Err(rbErr).Msg("nginx: rollback push failed")
			}
			if rbErr := s.openresty.Reload(ctx); rbErr != nil {
				s.logger.Error().Err(rbErr).Msg("nginx: rollback reload failed")
			}
		}
		return fmt.Errorf("reload nginx: %w", err)
	}

	// Step 4: Update DB (separate from OpenResty operations).
	now := time.Now().Format(timeFmt)

	// Clear old current.
	if err := qw.ClearCurrentNginxConfig(ctx, nullStr(now)); err != nil {
		s.logger.Warn().Err(err).Msg("nginx: clear current config failed")
	}

	// Mark new as current.
	if err := qw.MarkNginxConfigCurrent(ctx, repo.MarkNginxConfigCurrentParams{
		UpdateTime: nullStr(now),
		ID:         id,
	}); err != nil {
		return fmt.Errorf("mark config current: %w", err)
	}

	// Update proxy config statuses to APPLIED.
	activeConfigs, err := qr.ListActiveProxyConfigs(ctx)
	if err == nil {
		for _, pc := range activeConfigs {
			if pc.ConfigStatus.Valid && pc.ConfigStatus.String == "PENDING" {
				_ = qw.UpdateProxyConfigStatus(ctx, repo.UpdateProxyConfigStatusParams{
					ConfigStatus: nullStr("APPLIED"),
					UpdateTime:   nullStr(now),
					ID:           pc.ID,
				})
			}
		}
	}

	return nil
}

// TestNginxConfig tests a nginx config's syntax via the OpenResty API.
func (s *NginxService) TestNginxConfig(ctx context.Context, id int64) (bool, error) {
	cfg, err := repo.New(s.store.Read).FindNginxConfigById(ctx, id)
	if err != nil {
		return false, fmt.Errorf("find nginx config: %w", err)
	}
	if !cfg.ConfigContent.Valid {
		return false, fmt.Errorf("config content is empty")
	}
	valid, msg, err := s.openresty.TestConfig(ctx, cfg.ConfigContent.String)
	if err != nil {
		return false, err
	}
	if !valid {
		return false, fmt.Errorf("syntax test failed: %s", msg)
	}
	return true, nil
}

// ReloadNginx sends a reload command to OpenResty.
func (s *NginxService) ReloadNginx(ctx context.Context) error {
	return s.openresty.Reload(ctx)
}

// GetConfigDiff returns a line-level diff between the current and latest configs.
func (s *NginxService) GetConfigDiff(ctx context.Context) (map[string]any, error) {
	qr := repo.New(s.store.Read)

	latest, err := qr.FindLatestNginxConfig(ctx)
	if err != nil {
		return map[string]any{
			"message": "No generated config",
		}, nil
	}

	current, err := qr.FindCurrentNginxConfig(ctx)
	if err != nil {
		return map[string]any{
			"latest":  nginxConfigToDTO(latest),
			"message": "First config, nothing applied yet",
		}, nil
	}

	if current.ID == latest.ID {
		return map[string]any{
			"current": nginxConfigToDTO(current),
			"latest":  nginxConfigToDTO(latest),
			"message": "Config is up to date",
		}, nil
	}

	diff := buildLineDiff(
		nullStrVal(current.ConfigContent),
		nullStrVal(latest.ConfigContent),
	)

	return map[string]any{
		"current": nginxConfigToDTO(current),
		"latest":  nginxConfigToDTO(latest),
		"diff":    diff,
	}, nil
}

// GetNginxStatus returns the nginx config status (has changes, versions).
func (s *NginxService) GetNginxStatus(ctx context.Context) (map[string]any, error) {
	qr := repo.New(s.store.Read)

	latest, err := qr.FindLatestNginxConfig(ctx)
	if err != nil {
		return map[string]any{
			"hasChanges":     false,
			"currentVersion": 0,
			"latestVersion":  0,
		}, nil
	}

	currentVersion := int64(0)
	current, err := qr.FindCurrentNginxConfig(ctx)
	if err == nil && current.ConfigVersion.Valid {
		currentVersion = current.ConfigVersion.Int64
	}

	latestVersion := int64(0)
	if latest.ConfigVersion.Valid {
		latestVersion = latest.ConfigVersion.Int64
	}

	return map[string]any{
		"hasChanges":     currentVersion != latestVersion,
		"currentVersion": currentVersion,
		"latestVersion":  latestVersion,
	}, nil
}

// GetLatestNginxConfig returns the latest nginx config by version.
func (s *NginxService) GetLatestNginxConfig(ctx context.Context) (*NginxConfigDTO, error) {
	cfg, err := repo.New(s.store.Read).FindLatestNginxConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("find latest config: %w", err)
	}
	return nginxConfigToDTO(cfg), nil
}

// ---------------------------------------------------------------------------
// SSL Certificate Lifecycle
// ---------------------------------------------------------------------------

// CertificateRequestInput is the DTO for requesting a certificate.
type CertificateRequestInput struct {
	Domain           string `json:"domain"`
	Email            string `json:"email"`
	CertificateType  string `json:"certificateType"`
	DNSProvider      string `json:"dnsProvider"`
	ValidationMethod string `json:"validationMethod"`
	AutoRenew        bool   `json:"autoRenew"`
}

// CertificateDTO is the API representation for certificate match results.
type CertificateDTO struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	CertPath  string `json:"certPath"`
	KeyPath   string `json:"keyPath"`
	ExpiryDate string `json:"expiryDate"`
}

// SslCertificateDTO is the API representation of an ssl_certificate row.
type SslCertificateDTO struct {
	ID                int64  `json:"id"`
	Domain            string `json:"domain"`
	CertificateType   string `json:"certificateType"`
	Email             string `json:"email"`
	ValidationMethod  string `json:"validationMethod"`
	AutoRenew         bool   `json:"autoRenew"`
	CertificateStatus string `json:"certificateStatus"`
	IssueDate         string `json:"issueDate"`
	ExpireDate        string `json:"expireDate"`
	CertificatePath   string `json:"certificatePath"`
	PrivateKeyPath    string `json:"privateKeyPath"`
	CreateTime        string `json:"createTime"`
	UpdateTime        string `json:"updateTime"`
	DNSProvider       string `json:"dnsProvider"`
}

// RequestCertificate initiates an ACME certificate request. The actual ACME
// flow runs in a background goroutine.
func (s *NginxService) RequestCertificate(ctx context.Context, in CertificateRequestInput) (*SslCertificateDTO, error) {
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required")
	}

	// Check for existing valid/pending cert.
	qr := repo.New(s.store.Read)
	existing, err := qr.FindSslCertificateByDomain(ctx, in.Domain)
	if err == nil {
		status := nullStrVal(existing.CertificateStatus)
		if status == "VALID" || status == "PENDING" {
			return sslCertToDTO(existing), nil
		}
	}

	// Acquire per-domain lock.
	if !s.acquireDomainLock(in.Domain) {
		return nil, fmt.Errorf("certificate operation already in progress for domain: %s", in.Domain)
	}

	now := time.Now().Format(timeFmt)
	certType := in.CertificateType
	if certType == "" {
		certType = "LETS_ENCRYPT"
	}
	validation := in.ValidationMethod
	if validation == "" {
		validation = "dns"
	}
	dnsProv := in.DNSProvider
	if dnsProv == "" {
		dnsProv = "CLOUDFLARE"
	}

	id, err := repo.New(s.store.Write).InsertSslCertificate(ctx, repo.InsertSslCertificateParams{
		Domain:            in.Domain,
		CertificateType:   certType,
		Email:             nullStr(in.Email),
		ValidationMethod:  nullStr(validation),
		AutoRenew:         nullInt64FromBool(in.AutoRenew),
		CertificateStatus: nullStr("PENDING"),
		IssueDate:         nullStr(""),
		ExpireDate:        nullStr(""),
		CertificatePath:   nullStr(""),
		PrivateKeyPath:    nullStr(""),
		CreateTime:        nullStr(now),
		UpdateTime:        nullStr(now),
		DnsProvider:       nullStr(dnsProv),
	})
	if err != nil {
		s.releaseDomainLock(in.Domain)
		return nil, fmt.Errorf("insert ssl certificate: %w", err)
	}

	// Launch async ACME flow.
	go s.processAcmeRequestAsync(id, in.Domain, in.Email, in.AutoRenew)

	return &SslCertificateDTO{
		ID:                id,
		Domain:            in.Domain,
		CertificateType:   certType,
		Email:             in.Email,
		ValidationMethod:  validation,
		AutoRenew:         in.AutoRenew,
		CertificateStatus: "PENDING",
		CreateTime:        now,
		UpdateTime:        now,
		DNSProvider:       dnsProv,
	}, nil
}

// processAcmeRequestAsync runs the ACME certificate flow in a background goroutine.
func (s *NginxService) processAcmeRequestAsync(certID int64, domain, email string, autoRenew bool) {
	defer s.releaseDomainLock(domain)
	defer s.cleanupStaleDomainLocks()

	ctx := context.Background()
	now := time.Now().Format(timeFmt)
	qw := repo.New(s.store.Write)

	// Get Cloudflare API token from system config.
	cfAPIToken := s.sc.GetString(ctx, "cloudflare.api.token")
	if cfAPIToken == "" {
		s.logger.Error().Str("domain", domain).Msg("nginx: ACME failed — Cloudflare API token not configured")
		_ = qw.UpdateSslCertificate(ctx, repo.UpdateSslCertificateParams{
			Domain:            domain,
			CertificateType:   "LETS_ENCRYPT",
			Email:             nullStr(email),
			ValidationMethod:  nullStr("dns"),
			AutoRenew:         nullInt64FromBool(autoRenew),
			CertificateStatus: nullStr("ERROR"),
			IssueDate:         nullStr(""),
			ExpireDate:        nullStr(""),
			CertificatePath:   nullStr(""),
			PrivateKeyPath:    nullStr(""),
			UpdateTime:        nullStr(now),
			DnsProvider:       nullStr("CLOUDFLARE"),
			ID:                certID,
		})
		s.syncProxySslStatus(ctx, certID, "ERROR")
		return
	}

	staging := s.cfg.Ssl.Staging

	// Use lego for ACME.
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		s.logger.Error().Err(err).Str("domain", domain).Msg("nginx: ACME failed — generate account key")
		s.markCertError(ctx, certID, domain, email, autoRenew)
		return
	}

	user := &acmeUser{Email: email, Key: privKey}
	legoCfg := lego.NewConfig(user)
	if staging {
		legoCfg.CADirURL = lego.LEDirectoryStaging
	} else {
		legoCfg.CADirURL = lego.LEDirectoryProduction
	}

	client, err := lego.NewClient(legoCfg)
	if err != nil {
		s.logger.Error().Err(err).Str("domain", domain).Msg("nginx: ACME failed — create lego client")
		s.markCertError(ctx, certID, domain, email, autoRenew)
		return
	}

	_, err = client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		s.logger.Debug().Err(err).Str("domain", domain).Msg("nginx: ACME register (may be ok)")
	}

	cfProvider, err := cloudflare.NewDNSProviderConfig(&cloudflare.Config{
		AuthToken: cfAPIToken,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("domain", domain).Msg("nginx: ACME failed — Cloudflare provider")
		s.markCertError(ctx, certID, domain, email, autoRenew)
		return
	}

	err = client.Challenge.SetDNS01Provider(cfProvider,
		dns01.AddDNSTimeout(120*time.Second),
	)
	if err != nil {
		s.logger.Error().Err(err).Str("domain", domain).Msg("nginx: ACME failed — set DNS provider")
		s.markCertError(ctx, certID, domain, email, autoRenew)
		return
	}

	// Obtain certificate.
	certRes, err := client.Certificate.Obtain(certificate.ObtainRequest{
		Domains: []string{domain},
		Bundle:  true,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("domain", domain).Msg("nginx: ACME failed — obtain certificate")
		s.markCertError(ctx, certID, domain, email, autoRenew)
		return
	}

	// Save cert files to disk.
	certDir := filepath.Join(s.certBasePath, domain)
	if err := os.MkdirAll(certDir, 0700); err != nil {
		s.logger.Error().Err(err).Str("domain", domain).Msg("nginx: ACME failed — create cert dir")
		s.markCertError(ctx, certID, domain, email, autoRenew)
		return
	}

	certPath := filepath.Join(certDir, "fullchain.pem")
	keyPath := filepath.Join(certDir, "privkey.pem")

	if err := os.WriteFile(certPath, certRes.Certificate, 0600); err != nil {
		s.logger.Error().Err(err).Str("domain", domain).Msg("nginx: ACME failed — write cert")
		s.markCertError(ctx, certID, domain, email, autoRenew)
		return
	}
	if err := os.WriteFile(keyPath, certRes.PrivateKey, 0600); err != nil {
		s.logger.Error().Err(err).Str("domain", domain).Msg("nginx: ACME failed — write key")
		s.markCertError(ctx, certID, domain, email, autoRenew)
		return
	}

	// Parse expiry date from certificate.
	expiryDate := time.Now().Add(90 * 24 * time.Hour).Format(timeFmt)
	if parsed, err := parseCertExpiry(certRes.Certificate); err == nil {
		expiryDate = parsed.Format(timeFmt)
	}

	issueDate := time.Now().Format(timeFmt)

	// Update DB.
	if err := qw.UpdateSslCertificate(ctx, repo.UpdateSslCertificateParams{
		Domain:            domain,
		CertificateType:   "LETS_ENCRYPT",
		Email:             nullStr(email),
		ValidationMethod:  nullStr("dns"),
		AutoRenew:         nullInt64FromBool(autoRenew),
		CertificateStatus: nullStr("VALID"),
		IssueDate:         nullStr(issueDate),
		ExpireDate:        nullStr(expiryDate),
		CertificatePath:   nullStr(certPath),
		PrivateKeyPath:    nullStr(keyPath),
		UpdateTime:        nullStr(now),
		DnsProvider:       nullStr("CLOUDFLARE"),
		ID:                certID,
	}); err != nil {
		s.logger.Error().Err(err).Str("domain", domain).Msg("nginx: ACME — update DB failed")
		return
	}

	// Upload cert to OpenResty.
	if err := s.openresty.UploadSSLCert(ctx, domain, string(certRes.Certificate), string(certRes.PrivateKey), true); err != nil {
		s.logger.Warn().Err(err).Str("domain", domain).Msg("nginx: ACME — upload to OpenResty failed")
	}

	// Sync proxy SSL status.
	s.syncProxySslStatus(ctx, certID, "CONFIGURED")

	s.logger.Info().Str("domain", domain).Str("expiry", expiryDate).Msg("nginx: certificate obtained successfully")
}

// markCertError marks a certificate as ERROR and syncs proxy status.
func (s *NginxService) markCertError(ctx context.Context, certID int64, domain, email string, autoRenew bool) {
	now := time.Now().Format(timeFmt)
	_ = repo.New(s.store.Write).UpdateSslCertificate(ctx, repo.UpdateSslCertificateParams{
		Domain:            domain,
		CertificateType:   "LETS_ENCRYPT",
		Email:             nullStr(email),
		ValidationMethod:  nullStr("dns"),
		AutoRenew:         nullInt64FromBool(autoRenew),
		CertificateStatus: nullStr("ERROR"),
		IssueDate:         nullStr(""),
		ExpireDate:        nullStr(""),
		CertificatePath:   nullStr(""),
		PrivateKeyPath:    nullStr(""),
		UpdateTime:        nullStr(now),
		DnsProvider:       nullStr("CLOUDFLARE"),
		ID:                certID,
	})
	s.syncProxySslStatus(ctx, certID, "ERROR")
}

// syncProxySslStatus updates all proxy configs that reference the given cert.
func (s *NginxService) syncProxySslStatus(ctx context.Context, certID int64, sslStatus string) {
	now := time.Now().Format(timeFmt)
	qr := repo.New(s.store.Read)
	qw := repo.New(s.store.Write)

	proxies, err := qr.FindProxyConfigsBySslCertId(ctx, sql.NullInt64{Int64: certID, Valid: true})
	if err != nil {
		s.logger.Warn().Err(err).Int64("certID", certID).Msg("nginx: find proxies by cert failed")
		return
	}
	for _, pc := range proxies {
		_ = qw.UpdateProxyConfigSslFields(ctx, repo.UpdateProxyConfigSslFieldsParams{
			SslCertificateID: pc.SslCertificateID,
			EnableSsl:        pc.EnableSsl,
			SslStatus:        nullStr(sslStatus),
			UpdateTime:       nullStr(now),
			ID:               pc.ID,
		})
	}
}

// RenewCertificate renews an existing certificate.
func (s *NginxService) RenewCertificate(ctx context.Context, id int64) error {
	qr := repo.New(s.store.Read)
	cert, err := qr.FindSslCertificateById(ctx, id)
	if err != nil {
		return fmt.Errorf("find certificate: %w", err)
	}
	if cert.CertificateType != "LETS_ENCRYPT" {
		return fmt.Errorf("only LETS_ENCRYPT certificates can be renewed")
	}

	if !s.acquireDomainLock(cert.Domain) {
		return fmt.Errorf("certificate operation already in progress for domain: %s", cert.Domain)
	}

	now := time.Now().Format(timeFmt)
	_ = repo.New(s.store.Write).UpdateSslCertificate(ctx, repo.UpdateSslCertificateParams{
		Domain:            cert.Domain,
		CertificateType:   cert.CertificateType,
		Email:             cert.Email,
		ValidationMethod:  cert.ValidationMethod,
		AutoRenew:         cert.AutoRenew,
		CertificateStatus: nullStr("PENDING"),
		IssueDate:         cert.IssueDate,
		ExpireDate:        cert.ExpireDate,
		CertificatePath:   cert.CertificatePath,
		PrivateKeyPath:    cert.PrivateKeyPath,
		UpdateTime:        nullStr(now),
		DnsProvider:       cert.DnsProvider,
		ID:                id,
	})

	go s.processAcmeRequestAsync(id, cert.Domain, cert.Email.String, cert.AutoRenew.Valid && cert.AutoRenew.Int64 == 1)
	return nil
}

// DeleteCertificate deletes a certificate. Rejects if referenced by proxy configs.
func (s *NginxService) DeleteCertificate(ctx context.Context, id int64) error {
	qr := repo.New(s.store.Read)
	cert, err := qr.FindSslCertificateById(ctx, id)
	if err != nil {
		return fmt.Errorf("find certificate: %w", err)
	}

	// Check for referencing proxy configs.
	refCount, err := qr.ExistsProxyConfigBySslCertId(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		return fmt.Errorf("check proxy references: %w", err)
	}
	if refCount > 0 {
		// Find the referencing domains for a better error message.
		proxies, _ := qr.FindProxyConfigsBySslCertId(ctx, sql.NullInt64{Int64: id, Valid: true})
		domains := make([]string, 0, len(proxies))
		for _, p := range proxies {
			domains = append(domains, p.Domain)
		}
		return fmt.Errorf("certificate is referenced by %d proxy config(s): %s", refCount, strings.Join(domains, ", "))
	}

	// Best-effort ACME revocation (LETS_ENCRYPT + VALID).
	if cert.CertificateType == "LETS_ENCRYPT" && cert.CertificateStatus.Valid && cert.CertificateStatus.String == "VALID" {
		s.logger.Info().Int64("id", id).Str("domain", cert.Domain).Msg("nginx: skipping ACME revocation (not implemented in lego obtain-only flow)")
	}

	// Delete cert files from disk.
	if cert.CertificatePath.Valid && cert.CertificatePath.String != "" {
		_ = os.Remove(cert.CertificatePath.String)
	}
	if cert.PrivateKeyPath.Valid && cert.PrivateKeyPath.String != "" {
		_ = os.Remove(cert.PrivateKeyPath.String)
	}
	// Also try to remove the cert directory.
	if cert.CertificatePath.Valid {
		dir := filepath.Dir(cert.CertificatePath.String)
		_ = os.Remove(dir)
	}

	// Delete from OpenResty.
	if err := s.openresty.DeleteSSLCert(ctx, cert.Domain); err != nil {
		s.logger.Warn().Err(err).Str("domain", cert.Domain).Msg("nginx: delete cert from OpenResty failed")
	}

	// Delete DB row.
	return repo.New(s.store.Write).DeleteSslCertificate(ctx, id)
}

// ToggleAutoRenew toggles the auto_renew flag on a certificate.
func (s *NginxService) ToggleAutoRenew(ctx context.Context, id int64, enabled bool) error {
	qr := repo.New(s.store.Read)
	cert, err := qr.FindSslCertificateById(ctx, id)
	if err != nil {
		return fmt.Errorf("find certificate: %w", err)
	}
	now := time.Now().Format(timeFmt)
	return repo.New(s.store.Write).UpdateSslCertificate(ctx, repo.UpdateSslCertificateParams{
		Domain:            cert.Domain,
		CertificateType:   cert.CertificateType,
		Email:             cert.Email,
		ValidationMethod:  cert.ValidationMethod,
		AutoRenew:         nullInt64FromBool(enabled),
		CertificateStatus: cert.CertificateStatus,
		IssueDate:         cert.IssueDate,
		ExpireDate:        cert.ExpireDate,
		CertificatePath:   cert.CertificatePath,
		PrivateKeyPath:    cert.PrivateKeyPath,
		UpdateTime:        nullStr(now),
		DnsProvider:       cert.DnsProvider,
		ID:                id,
	})
}

// ListCertificates returns a paginated list of SSL certificates.
func (s *NginxService) ListCertificates(ctx context.Context, page, size int64) ([]SslCertificateDTO, int64, error) {
	if page < 0 {
		page = 0
	}
	if size <= 0 {
		size = 20
	}
	qr := repo.New(s.store.Read)
	total, err := qr.CountSslCertificates(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count certificates: %w", err)
	}
	rows, err := qr.ListSslCertificates(ctx, repo.ListSslCertificatesParams{
		Limit:  size,
		Offset: page * size,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list certificates: %w", err)
	}
	out := make([]SslCertificateDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, *sslCertToDTO(r))
	}
	return out, total, nil
}

// CheckExpiringCertificates returns certs expiring within 30 days with auto_renew.
func (s *NginxService) CheckExpiringCertificates(ctx context.Context) ([]SslCertificateDTO, error) {
	expiryThreshold := time.Now().Add(30 * 24 * time.Hour).Format(timeFmt)
	rows, err := repo.New(s.store.Read).ListExpiringCertificates(ctx, nullStr(expiryThreshold))
	if err != nil {
		return nil, fmt.Errorf("list expiring certificates: %w", err)
	}
	out := make([]SslCertificateDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, *sslCertToDTO(r))
	}
	return out, nil
}

// DownloadCertificate returns a ZIP archive containing the cert files and a README.
func (s *NginxService) DownloadCertificate(ctx context.Context, id int64) ([]byte, string, error) {
	cert, err := repo.New(s.store.Read).FindSslCertificateById(ctx, id)
	if err != nil {
		return nil, "", fmt.Errorf("find certificate: %w", err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Add fullchain.pem.
	if cert.CertificatePath.Valid && cert.CertificatePath.String != "" {
		certData, err := os.ReadFile(cert.CertificatePath.String)
		if err == nil {
			if f, err := zw.Create("fullchain.pem"); err == nil {
				f.Write(certData)
			}
		}
	}

	// Add privkey.pem.
	if cert.PrivateKeyPath.Valid && cert.PrivateKeyPath.String != "" {
		keyData, err := os.ReadFile(cert.PrivateKeyPath.String)
		if err == nil {
			if f, err := zw.Create("privkey.pem"); err == nil {
				f.Write(keyData)
			}
		}
	}

	// Add README.txt.
	readme := fmt.Sprintf("SSL Certificate for %s\n"+
		"Type: %s\n"+
		"Issued: %s\n"+
		"Expires: %s\n"+
		"\n"+
		"Usage:\n"+
		"  - fullchain.pem: The full certificate chain (server + intermediates)\n"+
		"  - privkey.pem: The private key\n"+
		"\n"+
		"Nginx configuration:\n"+
		"  ssl_certificate /path/to/fullchain.pem;\n"+
		"  ssl_certificate_key /path/to/privkey.pem;\n",
		cert.Domain, cert.CertificateType,
		nullStrVal(cert.IssueDate), nullStrVal(cert.ExpireDate))
	if f, err := zw.Create("README.txt"); err == nil {
		f.Write([]byte(readme))
	}

	zw.Close()

	filename := fmt.Sprintf("ssl-cert-%s.zip", cert.Domain)
	return buf.Bytes(), filename, nil
}

// MatchCertificatesByDomain finds VALID certificates matching a domain
// (exact, wildcard, or multi-domain).
func (s *NginxService) MatchCertificatesByDomain(ctx context.Context, domain string) ([]CertificateDTO, error) {
	certs, err := repo.New(s.store.Read).FindAllActiveSslCertificates(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active certificates: %w", err)
	}

	var matches []CertificateDTO
	for _, cert := range certs {
		if isDomainMatch(cert.Domain, domain) {
			matches = append(matches, CertificateDTO{
				ID:         cert.ID,
				Name:       cert.Domain,
				Domain:     cert.Domain,
				CertPath:   nullStrVal(cert.CertificatePath),
				KeyPath:    nullStrVal(cert.PrivateKeyPath),
				ExpiryDate: nullStrVal(cert.ExpireDate),
			})
		}
	}

	if matches == nil {
		matches = []CertificateDTO{}
	}
	return matches, nil
}

// isDomainMatch checks if a certificate domain matches the query domain.
// Supports exact match, single-level wildcard (*.example.com), and multi-domain.
func isDomainMatch(certDomain, queryDomain string) bool {
	// Multi-domain cert: split on comma.
	for _, d := range strings.Split(certDomain, ",") {
		d = strings.TrimSpace(d)
		if singleDomainMatch(d, queryDomain) {
			return true
		}
	}
	return false
}

func singleDomainMatch(certDomain, queryDomain string) bool {
	// Exact match.
	if certDomain == queryDomain {
		return true
	}
	// Wildcard match: *.example.com matches api.example.com (single-level only).
	if strings.HasPrefix(certDomain, "*.") {
		suffix := certDomain[1:] // ".example.com"
		if strings.HasSuffix(queryDomain, suffix) {
			// Ensure only one subdomain level.
			prefix := queryDomain[:len(queryDomain)-len(suffix)]
			return !strings.Contains(prefix, ".")
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// OpenResty Service Management
// ---------------------------------------------------------------------------

// CheckOpenRestyStatus checks if OpenResty is installed, running, and API available.
func (s *NginxService) CheckOpenRestyStatus(ctx context.Context) (map[string]any, error) {
	result := map[string]any{
		"installed":   false,
		"running":     false,
		"apiAvailable": false,
	}

	// Check if installed.
	cmd := exec.CommandContext(ctx, "openresty", "-v")
	if err := cmd.Run(); err == nil {
		result["installed"] = true
	} else {
		return result, nil
	}

	// Check if running.
	cmd = exec.CommandContext(ctx, "pgrep", "-f", "openresty")
	if err := cmd.Run(); err == nil {
		result["running"] = true
	} else {
		return result, nil
	}

	// Check API availability.
	ok, err := s.openresty.HealthCheck(ctx)
	if err == nil && ok {
		result["apiAvailable"] = true
	}

	return result, nil
}

// StartOpenResty starts the OpenResty service and waits for the API to become available.
func (s *NginxService) StartOpenResty(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "/usr/local/openresty/bin/openresty")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("start openresty: %w", err)
	}

	// Poll API availability.
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		ok, err := s.openresty.HealthCheck(ctx)
		if err == nil && ok {
			return nil
		}
	}

	return fmt.Errorf("openresty API did not become available within 10 seconds")
}

// ---------------------------------------------------------------------------
// SSL Auto-Renewal (called by scheduler)
// ---------------------------------------------------------------------------

// ProcessAutoRenewal checks for expiring certificates and renews them.
func (s *NginxService) ProcessAutoRenewal(ctx context.Context) {
	s.cleanupStaleDomainLocks()

	expiryThreshold := time.Now().Add(7 * 24 * time.Hour).Format(timeFmt)
	certs, err := repo.New(s.store.Read).ListExpiringCertificates(ctx, nullStr(expiryThreshold))
	if err != nil {
		s.logger.Error().Err(err).Msg("nginx: auto-renewal — list expiring certs failed")
		return
	}

	for _, cert := range certs {
		if !s.acquireDomainLock(cert.Domain) {
			s.logger.Info().Str("domain", cert.Domain).Msg("nginx: auto-renewal — skipping (lock held)")
			continue
		}

		s.logger.Info().Str("domain", cert.Domain).Int64("certID", cert.ID).Msg("nginx: auto-renewing certificate")
		now := time.Now().Format(timeFmt)
		_ = repo.New(s.store.Write).UpdateSslCertificate(ctx, repo.UpdateSslCertificateParams{
			Domain:            cert.Domain,
			CertificateType:   cert.CertificateType,
			Email:             cert.Email,
			ValidationMethod:  cert.ValidationMethod,
			AutoRenew:         cert.AutoRenew,
			CertificateStatus: nullStr("PENDING"),
			IssueDate:         cert.IssueDate,
			ExpireDate:        cert.ExpireDate,
			CertificatePath:   cert.CertificatePath,
			PrivateKeyPath:    cert.PrivateKeyPath,
			UpdateTime:        nullStr(now),
			DnsProvider:       cert.DnsProvider,
			ID:                cert.ID,
		})

		go s.processAcmeRequestAsync(cert.ID, cert.Domain, cert.Email.String, true)
	}
}

// SyncProxySslStatusByCertificate syncs proxy SSL status for a given cert ID.
func (s *NginxService) SyncProxySslStatusByCertificate(ctx context.Context, certID int64, status string) {
	s.syncProxySslStatus(ctx, certID, status)
}

// UploadSslToOpenResty uploads a certificate to OpenResty.
func (s *NginxService) UploadSslToOpenResty(ctx context.Context, certPath, keyPath, domain string, reloadAfter bool) error {
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("read cert: %w", err)
	}
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("read key: %w", err)
	}
	return s.openresty.UploadSSLCert(ctx, domain, string(certData), string(keyData), true)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// tryCreateCfDNSRecord attempts to create a Cloudflare A record (best-effort).
func (s *NginxService) tryCreateCfDNSRecord(domain string) {
	ctx := context.Background()
	cfAPIToken := s.sc.GetString(ctx, "cloudflare.api.token")
	if cfAPIToken == "" {
		return
	}

	// Try to find the zone for this domain.
	cfClient := dns.NewCfClient(cfAPIToken)
	zones, err := cfClient.ListZones(ctx)
	if err != nil {
		s.logger.Warn().Err(err).Str("domain", domain).Msg("nginx: CF DNS — list zones failed")
		return
	}

	// Find matching zone (longest suffix match).
	var matchedZone *dns.Zone
	for _, z := range zones {
		if strings.HasSuffix(domain, "."+z.Name) || domain == z.Name {
			if matchedZone == nil || len(z.Name) > len(matchedZone.Name) {
				matchedZone = &z
			}
		}
	}
	if matchedZone == nil {
		s.logger.Warn().Str("domain", domain).Msg("nginx: CF DNS — no matching zone found")
		return
	}

	// Get server IP (best-effort — use the machine's public IP).
	serverIP := s.getPublicIP()
	if serverIP == "" {
		s.logger.Warn().Str("domain", domain).Msg("nginx: CF DNS — could not determine server IP")
		return
	}

	_, err = cfClient.CreateDnsRecord(ctx, matchedZone.ID, dns.DnsRecord{
		Name:    domain,
		Type:    "A",
		Content: serverIP,
		TTL:     1, // auto
		Proxied: false,
	})
	if err != nil {
		s.logger.Warn().Err(err).Str("domain", domain).Msg("nginx: CF DNS — create A record failed")
	} else {
		s.logger.Info().Str("domain", domain).Str("ip", serverIP).Msg("nginx: CF DNS — A record created")
	}
}

// getPublicIP returns the machine's public IP (best-effort).
func (s *NginxService) getPublicIP() string {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 3*time.Second)
	if err != nil {
		return ""
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// acmeUser implements the lego registration.User interface.
type acmeUser struct {
	Email string
	Key   *ecdsa.PrivateKey
}

func (u *acmeUser) GetEmail() string                        { return u.Email }
func (u *acmeUser) GetRegistration() *registration.Resource { return nil }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.Key }

// parseCertExpiry parses the expiry date from a PEM-encoded certificate.
func parseCertExpiry(certPEM []byte) (*time.Time, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}
	return &cert.NotAfter, nil
}

// nginxConfigToDTO converts a repo.NginxConfig to a NginxConfigDTO.
func nginxConfigToDTO(c repo.NginxConfig) *NginxConfigDTO {
	return &NginxConfigDTO{
		ID:            c.ID,
		ConfigName:    nullStrVal(c.ConfigName),
		ConfigContent: nullStrVal(c.ConfigContent),
		IsCurrent:     c.IsCurrent.Valid && c.IsCurrent.Int64 == 1,
		ConfigVersion: nullInt64Val(c.ConfigVersion),
		ConfigStatus:  nullStrVal(c.ConfigStatus),
		CreateTime:    nullStrVal(c.CreateTime),
		UpdateTime:    nullStrVal(c.UpdateTime),
	}
}

// sslCertToDTO converts a repo.SslCertificate to a SslCertificateDTO.
func sslCertToDTO(c repo.SslCertificate) *SslCertificateDTO {
	return &SslCertificateDTO{
		ID:                c.ID,
		Domain:            c.Domain,
		CertificateType:   c.CertificateType,
		Email:             nullStrVal(c.Email),
		ValidationMethod:  nullStrVal(c.ValidationMethod),
		AutoRenew:         c.AutoRenew.Valid && c.AutoRenew.Int64 == 1,
		CertificateStatus: nullStrVal(c.CertificateStatus),
		IssueDate:         nullStrVal(c.IssueDate),
		ExpireDate:        nullStrVal(c.ExpireDate),
		CertificatePath:   nullStrVal(c.CertificatePath),
		PrivateKeyPath:    nullStrVal(c.PrivateKeyPath),
		CreateTime:        nullStrVal(c.CreateTime),
		UpdateTime:        nullStrVal(c.UpdateTime),
		DNSProvider:       nullStrVal(c.DnsProvider),
	}
}

// buildLineDiff computes a simple LCS-based line diff between two strings.
func buildLineDiff(old, new_ string) string {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new_, "\n")

	// Simple diff: mark removed (-) and added (+) lines.
	// For a proper LCS implementation, we'd use dynamic programming,
	// but for simplicity we use a line-set approach.
	oldSet := make(map[string]int)
	for _, l := range oldLines {
		oldSet[l]++
	}
	newSet := make(map[string]int)
	for _, l := range newLines {
		newSet[l]++
	}

	var sb strings.Builder
	// Lines in old but not in new (removed).
	for _, l := range oldLines {
		if newSet[l] == 0 {
			sb.WriteString("- " + l + "\n")
		} else {
			sb.WriteString("  " + l + "\n")
			newSet[l]-- // mark as consumed
		}
	}
	// Lines in new but not in old (added).
	for _, l := range newLines {
		if oldSet[l] == 0 {
			sb.WriteString("+ " + l + "\n")
		}
	}

	return sb.String()
}

// null helpers specific to this file (nullStr and nullInt64 are in tenant.go).
func nullInt64FromBool(b bool) sql.NullInt64 {
	if b {
		return sql.NullInt64{Int64: 1, Valid: true}
	}
	return sql.NullInt64{}
}

func nullInt64Ptr(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func nullStrVal(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

func nullInt64Val(v sql.NullInt64) int64 {
	if v.Valid {
		return v.Int64
	}
	return 0
}
