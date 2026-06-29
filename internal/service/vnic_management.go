// Package service — vnic_management.go: Phase 11.2 VNIC batch management.
// Service layer above the OCI wrappers. Resolves tenant from instanceId via
// the instance_detail table, builds OCI clients via oci.WithProxy, and
// delegates to OCI wrapper functions. Parity with Java VnicServiceImpl.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// VnicManagementService manages VNIC operations for OCI instances.
type VnicManagementService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool
}

// NewVnicManagementService constructs a VnicManagementService.
func NewVnicManagementService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *VnicManagementService {
	return &VnicManagementService{store: store, masterKey: masterKey, pool: pool}
}

// --- Response types ---

// VnicLoadDataResult is the full VNIC data response.
type VnicLoadDataResult struct {
	VnicList       []VnicInfo `json:"vnicList"`
	PrimaryVnic    *VnicInfo  `json:"primaryVnic"`
	SecondaryVnics []VnicInfo `json:"secondaryVnics"`
	Statistics     VnicStats  `json:"statistics"`
	TenantID       string     `json:"tenantId,omitempty"`
}

// VnicInfo is a single VNIC's info.
type VnicInfo struct {
	VnicID          string   `json:"vnicId"`
	VnicDisplayName string   `json:"vnicDisplayName"`
	PrivateIP       string   `json:"privateIp"`
	PublicIP        string   `json:"publicIp"`
	SubnetID        string   `json:"subnetId"`
	AttachmentID    string   `json:"attachmentId"`
	LifecycleState  string   `json:"lifecycleState"`
	IsPrimary       bool     `json:"isPrimary"`
	Ipv6Addresses   []string `json:"ipv6Addresses"`
	Ipv6IDs         []string `json:"ipv6Ids"`
	Success         bool     `json:"success"`
	ErrorMessage    string   `json:"errorMessage,omitempty"`
	CreatedAt       string   `json:"createdAt"`
	InstanceID      string   `json:"instanceId"`
	InstanceName    string   `json:"instanceName"`
}

// VnicStats holds VNIC statistics.
type VnicStats struct {
	TotalVnicCount     int `json:"totalVnicCount"`
	ActiveVnicCount    int `json:"activeVnicCount"`
	SecondaryVnicCount int `json:"secondaryVnicCount"`
	TotalIpv6Count     int `json:"totalIpv6Count"`
	PrimaryIpv6Count   int `json:"primaryIpv6Count"`
}

// IpSwitchResult holds the IP switch outcome.
type IpSwitchResult struct {
	OldIP string `json:"oldIp"`
	NewIP string `json:"newIp"`
}

// --- Concurrency guard for IP switch ---

var ipSwitchTasks sync.Map

func tryAcquireIPSwitch(instanceID string) bool {
	_, loaded := ipSwitchTasks.LoadOrStore(instanceID, struct{}{})
	return !loaded
}

func releaseIPSwitch(instanceID string) {
	ipSwitchTasks.Delete(instanceID)
}

// --- Service methods ---

