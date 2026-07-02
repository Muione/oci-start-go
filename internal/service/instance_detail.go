// Package service — instance_detail.go provides instance detail CRUD and
// query operations (Phase 5). Port of InstanceDetailsServiceImpl.java.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/util/crypto"
)

// InstanceDetailSvc manages instance_detail records.
type InstanceDetailSvc struct {
	store *db.Store
	// masterKey decrypts the at-rest root password (S4). nil before SetMasterKey
	// is wired; DecryptStringWithFallback then returns the raw value verbatim,
	// keeping reads correct for legacy plaintext rows and during bootstrap.
	// ponytail: not in NewInstanceDetailSvc to avoid changing its signature.
	masterKey []byte
	// tenantCache holds a tenantID -> userName map built from ListTenants, used
	// by List (which spans many tenants). Cached to avoid re-scanning the whole
	// tenant table on every List call.
	// ponytail: TTL-based invalidation; InstanceDetailSvc is not notified of
	// tenant save/delete, so a renamed tenant reflects in List after at most
	// tenantCacheTTL. GetByID/ListByTenant use FindTenantByID (always fresh).
	tenantCache atomic.Pointer[tenantNameCache]
}

// tenantNameCache is the cached id->name map plus its build time.
type tenantNameCache struct {
	m  map[int64]string
	at time.Time
}

// tenantCacheTTL is the max age of the List tenant-name cache before rebuild.
var tenantCacheTTL = 5 * time.Minute

func NewInstanceDetailSvc(store *db.Store) *InstanceDetailSvc {
	return &InstanceDetailSvc{store: store}
}

// SetMasterKey wires the AES-256-GCM master key used to decrypt the at-rest
// root password (S4). New exported setter — NewInstanceDetailSvc's signature is
// unchanged. nil/empty key leaves DecryptStringWithFallback returning the raw
// stored value, so callers stay correct before wiring and for legacy rows.
func (s *InstanceDetailSvc) SetMasterKey(key []byte) {
	s.masterKey = key
}

// GetRootPassword returns the decrypted root password for an instance detail.
// Reads the stored (encrypted) value and runs it through DecryptStringWithFallback:
// encrypted rows decrypt to plaintext; legacy plaintext rows (or a nil master key)
// return verbatim. This is the read-time downgrade path required by S4.
func (s *InstanceDetailSvc) GetRootPassword(ctx context.Context, id int64) (string, error) {
	r, err := repo.New(s.store.Read).FindInstanceDetailByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("find instance detail %d: %w", id, err)
	}
	return crypto.DecryptStringWithFallback(ns(r.Password), s.masterKey), nil
}

// InstanceDetailResp is the API-facing representation.
type InstanceDetailResp struct {
	ID                  int64  `json:"id"`
	TenantID            int64  `json:"tenantId"`
	TenantName          string `json:"tenantName"`
	InstanceID          string `json:"instanceId"`
	DisplayName         string `json:"displayName"`
	Shape               string `json:"shape"`
	State               string `json:"state"`
	Ocpus               int64  `json:"ocpus"`
	MemoryInGbs         int64  `json:"memoryInGbs"`
	BootVolumeSizeInGbs int64  `json:"bootVolumeSizeInGbs"`
	PublicIps           string `json:"publicIps"`
	PrivateIps          string `json:"privateIps"`
	AvailabilityDomain  string `json:"availabilityDomain"`
	CompartmentID       string `json:"compartmentId"`
	BootVolumeID        string `json:"bootVolumeId"`
	BootVolumeName      string `json:"bootVolumeName"`
	VpusPerGb           string `json:"vpusPerGb"`
	Ipv6Addresses       string `json:"ipv6Addresses"`
	VnicIds             string `json:"vnicIds"`
	Architecture        string `json:"architecture"`
	ConnTime            int64  `json:"connTime"`
	EnablePing          int64  `json:"enablePing"`
	OnLineEnable        int64  `json:"onLineEnable"`
	LastHeartbeat       string `json:"lastHeartbeat"`
	CreateTime          string `json:"createTime"`
}

