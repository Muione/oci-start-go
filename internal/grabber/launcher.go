// Package grabber — launcher.go implements the full createInstanceData
// orchestration (SPEC S8.4). Port of OracleCloudService.createInstanceData:
// if an OciComputerInfo cache exists, deserialize it and launch per AD;
// otherwise, query live OCI resources (ADs, shapes, images, VCN/Subnet/NSG)
// and build the LaunchInstanceDetails from scratch.
package grabber

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"

	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
)

// cachedLaunchResources holds the pre-discovered OCI resources serialized in
// oci_computer_info.computer_create_json. When present, subsequent grabs for
// the same boot task can skip the expensive discovery phase (list ADs, shapes,
// images, VCN/Subnet/NSG) and launch directly from the cached data.
type cachedLaunchResources struct {
	AvailabilityDomain string `json:"availabilityDomain"`
	Shape              string `json:"shape"`
	ImageID            string `json:"imageId"`
	SubnetID           string `json:"subnetId"`
	NsgID              string `json:"nsgId"`
	Architecture       string `json:"architecture"`
	Region             string `json:"region"`
}

// LauncherInput carries the resolved parameters for the full launch flow.
type LauncherInput struct {
	Task          repo.BootInstance
	TenantID      int64
	CompartmentID string // tenancy OCID
	Region        string // region code
	Architecture  string // ARM / AMD
	Ocpu          int64
	Memory        int64
	Disk          int64
	ImageID       string // optional; resolved if empty
	OperatingSystem        string
	OperatingSystemVersion string
	RootPassword           string
}

// createInstanceData is the full orchestration: check cached OciComputerInfo,
// otherwise perform the live resource discovery + launch flow. Each AD is
// tried in sequence; the first successful launch is returned. After a
// successful live-discovery launch, the discovered resources are cached into
// oci_computer_info for subsequent fast-path launches.
func (e *Engine) createInstanceData(ctx context.Context, clients oci.Clients, input LauncherInput) (*GrabResult, error) {
	q := repo.New(e.deps.Store.Write)
	bootID := ns(input.Task.BootID)

	// Check for cached OciComputerInfo (computerCreateJson).
	cached, err := q.FindComputerInfoByBootIDStr(ctx, input.Task.BootID)
	if err == nil && cached.ComputerCreateJson.Valid && cached.ComputerCreateJson.String != "" {
		var res cachedLaunchResources
		jsonErr := json.Unmarshal([]byte(cached.ComputerCreateJson.String), &res)
		if jsonErr == nil && res.Shape != "" {
			e.deps.Logger.Debug().Str("bootId", bootID).Str("shape", res.Shape).Msg("grabber: using cached computer info, launching directly")
			// Cached path: skip discovery, launch directly with cached resources.
			return e.launchFromCache(ctx, clients, input, &res)
		}
		e.deps.Logger.Warn().Str("bootId", bootID).Err(jsonErr).Msg("grabber: cached computer info invalid, falling back to live discovery")
	}

	// Live discovery path: get ADs, find compatible shape+image, ensure
	// network resources, then launch.
	result, err := e.launchWithDiscovery(ctx, clients, input)
	if err != nil {
		return nil, err
	}

	// Cache the discovered resources for future grabs.
	e.cacheDiscoveredResources(ctx, input, result)

	return result, nil
}

// launchFromCache launches an instance using cached (pre-discovered) OCI
// resources, skipping the expensive discovery phase.
func (e *Engine) launchFromCache(ctx context.Context, clients oci.Clients, input LauncherInput, cache *cachedLaunchResources) (*GrabResult, error) {
	bootID := ns(input.Task.BootID)
	opcRetryToken := fmt.Sprintf("instance-oci-start-%s-%d-%d-%d",
		bootID, input.Ocpu, input.Memory, input.Disk)

	details := buildLaunchDetails(input, cache.AvailabilityDomain, cache.Shape, cache.ImageID, cache.SubnetID, cache.NsgID)

	req := core.LaunchInstanceRequest{
		LaunchInstanceDetails: details,
		OpcRetryToken:         common.String(opcRetryToken),
	}
	resp, err := clients.Compute.LaunchInstance(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("cached launch in AD %s: %w", cache.AvailabilityDomain, err)
	}

	inst := resp.Instance
	if inst.Id == nil {
		return nil, fmt.Errorf("cached launch returned nil instance id")
	}

	info, err := waitForRunning(ctx, clients.Compute, *inst.Id)
	if err != nil {
		return nil, fmt.Errorf("wait for running (cached): %w", err)
	}

	return &GrabResult{
		TaskID:             bootID,
		Success:            true,
		InstanceID:         info.InstanceID,
		PublicIP:           info.PublicIP,
		AvailabilityDomain: cache.AvailabilityDomain,
		Shape:              cache.Shape,
		ImageID:            cache.ImageID,
		SubnetID:           cache.SubnetID,
		NsgID:              cache.NsgID,
	}, nil
}

