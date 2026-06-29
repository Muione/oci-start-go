// Package oci — vnic.go: VNIC management operations (SPEC S10.5).
// Port of VnicManagementUtils.java. Lists VNIC attachments, gets VNIC
// details (public/private IP, IPv6), enumerates VNICs for a tenant.
// Phase 11.2 adds batch create/delete, IPv6 management, polling helpers.
package oci

import (
	"context"
	"fmt"
	"math/rand"
	"time"

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

// AssignIpv6ToVnic assigns an IPv6 address to a VNIC. If forceNew is true, it
// unassigns all existing IPv6 addresses first. Returns the new/current IPv6 address.
// Port of OciIpv6Utils.enableOrRefreshVnicIpv6.
func AssignIpv6ToVnic(ctx context.Context, vcnClient *core.VirtualNetworkClient, vnicID string, forceNew bool) (string, error) {
	if forceNew {
		// List and detach existing IPv6 addresses from the VNIC
		var page *string
		for {
			resp, err := vcnClient.ListIpv6s(ctx, core.ListIpv6sRequest{
				VnicId: common.String(vnicID),
				Page:   page,
			})
			if err != nil {
				return "", fmt.Errorf("list ipv6s for vnic %s: %w", vnicID, err)
			}
			for _, ipv6 := range resp.Items {
				if ipv6.Id != nil {
					_, err := vcnClient.Ipv6VnicDetach(ctx, core.Ipv6VnicDetachRequest{
						Ipv6Id: ipv6.Id,
					})
					if err != nil {
						// Non-fatal per-address error
						continue
					}
				}
			}
			if resp.OpcNextPage == nil {
				break
			}
			page = resp.OpcNextPage
		}
	}

	// Assign a new IPv6
	resp, err := vcnClient.CreateIpv6(ctx, core.CreateIpv6Request{
		CreateIpv6Details: core.CreateIpv6Details{
			VnicId: common.String(vnicID),
		},
	})
	if err != nil {
		return "", fmt.Errorf("assign ipv6 to vnic %s: %w", vnicID, err)
	}
	if resp.IpAddress != nil {
		return *resp.IpAddress, nil
	}
	// Fallback: list to find assigned IPv6
	listResp, err := vcnClient.ListIpv6s(ctx, core.ListIpv6sRequest{
		VnicId: common.String(vnicID),
	})
	if err != nil {
		return "", fmt.Errorf("list ipv6s after assign: %w", err)
	}
	if len(listResp.Items) > 0 && listResp.Items[0].IpAddress != nil {
		return *listResp.Items[0].IpAddress, nil
	}
	return "", fmt.Errorf("no ipv6 address assigned")
}

// ---------------------------------------------------------------------------
// Phase 11.2: Batch VNIC creation/deletion types and functions
// ---------------------------------------------------------------------------

// BatchVnicCreationResult mirrors Java BatchVnicCreationResult.
type BatchVnicCreationResult struct {
	InstanceID                string               `json:"instanceId"`
	InstanceDisplayName       string               `json:"instanceDisplayName"`
	RequestedVnicCount        int                  `json:"requestedVnicCount"`
	RequestedIpv6CountPerVnic int                  `json:"requestedIpv6CountPerVnic"`
	SuccessfulVnicCount       int                  `json:"successfulVnicCount"`
	TotalIpv6Count            int                  `json:"totalIpv6Count"`
	VnicResults               []VnicCreationResult `json:"vnicResults"`
	AllSuccessful             bool                 `json:"allSuccessful"`
	Summary                   string               `json:"summary"`
	TotalExecutionTimeMs      int64                `json:"totalExecutionTimeMs"`
}

// VnicCreationResult mirrors Java VnicCreationResult.
type VnicCreationResult struct {
	VnicID          string   `json:"vnicId"`
	VnicDisplayName string   `json:"vnicDisplayName"`
	PrivateIP       string   `json:"privateIp"`
	PublicIP        string   `json:"publicIp"`
	SubnetID        string   `json:"subnetId"`
	AttachmentID    string   `json:"attachmentId"`
	LifecycleState  string   `json:"lifecycleState"`
	Ipv6Addresses   []string `json:"ipv6Addresses"`
	Ipv6IDs         []string `json:"ipv6Ids"`
	IsPrimary       bool     `json:"isPrimary"`
	Success         bool     `json:"success"`
	ErrorMessage    string   `json:"errorMessage,omitempty"`
}

// Ipv6CreationResult mirrors Java Ipv6CreationResult.
type Ipv6CreationResult struct {
	Ipv6ID       string `json:"ipv6Id"`
	Ipv6Address  string `json:"ipv6Address"`
	VnicID       string `json:"vnicId"`
	Success      bool   `json:"success"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// Constants per SPEC section 1.
const (
	MaxVnicPerInstance  = 32
	MaxIpv6PerVnic      = 32
	DefaultTimeoutSec   = 300
	PollIntervalSec     = 3
	SubnetDisplayName   = "oci-start-pro-subnet"
)

// ValidateVnicCreationParams validates vnicCount in [1,32] and ipv6CountPerVnic in [0,32].
func ValidateVnicCreationParams(vnicCount, ipv6CountPerVnic int) error {
	if vnicCount < 1 || vnicCount > MaxVnicPerInstance {
		return fmt.Errorf("vnicCount must be in [1, %d], got %d", MaxVnicPerInstance, vnicCount)
	}
	if ipv6CountPerVnic < 0 || ipv6CountPerVnic > MaxIpv6PerVnic {
		return fmt.Errorf("ipv6CountPerVnic must be in [0, %d], got %d", MaxIpv6PerVnic, ipv6CountPerVnic)
	}
	return nil
}

// CheckSubnetIpv6Support returns true if the subnet has IPv6 CIDR blocks.
func CheckSubnetIpv6Support(ctx context.Context, vcnClient *core.VirtualNetworkClient, subnetID string) (bool, error) {
	resp, err := vcnClient.GetSubnet(ctx, core.GetSubnetRequest{
		SubnetId: common.String(subnetID),
	})
	if err != nil {
		return false, fmt.Errorf("get subnet %s: %w", subnetID, err)
	}
	return len(resp.Subnet.Ipv6CidrBlocks) > 0, nil
}

// IsPrimaryVnic checks if a VNIC is the primary VNIC of an instance.
// Uses earliest timeCreated attachment (Java parity, SPEC section 5).
func IsPrimaryVnic(ctx context.Context, computeClient *core.ComputeClient, compartmentID, instanceID, vnicID string) bool {
	attachments, err := ListVnicAttachmentsForInstance(ctx, computeClient, compartmentID, instanceID)
	if err != nil || len(attachments) == 0 {
		return false
	}
	// Find attachment with earliest timeCreated.
	var earliest *core.VnicAttachment
	for i := range attachments {
		a := &attachments[i]
		if a.VnicId == nil || *a.VnicId != vnicID {
			continue
		}
		if earliest == nil || (a.TimeCreated != nil && (earliest.TimeCreated == nil || a.TimeCreated.Time.Before(earliest.TimeCreated.Time))) {
			earliest = a
		}
	}
	// Also check across all attachments: the primary is the one with earliest timeCreated overall.
	var primaryAtt *core.VnicAttachment
	for i := range attachments {
		a := &attachments[i]
		if a.TimeCreated == nil {
			continue
		}
		if primaryAtt == nil || a.TimeCreated.Time.Before(primaryAtt.TimeCreated.Time) {
			primaryAtt = a
		}
	}
	return primaryAtt != nil && primaryAtt.VnicId != nil && *primaryAtt.VnicId == vnicID
}

// CreateMultipleVnicsWithIpv6 creates batch VNICs on an instance, each with IPv6.
// Parity with Java VnicManagementUtils.createMultipleVnicsWithIpv6.
func CreateMultipleVnicsWithIpv6(ctx context.Context, c Clients, instanceID, subnetID string, vnicCount, ipv6CountPerVnic int) (*BatchVnicCreationResult, error) {
	start := time.Now()

	// Get instance for display name.
	inst, err := GetInstance(ctx, c, instanceID)
	if err != nil {
		return nil, fmt.Errorf("get instance: %w", err)
	}
	instanceName := ""
	if inst.DisplayName != nil {
		instanceName = *inst.DisplayName
	}

	result := &BatchVnicCreationResult{
		InstanceID:                instanceID,
		InstanceDisplayName:       instanceName,
		RequestedVnicCount:        vnicCount,
		RequestedIpv6CountPerVnic: ipv6CountPerVnic,
		AllSuccessful:             true,
	}

	// Check subnet IPv6 support.
	subnetSupportsIpv6, err := CheckSubnetIpv6Support(ctx, c.Vcn, subnetID)
	if err != nil {
		return nil, fmt.Errorf("check subnet ipv6: %w", err)
	}
	effectiveIpv6Count := ipv6CountPerVnic
	if !subnetSupportsIpv6 {
		effectiveIpv6Count = 0
	}

	for i := 0; i < vnicCount; i++ {
		displayName := generateVnicDisplayName(instanceName, i+1)
		vr, err := CreateSingleVnicWithIpv6(ctx, c, instanceID, subnetID, displayName, effectiveIpv6Count, subnetSupportsIpv6)
		if err != nil {
			// Partial failure: return immediately (SPEC section 9.4).
			vr = &VnicCreationResult{
				VnicDisplayName: displayName,
				Success:         false,
				ErrorMessage:    err.Error(),
			}
			result.VnicResults = append(result.VnicResults, *vr)
			result.AllSuccessful = false
			break
		}
		result.VnicResults = append(result.VnicResults, *vr)
		if vr.Success {
			result.SuccessfulVnicCount++
			result.TotalIpv6Count += len(vr.Ipv6Addresses)
		} else {
			result.AllSuccessful = false
			break
		}
	}

	result.TotalExecutionTimeMs = time.Since(start).Milliseconds()
	if result.AllSuccessful {
		result.Summary = fmt.Sprintf("实例 %s 的VNIC创建完成 - 成功: %d/%d, IPv6地址: %d个, 耗时: %dms",
			instanceID, result.SuccessfulVnicCount, vnicCount, result.TotalIpv6Count, result.TotalExecutionTimeMs)
	} else {
		result.Summary = fmt.Sprintf("实例 %s 的VNIC创建部分失败 - 成功: %d/%d, 耗时: %dms",
			instanceID, result.SuccessfulVnicCount, vnicCount, result.TotalExecutionTimeMs)
	}
	return result, nil
}

// CreateSingleVnicWithIpv6 creates one secondary VNIC with IPv6 addresses.
// Called in a loop by CreateMultipleVnicsWithIpv6.
func CreateSingleVnicWithIpv6(ctx context.Context, c Clients, instanceID, subnetID, displayName string, ipv6Count int, subnetSupportsIpv6 bool) (*VnicCreationResult, error) {
	hostnameLabel := generateHostnameLabel()

	// Attach a new secondary VNIC.
	attachResp, err := c.Compute.AttachVnic(ctx, core.AttachVnicRequest{
		AttachVnicDetails: core.AttachVnicDetails{
			InstanceId: common.String(instanceID),
			CreateVnicDetails: &core.CreateVnicDetails{
				SubnetId:              common.String(subnetID),
				DisplayName:           common.String(displayName),
				AssignPublicIp:        common.Bool(true),
				AssignPrivateDnsRecord: common.Bool(true),
				HostnameLabel:         common.String(hostnameLabel),
				SkipSourceDestCheck:   common.Bool(false),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("attach vnic: %w", err)
	}

	attachmentID := *attachResp.VnicAttachment.Id

	// Poll until ATTACHED.
	att, err := WaitForVnicAttachment(ctx, c.Compute, attachmentID, DefaultTimeoutSec*time.Second, PollIntervalSec*time.Second)
	if err != nil {
		return nil, fmt.Errorf("wait for vnic attachment: %w", err)
	}

	vnicID := *att.VnicId

	// Get VNIC details for IPs.
	vnicInfo, err := GetVnicInfo(ctx, c.Vcn, vnicID)
	if err != nil {
		return nil, fmt.Errorf("get vnic info: %w", err)
	}

	result := &VnicCreationResult{
		VnicID:          vnicID,
		VnicDisplayName: displayName,
		PrivateIP:       vnicInfo.PrivateIP,
		PublicIP:        vnicInfo.PublicIP,
		SubnetID:        subnetID,
		AttachmentID:    attachmentID,
		LifecycleState:  string(att.LifecycleState),
		IsPrimary:       false,
		Success:         true,
	}

	// Create IPv6 addresses.
	if ipv6Count > 0 && subnetSupportsIpv6 {
		ipv6Results, err := CreateIpv6ForVnic(ctx, c.Vcn, vnicID, ipv6Count)
		if err != nil {
			result.ErrorMessage = fmt.Sprintf("ipv6 creation partial failure: %v", err)
		}
		for _, r := range ipv6Results {
			if r.Success {
				result.Ipv6Addresses = append(result.Ipv6Addresses, r.Ipv6Address)
				result.Ipv6IDs = append(result.Ipv6IDs, r.Ipv6ID)
			}
		}
	}

	return result, nil
}

// CreateIpv6ForVnic creates multiple IPv6 addresses on an existing VNIC.
func CreateIpv6ForVnic(ctx context.Context, vcnClient *core.VirtualNetworkClient, vnicID string, count int) ([]Ipv6CreationResult, error) {
	results := make([]Ipv6CreationResult, 0, count)
	for i := 0; i < count; i++ {
		displayName := fmt.Sprintf("ipv6-%d", i+1)
		resp, err := vcnClient.CreateIpv6(ctx, core.CreateIpv6Request{
			CreateIpv6Details: core.CreateIpv6Details{
				VnicId:      common.String(vnicID),
				DisplayName: common.String(displayName),
			},
		})
		if err != nil {
			results = append(results, Ipv6CreationResult{
				VnicID:       vnicID,
				Success:      false,
				ErrorMessage: err.Error(),
			})
			continue
		}
		r := Ipv6CreationResult{
			VnicID:  vnicID,
			Success: true,
		}
		if resp.Ipv6.Id != nil {
			r.Ipv6ID = *resp.Ipv6.Id
		}
		if resp.Ipv6.IpAddress != nil {
			r.Ipv6Address = *resp.Ipv6.IpAddress
		}
		results = append(results, r)
	}
	return results, nil
}

// DeleteVnicWithIpv6 deletes a single secondary VNIC and all its IPv6 addresses.
// Returns true on success. Blocks primary VNIC deletion.
func DeleteVnicWithIpv6(ctx context.Context, c Clients, compartmentID, instanceID, vnicID string) (bool, error) {
	// Block primary VNIC deletion.
	if IsPrimaryVnic(ctx, c.Compute, compartmentID, instanceID, vnicID) {
		return false, fmt.Errorf("cannot delete primary VNIC %s", vnicID)
	}

	// 1. Delete all IPv6 addresses.
	if _, err := DeleteAllIpv6FromVnic(ctx, c.Vcn, vnicID); err != nil {
		return false, fmt.Errorf("delete ipv6 from vnic %s: %w", vnicID, err)
	}

	// 2. Detach VNIC from instance.
	if _, err := DetachVnicFromInstance(ctx, c.Compute, instanceID, vnicID); err != nil {
		return false, fmt.Errorf("detach vnic %s: %w", vnicID, err)
	}

	return true, nil
}

// DeleteAllIpv6FromVnic removes all IPv6 addresses from a VNIC.
func DeleteAllIpv6FromVnic(ctx context.Context, vcnClient *core.VirtualNetworkClient, vnicID string) (bool, error) {
	var page *string
	for {
		resp, err := vcnClient.ListIpv6s(ctx, core.ListIpv6sRequest{
			VnicId: common.String(vnicID),
			Page:   page,
		})
		if err != nil {
			return false, fmt.Errorf("list ipv6s for vnic %s: %w", vnicID, err)
		}
		for _, ipv6 := range resp.Items {
			if ipv6.Id != nil {
				_, err := vcnClient.DeleteIpv6(ctx, core.DeleteIpv6Request{
					Ipv6Id: ipv6.Id,
				})
				if err != nil {
					// Non-fatal per-address error, continue.
					continue
				}
			}
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return true, nil
}

// DetachVnicFromInstance detaches a VNIC from an instance and polls until DETACHED.
func DetachVnicFromInstance(ctx context.Context, computeClient *core.ComputeClient, instanceID, vnicID string) (bool, error) {
	// Find the attachment ID for this VNIC.
	attachments, err := ListVnicAttachmentsForInstance(ctx, computeClient, "", instanceID)
	if err != nil {
		return false, fmt.Errorf("list attachments: %w", err)
	}

	var attachmentID string
	for _, a := range attachments {
		if a.VnicId != nil && *a.VnicId == vnicID && a.Id != nil {
			attachmentID = *a.Id
			break
		}
	}
	if attachmentID == "" {
		return false, fmt.Errorf("attachment not found for vnic %s on instance %s", vnicID, instanceID)
	}

	// Detach.
	_, err = computeClient.DetachVnic(ctx, core.DetachVnicRequest{
		VnicAttachmentId: common.String(attachmentID),
	})
	if err != nil {
		return false, fmt.Errorf("detach vnic: %w", err)
	}

	// Poll until detached.
	detached, err := WaitForVnicDetachment(ctx, computeClient, attachmentID, DefaultTimeoutSec*time.Second, PollIntervalSec*time.Second)
	if err != nil {
		return false, fmt.Errorf("wait for detachment: %w", err)
	}
	return detached, nil
}

// DeleteAllSecondaryVnics deletes all non-primary VNICs on an instance.
// Returns a map of vnicID -> success.
func DeleteAllSecondaryVnics(ctx context.Context, c Clients, instanceID, compartmentID string) (map[string]bool, error) {
	// Get all VNIC attachments.
	attachments, err := ListVnicAttachmentsForInstance(ctx, c.Compute, compartmentID, instanceID)
	if err != nil {
		return nil, fmt.Errorf("list vnic attachments: %w", err)
	}

	// Find primary attachment (earliest timeCreated).
	var primaryVnicID string
	var earliest *core.VnicAttachment
	for i := range attachments {
		a := &attachments[i]
		if a.TimeCreated == nil || a.VnicId == nil {
			continue
		}
		if earliest == nil || a.TimeCreated.Time.Before(earliest.TimeCreated.Time) {
			earliest = a
		}
	}
	if earliest != nil && earliest.VnicId != nil {
		primaryVnicID = *earliest.VnicId
	}

	resultMap := make(map[string]bool)
	for _, a := range attachments {
		if a.VnicId == nil {
			continue
		}
		vnicID := *a.VnicId
		if vnicID == primaryVnicID {
			resultMap[vnicID] = true // primary is "success" (skipped)
			continue
		}
		ok, err := DeleteVnicWithIpv6(ctx, c, compartmentID, instanceID, vnicID)
		resultMap[vnicID] = ok && err == nil
	}
	return resultMap, nil
}

// WaitForVnicAttachment polls GetVnicAttachment until ATTACHED or timeout.
// If NotFound is received, it is treated as "not yet created" and polling continues.
func WaitForVnicAttachment(ctx context.Context, computeClient *core.ComputeClient, attachmentID string, timeout, interval time.Duration) (*core.VnicAttachment, error) {
	deadline := time.Now().Add(timeout)
	for {
		resp, err := computeClient.GetVnicAttachment(ctx, core.GetVnicAttachmentRequest{
			VnicAttachmentId: common.String(attachmentID),
		})
		if err != nil {
			// NotFound during attach = not yet created, keep polling.
			if isNotFound(err) {
				if time.Now().After(deadline) {
					return nil, fmt.Errorf("timeout waiting for vnic attachment %s", attachmentID)
				}
				time.Sleep(interval)
				continue
			}
			return nil, fmt.Errorf("get vnic attachment: %w", err)
		}
		if resp.VnicAttachment.LifecycleState == core.VnicAttachmentLifecycleStateAttached {
			return &resp.VnicAttachment, nil
		}
		if resp.VnicAttachment.LifecycleState == core.VnicAttachmentLifecycleStateDetached {
			return nil, fmt.Errorf("vnic attachment %s entered DETACHED state unexpectedly", attachmentID)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for vnic attachment %s (state: %s)", attachmentID, resp.VnicAttachment.LifecycleState)
		}
		time.Sleep(interval)
	}
}

// WaitForVnicDetachment polls GetVnicAttachment until DETACHED/NotFound or timeout.
// If NotFound is received, it is treated as successful deletion.
func WaitForVnicDetachment(ctx context.Context, computeClient *core.ComputeClient, attachmentID string, timeout, interval time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for {
		resp, err := computeClient.GetVnicAttachment(ctx, core.GetVnicAttachmentRequest{
			VnicAttachmentId: common.String(attachmentID),
		})
		if err != nil {
			if isNotFound(err) {
				return true, nil // NotFound = successfully detached/deleted
			}
			return false, fmt.Errorf("get vnic attachment: %w", err)
		}
		if resp.VnicAttachment.LifecycleState == core.VnicAttachmentLifecycleStateDetached {
			return true, nil
		}
		if time.Now().After(deadline) {
			return false, fmt.Errorf("timeout waiting for vnic detachment %s (state: %s)", attachmentID, resp.VnicAttachment.LifecycleState)
		}
		time.Sleep(interval)
	}
}

// --- Internal helpers ---

// generateHostnameLabel generates "oci-start-hn" + 6 random lowercase letters.
func generateHostnameLabel() string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return "oci-start-hn" + string(b)
}

// generateVnicDisplayName generates "vnic-{instanceName}-{index}".
func generateVnicDisplayName(instanceName string, index int) string {
	return fmt.Sprintf("vnic-%s-%d", instanceName, index)
}

// isNotFound checks if an OCI API error is a 404 Not Found.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	// OCI SDK returns ServiceError with HTTPStatus 404.
	type httpStatus interface {
		GetHTTPStatusCode() int
	}
	if hs, ok := err.(httpStatus); ok {
		return hs.GetHTTPStatusCode() == 404
	}
	return false
}
