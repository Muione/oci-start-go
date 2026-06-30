// Package oci — block_volume.go: Block Volume VPU and size management.
// Provides UpdateBootVolumeVpu and UpdateVolumeVpu for adjusting disk
// performance (VPUsPerGB) on boot volumes and block volumes.
// Parity with Java OciUtils.updateBootVolumeVpu / updateVolumeVpu.
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// BootVolumeInfo holds simplified boot volume details for API responses.
type BootVolumeInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	SizeInGBs   int64  `json:"sizeInGBs"`
	VpusPerGB   int64  `json:"vpusPerGB"`
}

// UpdateBootVolumeVpu updates the VPUsPerGB (performance level) of a boot volume.
// Allowed VPU values:
//   - 10: Balanced
//   - 20: Higher Performance
//   - 30-120: Ultra High Performance
//
// The boot volume must be detached (instance stopped) for this to work.
func UpdateBootVolumeVpu(ctx context.Context, c Clients, bootVolumeID string, vpusPerGB int64) (*core.BootVolume, error) {
	resp, err := c.Blockstorage.UpdateBootVolume(ctx, core.UpdateBootVolumeRequest{
		BootVolumeId: common.String(bootVolumeID),
		UpdateBootVolumeDetails: core.UpdateBootVolumeDetails{
			VpusPerGB: common.Int64(vpusPerGB),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("update boot volume %s VPU to %d: %w", bootVolumeID, vpusPerGB, err)
	}
	return &resp.BootVolume, nil
}

// UpdateVolumeVpu updates the VPUsPerGB (performance level) of a block volume.
// Allowed VPU values:
//   - 0: Lower Cost (Sparse volumes only)
//   - 10: Balanced
//   - 20: Higher Performance
//   - 30-120: Ultra High Performance
func UpdateVolumeVpu(ctx context.Context, c Clients, volumeID string, vpusPerGB int64) (*core.Volume, error) {
	resp, err := c.Blockstorage.UpdateVolume(ctx, core.UpdateVolumeRequest{
		VolumeId: common.String(volumeID),
		UpdateVolumeDetails: core.UpdateVolumeDetails{
			VpusPerGB: common.Int64(vpusPerGB),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("update volume %s VPU to %d: %w", volumeID, vpusPerGB, err)
	}
	return &resp.Volume, nil
}

// GetBootVolumeClients fetches a boot volume by OCID using the Clients wrapper.
func GetBootVolumeClients(ctx context.Context, c Clients, bootVolumeID string) (*core.BootVolume, error) {
	resp, err := c.Blockstorage.GetBootVolume(ctx, core.GetBootVolumeRequest{
		BootVolumeId: common.String(bootVolumeID),
	})
	if err != nil {
		return nil, fmt.Errorf("get boot volume %s: %w", bootVolumeID, err)
	}
	return &resp.BootVolume, nil
}

// GetVolume fetches a block volume by OCID.
func GetVolume(ctx context.Context, c Clients, volumeID string) (*core.Volume, error) {
	resp, err := c.Blockstorage.GetVolume(ctx, core.GetVolumeRequest{
		VolumeId: common.String(volumeID),
	})
	if err != nil {
		return nil, fmt.Errorf("get volume %s: %w", volumeID, err)
	}
	return &resp.Volume, nil
}