// cacheDiscoveredResources saves the resources discovered during a live launch
// into oci_computer_info so subsequent grabs for the same boot task can skip
// the expensive discovery phase.
func (e *Engine) cacheDiscoveredResources(ctx context.Context, input LauncherInput, result *GrabResult) {
	if result == nil || !result.Success {
		return
	}

	res, err := json.Marshal(cachedLaunchResources{
		AvailabilityDomain: result.AvailabilityDomain,
		Shape:              result.Shape,
		ImageID:            result.ImageID,
		SubnetID:           result.SubnetID,
		NsgID:              result.NsgID,
		Architecture:       input.Architecture,
		Region:             input.Region,
	})
	if err != nil {
		e.deps.Logger.Warn().Err(err).Str("bootId", ns(input.Task.BootID)).Msg("grabber: failed to marshal cached resources")
		return
	}

	q := repo.New(e.deps.Store.Write)
	if err := q.InsertComputerInfo(ctx, repo.InsertComputerInfoParams{
		BootIDStr:          input.Task.BootID,
		ComputerCreateJson: sql.NullString{String: string(res), Valid: true},
		TenantID:           sql.NullInt64{Int64: input.TenantID, Valid: true},
		Architecture:       sql.NullString{String: input.Architecture, Valid: true},
		CloudType:          input.Task.CloudType,
		ComputerRegion:     sql.NullString{String: input.Region, Valid: true},
	}); err != nil {
		e.deps.Logger.Warn().Err(err).Str("bootId", ns(input.Task.BootID)).Msg("grabber: failed to cache resource info (non-fatal)")
	} else {
		e.deps.Logger.Debug().Str("bootId", ns(input.Task.BootID)).Str("shape", result.Shape).Msg("grabber: cached discovered resources")
	}
}

// launchWithDiscovery performs the live resource discovery flow:
// getAvailabilityDomains → checkShapes (risk gate) → getShape → getImage →
// ensure VCN/Subnet/NSG → build details → launch.
func (e *Engine) launchWithDiscovery(ctx context.Context, clients oci.Clients, input LauncherInput) (*GrabResult, error) {
	compartmentID := input.CompartmentID

	// 1. List Availability Domains.
	ads, err := listADs(ctx, clients.Identity, compartmentID)
	if err != nil {
		return nil, fmt.Errorf("list ADs: %w", err)
	}
	if len(ads) == 0 {
		return nil, fmt.Errorf("no availability domains in compartment")
	}

	// 2. For each AD, try to find a compatible shape and launch.
	for _, ad := range ads {
		if ad.Name == nil {
			continue
		}
		adName := *ad.Name

		// Check available shapes in this AD.
		shapes, err := listShapes(ctx, clients.Compute, compartmentID, adName, input.Architecture)
		if err != nil {
			e.deps.Logger.Warn().Err(err).Str("ad", adName).Msg("grabber: list shapes failed")
			continue
		}
		if len(shapes) == 0 {
			// Risk gate: no compatible shape → skip this AD.
			// If ALL ADs return no shape, the account may be NOT_AUTH / no capacity.
			e.deps.Logger.Debug().Str("ad", adName).Str("arch", input.Architecture).Msg("grabber: no compatible shape")
			continue
		}
		shape := shapes[0] // pick first compatible shape

		// Find a compatible image.
		imageID, err := findImage(ctx, clients.Compute, compartmentID, shape, input.OperatingSystem, input.OperatingSystemVersion)
		if err != nil {
			e.deps.Logger.Warn().Err(err).Str("ad", adName).Msg("grabber: find image failed")
			continue
		}

		// Ensure network resources (VCN, subnet, NSG) exist for this region/compartment.
		subnetID, nsgID, err := e.ensureNetworkResources(ctx, clients, compartmentID, adName, input.Region)
		if err != nil {
			e.deps.Logger.Warn().Err(err).Str("ad", adName).Msg("grabber: ensure network failed")
			continue
		}

		// Build LaunchInstanceDetails.
		details := buildLaunchDetails(input, adName, shape, imageID, subnetID, nsgID)

		// Generate opcRetryToken.
		opcRetryToken := fmt.Sprintf("instance-oci-start-%s-%d-%d-%d",
			ns(input.Task.BootID), input.Ocpu, input.Memory, input.Disk)

		// Launch and wait.
		req := core.LaunchInstanceRequest{
			LaunchInstanceDetails: details,
			OpcRetryToken:         common.String(opcRetryToken),
		}
		resp, err := clients.Compute.LaunchInstance(ctx, req)
		if err != nil {
			e.deps.Logger.Warn().Err(err).Str("ad", adName).Msg("grabber: launch failed in AD")
			continue
		}

		inst := resp.Instance
		if inst.Id == nil {
			continue
		}

		// Wait for Running state.
		info, err := waitForRunning(ctx, clients.Compute, *inst.Id)
		if err != nil {
			e.deps.Logger.Warn().Err(err).Str("ad", adName).Str("instanceId", *inst.Id).Msg("grabber: wait for running failed")
			continue
		}

		return &GrabResult{
			TaskID:             ns(input.Task.BootID),
			Success:            true,
			InstanceID:         info.InstanceID,
			PublicIP:           info.PublicIP,
			AvailabilityDomain: adName,
			Shape:              shape,
			ImageID:            imageID,
			SubnetID:           subnetID,
			NsgID:              nsgID,
		}, nil
	}

	return nil, fmt.Errorf("failed to launch in any AD")
}

