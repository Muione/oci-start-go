import client from './client'
import type { Instance } from '@/types/api'

// ─── Tenant CRUD ─────────────────────────────────────────────────────

export interface Tenant {
  id: number
  tenantId?: string
  userName: string
  tenancy: string
  region: string
  regionName: string
  fingerprint: string
  apiSynced: boolean
  tenancyName?: string
  tenancyDes?: string
  accountType?: string
  cloudType?: number
  emailAddress?: string
  emailEnable?: boolean
  isActive?: boolean
  isHomeRegion?: boolean
  createdAt?: string
  enableIcmp?: boolean
  enableAllProtocol?: boolean
  parenId?: number
  regionEn?: string
  idStr?: string
  transferStatus?: number
  transferAmount?: string
  instanceCount?: number
  planType?: string
  accountCost?: string
  hasBootTask?: boolean
  hasChildren?: boolean
  activeDays?: string
}

export interface CheckResult {
  alive: boolean
  error?: string
  tenantId?: number
  userName?: string
  tenancyName?: string
}

export function tenantList(): Promise<Tenant[]> {
  return client.get<unknown, Tenant[]>('/tenants/listAll')
}

export function tenantGet(id: number): Promise<Tenant> {
  return client.get<unknown, Tenant>(`/tenants/${id}`)
}

