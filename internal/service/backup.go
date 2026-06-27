// Package service — backup.go: boot volume backup orchestration (Phase 5).
// Port of OpenSuccessService backup chain + BootVolumeService. Called from
// grabber.success.go after a 3-minute delay.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
)

// BackupSvc orchestrates boot volume backups after successful grabs.
type BackupSvc struct {
	store     *db.Store
	masterKey []byte
	logger    zerolog.Logger
}

func NewBackupSvc(store *db.Store, masterKey []byte, logger zerolog.Logger) *BackupSvc {
	return &BackupSvc{store: store, masterKey: masterKey, logger: logger}
}

// BackupInput carries the data needed to create a backup.
type BackupInput struct {
	TaskID        string
	InstanceID    string
	PublicIP      string
	TenantID      int64
	BootVolumeID  string
	RootPassword  string
	Shape         string
	Architecture  string
	Ocpus         int64
	MemoryInGbs   int64
	DisplayName   string
}

// ScheduleBackup is called by the grab engine 3 minutes after a successful
// launch. It pings the instance, then creates a boot volume backup.
// Stub for SSH portions (Phase 6 fully implements SSH).
func (s *BackupSvc) ScheduleBackup(ctx context.Context, input BackupInput) {
	s.logger.Info().
		Str("instanceId", input.InstanceID).
		Str("publicIp", input.PublicIP).
		Str("taskId", input.TaskID).
		Msg("backup: scheduled (3-min delay elapsed)")

	// TODO (Phase 6): Ping public IP → if unreachable, open security rules.
	// TODO (Phase 6): SSH enableRootLogin.

	// Create the boot volume backup.
	if input.BootVolumeID == "" || input.TenantID <= 0 {
		s.logger.Warn().Str("taskId", input.TaskID).Msg("backup: missing boot volume ID or tenant, skipping")
		return
	}

	if err := s.createBackup(ctx, input); err != nil {
		s.logger.Error().Err(err).Str("taskId", input.TaskID).Msg("backup: failed")
	}
}

// createBackup creates a boot volume backup via the OCI API and stores the
// record in instance_backup_detail.
func (s *BackupSvc) createBackup(ctx context.Context, input BackupInput) error {
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, input.TenantID)
	if err != nil {
		return fmt.Errorf("find tenant: %w", err)
	}

	creds := tenantToCreds(tenant)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}

	clients, err := oci.NewClients(prov)
	if err != nil {
		return fmt.Errorf("clients: %w", err)
	}

	displayName := fmt.Sprintf("oci-start-backup-%s-%s", input.TaskID, time.Now().Format("20060102-150405"))
	backupID, err := oci.CreateBootVolumeBackup(ctx, clients.Blockstorage, input.BootVolumeID, displayName)
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}

	s.logger.Info().
		Str("backupId", backupID).
		Str("instanceId", input.InstanceID).
		Msg("backup: created successfully")

	// Store backup record.
	params := repo.InsertInstanceBackupDetailParams{
		TenantID:             sql.NullInt64{Int64: input.TenantID, Valid: true},
		InstanceID:           sql.NullString{String: input.InstanceID, Valid: true},
		DisplayName:          sql.NullString{String: input.DisplayName, Valid: true},
		Shape:                sql.NullString{String: input.Shape, Valid: true},
		BootVolumeSizeInGbs:  sql.NullInt64{Int64: input.MemoryInGbs, Valid: true},
		PublicIps:            sql.NullString{String: input.PublicIP, Valid: true},
		BootVolumeName:       sql.NullString{String: displayName, Valid: true},
		Architecture:         sql.NullString{String: input.Architecture, Valid: true},
		BootVolumeID:         sql.NullString{String: input.BootVolumeID, Valid: true},
	}
	if err := repo.New(s.store.Write).InsertInstanceBackupDetail(ctx, params); err != nil {
		s.logger.Error().Err(err).Msg("backup: store record failed")
	}

	return nil
}

// ManualBackup triggers a backup manually via the API. Looks up the instance
// by OCI instance ID (OCID string), resolves the boot volume, and creates a
// backup via OCI.
func (s *BackupSvc) ManualBackup(ctx context.Context, tenantID int64, instanceID string) error {
	// Look up the instance detail by OCI instance_id string (not primary key).
	var bootVolumeID, displayName, shape, architecture sql.NullString
	err := s.store.Read.QueryRowContext(ctx,
		`SELECT boot_volume_id, display_name, shape, architecture
		 FROM instance_detail WHERE instance_id = ? LIMIT 1`,
		instanceID).Scan(&bootVolumeID, &displayName, &shape, &architecture)
	if err != nil {
		return fmt.Errorf("find instance %s: %w", instanceID, err)
	}

	if !bootVolumeID.Valid || bootVolumeID.String == "" {
		return fmt.Errorf("instance %s has no boot volume ID", instanceID)
	}

	input := BackupInput{
		InstanceID:   instanceID,
		TenantID:     tenantID,
		BootVolumeID: bootVolumeID.String,
		DisplayName:  displayName.String,
		Shape:        shape.String,
		Architecture: architecture.String,
	}

	return s.createBackup(ctx, input)
}
