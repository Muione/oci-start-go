// Shared API response types. The Axios interceptor (utils/request.ts) unwraps
// the {success, message, data, code} envelope — callers receive data directly.
// On success: `b.data` is returned.
// On failure: interceptor rejects with `new Error(b.message)`, so catch blocks
// receive the error message string.

/** Dashboard stats from GET /api/stats */
export interface DashboardStats {
  tenantCount: number
  proxyCount: number
  instanceCount: number
  backupCount: number
  onlineCount: number
}

/** Grab engine system status from GET /boot/systemStatus */
export interface EngineStatus {
  parentActive: number
  apiActive: number
  parentCapacity: number
  apiCapacity: number
  registeredJobs: number
  totalTasks?: number
  runningTasks?: number
  activeKeyCount?: number
  batchSize?: number
  running?: boolean
  parentPool?: { active: number; queue: number }
  apiPool?: { active: number; completed: number }
}

/** Message/notification channel status from GET /api/config/message-enabled */
export interface MessageChannels {
  enabled: boolean
  telegram: boolean
  dingtalk: boolean
  bark: boolean
  feishu: boolean
}

/** System config from GET /system/config */
export interface SystemConfig {
  strings: Record<string, string>
  bools: Record<string, boolean>
  appVersion: string
}

/** Tenant from GET /tenants/listAll */
export interface Tenant {
  id: number
  userName: string
  tenancy: string
  region: string
  regionName: string
  fingerprint: string
  apiSynced: boolean
}

/** Boot task from GET /boot/list */
export interface BootTask {
  id: number
  bootId: string
  tenantId: number
  ocpu: number
  memory: number
  disk: number
  loopTime: number
  instanceCount: number
  status: number
  architecture: string
  rootPassword: string
  publicIp: string
  imageId: string
  operatingSystem: string
  operatingSystemVersion: string
  dataGap: string
  notifyFlag: string
  nextExecutionTime: string
  failCount: number
  successCount: number
  remark: string
  cloudType: number
}

/** Instance from GET /instances/list */
export interface Instance {
  id: number
  tenantId: number
  tenantName: string
  instanceId: string
  displayName: string
  shape: string
  state: string
  ocpus: number
  memoryInGbs: number
  bootVolumeSizeInGbs: number
  publicIps: string
  privateIps: string
  availabilityDomain: string
  compartmentId: string
  bootVolumeId: string
  bootVolumeName: string
  vpusPerGb: string
  ipv6Addresses: string
  vnicIds: string
  architecture: string
  onLineEnable: number
  lastHeartbeat: string
  createTime: string
}

/** Proxy record from GET /proxies/list */
export interface Proxy {
  id: number
  proxyType: string
  proxyHost: string
  proxyPort: number
  proxyUsername: string
  proxyPassword: string
  availableStatus: number
}

/** DNS record from GET /dns/list */
export interface DnsRecord {
  id: number
  providerType: string
  domainName: string
  recordName: string
  recordType: string
  recordValue: string
  ttl: number
  proxied: boolean
  status: string
  zoneId: string
  createTime: string
  updateTime: string
}
