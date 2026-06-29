// Package oci -- mysql.go: OCI MySQL Database Service SDK operations (Phase 13.3).
// Wraps the OCI MySQL HeatWave service client for DB system CRUD, backup
// management, and channel (replication) operations. Parity with Java OciMysqlUtil.
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/mysql"
)

// MySQLOps groups all OCI MySQL Database Service SDK operations.
type MySQLOps struct{}

// --- DB System Operations ---

// CreateDbSystem creates a new MySQL DB System.
func (m *MySQLOps) CreateDbSystem(ctx context.Context, client *mysql.DbSystemClient, compartmentID, displayName, shapeName, adminUsername, adminPassword string, subnetID string, availabilityDomain string, mysqlVersion string) (*mysql.DbSystem, error) {
	req := mysql.CreateDbSystemRequest{
		CreateDbSystemDetails: mysql.CreateDbSystemDetails{
			CompartmentId:      &compartmentID,
			DisplayName:        &displayName,
			ShapeName:          &shapeName,
			SubnetId:           &subnetID,
			AvailabilityDomain: &availabilityDomain,
			AdminUsername:      &adminUsername,
			AdminPassword:      &adminPassword,
			MysqlVersion:       &mysqlVersion,
		},
	}
	resp, err := client.CreateDbSystem(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create db system: %w", err)
	}
	return &resp.DbSystem, nil
}

// GetDbSystem retrieves details of a MySQL DB System.
func (m *MySQLOps) GetDbSystem(ctx context.Context, client *mysql.DbSystemClient, dbSystemID string) (*mysql.DbSystem, error) {
	req := mysql.GetDbSystemRequest{
		DbSystemId: &dbSystemID,
	}
	resp, err := client.GetDbSystem(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get db system %s: %w", dbSystemID, err)
	}
	return &resp.DbSystem, nil
}

// ListDbSystems lists all MySQL DB Systems in a compartment.
func (m *MySQLOps) ListDbSystems(ctx context.Context, client *mysql.DbSystemClient, compartmentID string, displayName string, limit int, page string) ([]mysql.DbSystemSummary, *string, error) {
	req := mysql.ListDbSystemsRequest{
		CompartmentId: &compartmentID,
		Limit:         common.Int(limit),
	}
	if displayName != "" {
		req.DisplayName = &displayName
	}
	if page != "" {
		req.Page = &page
	}
	resp, err := client.ListDbSystems(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("list db systems: %w", err)
	}
	return resp.Items, resp.OpcNextPage, nil
}

// UpdateDbSystem updates a MySQL DB System display name.
// Returns a work request ID since the operation is asynchronous.
func (m *MySQLOps) UpdateDbSystem(ctx context.Context, client *mysql.DbSystemClient, dbSystemID string, displayName string) (*string, error) {
	req := mysql.UpdateDbSystemRequest{
		DbSystemId: &dbSystemID,
		UpdateDbSystemDetails: mysql.UpdateDbSystemDetails{
			DisplayName: &displayName,
		},
	}
	resp, err := client.UpdateDbSystem(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("update db system %s: %w", dbSystemID, err)
	}
	return resp.OpcWorkRequestId, nil
}

// DeleteDbSystem deletes a MySQL DB System.
func (m *MySQLOps) DeleteDbSystem(ctx context.Context, client *mysql.DbSystemClient, dbSystemID string) error {
	req := mysql.DeleteDbSystemRequest{
		DbSystemId: &dbSystemID,
	}
	_, err := client.DeleteDbSystem(ctx, req)
	if err != nil {
		return fmt.Errorf("delete db system %s: %w", dbSystemID, err)
	}
	return nil
}

// StartDbSystem starts a stopped MySQL DB System.
func (m *MySQLOps) StartDbSystem(ctx context.Context, client *mysql.DbSystemClient, dbSystemID string) error {
	req := mysql.StartDbSystemRequest{
		DbSystemId: &dbSystemID,
	}
	_, err := client.StartDbSystem(ctx, req)
	if err != nil {
		return fmt.Errorf("start db system %s: %w", dbSystemID, err)
	}
	return nil
}

// StopDbSystem stops a running MySQL DB System.
func (m *MySQLOps) StopDbSystem(ctx context.Context, client *mysql.DbSystemClient, dbSystemID string) error {
	req := mysql.StopDbSystemRequest{
		DbSystemId: &dbSystemID,
	}
	_, err := client.StopDbSystem(ctx, req)
	if err != nil {
		return fmt.Errorf("stop db system %s: %w", dbSystemID, err)
	}
	return nil
}

