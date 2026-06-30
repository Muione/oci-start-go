// Package oci — console.go: OCI instance console connection API.
// Creates and manages VNC console connections for instances.
// Uses the ComputeClient's console connection methods for OCI Console proxy
// tunnel access, which works for ALL instances (public and private).
package oci

import (
	"context"
	"fmt"
	"time"

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

// ConsoleConnectionInfo holds full console connection details including keys.
type ConsoleConnectionInfo struct {
	ID                  string
	InstanceID          string
	ConnectionString    string
	VncConnectionString string
	LifecycleState      string
	PrivateKeyPEM       string
	PublicKeySSH        string
}

// CreateConsoleConnection creates a VNC console connection for an instance.
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
		ID:                  derefStr(conn.Id),
		InstanceID:          derefStr(conn.InstanceId),
		VncConnectionString: derefStr(conn.VncConnectionString),
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
		ID:                  derefStr(conn.Id),
		InstanceID:          derefStr(conn.InstanceId),
		VncConnectionString: derefStr(conn.VncConnectionString),
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
			ID:                  derefStr(conn.Id),
			InstanceID:          derefStr(conn.InstanceId),
			VncConnectionString: derefStr(conn.VncConnectionString),
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

// GenerateConsoleConnection creates a new console connection with the given
// SSH public key and returns full connection info including keys.
func GenerateConsoleConnection(ctx context.Context, c Clients, instanceID, publicKey string) (*ConsoleConnectionInfo, error) {
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
	return &ConsoleConnectionInfo{
		ID:                  derefStr(conn.Id),
		InstanceID:          derefStr(conn.InstanceId),
		ConnectionString:    derefStr(conn.ConnectionString),
		VncConnectionString: derefStr(conn.VncConnectionString),
		LifecycleState:      string(conn.LifecycleState),
	}, nil
}

// WaitForConnectionActive polls a console connection until it reaches ACTIVE
// state, or fails/times out. Polls every 3 seconds.
func WaitForConnectionActive(ctx context.Context, c Clients, connID, instanceID string, timeout time.Duration) (*ConsoleConnectionInfo, error) {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for console connection %s to become ACTIVE", connID)
		}

		resp, err := c.Compute.GetInstanceConsoleConnection(ctx, core.GetInstanceConsoleConnectionRequest{
			InstanceConsoleConnectionId: common.String(connID),
		})
		if err != nil {
			return nil, fmt.Errorf("get console connection %s: %w", connID, err)
		}

		conn := resp.InstanceConsoleConnection
		state := string(conn.LifecycleState)

		info := &ConsoleConnectionInfo{
			ID:                  derefStr(conn.Id),
			InstanceID:          derefStr(conn.InstanceId),
			ConnectionString:    derefStr(conn.ConnectionString),
			VncConnectionString: derefStr(conn.VncConnectionString),
			LifecycleState:      state,
		}

		switch state {
		case "ACTIVE":
			return info, nil
		case "FAILED", "DELETED":
			return nil, fmt.Errorf("console connection %s entered terminal state: %s", connID, state)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

// FindActiveConsoleConnection finds the first ACTIVE console connection for an
// instance. Returns nil, nil if no active connection exists.
func FindActiveConsoleConnection(ctx context.Context, c Clients, compartmentID, instanceID string) (*ConsoleConnectionInfo, error) {
	conns, err := ListConsoleConnections(ctx, c, compartmentID, instanceID)
	if err != nil {
		return nil, err
	}

	for _, conn := range conns {
		if conn.LifecycleState == "ACTIVE" {
			// Fetch full details to get connection string.
			full, err := GetConsoleConnection(ctx, c, conn.ID)
			if err != nil {
				continue
			}
			resp, err := c.Compute.GetInstanceConsoleConnection(ctx, core.GetInstanceConsoleConnectionRequest{
				InstanceConsoleConnectionId: common.String(conn.ID),
			})
			if err != nil {
				continue
			}
			detail := resp.InstanceConsoleConnection
			return &ConsoleConnectionInfo{
				ID:                  full.ID,
				InstanceID:          full.InstanceID,
				ConnectionString:    derefStr(detail.ConnectionString),
				VncConnectionString: full.VncConnectionString,
				LifecycleState:      full.LifecycleState,
			}, nil
		}
	}
	return nil, nil
}

// CleanupConsoleConnections deletes all non-DELETED/non-DELETING console
// connections for an instance. Used during session cleanup.
func CleanupConsoleConnections(ctx context.Context, c Clients, compartmentID, instanceID string) error {
	conns, err := ListConsoleConnections(ctx, c, compartmentID, instanceID)
	if err != nil {
		return err
	}

	for _, conn := range conns {
		if conn.LifecycleState == "DELETED" || conn.LifecycleState == "DELETING" {
			continue
		}
		_ = DeleteConsoleConnection(ctx, c, conn.ID)
	}
	return nil
}
