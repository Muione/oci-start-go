import client from './client'
import type { Instance, ShapeInfo, ImageInfo } from '@/types/api'

// ─── Instance CRUD ────────────────────────────────────────────────────

export interface InstanceListResponse {
  items: Instance[]
  total: number
}

export function instanceList(params: { limit?: number; offset?: number } = {}): Promise<InstanceListResponse> {
  return client.get<unknown, InstanceListResponse>('/instances/list', { params })
}

export function instanceGet(id: number): Promise<Instance> {
  return client.get<unknown, Instance>(`/instances/${id}`)
}

// ─── Lifecycle ────────────────────────────────────────────────────────

export function instanceStart(id: number): Promise<unknown> {
  return client.post<unknown, unknown>(`/instances/${id}/start`)
}

export function instanceStop(id: number): Promise<unknown> {
  return client.post<unknown, unknown>(`/instances/${id}/stop`)
}

export function instanceRestart(id: number): Promise<unknown> {
  return client.post<unknown, unknown>(`/instances/${id}/restart`)
}

export function instanceTerminate(id: number, preserveBootVolume = false): Promise<unknown> {
  return client.post<unknown, unknown>(`/instances/${id}/terminate`, { preserveBootVolume })
}

// ─── Modify / Config ──────────────────────────────────────────────────

export interface ModifyInstancePayload {
  shape?: string
  ocpus?: number
  memoryInGbs?: number
  displayName?: string
}

export function instanceModify(id: number, data: ModifyInstancePayload): Promise<unknown> {
  return client.post<unknown, unknown>(`/instances/${id}/modify`, data)
}

export function instanceChangeIP(id: number): Promise<{ oldIp: string; newIp: string }> {
  return client.post<unknown, { oldIp: string; newIp: string }>(`/instances/${id}/change-ip`)
}

export function instanceIpv6(id: number): Promise<{ ipv6Address?: string }> {
  return client.post<unknown, { ipv6Address?: string }>(`/instances/${id}/enable-ipv6`)
}

// ─── SSH Config ───────────────────────────────────────────────────────

export interface SshConfig {
  username: string
  port: number
  password: string
}

export function instanceGetSshConfig(id: number): Promise<SshConfig> {
  return client.get<unknown, SshConfig>(`/instances/${id}/ssh-config`)
}

export function instanceSaveSshConfig(id: number, data: SshConfig): Promise<unknown> {
  return client.post<unknown, unknown>(`/instances/${id}/ssh-config`, data)
}

// ─── Remark ───────────────────────────────────────────────────────────

export function instanceUpdateRemark(id: number, remark: string): Promise<unknown> {
  return client.post<unknown, unknown>(`/instances/${id}/remark`, { remark })
}

// ─── Disk ─────────────────────────────────────────────────────────────

export function instanceUpdateVpu(id: number, vpusPerGB: number): Promise<unknown> {
  return client.post<unknown, unknown>(`/instances/${id}/vpu`, { vpusPerGB })
}

export function instanceResizeBootVolume(id: number, sizeInGBs: number): Promise<unknown> {
  return client.post<unknown, unknown>(`/instances/${id}/resize`, { sizeInGBs })
}

// ─── Console ──────────────────────────────────────────────────────────

export interface ConsoleConnection {
  id: string
  instanceId: string
  connectionType: string
  consoleUrl: string
  state: string
  createdAt: string
}

export function instanceConsoleConnections(id: number): Promise<ConsoleConnection[]> {
  return client.get<unknown, ConsoleConnection[]>(`/instances/${id}/console-connections`)
}

export function instanceDeleteConsoleConnection(id: number, connId: string): Promise<unknown> {
  return client.delete<unknown, unknown>(`/instances/${id}/console-connections/${connId}`)
}

// ─── Traffic ──────────────────────────────────────────────────────────

export interface InstanceTraffic {
  instanceId: string
  instanceName: string
  publicIp: string
  vnicCount: number
  egressGB: number
  egressBytes: number
  ingressBytes: number
  statsDate: string
  region: string
}

export function instanceTraffic(tenantId: number): Promise<InstanceTraffic[]> {
  return client.get<unknown, InstanceTraffic[]>('/instances/traffic', { params: { tenantId } })
}

// ─── Shapes & Images ──────────────────────────────────────────────────

export function instanceShapes(params: { tenantId: number; architecture?: string }): Promise<ShapeInfo[]> {
  return client.get<unknown, ShapeInfo[]>('/oci/shapes', { params })
}

export function instanceImages(params: { tenantId: number; architecture?: string; shape?: string }): Promise<ImageInfo[]> {
  return client.get<unknown, ImageInfo[]>('/oci/images', { params })
}

// ─── Export ───────────────────────────────────────────────────────────

export function instanceExportUrl(tenantId?: number): string {
  const base = '/instances/export'
  return tenantId ? `${base}?tenantId=${tenantId}` : base
}
