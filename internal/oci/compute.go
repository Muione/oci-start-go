// Package oci — compute.go: basic instance listing (Phase 3) + ResetInstance
// (Phase 11.2). Ports OciUtils instance-list calls (ListInstances, GetInstance)
// used by sync. Launch/terminate/resize/start/stop are Phase 4.
package oci

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// ListInstances lists all instances in a compartment, paginating on
// OpcNextPage (limit 100). Parity with the Java ListInstancesRequest loop.
func ListInstances(ctx context.Context, c Clients, compartmentID string) ([]core.Instance, error) {
	var out []core.Instance
	var page *string
	for {
		resp, err := c.Compute.ListInstances(ctx, core.ListInstancesRequest{
			CompartmentId: common.String(compartmentID),
			Limit:         common.Int(100),
			Page:          page,
		})
		if err != nil {
			return nil, fmt.Errorf("list instances in %s: %w", compartmentID, err)
		}
		out = append(out, resp.Items...)
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return out, nil
}

// GetInstance fetches a single instance by OCID.
func GetInstance(ctx context.Context, c Clients, instanceID string) (core.Instance, error) {
	resp, err := c.Compute.GetInstance(ctx, core.GetInstanceRequest{
		InstanceId: common.String(instanceID),
	})
	if err != nil {
		return core.Instance{}, fmt.Errorf("get instance %s: %w", instanceID, err)
	}
	return resp.Instance, nil
}

// TerminateInstance terminates an OCI compute instance and optionally preserves
// the boot volume. Parity with OracleInstanceServiceImpl.killInstance.
func TerminateInstance(ctx context.Context, c Clients, instanceID string, preserveBootVolume bool) error {
	_, err := c.Compute.TerminateInstance(ctx, core.TerminateInstanceRequest{
		InstanceId:         common.String(instanceID),
		PreserveBootVolume: common.Bool(preserveBootVolume),
	})
	if err != nil {
		return fmt.Errorf("terminate instance %s: %w", instanceID, err)
	}
	return nil
}

// instanceArchitecture derives the architecture label ("ARM"/"AMD"/...) from
// the instance's ShapeConfig.ProcessorDescription, parity with the Java logic.
// Returns "NONE" when unavailable (matches the InstanceDetails default).
func instanceArchitecture(shape *core.InstanceShapeConfig) string {
	if shape == nil || shape.ProcessorDescription == nil {
		return "NONE"
	}
	desc := *shape.ProcessorDescription
	low := strings.ToLower(desc)
	switch {
	case strings.Contains(low, "arm") || strings.Contains(low, "a1") || strings.Contains(low, "ampere"):
		return "ARM"
	case strings.Contains(low, "amd"):
		return "AMD"
	case strings.Contains(low, "intel") || strings.Contains(low, "x86"):
		return "AMD"
	default:
		return desc
	}
}

// ResetInstance stops then starts an instance. Required for IPv6 addresses to
// take effect. Parity with Java OciUtils.resetInstance.
func ResetInstance(ctx context.Context, c Clients, instanceID string) error {
	// 1. Stop
	_, err := c.Compute.InstanceAction(ctx, core.InstanceActionRequest{
		InstanceId: common.String(instanceID),
		Action:     core.InstanceActionActionStop,
	})
	if err != nil {
		return fmt.Errorf("reset: stop instance %s: %w", instanceID, err)
	}

	// 2. Poll until STOPPED
	if err := waitForInstanceState(ctx, c, instanceID, core.InstanceLifecycleStateStopped, 300*time.Second, 3*time.Second); err != nil {
		return fmt.Errorf("reset: wait stopped %s: %w", instanceID, err)
	}

	// 3. Start
	_, err = c.Compute.InstanceAction(ctx, core.InstanceActionRequest{
		InstanceId: common.String(instanceID),
		Action:     core.InstanceActionActionStart,
	})
	if err != nil {
		return fmt.Errorf("reset: start instance %s: %w", instanceID, err)
	}

	// 4. Poll until RUNNING
	if err := waitForInstanceState(ctx, c, instanceID, core.InstanceLifecycleStateRunning, 300*time.Second, 3*time.Second); err != nil {
		return fmt.Errorf("reset: wait running %s: %w", instanceID, err)
	}

	return nil
}

// waitForInstanceState polls GetInstance until the instance reaches the target
// lifecycle state or the timeout expires.
func waitForInstanceState(ctx context.Context, c Clients, instanceID string, target core.InstanceLifecycleStateEnum, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		resp, err := c.Compute.GetInstance(ctx, core.GetInstanceRequest{
			InstanceId: common.String(instanceID),
		})
		if err != nil {
			return fmt.Errorf("get instance: %w", err)
		}
		if resp.Instance.LifecycleState == target {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for state %s (current: %s)", target, resp.Instance.LifecycleState)
		}
		time.Sleep(interval)
	}
}
