// Package service -- mysql.go: Phase 13.3 MySQL Database Service.
// Manages MySQL DB System CRUD, backup management, and channel (replication)
// operations. Delegates to oci.MySQLOps wrappers.
// Parity with Java OciMysqlController.
package service

import (
	"context"
	"fmt"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/oracle/oci-go-sdk/v65/mysql"
)

// MySQLService manages OCI MySQL HeatWave operations.
type MySQLService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool
}

// NewMySQLService constructs a MySQLService.
func NewMySQLService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *MySQLService {
	return &MySQLService{store: store, masterKey: masterKey, pool: pool}
}

// ---------------------------------------------------------------------------
// DB System Operations
// ---------------------------------------------------------------------------

// ListDbSystems lists all MySQL DB Systems in a compartment.
func (s *MySQLService) ListDbSystems(ctx context.Context, tenantID int64, compartmentID string, displayName string, limit int, page string) ([]mysql.DbSystemSummary, *string, error) {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := buildCreds(tenant)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, nil, fmt.Errorf("provider: %w", err)
	}
	client, err := mysql.NewDbSystemClientWithConfigurationProvider(prov)
	if err != nil {
		return nil, nil, fmt.Errorf("db system client: %w", err)
	}
	return (&oci.MySQLOps{}).ListDbSystems(ctx, &client, compartmentID, displayName, limit, page)
}

// GetDbSystem retrieves details of a MySQL DB System.
func (s *MySQLService) GetDbSystem(ctx context.Context, tenantID int64, dbSystemID string) (*mysql.DbSystem, error) {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := buildCreds(tenant)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("provider: %w", err)
	}
	client, err := mysql.NewDbSystemClientWithConfigurationProvider(prov)
	if err != nil {
		return nil, fmt.Errorf("db system client: %w", err)
	}
	return (&oci.MySQLOps{}).GetDbSystem(ctx, &client, dbSystemID)
}

// CreateDbSystem creates a new MySQL DB System.
func (s *MySQLService) CreateDbSystem(ctx context.Context, tenantID int64, compartmentID, displayName, shapeName, adminUsername, adminPassword, subnetID, availabilityDomain, mysqlVersion string) (*mysql.DbSystem, error) {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := buildCreds(tenant)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("provider: %w", err)
	}
	client, err := mysql.NewDbSystemClientWithConfigurationProvider(prov)
	if err != nil {
		return nil, fmt.Errorf("db system client: %w", err)
	}
	return (&oci.MySQLOps{}).CreateDbSystem(ctx, &client, compartmentID, displayName, shapeName, adminUsername, adminPassword, subnetID, availabilityDomain, mysqlVersion)
}

// DeleteDbSystem deletes a MySQL DB System.
func (s *MySQLService) DeleteDbSystem(ctx context.Context, tenantID int64, dbSystemID string) error {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := buildCreds(tenant)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}
	client, err := mysql.NewDbSystemClientWithConfigurationProvider(prov)
	if err != nil {
		return fmt.Errorf("db system client: %w", err)
	}
	return (&oci.MySQLOps{}).DeleteDbSystem(ctx, &client, dbSystemID)
}

// Start starts a stopped MySQL DB System.
func (s *MySQLService) Start(ctx context.Context, tenantID int64, dbSystemID string) error {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := buildCreds(tenant)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}
	client, err := mysql.NewDbSystemClientWithConfigurationProvider(prov)
	if err != nil {
		return fmt.Errorf("db system client: %w", err)
	}
	return (&oci.MySQLOps{}).StartDbSystem(ctx, &client, dbSystemID)
}

// Stop stops a running MySQL DB System.
func (s *MySQLService) Stop(ctx context.Context, tenantID int64, dbSystemID string) error {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := buildCreds(tenant)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}
	client, err := mysql.NewDbSystemClientWithConfigurationProvider(prov)
	if err != nil {
		return fmt.Errorf("db system client: %w", err)
	}
	return (&oci.MySQLOps{}).StopDbSystem(ctx, &client, dbSystemID)
}

// Restart restarts a MySQL DB System.
func (s *MySQLService) Restart(ctx context.Context, tenantID int64, dbSystemID string) error {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := buildCreds(tenant)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}
	client, err := mysql.NewDbSystemClientWithConfigurationProvider(prov)
	if err != nil {
		return fmt.Errorf("db system client: %w", err)
	}
	return (&oci.MySQLOps{}).RestartDbSystem(ctx, &client, dbSystemID)
}

