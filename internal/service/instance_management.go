// Package service — instance_management.go: Phase 2 instance management.
// Handles VPU update (BE-102), Shape listing (BE-106), and Image listing (BE-107)
// with database caching for shapes and images.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
)

// InstanceManagementService provides instance configuration and resource listing.
type InstanceManagementService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool
}

// NewInstanceManagementService constructs an InstanceManagementService.
func NewInstanceManagementService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *InstanceManagementService {
	return &InstanceManagementService{store: store, masterKey: masterKey, pool: pool}
}

// --- VPU Update (BE-102) ---

// UpdateBootVolumeVpu updates the VPU performance of an instance's boot volume.
// Requires the instance to be stopped first.
func (s *InstanceManagementService) UpdateBootVolumeVpu(ctx context.Context, instanceID string, vpusPerGB int64) (*oci.BootVolumeInfo, error) {
	if vpusPerGB < 10 || vpusPerGB > 120 {
		return nil, fmt.Errorf("VPUsPerGB must be between 10 and 120, got %d", vpusPerGB)
	}

	_, rowID, bootVolumeID, creds, err := s.resolveInstanceForVPU(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if bootVolumeID == "" {
		return nil, fmt.Errorf("instance %s has no boot volume recorded", instanceID)
	}

	var result *oci.BootVolumeInfo
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		bv, err := oci.UpdateBootVolumeVpu(ctx, c, bootVolumeID, vpusPerGB)
		if err != nil {
			return err
		}
		result = &oci.BootVolumeInfo{
			ID:         derefStr(bv.Id),
			DisplayName: derefStr(bv.DisplayName),
			SizeInGBs:  derefInt64(bv.SizeInGBs),
			VpusPerGB:  derefInt64(bv.VpusPerGB),
		}
		// Update local DB record.
		_ = repo.New(s.store.Write).UpdateInstanceDetailVpusPerGb(ctx, repo.UpdateInstanceDetailVpusPerGbParams{
			VpusPerGb: sql.NullString{String: fmt.Sprintf("%d", vpusPerGB), Valid: true},
			ID:        rowID,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// --- Shape Listing (BE-106) ---

// ListShapes returns available shapes for a tenant, using cache when fresh.
// If forceRefresh is true or cache is stale (>1h), fetches from OCI API.
func (s *InstanceManagementService) ListShapes(ctx context.Context, tenantID int64, architecture string, forceRefresh bool) ([]oci.ShapeInfo, error) {
	// Check cache freshness unless forced.
	if !forceRefresh {
		cached, err := s.loadShapeCache(ctx, tenantID, architecture)
		if err == nil && len(cached) > 0 {
			return cached, nil
		}
	}

	// Fetch from OCI API.
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	creds := tenantToCreds(t)

	var shapes []oci.ShapeInfo
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		// Use tenancy OCID as compartment (root compartment for shapes).
		compartmentID := creds.Tenancy
		var innerErr error
		shapes, innerErr = oci.ListShapesFiltered(ctx, c, compartmentID, "", architecture)
		return innerErr
	})
	if err != nil {
		return nil, err
	}

	// Update cache.
	go s.refreshShapeCache(context.Background(), tenantID, creds.Tenancy, "", shapes)

	return shapes, nil
}

// --- Image Listing (BE-107) ---

// ListImages returns available images for a tenant, using cache when fresh.
// If forceRefresh is true or cache is stale (>1h), fetches from OCI API.
func (s *InstanceManagementService) ListImages(ctx context.Context, tenantID int64, shape, architecture string, forceRefresh bool) ([]oci.ImageInfo, error) {
	// Check cache freshness unless forced.
	if !forceRefresh {
		cached, err := s.loadImageCache(ctx, tenantID, architecture)
		if err == nil && len(cached) > 0 {
			return cached, nil
		}
	}

	// Fetch from OCI API.
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	creds := tenantToCreds(t)

	var images []oci.ImageInfo
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		compartmentID := creds.Tenancy
		var innerErr error
		images, innerErr = oci.ListImagesFiltered(ctx, c, compartmentID, shape, architecture)
		return innerErr
	})
	if err != nil {
		return nil, err
	}

	// Update cache.
	go s.refreshImageCache(context.Background(), tenantID, creds.Tenancy, images)

	return images, nil
}

// --- Cache helpers ---

const cacheStaleDuration = 1 * time.Hour