// ensureNetworkResources ensures a VCN, subnet, and NSG exist for the given
// compartment. Resources are cached per region+compartment to avoid creating
// new VCNs on every launch (parity with Java behavior of reusing existing
// shared network resources).
func (e *Engine) ensureNetworkResources(ctx context.Context, clients oci.Clients, compartmentID, adName, region string) (subnetID, nsgID string, err error) {
	// List existing VCNs in the compartment.
	vcns, err := clients.Vcn.ListVcns(ctx, core.ListVcnsRequest{
		CompartmentId: common.String(compartmentID),
	})
	if err != nil {
		return "", "", fmt.Errorf("list VCNs: %w", err)
	}

	var vcnID string
	if len(vcns.Items) == 0 {
		// Create a new VCN with a private CIDR block.
		cidr := "10.0.0.0/16"
		resp, err := clients.Vcn.CreateVcn(ctx, core.CreateVcnRequest{
			CreateVcnDetails: core.CreateVcnDetails{
				CompartmentId: common.String(compartmentID),
				DisplayName:   common.String("oci-start-grab-vcn"),
				CidrBlock:     common.String(cidr),
				DnsLabel:      common.String("ocistart"),
			},
		})
		if err != nil {
			return "", "", fmt.Errorf("create VCN: %w", err)
		}
		vcnID = *resp.Vcn.Id

		// Create Internet Gateway.
		igResp, err := clients.Vcn.CreateInternetGateway(ctx, core.CreateInternetGatewayRequest{
			CreateInternetGatewayDetails: core.CreateInternetGatewayDetails{
				CompartmentId: common.String(compartmentID),
				VcnId:         common.String(vcnID),
				DisplayName:   common.String("oci-start-grab-igw"),
				IsEnabled:     common.Bool(true),
			},
		})
		if err != nil {
			return "", "", fmt.Errorf("create IGW: %w", err)
		}

		// Add route rule 0.0.0.0/0 → IGW.
		_, err = clients.Vcn.UpdateRouteTable(ctx, core.UpdateRouteTableRequest{
			RtId: common.String(vcnID), // default route table
			UpdateRouteTableDetails: core.UpdateRouteTableDetails{
				RouteRules: []core.RouteRule{
					{
						Destination:     common.String("0.0.0.0/0"),
						NetworkEntityId: igResp.InternetGateway.Id,
					},
				},
			},
		})
		if err != nil {
			e.deps.Logger.Warn().Err(err).Msg("grabber: add route rule failed, continuing")
		}
	} else {
		vcnID = *vcns.Items[0].Id
	}

	// Find or create subnet in the AD.
	subnets, err := clients.Vcn.ListSubnets(ctx, core.ListSubnetsRequest{
		CompartmentId: common.String(compartmentID),
		VcnId:         common.String(vcnID),
	})
	if err != nil {
		return "", "", fmt.Errorf("list subnets: %w", err)
	}

	if len(subnets.Items) == 0 {
		// Create a subnet with a /24 within the VCN CIDR.
		subResp, err := clients.Vcn.CreateSubnet(ctx, core.CreateSubnetRequest{
			CreateSubnetDetails: core.CreateSubnetDetails{
				CompartmentId:        common.String(compartmentID),
				VcnId:                common.String(vcnID),
				DisplayName:          common.String("oci-start-grab-subnet"),
				CidrBlock:            common.String("10.0.0.0/24"),
				AvailabilityDomain:   common.String(adName),
				ProhibitPublicIpOnVnic: common.Bool(false),
			},
		})
		if err != nil {
			return "", "", fmt.Errorf("create subnet: %w", err)
		}
		subnetID = *subResp.Subnet.Id
	} else {
		// Pick first subnet in the AD, or fall back to the first subnet.
		for _, s := range subnets.Items {
			if s.AvailabilityDomain != nil && *s.AvailabilityDomain == adName {
				subnetID = *s.Id
				break
			}
		}
		if subnetID == "" {
			subnetID = *subnets.Items[0].Id
		}
	}

	// Find or create Network Security Group.
	nsgs, err := clients.Vcn.ListNetworkSecurityGroups(ctx, core.ListNetworkSecurityGroupsRequest{
		CompartmentId: common.String(compartmentID),
		VcnId:         common.String(vcnID),
	})
	if err != nil {
		return "", "", fmt.Errorf("list NSGs: %w", err)
	}

	if len(nsgs.Items) == 0 {
		nsgResp, err := clients.Vcn.CreateNetworkSecurityGroup(ctx, core.CreateNetworkSecurityGroupRequest{
			CreateNetworkSecurityGroupDetails: core.CreateNetworkSecurityGroupDetails{
				CompartmentId: common.String(compartmentID),
				VcnId:         common.String(vcnID),
				DisplayName:   common.String("oci-start-grab-nsg"),
			},
		})
		if err != nil {
			return "", "", fmt.Errorf("create NSG: %w", err)
		}
		nsgID = *nsgResp.NetworkSecurityGroup.Id

		// Add HTTP/SSH ingress rules.
		_ = addIngressRules(ctx, clients.Vcn, nsgID)
	} else {
		nsgID = *nsgs.Items[0].Id
	}

	return subnetID, nsgID, nil
}

