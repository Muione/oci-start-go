// Package oci — console.go: OCI instance console connection API (Phase 8).
// Creates and manages VNC console connections for instances.
// Uses the ComputeClient's console connection methods.
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// ConsoleConnection wraps the OCI console connection details for VNC access.
type ConsoleConnection struct {
	ID                  string `json:"id"`
	InstanceID          string `json:"instanceId"`
	VncConnectionString string `json:"vncConnectionString"`
	LifecycleState      string `json:"lifecycleState"`

}

// CreateConsoleConnection creates a VNC console connection for an instance.
// Parity with Java CreateInstanceConsoleConnectionRequest.
func CreateConsoleConnection(ctx context.Context, c Clients, instanceID string, publicKey string) (*ConsoleConnection, error) {
	req := core.CreateInstanceConsoleConnectionRequest{
		CreateInstanceConsoleConnectionDetails: core.CreateInstanceConsoleConnectionDetails{
			InstanceId: common.String(instanceID),
			PublicKey:  common.String(publicKey),
		},
	}

	resp, err := c.Compute.CreateInstanceConsoleConnection(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create console connection for %s: %w", instanceID, err)
	}

	conn := resp.InstanceConsoleConnection
	return &ConsoleConnection{
		ID:                  stringOrEmpty(conn.Id),
		InstanceID:          stringOrEmpty(conn.InstanceId),
		VncConnectionString: stringOrEmpty(conn.VncConnectionString),
		LifecycleState:      string(conn.LifecycleState),
	}, nil
}

// GetConsoleConnection gets an existing console connection by ID.
func GetConsoleConnection(ctx context.Context, c Clients, connID string) (*ConsoleConnection, error) {
	resp, err := c.Compute.GetInstanceConsoleConnection(ctx, core.GetInstanceConsoleConnectionRequest{
		InstanceConsoleConnectionId: common.String(connID),
	})
	if err != nil {
		return nil, fmt.Errorf("get console connection %s: %w", connID, err)
	}

	conn := resp.InstanceConsoleConnection
	return &ConsoleConnection{
		ID:                  stringOrEmpty(conn.Id),
		InstanceID:          stringOrEmpty(conn.InstanceId),
		VncConnectionString: stringOrEmpty(conn.VncConnectionString),
		LifecycleState:      string(conn.LifecycleState),
	}, nil
}

// ListConsoleConnections lists console connections for an instance.
func ListConsoleConnections(ctx context.Context, c Clients, compartmentID, instanceID string) ([]ConsoleConnection, error) {
	resp, err := c.Compute.ListInstanceConsoleConnections(ctx, core.ListInstanceConsoleConnectionsRequest{
		CompartmentId: common.String(compartmentID),
		InstanceId:    common.String(instanceID),
		Limit:         common.Int(50),
	})
	if err != nil {
		return nil, fmt.Errorf("list console connections for %s: %w", instanceID, err)
	}

	var result []ConsoleConnection
	for _, conn := range resp.Items {
		result = append(result, ConsoleConnection{
			ID:                  stringOrEmpty(conn.Id),
			InstanceID:          stringOrEmpty(conn.InstanceId),
			VncConnectionString: stringOrEmpty(conn.VncConnectionString),
			LifecycleState:      string(conn.LifecycleState),
		})
	}
	return result, nil
}

// DeleteConsoleConnection deletes a console connection.
func DeleteConsoleConnection(ctx context.Context, c Clients, connID string) error {
	_, err := c.Compute.DeleteInstanceConsoleConnection(ctx, core.DeleteInstanceConsoleConnectionRequest{
		InstanceConsoleConnectionId: common.String(connID),
	})
	if err != nil {
		return fmt.Errorf("delete console connection %s: %w", connID, err)
	}
	return nil
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
