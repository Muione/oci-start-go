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

// --- Shape & Image (Phase 2) ---

/** Shape info from GET /oci/shapes */
export interface ShapeInfo {
  shape: string
  ocpus: number
  memoryInGBs: number
  processorDescription: string
  architecture: string
  maxVnicAttachments: number
  gpuDescription?: string
  gpuCount?: number
  localDiskDescription?: string
  isFlexible: boolean
  baselineOcpu?: number
  networkingDescription?: string
}

/** Image info from GET /oci/images */
export interface ImageInfo {
  id: string
  displayName: string
  operatingSystem: string
  operatingSystemVersion: string
  architecture: string
  timeCreated: string
  sizeInGBs?: number
  launchMode?: string
}

/** VPU performance level option */
export interface VpuLevel {
  value: number
  label: string
  description: string
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

// --- Bastion (Phase 14) ---

/** Bastion from GET /oci/bastion/list */
export interface Bastion {
  id: string
  bastionId: string
  displayName: string
  bastionType: string
  targetSubnetId: string
  targetSubnetDisplayName?: string
  maxSessionsAllowed: number
  lifecycleState: string
  timeCreated: string
  compartmentId: string
}

/** Bastion session from GET /oci/bastion/session/list */
export interface BastionSession {
  sessionId: string
  bastionId: string
  displayName: string
  sessionType: string
  targetResourceDetails: string
  lifecycleState: string
  sessionTtlInSeconds: number
  sshMetadata?: Record<string, string>
  timeCreated: string
  timeUpdated: string
}

/** Create bastion session request body */
export interface CreateSessionRequest {
  bastionId: string
  sessionType: string
  targetResourceId: string
  targetPort?: number
  sessionTtlInSeconds: number
  publicKeyContent?: string
}

// --- Container Registry (Phase 14) ---

/** Repository from GET /oci/registry/repos */
export interface RegistryRepository {
  name: string
  namespace: string
  compartmentId: string
  compartmentName?: string
  imageCount?: number
  timeCreated: string
}

/** Image from GET /oci/registry/images */
export interface RegistryImage {
  digest: string
  tags: string[]
  sizeInBytes: number
  layersCount: number
  timeCreated: string
  repositoryName: string
}

/** Cleanup old images request body */
export interface RegistryCleanupRequest {
  keepDays: number
  repositoryName?: string
  compartmentName?: string
}

// --- AI Vision (Phase 14) ---

/** Vision job from GET /oci/vision/jobs */
export interface VisionJob {
  jobId: string
  jobType: string
  lifecycleState: string
  features: string[]
  timeCreated: string
  timeUpdated: string
  outputLocation?: any
}

/** Image classification label */
export interface ImageClassificationLabel {
  name: string
  confidence: number
}

/** Detected object */
export interface DetectedObject {
  name: string
  confidence: number
  boundingPolygon?: {
    normalizedVertices?: Array<{ x: number; y: number }>
    vertices?: Array<{ x: number; y: number }>
  }
}

/** Vision analysis result */
export interface VisionAnalysisResult {
  imageClassification?: {
    labels: ImageClassificationLabel[]
  }
  objectDetection?: {
    detectedObjects: DetectedObject[]
  }
  faceDetection?: {
    faceCount?: number
    faces?: Array<{
      confidence: number
      boundingPolygon?: any
    }>
  }
  textDetection?: {
    text?: string
    words?: Array<{
      text: string
      confidence: number
    }>
  }
  textExtraction?: {
    text: string
  }
  tableExtraction?: {
    tables?: Array<{
      headerRows?: string[][]
      rows: string[][]
    }>
  }
  keyValueExtraction?: {
    fields?: Array<{
      key: string
      value: string
      confidence?: number
    }>
    keyValuePairs?: Array<{
      key: string
      value: string
      confidence?: number
    }>
  }
  languageDetection?: {
    language: string
    confidence: number
  }
}

// --- IP Quality (Phase 13) ---

/** IP quality test result from POST /oci/ip-quality/test */
export interface IpQualityTestResult {
  score: number
  latency: number
  packetLoss: number
  downloadSpeed?: number
  uploadSpeed?: number
  location?: string
  isp?: string
  testTime?: string
}

/** IP auto-switch config from GET/POST /oci/ip-quality/auto-switch */
export interface IpAutoSwitchConfig {
  enabled: boolean
  threshold: number
  intervalMinutes: number
}

/** IP switch history record from GET /oci/ip-quality/history */
export interface IpSwitchHistory {
  instanceName: string
  oldIp: string
  newIp: string
  reason: string
  oldScore: number
  newScore: number
  switchTime: string
}

// --- Quick DD (Phase 13) ---

/** Quick DD start request body */
export interface QuickDdRequest {
  instanceId: string
  osImage?: string
  imageUrl?: string
  rootPassword?: string
}

/** Quick DD SSE progress event */
export interface QuickDdProgress {
  percent?: number
  speed?: string
  eta?: string
  step?: string
  message?: string
  status?: string
}

// --- NoSQL Database (Phase 13) ---

/** NoSQL table from GET /oci/nosql/tables */
export interface NosqlTable {
  name: string
  compartmentName: string
  compartmentId: string
  lifecycleState: string
  storageUsed: number
  timeCreated: string
}

/** Create NoSQL table request */
export interface CreateNosqlTableRequest {
  tenantId: number
  name: string
  ddl?: string
  readUnits?: number
  writeUnits?: number
  storageGB?: number
}

/** NoSQL query request */
export interface NosqlQueryRequest {
  tenantId: number
  statement: string
}

// --- MySQL Database (Phase 13) ---

/** MySQL DB System from GET /oci/mysql/systems */
export interface MysqlSystem {
  id: string
  displayName: string
  shapeName: string
  mysqlVersion: string
  lifecycleState: string
  availabilityDomain: string
  dataStorageSizeInGBs: number
  hostname: string
  ipAddress: string
  port: number
  timeCreated: string
}

/** Create MySQL DB System request */
export interface CreateMysqlSystemRequest {
  tenantId: number
  displayName: string
  shapeName: string
  mysqlVersion: string
  adminUsername: string
  adminPassword: string
  subnetId: string
  availabilityDomain?: string
  dataStorageSizeInGBs?: number
  hostname?: string
}

/** MySQL backup from GET /oci/mysql/backups */
export interface MysqlBackup {
  id: string
  displayName: string
  lifecycleState: string
  backupType: string
  sizeInBytes: number
  timeCreated: string
}

/** MySQL channel from GET /oci/mysql/channels */
export interface MysqlChannel {
  id: string
  displayName: string
  sourceDisplayName: string
  targetDisplayName: string
  lifecycleState: string
}

// --- Resource Manager / Terraform (Phase 13) ---

/** Stack from GET /oci/resmgr/stacks */
export interface ResmgrStack {
  id: string
  displayName: string
  description: string
  lifecycleState: string
  timeCreated: string
  compartmentId: string
}

/** Create stack request */
export interface CreateStackRequest {
  tenantId: number
  displayName: string
  description?: string
  sourceType: 'zip' | 'git'
  gitUrl?: string
  gitBranch?: string
  workingDirectory?: string
  variables?: Record<string, any>
}

/** Job from GET /oci/resmgr/stack/jobs */
export interface ResmgrJob {
  id: string
  stackId: string
  operation: string
  lifecycleState: string
  timeCreated: string
  timeFinished: string
  failureDetails?: string
}

/** Create job request */
export interface CreateJobRequest {
  tenantId: number
  stackId: string
  operation: 'PLAN' | 'APPLY' | 'DESTROY'
  variables?: Record<string, any>
}

// ─── Traffic Monitoring ───────────────────────────────────────────────

/** Traffic data from GET /oci/traffic */
export interface TrafficData {
  ingressToday: number    // GB
  egressToday: number     // GB
  ingressMonth: number    // GB
  egressMonth: number     // GB
}

// ─── Boot Volume ──────────────────────────────────────────────────────

/** Boot volume detail from GET /oci/boot-volume */
export interface BootVolumeDetail {
  id: string
  displayName: string
  sizeInGBs: number
  vpusPerGB: number
  lifecycleState: string
  timeCreated: string
}

/** Boot volume backup from GET /oci/boot-volume/backup */
export interface BootVolumeBackup {
  id: string
  displayName: string
  bootVolumeId: string
  sizeInGBs: number
  lifecycleState: string
  timeCreated: string
}

// ─── VCN ──────────────────────────────────────────────────────────────

/** VCN info from GET /oci/vcn */
export interface VcnInfo {
  id: string
  displayName: string
  cidrBlock: string
  dnsLabel: string
  defaultSecurityListId: string
  defaultRouteTableId: string
  timeCreated: string
}

// ─── NAT Gateway ──────────────────────────────────────────────────────

/** NAT gateway info from GET /oci/nat-gateway */
export interface NatGatewayInfo {
  id: string
  displayName: string
  vcnId: string
  lifecycleState: string
}

// ─── Route Table ──────────────────────────────────────────────────────

/** Route table info from GET /oci/route-table */
export interface RouteTableInfo {
  id: string
  displayName: string
  vcnId: string
  routeRules: RouteRuleInfo[]
}

/** Route rule within a route table */
export interface RouteRuleInfo {
  destination: string
  destinationType: string
  networkEntityId: string
}

// ─── IPv6 ─────────────────────────────────────────────────────────────

/** IPv6 address info from GET /oci/ipv6 */
export interface Ipv6Info {
  id: string
  ipAddress: string
  vnicId: string
}
