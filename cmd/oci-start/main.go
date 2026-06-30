// oci-start Go rewrite — main entrypoint. Assembly + graceful shutdown.
// Phase 2 wires auth (keypair store, session service, sysconf, turnstile bypass,
// OAuth state cache) into the httpapi Deps. See SPEC §10 / plan §8.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/auth"
	"github.com/Muione/oci-start-go/internal/bootstrap"
	"github.com/Muione/oci-start-go/internal/config"
	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/dns"
	"github.com/Muione/oci-start-go/internal/acme"
	"github.com/Muione/oci-start-go/internal/grabber"
	"github.com/Muione/oci-start-go/internal/migration"
	"github.com/Muione/oci-start-go/internal/notify"
	"github.com/Muione/oci-start-go/internal/httpapi"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/scheduler"
	"github.com/Muione/oci-start-go/internal/service"
	"github.com/Muione/oci-start-go/internal/sysconf"
	"github.com/Muione/oci-start-go/internal/util/crypto"
	"github.com/Muione/oci-start-go/internal/util/httpclient"
	logpkg "github.com/Muione/oci-start-go/internal/util/log"
	"github.com/Muione/oci-start-go/internal/util/rsakey"
	"github.com/Muione/oci-start-go/internal/ws"
)

func main() {
	// Honor TZ env (parity with Docker TZ).
	if tz := os.Getenv("TZ"); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			time.Local = loc
		}
	}

	cfg, err := config.Load()
	die(err, "load config")

	logger := logpkg.Init(logpkg.Config{
		Level:               cfg.Logging.Level,
		LogHome:             cfg.Logging.LogHome,
		File:                cfg.Logging.File,
		MaxSizeMB:           cfg.Logging.MaxSizeMB,
		MaxAgeDays:          cfg.Logging.MaxAgeDays,
		TotalSizeCapGB:      cfg.Logging.TotalSizeCapGB,
		CleanHistoryOnStart: cfg.Logging.CleanHistoryOnStart,
		PrettyConsole:       cfg.Logging.PrettyConsole,
	})
	logger.Info().Msg("oci-start booting")

	masterKey, err := crypto.LoadMasterKey(cfg.MasterKeyPath())
	if err != nil {
		logger.Fatal().Err(err).Msg("load master key")
	}
	logger.Info().Msg("master key ready")

	store, err := db.Open(cfg.Datasource.URL, cfg.Datasource.MaxOpenConns, cfg.Datasource.ReadMaxOpenConns)
	if err != nil {
		logger.Fatal().Err(err).Msg("open db")
	}
	defer store.Close()

	if cfg.Migrate.AutoOnBoot {
		if err := store.Migrate(cfg.Migrate.Path); err != nil {
			logger.Fatal().Err(err).Msg("run migrations")
		}
	}
	logger.Info().Msg("database ready")

	bctx := context.Background()
	if err := bootstrap.AppVersion(bctx, store, cfg, logger); err != nil {
		logger.Fatal().Err(err).Msg("bootstrap app version")
	}

	// Phase 2 wiring.
	sc := sysconf.New(store)
	kpStore := rsakey.NewKeypairStore()
	defer kpStore.Stop()
	bypass := &bootstrap.BypassTokenHolder{}
	if err := bypass.CheckAndLog(bctx, sc, cfg.Turnstile.Local.Bypass, logger); err != nil {
		logger.Fatal().Err(err).Msg("bootstrap turnstile")
	}
	// NOTE: initDeviceRegistration intentionally NOT called — D3 removes the
	// phone-home/user-statistics feature.
	sessionSvc := auth.NewSessionService(store)
	oauthState := httpapi.NewStateCache()
	defer oauthState.Stop()

	// Phase 3 wiring: OCI integration (proxy pool + tenant service). Private
	// keys are encrypted at rest with the master key (plan D1).
	proxyPool := oci.NewProxyPool(repo.New(store.Read))
	tenantSvc := service.NewTenantService(store, masterKey, proxyPool)

	// Phase 7 wiring: notification (before engine so it can be passed in).
	// Use proxy-aware HTTP client if system proxy is configured.
	bgCtx := context.Background()
	proxyClient := httpclient.NewClient(sc, 10*time.Second)
	tgToken := sc.GetString(bgCtx, "telegram.bot.token")
	tgChatID := sc.GetString(bgCtx, "telegram.chat.id")
	tgNotifier := notify.NewTelegramNotifier(tgToken, tgChatID, logger, proxyClient)

	// Phase 5: backupSvc created before engine so OnGrabSuccess can reference it.
	backupSvc := service.NewBackupSvc(store, masterKey, logger)

	// Phase 4 wiring: grab engine + scheduler + boot task CRUD.
	engine, err := grabber.NewEngine(grabber.EngineConfig{}, grabber.EngineDeps{
		Store:     store,
		ProxyPool: proxyPool,
		MasterKey: masterKey,
		Logger:    logger,
		Notifier:  tgNotifier,
		OnGrabSuccess: func(ctx context.Context, task repo.BootInstance, result *grabber.GrabResult) {
			// Bridge grabber → backup service.
			backupSvc.ScheduleBackup(ctx, service.BackupInput{
				InstanceID:   result.InstanceID,
				PublicIP:     result.PublicIP,
				TenantID:     task.TenantID.Int64,
				RootPassword: ns(task.RootPassword),
				Architecture: ns(task.Architecture),
			})
		},
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("create grab engine")
	}
	bootSvc := service.NewBootService(store)

	// Phase 5 wiring: instance detail, traffic, check-live, ping, offline
	// (backupSvc created earlier for OnGrabSuccess callback).
	instanceDetailSvc := service.NewInstanceDetailSvc(store)
	trafficSvc := service.NewTrafficSvc(store, masterKey, logger, tgNotifier)
	checkLiveSvc := service.NewCheckLiveSvc(store, masterKey, logger, tgNotifier)
	pingSvc := service.NewPingSvc(store, logger)
	offlineSvc := service.NewOfflineSvc(store, logger, tgNotifier)

	// Phase 11.3 wiring: security list rule management (needed before Phase 6
	// rescue handler and Phase 12.3 injection into backup/ping).
	securityRuleSvc := service.NewSecurityRuleService(store, masterKey, proxyPool)

	// Phase 12.3 wiring: SSH root login configurator.
	sshConfig := service.NewSSHConfigurator(logger)

	// Inject security rules and SSH config into backup and ping services.
	backupSvc.SetSecurityRules(securityRuleSvc)
	backupSvc.SetSSHConfig(sshConfig)
	pingSvc.SetSecurityRules(securityRuleSvc)

	// Phase 6 wiring: WebSocket hub.
	wsHub := ws.NewHub()

	// Wire console handler with instance lookup from DB.
	wsHub.Console.SetDeps(&ws.ConsoleDeps{
		Logger:    logger,
		MasterKey: masterKey,
		InstanceLookup: func(instanceID string) (*ws.ConsoleInstanceInfo, error) {
			var (
				username, port, password, compID, ad sql.NullString
				tenantID                             sql.NullInt64
				info                                 ws.ConsoleInstanceInfo
			)
			err := store.Read.QueryRowContext(context.Background(),
				`SELECT instance_id, display_name, public_ips, private_ips, shape,
				        username, port, password, tenant_id, compartment_id,
				        availability_domain
				 FROM instance_detail WHERE instance_id = ? LIMIT 1`,
				instanceID).Scan(
				&info.InstanceID, &info.DisplayName, &info.PublicIPs, &info.PrivateIPs,
				&info.Shape, &username, &port, &password,
				&tenantID, &compID, &ad)
			if err != nil {
				return nil, fmt.Errorf("instance %s not found: %w", instanceID, err)
			}
			info.Username = username.String
			info.Port = port.String
			info.Password = password.String
			info.TenantID = tenantID.Int64
			info.CompartmentID = compID.String
			info.AvailabilityDomain = ad.String
			return &info, nil
		},
		BuildClients: func(ctx context.Context, tenantID int64) (oci.Clients, error) {
			creds, err := lookupTenantCreds(store, tenantID)
			if err != nil {
				return oci.Clients{}, err
			}
			prov, err := oci.NewProvider(creds, masterKey)
			if err != nil {
				return oci.Clients{}, fmt.Errorf("oci provider: %w", err)
			}
			return oci.NewClients(prov)
		},
		GetCompartmentID: func(ctx context.Context, tenantID int64, instanceID string) (string, error) {
			// Get the instance's compartment from DB, falling back to tenancy OCID.
			var compID sql.NullString
			err := store.Read.QueryRowContext(ctx,
				`SELECT compartment_id FROM instance_detail WHERE instance_id = ? LIMIT 1`,
				instanceID).Scan(&compID)
			if err == nil && compID.Valid && compID.String != "" {
				return compID.String, nil
			}
			// Fallback: use tenant's tenancy OCID as compartment.
			tenant, err := repo.New(store.Read).FindTenantByID(ctx, tenantID)
			if err != nil {
				return "", fmt.Errorf("tenant %d not found: %w", tenantID, err)
			}
			return ns(tenant.Tenancy), nil
		},
	})

	// Wire rescue handler with OCI operations (full multi-step flow).
	wsHub.Rescue.SetDeps(&ws.RescueDeps{
		Logger:    logger,
		MasterKey: masterKey,
		GetInstance: func(instanceID string, tenantID int64) (*ws.RescueInstanceInfo, error) {
			var info ws.RescueInstanceInfo
			err := store.Read.QueryRowContext(context.Background(),
				`SELECT instance_id, display_name, state, boot_volume_id, shape,
				        availability_domain, compartment_id, public_ips, username, password
				 FROM instance_detail WHERE instance_id = ? LIMIT 1`,
				instanceID).Scan(
				&info.ID, &info.DisplayName, &info.State, &info.BootVolumeID,
				&info.Shape, &info.AvailabilityDomain, &info.CompartmentID,
				&info.PublicIP, &info.SSHUsername, &info.SSHPassword)
			if err != nil {
				return nil, fmt.Errorf("instance %s not found: %w", instanceID, err)
			}
			return &info, nil
		},
		StopInstance: func(instanceID string, tenantID int64) error {
			return ociOpFromTenant(store, proxyPool, masterKey, tenantID, instanceID,
				func(ctx context.Context, c oci.Clients, instID string) error {
					return oci.StopInstance(ctx, c, instID)
				})
		},
		StartInstance: func(instanceID string, tenantID int64) error {
			return ociOpFromTenant(store, proxyPool, masterKey, tenantID, instanceID,
				func(ctx context.Context, c oci.Clients, instID string) error {
					return oci.StartInstance(ctx, c, instID)
				})
		},
		DetachBootVolume: func(instanceID string, tenantID int64) (string, error) {
			return ociDetachFromTenant(store, proxyPool, masterKey, tenantID, instanceID)
		},
		AttachBootVolume: func(instanceID string, tenantID int64, bootVolumeID string) error {
			return ociAttachFromTenant(store, proxyPool, masterKey, tenantID, instanceID, bootVolumeID)
		},
		AttachRescueVolume: func(instanceID string, tenantID int64, rescueImageID string) (string, error) {
			return ociAttachRescueFromTenant(store, proxyPool, masterKey, tenantID, instanceID, rescueImageID)
		},
		CheckAndEnableRule: func(ctx context.Context, tenantID int64) error {
			return securityRuleSvc.CheckAndEnableRule(ctx, tenantID)
		},
		EnableRootLogin: func(host, username, password, rootPassword string, port int) error {
			return sshConfig.EnableRootLogin(host, username, password, rootPassword, port)
		},
	})

	// Phase 7 wiring: DNS + ACME cert manager.
	dnsSvc := dns.NewDnsService(store)

	// DNS cache: persists Cloudflare API responses to SQLite so the
	// DNS management page serves cached data immediately after restart.
	dnsCacheStore := dns.NewCacheStore(store.Write)
	if err := dnsCacheStore.EnsureTable(); err != nil {
		logger.Warn().Err(err).Msg("main: failed to ensure api_cache table")
	}
	dns.SetCacheLogger(logger)
	dns.SetGlobalCacheStore(dnsCacheStore)

	certManager := acme.NewCertManager(logger)

	// Phase 8 wiring: data migration.
	migSplitter := migration.NewSQLSplitter(logger)
	migImporter := migration.NewImporter(store.Write, logger, migSplitter)
	migHandler := httpapi.NewMigrationHandler(migImporter, migSplitter, cfg.MasterKeyPath()+"/keys", store.Write)

	// Phase 9 wiring: tenant email & social config services.
	tenantEmailSvc := service.NewTenantEmailService(store)
	tenantSocialSvc := service.NewTenantSocialService(store)

	// Phase 10 wiring: tenant IAM user management.
	tenantUserSvc := service.NewTenantUserService(store, masterKey)

	// Phase 11.4 wiring: quota, region subscription, audit log.
	quotaSvc := service.NewQuotaService(store, masterKey, proxyPool)
	regionSubSvc := service.NewRegionSubService(store, masterKey, proxyPool)
	auditSvc := service.NewAuditService(store, masterKey, proxyPool)

	// Phase 11.1 wiring: object storage.
	objectStorageSvc := service.NewObjectStorageService(store, masterKey, proxyPool)

	// Phase 11.2 wiring: VNIC batch management.
	vnicMgmtSvc := service.NewVnicManagementService(store, masterKey, proxyPool)

	// Phase 12.2 wiring: Email Delivery service.
	emailSvc := service.NewEmailService(store, dnsSvc, sc, proxyPool, masterKey)

	// Phase B wiring: Billing (subscription + usage/cost).
	billingSvc := service.NewBillingService(store, masterKey, proxyPool)

	sched := scheduler.New(engine, store, logger, &scheduler.SvcSet{
		Traffic:        trafficSvc,
		CheckLive:      checkLiveSvc,
		Ping:           pingSvc,
		Offline:        offlineSvc,
		CertManager:    certManager,
		WsHub:          wsHub,
		ObjectStorage:  objectStorageSvc,
	})

	deps := &httpapi.Deps{
		Store:        store,
		Cfg:          cfg,
		Logger:       logger,
		Keypair:      kpStore,
		Session:      sessionSvc,
		SysConf:      sc,
		Bypass:       bypass,
		OAuthState:   oauthState,
		Tenant:       tenantSvc,
		ProxyPool:    proxyPool,
		MasterKey:    masterKey,
		Engine:       engine,
		Scheduler:    sched,
		Boot:         bootSvc,
		InstanceSvc:  instanceDetailSvc,
		TrafficSvc:   trafficSvc,
		BackupSvc:    backupSvc,
		CheckLiveSvc: checkLiveSvc,
		PingSvc:      pingSvc,
		OfflineSvc:   offlineSvc,
		WsHub:        wsHub,
		Notifier:     tgNotifier,
		DnsSvc:       dnsSvc,
		CertManager:  certManager,
		Migration:    migHandler,
		TenantEmail:  tenantEmailSvc,
		TenantSocial: tenantSocialSvc,
		TenantUser:   tenantUserSvc,
		SecurityRule: securityRuleSvc,
		Quota:        quotaSvc,
		RegionSub:    regionSubSvc,
		Audit:        auditSvc,
		ObjectStorageSvc: objectStorageSvc,
		VnicMgmtSvc:     vnicMgmtSvc,
		SSHConfig:       sshConfig,
		EmailSvc:        emailSvc,
		BillingSvc:      billingSvc,
		TotpSetup:       httpapi.NewTotpSetupCache(),
	}
	router := httpapi.NewServer(deps)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	go func() {
		logger.Info().Int("port", cfg.Server.Port).Msg("listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("server failed")
		}
	}()

	// Start the cron scheduler (Phase 4).
	sched.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutting down")

	// Phase 4: stop scheduler first, then drain grab engine, then HTTP.
	sched.Stop()
	engine.Shutdown()
	wsHub.Shutdown()

	sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		logger.Error().Err(err).Msg("http shutdown error")
	}
	logger.Info().Msg("stopped")
}