// addIngressRules adds HTTP (80, 443) and SSH (22) ingress rules to the NSG.
func addIngressRules(ctx context.Context, vcn *core.VirtualNetworkClient, nsgID string) error {
	rules := []core.AddSecurityRuleDetails{
		{
			Direction:   core.AddSecurityRuleDetailsDirectionIngress,
			Protocol:    common.String("6"), // TCP
			Source:      common.String("0.0.0.0/0"),
			Description: common.String("HTTP"),
			TcpOptions: &core.TcpOptions{
				DestinationPortRange: &core.PortRange{Max: common.Int(80), Min: common.Int(80)},
			},
		},
		{
			Direction:   core.AddSecurityRuleDetailsDirectionIngress,
			Protocol:    common.String("6"),
			Source:      common.String("0.0.0.0/0"),
			Description: common.String("HTTPS"),
			TcpOptions: &core.TcpOptions{
				DestinationPortRange: &core.PortRange{Max: common.Int(443), Min: common.Int(443)},
			},
		},
		{
			Direction:   core.AddSecurityRuleDetailsDirectionIngress,
			Protocol:    common.String("6"),
			Source:      common.String("0.0.0.0/0"),
			Description: common.String("SSH"),
			TcpOptions: &core.TcpOptions{
				DestinationPortRange: &core.PortRange{Max: common.Int(22), Min: common.Int(22)},
			},
		},
	}
	_, err := vcn.AddNetworkSecurityGroupSecurityRules(ctx, core.AddNetworkSecurityGroupSecurityRulesRequest{
		NetworkSecurityGroupId: common.String(nsgID),
		AddNetworkSecurityGroupSecurityRulesDetails: core.AddNetworkSecurityGroupSecurityRulesDetails{
			SecurityRules: rules,
		},
	})
	return err
}

