<template>
  <div class="quick-dd-page">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>Quick DD 一键重装</h2>
      </div>
    </div>

    <!-- DD form -->
    <el-card shadow="none" class="form-card">
      <el-form :model="ddForm" label-width="120px">
        <el-form-item label="选择实例">
          <el-select
            v-model="ddForm.instanceId"
            placeholder="选择实例"
            filterable
            style="width: 100%"
            :disabled="ddRunning"
          >
            <el-option
              v-for="inst in instances"
              :key="inst.instanceId"
              :label="`${inst.displayName} (${inst.publicIps || '-'})`"
              :value="inst.instanceId"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="操作系统">
          <el-select
            v-model="ddForm.osImage"
            placeholder="选择操作系统镜像"
            filterable
            style="width: 100%"
            :disabled="ddRunning"
          >
            <el-option-group label="Ubuntu">
              <el-option label="Ubuntu 22.04 LTS" value="ubuntu-22.04" />
              <el-option label="Ubuntu 20.04 LTS" value="ubuntu-20.04" />
              <el-option label="Ubuntu 24.04 LTS" value="ubuntu-24.04" />
            </el-option-group>
            <el-option-group label="Debian">
              <el-option label="Debian 12" value="debian-12" />
              <el-option label="Debian 11" value="debian-11" />
            </el-option-group>
            <el-option-group label="CentOS">
              <el-option label="CentOS 7" value="centos-7" />
              <el-option label="CentOS Stream 9" value="centos-stream-9" />
            </el-option-group>
            <el-option-group label="AlmaLinux">
              <el-option label="AlmaLinux 9" value="almalinux-9" />
              <el-option label="AlmaLinux 8" value="almalinux-8" />
            </el-option-group>
            <el-option-group label="Rocky Linux">
              <el-option label="Rocky Linux 9" value="rocky-9" />
              <el-option label="Rocky Linux 8" value="rocky-8" />
            </el-option-group>
            <el-option-group label="Fedora">
              <el-option label="Fedora 39" value="fedora-39" />
              <el-option label="Fedora 38" value="fedora-38" />
            </el-option-group>
          </el-select>
        </el-form-item>
        <el-form-item label="自定义镜像URL">
          <el-input
            v-model="ddForm.customImageUrl"
            placeholder="留空则使用上面选择的系统镜像，或输入自定义 DD 镜像 URL"
            :disabled="ddRunning"
          />
        </el-form-item>
        <el-form-item label="Root 密码">
          <el-input
            v-model="ddForm.rootPassword"
            type="password"
            show-password
            placeholder="设置新的 root 密码（可选）"
            :disabled="ddRunning"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            :loading="ddRunning && !ddCancelling"
            :disabled="ddRunning"
            @click="startDd"
          >
            <el-icon><VideoPlay /></el-icon> 开始重装
          </el-button>
          <el-button
            v-if="ddRunning"
            type="danger"
            :loading="ddCancelling"
            @click="cancelDd"
          >
            <el-icon><VideoPause /></el-icon> 取消
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- DD progress -->
    <el-card v-if="ddRunning || ddProgress.length > 0" shadow="none" class="progress-card">
      <template #header>
        <div class="card-header">
          <span>重装进度</span>
          <el-tag v-if="ddRunning" type="warning" size="small">进行中</el-tag>
          <el-tag v-else-if="ddCompleted" type="success" size="small">已完成</el-tag>
          <el-tag v-else-if="ddError" type="danger" size="small">失败</el-tag>
        </div>
      </template>

      <!-- Overall progress bar -->
      <div class="progress-overview" v-if="ddRunning">
        <el-progress
          :percentage="overallPercent"
          :stroke-width="20"
          :text-inside="true"
          :status="overallPercent >= 100 ? 'success' : ''"
        />
        <div class="progress-stats">
          <span v-if="ddSpeed">{{ ddSpeed }}</span>
          <span v-if="ddEta">ETA: {{ ddEta }}</span>
        </div>
      </div>

      <!-- Step log -->
      <div class="step-log" ref="stepLogRef">
        <div
          v-for="(step, idx) in ddProgress"
          :key="idx"
          class="step-item"
          :class="{ 'step-item--active': idx === ddProgress.length - 1 && ddRunning }"
        >
          <span class="step-icon">
            <el-icon v-if="step.status === 'success'" color="var(--status-up)"><SuccessFilled /></el-icon>
            <el-icon v-else-if="step.status === 'error'" color="var(--status-down)"><CircleCloseFilled /></el-icon>
            <el-icon v-else-if="step.status === 'running'" color="var(--status-warn)"><Loading /></el-icon>
            <el-icon v-else color="var(--text-muted)"><InfoFilled /></el-icon>
          </span>
          <span class="step-desc">{{ step.description }}</span>
          <span class="step-time">{{ step.time }}</span>
        </div>
      </div>
    </el-card>

    <!-- Warning alert -->
    <el-alert
      title="注意事项"
      type="warning"
      :closable="false"
      show-icon
      style="margin-top: var(--space-4)"
    >
      <template #default>
        <ul style="margin:0;padding-left:20px;font-size:var(--text-sm)">
          <li>DD 重装会完全覆盖实例上的数据，请确保重要数据已备份</li>
          <li>重装过程通常需要 5-15 分钟，取决于镜像大小和网络速度</li>
          <li>重装完成后实例会自动重启，SSH 密码将被重置</li>
          <li>重装期间请勿关闭此页面，否则需要重新开始</li>
        </ul>
      </template>
    </el-alert>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, nextTick, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { VideoPlay, VideoPause, SuccessFilled, CircleCloseFilled, Loading, InfoFilled } from '@element-plus/icons-vue'