// tenantNameMap builds (and caches) the full tenantID -> userName map. Used by
// List, which needs names for arbitrary tenants across the result page. The
// cache avoids a full-table scan on every List call.
func (s *InstanceDetailSvc) tenantNameMap(ctx context.Context) (map[int64]string, error) {
	if c := s.tenantCache.Load(); c != nil && time.Since(c.at) < tenantCacheTTL {
		return c.m, nil
	}
	tenants, err := repo.New(s.store.Read).ListTenants(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[int64]string, len(tenants))
	for _, t := range tenants {
		m[t.ID] = ns(t.UserName)
	}
	s.tenantCache.Store(&tenantNameCache{m: m, at: time.Now()})
	return m, nil
}

// tenantNameByID resolves a single tenant's name via FindTenantByID (one row),
// used by GetByID/ListByTenant where only one tenant is relevant. Always fresh,
// so a renamed tenant is visible immediately on these hot paths.
func (s *InstanceDetailSvc) tenantNameByID(ctx context.Context, tid int64) map[int64]string {
	if tid == 0 {
		return map[int64]string{}
	}
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tid)
	if err != nil {
		return map[int64]string{} // name resolution is best-effort
	}
	return map[int64]string{tid: ns(t.UserName)}
}

// GetByID returns a single instance detail by primary key.
func (s *InstanceDetailSvc) GetByID(ctx context.Context, id int64) (*InstanceDetailResp, error) {
	r, err := repo.New(s.store.Read).FindInstanceDetailByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find instance detail %d: %w", id, err)
	}
	tNames := s.tenantNameByID(ctx, ni(r.TenantID))
	resp := toDetailResp(r, tNames)
	return &resp, nil
}

// List returns paginated instance details, ordered by create_time desc.
func (s *InstanceDetailSvc) List(ctx context.Context, limit, offset int64) ([]InstanceDetailResp, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	q := repo.New(s.store.Read)
	total, _ := q.CountInstanceDetails(ctx)
	rows, err := q.ListAllInstanceDetails(ctx, repo.ListAllInstanceDetailsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list instance details: %w", err)
	}
	tNames, _ := s.tenantNameMap(ctx)
	out := make([]InstanceDetailResp, 0, len(rows))
	for _, r := range rows {
		out = append(out, toListDetailResp(r, tNames))
	}
	return out, total, nil
}