func (s *InstanceManagementService) loadShapeCache(ctx context.Context, tenantID int64, architecture string) ([]oci.ShapeInfo, error) {
	var rows []repo.ShapeCache
	var err error
	if architecture != "" {
		rows, err = repo.New(s.store.Read).ListShapeCacheByTenantAndArch(ctx, repo.ListShapeCacheByTenantAndArchParams{
			TenantID:     tenantID,
			Architecture: architecture,
		})
	} else {
		rows, err = repo.New(s.store.Read).ListShapeCacheByTenant(ctx, tenantID)
	}
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("cache empty")
	}
	// Check freshness.
	lastSynced := rows[0].LastSyncedAt
	t, parseErr := time.Parse("2006-01-02 15:04:05", lastSynced)
	if parseErr != nil || time.Since(t) > cacheStaleDuration {
		return nil, fmt.Errorf("cache stale")
	}

	result := make([]oci.ShapeInfo, 0, len(rows))
	for _, r := range rows {
		info := oci.ShapeInfo{
			Shape:         r.Shape,
			Architecture:  r.Architecture,
			IsFlexible:    r.IsFlexible != 0,
			ProcessorDesc: ns2(r.ProcessorDescription),
		}
		if r.Ocpus.Valid {
			info.Ocpus = float32(r.Ocpus.Float64)
		}
		if r.MemoryInGbs.Valid {
			info.MemoryInGBs = float32(r.MemoryInGbs.Float64)
		}
		if r.MaxVnicAttachments.Valid {
			info.MaxVnicAttachments = int(r.MaxVnicAttachments.Int64)
		}
		if r.GpuDescription.Valid {
			info.GpuDescription = r.GpuDescription.String
		}
		if r.GpuCount.Valid {
			info.GpuCount = int(r.GpuCount.Int64)
		}
		if r.LocalDiskDescription.Valid {
			info.LocalDiskDesc = r.LocalDiskDescription.String
		}
		if r.NetworkingDescription.Valid {
			info.NetworkingDesc = r.NetworkingDescription.String
		}
		if r.BaselineOcpu.Valid {
			baseline := float32(r.BaselineOcpu.Float64)
			info.BaselineOcpu = &baseline
		}
		result = append(result, info)
	}
	return result, nil
}

func (s *InstanceManagementService) loadImageCache(ctx context.Context, tenantID int64, architecture string) ([]oci.ImageInfo, error) {
	var rows []repo.ImageCache
	var err error
	if architecture != "" {
		rows, err = repo.New(s.store.Read).ListImageCacheByTenantAndArch(ctx, repo.ListImageCacheByTenantAndArchParams{
			TenantID:     tenantID,
			Architecture: architecture,
		})
	} else {
		rows, err = repo.New(s.store.Read).ListImageCacheByTenant(ctx, tenantID)
	}
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("cache empty")
	}
	// Check freshness.
	lastSynced := rows[0].LastSyncedAt
	t, parseErr := time.Parse("2006-01-02 15:04:05", lastSynced)
	if parseErr != nil || time.Since(t) > cacheStaleDuration {
		return nil, fmt.Errorf("cache stale")
	}

	result := make([]oci.ImageInfo, 0, len(rows))
	for _, r := range rows {
		info := oci.ImageInfo{
			ID:                 r.ImageID,
			DisplayName:        ns2(r.DisplayName),
			OperatingSystem:    ns2(r.OperatingSystem),
			OperatingSystemVer: ns2(r.OperatingSystemVersion),
			Architecture:       r.Architecture,
			LaunchMode:         ns2(r.LaunchMode),
			TimeCreated:        ns2(r.TimeCreated),
		}
		if r.SizeInGbs.Valid {
			info.SizeInGBs = &r.SizeInGbs.Int64
		}
		result = append(result, info)
	}
	return result, nil
}

