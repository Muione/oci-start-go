// Package service -- container_registry.go: Phase 14.2 Container Registry service.
// Manages container repository and image CRUD, plus image cleanup.
// Parity with Java OcirController.
package service

import (
	"context"
	"fmt"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
)

// ContainerRegistryService manages OCI Container/Artifact Registry operations.
type ContainerRegistryService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool
}

// NewContainerRegistryService constructs a ContainerRegistryService.
func NewContainerRegistryService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *ContainerRegistryService {
	return &ContainerRegistryService{store: store, masterKey: masterKey, pool: pool}
}

// ListRepositories returns all container repositories in a compartment.
func (s *ContainerRegistryService) ListRepositories(ctx context.Context, tenantID int64, compartmentID string) ([]oci.ContainerRepositoryVO, error) {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := oci.Credentials{
		Tenancy:     nsStr(tenant.Tenancy),
		UserID:      nsStr(tenant.TenantID),
		Fingerprint: nsStr(tenant.Fingerprint),
		Region:      nsStr(tenant.Region),
		KeyFileBlob: nsStr(tenant.KeyFileBlob),
		KeyFile:     nsStr(tenant.KeyFile),
	}

	var result []oci.ContainerRepositoryVO
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		var innerErr error
		result, innerErr = oci.ListContainerRepositories(ctx, c.Artifacts, compartmentID)
		return innerErr
	})
	return result, err
}

// ListImages returns container images in a repository.
func (s *ContainerRegistryService) ListImages(ctx context.Context, tenantID int64, compartmentID, repositoryName string) ([]oci.ContainerImageVO, error) {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := oci.Credentials{
		Tenancy:     nsStr(tenant.Tenancy),
		UserID:      nsStr(tenant.TenantID),
		Fingerprint: nsStr(tenant.Fingerprint),
		Region:      nsStr(tenant.Region),
		KeyFileBlob: nsStr(tenant.KeyFileBlob),
		KeyFile:     nsStr(tenant.KeyFile),
	}

	var result []oci.ContainerImageVO
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		var innerErr error
		result, innerErr = oci.ListContainerImages(ctx, c.Artifacts, compartmentID, repositoryName)
		return innerErr
	})
	return result, err
}

// DeleteImage deletes a container image by ID.
func (s *ContainerRegistryService) DeleteImage(ctx context.Context, tenantID int64, imageID string) error {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := oci.Credentials{
		Tenancy:     nsStr(tenant.Tenancy),
		UserID:      nsStr(tenant.TenantID),
		Fingerprint: nsStr(tenant.Fingerprint),
		Region:      nsStr(tenant.Region),
		KeyFileBlob: nsStr(tenant.KeyFileBlob),
		KeyFile:     nsStr(tenant.KeyFile),
	}

	return oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		return oci.DeleteContainerImage(ctx, c.Artifacts, imageID)
	})
}

// DeleteRepository deletes a container repository by ID.
func (s *ContainerRegistryService) DeleteRepository(ctx context.Context, tenantID int64, repositoryID string) error {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := oci.Credentials{
		Tenancy:     nsStr(tenant.Tenancy),
		UserID:      nsStr(tenant.TenantID),
		Fingerprint: nsStr(tenant.Fingerprint),
		Region:      nsStr(tenant.Region),
		KeyFileBlob: nsStr(tenant.KeyFileBlob),
		KeyFile:     nsStr(tenant.KeyFile),
	}

	return oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		return oci.DeleteContainerRepository(ctx, c.Artifacts, repositoryID)
	})
}

// CleanupOldImages retains only the keepCount most recent images in a repository
// and deletes the rest.
func (s *ContainerRegistryService) CleanupOldImages(ctx context.Context, tenantID int64, compartmentID, repositoryName string, keepCount int) (*oci.CleanupResult, error) {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := oci.Credentials{
		Tenancy:     nsStr(tenant.Tenancy),
		UserID:      nsStr(tenant.TenantID),
		Fingerprint: nsStr(tenant.Fingerprint),
		Region:      nsStr(tenant.Region),
		KeyFileBlob: nsStr(tenant.KeyFileBlob),
		KeyFile:     nsStr(tenant.KeyFile),
	}

	var result *oci.CleanupResult
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		var innerErr error
		result, innerErr = oci.CleanupOldImages(ctx, c.Artifacts, compartmentID, repositoryName, keepCount)
		return innerErr
	})
	return result, err
}
