// oci-start Go rewrite — main entrypoint. Assembly + graceful shutdown.
// Phase 2 wires auth (keypair store, session service, sysconf, turnstile bypass,
// OAuth state cache) into the httpapi Deps. See SPEC §10 / plan §8.
package main

import (
	"context"
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
	"github.com/Muione/oci-start-go/internal/httpapi"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/service"
	"github.com/Muione/oci-start-go/internal/sysconf"
	"github.com/Muione/oci-start-go/internal/util/crypto"
	logpkg "github.com/Muione/oci-start-go/internal/util/log"
	"github.com/Muione/oci-start-go/internal/util/rsakey"
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

	deps := &httpapi.Deps{
		Store:      store,
		Cfg:        cfg,
		Logger:     logger,
		Keypair:    kpStore,
		Session:    sessionSvc,
		SysConf:    sc,
		Bypass:     bypass,
		OAuthState: oauthState,
		Tenant:     tenantSvc,
		ProxyPool:  proxyPool,
		MasterKey:  masterKey,
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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		logger.Error().Err(err).Msg("http shutdown error")
	}
	logger.Info().Msg("stopped")
}

// die is the pre-logger fatal path (config load). Post-logger errors use
// logger.Fatal().
func die(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %s: %v\n", msg, err)
		os.Exit(1)
	}
}

var _ = zerolog.Logger{} // keep zerolog import meaningful for future log calls