export function tenantSave(fd: FormData): Promise<unknown> {
  return client.post<unknown, unknown>('/tenants/save', fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

export function tenantUpdate(id: number, data: Partial<Tenant>): Promise<unknown> {
  return client.put<unknown, unknown>(`/tenants/${id}`, data)
}

export function tenantDelete(id: number): Promise<unknown> {
  return client.get<unknown, unknown>('/tenants/deleteApi', { params: { tenantId: id } })
}

export function tenantSyncOci(id: number): Promise<unknown> {
  return client.get<unknown, unknown>('/tenants/syncOci', { params: { tenantId: id } })
}

export function tenantCheck(id: number): Promise<CheckResult> {
  return client.get<unknown, CheckResult>(`/tenants/${id}/check`)
}

export function tenantCheckBatch(ids: number[]): Promise<CheckResult[]> {
  return client.post<unknown, CheckResult[]>('/tenants/check-batch', ids)
}

export function tenantExport(id: number): Promise<Blob> {
  return client.get<unknown, Blob>(`/tenants/${id}/export`, { responseType: 'blob' })
}

export function tenantUpdateDetail(id: number): Promise<unknown> {
  return client.post<unknown, unknown>(`/tenants/${id}/update-detail`)
}

export function tenantSaveAccountCost(id: number, cost: string): Promise<unknown> {
  return client.post<unknown, unknown>(`/tenants/${id}/account-cost`, { cost })
}

// ─── Instances ───────────────────────────────────────────────────────

export function tenantInstances(id: number): Promise<Instance[]> {
  return client.get<unknown, Instance[]>(`/tenants/${id}/instances`)
}

// ─── Email ───────────────────────────────────────────────────────────

export interface EmailConfig {
  id?: number
  domainName: string
  smtpHost: string
  smtpPort: string
  smtpUsername: string
  smtpPassword: string
  senderEmail: string
  active: boolean
}

export function tenantEmailGet(id: number): Promise<EmailConfig> {
  return client.get<unknown, EmailConfig>(`/tenants/${id}/email`)
}

export function tenantEmailSave(id: number, data: EmailConfig): Promise<unknown> {
  return client.post<unknown, unknown>(`/tenants/${id}/email`, data)
}

export function tenantEmailDelete(id: number): Promise<unknown> {
  return client.delete<unknown, unknown>(`/tenants/${id}/email`)
}

export function tenantEmailEnable(tenantId: number, domainName: string): Promise<unknown> {
  return client.post<unknown, unknown>('/api/email/enable', { tenantId, domainName })
}

export function tenantEmailDisable(emailConfigId: number): Promise<unknown> {
  return client.post<unknown, unknown>('/api/email/disable', { tenantEmailConfigId: emailConfigId })
}

// ─── Social ──────────────────────────────────────────────────────────

export interface SocialConfig {
  id: string
  socialType: string
  socialTypeStr: string
  clientId: string
  clientSecret?: string
  redirectUrl: string
  loginUrl: string
  socialStatus: string
}

export function tenantSocialList(id: number): Promise<SocialConfig[]> {
  return client.get<unknown, SocialConfig[]>(`/tenants/${id}/social`)
}

export function tenantSocialSave(id: number, data: Partial<SocialConfig>): Promise<unknown> {
  return client.post<unknown, unknown>(`/tenants/${id}/social`, data)
}

export function tenantSocialToggle(id: number, socialId: string): Promise<unknown> {
  return client.put<unknown, unknown>(`/tenants/${id}/social/${socialId}/toggle`)
}

export function tenantSocialDelete(id: number, socialId: string): Promise<unknown> {
  return client.delete<unknown, unknown>(`/tenants/${id}/social/${socialId}`)
}

// ─── IAM Users ───────────────────────────────────────────────────────

export interface IamUser {
  name: string
  ocid: string
  description: string
  lifecycleState: string
  email: string
}

export interface IamGroup {
  name: string
  ocid: string
}

export interface PasswordPolicy {
  isPasswordExpiryEnabled: boolean
  passwordExpiryDays: number
}

export interface MfaStatus {
  totpEnabled: boolean
  emailEnabled: boolean
  smsEnabled: boolean
  securityQuestionsEnabled: boolean
}

export function tenantUsersList(id: number): Promise<IamUser[]> {
  return client.get<unknown, IamUser[]>(`/tenants/${id}/users`)
}

export function tenantUsersCreate(
  id: number,
  data: { username: string; email: string; groupName?: string },
): Promise<{ password?: string }> {
  return client.post<unknown, { password?: string }>(`/tenants/${id}/users`, data)
}

export function tenantUsersDelete(id: number, userOcid: string): Promise<unknown> {
  return client.delete<unknown, unknown>(`/tenants/${id}/users/${encodeURIComponent(userOcid)}`)
}

export function tenantUsersResetPassword(
  id: number,
  userOcid: string,
): Promise<{ password?: string }> {
  return client.post<unknown, { password?: string }>(
    `/tenants/${id}/users/${encodeURIComponent(userOcid)}/reset-password`,
  )
}

export function tenantGroupsList(id: number): Promise<IamGroup[]> {
  return client.get<unknown, IamGroup[]>(`/tenants/${id}/groups`)
}

export function tenantPasswordPolicyGet(id: number): Promise<PasswordPolicy> {
  return client.get<unknown, PasswordPolicy>(`/tenants/${id}/password-policy`)
}

export function tenantPasswordPolicySave(
  id: number,
  data: { enableExpiry: boolean; expiryDays: number },
): Promise<unknown> {
  return client.post<unknown, unknown>(`/tenants/${id}/password-policy`, data)
}

export function tenantMfaStatus(id: number): Promise<MfaStatus> {
  return client.get<unknown, MfaStatus>(`/tenants/${id}/mfa/status`)
}

export function tenantMfaToggle(id: number, enable: boolean): Promise<unknown> {
  return client.post<unknown, unknown>(`/tenants/${id}/mfa/toggle`, { enable })
}

export function tenantMfaReset(id: number): Promise<{ deletedDevices?: number }> {
  return client.post<unknown, { deletedDevices?: number }>(`/tenants/${id}/mfa/reset`)
}

// ─── Domains ─────────────────────────────────────────────────────────

export function tenantDomainTenants(id: number): Promise<Tenant[]> {
  return client.get<unknown, Tenant[]>(`/tenants/${id}/domains`)
}

// ─── Audit ───────────────────────────────────────────────────────────

export interface AuditEvent {
  eventType: string
  userName: string
  userType?: string
  ipAddress: string
  clientEnv?: string
  eventTime: string
  responseStatus: string
}

export function tenantAuditLog(
  id: number,
  params: { days?: number; startDate?: string; endDate?: string },
): Promise<{ data: AuditEvent[] }> {
  return client.post<unknown, { data: AuditEvent[] }>(`/tenants/${id}/audit-log`, params)
}

// ─── Notification Recipients ─────────────────────────────────────────

export interface NotifRecipient {
  email: string
  state: string
}

export function tenantNotifRecipients(id: number): Promise<NotifRecipient[]> {
  return client.get<unknown, NotifRecipient[]>(`/tenants/${id}/notification-recipients`)
}

export function tenantNotifRecipientsUpdate(id: number, emails: string[]): Promise<unknown> {
  return client.post<unknown, unknown>(`/tenants/${id}/notification-recipients/update`, { emails })
}

// ─── Quota & Regions ─────────────────────────────────────────────────

export interface QuotaService {
  name: string
  description: string
}

export interface QuotaItem {
  name: string
  total: number
  used: number
  available: number
}

export interface RegionSubInfo {
  regionKey: string
  regionName: string
  status: string
  isHomeRegion: boolean
}

export interface RegionUnsubInfo {
  key: string
  name: string
  cnName?: string
}

export interface RegionSummary {
  totalRegions: number
  subscribedRegions: number
  unsubscribedRegions: number
}

export function tenantQuotaServices(id: number): Promise<QuotaService[]> {
  return client.get<unknown, QuotaService[]>(`/tenants/${id}/quota/services`)
}

export function tenantQuota(
  id: number,
  serviceName: string,
  pageSize = 100,
): Promise<{ items: QuotaItem[] }> {
  return client.get<unknown, { items: QuotaItem[] }>(`/tenants/${id}/quota`, {
    params: { serviceName, pageSize },
  })
}

export function tenantRegionsSummary(id: number): Promise<RegionSummary> {
  return client.get<unknown, RegionSummary>(`/tenants/${id}/regions/summary`)
}

export function tenantRegionsSubscribed(id: number): Promise<RegionSubInfo[]> {
  return client.get<unknown, RegionSubInfo[]>(`/tenants/${id}/regions/subscribed`)
}

export function tenantRegionsUnsubscribed(id: number): Promise<RegionUnsubInfo[]> {
  return client.get<unknown, RegionUnsubInfo[]>(`/tenants/${id}/regions/unsubscribed`)
}

export function tenantRegionsSubscribe(
  id: number,
  regionKeys: string[],
): Promise<{ details?: Array<{ success: boolean }> }> {
  return client.post<unknown, { details?: Array<{ success: boolean }> }>(
    `/tenants/${id}/regions/subscribe`,
    { regionKeys },
  )
}
