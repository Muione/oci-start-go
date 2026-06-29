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

/** Security rule from GET /tenants/security-rules */
export interface SecurityRule {
  id?: string
  type: string
  protocol: string
  source: string
  ports?: string
  tenantId?: number
  icmpType?: string
}

/** Quota item from GET /tenants/:id/quota */
export interface QuotaItem {
  name: string
  total: number
  used: number
  available: number
}

/** Quota page response from GET /tenants/:id/quota */
export interface QuotaPage {
  region?: string
  regionEn?: string
  service?: string
  items: QuotaItem[]
  page: number
  pageSize: number
  hasNextPage: boolean
}

/** Region subscription info from GET /tenants/:id/regions/subscribed */
export interface RegionSubInfo {
  regionKey: string
  regionName: string
  status: string
  isHomeRegion: boolean
}

/** Unsubscribed region from GET /tenants/:id/regions/unsubscribed */
export interface RegionUnsubInfo {
  key: string
  name: string
  cnName?: string
}

/** Region summary from GET /tenants/:id/regions/summary */
export interface RegionSummary {
  totalRegions: number
  subscribedRegions: number
  unsubscribedRegions: number
}

/** Audit event from POST /tenants/:id/audit-log */
export interface AuditEvent {
  eventType: string
  userName: string
  userType?: string
  ipAddress: string
  clientEnv?: string
  eventTime: string
  responseStatus: string
}

/** Audit log page response */
export interface AuditLogPage {
  data: AuditEvent[]
  nextPageToken?: string
}

// --- VNIC Management (Phase 11.2) ---

/** Single VNIC info from GET /oci/vnic/loadData */
export interface VnicInfo {
  vnicId: string
  vnicDisplayName: string
  privateIp: string
  publicIp: string
  subnetId: string
  attachmentId: string
  lifecycleState: string
  isPrimary: boolean
  ipv6Addresses: string[]
  ipv6Ids: string[]
  success: boolean
  errorMessage: string | null
  createdAt: string
  instanceId: string
  instanceName: string
}

/** VNIC statistics from GET /oci/vnic/loadData */
export interface VnicStatistics {
  totalVnicCount: number
  activeVnicCount: number
  secondaryVnicCount: number
  totalIpv6Count: number
  primaryIpv6Count: number
}

/** Full VNIC load data response from GET /oci/vnic/loadData */
export interface VnicLoadData {
  vnicList: VnicInfo[]
  primaryVnic: VnicInfo | null
  secondaryVnics: VnicInfo[]
  statistics: VnicStatistics
  tenantId: string
}

/** Single VNIC creation result from batch create */
export interface VnicCreationResult {
  vnicId: string
  vnicDisplayName: string
  privateIp: string
  publicIp: string
  subnetId: string
  attachmentId: string
  lifecycleState: string
  ipv6Addresses: string[]
  ipv6Ids: string[]
  isPrimary: boolean
  success: boolean
  errorMessage: string | null
}

/** Batch VNIC creation result from POST /oci/vnic/create */
export interface BatchVnicResult {
  instanceId: string
  instanceDisplayName: string
  requestedVnicCount: number
  requestedIpv6CountPerVnic: number
  successfulVnicCount: number
  totalIpv6Count: number
  vnicResults: VnicCreationResult[]
  allSuccessful: boolean
  summary: string
  totalExecutionTimeMs: number
}

/** IPv6 creation result */
export interface Ipv6CreationResult {
  ipv6Id: string
  ipv6Address: string
  vnicId: string
  success: boolean
  errorMessage: string | null
}

/** Network config result from POST /oci/vnic/network/configureLoadBalancer */
export interface NetworkConfigResult {
  success: boolean
  message: string
  natGatewayId: string
  natGatewayName: string
  routeTableId: string
  routeTableName: string
  networkLoadBalancerId: string
  networkLoadBalancerName: string
  nlpIpAddress: string
}

/** IP switch result from POST /oci/vnic/changeSpecIp */
export interface IpSwitchResult {
  status: string
  message: string
  details: {
    oldIp: string
    newIp: string
  }
}

/** Subnet info for VNIC creation */
export interface SubnetInfo {
  subnetId: string
  displayName: string
  cidrBlock: string
  ipv6CidrBlock: string
  vcnId: string
  availabilityDomain: string
  lifecycleState: string
}

/** Object Storage bucket from GET /oci/storage/buckets */
export interface Bucket {
  name: string
  namespace: string
  timeCreated: string
  publicAccess: string
}

/** Object Storage object from GET /oci/storage/objects */
export interface StorageObject {
  name: string
  size: number
  timeModified: string
  contentType: string
  md5: string
}

/** Multipart upload record from GET /oci/storage/object/multipart/resumeable */
export interface MultipartUpload {
  id: number
  uploadId: string
  objectName: string
  bucketName: string
  namespace: string
  totalSize: number
  chunkSize: number
  totalParts: number
  completedPartCount: number
  completedParts: Array<{ partNum: number; etag: string }>
  createTime: string
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

// --- Nginx / Reverse Proxy (Phase 12.1) ---

/** Proxy config from GET /ssl/proxy/list */
export interface ProxyConfig {
  id: number
  domain: string
  targetHost: string
  targetPort: number
  protocol: string
  enableSsl: boolean
  enableWebSocket: boolean
  sslCertificateId: number | null
  configStatus: string
  sslStatus: string
  customConfig: string
  remark: string
  loadBalanceType: string
  enableHealthCheck: boolean
  healthCheckPath: string
  healthCheckInterval: number
  enableRateLimit: boolean
  rateLimit: number
  enableCache: boolean
  cacheTime: number
  createTime: string
  updateTime: string
}

/** SSL certificate from GET /ssl/certificates/list */
export interface SslCertificate {
  id: number
  domain: string
  certificateType: string
  email: string
  validationMethod: string
  autoRenew: number
  certificateStatus: string
  issueDate: string
  expireDate: string
  certificatePath: string
  privateKeyPath: string
  dnsProvider: string
  createTime: string
  updateTime: string
}

/** Nginx config version from GET /ssl/nginx/latest */
export interface NginxConfigVersion {
  id: number
  configName: string
  configContent: string
  isCurrent: number
  configVersion: number
  configStatus: string
  createTime: string
  updateTime: string
}

/** OpenResty service status from GET /ssl/openresty/status */
export interface OpenRestyServiceStatus {
  installed: boolean
  running: boolean
  apiAvailable: boolean
}

/** Nginx config status from GET /ssl/nginx/status */
export interface NginxConfigStatus {
  hasChanges: boolean
  currentVersion: number | null
  latestVersion: number | null
}
