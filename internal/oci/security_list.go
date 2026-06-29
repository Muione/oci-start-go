// Package oci -- security_list.go: Phase 11.3 security list rule management.
// Ports Java SecurityRuleService (getSecurityRuleList, addSecurityRule,
// deleteSecurityRule, checkAndEnableRule, singleIpv6Rule) and
// OciUtils.configureIpv6SecurityRules. All operations use the
// VirtualNetworkClient (c.Vcn). Security rules are NOT stored in the DB --
// they are fetched and mutated live via the OCI API.
package oci

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// SecurityRuleDTO is the API request/response type for a single security rule.
type SecurityRuleDTO struct {
	ID       string  `json:"id"`
	Type     string  `json:"type"`      // "ingress" or "egress" (request); "入站" or "出站" (response)
	Protocol string  `json:"protocol"`  // "all", "tcp", "udp", "icmp", "6", "17", "1"
	Source   string  `json:"source"`    // CIDR: "0.0.0.0/0", "::/0", "10.0.0.0/16"
	Ports    string  `json:"ports"`     // "80", "8080-9090", ""
	TenantID *int64  `json:"tenantId"`
	ICMPType *string `json:"icmpType"`  // "8, 0" or nil
}

// ListSecurityRules lists all ingress or egress rules across ALL Security Lists
// in a compartment. Returns rules with type label "入站" (ingress) or
// "出站" (egress). Parity with Java SecurityRuleService.getSecurityRuleList.
func ListSecurityRules(ctx context.Context, c Clients, compartmentID, ruleType string) ([]SecurityRuleDTO, error) {
	lists, err := listAllSecurityLists(ctx, c, compartmentID)
	if err != nil {
		return nil, err
	}

	var out []SecurityRuleDTO
	for _, sl := range lists {
		if ruleType == "ingress" {
			for _, r := range sl.IngressSecurityRules {
				out = append(out, ingressRuleToDTO(r))
			}
		} else {
			for _, r := range sl.EgressSecurityRules {
				out = append(out, egressRuleToDTO(r))
			}
		}
	}
	return out, nil
}

// AddSecurityRule adds (or replaces) a rule on the FIRST Security List in the
// compartment. Before appending, removes all existing rules that match by
// protocol + CIDR + ports (duplicate detection). Then calls UpdateSecurityList.
// Parity with Java SecurityRuleService.addSecurityRule.
func AddSecurityRule(ctx context.Context, c Clients, compartmentID string, rule SecurityRuleDTO) error {
	lists, err := listAllSecurityLists(ctx, c, compartmentID)
	if err != nil {
		return err
	}
	if len(lists) == 0 {
		return fmt.Errorf("no security lists found in compartment %s", compartmentID)
	}

	sl := lists[0]

	if rule.Type == "ingress" {
		newRule := buildIngressRule(rule)
		var filtered []core.IngressSecurityRule
		for _, r := range sl.IngressSecurityRules {
			if !matchIngressRule(r, newRule) {
				filtered = append(filtered, r)
			}
		}
		filtered = append(filtered, newRule)
		sl.IngressSecurityRules = filtered
	} else {
		newRule := buildEgressRule(rule)
		var filtered []core.EgressSecurityRule
		for _, r := range sl.EgressSecurityRules {
			if !matchEgressRule(r, newRule) {
				filtered = append(filtered, r)
			}
		}
		filtered = append(filtered, newRule)
		sl.EgressSecurityRules = filtered
	}

	_, err = c.Vcn.UpdateSecurityList(ctx, core.UpdateSecurityListRequest{
		SecurityListId: sl.Id,
		UpdateSecurityListDetails: core.UpdateSecurityListDetails{
			IngressSecurityRules: sl.IngressSecurityRules,
			EgressSecurityRules:  sl.EgressSecurityRules,
		},
	})
	return err
}

