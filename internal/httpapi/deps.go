package httpapi

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/auth"
	"github.com/Muione/oci-start-go/internal/bootstrap"
	"github.com/Muione/oci-start-go/internal/config"
	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/service"
	"github.com/Muione/oci-start-go/internal/sysconf"
	"github.com/Muione/oci-start-go/internal/util/rsakey"
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
}
