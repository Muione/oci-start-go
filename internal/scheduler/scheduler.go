// Package scheduler manages cron jobs using robfig/cron/v3 (SPEC S9).
// Replaces the Quartz JDBC scheduler from the Java project. Jobs are registered
// at startup with hardcoded cron expressions (parity with SPEC S9.1).
// The CreateInstanceJob (every 6s) triggers the grab engine.
package scheduler

import (
	"context"
	"database/sql"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/acme"
	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/grabber"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/service"
	"github.com/Muione/oci-start-go/internal/sysconf"
	"github.com/Muione/oci-start-go/internal/ws"
)

// Scheduler owns the robfig/cron instance and all registered jobs.
type Scheduler struct {
	cron   *cron.Cron
	engine *grabber.Engine
	store  *db.Store
	logger zerolog.Logger

	// Phase 5 services (wired for real job implementations).
	trafficSvc   *service.TrafficSvc
	checkLiveSvc *service.CheckLiveSvc
	pingSvc      *service.PingSvc
	offlineSvc   *service.OfflineSvc

	// Phase 8: ACME cert manager for SslCertJob.
	certManager *acme.CertManager

	// Phase 8: WebSocket hub for MonitorFlashHeartbeatJob.
	wsHub *ws.Hub
}

// SvcSet bundles services for the scheduler to consume.
type SvcSet struct {
	Traffic     *service.TrafficSvc
	CheckLive   *service.CheckLiveSvc
	Ping        *service.PingSvc
	Offline     *service.OfflineSvc
	CertManager *acme.CertManager // Phase 8: SSL cert renewal
	WsHub       *ws.Hub           // Phase 8: monitor heartbeat broadcast
}

// New creates the scheduler, registering all jobs. The engine may be nil
// (e.g. for phases where the grab engine isn't ready yet); grab-dependent
// jobs are skipped when engine is nil. Phase 5+ services fill in previously
// stubbed job implementations.
func New(engine *grabber.Engine, store *db.Store, logger zerolog.Logger, svcs *SvcSet) *Scheduler {
	s := &Scheduler{
		cron: cron.New(
			cron.WithSeconds(),
		),
		engine: engine,
		store:  store,
		logger: logger,
	}
	if svcs != nil {
		s.trafficSvc = svcs.Traffic
		s.checkLiveSvc = svcs.CheckLive
		s.pingSvc = svcs.Ping
		s.offlineSvc = svcs.Offline
		s.certManager = svcs.CertManager
		s.wsHub = svcs.WsHub
	}
	s.registerJobs()
	return s
}

func (s *Scheduler) registerJobs() {
	// CreateInstanceJob: every 6 seconds.
	if s.engine != nil {
		_, _ = s.cron.AddFunc("*/6 * * * * *", func() {
			ctx := context.Background()
			s.engine.CheckAndExecuteTasksOnce(ctx)
		})
	}

	// InstanceTrafficJob: every 30 minutes (Phase 5).
	if s.trafficSvc != nil {
		_, _ = s.cron.AddFunc("0 */30 * * * *", func() {
			s.trafficSvc.CheckAllTenantsTraffic(context.Background())
		})
	} else {
		_, _ = s.cron.AddFunc("0 */5 * * * *", func() {
			s.logger.Debug().Msg("scheduler: InstanceTrafficJob tick (stub)")
		})
	}

	// CheckLiveJob: every hour at :00 (Phase 5).
	if s.checkLiveSvc != nil {
		_, _ = s.cron.AddFunc("0 0 * * * *", func() {
			s.checkLiveSvc.CheckAccountLive(context.Background())
		})
	} else {
		_, _ = s.cron.AddFunc("0 0 * * * *", func() {
			s.logger.Debug().Msg("scheduler: CheckLiveJob tick (stub)")
		})
	}

	// PingConnTimeJob: every 5 minutes (Phase 5).
	if s.pingSvc != nil {
		_, _ = s.cron.AddFunc("0 */5 * * * *", func() {
			s.pingSvc.CheckPingConn(context.Background())
		})
	} else {
		_, _ = s.cron.AddFunc("0 */5 * * * *", func() {
			s.logger.Debug().Msg("scheduler: PingConnTimeJob tick (stub)")
		})
	}

	// SslCertJob: daily at 04:00 (Phase 8 — real ACME renewal).
	_, _ = s.cron.AddFunc("0 0 4 * * *", func() {
		s.sslCertJob()
	})

	// BootInstanceRefreshJob: daily at 00:00 — reset daily attempt counters.
	_, _ = s.cron.AddFunc("0 0 0 * * *", func() {
		s.bootInstanceRefreshJob()
	})

	// MonitorFlashHeartbeatJob: every 15 seconds (Phase 8).
	_, _ = s.cron.AddFunc("*/15 * * * * *", func() {
		s.monitorHeartbeatJob()
	})

	// CheckOfflineInstanceJob: every 1 minute (Phase 5).
	if s.offlineSvc != nil {
		_, _ = s.cron.AddFunc("0 */1 * * * *", func() {
			s.offlineSvc.CheckOfflineInstances(context.Background())
		})
	} else {
		_, _ = s.cron.AddFunc("0 */1 * * * *", func() {
			s.logger.Debug().Msg("scheduler: CheckOfflineInstanceJob tick (stub)")
		})
	}

	// MultipartUploadCleanupJob: daily at 02:00 (Phase 8 — OCI object storage
	// multipart upload cleanup. In the SQLite-based Go version, this is a no-op
	// unless OCI object storage is configured).
	_, _ = s.cron.AddFunc("0 0 2 * * *", func() {
		s.multipartCleanupJob()
	})

	s.logger.Info().Int("jobs", len(s.cron.Entries())).Msg("scheduler: jobs registered")
}