// DeleteSecurityRule deletes a rule identified by composite ID
// "{tenantId}_{ruleIndex}_{type}". Uses global index across all Security Lists
// to locate the target rule, then removes all matching rules from that
// Security List. Parity with Java SecurityRuleService.deleteSecurityRule.
func DeleteSecurityRule(ctx context.Context, c Clients, compartmentID, compositeID string) error {
	parts := strings.Split(compositeID, "_")
	if len(parts) < 3 {
		return fmt.Errorf("invalid composite ID: %s", compositeID)
	}
	ruleIndex, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid rule index in composite ID: %s", parts[1])
	}
	ruleType := parts[2]

	lists, err := listAllSecurityLists(ctx, c, compartmentID)
	if err != nil {
		return err
	}

	if ruleType == "ingress" {
		return deleteIngressByGlobalIndex(ctx, c, lists, ruleIndex)
	}
	return deleteEgressByGlobalIndex(ctx, c, lists, ruleIndex)
}

// EnableAllForTenant adds "all protocol" ingress/egress + ICMP rules for a
// single tenant. Adds: ingress all/0.0.0.0/0, ingress all/::/0, ingress ICMP
// 0.0.0.0/0 (type 8 code 0), ingress ICMP 10.0.0.0/16, egress all/0.0.0.0/0,
// egress all/::/0. IPv6 failures are logged and skipped.
// Parity with Java SecurityRuleService.checkAndEnableRule.
func EnableAllForTenant(ctx context.Context, c Clients, compartmentID string) (addedIPv6 bool, err error) {
	// Ingress: all protocols from 0.0.0.0/0
	if err := AddSecurityRule(ctx, c, compartmentID, SecurityRuleDTO{
		Type:     "ingress",
		Protocol: "all",
		Source:   "0.0.0.0/0",
	}); err != nil {
		return false, fmt.Errorf("add ingress all/0.0.0.0/0: %w", err)
	}

	// Ingress: all protocols from ::/0 (IPv6 -- failures tolerated)
	if err := AddSecurityRule(ctx, c, compartmentID, SecurityRuleDTO{
		Type:     "ingress",
		Protocol: "all",
		Source:   "::/0",
	}); err != nil {
		// IPv6 failure is logged and skipped (tolerant)
		addedIPv6 = false
	} else {
		addedIPv6 = true
	}

	// Ingress: ICMP from 0.0.0.0/0 (type 8 code 0)
	icmpDefault := "8, 0"
	if err := AddSecurityRule(ctx, c, compartmentID, SecurityRuleDTO{
		Type:     "ingress",
		Protocol: "icmp",
		Source:   "0.0.0.0/0",
		ICMPType: &icmpDefault,
	}); err != nil {
		return addedIPv6, fmt.Errorf("add ingress ICMP/0.0.0.0/0: %w", err)
	}

	// Ingress: ICMP from 10.0.0.0/16 (type 8 code 0)
	if err := AddSecurityRule(ctx, c, compartmentID, SecurityRuleDTO{
		Type:     "ingress",
		Protocol: "icmp",
		Source:   "10.0.0.0/16",
		ICMPType: &icmpDefault,
	}); err != nil {
		return addedIPv6, fmt.Errorf("add ingress ICMP/10.0.0.0/16: %w", err)
	}

	// Egress: all protocols to 0.0.0.0/0
	if err := AddSecurityRule(ctx, c, compartmentID, SecurityRuleDTO{
		Type:     "egress",
		Protocol: "all",
		Source:   "0.0.0.0/0",
	}); err != nil {
		return addedIPv6, fmt.Errorf("add egress all/0.0.0.0/0: %w", err)
	}

	// Egress: all protocols to ::/0 (IPv6 -- failures tolerated)
	if err := AddSecurityRule(ctx, c, compartmentID, SecurityRuleDTO{
		Type:     "egress",
		Protocol: "all",
		Source:   "::/0",
	}); err != nil {
		// IPv6 failure is logged and skipped
	} else {
		addedIPv6 = addedIPv6 && true
	}

	return addedIPv6, nil
}

// EnableIPv6ForTenant adds IPv6 ingress/egress rules for a single tenant.
// Adds: ingress all/::/0, egress all/::/0.
// Parity with Java SecurityRuleService.singleIpv6Rule.
func EnableIPv6ForTenant(ctx context.Context, c Clients, compartmentID string) error {
	if err := AddSecurityRule(ctx, c, compartmentID, SecurityRuleDTO{
		Type:     "ingress",
		Protocol: "all",
		Source:   "::/0",
	}); err != nil {
		return fmt.Errorf("add ingress all/::/0: %w", err)
	}
	if err := AddSecurityRule(ctx, c, compartmentID, SecurityRuleDTO{
		Type:     "egress",
		Protocol: "all",
		Source:   "::/0",
	}); err != nil {
		return fmt.Errorf("add egress all/::/0: %w", err)
	}
	return nil
}