func (s *InstanceManagementService) refreshShapeCache(ctx context.Context, tenantID int64, compartmentID, adName string, shapes []oci.ShapeInfo) {
	q := repo.New(s.store.Write)
	now := time.Now().Format("2006-01-02 15:04:05")
	// Clear old cache.
	_ = q.DeleteShapeCacheByTenant(ctx, tenantID)
	for _, sh := range shapes {
		params := repo.UpsertShapeCacheParams{
			TenantID:           tenantID,
			CompartmentID:      compartmentID,
			Shape:              sh.Shape,
			Architecture:       sh.Architecture,
			IsFlexible:         boolToInt64(sh.IsFlexible),
			LastSyncedAt:       now,
		}
		if adName != "" {
			params.AvailabilityDomain = sql.NullString{String: adName, Valid: true}
		}
		if sh.Ocpus > 0 {
			params.Ocpus = sql.NullFloat64{Float64: float64(sh.Ocpus), Valid: true}
		}
		if sh.MemoryInGBs > 0 {
			params.MemoryInGbs = sql.NullFloat64{Float64: float64(sh.MemoryInGBs), Valid: true}
		}
		if sh.ProcessorDesc != "" {
			params.ProcessorDescription = sql.NullString{String: sh.ProcessorDesc, Valid: true}
		}
		if sh.MaxVnicAttachments > 0 {
			params.MaxVnicAttachments = sql.NullInt64{Int64: int64(sh.MaxVnicAttachments), Valid: true}
		}
		if sh.GpuDescription != "" {
			params.GpuDescription = sql.NullString{String: sh.GpuDescription, Valid: true}
		}
		if sh.GpuCount > 0 {
			params.GpuCount = sql.NullInt64{Int64: int64(sh.GpuCount), Valid: true}
		}
		if sh.LocalDiskDesc != "" {
			params.LocalDiskDescription = sql.NullString{String: sh.LocalDiskDesc, Valid: true}
		}
		if sh.NetworkingDesc != "" {
			params.NetworkingDescription = sql.NullString{String: sh.NetworkingDesc, Valid: true}
		}
		if sh.BaselineOcpu != nil {
			params.BaselineOcpu = sql.NullFloat64{Float64: float64(*sh.BaselineOcpu), Valid: true}
		}
		_ = q.UpsertShapeCache(ctx, params)
	}
}

func (s *InstanceManagementService) refreshImageCache(ctx context.Context, tenantID int64, compartmentID string, images []oci.ImageInfo) {
	q := repo.New(s.store.Write)
	now := time.Now().Format("2006-01-02 15:04:05")
	// Clear old cache.
	_ = q.DeleteImageCacheByTenant(ctx, tenantID)
	for _, img := range images {
		params := repo.UpsertImageCacheParams{
			TenantID:      tenantID,
			CompartmentID: compartmentID,
			ImageID:       img.ID,
			Architecture:  img.Architecture,
			LastSyncedAt:  now,
		}
		if img.DisplayName != "" {
			params.DisplayName = sql.NullString{String: img.DisplayName, Valid: true}
		}
		if img.OperatingSystem != "" {
			params.OperatingSystem = sql.NullString{String: img.OperatingSystem, Valid: true}
		}
		if img.OperatingSystemVer != "" {
			params.OperatingSystemVersion = sql.NullString{String: img.OperatingSystemVer, Valid: true}
		}
		if img.SizeInGBs != nil {
			params.SizeInGbs = sql.NullInt64{Int64: *img.SizeInGBs, Valid: true}
		}
		if img.LaunchMode != "" {
			params.LaunchMode = sql.NullString{String: img.LaunchMode, Valid: true}
		}
		if img.TimeCreated != "" {
			params.TimeCreated = sql.NullString{String: img.TimeCreated, Valid: true}
		}
		_ = q.UpsertImageCache(ctx, params)
	}
}

// resolveInstanceForVPU resolves instanceId -> tenant + boot volume ID.
func (s *InstanceManagementService) resolveInstanceForVPU(ctx context.Context, instanceID string) (*repo.Tenant, int64, string, oci.Credentials, error) {
	var rowID int64
	var tenantID sql.NullInt64
	var bootVolumeID sql.NullString
	err := s.store.Read.QueryRowContext(ctx,
		`SELECT id, tenant_id, boot_volume_id
		 FROM instance_detail WHERE instance_id = ? LIMIT 1`,
		instanceID).Scan(&rowID, &tenantID, &bootVolumeID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, 0, "", oci.Credentials{}, fmt.Errorf("instance not found: %s", instanceID)
		}
		return nil, 0, "", oci.Credentials{}, fmt.Errorf("query instance: %w", err)
	}
	if !tenantID.Valid {
		return nil, 0, "", oci.Credentials{}, fmt.Errorf("instance %s has no tenant", instanceID)
	}

	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID.Int64)
	if err != nil {
		return nil, 0, "", oci.Credentials{}, fmt.Errorf("find tenant: %w", err)
	}
	creds := tenantToCreds(t)
	bvID := ""
	if bootVolumeID.Valid {
		bvID = bootVolumeID.String
	}
	return &t, rowID, bvID, creds, nil
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