func (s *Scheduler) bootInstanceRefreshJob() {
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")
	q := repo.New(s.store.Write)

	if err := q.ResetDailyCounts(ctx, sql.NullString{String: today, Valid: true}); err != nil {
		s.logger.Error().Err(err).Msg("scheduler: reset daily counts failed")
	}
	if err := q.MarkDailyReset(ctx, sql.NullString{String: today, Valid: true}); err != nil {
		s.logger.Error().Err(err).Msg("scheduler: mark daily reset failed")
	}

	s.logger.Info().Str("date", today).Msg("scheduler: BootInstanceRefreshJob completed")
}

// sslCertJob checks all configured SSL certificates and renews those nearing
// expiration (within 30 days). Uses the ACME CertManager with Cloudflare DNS.
func (s *Scheduler) sslCertJob() {
	if s.certManager == nil {
		s.logger.Debug().Msg("scheduler: SslCertJob skipped — cert manager not configured")
		return
	}

	ctx := context.Background()
	sc := sysconf.New(s.store)

	// Read SSL config from system config.
	domain := sc.GetString(ctx, "ssl.domain")
	email := sc.GetString(ctx, "ssl.email")
	cfAPIToken := sc.GetString(ctx, "cloudflare.api.token")
	staging := sc.GetBool(ctx, "ssl.staging")

	if domain == "" || email == "" || cfAPIToken == "" {
		s.logger.Debug().Msg("scheduler: SslCertJob skipped — SSL/CF config incomplete")
		return
	}

	s.logger.Info().Str("domain", domain).Msg("scheduler: SslCertJob checking cert renewal")

	result, err := s.certManager.RenewCertificate(ctx, domain, email, cfAPIToken, staging)
	if err != nil {
		s.logger.Error().Err(err).Str("domain", domain).Msg("scheduler: SslCertJob cert renewal failed")
		return
	}

	s.logger.Info().
		Str("domain", result.Domain).
		Str("notAfter", result.NotAfter).
		Msg("scheduler: SslCertJob cert renewed successfully")
}

// monitorHeartbeatJob broadcasts a heartbeat ping to all connected monitor
// WebSocket clients, allowing dashboards to detect stale connections.
// Runs every 15 seconds.
func (s *Scheduler) monitorHeartbeatJob() {
	if s.wsHub == nil || s.wsHub.Monitor == nil {
		return
	}

	count := s.wsHub.Monitor.OnlineCount()
	if count > 0 {
		s.wsHub.Monitor.Broadcast(ws.MonitorReportDTO{
			Type: "heartbeat",
		})
		s.logger.Debug().Int("clients", count).Msg("scheduler: monitor heartbeat broadcast")
	}
}

// multipartCleanupJob cleans up stale OCI object storage multipart uploads.
// In the SQLite-based Go version, database maintenance replaces this.
// If no OCI object storage client is configured, runs a DB vacuum instead.
func (s *Scheduler) multipartCleanupJob() {
	ctx := context.Background()
	sc := sysconf.New(s.store)

	// If OCI object storage is configured, attempt multipart cleanup.
	bucket := sc.GetString(ctx, "oci.objectstorage.bucket")
	if bucket != "" {
		s.logger.Debug().Str("bucket", bucket).Msg("scheduler: MultipartUploadCleanupJob — OCI OS cleanup not yet integrated (use OCI console)")
		// Full OCI ObjectStorage multipart cleanup requires the ObjectStorageClient.
		// This is a no-op for now — the bucket should have lifecycle rules.
		return
	}

	// Otherwise, run SQLite WAL checkpoint as database maintenance.
	_, err := s.store.Write.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	if err != nil {
		s.logger.Warn().Err(err).Msg("scheduler: MultipartCleanupJob wal_checkpoint failed")
	} else {
		s.logger.Debug().Msg("scheduler: MultipartCleanupJob wal_checkpoint completed")
	}
}

// Start begins the cron scheduler. Non-blocking; runs in background goroutines.
func (s *Scheduler) Start() {
	s.cron.Start()
	s.logger.Info().Msg("scheduler: started")
}

// Stop gracefully stops the cron scheduler, waiting for running jobs to finish.
func (s *Scheduler) Stop() context.Context {
	ctx := s.cron.Stop()
	s.logger.Info().Msg("scheduler: stopped")
	return ctx
}