// ConfigureIPv6SecurityRules adds ICMPv6 + SSH + egress rules to the VCN's
// default Security List (obtained via GetVcn). Called during IPv6 enable on
// an instance. Adds ICMPv6 types 128,129,133-137 from ::/0, TCP port 22
// from ::/0, and egress all to ::/0. Parity with Java
// OciUtils.configureIpv6SecurityRules.
func ConfigureIPv6SecurityRules(ctx context.Context, c Clients, vcnID string) error {
	// Get VCN to find default Security List ID.
	vcnResp, err := c.Vcn.GetVcn(ctx, core.GetVcnRequest{
		VcnId: common.String(vcnID),
	})
	if err != nil {
		return fmt.Errorf("get vcn: %w", err)
	}
	if vcnResp.DefaultSecurityListId == nil {
		return fmt.Errorf("vcn %s has no default security list", vcnID)
	}
	slID := *vcnResp.DefaultSecurityListId

	// Get the Security List.
	slResp, err := c.Vcn.GetSecurityList(ctx, core.GetSecurityListRequest{
		SecurityListId: common.String(slID),
	})
	if err != nil {
		return fmt.Errorf("get security list: %w", err)
	}
	sl := slResp.SecurityList

	var newIngress []core.IngressSecurityRule
	var newEgress []core.EgressSecurityRule

	// Check ICMPv6 rules for types 128, 129, 133-137.
	icmpv6Types := []int{128, 129, 133, 134, 135, 136, 137}
	for _, t := range icmpv6Types {
		found := false
		for _, r := range sl.IngressSecurityRules {
			if r.Protocol != nil && *r.Protocol == "58" &&
				r.Source != nil && *r.Source == "::/0" &&
				r.IcmpOptions != nil && r.IcmpOptions.Type != nil && *r.IcmpOptions.Type == t {
				found = true
				break
			}
		}
		if !found {
			newIngress = append(newIngress, core.IngressSecurityRule{
				Protocol: common.String("58"),
				Source:   common.String("::/0"),
				IcmpOptions: &core.IcmpOptions{
					Type: intP(t),
					Code: intP(0),
				},
			})
		}
	}

	// Check IPv6 TCP SSH rule (protocol 6, source ::/0, port 22).
	sshFound := false
	for _, r := range sl.IngressSecurityRules {
		if r.Protocol != nil && *r.Protocol == "6" &&
			r.Source != nil && *r.Source == "::/0" &&
			r.TcpOptions != nil && r.TcpOptions.DestinationPortRange != nil &&
			r.TcpOptions.DestinationPortRange.Min != nil && *r.TcpOptions.DestinationPortRange.Min == 22 {
			sshFound = true
			break
		}
	}
	if !sshFound {
		newIngress = append(newIngress, core.IngressSecurityRule{
			Protocol: common.String("6"),
			Source:   common.String("::/0"),
			TcpOptions: &core.TcpOptions{
				DestinationPortRange: &core.PortRange{
					Min: intP(22),
					Max: intP(22),
				},
			},
		})
	}

	// Check IPv6 egress rule (protocol all, destination ::/0).
	egressFound := false
	for _, r := range sl.EgressSecurityRules {
		if r.Protocol != nil && *r.Protocol == "all" &&
			r.Destination != nil && *r.Destination == "::/0" {
			egressFound = true
			break
		}
	}
	if !egressFound {
		newEgress = append(newEgress, core.EgressSecurityRule{
			Protocol:    common.String("all"),
			Destination: common.String("::/0"),
		})
	}

	if len(newIngress) == 0 && len(newEgress) == 0 {
		return nil
	}

	// Merge with existing rules and update.
	allIngress := append(sl.IngressSecurityRules, newIngress...)
	allEgress := append(sl.EgressSecurityRules, newEgress...)

	_, err = c.Vcn.UpdateSecurityList(ctx, core.UpdateSecurityListRequest{
		SecurityListId: common.String(slID),
		UpdateSecurityListDetails: core.UpdateSecurityListDetails{
			IngressSecurityRules: allIngress,
			EgressSecurityRules:  allEgress,
		},
	})
	return err
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// listAllSecurityLists paginates through all Security Lists in a compartment.
func listAllSecurityLists(ctx context.Context, c Clients, compartmentID string) ([]core.SecurityList, error) {
	var all []core.SecurityList
	var page *string
	for {
		resp, err := c.Vcn.ListSecurityLists(ctx, core.ListSecurityListsRequest{
			CompartmentId: common.String(compartmentID),
			Page:          page,
		})
		if err != nil {
			return nil, fmt.Errorf("list security lists: %w", err)
		}
		all = append(all, resp.Items...)
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return all, nil
}

// ingressRuleToDTO converts an OCI IngressSecurityRule to the API DTO.
func ingressRuleToDTO(r core.IngressSecurityRule) SecurityRuleDTO {
	dto := SecurityRuleDTO{
		Type:   "入站", // ingress label
		Source: ptrStr(r.Source),
	}
	dto.Protocol = getProtocolName(ptrStr(r.Protocol))
	dto.Ports = formatPorts(r.TcpOptions, r.UdpOptions)
	dto.ICMPType = formatICMPType(r.IcmpOptions)
	return dto
}

// egressRuleToDTO converts an OCI EgressSecurityRule to the API DTO.
func egressRuleToDTO(r core.EgressSecurityRule) SecurityRuleDTO {
	dto := SecurityRuleDTO{
		Type:   "出站", // egress label
		Source: ptrStr(r.Destination),
	}
	dto.Protocol = getProtocolName(ptrStr(r.Protocol))
	dto.Ports = formatPorts(r.TcpOptions, r.UdpOptions)
	dto.ICMPType = formatICMPType(r.IcmpOptions)
	return dto
}

// getProtocolNumber converts protocol name to OCI protocol number string.
func getProtocolNumber(protocol string) string {
	switch strings.ToLower(protocol) {
	case "tcp":
		return "6"
	case "udp":
		return "17"
	case "icmp":
		return "1"
	case "icmpv6":
		return "58"
	case "all":
		return "all"
	default:
		return protocol // already a number or unknown
	}
}

// getProtocolName converts OCI protocol number string to a human-readable name.
func getProtocolName(protocol string) string {
	switch protocol {
	case "6":
		return "tcp"
	case "17":
		return "udp"
	case "1":
		return "icmp"
	case "58":
		return "icmpv6"
	case "all":
		return "all"
	default:
		return protocol
	}
}

// parsePorts parses the DTO ports string into optional min/max values.
// "80" -> (80,80), "8080-9090" -> (8080,9090), "ALL"/"" -> (nil,nil).
// Comma-separated ports use only the first (OCI limitation).
func parsePorts(ports string) (min, max *int) {
	ports = strings.TrimSpace(ports)
	if ports == "" || strings.EqualFold(ports, "ALL") {
		return nil, nil
	}
	// Comma-separated: only first port used.
	if idx := strings.Index(ports, ","); idx >= 0 {
		ports = ports[:idx]
	}
	// Range "8080-9090".
	if idx := strings.Index(ports, "-"); idx >= 0 {
		minVal, err1 := strconv.Atoi(strings.TrimSpace(ports[:idx]))
		maxVal, err2 := strconv.Atoi(strings.TrimSpace(ports[idx+1:]))
		if err1 != nil || err2 != nil {
			return nil, nil
		}
		return intP(minVal), intP(maxVal)
	}
	// Single port.
	val, err := strconv.Atoi(strings.TrimSpace(ports))
	if err != nil {
		return nil, nil
	}
	return intP(val), intP(val)
}

// formatPorts converts OCI TCP/UDP port options back to a display string.
func formatPorts(tcpOpts *core.TcpOptions, udpOpts *core.UdpOptions) string {
	var pr *core.PortRange
	if tcpOpts != nil && tcpOpts.DestinationPortRange != nil {
		pr = tcpOpts.DestinationPortRange
	} else if udpOpts != nil && udpOpts.DestinationPortRange != nil {
		pr = udpOpts.DestinationPortRange
	}
	if pr == nil {
		return ""
	}
	lo, hi := 0, 0
	if pr.Min != nil {
		lo = *pr.Min
	}
	if pr.Max != nil {
		hi = *pr.Max
	}
	if lo == hi {
		return strconv.Itoa(lo)
	}
	return fmt.Sprintf("%d-%d", lo, hi)
}

// formatICMPType converts OCI IcmpOptions to "type, code" string pointer.
func formatICMPType(opts *core.IcmpOptions) *string {
	if opts == nil {
		return nil
	}
	t, c := 0, 0
	if opts.Type != nil {
		t = *opts.Type
	}
	if opts.Code != nil {
		c = *opts.Code
	}
	s := fmt.Sprintf("%d, %d", t, c)
	return &s
}

// parseICMPType parses "type, code" or "type" string into (type, code).
// Defaults to (8, 0) for echo request.
func parseICMPType(icmpType *string) (typ, code int) {
	if icmpType == nil || *icmpType == "" {
		return 8, 0
	}
	parts := strings.Split(*icmpType, ",")
	if len(parts) >= 1 {
		if t, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
			typ = t
		}
	}
	if len(parts) >= 2 {
		if c, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
			code = c
		}
	}
	return typ, code
}