// ListByTenant returns all instance details for a given tenant.
func (s *InstanceDetailSvc) ListByTenant(ctx context.Context, tenantID int64) ([]InstanceDetailResp, error) {
	rows, err := repo.New(s.store.Read).FindInstancesByTenantId(ctx, sql.NullInt64{Int64: tenantID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("find instances by tenant %d: %w", tenantID, err)
	}
	tNames := s.tenantNameByID(ctx, tenantID)
	out := make([]InstanceDetailResp, 0, len(rows))
	for _, r := range rows {
		out = append(out, toFindByTenantResp(r, tNames))
	}
	return out, nil
}

// UpdateRemark updates the remark field for an instance.
func (s *InstanceDetailSvc) UpdateRemark(ctx context.Context, id int64, remark string) error {
	return repo.New(s.store.Write).UpdateInstanceDetailRemark(ctx, repo.UpdateInstanceDetailRemarkParams{
		Remark: sql.NullString{String: remark, Valid: true},
		ID:     id,
	})
}

// UpdateConnTime sets the last connection time for an instance.
func (s *InstanceDetailSvc) UpdateConnTime(ctx context.Context, id int64, now time.Time) error {
	nowStr := now.Format("2006-01-02 15:04:05")
	return repo.New(s.store.Write).UpdateInstanceConnTime(ctx, repo.UpdateInstanceConnTimeParams{
		ConnTime:      now.Unix(),
		LastHeartbeat: sql.NullString{String: nowStr, Valid: true},
		ID:            id,
	})
}

// MarkOffline marks an instance as offline.
func (s *InstanceDetailSvc) MarkOffline(ctx context.Context, id int64) error {
	return repo.New(s.store.Write).UpdateInstanceOffline(ctx, repo.UpdateInstanceOfflineParams{
		OnLineEnable:  0,
		OfflineNotify: 1,
		ID:            id,
	})
}

// MarkOnline marks an instance as back online (resume).
func (s *InstanceDetailSvc) MarkOnline(ctx context.Context, id int64) error {
	return repo.New(s.store.Write).UpdateInstanceResumeNotify(ctx, id)
}

// FindOfflineInstances returns instances that have been offline past the cutoff.
func (s *InstanceDetailSvc) FindOfflineInstances(ctx context.Context, cutoff time.Time) ([]repo.FindOfflineInstancesRow, error) {
	return repo.New(s.store.Read).FindOfflineInstances(ctx, sql.NullString{
		String: cutoff.Format("2006-01-02 15:04:05"),
		Valid:  true,
	})
}

// BackupListResp is the API representation of a backup record.
type BackupListResp struct {
	ID                  int64  `json:"id"`
	TenantID            int64  `json:"tenantId"`
	InstanceID          string `json:"instanceId"`
	DisplayName         string `json:"displayName"`
	Shape               string `json:"shape"`
	State               string `json:"state"`
	BootVolumeSizeInGbs int64  `json:"bootVolumeSizeInGbs"`
	PublicIps           string `json:"publicIps"`
	BootVolumeName      string `json:"bootVolumeName"`
	Architecture        string `json:"architecture"`
}

// ListBackups returns backup records for a tenant.
func (s *InstanceDetailSvc) ListBackups(ctx context.Context, tenantID int64) ([]BackupListResp, error) {
	rows, err := repo.New(s.store.Read).FindInstanceBackupsByTenantId(ctx, sql.NullInt64{Int64: tenantID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("find backups by tenant %d: %w", tenantID, err)
	}
	out := make([]BackupListResp, 0, len(rows))
	for _, r := range rows {
		out = append(out, BackupListResp{
			ID:                  r.ID,
			TenantID:            ni(r.TenantID),
			InstanceID:          ns(r.InstanceID),
			DisplayName:         ns(r.DisplayName),
			Shape:               ns(r.Shape),
			State:               ns(r.State),
			BootVolumeSizeInGbs: ni(r.BootVolumeSizeInGbs),
			PublicIps:           ns(r.PublicIps),
			BootVolumeName:      ns(r.BootVolumeName),
			Architecture:        ns(r.Architecture),
		})
	}
	return out, nil
}

// DeleteBackup deletes a backup record by id.
func (s *InstanceDetailSvc) DeleteBackup(ctx context.Context, id int64) error {
	return repo.New(s.store.Write).DeleteInstanceBackupDetail(ctx, id)
}

// InsertBackup creates a backup record.
func (s *InstanceDetailSvc) InsertBackup(ctx context.Context, params repo.InsertInstanceBackupDetailParams) error {
	return repo.New(s.store.Write).InsertInstanceBackupDetail(ctx, params)
}

// --- converters from sqlc Row types to API response ---

func toDetailResp(r repo.FindInstanceDetailByIDRow, tNames map[int64]string) InstanceDetailResp {
	tid := ni(r.TenantID)
	return InstanceDetailResp{
		ID:                  r.ID,
		TenantID:            tid,
		TenantName:          tNames[tid],
		InstanceID:          ns(r.InstanceID),
		DisplayName:         ns(r.DisplayName),
		Shape:               ns(r.Shape),
		State:               ns(r.State),
		Ocpus:               ni(r.Ocpus),
		MemoryInGbs:         ni(r.MemoryInGbs),
		BootVolumeSizeInGbs: ni(r.BootVolumeSizeInGbs),
		PublicIps:           ns(r.PublicIps),
		PrivateIps:          ns(r.PrivateIps),
		AvailabilityDomain:  ns(r.AvailabilityDomain),
		CompartmentID:       ns(r.CompartmentID),
		BootVolumeID:        ns(r.BootVolumeID),
		BootVolumeName:      ns(r.BootVolumeName),
		VpusPerGb:           ns(r.VpusPerGb),
		Ipv6Addresses:       ns(r.Ipv6Addresses),
		VnicIds:             ns(r.VnicIds),
		Architecture:        ns(r.Architecture),
		ConnTime:            r.ConnTime,
		EnablePing:          r.EnablePing,
		OnLineEnable:        r.OnLineEnable,
		LastHeartbeat:       ns(r.LastHeartbeat),
		CreateTime:          ns(r.CreateTime),
	}
}

func toListDetailResp(r repo.ListAllInstanceDetailsRow, tNames map[int64]string) InstanceDetailResp {
	tid := ni(r.TenantID)
	return InstanceDetailResp{
		ID:                  r.ID,
		TenantID:            tid,
		TenantName:          tNames[tid],
		InstanceID:          ns(r.InstanceID),
		DisplayName:         ns(r.DisplayName),
		Shape:               ns(r.Shape),
		State:               ns(r.State),
		Ocpus:               ni(r.Ocpus),
		MemoryInGbs:         ni(r.MemoryInGbs),
		BootVolumeSizeInGbs: ni(r.BootVolumeSizeInGbs),
		PublicIps:           ns(r.PublicIps),
		PrivateIps:          ns(r.PrivateIps),
		AvailabilityDomain:  ns(r.AvailabilityDomain),
		CompartmentID:       ns(r.CompartmentID),
		BootVolumeID:        ns(r.BootVolumeID),
		BootVolumeName:      ns(r.BootVolumeName),
		VpusPerGb:           ns(r.VpusPerGb),
		Ipv6Addresses:       ns(r.Ipv6Addresses),
		VnicIds:             ns(r.VnicIds),
		Architecture:        ns(r.Architecture),
		ConnTime:            r.ConnTime,
		EnablePing:          r.EnablePing,
		OnLineEnable:        r.OnLineEnable,
		LastHeartbeat:       ns(r.LastHeartbeat),
		CreateTime:          ns(r.CreateTime),
	}
}

func toFindByTenantResp(r repo.FindInstancesByTenantIdRow, tNames map[int64]string) InstanceDetailResp {
	tid := ni(r.TenantID)
	return InstanceDetailResp{
		ID:                  r.ID,
		TenantID:            tid,
		TenantName:          tNames[tid],
		InstanceID:          ns(r.InstanceID),
		DisplayName:         ns(r.DisplayName),
		Shape:               ns(r.Shape),
		State:               ns(r.State),
		Ocpus:               ni(r.Ocpus),
		MemoryInGbs:         ni(r.MemoryInGbs),
		BootVolumeSizeInGbs: ni(r.BootVolumeSizeInGbs),
		PublicIps:           ns(r.PublicIps),
		PrivateIps:          ns(r.PrivateIps),
		AvailabilityDomain:  ns(r.AvailabilityDomain),
		CompartmentID:       ns(r.CompartmentID),
		BootVolumeID:        ns(r.BootVolumeID),
		BootVolumeName:      ns(r.BootVolumeName),
		VpusPerGb:           ns(r.VpusPerGb),
		Ipv6Addresses:       ns(r.Ipv6Addresses),
		VnicIds:             ns(r.VnicIds),
		Architecture:        ns(r.Architecture),
		ConnTime:            r.ConnTime,
		EnablePing:          r.EnablePing,
		OnLineEnable:        r.OnLineEnable,
		LastHeartbeat:       ns(r.LastHeartbeat),
		CreateTime:          ns(r.CreateTime),
	}
}
