// Package oci — instance_control.go: OCI instance lifecycle operations
// (start/stop/attach/detach). Used by rescue flow and general instance
// management. Parity with Java OciInstanceControlService.
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// StopInstance stops a running OCI instance.
func StopInstance(ctx context.Context, c Clients, instanceID string) error {
	_, err := c.Compute.InstanceAction(ctx, core.InstanceActionRequest{
		InstanceId: common.String(instanceID),
		Action:     core.InstanceActionActionStop,
	})
	if err != nil {
		return fmt.Errorf("stop instance %s: %w", instanceID, err)
	}
	return nil
}

// StartInstance starts a stopped OCI instance.
func StartInstance(ctx context.Context, c Clients, instanceID string) error {
	_, err := c.Compute.InstanceAction(ctx, core.InstanceActionRequest{
		InstanceId: common.String(instanceID),
		Action:     core.InstanceActionActionStart,
	})
	if err != nil {
		return fmt.Errorf("start instance %s: %w", instanceID, err)
	}
	return nil
}

// GetInstanceFull fetches full instance details.
func GetInstanceFull(ctx context.Context, c Clients, instanceID string) (*core.Instance, error) {
	resp, err := c.Compute.GetInstance(ctx, core.GetInstanceRequest{
		InstanceId: common.String(instanceID),
	})
	if err != nil {
		return nil, fmt.Errorf("get instance %s: %w", instanceID, err)
	}
	return &resp.Instance, nil
}

// ListBootVolumeAttachments lists all boot volume attachments for an instance.
func ListBootVolumeAttachments(ctx context.Context, c Clients, compartmentID, instanceID, ad string) ([]core.BootVolumeAttachment, error) {
	var out []core.BootVolumeAttachment
	var page *string
	for {
		resp, err := c.Compute.ListBootVolumeAttachments(ctx, core.ListBootVolumeAttachmentsRequest{
			CompartmentId:      common.String(compartmentID),
			InstanceId:         common.String(instanceID),
			AvailabilityDomain: common.String(ad),
			Limit:              common.Int(100),
			Page:               page,
		})
		if err != nil {
			return nil, fmt.Errorf("list boot volume attachments for %s: %w", instanceID, err)
		}
		out = append(out, resp.Items...)
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return out, nil
}

// DetachBootVolume detaches a boot volume attachment by ID.
func DetachBootVolume(ctx context.Context, c Clients, attachmentID string) error {
	_, err := c.Compute.DetachBootVolume(ctx, core.DetachBootVolumeRequest{
		BootVolumeAttachmentId: common.String(attachmentID),
	})
	if err != nil {
		return fmt.Errorf("detach boot volume %s: %w", attachmentID, err)
	}
	return nil
}

// AttachBootVolume attaches a boot volume to an instance.
func AttachBootVolume(ctx context.Context, c Clients, instanceID, bootVolumeID string) (*core.BootVolumeAttachment, error) {
	resp, err := c.Compute.AttachBootVolume(ctx, core.AttachBootVolumeRequest{
		AttachBootVolumeDetails: core.AttachBootVolumeDetails{
			InstanceId:   common.String(instanceID),
			BootVolumeId: common.String(bootVolumeID),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("attach boot volume %s to %s: %w", bootVolumeID, instanceID, err)
	}
	return &resp.BootVolumeAttachment, nil
}

// GetInstanceState is a convenience wrapper that returns the lifecycle state string.
func GetInstanceState(ctx context.Context, c Clients, instanceID string) (string, error) {
	inst, err := GetInstanceFull(ctx, c, instanceID)
	if err != nil {
		return "", err
	}
	return string(inst.LifecycleState), nil
}
