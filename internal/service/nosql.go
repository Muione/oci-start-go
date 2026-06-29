// Package service -- nosql.go: Phase 13.3 NoSQL Database service.
// Orchestrates OCI NoSQL table/index/row operations via direct provider creation.
package service

import (
	"context"
	"fmt"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/nosql"
)

// NoSQLService manages OCI NoSQL Database operations per tenant.
type NoSQLService struct {
	store     *db.Store
	masterKey []byte
}

// NewNoSQLService constructs a NoSQLService.
func NewNoSQLService(store *db.Store, masterKey []byte) *NoSQLService {
	return &NoSQLService{store: store, masterKey: masterKey}
}

// ListTables lists all NoSQL tables in a compartment.
func (s *NoSQLService) ListTables(ctx context.Context, tenantID int64, compartmentID string, limit int, page string) ([]nosql.TableSummary, *string, error) {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	ops := &oci.NoSQLOps{}
	return ops.ListTables(ctx, client, compartmentID, limit, page)
}

// GetTable retrieves details of a NoSQL table.
func (s *NoSQLService) GetTable(ctx context.Context, tenantID int64, tableNameOrID, compartmentID string) (*nosql.Table, error) {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	ops := &oci.NoSQLOps{}
	return ops.GetTable(ctx, client, tableNameOrID, compartmentID)
}

// CreateTable creates a new NoSQL table (async — returns work request ID).
func (s *NoSQLService) CreateTable(ctx context.Context, tenantID int64, compartmentID, tableName, ddlStatement string, tableLimits *nosql.TableLimits) (*string, error) {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	ops := &oci.NoSQLOps{}
	return ops.CreateTable(ctx, client, compartmentID, tableName, ddlStatement, tableLimits)
}

// DeleteTable deletes a NoSQL table.
func (s *NoSQLService) DeleteTable(ctx context.Context, tenantID int64, tableNameOrID, compartmentID string) error {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return err
	}
	ops := &oci.NoSQLOps{}
	return ops.DeleteTable(ctx, client, tableNameOrID, compartmentID)
}

// GetRow gets a single row from a NoSQL table by primary key.
func (s *NoSQLService) GetRow(ctx context.Context, tenantID int64, tableNameOrID, compartmentID string, key []string) (*nosql.Row, error) {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	req := nosql.GetRowRequest{
		TableNameOrId: &tableNameOrID,
		CompartmentId: &compartmentID,
		Key:           key,
	}
	resp, err := client.GetRow(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get row from %s: %w", tableNameOrID, err)
	}
	return &resp.Row, nil
}

// UpdateRow puts/updates a single row in a NoSQL table.
func (s *NoSQLService) UpdateRow(ctx context.Context, tenantID int64, tableNameOrID, compartmentID string, value map[string]interface{}) error {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return err
	}
	req := nosql.UpdateRowRequest{
		TableNameOrId: &tableNameOrID,
		UpdateRowDetails: nosql.UpdateRowDetails{
			Value:         value,
			CompartmentId: &compartmentID,
		},
	}
	_, err = client.UpdateRow(ctx, req)
	if err != nil {
		return fmt.Errorf("update row in %s: %w", tableNameOrID, err)
	}
	return nil
}

// DeleteRow deletes a single row from a NoSQL table by primary key.
func (s *NoSQLService) DeleteRow(ctx context.Context, tenantID int64, tableNameOrID, compartmentID string, key []string) error {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return err
	}
	req := nosql.DeleteRowRequest{
		TableNameOrId: &tableNameOrID,
		CompartmentId: &compartmentID,
		Key:           key,
	}
	_, err = client.DeleteRow(ctx, req)
	if err != nil {
		return fmt.Errorf("delete row from %s: %w", tableNameOrID, err)
	}
	return nil
}

// QueryRows executes a SQL query against a NoSQL table.
// Returns a slice of maps, each representing a row with column-name keys.
func (s *NoSQLService) QueryRows(ctx context.Context, tenantID int64, compartmentID, statement string, limit int, page string) ([]map[string]interface{}, *string, error) {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	req := nosql.QueryRequest{
		QueryDetails: nosql.QueryDetails{
			CompartmentId: &compartmentID,
			Statement:     &statement,
		},
		Limit: common.Int(limit),
	}
	if page != "" {
		req.Page = &page
	}
	resp, err := client.Query(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("query nosql: %w", err)
	}
	return resp.Items, resp.OpcNextPage, nil
}

// newClient creates a NosqlClient from tenant credentials.
func (s *NoSQLService) newClient(ctx context.Context, tenantID int64) (*nosql.NosqlClient, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant %d not found: %w", tenantID, err)
	}
	creds := oci.Credentials{
		Tenancy:     nsStr(t.Tenancy),
		UserID:      nsStr(t.TenantID),
		Fingerprint: nsStr(t.Fingerprint),
		Region:      nsStr(t.Region),
		KeyFileBlob: nsStr(t.KeyFileBlob),
		KeyFile:     nsStr(t.KeyFile),
	}
	provider, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("oci provider: %w", err)
	}
	client, err := nosql.NewNosqlClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("nosql client: %w", err)
	}
	return &client, nil
}
