// Package auth provides session management, the gin middleware chain
// (IpBan/SessionAuth/UserContext/TenantContext), and context helpers —
// Sa-Token parity. See SPEC §7, §6.1.
package auth

import (
	"context"

	"github.com/Muione/oci-start-go/internal/repo"
)

type ctxKey int

const (
	keyLoginUser ctxKey = iota
	keyUsername
	keyTenantID
	keyTenant
)

func WithLoginUser(ctx context.Context, u repo.LoginUser) context.Context {
	return context.WithValue(ctx, keyLoginUser, u)
}
func LoginUserFromContext(ctx context.Context) (repo.LoginUser, bool) {
	u, ok := ctx.Value(keyLoginUser).(repo.LoginUser)
	return u, ok
}

func WithUsername(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, keyUsername, name)
}
func UsernameFromContext(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(keyUsername).(string)
	return s, ok
}

func WithTenantID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, keyTenantID, id)
}
func TenantIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(keyTenantID).(int64)
	return id, ok
}

// WithTenant stores the loaded Tenant row (X-Tenant-Id → FindTenantByID) so
// downstream handlers/services can build an OCI provider without re-querying.
// The OCI provider itself is NOT stored here (would import oci → cycle);
// consumers build it lazily via oci.NewProvider from this Tenant.
func WithTenant(ctx context.Context, t repo.Tenant) context.Context {
	return context.WithValue(ctx, keyTenant, t)
}
func TenantFromContext(ctx context.Context) (repo.Tenant, bool) {
	t, ok := ctx.Value(keyTenant).(repo.Tenant)
	return t, ok
}