// --- OCI rescue helpers (Phase 8: real OCI SDK calls with tenant credentials) ---

// lookupTenantCreds fetches OCI credentials from the tenant table.
func lookupTenantCreds(store *db.Store, tenantID int64) (oci.Credentials, error) {
	tenant, err := repo.New(store.Read).FindTenantByID(context.Background(), tenantID)
	if err != nil {
		return oci.Credentials{}, fmt.Errorf("tenant %d not found: %w", tenantID, err)
	}
	return oci.Credentials{
		Tenancy:     ns(tenant.Tenancy),
		UserID:      ns(tenant.TenantID),
		Fingerprint: ns(tenant.Fingerprint),
		Region:      ns(tenant.Region),
		KeyFileBlob: ns(tenant.KeyFileBlob),
		KeyFile:     ns(tenant.KeyFile),
	}, nil
}

// ociOpFromTenant is a generic helper that looks up tenant creds, creates OCI
// clients through the proxy, and executes the given OCI operation.
func ociOpFromTenant(store *db.Store, proxyPool *oci.ProxyPool, masterKey []byte, tenantID int64, instanceID string, op func(context.Context, oci.Clients, string) error) error {
	creds, err := lookupTenantCreds(store, tenantID)
	if err != nil {
		return err
	}
	return oci.WithProxy(context.Background(), proxyPool, creds, masterKey, func(c oci.Clients) error {
		return op(context.Background(), c, instanceID)
	})
}

