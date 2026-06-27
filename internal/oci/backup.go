// Package oci — backup.go: boot volume backup operations (SPEC S8.6, S10.3).
// Port of BootVolumeServiceImpl.java. Uses BlockstorageClient for
// CreateBootVolumeBackup, ListBootVolumeBackups, Copy/Delete.
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// CreateBootVolumeBackup creates a backup of the given boot volume.
// Returns the backup OCID. Parity with BootVolumeServiceImpl.createBootVolumeBackup.
func CreateBootVolumeBackup(ctx context.Context, client *core.BlockstorageClient, bootVolumeID, displayName string) (string, error) {
	req := core.CreateBootVolumeBackupRequest{
		CreateBootVolumeBackupDetails: core.CreateBootVolumeBackupDetails{
			BootVolumeId: common.String(bootVolumeID),
			DisplayName:  common.String(displayName),
		},
	}
	resp, err := client.CreateBootVolumeBackup(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create boot volume backup: %w", err)
	}
	if resp.BootVolumeBackup.Id == nil {
		return "", fmt.Errorf("backup returned nil id")
	}
	return *resp.BootVolumeBackup.Id, nil
}

// ListBootVolumeBackups returns all backups for a given boot volume in a
// compartment. Parity with BootVolumeServiceImpl.listBootVolumeBackups.
func ListBootVolumeBackups(ctx context.Context, client *core.BlockstorageClient, compartmentID, bootVolumeID string) ([]core.BootVolumeBackup, error) {
	var out []core.BootVolumeBackup
	var page *string
	for {
		resp, err := client.ListBootVolumeBackups(ctx, core.ListBootVolumeBackupsRequest{
			CompartmentId: common.String(compartmentID),
			BootVolumeId:  common.String(bootVolumeID),
			Limit:         common.Int(100),
			Page:          page,
		})
		if err != nil {
			return nil, fmt.Errorf("list boot volume backups: %w", err)
		}
		out = append(out, resp.Items...)
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return out, nil
}

// CopyBootVolumeBackup copies a backup to a different region.
// Parity with BootVolumeServiceImpl.copyBootVolumeBackup.
func CopyBootVolumeBackup(ctx context.Context, client *core.BlockstorageClient, backupID, targetRegion, displayName string) (string, error) {
	req := core.CopyBootVolumeBackupRequest{
		BootVolumeBackupId: common.String(backupID),
		CopyBootVolumeBackupDetails: core.CopyBootVolumeBackupDetails{
			DestinationRegion: common.String(targetRegion),
			DisplayName:       common.String(displayName),
		},
	}
	resp, err := client.CopyBootVolumeBackup(ctx, req)
	if err != nil {
		return "", fmt.Errorf("copy boot volume backup: %w", err)
	}
	if resp.BootVolumeBackup.Id == nil {
		return "", fmt.Errorf("copied backup returned nil id")
	}
	return *resp.BootVolumeBackup.Id, nil
}

// DeleteBootVolumeBackup deletes a backup by OCID.
func DeleteBootVolumeBackup(ctx context.Context, client *core.BlockstorageClient, backupID string) error {
	_, err := client.DeleteBootVolumeBackup(ctx, core.DeleteBootVolumeBackupRequest{
		BootVolumeBackupId: common.String(backupID),
	})
	if err != nil {
		return fmt.Errorf("delete boot volume backup: %w", err)
	}
	return nil
}

// GetBootVolume returns a boot volume by OCID.
func GetBootVolume(ctx context.Context, client *core.BlockstorageClient, bootVolumeID string) (*core.BootVolume, error) {
	resp, err := client.GetBootVolume(ctx, core.GetBootVolumeRequest{
		BootVolumeId: common.String(bootVolumeID),
	})
	if err != nil {
		return nil, fmt.Errorf("get boot volume: %w", err)
	}
	return &resp.BootVolume, nil
}