// listADs returns all availability domains in the compartment.
func listADs(ctx context.Context, client *identity.IdentityClient, compartmentID string) ([]identity.AvailabilityDomain, error) {
	resp, err := client.ListAvailabilityDomains(ctx, identity.ListAvailabilityDomainsRequest{
		CompartmentId: common.String(compartmentID),
	})
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// listShapes returns shapes compatible with the given architecture in an AD.
func listShapes(ctx context.Context, client *core.ComputeClient, compartmentID, adName, architecture string) ([]string, error) {
	resp, err := client.ListShapes(ctx, core.ListShapesRequest{
		CompartmentId:      common.String(compartmentID),
		AvailabilityDomain: common.String(adName),
	})
	if err != nil {
		return nil, err
	}

	isARM := strings.EqualFold(architecture, "ARM")
	var out []string
	for _, shape := range resp.Items {
		if shape.Shape == nil {
			continue
		}
		name := *shape.Shape
		// Filter: only VM shapes (not BM).
		if !strings.HasPrefix(name, "VM.") {
			continue
		}
		shapeIsARM := strings.Contains(strings.ToLower(name), "a1") ||
			strings.Contains(strings.ToLower(name), "ampere") ||
			strings.Contains(strings.ToLower(name), "arm")
		if isARM == shapeIsARM {
			out = append(out, name)
		}
	}
	return out, nil
}

// findImage returns an image OCID matching the given shape and OS preferences.
func findImage(ctx context.Context, client *core.ComputeClient, compartmentID, shape, os, osVersion string) (string, error) {
	resp, err := client.ListImages(ctx, core.ListImagesRequest{
		CompartmentId: common.String(compartmentID),
		Shape:         common.String(shape),
	})
	if err != nil {
		return "", err
	}

	// Prefer an image matching the OS/version hint; fall back to any Canonical Ubuntu or Oracle Linux.
	osLower := strings.ToLower(os)
	for _, img := range resp.Items {
		if img.Id == nil || img.DisplayName == nil {
			continue
		}
		name := strings.ToLower(*img.DisplayName)
		if osLower != "" && strings.Contains(name, osLower) {
			if osVersion == "" || strings.Contains(name, strings.ToLower(osVersion)) {
				return *img.Id, nil
			}
		}
	}
	// Fallback: first Canonical Ubuntu or Oracle Linux image.
	for _, img := range resp.Items {
		if img.Id == nil || img.DisplayName == nil {
			continue
		}
		name := strings.ToLower(*img.DisplayName)
		if strings.Contains(name, "canonical") || strings.Contains(name, "ubuntu") ||
			strings.Contains(name, "oracle") && strings.Contains(name, "linux") {
			return *img.Id, nil
		}
	}
	if len(resp.Items) > 0 && resp.Items[0].Id != nil {
		return *resp.Items[0].Id, nil
	}
	return "", fmt.Errorf("no compatible image found for shape %s", shape)
}

// buildLaunchDetails constructs the complete LaunchInstanceDetails.
func buildLaunchDetails(input LauncherInput, adName, shape, imageID, subnetID, nsgID string) core.LaunchInstanceDetails {
	rootPass := input.RootPassword
	var metadata map[string]string
	if rootPass != "" {
		userData := fmt.Sprintf(`#!/bin/bash
echo "root:%s" | chpasswd
sed -i 's/^#PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
sed -i 's/^PasswordAuthentication.*no/PasswordAuthentication yes/' /etc/ssh/sshd_config
systemctl restart sshd
`, rootPass)
		metadata = map[string]string{"user_data": userData}
	}

	details := core.LaunchInstanceDetails{
		DisplayName:   common.String("oci-start-" + ns(input.Task.BootID)),
		CompartmentId: common.String(input.CompartmentID),
		AvailabilityDomain: common.String(adName),
		Shape:              common.String(shape),
		ShapeConfig: &core.LaunchInstanceShapeConfigDetails{
			Ocpus:       common.Float32(float32(input.Ocpu)),
			MemoryInGBs: common.Float32(float32(input.Memory)),
		},
		CreateVnicDetails: &core.CreateVnicDetails{
			AssignPublicIp: common.Bool(true),
			DisplayName:    common.String("primary-vnic"),
			SubnetId:       common.String(subnetID),
			NsgIds:         []string{nsgID},
		},
		SourceDetails: core.InstanceSourceViaImageDetails{
			ImageId:             common.String(imageID),
			BootVolumeSizeInGBs: common.Int64(input.Disk),
		},
		Metadata: metadata,
	}

	// Use SubnetId and NsgIds explicitly.
	_ = details.CreateVnicDetails

	return details
}

var _ = sql.NullString{} // keep import
var _ = oci.Clients{}    // keep import
