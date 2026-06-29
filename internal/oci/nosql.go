// Package oci -- nosql.go: OCI NoSQL Database SDK operations (Phase 13.3).
// Wraps the OCI NoSQL Database service client for table CRUD, index management,
// and data operations. Parity with Java OciNoSqlUtil.
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/nosql"
)

// NoSQLOps groups all OCI NoSQL Database SDK operations.
type NoSQLOps struct{}

// CreateTable creates a new NoSQL table.
func (o *NoSQLOps) CreateTable(ctx context.Context, client *nosql.NosqlClient, compartmentID, tableName string, ddlStatement string, tableLimits *nosql.TableLimits) (*nosql.Table, error) {
	req := nosql.CreateTableRequest{
		CreateTableDetails: nosql.CreateTableDetails{
			CompartmentId: &compartmentID,
			Name:          &tableName,
			DdlStatement:  &ddlStatement,
			TableLimits:   tableLimits,
		},
	}
	resp, err := client.CreateTable(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create table %s: %w", tableName, err)
	}
	return &resp.Table, nil
}

// GetTable retrieves details of a NoSQL table by name or OCID.
func (o *NoSQLOps) GetTable(ctx context.Context, client *nosql.NosqlClient, tableNameOrID, compartmentID string) (*nosql.Table, error) {
	req := nosql.GetTableRequest{
		TableNameOrId: &tableNameOrID,
		CompartmentId: &compartmentID,
	}
	resp, err := client.GetTable(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get table %s: %w", tableNameOrID, err)
	}
	return &resp.Table, nil
}

// ListTables lists all NoSQL tables in a compartment.
func (o *NoSQLOps) ListTables(ctx context.Context, client *nosql.NosqlClient, compartmentID string, limit int, page string) ([]nosql.TableSummary, *string, error) {
	req := nosql.ListTablesRequest{
		CompartmentId: &compartmentID,
		Limit:         common.Int(limit),
	}
	if page != "" {
		req.Page = &page
	}
	resp, err := client.ListTables(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("list tables: %w", err)
	}
	return resp.Items, resp.OpcNextPage, nil
}

// DeleteTable deletes a NoSQL table by name or OCID.
func (o *NoSQLOps) DeleteTable(ctx context.Context, client *nosql.NosqlClient, tableNameOrID, compartmentID string) error {
	req := nosql.DeleteTableRequest{
		TableNameOrId: &tableNameOrID,
		CompartmentId: &compartmentID,
	}
	_, err := client.DeleteTable(ctx, req)
	if err != nil {
		return fmt.Errorf("delete table %s: %w", tableNameOrID, err)
	}
	return nil
}

// UpdateTable updates a NoSQL table (e.g., table limits).
func (o *NoSQLOps) UpdateTable(ctx context.Context, client *nosql.NosqlClient, tableNameOrID, compartmentID string, tableLimits *nosql.UpdateTableDetails) (*nosql.Table, error) {
	req := nosql.UpdateTableRequest{
		TableNameOrId:      &tableNameOrID,
		CompartmentId:      &compartmentID,
		UpdateTableDetails: *tableLimits,
	}
	resp, err := client.UpdateTable(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("update table %s: %w", tableNameOrID, err)
	}
	return &resp.Table, nil
}

// CreateIndex creates an index on a NoSQL table.
func (o *NoSQLOps) CreateIndex(ctx context.Context, client *nosql.NosqlClient, tableNameOrID, compartmentID, indexName string, keys []nosql.FieldKey) (*nosql.Index, error) {
	req := nosql.CreateIndexRequest{
		TableNameOrId: &tableNameOrID,
		CreateIndexDetails: nosql.CreateIndexDetails{
			CompartmentId: &compartmentID,
			Name:          &indexName,
			Keys:          keys,
		},
	}
	resp, err := client.CreateIndex(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create index %s on %s: %w", indexName, tableNameOrID, err)
	}
	return &resp.Index, nil
}

// ListIndexes lists all indexes on a NoSQL table.
func (o *NoSQLOps) ListIndexes(ctx context.Context, client *nosql.NosqlClient, tableNameOrID, compartmentID string) ([]nosql.IndexSummary, error) {
	req := nosql.ListIndexesRequest{
		TableNameOrId: &tableNameOrID,
		CompartmentId: &compartmentID,
	}
	resp, err := client.ListIndexes(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list indexes on %s: %w", tableNameOrID, err)
	}
	return resp.Items, nil
}

// DeleteIndex deletes an index from a NoSQL table.
func (o *NoSQLOps) DeleteIndex(ctx context.Context, client *nosql.NosqlClient, tableNameOrID, compartmentID, indexName string) error {
	req := nosql.DeleteIndexRequest{
		TableNameOrId: &tableNameOrID,
		IndexName:     &indexName,
		CompartmentId: &compartmentID,
	}
	_, err := client.DeleteIndex(ctx, req)
	if err != nil {
		return fmt.Errorf("delete index %s from %s: %w", indexName, tableNameOrID, err)
	}
	return nil
}

// GetIndex retrieves details of a specific index.
func (o *NoSQLOps) GetIndex(ctx context.Context, client *nosql.NosqlClient, tableNameOrID, compartmentID, indexName string) (*nosql.Index, error) {
	req := nosql.GetIndexRequest{
		TableNameOrId: &tableNameOrID,
		IndexName:     &indexName,
		CompartmentId: &compartmentID,
	}
	resp, err := client.GetIndex(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get index %s from %s: %w", indexName, tableNameOrID, err)
	}
	return &resp.Index, nil
}

// ChangeTableCompartment moves a NoSQL table to a different compartment.
func (o *NoSQLOps) ChangeTableCompartment(ctx context.Context, client *nosql.NosqlClient, tableNameOrID, compartmentID string) error {
	req := nosql.ChangeTableCompartmentRequest{
		TableNameOrId: &tableNameOrID,
		ChangeTableCompartmentDetails: nosql.ChangeTableCompartmentDetails{
			CompartmentId: &compartmentID,
		},
	}
	_, err := client.ChangeTableCompartment(ctx, req)
	if err != nil {
		return fmt.Errorf("change table compartment %s: %w", tableNameOrID, err)
	}
	return nil
}