// ociDetachFromTenant detaches the boot volume from the instance identified by
// instanceID. It first lists boot volume attachments to find the attachment ID.
func ociDetachFromTenant(store *db.Store, proxyPool *oci.ProxyPool, masterKey []byte, tenantID int64, instanceID string) (string, error) {
	creds, err := lookupTenantCreds(store, tenantID)
	if err != nil {
		return "", err
	}
	var bootVolID string
	err = oci.WithProxy(context.Background(), proxyPool, creds, masterKey, func(c oci.Clients) error {
		// Get instance to find compartment + AD.
		inst, err := oci.GetInstanceFull(context.Background(), c, instanceID)
		if err != nil {
			return fmt.Errorf("get instance %s: %w", instanceID, err)
		}
		compID := nsStrPtr(inst.CompartmentId)
		ad := nsStrPtr(inst.AvailabilityDomain)

		// List boot volume attachments.
		attachments, err := oci.ListBootVolumeAttachments(context.Background(), c, compID, instanceID, ad)
		if err != nil {
			return fmt.Errorf("list attachments: %w", err)
		}
		if len(attachments) == 0 {
			return fmt.Errorf("no boot volume attachments found for instance %s", instanceID)
		}
		att := attachments[0]
		if att.Id == nil {
			return fmt.Errorf("attachment has nil ID")
		}
		if att.BootVolumeId != nil {
			bootVolID = *att.BootVolumeId
		}
		return oci.DetachBootVolume(context.Background(), c, *att.Id)
	})
	return bootVolID, err
}

