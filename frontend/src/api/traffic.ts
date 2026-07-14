import client from './client'
import type { TrafficStats, TrafficAlert, AutoShutdownConfig } from '@/types/api'

// ─── Traffic Stats ─────────────────────────────────────────────────────

export function trafficStats(tenantId: number, instanceId?: string): Promise<TrafficStats[]> {
  return client.get<unknown, TrafficStats[]>('/oci/traffic', {
    params: { tenantId, ...(instanceId ? { instanceId } : {}) },
  })
}

// ─── Alerts ────────────────────────────────────────────────────────────

export function trafficAlerts(tenantId: number): Promise<TrafficAlert[]> {
  return client.get<unknown, TrafficAlert[]>('/oci/traffic/alerts', { params: { tenantId } })
}

// ─── Auto-Shutdown Config ──────────────────────────────────────────────

export function getAutoShutdownConfig(): Promise<AutoShutdownConfig> {
  return client.get<unknown, AutoShutdownConfig>('/oci/traffic/auto-shutdown')
}

export function updateAutoShutdownConfig(config: AutoShutdownConfig): Promise<unknown> {
  return client.post<unknown, unknown>('/oci/traffic/auto-shutdown', config)
}