// RestartDbSystem restarts a MySQL DB System.
func (m *MySQLOps) RestartDbSystem(ctx context.Context, client *mysql.DbSystemClient, dbSystemID string) error {
	req := mysql.RestartDbSystemRequest{
		DbSystemId: &dbSystemID,
	}
	_, err := client.RestartDbSystem(ctx, req)
	if err != nil {
		return fmt.Errorf("restart db system %s: %w", dbSystemID, err)
	}
	return nil
}

// --- Backup Operations (use DbBackupsClient) ---

// CreateBackup creates a manual backup of a MySQL DB System.
func (m *MySQLOps) CreateBackup(ctx context.Context, client *mysql.DbBackupsClient, dbSystemID, displayName, compartmentID string) (*mysql.Backup, error) {
	req := mysql.CreateBackupRequest{
		CreateBackupDetails: mysql.CreateBackupDetails{
			DisplayName: &displayName,
			DbSystemId:  &dbSystemID,
		},
	}
	resp, err := client.CreateBackup(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create backup: %w", err)
	}
	return &resp.Backup, nil
}

// GetBackup retrieves details of a MySQL backup.
func (m *MySQLOps) GetBackup(ctx context.Context, client *mysql.DbBackupsClient, backupID string) (*mysql.Backup, error) {
	req := mysql.GetBackupRequest{
		BackupId: &backupID,
	}
	resp, err := client.GetBackup(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get backup %s: %w", backupID, err)
	}
	return &resp.Backup, nil
}

// ListBackups lists all MySQL backups in a compartment.
func (m *MySQLOps) ListBackups(ctx context.Context, client *mysql.DbBackupsClient, compartmentID string, dbSystemID string, limit int, page string) ([]mysql.BackupSummary, *string, error) {
	req := mysql.ListBackupsRequest{
		CompartmentId: &compartmentID,
		Limit:         common.Int(limit),
	}
	if dbSystemID != "" {
		req.DbSystemId = &dbSystemID
	}
	if page != "" {
		req.Page = &page
	}
	resp, err := client.ListBackups(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("list backups: %w", err)
	}
	return resp.Items, resp.OpcNextPage, nil
}

// DeleteBackup deletes a MySQL backup.
func (m *MySQLOps) DeleteBackup(ctx context.Context, client *mysql.DbBackupsClient, backupID string) error {
	req := mysql.DeleteBackupRequest{
		BackupId: &backupID,
	}
	_, err := client.DeleteBackup(ctx, req)
	if err != nil {
		return fmt.Errorf("delete backup %s: %w", backupID, err)
	}
	return nil
}

// --- Channel (Replication) Operations (use ChannelsClient) ---

// CreateChannel creates a MySQL channel for replication.
func (m *MySQLOps) CreateChannel(ctx context.Context, client *mysql.ChannelsClient, compartmentID, displayName string, source mysql.CreateChannelSourceDetails, target mysql.CreateChannelTargetDetails) (*mysql.Channel, error) {
	req := mysql.CreateChannelRequest{
		CreateChannelDetails: mysql.CreateChannelDetails{
			CompartmentId: &compartmentID,
			DisplayName:   &displayName,
			Source:        source,
			Target:        target,
		},
	}
	resp, err := client.CreateChannel(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}
	return &resp.Channel, nil
}

// GetChannel retrieves details of a MySQL channel.
func (m *MySQLOps) GetChannel(ctx context.Context, client *mysql.ChannelsClient, channelID string) (*mysql.Channel, error) {
	req := mysql.GetChannelRequest{
		ChannelId: &channelID,
	}
	resp, err := client.GetChannel(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get channel %s: %w", channelID, err)
	}
	return &resp.Channel, nil
}

// ListChannels lists all MySQL channels in a compartment.
func (m *MySQLOps) ListChannels(ctx context.Context, client *mysql.ChannelsClient, compartmentID, dbSystemID string, limit int, page string) ([]mysql.ChannelSummary, *string, error) {
	req := mysql.ListChannelsRequest{
		CompartmentId: &compartmentID,
		Limit:         common.Int(limit),
	}
	if dbSystemID != "" {
		req.DbSystemId = &dbSystemID
	}
	if page != "" {
		req.Page = &page
	}
	resp, err := client.ListChannels(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("list channels: %w", err)
	}
	return resp.Items, resp.OpcNextPage, nil
}

// DeleteChannel deletes a MySQL channel.
func (m *MySQLOps) DeleteChannel(ctx context.Context, client *mysql.ChannelsClient, channelID string) error {
	req := mysql.DeleteChannelRequest{
		ChannelId: &channelID,
	}
	_, err := client.DeleteChannel(ctx, req)
	if err != nil {
		return fmt.Errorf("delete channel %s: %w", channelID, err)
	}
	return nil
}