// ociAttachFromTenant attaches a boot volume to an instance.
func ociAttachFromTenant(store *db.Store, proxyPool *oci.ProxyPool, masterKey []byte, tenantID int64, instanceID string, bootVolumeID string) error {
	creds, err := lookupTenantCreds(store, tenantID)
	if err != nil {
		return err
	}
	return oci.WithProxy(context.Background(), proxyPool, creds, masterKey, func(c oci.Clients) error {
		_, err = oci.AttachBootVolume(context.Background(), c, instanceID, bootVolumeID)
		return err
	})
}

// ociAttachRescueFromTenant attaches a rescue boot volume to an instance.
// The rescueImageID parameter is treated as a boot volume OCID (not an image OCID).
// In practice, a rescue boot volume should be pre-created from a rescue image
// using the OCI console or API, and its OCID passed here.
func ociAttachRescueFromTenant(store *db.Store, proxyPool *oci.ProxyPool, masterKey []byte, tenantID int64, instanceID string, rescueImageID string) (string, error) {
	if rescueImageID == "" {
		return "", fmt.Errorf("rescue boot volume ID is required")
	}
	creds, err := lookupTenantCreds(store, tenantID)
	if err != nil {
		return "", err
	}
	err = oci.WithProxy(context.Background(), proxyPool, creds, masterKey, func(c oci.Clients) error {
		_, err := oci.AttachBootVolume(context.Background(), c, instanceID, rescueImageID)
		return err
	})
	return rescueImageID, err
}

// nsStrPtr returns the string value pointed to by ptr, or "" if nil.
func nsStrPtr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// die is the pre-logger fatal path (config load). Post-logger errors use
// logger.Fatal().
func die(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %s: %v\n", msg, err)
		os.Exit(1)
	}
}

// ns unwraps a sql.NullString, returning "" when invalid.
func ns(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

var _ = zerolog.Logger{} // keep zerolog import meaningful for future log calls
