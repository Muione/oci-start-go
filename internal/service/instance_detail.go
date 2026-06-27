// Package service — instance_detail.go provides instance detail CRUD and
// query operations (Phase 5). Port of InstanceDetailsServiceImpl.java.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/repo"
)

// InstanceDetailSvc manages instance_detail records.
type InstanceDetailSvc struct {
	store *db.Store
}

func NewInstanceDetailSvc(store *db.Store) *InstanceDetailSvc {
	return &InstanceDetailSvc{store: store}
}

// InstanceDetailResp is the API-facing representation.
type InstanceDetailResp struct {
	ID                  int64  `json:"id"`
	TenantID            int64  `json:"tenantId"`
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

// GetByID returns a single instance detail by primary key.
func (s *InstanceDetailSvc) GetByID(ctx context.Context, id int64) (*InstanceDetailResp, error) {
	r, err := repo.New(s.store.Read).FindInstanceDetailByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find instance detail %d: %w", id, err)
	}
	resp := toDetailResp(r)
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
	out := make([]InstanceDetailResp, 0, len(rows))
	for _, r := range rows {
		out = append(out, toListDetailResp(r))
	}
	return out, total, nil
}

// ListByTenant returns all instance details for a given tenant.
func (s *InstanceDetailSvc) ListByTenant(ctx context.Context, tenantID int64) ([]InstanceDetailResp, error) {
	rows, err := repo.New(s.store.Read).FindInstancesByTenantId(ctx, sql.NullInt64{Int64: tenantID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("find instances by tenant %d: %w", tenantID, err)
	}
	out := make([]InstanceDetailResp, 0, len(rows))
	for _, r := range rows {
		out = append(out, toFindByTenantResp(r))
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

func toDetailResp(r repo.FindInstanceDetailByIDRow) InstanceDetailResp {
	return InstanceDetailResp{
		ID:                  r.ID,
		TenantID:            ni(r.TenantID),
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

func toListDetailResp(r repo.ListAllInstanceDetailsRow) InstanceDetailResp {
	return InstanceDetailResp{
		ID:                  r.ID,
		TenantID:            ni(r.TenantID),
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

func toFindByTenantResp(r repo.FindInstancesByTenantIdRow) InstanceDetailResp {
	return InstanceDetailResp{
		ID:                  r.ID,
		TenantID:            ni(r.TenantID),
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