import request from '../utils/request'

// ---- Types ----
interface Instance {
  instanceId: string
  displayName: string
  publicIps: string
}

interface DdStep {
  description: string
  status: 'pending' | 'running' | 'success' | 'error'
  time: string
}

// ---- State ----
const instances = ref<Instance[]>([])
const ddForm = ref({
  instanceId: '',
  osImage: '',
  customImageUrl: '',
  rootPassword: '',
})

const ddRunning = ref(false)
const ddCancelling = ref(false)
const ddCompleted = ref(false)
const ddError = ref(false)
const ddProgress = ref<DdStep[]>([])
const overallPercent = ref(0)
const ddSpeed = ref('')
const ddEta = ref('')
const stepLogRef = ref<HTMLElement | null>(null)

let eventSource: EventSource | null = null

// ---- Helpers ----
function now(): string {
  return new Date().toLocaleTimeString('zh-CN')
}

function scrollToBottom() {
  nextTick(() => {
    if (stepLogRef.value) {
      stepLogRef.value.scrollTop = stepLogRef.value.scrollHeight
    }
  })
}

// ---- Data loading ----
async function loadInstances() {
  try {
    const res = await request.get('/instances/list', { params: { limit: 9999, offset: 0 } }) as any
    instances.value = (res.items || []).map((i: any) => ({
      instanceId: i.instanceId,
      displayName: i.displayName,
      publicIps: i.publicIps,
    }))
  } catch { /* ignore */ }
}

// ---- DD operations ----
async function startDd() {
  if (!ddForm.value.instanceId) {
    ElMessage.warning('请先选择实例')
    return
  }
  if (!ddForm.value.osImage && !ddForm.value.customImageUrl) {
    ElMessage.warning('请选择操作系统或输入自定义镜像 URL')
    return
  }

  ddRunning.value = true
  ddCompleted.value = false
  ddError.value = false
  ddProgress.value = []
  overallPercent.value = 0
  ddSpeed.value = ''
  ddEta.value = ''

  // Add initial step
  ddProgress.value.push({
    description: '正在连接实例...',
    status: 'running',
    time: now(),
  })
  scrollToBottom()

  try {
    // Get the auth token for SSE
    const body: any = {
      instanceId: ddForm.value.instanceId,
    }
    if (ddForm.value.customImageUrl) {
      body.imageUrl = ddForm.value.customImageUrl
    } else {
      body.osImage = ddForm.value.osImage
    }
    if (ddForm.value.rootPassword) {
      body.rootPassword = ddForm.value.rootPassword
    }

    // Start the DD job and get a job ID
    const startRes = await request.post('/oci/quick-dd/start', body) as any
    const jobId = startRes?.jobId || startRes?.id || ''
    if (!jobId) {
      throw new Error('未获取到任务 ID')
    }

    // Connect to SSE stream
    const sseUrl = `/oci/quick-dd/stream?jobId=${encodeURIComponent(jobId)}`
    eventSource = new EventSource(sseUrl)

    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        handleSseEvent(data)
      } catch {
        // Plain text message
        ddProgress.value.push({
          description: event.data,
          status: 'running',
          time: now(),
        })
        scrollToBottom()
      }
    }

    eventSource.addEventListener('progress', (event) => {
      try {
        const data = JSON.parse((event as MessageEvent).data)
        handleSseEvent(data)
      } catch { /* ignore */ }
    })

    eventSource.addEventListener('step', (event) => {
      try {
        const data = JSON.parse((event as MessageEvent).data)
        if (data.description) {
          // Mark previous running step as success
          const lastRunning = ddProgress.value.find(s => s.status === 'running')
          if (lastRunning) lastRunning.status = 'success'
          ddProgress.value.push({
            description: data.description,
            status: 'running',
            time: now(),
          })
          scrollToBottom()
        }
      } catch { /* ignore */ }
    })

    eventSource.addEventListener('done', (event) => {
      try {
        const data = JSON.parse((event as MessageEvent).data)
        // Mark all running steps as success
        ddProgress.value.forEach(s => {
          if (s.status === 'running') s.status = 'success'
        })
        ddProgress.value.push({
          description: data.message || '重装完成！实例正在重启...',
          status: 'success',
          time: now(),
        })
        overallPercent.value = 100
        ddCompleted.value = true
        ddRunning.value = false
        scrollToBottom()
        ElMessage.success('重装已完成')
      } catch { /* ignore */ }
      closeSse()
    })

    eventSource.addEventListener('error', (event) => {
      try {
        const data = JSON.parse((event as MessageEvent).data)
        ddProgress.value.forEach(s => {
          if (s.status === 'running') s.status = 'error'
        })
        ddProgress.value.push({
          description: data.message || '重装失败',
          status: 'error',
          time: now(),
        })
        ddError.value = true
        ddRunning.value = false
        scrollToBottom()
        ElMessage.error(data.message || '重装失败')
      } catch {
        ddError.value = true
        ddRunning.value = false
      }
      closeSse()
    })

    eventSource.onerror = () => {
      if (ddRunning.value && !ddCompleted.value) {
        // Mark last running step as error
        const lastRunning = ddProgress.value.find(s => s.status === 'running')
        if (lastRunning) lastRunning.status = 'error'
        ddProgress.value.push({
          description: '连接中断，请检查实例状态',
          status: 'error',
          time: now(),
        })
        ddError.value = true
        ddRunning.value = false
        scrollToBottom()
      }
      closeSse()
    }
  } catch (e: any) {
    ddProgress.value.forEach(s => {
      if (s.status === 'running') s.status = 'error'
    })
    ddProgress.value.push({
      description: e.message || '启动失败',
      status: 'error',
      time: now(),
    })
    ddError.value = true
    ddRunning.value = false
    scrollToBottom()
    ElMessage.error(e.message || '启动 DD 重装失败')
  }
}

