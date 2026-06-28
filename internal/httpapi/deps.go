package httpapi

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/auth"
	"github.com/Muione/oci-start-go/internal/bootstrap"
	"github.com/Muione/oci-start-go/internal/config"
	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/dns"
	"github.com/Muione/oci-start-go/internal/grabber"
	"github.com/Muione/oci-start-go/internal/notify"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/scheduler"
	"github.com/Muione/oci-start-go/internal/acme"
	"github.com/Muione/oci-start-go/internal/cloud/gcp"
	"github.com/Muione/oci-start-go/internal/service"
	"github.com/Muione/oci-start-go/internal/sysconf"
	"github.com/Muione/oci-start-go/internal/util/rsakey"
	"github.com/Muione/oci-start-go/internal/ws"
)

const httpTimeFmt = "2006-01-02 15:04:05"

func nowStr() string { return time.Now().Format(httpTimeFmt) }

// Deps bundles the server's dependencies, wired in main.go and passed to
// handler factories.
type Deps struct {
	Store      *db.Store
	Cfg        *config.Config
	Logger     zerolog.Logger
	Keypair    *rsakey.KeypairStore
	Session    *auth.SessionService
	SysConf    *sysconf.Service
	Bypass     *bootstrap.BypassTokenHolder
	OAuthState *StateCache
	Tenant     *service.TenantService
	ProxyPool  *oci.ProxyPool
	MasterKey  []byte

	// Phase 4: grab engine + scheduler + boot task CRUD.
	Engine    *grabber.Engine
	Scheduler *scheduler.Scheduler
	Boot      *service.BootService

	// Phase 5: instance details, traffic, backup, check-live, ping, offline.
	InstanceSvc *service.InstanceDetailSvc
	TrafficSvc  *service.TrafficSvc
	BackupSvc   *service.BackupSvc
	CheckLiveSvc *service.CheckLiveSvc
	PingSvc     *service.PingSvc
	OfflineSvc  *service.OfflineSvc

	// Phase 6: WebSocket hub.
	WsHub *ws.Hub

	// Phase 7: notification, DNS.
	Notifier notify.Notifier
	DnsSvc   *dns.DnsService

	// Phase 8: ACME cert manager for SSL issuance.
	CertManager *acme.CertManager

	// Phase 8: data migration.
	Migration *MigrationHandler

	// Phase 8: GCP Compute Engine boot instance service.
	GcpSvc *gcp.GcpService

	// Phase 9: tenant email & social config services.
	TenantEmail  *service.TenantEmailService
	TenantSocial *service.TenantSocialService

	// Phase 10: tenant IAM user management.
	TenantUser *service.TenantUserService
}
