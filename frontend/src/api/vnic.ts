import client from './client'
import type {
  VnicLoadData, BatchVnicResult, NetworkConfigResult, IpSwitchResult,
  SecurityRule, VcnInfo, NatGatewayInfo, RouteTableInfo, SubnetInfo,
} from '@/types/api'

// ─── VNIC Data ─────────────────────────────────────────────────────────

export function vnicLoadData(instanceId: string): Promise<VnicLoadData> {
  return client.get<unknown, VnicLoadData>('/oci/vnic/loadData', { params: { instanceId } })
}

export function vnicRefresh(instanceId: string): Promise<VnicLoadData> {
  return client.get<unknown, VnicLoadData>('/oci/vnic/refresh', { params: { instanceId } })
}

// ─── Batch Create ──────────────────────────────────────────────────────

export function vnicBatchCreate(
  instanceId: string, subnetId: string, vnicCount: number, ipv6CountPerVnic = 0,
): Promise<BatchVnicResult> {
  return client.post<unknown, BatchVnicResult>('/oci/vnic/create', {
    instanceId, subnetId, vnicCount, ipv6CountPerVnic,
  })
}

// ─── Delete ────────────────────────────────────────────────────────────

export function vnicDelete(instanceId: string, vnicId: string): Promise<unknown> {
  return client.post<unknown, unknown>('/oci/vnic/delete', { instanceId, vnicId })
}

export function vnicDeleteAllSecondary(instanceId: string): Promise<Record<string, boolean>> {
  return client.post<unknown, Record<string, boolean>>('/oci/vnic/deleteAllSecondary', { instanceId })
}

// ─── IPv6 ──────────────────────────────────────────────────────────────

export function vnicCreateIpv6(vnicId: string, ipv6Count: number, instanceId: string): Promise<unknown> {
  return client.post<unknown, unknown>('/oci/vnic/createIpv6', { vnicId, ipv6Count, instanceId })
}

export function vnicDeleteIpv6(ipv6Address: string, vnicId: string, instanceId: string): Promise<unknown> {
  return client.post<unknown, unknown>('/oci/vnic/deleteIpv6', { ipv6Address, vnicId, instanceId })
}

// ─── Security Rules ────────────────────────────────────────────────────

export function vnicSecurityRules(tenantId: number, type: 'ingress' | 'egress'): Promise<SecurityRule[]> {
  return client.get<unknown, SecurityRule[]>('/tenants/security-rules', { params: { tenantId, type } })
}

export function vnicAddSecurityRule(data: {
  tenantId: number; type: string; protocol: string; source: string; ports?: string; icmpType?: string;
}): Promise<unknown> {
  return client.post<unknown, unknown>('/tenants/security-rules', data)
}

export function vnicDeleteSecurityRule(ruleId: string): Promise<unknown> {
  return client.delete<unknown, unknown>(`/tenants/security-rules/${ruleId}`)
}

export function vnicEnableAll(tenantId: number): Promise<unknown> {
  return client.post<unknown, unknown>('/tenants/enableAll', { tenantId })
}

export function vnicEnableIpv6(tenantId: number): Promise<unknown> {
  return client.post<unknown, unknown>('/tenants/enableIpv6', { tenantId })
}

// ─── VCN ───────────────────────────────────────────────────────────────

export function vcnList(tenantId: number, compartmentId: string): Promise<VcnInfo[]> {
  return client.get<unknown, VcnInfo[]>('/oci/vcn/list', { params: { tenantId, compartmentId } })
}

// ─── NAT Gateway ───────────────────────────────────────────────────────

export function natList(tenantId: number, compartmentId: string, vcnId: string): Promise<NatGatewayInfo[]> {
  return client.get<unknown, NatGatewayInfo[]>('/oci/nat/list', { params: { tenantId, compartmentId, vcnId } })
}

export function natCreate(tenantId: number, compartmentId: string, vcnId: string, displayName: string): Promise<unknown> {
  return client.post<unknown, unknown>('/oci/nat/create', { tenantId, compartmentId, vcnId, displayName })
}

export function natDelete(tenantId: number, natGatewayId: string): Promise<unknown> {
  return client.delete<unknown, unknown>('/oci/nat/delete', { params: { tenantId, natGatewayId } })
}

// ─── Route Table ───────────────────────────────────────────────────────

export function routeTableList(tenantId: number, compartmentId: string, vcnId: string): Promise<RouteTableInfo[]> {
  return client.get<unknown, RouteTableInfo[]>('/oci/route-table/list', { params: { tenantId, compartmentId, vcnId } })
}

export function routeTableCreate(tenantId: number, compartmentId: string, vcnId: string, displayName: string): Promise<unknown> {
  return client.post<unknown, unknown>('/oci/route-table/create', { tenantId, compartmentId, vcnId, displayName })
}

export function routeTableDelete(tenantId: number, routeTableId: string): Promise<unknown> {
  return client.delete<unknown, unknown>('/oci/route-table/delete', { params: { tenantId, routeTableId } })
}

export function routeTableReset(tenantId: number, instanceId: string, compartmentId: string): Promise<unknown> {
  return client.post<unknown, unknown>('/oci/route-table/reset', { tenantId, instanceId, compartmentId })
}

// ─── IP Switch / Reassign ──────────────────────────────────────────────

export function vnicChangeSpecIp(instanceId: string, vnicId: string, cidrRanges: string[]): Promise<IpSwitchResult> {
  return client.post<unknown, IpSwitchResult>('/oci/vnic/changeSpecIp', { instanceId, vnicId, cidrRanges })
}

export function vnicReassignIp(instanceId: string, instanceDbId: number): Promise<{ publicIp: string }> {
  return client.post<unknown, { publicIp: string }>('/oci/vnic/reassignIp', { instanceId, instanceDbId })
}

// ─── Load Balancer / Network ───────────────────────────────────────────

export function vnicConfigureLB(instanceId: string): Promise<NetworkConfigResult> {
  return client.post<unknown, NetworkConfigResult>('/oci/vnic/network/configureLoadBalancer', { instanceId })
}

export function vnicRestoreNetwork(instanceId: string): Promise<{ success: boolean; message: string }> {
  return client.post<unknown, { success: boolean; message: string }>('/oci/vnic/network/restoreNetwork', { instanceId })
}

// ─── VCN IPv6 Config ──────────────────────────────────────────────────

export function vcnConfigureIpv6(tenantId: number, vcnId: string): Promise<unknown> {
  return client.post<unknown, unknown>('/oci/vcn/configureIpv6', { tenantId, vcnId })
}

// ─── Subnets ───────────────────────────────────────────────────────────

export function subnetList(tenantId: number, compartmentId: string): Promise<SubnetInfo[]> {
  return client.get<unknown, SubnetInfo[]>('/oci/subnet/list', { params: { tenantId, compartmentId } })
}
