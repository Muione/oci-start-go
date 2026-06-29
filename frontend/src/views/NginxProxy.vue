<template>
  <div class="nginx-page">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>Nginx 管理</h2>
        <el-tag :type="openrestyStatus.running ? 'success' : 'danger'" size="small" effect="dark">
          OpenResty: {{ openrestyStatus.running ? '运行中' : openrestyStatus.installed ? '已停止' : '未安装' }}
        </el-tag>
      </div>
      <div class="toolbar-right">
        <el-button size="small" @click="checkOpenRestyStatus" :loading="openrestyLoading">
          <el-icon><RefreshRight /></el-icon> 刷新状态
        </el-button>
        <el-button v-if="openrestyStatus.installed && !openrestyStatus.running" size="small" type="success" @click="startOpenResty" :loading="openrestyStarting">
          <el-icon><VideoPlay /></el-icon> 启动 OpenResty
        </el-button>
        <el-button v-if="openrestyStatus.running" size="small" @click="reloadNginx" :loading="nginxReloading">
          <el-icon><RefreshRight /></el-icon> 重载配置
        </el-button>
      </div>
    </div>

    <el-tabs v-model="activeTab" @tab-change="onTabChange" type="border-card">
      <!-- ============================== -->
      <!-- Tab 1: Proxy Config -->
      <!-- ============================== -->
      <el-tab-pane label="反向代理" name="proxy">
        <div class="provider-toolbar">
          <div class="provider-left">
            <el-tag type="info" size="small">{{ proxyTotal }} 条配置</el-tag>
          </div>
          <div class="provider-right">
            <el-button size="small" type="primary" @click="openProxyAdd">
              <el-icon><Plus /></el-icon> 添加代理
            </el-button>
            <el-button size="small" @click="loadProxies" :loading="proxyLoading">
              <el-icon><RefreshRight /></el-icon> 刷新
            </el-button>
          </div>
        </div>

        <el-card shadow="none" class="table-card" style="margin-top:12px">
          <el-table :data="proxies" v-loading="proxyLoading" border stripe size="default" style="cursor:pointer">
            <template #empty>
              <el-empty description="暂无代理配置" :image-size="60">
                <el-button type="primary" size="small" @click="openProxyAdd">添加代理</el-button>
              </el-empty>
            </template>
            <el-table-column prop="domain" label="域名" min-width="180" show-overflow-tooltip />
            <el-table-column label="目标地址" min-width="200">
              <template #default="{ row }">
                <span class="data-mono">{{ row.targetHost }}:{{ row.targetPort }}</span>
              </template>
            </el-table-column>
            <el-table-column label="协议" width="80" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="row.protocol === 'https' ? 'success' : ''">{{ row.protocol || 'http' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="configStatusType(row.configStatus)" size="small">{{ configStatusLabel(row.configStatus) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="SSL" width="100" align="center">
              <template #default="{ row }">
                <template v-if="row.enableSsl">
                  <el-tag :type="sslStatusType(row.sslStatus)" size="small">{{ sslStatusLabel(row.sslStatus) }}</el-tag>
                </template>
                <span v-else class="text-muted">未启用</span>
              </template>
            </el-table-column>
            <el-table-column label="WebSocket" width="100" align="center">
              <template #default="{ row }">
                <el-icon v-if="row.enableWebSocket" color="var(--status-up)"><CircleCheck /></el-icon>
                <el-icon v-else color="var(--text-muted)"><Remove /></el-icon>
              </template>
            </el-table-column>
            <el-table-column label="创建时间" width="160">
              <template #default="{ row }">
                <span class="data-mono" style="font-size:12px">{{ formatTime(row.createTime) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="280" fixed="right">
              <template #default="{ row }">
                <el-button size="small" link @click.stop="openProxyEdit(row)">编辑</el-button>
                <el-button size="small" link :type="row.configStatus === 'DISABLED' ? 'success' : 'warning'" @click.stop="toggleProxy(row)">
                  {{ row.configStatus === 'DISABLED' ? '启用' : '禁用' }}
                </el-button>
                <el-button size="small" link @click.stop="testConnection(row)" :loading="testingId === row.id">测试</el-button>
                <el-dropdown trigger="click" @command="(cmd: string) => handleProxyAction(cmd, row)" style="margin-left:4px">
                  <el-button size="small" link>更多</el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="ssl" :disabled="row.enableSsl">
                        <el-icon><Lock /></el-icon> 配置 SSL
                      </el-dropdown-item>
                      <el-dropdown-item command="fix" :disabled="row.configStatus !== 'ERROR'">
                        <el-icon><Tools /></el-icon> 修复配置
                      </el-dropdown-item>
                      <el-dropdown-item command="delete" divided style="color:var(--status-down)">
                        <el-icon><Delete /></el-icon> 删除
                      </el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </template>
            </el-table-column>
          </el-table>

          <div v-if="proxyTotalPages > 1" style="display:flex;justify-content:center;margin-top:16px">
            <el-pagination
              v-model:current-page="proxyPage"
              :page-size="proxyPageSize"
              :total="proxyTotal"
              layout="prev, pager, next, total"
              @current-change="loadProxies"
            />
          </div>
        </el-card>
      </el-tab-pane>

      <!-- ============================== -->
      <!-- Tab 2: SSL Certificates -->
      <!-- ============================== -->
      <el-tab-pane label="SSL 证书" name="ssl">
        <div class="provider-toolbar">
          <div class="provider-left">
            <el-tag type="info" size="small">{{ certTotal }} 张证书</el-tag>
          </div>
          <div class="provider-right">
            <el-button size="small" type="primary" @click="openCertRequest">
              <el-icon><Plus /></el-icon> 申请证书
            </el-button>
            <el-button size="small" @click="loadCerts" :loading="certLoading">
              <el-icon><RefreshRight /></el-icon> 刷新
            </el-button>
          </div>
        </div>

        <el-card shadow="none" class="table-card" style="margin-top:12px">
          <el-table :data="certs" v-loading="certLoading" border stripe size="default">
            <template #empty>
              <el-empty description="暂无 SSL 证书" :image-size="60">
                <el-button type="primary" size="small" @click="openCertRequest">申请证书</el-button>
              </el-empty>
            </template>
            <el-table-column prop="domain" label="域名" min-width="180" show-overflow-tooltip />
            <el-table-column label="状态" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="certStatusType(row.certificateStatus)" size="small">{{ certStatusLabel(row.certificateStatus) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="类型" width="120" align="center">
              <template #default="{ row }">
                <el-tag size="small" effect="plain">{{ row.certificateType || 'LETS_ENCRYPT' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="签发时间" width="160">
              <template #default="{ row }">
                <span class="data-mono" style="font-size:12px">{{ formatTime(row.issueDate) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="过期时间" width="160">
              <template #default="{ row }">
                <span class="data-mono" style="font-size:12px" :class="{ 'text-danger': isExpiringSoon(row.expireDate) }">{{ formatTime(row.expireDate) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="自动续签" width="100" align="center">
              <template #default="{ row }">
                <el-switch v-model="row.autoRenew" :active-value="1" :inactive-value="0" size="small" @change="(val: any) => toggleAutoRenew(row, val)" />
              </template>
            </el-table-column>
            <el-table-column label="DNS 服务商" width="120" align="center">
              <template #default="{ row }">
                <span>{{ row.dnsProvider || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="200" fixed="right">
              <template #default="{ row }">
                <el-button size="small" link type="primary" @click="renewCert(row)" :disabled="row.certificateStatus !== 'VALID' && row.certificateStatus !== 'EXPIRING_SOON'">
                  续签
                </el-button>
                <el-button size="small" link @click="downloadCert(row)" :disabled="row.certificateStatus !== 'VALID'">
                  下载
                </el-button>
                <el-popconfirm title="确定删除此证书？如果有代理引用此证书，删除将被拒绝。" @confirm="deleteCert(row)">
                  <template #reference>
                    <el-button size="small" link type="danger">删除</el-button>
                  </template>
                </el-popconfirm>
              </template>
            </el-table-column>
          </el-table>

          <div v-if="certTotalPages > 1" style="display:flex;justify-content:center;margin-top:16px">
            <el-pagination
              v-model:current-page="certPage"
              :page-size="certPageSize"
              :total="certTotal"
              layout="prev, pager, next, total"
              @current-change="loadCerts"
            />
          </div>
        </el-card>
      </el-tab-pane>

      <!-- ============================== -->
      <!-- Tab 3: Nginx Config -->
      <!-- ============================== -->
      <el-tab-pane label="Nginx 配置" name="config">
        <div class="provider-toolbar">
          <div class="provider-left">
            <el-tag v-if="nginxStatus.hasChanges" type="warning" size="small">有未应用的变更</el-tag>
            <el-tag v-else type="success" size="small">配置已同步</el-tag>
            <span v-if="nginxStatus.currentVersion" style="font-size:12px;color:var(--text-secondary)">
              当前版本: v{{ nginxStatus.currentVersion }}
            </span>
            <span v-if="nginxStatus.latestVersion" style="font-size:12px;color:var(--text-secondary)">
              最新版本: v{{ nginxStatus.latestVersion }}
            </span>
          </div>
          <div class="provider-right">
            <el-button size="small" type="primary" @click="generateConfig" :loading="configGenerating">
              <el-icon><DocumentAdd /></el-icon> 生成配置
            </el-button>
            <el-button size="small" type="success" @click="applyConfig" :loading="configApplying" :disabled="!latestConfig.id">
              <el-icon><Select /></el-icon> 应用配置
            </el-button>
            <el-button size="small" @click="testConfig" :loading="configTesting" :disabled="!latestConfig.id">
              <el-icon><VideoPlay /></el-icon> 测试配置
            </el-button>
            <el-button size="small" @click="loadNginxStatus" :loading="nginxStatusLoading">
              <el-icon><RefreshRight /></el-icon> 刷新
            </el-button>
          </div>
        </div>

        <!-- Diff View -->
        <el-card v-if="configDiff" shadow="none" style="margin-top:12px">
          <template #header>
            <div style="display:flex;align-items:center;justify-content:space-between">
              <span style="font-weight:var(--font-semibold)">配置差异</span>
              <el-button size="small" link @click="configDiff = ''">关闭</el-button>
            </div>
          </template>
          <pre class="diff-view">{{ configDiff }}</pre>
        </el-card>

        <!-- Current Config Display -->
        <el-card shadow="none" style="margin-top:12px">
          <template #header>
            <div style="display:flex;align-items:center;justify-content:space-between">
              <span style="font-weight:var(--font-semibold)">
                {{ latestConfig.configName || 'Nginx 配置' }}
                <el-tag v-if="latestConfig.configStatus" size="small" :type="configVersionStatusType(latestConfig.configStatus)" style="margin-left:8px">
                  {{ latestConfig.configStatus }}
                </el-tag>
              </span>
              <el-button size="small" link @click="viewDiff" :loading="diffLoading">查看差异</el-button>
            </div>
          </template>
          <div v-if="configLoading" style="padding:24px">
            <el-skeleton :rows="8" animated />
          </div>
          <pre v-else-if="latestConfig.configContent" class="config-view">{{ latestConfig.configContent }}</pre>
          <el-empty v-else description="暂无配置，请先生成" :image-size="60" />
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- ============================== -->
    <!-- Proxy Add/Edit Dialog -->
    <!-- ============================== -->
    <el-dialog v-model="proxyDialogVisible" :title="proxyIsEdit ? '编辑代理配置' : '添加代理配置'" width="660px" destroy-on-close>
      <el-form :model="proxyForm" label-width="120px">
        <el-form-item label="域名" required>
          <el-input v-model="proxyForm.domain" placeholder="app.example.com" :disabled="proxyIsEdit" />
        </el-form-item>
        <el-form-item label="目标主机" required>
          <el-input v-model="proxyForm.targetHost" placeholder="10.0.0.5 或 localhost" />
        </el-form-item>
        <el-form-item label="目标端口" required>
          <el-input-number v-model="proxyForm.targetPort" :min="1" :max="65535" style="width:100%" />
        </el-form-item>
        <el-form-item label="协议">
          <el-select v-model="proxyForm.protocol" style="width:100%">
            <el-option label="HTTP" value="http" />
            <el-option label="HTTPS" value="https" />
          </el-select>
        </el-form-item>
        <el-form-item label="启用 SSL">
          <el-switch v-model="proxyForm.enableSsl" />
          <span style="font-size:12px;color:var(--text-secondary);margin-left:8px">启用后将自动生成 HTTPS 配置</span>
        </el-form-item>
        <el-form-item label="WebSocket">
          <el-switch v-model="proxyForm.enableWebSocket" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="proxyForm.remark" placeholder="备注信息" />
        </el-form-item>

        <el-divider content-position="left">高级选项</el-divider>

        <el-form-item label="负载均衡">
          <el-select v-model="proxyForm.loadBalanceType" style="width:100%">
            <el-option label="轮询 (round_robin)" value="round_robin" />
            <el-option label="IP Hash" value="ip_hash" />
            <el-option label="最少连接" value="least_conn" />
          </el-select>
        </el-form-item>
        <el-form-item label="健康检查">
          <el-switch v-model="proxyForm.enableHealthCheck" />
        </el-form-item>
        <template v-if="proxyForm.enableHealthCheck">
          <el-form-item label="检查路径">
            <el-input v-model="proxyForm.healthCheckPath" placeholder="/health" />
          </el-form-item>
          <el-form-item label="检查间隔 (秒)">
            <el-input-number v-model="proxyForm.healthCheckInterval" :min="5" :max="3600" style="width:100%" />
          </el-form-item>
        </template>
        <el-form-item label="限流">
          <el-switch v-model="proxyForm.enableRateLimit" />
        </el-form-item>
        <template v-if="proxyForm.enableRateLimit">
          <el-form-item label="请求/秒">
            <el-input-number v-model="proxyForm.rateLimit" :min="1" :max="100000" style="width:100%" />
          </el-form-item>
        </template>
        <el-form-item label="缓存">
          <el-switch v-model="proxyForm.enableCache" />
        </el-form-item>
        <template v-if="proxyForm.enableCache">
          <el-form-item label="缓存时间 (秒)">
            <el-input-number v-model="proxyForm.cacheTime" :min="10" :max="86400" style="width:100%" />
          </el-form-item>
        </template>
        <el-form-item label="自定义配置">
          <el-input v-model="proxyForm.customConfig" type="textarea" :rows="4" placeholder="# 自定义 nginx 配置片段" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="proxyDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="proxySaving" @click="doProxySave">
          {{ proxyIsEdit ? '更新' : '创建' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- ============================== -->
    <!-- SSL Request Dialog -->
    <!-- ============================== -->
    <el-dialog v-model="certDialogVisible" title="申请 SSL 证书" width="520px" destroy-on-close>
      <el-form :model="certForm" label-width="120px">
        <el-form-item label="域名" required>
          <el-input v-model="certForm.domain" placeholder="app.example.com" />
        </el-form-item>
        <el-form-item label="联系邮箱" required>
          <el-input v-model="certForm.email" placeholder="admin@example.com" />
        </el-form-item>
        <el-form-item label="证书类型">
          <el-select v-model="certForm.certificateType" style="width:100%">
            <el-option label="Let's Encrypt" value="LETS_ENCRYPT" />
          </el-select>
        </el-form-item>
        <el-form-item label="DNS 服务商">
          <el-select v-model="certForm.dnsProvider" style="width:100%">
            <el-option label="Cloudflare" value="CLOUDFLARE" />
            <el-option label="阿里云" value="ALIYUN" />
          </el-select>
        </el-form-item>
        <el-form-item label="验证方式">
          <el-select v-model="certForm.validationMethod" style="width:100%">
            <el-option label="DNS 验证" value="dns" />
            <el-option label="HTTP 验证" value="http" />
          </el-select>
        </el-form-item>
        <el-form-item label="自动续签">
          <el-switch v-model="certForm.autoRenew" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="certDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="certSaving" @click="doCertRequest">提交申请</el-button>
      </template>
    </el-dialog>

    <!-- ============================== -->
    <!-- Apply SSL Dialog -->
    <!-- ============================== -->
    <el-dialog v-model="sslDialogVisible" title="配置 SSL" width="460px" destroy-on-close>
      <el-alert title="将为此域名申请 Let's Encrypt 证书并启用 HTTPS" type="info" :closable="false" show-icon style="margin-bottom:16px" />
      <el-form :model="sslForm" label-width="100px">
        <el-form-item label="域名">
          <el-input :model-value="sslForm.domain" disabled />
        </el-form-item>
        <el-form-item label="联系邮箱" required>
          <el-input v-model="sslForm.email" placeholder="admin@example.com" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="sslDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="sslApplying" @click="doApplySsl">申请并配置</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus, RefreshRight, CircleCheck, Remove, VideoPlay,
  Delete, Lock, Tools, DocumentAdd, Select, Connection,
} from '@element-plus/icons-vue'
import request from '../utils/request'

// ---- Types ----
interface ProxyConfig {
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

interface SslCertificate {
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

interface NginxConfigRow {
  id: number
  configName: string
  configContent: string
  isCurrent: number
  configVersion: number
  configStatus: string
  createTime: string
  updateTime: string
}

interface OpenRestyStatus {
  installed: boolean
  running: boolean
  apiAvailable: boolean
}

interface NginxStatusInfo {
  hasChanges: boolean
  currentVersion: number | null
  latestVersion: number | null
}

// ---- State ----
const activeTab = ref('proxy')

// OpenResty status
const openrestyStatus = reactive<OpenRestyStatus>({ installed: false, running: false, apiAvailable: false })
const openrestyLoading = ref(false)
const openrestyStarting = ref(false)
const nginxReloading = ref(false)

// ---- Proxy Config State ----
const proxies = ref<ProxyConfig[]>([])
const proxyLoading = ref(false)
const proxySaving = ref(false)
const proxyPage = ref(1)
const proxyPageSize = ref(20)
const proxyTotal = ref(0)
const proxyTotalPages = ref(0)
const proxyDialogVisible = ref(false)
const proxyIsEdit = ref(false)
const proxyEditId = ref(0)
const testingId = ref(0)

const defaultProxyForm = () => ({
  domain: '',
  targetHost: '',
  targetPort: 8080,
  protocol: 'http',
  enableSsl: false,
  enableWebSocket: false,
  customConfig: '',
  remark: '',
  loadBalanceType: 'round_robin',
  enableHealthCheck: false,
  healthCheckPath: '/health',
  healthCheckInterval: 30,
  enableRateLimit: false,
  rateLimit: 100,
  enableCache: false,
  cacheTime: 300,
})
const proxyForm = ref(defaultProxyForm())

// ---- SSL Certificate State ----
const certs = ref<SslCertificate[]>([])
const certLoading = ref(false)
const certSaving = ref(false)
const certPage = ref(1)
const certPageSize = ref(20)
const certTotal = ref(0)
const certTotalPages = ref(0)
const certDialogVisible = ref(false)

const certForm = ref({
  domain: '',
  email: '',
  certificateType: 'LETS_ENCRYPT',
  dnsProvider: 'CLOUDFLARE',
  validationMethod: 'dns',
  autoRenew: true,
})

// SSL apply dialog
const sslDialogVisible = ref(false)
const sslApplying = ref(false)
const sslForm = ref({ proxyId: 0, domain: '', email: '' })

// ---- Nginx Config State ----
const nginxStatus = reactive<NginxStatusInfo>({ hasChanges: false, currentVersion: null, latestVersion: null })
const nginxStatusLoading = ref(false)
const latestConfig = ref<Partial<NginxConfigRow>>({})
const configLoading = ref(false)
const configGenerating = ref(false)
const configApplying = ref(false)
const configTesting = ref(false)
const configDiff = ref('')
const diffLoading = ref(false)

// ---- Helper Functions ----

function formatTime(t: string | undefined): string {
  if (!t) return '-'
  try { return new Date(t).toLocaleString('zh-CN') } catch { return t }
}

function isExpiringSoon(dateStr: string | undefined): boolean {
  if (!dateStr) return false
  const d = new Date(dateStr)
  const now = new Date()
  const diff = d.getTime() - now.getTime()
  return diff < 30 * 24 * 60 * 60 * 1000 && diff > 0
}

function configStatusType(s: string): '' | 'success' | 'warning' | 'danger' | 'info' {
  const map: Record<string, '' | 'success' | 'warning' | 'danger' | 'info'> = {
    PENDING: 'warning', APPLIED: 'success', ERROR: 'danger', DISABLED: 'info',
  }
  return map[s] || ''
}

function configStatusLabel(s: string): string {
  const map: Record<string, string> = {
    PENDING: '待应用', APPLIED: '已应用', ERROR: '错误', DISABLED: '已禁用',
  }
  return map[s] || s || '-'
}

function sslStatusType(s: string): '' | 'success' | 'warning' | 'danger' | 'info' {
  const map: Record<string, '' | 'success' | 'warning' | 'danger' | 'info'> = {
    NOT_CONFIGURED: 'info', CONFIGURED: 'success', PENDING: 'warning', ERROR: 'danger',
  }
  return map[s] || 'info'
}

function sslStatusLabel(s: string): string {
  const map: Record<string, string> = {
    NOT_CONFIGURED: '未配置', CONFIGURED: '已配置', PENDING: '处理中', ERROR: '错误',
  }
  return map[s] || s || '-'
}

function certStatusType(s: string): '' | 'success' | 'warning' | 'danger' | 'info' {
  const map: Record<string, '' | 'success' | 'warning' | 'danger' | 'info'> = {
    PENDING: 'warning', VALID: 'success', EXPIRING_SOON: 'warning', EXPIRED: 'danger', ERROR: 'danger',
  }
  return map[s] || 'info'
}

function certStatusLabel(s: string): string {
  const map: Record<string, string> = {
    PENDING: '申请中', VALID: '有效', EXPIRING_SOON: '即将过期', EXPIRED: '已过期', ERROR: '错误',
  }
  return map[s] || s || '-'
}

function configVersionStatusType(s: string): '' | 'success' | 'warning' | 'danger' | 'info' {
  const map: Record<string, '' | 'success' | 'warning' | 'danger' | 'info'> = {
    DRAFT: 'info', TESTING: 'warning', APPLIED: 'success', ERROR: 'danger',
  }
  return map[s] || 'info'
}

// ---- Tab Change ----
function onTabChange(tab: string | number) {
  if (tab === 'proxy') loadProxies()
  else if (tab === 'ssl') loadCerts()
  else if (tab === 'config') { loadNginxStatus(); loadLatestConfig() }
}

// ---- OpenResty Operations ----

async function checkOpenRestyStatus() {
  openrestyLoading.value = true
  try {
    const res = await request.get('/ssl/openresty/status') as any
    openrestyStatus.installed = res?.installed ?? false
    openrestyStatus.running = res?.running ?? false
    openrestyStatus.apiAvailable = res?.apiAvailable ?? false
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    openrestyLoading.value = false
  }
}

async function startOpenResty() {
  openrestyStarting.value = true
  try {
    await request.post('/ssl/openresty/start')
    ElMessage.success('OpenResty 启动成功')
    await checkOpenRestyStatus()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    openrestyStarting.value = false
  }
}

async function reloadNginx() {
  nginxReloading.value = true
  try {
    await request.post('/ssl/nginx/reload')
    ElMessage.success('配置已重载')
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    nginxReloading.value = false
  }
}

// ---- Proxy Config Operations ----

async function loadProxies() {
  proxyLoading.value = true
  try {
    const res = await request.get('/ssl/proxy/list', { params: { page: proxyPage.value - 1, size: proxyPageSize.value } }) as any
    proxies.value = res?.list || res?.records || []
    proxyTotal.value = res?.total ?? proxies.value.length
    proxyTotalPages.value = Math.ceil(proxyTotal.value / proxyPageSize.value)
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    proxyLoading.value = false
  }
}

function openProxyAdd() {
  proxyIsEdit.value = false
  proxyEditId.value = 0
  proxyForm.value = defaultProxyForm()
  proxyDialogVisible.value = true
}

function openProxyEdit(row: ProxyConfig) {
  proxyIsEdit.value = true
  proxyEditId.value = row.id
  proxyForm.value = {
    domain: row.domain,
    targetHost: row.targetHost,
    targetPort: row.targetPort,
    protocol: row.protocol || 'http',
    enableSsl: !!row.enableSsl,
    enableWebSocket: !!row.enableWebSocket,
    customConfig: row.customConfig || '',
    remark: row.remark || '',
    loadBalanceType: row.loadBalanceType || 'round_robin',
    enableHealthCheck: !!row.enableHealthCheck,
    healthCheckPath: row.healthCheckPath || '/health',
    healthCheckInterval: row.healthCheckInterval || 30,
    enableRateLimit: !!row.enableRateLimit,
    rateLimit: row.rateLimit || 100,
    enableCache: !!row.enableCache,
    cacheTime: row.cacheTime || 300,
  }
  proxyDialogVisible.value = true
}

async function doProxySave() {
  if (!proxyForm.value.domain || !proxyForm.value.targetHost || !proxyForm.value.targetPort) {
    ElMessage.warning('请填写域名、目标主机和端口')
    return
  }
  proxySaving.value = true
  try {
    if (proxyIsEdit.value) {
      await request.put(`/ssl/proxy/${proxyEditId.value}`, proxyForm.value)
      ElMessage.success('代理配置已更新')
    } else {
      await request.post('/ssl/proxy/create', proxyForm.value)
      ElMessage.success('代理配置已创建')
    }
    proxyDialogVisible.value = false
    await loadProxies()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    proxySaving.value = false
  }
}

async function toggleProxy(row: ProxyConfig) {
  const enabled = row.configStatus === 'DISABLED'
  try {
    await request.put(`/ssl/proxy/${row.id}/toggle`, { enabled })
    ElMessage.success(enabled ? '已启用' : '已禁用')
    await loadProxies()
  } catch (e: any) {
    ElMessage.error(e.message)
  }
}

async function testConnection(row: ProxyConfig) {
  testingId.value = row.id
  try {
    const res = await request.post(`/ssl/proxy/${row.id}/test-connection`) as any
    if (res?.connected) {
      ElMessage.success('连接成功')
    } else {
      ElMessage.warning('连接失败 — 目标不可达')
    }
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    testingId.value = 0
  }
}

function handleProxyAction(cmd: string, row: ProxyConfig) {
  if (cmd === 'ssl') {
    sslForm.value = { proxyId: row.id, domain: row.domain, email: '' }
    sslDialogVisible.value = true
  } else if (cmd === 'fix') {
    fixProxy(row)
  } else if (cmd === 'delete') {
    deleteProxy(row)
  }
}

async function doApplySsl() {
  if (!sslForm.value.email) {
    ElMessage.warning('请填写联系邮箱')
    return
  }
  sslApplying.value = true
  try {
    await request.post(`/ssl/proxy/${sslForm.value.proxyId}/ssl?email=${encodeURIComponent(sslForm.value.email)}`)
    ElMessage.success('SSL 证书申请已提交')
    sslDialogVisible.value = false
    await loadProxies()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    sslApplying.value = false
  }
}

async function fixProxy(row: ProxyConfig) {
  try {
    await request.post(`/ssl/proxy/${row.id}/fix`)
    ElMessage.success('已重置配置状态')
    await loadProxies()
  } catch (e: any) {
    ElMessage.error(e.message)
  }
}

async function deleteProxy(row: ProxyConfig) {
  try {
    await ElMessageBox.confirm(`确定删除代理配置「${row.domain}」？`, '确认删除', { type: 'warning' })
    await request.delete(`/ssl/proxy/${row.id}`)
    ElMessage.success('已删除')
    await loadProxies()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// ---- SSL Certificate Operations ----

async function loadCerts() {
  certLoading.value = true
  try {
    const res = await request.get('/ssl/certificates/list', { params: { page: certPage.value - 1, size: certPageSize.value } }) as any
    certs.value = res?.list || res?.records || []
    certTotal.value = res?.total ?? certs.value.length
    certTotalPages.value = Math.ceil(certTotal.value / certPageSize.value)
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    certLoading.value = false
  }
}

function openCertRequest() {
  certForm.value = {
    domain: '',
    email: '',
    certificateType: 'LETS_ENCRYPT',
    dnsProvider: 'CLOUDFLARE',
    validationMethod: 'dns',
    autoRenew: true,
  }
  certDialogVisible.value = true
}

async function doCertRequest() {
  if (!certForm.value.domain || !certForm.value.email) {
    ElMessage.warning('请填写域名和联系邮箱')
    return
  }
  certSaving.value = true
  try {
    await request.post('/ssl/certificates/request', certForm.value)
    ElMessage.success('证书申请已提交，请等待签发')
    certDialogVisible.value = false
    await loadCerts()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    certSaving.value = false
  }
}

async function renewCert(row: SslCertificate) {
  try {
    await ElMessageBox.confirm(`确定续签证书「${row.domain}」？`, '确认续签', { type: 'info' })
    await request.post(`/ssl/certificates/${row.id}/renew`)
    ElMessage.success('续签请求已提交')
    await loadCerts()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

async function deleteCert(row: SslCertificate) {
  try {
    await request.delete(`/ssl/certificates/${row.id}`)
    ElMessage.success('证书已删除')
    await loadCerts()
  } catch (e: any) {
    ElMessage.error(e.message)
  }
}

async function downloadCert(row: SslCertificate) {
  try {
    const res = await request.get(`/ssl/certificates/${row.id}/download`, { responseType: 'blob' }) as any
    const blob = res instanceof Blob ? res : new Blob([res], { type: 'application/zip' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `cert_${row.domain.replace(/\*/g, 'wildcard')}.zip`
    a.click()
    URL.revokeObjectURL(url)
    ElMessage.success('下载成功')
  } catch (e: any) {
    ElMessage.error(e.message)
  }
}

async function toggleAutoRenew(row: SslCertificate, val: any) {
  const enabled = val === true || val === 1
  try {
    await request.put(`/ssl/certificates/${row.id}/auto-renew`, { enabled })
    ElMessage.success(enabled ? '已开启自动续签' : '已关闭自动续签')
  } catch (e: any) {
    ElMessage.error(e.message)
    // Revert on error
    row.autoRenew = enabled ? 0 : 1
  }
}

// ---- Nginx Config Operations ----

async function loadNginxStatus() {
  nginxStatusLoading.value = true
  try {
    const res = await request.get('/ssl/nginx/status') as any
    nginxStatus.hasChanges = res?.hasChanges ?? false
    nginxStatus.currentVersion = res?.currentVersion ?? null
    nginxStatus.latestVersion = res?.latestVersion ?? null
  } catch (e: any) {
    // Status endpoint may not exist yet, silently ignore
  } finally {
    nginxStatusLoading.value = false
  }
}

async function loadLatestConfig() {
  configLoading.value = true
  try {
    const res = await request.get('/ssl/nginx/latest') as any
    latestConfig.value = res || {}
  } catch {
    latestConfig.value = {}
  } finally {
    configLoading.value = false
  }
}

async function generateConfig() {
  configGenerating.value = true
  try {
    const res = await request.post('/ssl/nginx/generate') as any
    ElMessage.success(`配置已生成 (版本 ${res?.configVersion || '-'})`)
    await loadNginxStatus()
    await loadLatestConfig()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    configGenerating.value = false
  }
}

async function applyConfig() {
  if (!latestConfig.value.id) {
    ElMessage.warning('没有可应用的配置')
    return
  }
  try {
    await ElMessageBox.confirm('将把最新配置推送到 OpenResty 并重载，是否继续？', '确认应用', { type: 'warning' })
  } catch { return }

  configApplying.value = true
  try {
    await request.post(`/ssl/nginx/${latestConfig.value.id}/apply`)
    ElMessage.success('配置已应用')
    await loadNginxStatus()
    await loadLatestConfig()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    configApplying.value = false
  }
}

async function testConfig() {
  if (!latestConfig.value.id) {
    ElMessage.warning('没有可测试的配置')
    return
  }
  configTesting.value = true
  try {
    await request.post(`/ssl/nginx/${latestConfig.value.id}/test`)
    ElMessage.success('配置语法测试通过')
  } catch (e: any) {
    ElMessage.error('配置测试失败: ' + e.message)
  } finally {
    configTesting.value = false
  }
}

async function viewDiff() {
  diffLoading.value = true
  try {
    const res = await request.get('/ssl/nginx/diff') as any
    if (res?.diff) {
      configDiff.value = res.diff
    } else {
      configDiff.value = res?.message || '无差异'
    }
  } catch (e: any) {
    configDiff.value = ''
    ElMessage.error(e.message)
  } finally {
    diffLoading.value = false
  }
}

// ---- Lifecycle ----
onMounted(async () => {
  await checkOpenRestyStatus()
  await loadProxies()
})
</script>

<style scoped>
.nginx-page { padding: 0; }

.toolbar {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: var(--space-5); flex-wrap: wrap; gap: var(--space-4);
}
.toolbar-left { display: flex; align-items: center; gap: var(--space-3); }
.toolbar-left h2 {
  margin: 0; font-size: var(--text-xl); font-weight: var(--font-bold);
  color: var(--text-primary); letter-spacing: var(--tracking-tight);
}
.toolbar-right { display: flex; gap: var(--space-2); }

.provider-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  gap: var(--space-3); flex-wrap: wrap;
}
.provider-left { display: flex; align-items: center; gap: var(--space-2); }
.provider-right { display: flex; gap: var(--space-2); }

.table-card { border-radius: var(--radius-md); overflow: hidden; }
.table-card :deep(.el-card__body) { padding: 0; }

.data-mono { font-family: var(--font-mono, monospace); }
.text-muted { color: var(--text-muted); font-size: 13px; }
.text-danger { color: var(--status-down); }

.config-view {
  background: var(--bg-raised);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  padding: var(--space-4);
  font-family: var(--font-mono, monospace);
  font-size: 13px;
  line-height: 1.6;
  overflow-x: auto;
  white-space: pre;
  max-height: 600px;
  margin: 0;
}

.diff-view {
  background: var(--bg-raised);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  padding: var(--space-4);
  font-family: var(--font-mono, monospace);
  font-size: 13px;
  line-height: 1.6;
  overflow-x: auto;
  white-space: pre;
  max-height: 400px;
  margin: 0;
}

:deep(.el-tabs--border-card) {
  border: 1px solid var(--border-default); border-radius: var(--radius-md);
  background: var(--bg-surface); box-shadow: none;
}
:deep(.el-tabs--border-card > .el-tabs__header) {
  background: var(--bg-raised); border-bottom: 1px solid var(--border-default);
}
:deep(.el-tabs--border-card > .el-tabs__content) { padding: var(--space-4); }
:deep(.el-table) { border-radius: var(--radius-md); overflow: hidden; }
:deep(.el-table th) { background: var(--bg-raised); font-weight: var(--font-semibold); color: var(--text-primary); }
:deep(.el-dialog) { border-radius: var(--radius-lg); }
:deep(.el-dialog__title) { font-size: var(--text-lg); font-weight: var(--font-semibold); }
:deep(.el-pagination) { justify-content: center; margin-top: var(--space-5); }

@media (max-width: 768px) {
  .toolbar { flex-direction: column; align-items: flex-start; }
  .toolbar-left h2 { font-size: var(--text-lg); }
  .provider-toolbar { flex-direction: column; align-items: flex-start; }
  .toolbar-right { width: 100%; justify-content: flex-start; flex-wrap: wrap; }
}
</style>
