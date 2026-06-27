// Package oci — vnic.go: VNIC management operations (SPEC S10.5).
// Port of VnicManagementUtils.java. Lists VNIC attachments, gets VNIC
// details (public/private IP, IPv6), enumerates VNICs for a tenant.
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// VnicAttachmentInfo holds the parsed VNIC attachment result.
type VnicAttachmentInfo struct {
	VnicID       string
	InstanceID   string
	InstanceName string
	PublicIP     string
	PrivateIP    string
	Ipv6Addresses []string
	SubnetID     string
	VlanTag      *int
}

// ListVnicAttachmentsForInstance lists all VNIC attachments for an instance.
func ListVnicAttachmentsForInstance(ctx context.Context, computeClient *core.ComputeClient, compartmentID, instanceID string) ([]core.VnicAttachment, error) {
	var out []core.VnicAttachment
	var page *string
	for {
		resp, err := computeClient.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
			CompartmentId: common.String(compartmentID),
			InstanceId:    common.String(instanceID),
			Limit:         common.Int(100),
			Page:          page,
		})
		if err != nil {
			return nil, fmt.Errorf("list vnic attachments for %s: %w", instanceID, err)
		}
		out = append(out, resp.Items...)
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return out, nil
}

// GetVnicInfo resolves a VNIC OCID to its IP details (public, private, IPv6).
func GetVnicInfo(ctx context.Context, vcnClient *core.VirtualNetworkClient, vnicID string) (*VnicAttachmentInfo, error) {
	resp, err := vcnClient.GetVnic(ctx, core.GetVnicRequest{
		VnicId: common.String(vnicID),
	})
	if err != nil {
		return nil, fmt.Errorf("get vnic %s: %w", vnicID, err)
	}
	v := resp.Vnic
	info := &VnicAttachmentInfo{
		VnicID: vnicID,
	}
	if v.PublicIp != nil {
		info.PublicIP = *v.PublicIp
	}
	if v.PrivateIp != nil {
		info.PrivateIP = *v.PrivateIp
	}
	if v.SubnetId != nil {
		info.SubnetID = *v.SubnetId
	}
	info.Ipv6Addresses = v.Ipv6Addresses
	return info, nil
}

// ListAllVnicsForInstance resolves all VNIC attachments for an instance and
// returns them with full VNIC info (IPs, IPv6). Parity with
// VnicManagementUtils.listAllVnicsForInstance.
func ListAllVnicsForInstance(ctx context.Context, computeClient *core.ComputeClient, vcnClient *core.VirtualNetworkClient, compartmentID, instanceID string) ([]VnicAttachmentInfo, error) {
	attachments, err := ListVnicAttachmentsForInstance(ctx, computeClient, compartmentID, instanceID)
	if err != nil {
		return nil, err
	}
	var out []VnicAttachmentInfo
	for _, att := range attachments {
		if att.VnicId == nil {
			continue
		}
		info, err := GetVnicInfo(ctx, vcnClient, *att.VnicId)
		if err != nil {
			continue // per-VNIC error non-fatal
		}
		info.InstanceID = instanceID
		if att.DisplayName != nil {
			info.InstanceName = *att.DisplayName
		}
		out = append(out, *info)
	}
	return out, nil
}
