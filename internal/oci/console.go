// Package oci — console.go: OCI instance console connection API.
// Creates and manages VNC console connections for instances.
// Uses the ComputeClient's console connection methods for OCI Console proxy
// tunnel access, which works for ALL instances (public and private).
package oci

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// consoleOps abstracts the four console-connection RPCs so the clear/wait/
// ensure/recover logic can be unit-tested without a live ComputeClient
// (Clients.Compute is a concrete *core.ComputeClient, not an interface).
// Field order matches positional consoleOps{list, create, del, get} in tests.
type consoleOps struct {
	list   func(context.Context) ([]ConsoleConnection, error)
	create func(context.Context) (*ConsoleConnectionInfo, error)
	del    func(context.Context, string) error
	get    func(context.Context, string) (*ConsoleConnectionInfo, error)
}

// isConsoleConflict reports whether err is the OCI 409 IncorrectState returned
// by CreateInstanceConsoleConnection when a connection already exists or has
// not finished terminating. Mirrors isNotFound's detection (vnic.go) but uses
// errors.As to see through the %w wrap applied by GenerateConsoleConnection,
// with a string fallback for non-SDK errors carrying the same markers.
func isConsoleConflict(err error) bool {
	if err == nil {
		return false
	}
	type ociError interface {
		GetHTTPStatusCode() int
		GetCode() string
	}
	var e ociError
	if errors.As(err, &e) {
		return e.GetHTTPStatusCode() == 409 && e.GetCode() == "IncorrectState"
	}
	msg := err.Error()
	return strings.Contains(msg, "409") && strings.Contains(msg, "IncorrectState")
}

// waitForCleared polls list until every console connection for the instance is
// in a terminal state (DELETED) or none remain, or timeout/ctx expires.
// Required because DeleteInstanceConsoleConnection is asynchronous: a
// connection still DELETING makes the next Create return 409.
func waitForCleared(ctx context.Context, list func(context.Context) ([]ConsoleConnection, error), poll, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		conns, err := list(ctx)
		if err != nil {
			return fmt.Errorf("list during clear-wait: %w", err)
		}
		cleared := true
		for _, c := range conns {
			if c.LifecycleState != "DELETED" {
				cleared = false
				break
			}
		}
		if cleared {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for console connections to clear (still non-DELETED)")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
	}
}

// clearAll deletes every non-terminal console connection for an instance
// (ACTIVE/CREATING/REQUESTED/FAILED). DELETING/DELETED are left to finish on
// their own. Best-effort: all are attempted; the first error is returned.
func clearAll(ctx context.Context, list func(context.Context) ([]ConsoleConnection, error), del func(context.Context, string) error) error {
	conns, err := list(ctx)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	var firstErr error
	for _, c := range conns {
		if c.LifecycleState == "DELETED" || c.LifecycleState == "DELETING" {
			continue
		}
		if derr := del(ctx, c.ID); derr != nil && firstErr == nil {
			firstErr = derr
		}
	}
	return firstErr
}

// waitForActive polls get until the connection is ACTIVE (returns info) or
// enters a terminal state FAILED/DELETED (error), or times out.
func waitForActive(ctx context.Context, get func(context.Context, string) (*ConsoleConnectionInfo, error), connID string, poll, timeout time.Duration) (*ConsoleConnectionInfo, error) {
	deadline := time.Now().Add(timeout)
	for {
		info, err := get(ctx, connID)
		if err != nil {
			return nil, fmt.Errorf("get console connection %s: %w", connID, err)
		}
		switch info.LifecycleState {
		case "ACTIVE":
			return info, nil
		case "FAILED", "DELETED":
			return nil, fmt.Errorf("console connection %s entered terminal state: %s", connID, info.LifecycleState)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for console connection %s to become ACTIVE", connID)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(poll):
		}
	}
}

