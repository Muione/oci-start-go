import client from './client'
import type { BootVolumeBackup } from '@/types/api'

// ─── Backup CRUD ──────────────────────────────────────────────────────

export function backupList(tenantId: number): Promise<BootVolumeBackup[]> {
  return client.get<unknown, BootVolumeBackup[]>('/backup/list', { params: { tenantId } })
}

export function backupCreate(data: {
  tenantId: number
  instanceId: string
  displayName: string
}): Promise<unknown> {
  return client.post<unknown, unknown>('/backup/create', data)
}

export function backupDelete(id: string): Promise<unknown> {
  return client.delete<unknown, unknown>('/backup/delete', { params: { id } })
}

export function backupRestore(data: {
  tenantId: number
  backupId: string
  instanceId?: string
}): Promise<unknown> {
  return client.post<unknown, unknown>('/backup/restore', data)
}

export function backupCopy(data: {
  tenantId: number
  backupId: string
  targetRegion: string
  displayName?: string
}): Promise<unknown> {
  return client.post<unknown, unknown>('/backup/copy', data)
}