// buildIngressRule converts a SecurityRuleDTO into core.IngressSecurityRule.
func buildIngressRule(rule SecurityRuleDTO) core.IngressSecurityRule {
	protocol := getProtocolNumber(rule.Protocol)
	r := core.IngressSecurityRule{
		Protocol: common.String(protocol),
		Source:   common.String(rule.Source),
	}
	if protocol == "all" {
		return r
	}
	min, max := parsePorts(rule.Ports)
	switch protocol {
	case "6": // TCP
		if min != nil || max != nil {
			r.TcpOptions = &core.TcpOptions{
				DestinationPortRange: &core.PortRange{Min: min, Max: max},
			}
		}
	case "17": // UDP
		if min != nil || max != nil {
			r.UdpOptions = &core.UdpOptions{
				DestinationPortRange: &core.PortRange{Min: min, Max: max},
			}
		}
	case "1": // ICMP
		icmpType, icmpCode := parseICMPType(rule.ICMPType)
		r.IcmpOptions = &core.IcmpOptions{
			Type: intP(icmpType),
			Code: intP(icmpCode),
		}
	}
	return r
}

// buildEgressRule converts a SecurityRuleDTO into core.EgressSecurityRule.
func buildEgressRule(rule SecurityRuleDTO) core.EgressSecurityRule {
	protocol := getProtocolNumber(rule.Protocol)
	r := core.EgressSecurityRule{
		Protocol:    common.String(protocol),
		Destination: common.String(rule.Source), // DTO source maps to OCI destination for egress
	}
	if protocol == "all" {
		return r
	}
	min, max := parsePorts(rule.Ports)
	switch protocol {
	case "6": // TCP
		if min != nil || max != nil {
			r.TcpOptions = &core.TcpOptions{
				DestinationPortRange: &core.PortRange{Min: min, Max: max},
			}
		}
	case "17": // UDP
		if min != nil || max != nil {
			r.UdpOptions = &core.UdpOptions{
				DestinationPortRange: &core.PortRange{Min: min, Max: max},
			}
		}
	case "1": // ICMP
		icmpType, icmpCode := parseICMPType(rule.ICMPType)
		r.IcmpOptions = &core.IcmpOptions{
			Type: intP(icmpType),
			Code: intP(icmpCode),
		}
	}
	return r
}

