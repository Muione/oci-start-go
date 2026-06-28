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
	gcp2 "github.com/Muione/oci-start-go/internal/cloud/gcp"
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
	bgCtx := context.Background()
	tgToken := sc.GetString(bgCtx, "telegram.bot.token")
	tgChatID := sc.GetString(bgCtx, "telegram.chat.id")
	tgNotifier := notify.NewTelegramNotifier(tgToken, tgChatID, logger)

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
	})

	// Phase 7 wiring: DNS + ACME cert manager.
	dnsSvc := dns.NewDnsService(store)
	certManager := acme.NewCertManager(logger)

	// Phase 8 wiring: GCP Compute Engine (lazy init from system config).
	var gcpSvc *gcp2.GcpService
	if creds := sc.GetString(bctx, "gcp.serviceAccountJson"); creds != "" {
		projectID := sc.GetString(bctx, "gcp.projectId")
		if client, err := gcp2.NewGcpClient(creds, projectID); err == nil {
			gcpSvc = gcp2.NewGcpServiceWithClient(client)
			logger.Info().Msg("main: GCP service initialized")
		} else {
			logger.Warn().Err(err).Msg("main: GCP credentials found but invalid")
		}
	}

	// Phase 8 wiring: data migration.
	migSplitter := migration.NewSQLSplitter(logger)
	migImporter := migration.NewImporter(store.Write, logger, migSplitter)
	migHandler := httpapi.NewMigrationHandler(migImporter, migSplitter, cfg.MasterKeyPath()+"/keys", store.Write)

	// Phase 9 wiring: tenant email & social config services.
	tenantEmailSvc := service.NewTenantEmailService(store)
	tenantSocialSvc := service.NewTenantSocialService(store)

	sched := scheduler.New(engine, store, logger, &scheduler.SvcSet{
		Traffic:     trafficSvc,
		CheckLive:   checkLiveSvc,
		Ping:        pingSvc,
		Offline:     offlineSvc,
		CertManager: certManager,
		WsHub:       wsHub,
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
		GcpSvc:       gcpSvc,
		TenantEmail:  tenantEmailSvc,
		TenantSocial: tenantSocialSvc,
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
