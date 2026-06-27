// Package oci — identity.go: basic IAM domain (compartments). Ports the
// compartment enumeration used by OciUtils.getAllInstancesByTenant.
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

// ListCompartments returns all active subcompartments reachable from
// tenancyOCID (CompartmentIdInSubtree=true, AccessLevel=Accessible,
// LifecycleState=Active). Note: the tenancy (root compartment) itself is NOT
// included in this list — callers that need the root must add tenancyOCID.
func ListCompartments(ctx context.Context, c Clients, tenancyOCID string) ([]identity.Compartment, error) {
	var out []identity.Compartment
	var page *string
	for {
		resp, err := c.Identity.ListCompartments(ctx, identity.ListCompartmentsRequest{
			CompartmentId:          common.String(tenancyOCID),
			CompartmentIdInSubtree: common.Bool(true),
			AccessLevel:            identity.ListCompartmentsAccessLevelAccessible,
			LifecycleState:         identity.CompartmentLifecycleStateActive,
			Limit:                  common.Int(200),
			Page:                   page,
		})
		if err != nil {
			return nil, fmt.Errorf("list compartments: %w", err)
		}
		out = append(out, resp.Items...)
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return out, nil
}
