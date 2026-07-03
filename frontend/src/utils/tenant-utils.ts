// Shared utility functions for the tenant management UI.
// Extracted from Tenants.vue during the TenantList/TenantDetail refactor.

/** Mask a tenant name: show first + last char, middle replaced with *** */
export function maskedName(n: string): string {
  if (!n || n.length <= 2) return n || '***'
  return n[0] + '***' + n[n.length - 1]
}

/** Tag type for the account type badge */
export function accountTypeTag(t: string | undefined): 'success' | 'warning' | 'info' | '' {
  if (!t) return 'info'
  const s = t.toLowerCase()
  if (s === 'free_tier' || s === 'free' || s.includes('trial') || s.includes('试用')) return 'warning'
  if (s === 'payg' || s.includes('paid') || s.includes('付费')) return 'success'
  if (s.includes('enterprise') || s.includes('企业')) return ''
  return 'info'
}

/** Human-readable label for the account type (= subscription plan type) */
export function accountTypeLabel(t: string | undefined): string {
  if (!t) return '—'
  const m: Record<string, string> = {
    FREE_TIER: '免费', FREE: '免费',
    PAYG: '升级号',
    // legacy values (pre-sync) — keep for backward compat
    trial: '免费试用', paid: '付费账户', enterprise: '企业账户', free: '免费账户',
    PERSONAL: '个人', CORPORATE: '企业',
  }
  return m[t] || t
}

/** Cloud provider label from the numeric code */
export function cloudTypeLabel(ct: number | undefined): string {
  return ct === 1 ? 'OCI' : ct === 2 ? 'AWS' : ct === 4 ? 'Azure' : String(ct || '—')
}

/** CSS class for the instance state dot */
export function instStateDot(state: string): string {
  if (!state) return 'status-dot--idle'
  const s = state.toLowerCase()
  if (s === 'running') return 'status-dot--up status-dot--pulse'
  if (s === 'stopped' || s === 'terminated') return 'status-dot--down'
  if (s === 'starting' || s === 'stopping') return 'status-dot--warn'
  return 'status-dot--idle'
}

/** Format bytes to human-readable (B / KB / MB / GB) */
export function formatBytes(bytes: number | string | undefined): string {
  const n = Number(bytes)
  if (!n || isNaN(n)) return '0 B'
  if (n < 1024) return n + ' B'
  if (n < 1048576) return (n / 1024).toFixed(1) + ' KB'
  if (n < 1073741824) return (n / 1048576).toFixed(1) + ' MB'
  return (n / 1073741824).toFixed(2) + ' GB'
}