// ---------------------------------------------------------------------------
// Backup Operations
// ---------------------------------------------------------------------------

// ListBackups lists all MySQL backups in a compartment.
func (s *MySQLService) ListBackups(ctx context.Context, tenantID int64, compartmentID, dbSystemID string, limit int, page string) ([]mysql.BackupSummary, *string, error) {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := buildCreds(tenant)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, nil, fmt.Errorf("provider: %w", err)
	}
	client, err := mysql.NewDbBackupsClientWithConfigurationProvider(prov)
	if err != nil {
		return nil, nil, fmt.Errorf("db backups client: %w", err)
	}
	return (&oci.MySQLOps{}).ListBackups(ctx, &client, compartmentID, dbSystemID, limit, page)
}

// CreateBackup creates a manual backup of a MySQL DB System.
func (s *MySQLService) CreateBackup(ctx context.Context, tenantID int64, dbSystemID, displayName, compartmentID string) (*mysql.Backup, error) {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := buildCreds(tenant)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("provider: %w", err)
	}
	client, err := mysql.NewDbBackupsClientWithConfigurationProvider(prov)
	if err != nil {
		return nil, fmt.Errorf("db backups client: %w", err)
	}
	return (&oci.MySQLOps{}).CreateBackup(ctx, &client, dbSystemID, displayName, compartmentID)
}

// DeleteBackup deletes a MySQL backup.
func (s *MySQLService) DeleteBackup(ctx context.Context, tenantID int64, backupID string) error {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := buildCreds(tenant)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}
	client, err := mysql.NewDbBackupsClientWithConfigurationProvider(prov)
	if err != nil {
		return fmt.Errorf("db backups client: %w", err)
	}
	return (&oci.MySQLOps{}).DeleteBackup(ctx, &client, backupID)
}

// ---------------------------------------------------------------------------
// Channel (Replication) Operations
// ---------------------------------------------------------------------------

// ListChannels lists all MySQL channels in a compartment.
func (s *MySQLService) ListChannels(ctx context.Context, tenantID int64, compartmentID, dbSystemID string, limit int, page string) ([]mysql.ChannelSummary, *string, error) {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := buildCreds(tenant)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, nil, fmt.Errorf("provider: %w", err)
	}
	client, err := mysql.NewChannelsClientWithConfigurationProvider(prov)
	if err != nil {
		return nil, nil, fmt.Errorf("channels client: %w", err)
	}
	return (&oci.MySQLOps{}).ListChannels(ctx, &client, compartmentID, dbSystemID, limit, page)
}

// CreateChannel creates a MySQL channel for replication.
func (s *MySQLService) CreateChannel(ctx context.Context, tenantID int64, compartmentID, displayName string, source mysql.CreateChannelSourceDetails, target mysql.CreateChannelTargetDetails) (*mysql.Channel, error) {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := buildCreds(tenant)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("provider: %w", err)
	}
	client, err := mysql.NewChannelsClientWithConfigurationProvider(prov)
	if err != nil {
		return nil, fmt.Errorf("channels client: %w", err)
	}
	return (&oci.MySQLOps{}).CreateChannel(ctx, &client, compartmentID, displayName, source, target)
}

// DeleteChannel deletes a MySQL channel.
func (s *MySQLService) DeleteChannel(ctx context.Context, tenantID int64, channelID string) error {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant %d: %w", tenantID, err)
	}
	creds := buildCreds(tenant)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}
	client, err := mysql.NewChannelsClientWithConfigurationProvider(prov)
	if err != nil {
		return fmt.Errorf("channels client: %w", err)
	}
	return (&oci.MySQLOps{}).DeleteChannel(ctx, &client, channelID)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildCreds converts a repo.Tenant to oci.Credentials.
func buildCreds(tenant repo.Tenant) oci.Credentials {
	return oci.Credentials{
		Tenancy:     nsStr(tenant.Tenancy),
		UserID:      nsStr(tenant.TenantID),
		Fingerprint: nsStr(tenant.Fingerprint),
		Region:      nsStr(tenant.Region),
		KeyFileBlob: nsStr(tenant.KeyFileBlob),
		KeyFile:     nsStr(tenant.KeyFile),
	}
}
