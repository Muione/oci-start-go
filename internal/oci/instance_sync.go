// Package oci — instance_sync.go: enumerates a tenancy's instances and
// assembles local InstanceDetail rows. Port of OciUtils.getAllInstancesByTenant.
// Used by service.TenantService.SyncOci (delete-by-tenant + insert-all).
package oci

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// ListInstancesByTenant enumerates all non-terminated instances across the
// tenancy's root + active subcompartments and assembles local InstanceDetail
// rows (tenant_id = tenantID). Parity with OciUtils.getAllInstancesByTenant.
// Uses direct (non-proxy) clients; proxy is only for launch/audit (Phase 4).
func ListInstancesByTenant(ctx context.Context, tenantID int64, creds Credentials, masterKey []byte) ([]repo.InsertInstanceDetailParams, error) {
	prov, err := NewProvider(creds, masterKey)
	if err != nil {
		return nil, err
	}
	clients, err := NewClients(prov)
	if err != nil {
		return nil, err
	}

	compartments, err := ListCompartments(ctx, clients, creds.Tenancy)
	if err != nil {
		return nil, err
	}

	// Iterate root (tenancy) + all active subcompartments.
	compIDs := []string{creds.Tenancy}
	for _, comp := range compartments {
		if comp.Id != nil {
			compIDs = append(compIDs, *comp.Id)
		}
	}

	var rows []repo.InsertInstanceDetailParams
	now := time.Now().Format("2006-01-02 15:04:05")
	for _, cid := range compIDs {
		instances, err := ListInstances(ctx, clients, cid)
		if err != nil {
			continue // per-compartment failure non-fatal (parity)
		}
		for _, inst := range instances {
			if inst.LifecycleState == core.InstanceLifecycleStateTerminated {
				continue
			}
			rows = append(rows, buildInstanceDetailRow(ctx, clients, tenantID, inst, cid, now))
		}
	}
	return rows, nil
}

func buildInstanceDetailRow(ctx context.Context, c Clients, tenantID int64, inst core.Instance, compartmentID, now string) repo.InsertInstanceDetailParams {
	row := repo.InsertInstanceDetailParams{
		TenantID:             sql.NullInt64{Int64: tenantID, Valid: true},
		InstanceID:           nullStr(ptrStr(inst.Id)),
		DisplayName:          nullStr(ptrStr(inst.DisplayName)),
		Shape:                nullStr(ptrStr(inst.Shape)),
		State:                nullStr(string(inst.LifecycleState)),
		AvailabilityDomain:   nullStr(ptrStr(inst.AvailabilityDomain)),
		CompartmentID:        nullStr(compartmentID),
		CloudType:            sql.NullInt64{Int64: 1, Valid: true}, // ORACLE_CLOUD
		CreateTime:           sql.NullString{String: now, Valid: true},
		Architecture:         nullStr("NONE"),
		ProcessorDescription: nullStr("NONE"),
	}
	if inst.ShapeConfig != nil {
		if inst.ShapeConfig.ProcessorDescription != nil {
			row.ProcessorDescription = nullStr(*inst.ShapeConfig.ProcessorDescription)
		}
		row.Architecture = nullStr(instanceArchitecture(inst.ShapeConfig))
		if inst.ShapeConfig.Ocpus != nil {
			row.Ocpus = sql.NullInt64{Int64: int64(*inst.ShapeConfig.Ocpus), Valid: true}
		}
		if inst.ShapeConfig.MemoryInGBs != nil {
			row.MemoryInGbs = sql.NullInt64{Int64: int64(*inst.ShapeConfig.MemoryInGBs), Valid: true}
		}
	}
	// Primary VNIC: public/private IP + IPv6 + vnic id.
	if vnic, err := GetPrimaryVnic(ctx, c, ptrStr(inst.Id), compartmentID); err == nil {
		row.PublicIps = nullStr(ptrStr(vnic.PublicIp))
		row.PrivateIps = nullStr(ptrStr(vnic.PrivateIp))
		if len(vnic.Ipv6Addresses) > 0 {
			row.Ipv6Addresses = nullStr(strings.Join(vnic.Ipv6Addresses, ","))
		}
		if vnic.Id != nil {
			row.VnicIds = nullStr(*vnic.Id)
		}
	}
	// Boot volume: id/name/size/vpus (first attachment).
	if id, name, size, vpus, ok := bootVolumeInfo(ctx, c, ptrStr(inst.Id), compartmentID, ptrStr(inst.AvailabilityDomain)); ok {
		row.BootVolumeID = nullStr(id)
		row.BootVolumeName = nullStr(name)
		if size != nil {
			row.BootVolumeSizeInGbs = sql.NullInt64{Int64: *size, Valid: true}
		}
		if vpus != nil {
			row.VpusPerGb = nullStr(fmt.Sprintf("%d", *vpus))
		}
	}
	return row
}

func bootVolumeInfo(ctx context.Context, c Clients, instanceID, compartmentID, availabilityDomain string) (id, name string, size, vpus *int64, ok bool) {
	resp, err := c.Compute.ListBootVolumeAttachments(ctx, core.ListBootVolumeAttachmentsRequest{
		CompartmentId:      common.String(compartmentID),
		AvailabilityDomain: common.String(availabilityDomain),
		InstanceId:         common.String(instanceID),
	})
	if err != nil {
		return
	}
	for _, a := range resp.Items {
		if a.BootVolumeId == nil {
			continue
		}
		bv, err := c.Blockstorage.GetBootVolume(ctx, core.GetBootVolumeRequest{BootVolumeId: a.BootVolumeId})
		if err != nil {
			continue
		}
		return ptrStr(bv.Id), ptrStr(bv.DisplayName), bv.SizeInGBs, bv.VpusPerGB, true
	}
	return
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
