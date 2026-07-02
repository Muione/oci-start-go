<template>
  <div class="dashboard">
    <!-- Page header -->
    <PageHeader title="仪表盘">
      <template #actions>
        <el-button @click="loadAll" :loading="loading" size="small">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </template>
    </PageHeader>

    <!-- Error alert -->
    <el-alert
      v-if="loadError"
      :title="loadError"
      type="error"
      show-icon
      :closable="true"
      @close="loadError = ''"
      style="margin-bottom: 20px"
    />

    <!-- Stat cards -->
    <div class="stat-grid">
      <component
        v-for="card in statCards"
        :key="card.label"
        :is="card.link ? 'router-link' : 'div'"
        :to="card.link"
        class="stat-card"
        :class="{ 'stat-card--link': card.link }"
      >
        <div class="stat-header">
          <span class="stat-icon">
            <el-icon :size="16"><component :is="card.icon" /></el-icon>
          </span>
          <span class="stat-label">{{ card.label }}</span>
        </div>
        <div class="stat-body">
          <span class="stat-value">{{ card.value }}</span>
          <span v-if="card.sub" class="stat-sub">{{ card.sub }}</span>
        </div>
        <div class="stat-rule"></div>
        <div class="stat-status">
          <span class="status-dot" :class="'status-dot--' + card.statusColor"></span>
          <span class="stat-status-text">{{ card.statusText }}</span>
        </div>
      </component>

      <!-- Engine stat card -->
      <router-link to="/boot" class="stat-card stat-card--link">
        <div class="stat-header">
          <span class="stat-icon">
            <el-icon :size="16"><SetUp /></el-icon>
          </span>
          <span class="stat-label">抢机引擎</span>
        </div>
        <div class="stat-body">
          <span class="stat-value">{{ engine.runningTasks ?? 0 }}</span>
          <span class="stat-sub"> / {{ engine.totalTasks ?? 0 }}</span>
        </div>
        <div class="stat-rule"></div>
        <div class="stat-status">
          <span class="status-dot" :class="engineRunning ? 'status-dot--up' : 'status-dot--down'"></span>
          <span class="stat-status-text">{{ engineRunning ? '运行中' : '已停止' }}</span>
        </div>
      </router-link>
    </div>

    <!-- Engine + Channels -->
    <div class="panel-grid">
      <!-- Notification channels -->
      <div class="panel">
        <div class="panel-header">
          <span class="panel-title">
            <el-icon :size="16"><Bell /></el-icon> 通知渠道
          </span>
        </div>
        <div class="panel-body">
          <div class="channel-list">
            <router-link v-for="ch in notificationChannels" :key="ch.name" to="/settings" class="channel-row channel-row--link">
              <div class="channel-left">
                <el-icon :size="16" :color="ch.enabled ? 'var(--status-up)' : 'var(--text-muted)'">
                  <ChatDotRound v-if="ch.name === 'Telegram'" />
                  <Message v-else-if="ch.name === 'DingTalk'" />
                  <Bell v-else-if="ch.name === 'Bark'" />
                  <Notification v-else />
                </el-icon>
                <span class="channel-name">{{ ch.label }}</span>
              </div>
              <span class="channel-state" :class="ch.enabled ? 'channel-state--up' : ''">
                {{ ch.enabled ? '已配置' : '未配置' }}
              </span>
            </router-link>
          </div>
        </div>
      </div>
    </div>

    <!-- Quick links + System overview -->
    <div class="panel-grid">
      <!-- Quick links -->
      <div class="panel">
        <div class="panel-header">
          <span class="panel-title">
            <el-icon :size="16"><Link /></el-icon> 快捷入口
          </span>
        </div>
        <div class="panel-body">
          <div class="quick-links">
            <router-link
              v-for="link in quickLinks"
              :key="link.path"
              :to="link.path"
              class="quick-link"
            >
              <el-icon :size="14"><component :is="link.icon" /></el-icon>
              {{ link.label }}
            </router-link>
          </div>
        </div>
      </div>

      <!-- System overview -->
      <div class="panel">
        <div class="panel-header">
          <span class="panel-title">
            <el-icon :size="16"><InfoFilled /></el-icon> 系统概览
          </span>
        </div>
        <div class="panel-body">
          <div class="info-list">
            <div class="info-row">
              <span class="info-label">应用版本</span>
              <span class="info-value data-mono">{{ appVersion || '-' }}</span>
            </div>
            <router-link to="/settings" class="info-row info-row--link">
              <span class="info-label">MFA</span>
              <span class="info-state" :class="mfaEnabled ? 'info-state--up' : ''">
                {{ mfaEnabled ? '已启用' : '已禁用' }}
                <el-icon :size="12" style="margin-left: 4px;"><ArrowRight /></el-icon>
              </span>
            </router-link>
            <div class="info-row">
              <span class="info-label">SSL 证书</span>
              <span class="info-state" :class="sslDomain ? 'info-state--up' : ''">
                {{ sslDomain ? '已配置' : '未配置' }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  Refresh, Monitor, User, Connection,
  SetUp, Bell, Link, InfoFilled, ArrowRight,
  ChatDotRound, Message, Notification,
  Platform, Share, Setting, Promotion, VideoCamera, Warning
} from '@element-plus/icons-vue'
import PageHeader from '../components/common/PageHeader.vue'
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

const appVersion = ref('')
const mfaEnabled = ref(false)
const sslDomain = ref('')

const engineRunning = computed(() => {
  return engine.value.parentActive > 0 || engine.value.registeredJobs > 0
})