// LoadData returns all VNIC info for an instance (primary + secondary + statistics).
func (s *VnicManagementService) LoadData(ctx context.Context, instanceID string, includeTenantID bool) (*VnicLoadDataResult, error) {
	t, inst, creds, err := s.resolveTenantFromInstanceID(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	var result *VnicLoadDataResult
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		compartmentID := ns2(inst.CompartmentID)

		vnics, err := oci.ListAllVnicsForInstance(ctx, c.Compute, c.Vcn, compartmentID, instanceID)
		if err != nil {
			return fmt.Errorf("list vnics: %w", err)
		}

		loadResult := &VnicLoadDataResult{
			Statistics: VnicStats{TotalVnicCount: len(vnics)},
		}
		if includeTenantID {
			loadResult.TenantID = fmt.Sprintf("%d", t.ID)
		}

		// Find primary VNIC ID using earliest timeCreated attachment.
		attachments, err := oci.ListVnicAttachmentsForInstance(ctx, c.Compute, compartmentID, instanceID)
		if err != nil {
			return fmt.Errorf("list attachments: %w", err)
		}
		primaryVnicID := findPrimaryVnicIDFromAttachments(attachments)

		for _, v := range vnics {
			info := VnicInfo{
				VnicID:        v.VnicID,
				PrivateIP:     v.PrivateIP,
				PublicIP:      v.PublicIP,
				SubnetID:      v.SubnetID,
				Ipv6Addresses: v.Ipv6Addresses,
				IsPrimary:     v.VnicID == primaryVnicID,
				Success:       true,
				InstanceID:    instanceID,
				InstanceName:  v.InstanceName,
			}
			// Find attachment details.
			for _, att := range attachments {
				if att.VnicId != nil && *att.VnicId == v.VnicID {
					if att.Id != nil {
						info.AttachmentID = *att.Id
					}
					info.LifecycleState = string(att.LifecycleState)
					if att.TimeCreated != nil {
						info.CreatedAt = att.TimeCreated.Time.Format(time.RFC3339)
					}
					break
				}
			}
			// Get IPv6 IDs.
			if len(v.Ipv6Addresses) > 0 {
				info.Ipv6IDs = listIpv6IDs(ctx, c.Vcn, v.VnicID)
			}

			loadResult.VnicList = append(loadResult.VnicList, info)
			if info.IsPrimary {
				p := info
				loadResult.PrimaryVnic = &p
				loadResult.Statistics.PrimaryIpv6Count = len(info.Ipv6Addresses)
			} else {
				loadResult.SecondaryVnics = append(loadResult.SecondaryVnics, info)
			}
			loadResult.Statistics.TotalIpv6Count += len(info.Ipv6Addresses)
		}
		loadResult.Statistics.SecondaryVnicCount = len(loadResult.SecondaryVnics)
		loadResult.Statistics.ActiveVnicCount = len(vnics)

		result = loadResult
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// CreateVnics batch-creates secondary VNICs with IPv6.
func (s *VnicManagementService) CreateVnics(ctx context.Context, instanceID, subnetID string, vnicCount, ipv6CountPerVnic int) (*oci.BatchVnicCreationResult, error) {
	if err := oci.ValidateVnicCreationParams(vnicCount, ipv6CountPerVnic); err != nil {
		return nil, fmt.Errorf("参数验证失败: %w", err)
	}

	_, _, creds, err := s.resolveTenantFromInstanceID(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	var result *oci.BatchVnicCreationResult
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		var innerErr error
		result, innerErr = oci.CreateMultipleVnicsWithIpv6(ctx, c, instanceID, subnetID, vnicCount, ipv6CountPerVnic)
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteVnic deletes a single secondary VNIC and its IPv6 addresses.
func (s *VnicManagementService) DeleteVnic(ctx context.Context, instanceID, vnicID string) error {
	_, inst, creds, err := s.resolveTenantFromInstanceID(ctx, instanceID)
	if err != nil {
		return err
	}

	return oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		compartmentID := ns2(inst.CompartmentID)
		_, err := oci.DeleteVnicWithIpv6(ctx, c, compartmentID, instanceID, vnicID)
		return err
	})
}

// CreateIpv6 creates IPv6 addresses on an existing VNIC, then resets the instance.
func (s *VnicManagementService) CreateIpv6(ctx context.Context, instanceID, vnicID string, ipv6Count int) ([]oci.Ipv6CreationResult, error) {
	if ipv6Count < 1 || ipv6Count > oci.MaxIpv6PerVnic {
		return nil, fmt.Errorf("ipv6Count must be in [1, %d], got %d", oci.MaxIpv6PerVnic, ipv6Count)
	}

	_, _, creds, err := s.resolveTenantFromInstanceID(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	var results []oci.Ipv6CreationResult
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		var innerErr error
		results, innerErr = oci.CreateIpv6ForVnic(ctx, c.Vcn, vnicID, ipv6Count)
		if innerErr != nil {
			return innerErr
		}
		// Reset instance for IPv6 to take effect.
		return oci.ResetInstance(ctx, c, instanceID)
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// DeleteIpv6 deletes a single IPv6 address by its address string.
func (s *VnicManagementService) DeleteIpv6(ctx context.Context, instanceID, vnicID, ipv6Address string) error {
	_, _, creds, err := s.resolveTenantFromInstanceID(ctx, instanceID)
	if err != nil {
		return err
	}

	return oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		return deleteIpv6ByAddress(ctx, c.Vcn, vnicID, ipv6Address)
	})
}

// DeleteAllSecondary deletes all non-primary VNICs on an instance.
func (s *VnicManagementService) DeleteAllSecondary(ctx context.Context, instanceID string) (map[string]bool, error) {
	_, inst, creds, err := s.resolveTenantFromInstanceID(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	var resultMap map[string]bool
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		compartmentID := ns2(inst.CompartmentID)
		var innerErr error
		resultMap, innerErr = oci.DeleteAllSecondaryVnics(ctx, c, instanceID, compartmentID)
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	return resultMap, nil
}

// RefreshVnicInfo returns current VNIC data (same as LoadData without tenantId).
func (s *VnicManagementService) RefreshVnicInfo(ctx context.Context, instanceID string) (*VnicLoadDataResult, error) {
	return s.LoadData(ctx, instanceID, false)
}

// ChangeSpecIp switches a VNIC's public IP until it falls within specified CIDR ranges.
func (s *VnicManagementService) ChangeSpecIp(ctx context.Context, instanceID, vnicID string, cidrRanges []string) (*IpSwitchResult, error) {
	if !tryAcquireIPSwitch(instanceID) {
		return nil, fmt.Errorf("该实例正在进行IP切换，请稍后再试")
	}
	defer releaseIPSwitch(instanceID)

	_, inst, creds, err := s.resolveTenantFromInstanceID(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	var result *IpSwitchResult
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		compartmentID := ns2(inst.CompartmentID)

		// Get current public IP.
		vnicInfo, err := oci.GetVnicInfo(ctx, c.Vcn, vnicID)
		if err != nil {
			return fmt.Errorf("get vnic info: %w", err)
		}
		oldIP := vnicInfo.PublicIP

		// Check if already in range.
		if isIPInRanges(oldIP, cidrRanges) {
			result = &IpSwitchResult{OldIP: oldIP, NewIP: oldIP}
			return nil
		}

		maxRetries := 50
		for i := 0; i < maxRetries; i++ {
			newIP, err := oci.ReassignPublicIP(ctx, c, compartmentID, instanceID)
			if err != nil {
				return fmt.Errorf("reassign public IP (attempt %d): %w", i+1, err)
			}

			if isIPInRanges(newIP, cidrRanges) {
				result = &IpSwitchResult{OldIP: oldIP, NewIP: newIP}
				// Update instance_detail.public_ips in DB.
				_ = repo.New(s.store.Write).UpdateInstanceDetailPublicIp(ctx, repo.UpdateInstanceDetailPublicIpParams{
					PublicIps: sql.NullString{String: newIP, Valid: true},
					ID:        inst.RowID,
				})
				return nil
			}

			// Wait 60-80 seconds before next attempt.
			waitSec := 60 + randIntn(21)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(waitSec) * time.Second):
			}
		}

		return fmt.Errorf("IP切换失败: 经过%d次尝试后仍未获得指定范围内的IP", maxRetries)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ConfigureLoadBalancer sets up NAT GW + route table + NLB for an instance.
func (s *VnicManagementService) ConfigureLoadBalancer(ctx context.Context, instanceID string) (*oci.NetworkConfigResult, error) {
	t, inst, creds, err := s.resolveTenantFromInstanceID(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	// Account type gate.
	accountType := ns2(t.AccountType)
	if accountType != "TRIAL_PAID_ACCOUNT" && accountType != "UPGRADE_ACCOUNT" {
		return nil, fmt.Errorf("当前租户不支持")
	}

	var result *oci.NetworkConfigResult
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		// Architecture gate.
		instResp, err := oci.GetInstance(ctx, c, instanceID)
		if err != nil {
			return fmt.Errorf("get instance: %w", err)
		}
		arch := instanceArchitecture(instResp.ShapeConfig)
		if arch != "AMD" {
			return fmt.Errorf("当前租户不支持 (architecture: %s)", arch)
		}

		compartmentID := ns2(inst.CompartmentID)

		primaryVnic, err := oci.GetPrimaryVnic(ctx, c, instanceID, compartmentID)
		if err != nil {
			return fmt.Errorf("get primary vnic: %w", err)
		}
		if primaryVnic.SubnetId == nil {
			return fmt.Errorf("primary vnic has no subnet")
		}
		subnetID := *primaryVnic.SubnetId

		// Get VCN ID from subnet.
		subnetResp, err := c.Vcn.GetSubnet(ctx, core.GetSubnetRequest{
			SubnetId: common.String(subnetID),
		})
		if err != nil {
			return fmt.Errorf("get subnet: %w", err)
		}
		vcnID := *subnetResp.Subnet.VcnId

		privateIP := ""
		if primaryVnic.PrivateIp != nil {
			privateIP = *primaryVnic.PrivateIp
		}

		// 1. Create or get NAT gateway.
		natGW, err := oci.CreateOrGetNatGateway(ctx, c.Vcn, compartmentID, vcnID, "amd")
		if err != nil {
			return fmt.Errorf("create NAT gateway: %w", err)
		}

		// 2. Create or get route table with NAT route.
		routeTable, err := oci.CreateOrGetNatRouteTable(ctx, c.Vcn, compartmentID, vcnID, *natGW.Id, "amd")
		if err != nil {
			return fmt.Errorf("create route table: %w", err)
		}

		// 3. Update primary VNIC's route table.
		if err := oci.UpdateInstanceVnicRouteTable(ctx, c, instanceID, compartmentID, *routeTable.Id); err != nil {
			return fmt.Errorf("update vnic route table: %w", err)
		}

		// 4. Create NLB.
		nlb, err := oci.CreateOrGetNetworkLoadBalancer(ctx, c.NLB, compartmentID, subnetID, instanceID, "amd", privateIP)
		if err != nil {
			return fmt.Errorf("create NLB: %w", err)
		}

		nlbIP := ""
		if len(nlb.IpAddresses) > 0 && nlb.IpAddresses[0].IpAddress != nil {
			nlbIP = *nlb.IpAddresses[0].IpAddress
		}

		result = &oci.NetworkConfigResult{
			Success:                 true,
			Message:                 "实例网络配置完成",
			NatGatewayID:            *natGW.Id,
			NatGatewayName:          "amd",
			RouteTableID:            *routeTable.Id,
			RouteTableName:          "amd",
			RouteTableUpdated:       true,
			LoadBalancerCreated:     true,
			NetworkLoadBalancerID:   *nlb.Id,
			NetworkLoadBalancerName: "amd",
			NlbIPAddress:            nlbIP,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RestoreNetwork reverts load balancer configuration.
func (s *VnicManagementService) RestoreNetwork(ctx context.Context, instanceID string) error {
	t, inst, creds, err := s.resolveTenantFromInstanceID(ctx, instanceID)
	if err != nil {
		return err
	}

	// Account type gate.
	accountType := ns2(t.AccountType)
	if accountType != "TRIAL_PAID_ACCOUNT" && accountType != "UPGRADE_ACCOUNT" {
		return fmt.Errorf("当前租户不支持")
	}

	return oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(c oci.Clients) error {
		// Architecture gate.
		instResp, err := oci.GetInstance(ctx, c, instanceID)
		if err != nil {
			return fmt.Errorf("get instance: %w", err)
		}
		arch := instanceArchitecture(instResp.ShapeConfig)
		if arch != "AMD" {
			return fmt.Errorf("当前租户不支持 (architecture: %s)", arch)
		}

		compartmentID := ns2(inst.CompartmentID)

		// 1. Reset VNIC route table to default.
		if err := oci.ResetVnicToDefaultRouteTable(ctx, c, instanceID, compartmentID); err != nil {
			return fmt.Errorf("reset vnic route table: %w", err)
		}

		// 2. Delete NLBs with display name containing "amd".
		nlbs, err := oci.ListNetworkLoadBalancers(ctx, c.NLB, compartmentID)
		if err != nil {
			return fmt.Errorf("list NLBs: %w", err)
		}
		for _, nlb := range nlbs {
			if strings.Contains(strings.ToLower(nlb.DisplayName), "amd") {
				_ = oci.DeleteNetworkLoadBalancer(ctx, c.NLB, nlb.ID)
			}
		}

		// 3. Delete NAT gateways + associated route tables.
		var page *string
		for {
			resp, err := c.Vcn.ListNatGateways(ctx, core.ListNatGatewaysRequest{
				CompartmentId: common.String(compartmentID),
				Page:          page,
			})
			if err != nil {
				return fmt.Errorf("list NAT gateways: %w", err)
			}
			for _, gw := range resp.Items {
				if gw.DisplayName != nil && strings.Contains(strings.ToLower(*gw.DisplayName), "amd") {
					deleteAssociatedRouteTables(ctx, c.Vcn, compartmentID, *gw.VcnId, *gw.Id)
					_ = oci.DeleteNatGateway(ctx, c.Vcn, *gw.Id)
				}
			}
			if resp.OpcNextPage == nil {
				break
			}
			page = resp.OpcNextPage
		}

		return nil
	})
}

// --- Internal helpers ---

type instanceRow struct {
	RowID         int64
	TenantID      sql.NullInt64
	InstanceID    sql.NullString
	DisplayName   sql.NullString
	CompartmentID sql.NullString
	Shape         sql.NullString
}

// resolveTenantFromInstanceID resolves instanceId -> instance_detail -> tenant -> creds.
func (s *VnicManagementService) resolveTenantFromInstanceID(ctx context.Context, instanceID string) (*repo.Tenant, *instanceRow, oci.Credentials, error) {
	var inst instanceRow
	err := s.store.Read.QueryRowContext(ctx,
		`SELECT id, tenant_id, instance_id, display_name, compartment_id, shape
		 FROM instance_detail WHERE instance_id = ? LIMIT 1`,
		instanceID).Scan(&inst.RowID, &inst.TenantID, &inst.InstanceID, &inst.DisplayName, &inst.CompartmentID, &inst.Shape)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, oci.Credentials{}, fmt.Errorf("找不到对应的租户信息")
		}
		return nil, nil, oci.Credentials{}, fmt.Errorf("查询实例信息失败: %w", err)
	}
	if !inst.TenantID.Valid {
		return nil, nil, oci.Credentials{}, fmt.Errorf("找不到对应的租户信息")
	}

	t, err := repo.New(s.store.Read).FindTenantByID(ctx, inst.TenantID.Int64)
	if err != nil {
		return nil, nil, oci.Credentials{}, fmt.Errorf("查找租户失败: %w", err)
	}
	creds := tenantToCreds(t)
	return &t, &inst, creds, nil
}

// findPrimaryVnicIDFromAttachments finds the primary VNIC ID from attachments (earliest timeCreated).
func findPrimaryVnicIDFromAttachments(attachments []core.VnicAttachment) string {
	var earliest *core.VnicAttachment
	for i := range attachments {
		a := &attachments[i]
		if a.VnicId == nil || a.TimeCreated == nil {
			continue
		}
		if earliest == nil || a.TimeCreated.Time.Before(earliest.TimeCreated.Time) {
			earliest = a
		}
	}
	if earliest != nil && earliest.VnicId != nil {
		return *earliest.VnicId
	}
	return ""
}

// listIpv6IDs lists IPv6 addresses on a VNIC and returns their IDs.
func listIpv6IDs(ctx context.Context, vcnClient *core.VirtualNetworkClient, vnicID string) []string {
	resp, err := vcnClient.ListIpv6s(ctx, core.ListIpv6sRequest{
		VnicId: common.String(vnicID),
	})
	if err != nil {
		return nil
	}
	var ids []string
	for _, ipv6 := range resp.Items {
		if ipv6.Id != nil {
			ids = append(ids, *ipv6.Id)
		}
	}
	return ids
}

// deleteIpv6ByAddress lists all IPv6 on a VNIC, finds the one matching the
// address string, and deletes it.
func deleteIpv6ByAddress(ctx context.Context, vcnClient *core.VirtualNetworkClient, vnicID, ipv6Address string) error {
	resp, err := vcnClient.ListIpv6s(ctx, core.ListIpv6sRequest{
		VnicId: common.String(vnicID),
	})
	if err != nil {
		return fmt.Errorf("list ipv6s: %w", err)
	}
	for _, ipv6 := range resp.Items {
		if ipv6.IpAddress != nil && *ipv6.IpAddress == ipv6Address && ipv6.Id != nil {
			_, err := vcnClient.DeleteIpv6(ctx, core.DeleteIpv6Request{
				Ipv6Id: ipv6.Id,
			})
			if err != nil {
				return fmt.Errorf("delete ipv6: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("ipv6 address %s not found on vnic %s", ipv6Address, vnicID)
}

// isIPInRanges checks if an IP address falls within any of the given CIDR ranges.
func isIPInRanges(ipStr string, cidrRanges []string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, cidr := range cidrRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// instanceArchitecture derives architecture from shape config.
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

// deleteAssociatedRouteTables finds and deletes route tables that route to the given NAT gateway.
func deleteAssociatedRouteTables(ctx context.Context, vcnClient *core.VirtualNetworkClient, compartmentID, vcnID, natGatewayID string) {
	var page *string
	for {
		resp, err := vcnClient.ListRouteTables(ctx, core.ListRouteTablesRequest{
			CompartmentId: common.String(compartmentID),
			VcnId:         common.String(vcnID),
			Page:          page,
		})
		if err != nil {
			return
		}
		for _, rt := range resp.Items {
			if rt.DisplayName != nil && strings.Contains(strings.ToLower(*rt.DisplayName), "amd") {
				// Check if this route table has rules pointing to the NAT gateway.
				for _, rule := range rt.RouteRules {
					if rule.NetworkEntityId != nil && *rule.NetworkEntityId == natGatewayID {
						_ = oci.DeleteRouteTable(ctx, vcnClient, *rt.Id)
						break
					}
				}
			}
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
}

// ns2 unwraps a sql.NullString, returning "" when invalid.
func ns2(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

// randIntn returns a random int in [0, n).
func randIntn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(time.Now().UnixNano()%int64(n))
}