function handleSseEvent(data: any) {
  if (data.percent !== undefined) {
    overallPercent.value = Math.min(100, Math.max(0, Number(data.percent)))
  }
  if (data.speed) {
    ddSpeed.value = data.speed
  }
  if (data.eta) {
    ddEta.value = data.eta
  }
  if (data.step) {
    const lastRunning = ddProgress.value.find(s => s.status === 'running')
    if (lastRunning) lastRunning.status = 'success'
    ddProgress.value.push({
      description: data.step,
      status: 'running',
      time: now(),
    })
    scrollToBottom()
  }
  if (data.message && !data.step) {
    ddProgress.value.push({
      description: data.message,
      status: data.status === 'error' ? 'error' : 'running',
      time: now(),
    })
    scrollToBottom()
  }
}

async function cancelDd() {
  ddCancelling.value = true
  try {
    await request.post('/oci/quick-dd/cancel', { instanceId: ddForm.value.instanceId })
    ElMessage.info('正在取消重装...')
    ddProgress.value.push({
      description: '正在取消重装...',
      status: 'running',
      time: now(),
    })
    scrollToBottom()
  } catch (e: any) {
    ElMessage.error(e.message || '取消失败')
  } finally {
    ddCancelling.value = false
  }
  closeSse()
  ddRunning.value = false
}

function closeSse() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
}

// ---- Init ----
onMounted(loadInstances)
onBeforeUnmount(closeSse)
</script>

<style scoped>
.quick-dd-page {
  padding: 0;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-4);
  flex-wrap: wrap;
  gap: var(--space-4);
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.toolbar-left h2 {
  margin: 0;
  font-size: var(--text-xl);
  font-weight: var(--font-bold);
  color: var(--text-primary);
  letter-spacing: var(--tracking-tight);
}

.form-card {
  margin-bottom: var(--space-4);
  border-radius: var(--radius-md);
}

.progress-card {
  margin-bottom: var(--space-4);
  border-radius: var(--radius-md);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.progress-overview {
  margin-bottom: var(--space-4);
}

.progress-stats {
  display: flex;
  justify-content: space-between;
  margin-top: var(--space-2);
  font-size: var(--text-sm);
  color: var(--text-secondary);
  font-family: 'SF Mono', 'Fira Code', monospace;
}

.step-log {
  max-height: 400px;
  overflow-y: auto;
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  background: var(--bg-raised);
  padding: var(--space-3);
}

.step-item {
  display: flex;
  align-items: flex-start;
  gap: var(--space-2);
  padding: var(--space-2) 0;
  font-size: var(--text-sm);
  color: var(--text-secondary);
}

.step-item--active {
  color: var(--text-primary);
  font-weight: var(--font-medium);
}

.step-item + .step-item {
  border-top: 1px solid var(--border-subtle);
}

.step-icon {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  margin-top: 2px;
}

.step-desc {
  flex: 1;
}

.step-time {
  flex-shrink: 0;
  font-size: var(--text-xs);
  color: var(--text-muted);
  font-family: 'SF Mono', 'Fira Code', monospace;
}

:deep(.el-card__header) {
  padding: 12px 20px;
  font-weight: var(--font-semibold);
}

:deep(.el-descriptions) {
  background: transparent;
}

:deep(.el-descriptions__body) {
  background: transparent;
}

@media (max-width: 768px) {
  .toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .toolbar-left h2 {
    font-size: var(--text-lg);
  }
}
</style>