// matchIngressRule checks if two ingress rules match by protocol + CIDR + ports.
func matchIngressRule(a, b core.IngressSecurityRule) bool {
	if !strPtrEqual(a.Protocol, b.Protocol) {
		return false
	}
	if !strPtrEqual(a.Source, b.Source) {
		return false
	}
	proto := ptrStr(a.Protocol)
	switch proto {
	case "6":
		return matchTcpOptions(a.TcpOptions, b.TcpOptions)
	case "17":
		return matchUdpOptions(a.UdpOptions, b.UdpOptions)
	case "1", "58":
		return matchIcmpOptions(a.IcmpOptions, b.IcmpOptions)
	default:
		return true
	}
}

// matchEgressRule checks if two egress rules match by protocol + CIDR + ports.
func matchEgressRule(a, b core.EgressSecurityRule) bool {
	if !strPtrEqual(a.Protocol, b.Protocol) {
		return false
	}
	if !strPtrEqual(a.Destination, b.Destination) {
		return false
	}
	proto := ptrStr(a.Protocol)
	switch proto {
	case "6":
		return matchTcpOptions(a.TcpOptions, b.TcpOptions)
	case "17":
		return matchUdpOptions(a.UdpOptions, b.UdpOptions)
	case "1", "58":
		return matchIcmpOptions(a.IcmpOptions, b.IcmpOptions)
	default:
		return true
	}
}