// ensureConsoleConnection creates a fresh console connection, first clearing
// any existing connection and waiting for the async delete to settle — the
// fix for the 409 IncorrectState ("already exists or has not been terminated")
// that occurs when a previous connection lingers. On a 409 from create (race
// or external leftover) it clears + waits + retries once, then gives up.
func ensureConsoleConnection(ctx context.Context, ops consoleOps, poll, timeout time.Duration) (*ConsoleConnectionInfo, error) {
	// 1. Clear leftover, then wait for async deletes to settle before create.
	_ = clearAll(ctx, ops.list, ops.del) // best-effort; waitForCleared surfaces real failures
	if err := waitForCleared(ctx, ops.list, poll, timeout); err != nil {
		return nil, fmt.Errorf("clear existing console connection: %w", err)
	}

	info, err := ops.create(ctx)
	if err == nil {
		return waitActiveOrReturn(ctx, ops, info, poll, timeout)
	}
	if !isConsoleConflict(err) {
		return nil, fmt.Errorf("create console connection: %w", err)
	}

	// 2. 409 race / external leftover not visible to the first clear: retry once.
	_ = clearAll(ctx, ops.list, ops.del)
	if cerr := waitForCleared(ctx, ops.list, poll, timeout); cerr != nil {
		return nil, fmt.Errorf("clear after 409: %w (original create: %v)", cerr, err)
	}
	info, err = ops.create(ctx)
	if err != nil {
		return nil, fmt.Errorf("create console connection (retry after 409): %w", err)
	}
	return waitActiveOrReturn(ctx, ops, info, poll, timeout)
}

// waitActiveOrReturn returns info directly if it is already ACTIVE; otherwise
// polls ops.get until ACTIVE/terminal/timeout.
func waitActiveOrReturn(ctx context.Context, ops consoleOps, info *ConsoleConnectionInfo, poll, timeout time.Duration) (*ConsoleConnectionInfo, error) {
	if info.LifecycleState == "ACTIVE" {
		return info, nil
	}
	return waitForActive(ctx, ops.get, info.ID, poll, timeout)
}

// EnsureConsoleConnection creates a fresh, ACTIVE console connection for an
// instance, transparently clearing any lingering connection first and
// recovering from a 409 IncorrectState by retrying once. It replaces the
// broken create-immediately-after-delete flow that surfaced Oracle's 409 to
// users. timeout governs both the clear-wait and the wait-for-ACTIVE polls.
func EnsureConsoleConnection(ctx context.Context, c Clients, compartmentID, instanceID, publicKey string, timeout time.Duration) (*ConsoleConnectionInfo, error) {
	ops := consoleOps{
		list: func(ctx context.Context) ([]ConsoleConnection, error) {
			return ListConsoleConnections(ctx, c, compartmentID, instanceID)
		},
		create: func(ctx context.Context) (*ConsoleConnectionInfo, error) {
			return GenerateConsoleConnection(ctx, c, instanceID, publicKey)
		},
		del: func(ctx context.Context, connID string) error {
			return DeleteConsoleConnection(ctx, c, connID)
		},
		get: func(ctx context.Context, connID string) (*ConsoleConnectionInfo, error) {
			return GetConsoleConnectionInfo(ctx, c, connID)
		},
	}
	const poll = 3 * time.Second
	return ensureConsoleConnection(ctx, ops, poll, timeout)
}

// WaitForConnectionsCleared polls until no non-terminal console connection
// remains for the instance, or timeout. Public wrapper around waitForCleared
// for callers that only need the wait (e.g. shutdown/cleanup paths).
func WaitForConnectionsCleared(ctx context.Context, c Clients, compartmentID, instanceID string, timeout time.Duration) error {
	return waitForCleared(ctx, func(ctx context.Context) ([]ConsoleConnection, error) {
		return ListConsoleConnections(ctx, c, compartmentID, instanceID)
	}, 3*time.Second, timeout)
}

// GetConsoleConnectionInfo fetches full connection details (including the
// connection string) by ID — the Get-based counterpart to
// GenerateConsoleConnection, used by EnsureConsoleConnection's wait-for-ACTIVE.
func GetConsoleConnectionInfo(ctx context.Context, c Clients, connID string) (*ConsoleConnectionInfo, error) {
	resp, err := c.Compute.GetInstanceConsoleConnection(ctx, core.GetInstanceConsoleConnectionRequest{
		InstanceConsoleConnectionId: common.String(connID),
	})
	if err != nil {
		return nil, fmt.Errorf("get console connection %s: %w", connID, err)
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