const statCards = computed(() => [
  {
    label: '在线实例',
    value: stats.value.onlineCount,
    icon: Monitor,
    sub: stats.value.instanceCount ? ` / ${stats.value.instanceCount}` : undefined,
    statusColor: 'up',
    statusText: stats.value.instanceCount
      ? Math.round((stats.value.onlineCount / stats.value.instanceCount) * 100) + '% 在线率'
      : '暂无数据',
    link: '/instances',
  },
  {
    label: '租户数量',
    value: stats.value.tenantCount,
    icon: User,
    sub: undefined,
    statusColor: 'idle',
    statusText: '已配置',
    link: '/tenants',
  },
])

const quickLinks = [
  { label: '抢机任务', path: '/boot', icon: Platform },
  { label: '实例列表', path: '/instances', icon: Monitor },
  { label: 'SSH 终端', path: '/terminal', icon: Promotion },
  { label: '系统设置', path: '/settings', icon: Setting },
  { label: 'VNC 控制台', path: '/console', icon: VideoCamera },
  { label: '救援模式', path: '/rescue', icon: Warning },
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

onMounted(loadAll)
</script>

<style scoped>
.dashboard {
  max-width: 1200px;
}

/* ============================================================
   Stat Grid
   ============================================================ */

.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: var(--space-4);
  margin-bottom: var(--space-6);
}

.stat-card {
  background: var(--bg-surface);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  padding: var(--space-4) var(--space-5);
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
}

.stat-card:hover {
  border-color: var(--border-default);
}

.stat-card--link {
  text-decoration: none;
  color: inherit;
  cursor: pointer;
}

.stat-card--link:hover {
  border-color: var(--accent);
  box-shadow: 0 0 0 1px var(--accent-subtle);
}

/* Header row */
.stat-header {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.stat-icon {
  display: flex;
  align-items: center;
  color: var(--text-muted);
}

.stat-label {
  font-size: var(--text-xs);
  font-weight: var(--font-medium);
  color: var(--text-muted);
  letter-spacing: var(--tracking-wide);
}

/* Value row */
.stat-body {
  display: flex;
  align-items: baseline;
  gap: var(--space-1);
}

.stat-value {
  font-size: var(--text-3xl);
  font-weight: var(--font-bold);
  color: var(--text-primary);
  line-height: 1;
  letter-spacing: var(--tracking-tight);
  font-variant-numeric: tabular-nums;
}

.stat-sub {
  font-size: var(--text-sm);
  color: var(--text-muted);
  font-weight: var(--font-medium);
}

/* Rule */
.stat-rule {
  height: 1px;
  background: var(--border-subtle);
}

/* Status row */
.stat-status {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.stat-status-text {
  font-size: var(--text-xs);
  color: var(--text-muted);
  font-weight: var(--font-medium);
}

/* ============================================================
   Panel Grid (2-col)
   ============================================================ */

.panel-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--space-4);
  margin-bottom: var(--space-4);
}

@media (max-width: 768px) {
  .panel-grid {
    grid-template-columns: 1fr;
  }
}

/* ============================================================
   Panel (card)
   ============================================================ */

.panel {
  background: var(--bg-surface);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  overflow: hidden;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-3) var(--space-5);
  border-bottom: 1px solid var(--border-subtle);
}

.panel-title {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-sm);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

.panel-title :deep(.el-icon) {
  color: var(--text-muted);
}

.panel-body {
  padding: var(--space-4) var(--space-5);
}

/* ============================================================
   Engine
   ============================================================ */

/* ============================================================
   Notification Channels
   ============================================================ */

.channel-list {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.channel-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-sm);
  transition: background var(--transition-fast);
}

.channel-row:hover {
  background: var(--bg-raised);
}

.channel-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.channel-name {
  font-size: var(--text-sm);
  color: var(--text-primary);
  font-weight: var(--font-medium);
}

.channel-state {
  font-size: var(--text-xs);
  color: var(--text-muted);
  font-weight: var(--font-medium);
}

.channel-state--up {
  color: var(--status-up);
}

.channel-row--link {
  text-decoration: none;
  color: inherit;
  cursor: pointer;
}

.channel-row--link:hover {
  background: var(--bg-hover);
}

/* ============================================================
   Quick Links
   ============================================================ */

.quick-links {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--space-2);
}

@media (max-width: 480px) {
  .quick-links {
    grid-template-columns: 1fr;
  }
}

.quick-link {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-sm);
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  color: var(--text-secondary);
  text-decoration: none;
  transition: background var(--transition-fast), color var(--transition-fast);
}

.quick-link:hover {
  background: var(--bg-raised);
  color: var(--accent);
}

/* ============================================================
   System Info
   ============================================================ */

.info-list {
  display: flex;
  flex-direction: column;
}

.info-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-2) 0;
}

.info-row--link {
  text-decoration: none;
  color: inherit;
  cursor: pointer;
  border-radius: var(--radius-sm);
  transition: background var(--transition-fast);
  padding-left: var(--space-1);
  padding-right: var(--space-1);
  margin: 0 calc(var(--space-1) * -1);
}

.info-row--link:hover {
  background: var(--bg-hover);
}

.info-row + .info-row {
  border-top: 1px solid var(--border-subtle);
}

.info-label {
  font-size: var(--text-sm);
  color: var(--text-secondary);
  font-weight: var(--font-medium);
}

.info-value {
  font-size: var(--text-sm);
  color: var(--text-primary);
  font-weight: var(--font-medium);
}

.info-state {
  font-size: var(--text-xs);
  color: var(--text-muted);
  font-weight: var(--font-medium);
}

.info-state--up {
  color: var(--status-up);
}

/* ============================================================
   Responsive
   ============================================================ */

@media (max-width: 768px) {
  .stat-grid {
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: var(--space-3);
  }

  .stat-value {
    font-size: var(--text-2xl);
  }
}

@media (max-width: 480px) {
  .stat-grid {
    grid-template-columns: 1fr 1fr;
  }

}
</style>
