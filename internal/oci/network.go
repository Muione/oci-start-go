// Package oci — network.go: basic VCN/VNIC domain (Phase 3). Ports
// OciUtils.getVnicPrimary (primary VNIC → public/private IP + IPv6) and
// listVcns. VNIC batch-create, IPv6 enable, public-IP reassignment are later
// phases (SPEC §10.5).
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// GetPrimaryVnic returns the primary VNIC of an instance (or the first
// attachment if none is marked primary). Parity with OciUtils.getVnicPrimary.
func GetPrimaryVnic(ctx context.Context, c Clients, instanceID, compartmentID string) (core.Vnic, error) {
	resp, err := c.Compute.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
		CompartmentId: common.String(compartmentID),
		InstanceId:    common.String(instanceID),
	})
	if err != nil {
		return core.Vnic{}, fmt.Errorf("list vnic attachments: %w", err)
	}
	// First pass: the primary VNIC.
	for _, a := range resp.Items {
		if a.VnicId == nil {
			continue
		}
		vr, err := c.Vcn.GetVnic(ctx, core.GetVnicRequest{VnicId: a.VnicId})
		if err != nil {
			continue
		}
		if vr.Vnic.IsPrimary != nil && *vr.Vnic.IsPrimary {
			return vr.Vnic, nil
		}
	}
	// Fallback: first attachment with a reachable Vnic.
	for _, a := range resp.Items {
		if a.VnicId == nil {
			continue
		}
		vr, err := c.Vcn.GetVnic(ctx, core.GetVnicRequest{VnicId: a.VnicId})
		if err == nil {
			return vr.Vnic, nil
		}
	}
	return core.Vnic{}, fmt.Errorf("no vnic for instance %s", instanceID)
}

// ListVcns lists VCNs in a compartment (basic domain helper).
func ListVcns(ctx context.Context, c Clients, compartmentID string) ([]core.Vcn, error) {
	resp, err := c.Vcn.ListVcns(ctx, core.ListVcnsRequest{
		CompartmentId: common.String(compartmentID),
	})
	if err != nil {
		return nil, fmt.Errorf("list vcns: %w", err)
	}
	return resp.Items, nil
}