func matchTcpOptions(a, b *core.TcpOptions) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return portRangeEqual(a.DestinationPortRange, b.DestinationPortRange) &&
		portRangeEqual(a.SourcePortRange, b.SourcePortRange)
}

func matchUdpOptions(a, b *core.UdpOptions) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return portRangeEqual(a.DestinationPortRange, b.DestinationPortRange) &&
		portRangeEqual(a.SourcePortRange, b.SourcePortRange)
}

func matchIcmpOptions(a, b *core.IcmpOptions) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return intPtrEqual(a.Type, b.Type) && intPtrEqual(a.Code, b.Code)
}

func portRangeEqual(a, b *core.PortRange) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return intPtrEqual(a.Min, b.Min) && intPtrEqual(a.Max, b.Max)
}

// deleteIngressByGlobalIndex locates the Security List and local index for the
// given global index across all Security Lists' ingress rules, then removes all
// matching rules from that Security List and updates it.
func deleteIngressByGlobalIndex(ctx context.Context, c Clients, lists []core.SecurityList, globalIdx int) error {
	cursor := 0
	for _, sl := range lists {
		rules := sl.IngressSecurityRules
		if cursor+len(rules) > globalIdx {
			localIdx := globalIdx - cursor
			target := rules[localIdx]

			var filtered []core.IngressSecurityRule
			for _, r := range rules {
				if !matchIngressRule(r, target) {
					filtered = append(filtered, r)
				}
			}

			_, err := c.Vcn.UpdateSecurityList(ctx, core.UpdateSecurityListRequest{
				SecurityListId: sl.Id,
				UpdateSecurityListDetails: core.UpdateSecurityListDetails{
					IngressSecurityRules: filtered,
					EgressSecurityRules:  sl.EgressSecurityRules,
				},
			})
			return err
		}
		cursor += len(rules)
	}
	return fmt.Errorf("ingress rule index %d out of range (total %d)", globalIdx, cursor)
}

// deleteEgressByGlobalIndex locates the Security List and local index for the
// given global index across all Security Lists' egress rules, then removes all
// matching rules from that Security List and updates it.
func deleteEgressByGlobalIndex(ctx context.Context, c Clients, lists []core.SecurityList, globalIdx int) error {
	cursor := 0
	for _, sl := range lists {
		rules := sl.EgressSecurityRules
		if cursor+len(rules) > globalIdx {
			localIdx := globalIdx - cursor
			target := rules[localIdx]

			var filtered []core.EgressSecurityRule
			for _, r := range rules {
				if !matchEgressRule(r, target) {
					filtered = append(filtered, r)
				}
			}

			_, err := c.Vcn.UpdateSecurityList(ctx, core.UpdateSecurityListRequest{
				SecurityListId: sl.Id,
				UpdateSecurityListDetails: core.UpdateSecurityListDetails{
					IngressSecurityRules: sl.IngressSecurityRules,
					EgressSecurityRules:  filtered,
				},
			})
			return err
		}
		cursor += len(rules)
	}
	return fmt.Errorf("egress rule index %d out of range (total %d)", globalIdx, cursor)
}

// --- pointer helpers ---

func intP(v int) *int { return &v }

func strPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func intPtrEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
