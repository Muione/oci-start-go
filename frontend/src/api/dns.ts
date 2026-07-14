import client from './client'
import type { CfZone, CfRecord, EoRecord } from '@/types/api'

// ─── Cloudflare ────────────────────────────────────────────────────────

export function cfListZones(): Promise<CfZone[]> {
  return client.get<unknown, CfZone[]>('/dns/cloudflare/zones')
}

export interface CfListRecordsParams {
  page?: number
  perPage?: number
  type?: string
}

export interface CfListRecordsResponse {
  records: CfRecord[]
  totalPages: number
  totalCount: number
}

export function cfListRecords(zoneId: string, params?: CfListRecordsParams): Promise<CfListRecordsResponse> {
  return client.get<unknown, CfListRecordsResponse>(`/dns/cloudflare/zones/${zoneId}/records`, { params })
}

export interface CfCreateRecordPayload {
  type: string
  name: string
  content: string
  ttl?: number
  proxied?: boolean
}

export function cfCreateRecord(zoneId: string, data: CfCreateRecordPayload): Promise<unknown> {
  return client.post<unknown, unknown>(`/dns/cloudflare/zones/${zoneId}/records`, data)
}

export function cfDeleteRecord(zoneId: string, recordId: string): Promise<unknown> {
  return client.delete<unknown, unknown>(`/dns/cloudflare/zones/${zoneId}/records/${recordId}`)
}

// ─── EdgeOne ───────────────────────────────────────────────────────────

export function eoListRecords(): Promise<EoRecord[]> {
  return client.get<unknown, EoRecord[]>('/dns/edgeone/records')
}

export interface EoCreateRecordPayload {
  type: string
  name: string
  content: string
  ttl?: number
  priority?: number
}

export function eoCreateRecord(data: EoCreateRecordPayload): Promise<unknown> {
  return client.post<unknown, unknown>('/dns/edgeone/records', data)
}

export function eoDeleteRecord(recordId: string): Promise<unknown> {
  return client.delete<unknown, unknown>(`/dns/edgeone/records/${recordId}`)
}
