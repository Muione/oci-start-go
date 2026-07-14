import client from './client'
import type { SystemConfig, Proxy } from '@/types/api'

// ─── System Config ────────────────────────────────────────────────────

export function getSystemConfig(): Promise<SystemConfig> {
  return client.get<unknown, SystemConfig>('/system/config')
}

export function saveSystemConfigField(key: string, value: string): Promise<unknown> {
  return client.post<unknown, unknown>('/system/config/save', { key, value })
}

export function getSystemVersion(): Promise<{ version: string }> {
  return client.get<unknown, { version: string }>('/api/version')
}

// ─── System Settings (structured) ─────────────────────────────────────

export interface SystemSettings {
  security?: {
    turnstile?: { enabled: boolean; siteKey: string; secretKey: string }
  }
  oauth?: {
    github?: { enabled: boolean; clientId: string; clientSecret: string; redirectUri: string }
    google?: { enabled: boolean; clientId: string; clientSecret: string; redirectUri: string }
  }
}

export function getSystemSettings(): Promise<SystemSettings> {
  return client.get<unknown, SystemSettings>('/system/settings')
}

export function updateSystemSettings(data: SystemSettings): Promise<unknown> {
  return client.put<unknown, unknown>('/system/settings', data)
}

// ─── Notification ─────────────────────────────────────────────────────

export function testNotification(channel: string): Promise<unknown> {
  return client.post<unknown, unknown>('/system/notification/test', { channel })
}

export interface NotificationHistoryItem {
  time: string
  channel: string
  message: string
  success: boolean
}

export function getNotificationHistory(params?: { channel?: string }): Promise<{ history: NotificationHistoryItem[] }> {
  return client.get<unknown, { history: NotificationHistoryItem[] }>('/system/notification/history', { params })
}

// ─── SSL Certificates ─────────────────────────────────────────────────

export interface SslCertInfo {
  id: number
  domain: string
  certificateType: string
  email: string
  certificateStatus: string
  issueDate: string
  expireDate: string
  autoRenew: number
}

export function sslList(): Promise<SslCertInfo[]> {
  return client.get<unknown, SslCertInfo[]>('/ssl/list')
}

export function sslIssue(data: { domain: string; email: string }): Promise<unknown> {
  return client.post<unknown, unknown>('/ssl/issue', data)
}

// ─── Outbound Proxy ───────────────────────────────────────────────────

export interface OutboundProxyConfig {
  type: string
  host: string
  port: number
  username: string
  password: string
  enabled: boolean
}

export function getOutboundProxy(): Promise<OutboundProxyConfig> {
  return client.get<unknown, OutboundProxyConfig>('/system/proxy')
}

export function updateOutboundProxy(data: OutboundProxyConfig): Promise<unknown> {
  return client.put<unknown, unknown>('/system/proxy', data)
}

export function testOutboundProxy(data: Partial<OutboundProxyConfig>): Promise<{ reachable: boolean; message: string }> {
  return client.post<unknown, { reachable: boolean; message: string }>('/system/proxy/test', data)
}

// ─── VPN Proxy CRUD ───────────────────────────────────────────────────

export function proxyList(): Promise<Proxy[]> {
  return client.get<unknown, Proxy[]>('/proxies/list')
}

export function proxySave(data: FormData): Promise<unknown> {
  return client.post<unknown, unknown>('/proxies/save', data, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

export function proxyDelete(id: number): Promise<unknown> {
  return client.get<unknown, unknown>('/proxies/delete', { params: { id } })
}

// ─── Migration ────────────────────────────────────────────────────────

export interface MigrationResult {
  totalLines?: number
  inserted?: number
  skipped?: number
  skippedDups?: number
  skippedUser?: number
  errors?: number
  tablesFound?: Record<string, number>
  message?: string
}

export function migrationImportPlain(file: File): Promise<MigrationResult> {
  const fd = new FormData()
  fd.append('file', file)
  return client.post<unknown, MigrationResult>('/migration/import', fd)
}

export function migrationImportEncrypted(file: File, masterKey: string): Promise<MigrationResult> {
  const fd = new FormData()
  fd.append('file', file)
  fd.append('masterKey', masterKey)
  return client.post<unknown, MigrationResult>('/migration/import-encrypted', fd)
}

// ─── Security ─────────────────────────────────────────────────────────

export interface MfaStatus {
  enabled: boolean
  configured: boolean
}

export function getMfaStatus(): Promise<MfaStatus> {
  return client.get<unknown, MfaStatus>('/api/mfa/status')
}

export interface TotpSetup {
  secret: string
  otpauthUrl: string
  qrCodeBase64: string
}

export function mfaTotpSetup(): Promise<TotpSetup> {
  return client.post<unknown, TotpSetup>('/api/mfa/totp/setup')
}

export function mfaTotpVerify(code: string): Promise<unknown> {
  return client.post<unknown, unknown>('/api/mfa/totp/verify', { code })
}

export function mfaDisable(): Promise<unknown> {
  return client.post<unknown, unknown>('/api/mfa/disable')
}

export interface LoginHistoryItem {
  created_at: string
  ip_address: string
  success: boolean
}

export function getLoginHistory(limit = 20): Promise<{ items: LoginHistoryItem[] }> {
  return client.get<unknown, { items: LoginHistoryItem[] }>('/api/security/login-history', { params: { limit } })
}

export interface SessionInfo {
  id: string
  ip_address: string
  user_agent: string
  last_active_at: string
  created_at: string
  is_current: boolean
}

export function getSessions(): Promise<{ sessions: SessionInfo[] }> {
  return client.get<unknown, { sessions: SessionInfo[] }>('/api/security/sessions')
}

export function deleteSession(id: string): Promise<unknown> {
  return client.delete<unknown, unknown>(`/api/security/sessions/${id}`)
}

export function logoutAllSessions(): Promise<unknown> {
  return client.post<unknown, unknown>('/api/security/logout-all')
}

export function changePassword(data: { currentPassword: string; newPassword: string }): Promise<unknown> {
  return client.post<unknown, unknown>('/api/change-password', data)
}
