import client from './client'
import type { BootTask, EngineStatus } from '@/types/api'

// ─── Tasks ─────────────────────────────────────────────────────────────

export function bootList(): Promise<BootTask[]> {
  return client.get<unknown, BootTask[]>('/boot/list')
}

export function bootSave(data: Partial<BootTask>): Promise<unknown> {
  return client.post<unknown, unknown>('/boot/save', data)
}

export function bootDelete(bootId: string): Promise<unknown> {
  return client.get<unknown, unknown>('/boot/delete', { params: { bootId } })
}

export function bootToggle(bootId: string, enable: boolean): Promise<unknown> {
  return client.get<unknown, unknown>('/boot/toggle', { params: { bootId, enable: enable ? 1 : 0 } })
}

// ─── Engine Status ─────────────────────────────────────────────────────

export function bootSystemStatus(): Promise<EngineStatus> {
  return client.get<unknown, EngineStatus>('/boot/systemStatus')
}

// ─── Tenants (for boot task creation) ──────────────────────────────────

export interface BootTenant {
  id: number
  name: string
  region: string
  tenancy: string
}

export function bootTenants(): Promise<BootTenant[]> {
  return client.get<unknown, BootTenant[]>('/boot/tenants')
}
