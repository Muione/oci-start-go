<template>
  <div class="dashboard">
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>📊 仪表盘</h2>
        <el-tag type="info" size="small" effect="plain">实时监控</el-tag>
      </div>
      <el-button type="primary" @click="loadAll" :loading="loading">
        <el-icon><Refresh /></el-icon> 刷新
      </el-button>
    </div>

    <!-- Error Alert -->
    <el-alert
      v-if="loadError"
      :title="loadError"
      type="error"
      show-icon
      :closable="true"
      @close="loadError = ''"
      style="margin-bottom: 20px"
    />

    <!-- Stats Row -->
    <el-row :gutter="20" class="stats-row" v-loading="loading">
      <el-col :span="24" :md="12" :lg="8" :xl="4" v-for="card in statCards" :key="card.label" style="margin-bottom:20px">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-inner">
            <div class="stat-icon" :style="{ background: card.color }">
              <el-icon :size="28">
                <component :is="card.icon" />
              </el-icon>
            </div>
            <div class="stat-body">
              <div class="stat-value">{{ card.value }}</div>
              <div class="stat-label">{{ card.label }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Engine + Channels Row -->
    <el-row :gutter="20">
      <!-- Grab Engine -->
      <el-col :span="24" :lg="14">
        <el-card shadow="hover" class="section-card">
          <template #header>
            <div class="card-header">
              <span><el-icon><SetUp /></el-icon> 抢机引擎</span>
              <el-tag :type="engineRunning ? 'success' : 'danger'" size="small">
                {{ engineRunning ? '运行中' : '已停止' }}
              </el-tag>
            </div>
          </template>
          <el-row :gutter="24">
            <el-col :span="12">
              <div class="engine-section">
                <div class="engine-section-title">父池 (Parent Pool)</div>
                <div class="engine-metrics">
                  <div class="engine-metric">
                    <span class="metric-num">{{ engine.parentActive }}</span>
                    <span class="metric-label">活跃</span>
                  </div>
                  <div class="engine-metric">
                    <span class="metric-num">{{ engine.parentCapacity }}</span>
                    <span class="metric-label">容量</span>
                  </div>
                </div>
              </div>
            </el-col>
            <el-col :span="12">
              <div class="engine-section">
                <div class="engine-section-title">API 池</div>
                <div class="engine-metrics">
                  <div class="engine-metric">
                    <span class="metric-num">{{ engine.apiActive }}</span>
                    <span class="metric-label">活跃</span>
                  </div>
                  <div class="engine-metric">
                    <span class="metric-num">{{ engine.apiCapacity }}</span>
                    <span class="metric-label">容量</span>
                  </div>
                </div>
              </div>
            </el-col>
          </el-row>
          <el-divider style="margin:12px 0" />
          <el-row :gutter="16">
            <el-col :span="8">
              <el-statistic title="已注册 Cron" :value="engine.registeredJobs" />
            </el-col>
            <el-col :span="8">
              <el-statistic title="总任务数" :value="engine.totalTasks ?? '-'" />
            </el-col>
            <el-col :span="8">
              <el-statistic title="运行中任务" :value="engine.runningTasks ?? '-'" />
            </el-col>
          </el-row>
        </el-card>
      </el-col>

      <!-- Notification Channels -->
      <el-col :span="24" :lg="10">
        <el-card shadow="hover" class="section-card">
          <template #header>
            <span><el-icon><Bell /></el-icon> 通知渠道</span>
          </template>
          <div class="channels-list">
            <div v-for="ch in notificationChannels" :key="ch.name" class="channel-row">
              <div class="channel-left">
                <el-icon :size="20" :color="ch.enabled ? '#67c23a' : '#c0c4cc'">
                  <ChatDotRound v-if="ch.name === 'Telegram'" />
                  <Message v-else-if="ch.name === 'DingTalk'" />
                  <Bell v-else-if="ch.name === 'Bark'" />
                  <Notification v-else />
                </el-icon>
                <span class="channel-name">{{ ch.label }}</span>
              </div>
              <el-tag :type="ch.enabled ? 'success' : 'info'" size="small">
                {{ ch.enabled ? '已配置' : '未配置' }}
              </el-tag>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Quick Actions + System Info -->
    <el-row :gutter="20" style="margin-top:20px">
      <el-col :span="24" :lg="14">
        <el-card shadow="hover" class="section-card">
          <template #header>
            <span><el-icon><Link /></el-icon> 快捷入口</span>
          </template>
          <el-row :gutter="12">
            <el-col :span="8" v-for="link in quickLinks" :key="link.path" style="margin-bottom:12px">
              <el-button :type="link.type" plain @click="$router.push(link.path)" style="width:100%">
                <el-icon><component :is="link.icon" /></el-icon>
                {{ link.label }}
              </el-button>
            </el-col>
          </el-row>
        </el-card>
      </el-col>

      <el-col :span="24" :lg="10">
        <el-card shadow="hover" class="section-card">
          <template #header>
            <span><el-icon><InfoFilled /></el-icon> 系统概览</span>
          </template>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="应用版本">
              {{ appVersion || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="MFA">
              <el-tag :type="mfaEnabled ? 'success' : 'info'" size="small">
                {{ mfaEnabled ? '已启用' : '已禁用' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="SSL 证书">
              <el-tag v-if="sslDomain" type="success" size="small">已配置</el-tag>
              <span v-else style="color:#909399">未配置</span>
            </el-descriptions-item>
            <el-descriptions-item label="GCP">
              <el-tag :type="gcpConfigured ? 'success' : 'info'" size="small">
                {{ gcpConfigured ? '已配置' : '未配置' }}
              </el-tag>
            </el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  Refresh, Monitor, User, Connection, Files,
  SetUp, Bell, Link, InfoFilled,
  ChatDotRound, Message, Notification,
  Platform, Share, Setting, Promotion, VideoCamera, Warning
} from '@element-plus/icons-vue'
import request from '../utils/request'
import type { DashboardStats, EngineStatus, MessageChannels, SystemConfig } from '../types/api'

const loading = ref(false)
const loadError = ref('')

const stats = ref<DashboardStats>({
  tenantCount: 0, proxyCount: 0, instanceCount: 0, backupCount: 0, onlineCount: 0,
})
const engine = ref({
  parentActive: 0,
  apiActive: 0,
  parentCapacity: 0,
  apiCapacity: 0,
  registeredJobs: 0,
  totalTasks: 0,
  runningTasks: 0,
})
const notificationChannels = ref<Array<{ name: string; label: string; enabled: boolean }>>([])

// System overview
const appVersion = ref('')
const mfaEnabled = ref(false)
const sslDomain = ref('')
const gcpConfigured = ref(false)

const engineRunning = computed(() => {
  return engine.value.parentActive > 0 || engine.value.registeredJobs > 0
})

const statCards = computed(() => [
  {
    label: '实例总数',
    value: stats.value.instanceCount,
    icon: Monitor,
    color: 'linear-gradient(135deg, #409eff, #337ecc)',
  },
  {
    label: '在线实例',
    value: stats.value.onlineCount,
    icon: Monitor,
    color: 'linear-gradient(135deg, #67c23a, #529b2e)',
  },
  {
    label: '租户数量',
    value: stats.value.tenantCount,
    icon: User,
    color: 'linear-gradient(135deg, #e6a23c, #d4882e)',
  },
  {
    label: '代理数量',
    value: stats.value.proxyCount,
    icon: Connection,
    color: 'linear-gradient(135deg, #909399, #7a7e84)',
  },
  {
    label: '备份总数',
    value: stats.value.backupCount,
    icon: Files,
    color: 'linear-gradient(135deg, #f56c6c, #e05a5a)',
  },
])

const quickLinks = [
  { label: '抢机任务', path: '/boot', icon: Platform, type: 'primary' as const },
  { label: '实例列表', path: '/instances', icon: Monitor, type: 'success' as const },
  { label: 'SSH 终端', path: '/terminal', icon: Promotion, type: 'warning' as const },
  { label: '系统设置', path: '/settings', icon: Setting, type: 'info' as const },
  { label: 'VNC 控制台', path: '/console', icon: VideoCamera, type: '' as const },
  { label: '救援模式', path: '/rescue', icon: Warning, type: 'danger' as const },
]

async function loadAll() {
  loading.value = true
  loadError.value = ''
  try {
    await Promise.all([
      loadStats(),
      loadEngine(),
      loadNotifications(),
      loadSystemInfo(),
      loadGcpStatus(),
    ])
  } catch {
    // individual load functions handle errors
  }
  loading.value = false
}

async function loadStats() {
  try {
    const data: DashboardStats = await request.get('/api/stats')
    if (data) stats.value = data
  } catch (e: any) {
    if (!loadError.value) loadError.value = '加载统计数据失败: ' + (e.message || '未知错误')
  }
}

async function loadEngine() {
  try {
    const data: EngineStatus = await request.get('/boot/systemStatus')
    if (data) {
      engine.value = {
        parentActive: data.parentActive || 0,
        apiActive: data.apiActive || 0,
        parentCapacity: data.parentCapacity || 0,
        apiCapacity: data.apiCapacity || 0,
        registeredJobs: data.registeredJobs || 0,
        totalTasks: data.totalTasks || 0,
        runningTasks: data.runningTasks || 0,
      }
    }
  } catch { /* engine may not be available */ }
}

async function loadNotifications() {
  try {
    const data: MessageChannels = await request.get('/api/config/message-enabled')
    if (data) {
      notificationChannels.value = [
        { name: 'Telegram', label: 'Telegram', enabled: data.telegram },
        { name: 'DingTalk', label: '钉钉', enabled: data.dingtalk },
        { name: 'Bark', label: 'Bark', enabled: data.bark },
        { name: 'Feishu', label: '飞书', enabled: data.feishu },
      ]
    }
  } catch { /* optional */ }
}

async function loadSystemInfo() {
  try {
    const data: SystemConfig = await request.get('/system/config')
    if (data) {
      appVersion.value = data.appVersion || ''
      mfaEnabled.value = data.bools?.['mfa.enabled'] || false
      sslDomain.value = data.strings?.['ssl.domain'] || ''
    }
  } catch { /* optional */ }
}

async function loadGcpStatus() {
  try {
    const data = await request.get('/boot-instance/gcp/status')
    gcpConfigured.value = data?.configured || false
  } catch { /* GCP optional */ }
}

onMounted(loadAll)
</script>

<style scoped>
.dashboard { padding: 4px 0; }

.toolbar {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 20px; flex-wrap: wrap; gap: 12px;
}
.toolbar-left { display: flex; align-items: center; gap: 12px; }
.toolbar-left h2 { margin: 0; font-size: 20px; color: #303133; }

/* Stat Cards */
.stats-row { margin-bottom: 0; }
.stat-card { border-radius: 8px; cursor: default; }
.stat-card :deep(.el-card__body) { padding: 20px; }
.stat-inner { display: flex; align-items: center; gap: 16px; }
.stat-icon {
  width: 56px; height: 56px;
  border-radius: 12px;
  display: flex; align-items: center; justify-content: center;
  color: #fff; flex-shrink: 0;
}
.stat-body { flex: 1; min-width: 0; }
.stat-value { font-size: 28px; font-weight: 700; color: #303133; line-height: 1.2; }
.stat-label { font-size: 13px; color: #909399; margin-top: 4px; }

/* Section Cards */
.section-card { margin-bottom: 20px; border-radius: 8px; }
.section-card :deep(.el-card__header) { padding: 12px 20px; font-weight: 600; }
.card-header { display: flex; align-items: center; justify-content: space-between; }
.card-header .el-icon { margin-right: 6px; vertical-align: middle; }

/* Engine */
.engine-section { padding: 8px 0; }
.engine-section-title { font-size: 13px; color: #909399; margin-bottom: 12px; }
.engine-metrics { display: flex; gap: 24px; }
.engine-metric { text-align: center; }
.metric-num { font-size: 28px; font-weight: 700; color: #303133; display: block; }
.metric-label { font-size: 12px; color: #909399; }

/* Channels */
.channels-list { display: flex; flex-direction: column; gap: 12px; }
.channel-row {
  display: flex; align-items: center; justify-content: space-between;
  padding: 8px 12px; border-radius: 6px;
  background: #fafafa; transition: background 0.2s;
}
.channel-row:hover { background: #f0f2f5; }
.channel-left { display: flex; align-items: center; gap: 10px; }
.channel-name { font-size: 14px; color: #303133; }

/* Responsive */
@media (max-width: 768px) {
  .stat-inner { gap: 12px; }
  .stat-icon { width: 44px; height: 44px; }
  .stat-value { font-size: 22px; }
}
</style>
