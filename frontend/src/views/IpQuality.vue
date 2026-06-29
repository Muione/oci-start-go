<template>
  <div class="ip-quality-page">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>IP 质量检测</h2>
      </div>
      <div class="toolbar-right">
        <el-button @click="loadHistory" :loading="historyLoading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </div>
    </div>

    <!-- Test form -->
    <el-card shadow="none" class="form-card">
      <el-form :model="testForm" label-width="120px">
        <el-form-item label="选择实例">
          <el-select
            v-model="testForm.instanceId"
            placeholder="选择实例"
            filterable
            style="width: 100%"
            @change="onInstanceChange"
          >
            <el-option
              v-for="inst in instances"
              :key="inst.instanceId"
              :label="`${inst.displayName} (${inst.publicIps || '-'})`"
              :value="inst.instanceId"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="测试类型">
          <el-radio-group v-model="testForm.testType">
            <el-radio-button label="ping">Ping</el-radio-button>
            <el-radio-button label="http">HTTP</el-radio-button>
            <el-radio-button label="tcp">TCP</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="目标 IP / 域名">
          <el-input v-model="testForm.target" placeholder="例如: 8.8.8.8 或 example.com" />
        </el-form-item>
        <el-form-item label="目标端口" v-if="testForm.testType === 'tcp' || testForm.testType === 'http'">
          <el-input-number v-model="testForm.port" :min="1" :max="65535" controls-position="right" style="width: 200px" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="testLoading" @click="startTest">
            开始测试
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Test results -->
    <el-card v-if="testResult" shadow="none" class="result-card">
      <template #header>
        <span>测试结果</span>
      </template>
      <el-descriptions :column="3" border size="small">
        <el-descriptions-item label="综合评分">
          <span class="score-value" :class="scoreClass(testResult.score)">{{ testResult.score }}</span>
          <span class="score-unit">分</span>
        </el-descriptions-item>
        <el-descriptions-item label="延迟">
          <span class="data-mono">{{ testResult.latency }} ms</span>
        </el-descriptions-item>
        <el-descriptions-item label="丢包率">
          <span class="data-mono">{{ testResult.packetLoss }}%</span>
        </el-descriptions-item>
        <el-descriptions-item label="下载速度" v-if="testResult.downloadSpeed">
          <span class="data-mono">{{ testResult.downloadSpeed }} Mbps</span>
        </el-descriptions-item>
        <el-descriptions-item label="上传速度" v-if="testResult.uploadSpeed">
          <span class="data-mono">{{ testResult.uploadSpeed }} Mbps</span>
        </el-descriptions-item>
        <el-descriptions-item label="IP 归属地" v-if="testResult.location">
          {{ testResult.location }}
        </el-descriptions-item>
        <el-descriptions-item label="运营商" v-if="testResult.isp">
          {{ testResult.isp }}
        </el-descriptions-item>
        <el-descriptions-item label="测试时间" v-if="testResult.testTime">
          {{ testResult.testTime }}
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- Auto-switch toggle -->
    <el-card shadow="none" class="switch-card">
      <div class="switch-row">
        <div class="switch-info">
          <h4>自动切换 IP</h4>
          <p>当 IP 质量评分低于阈值时，自动更换 IP 地址</p>
        </div>
        <el-switch
          v-model="autoSwitchEnabled"
          :loading="autoSwitchLoading"
          @change="toggleAutoSwitch"
        />
      </div>
      <div v-if="autoSwitchEnabled" class="switch-config">
        <el-form inline size="small">
          <el-form-item label="最低评分">
            <el-input-number v-model="autoSwitchThreshold" :min="1" :max="100" :step="5" controls-position="right" />
          </el-form-item>
          <el-form-item label="检测间隔(分钟)">
            <el-input-number v-model="autoSwitchInterval" :min="5" :max="1440" :step="5" controls-position="right" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" size="small" :loading="autoSwitchSaving" @click="saveAutoSwitchConfig">
              保存配置
            </el-button>
          </el-form-item>
        </el-form>
      </div>
    </el-card>

    <!-- IP switch history -->
    <el-card shadow="none" class="table-card">
      <template #header>
        <div class="card-header">
          <span>IP 切换历史</span>
          <el-tag type="info" size="small">{{ history.length }} 条记录</el-tag>
        </div>
      </template>
      <el-table :data="history" v-loading="historyLoading" border stripe style="width: 100%">
        <template #empty>
          <el-empty description="暂无切换记录" :image-size="80" />
        </template>
        <el-table-column type="index" label="#" width="50" align="center" />
        <el-table-column prop="instanceName" label="实例名称" min-width="140" />
        <el-table-column prop="oldIp" label="旧 IP" width="150">
          <template #default="{ row }">
            <span class="data-mono" style="font-size:12px">{{ row.oldIp || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="newIp" label="新 IP" width="150">
          <template #default="{ row }">
            <span class="data-mono" style="font-size:12px">{{ row.newIp || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="reason" label="切换原因" min-width="160" />
        <el-table-column prop="oldScore" label="旧评分" width="80" align="center">
          <template #default="{ row }">
            <span :class="scoreClass(row.oldScore)">{{ row.oldScore || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="newScore" label="新评分" width="80" align="center">
          <template #default="{ row }">
            <span :class="scoreClass(row.newScore)">{{ row.newScore || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="switchTime" label="切换时间" width="180">
          <template #default="{ row }">
            <span class="data-mono" style="font-size:12px">{{ formatTime(row.switchTime) }}</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import request from '../utils/request'

// ---- Types ----
interface Instance {
  instanceId: string
  displayName: string
  publicIps: string
}

interface TestResult {
  score: number
  latency: number
  packetLoss: number
  downloadSpeed?: number
  uploadSpeed?: number
  location?: string
  isp?: string
  testTime?: string
}

interface SwitchHistory {
  instanceName: string
  oldIp: string
  newIp: string
  reason: string
  oldScore: number
  newScore: number
  switchTime: string
}

// ---- State ----
const instances = ref<Instance[]>([])
const testForm = ref({
  instanceId: '',
  testType: 'ping',
  target: '',
  port: 80,
})
const testLoading = ref(false)
const testResult = ref<TestResult | null>(null)

const autoSwitchEnabled = ref(false)
const autoSwitchLoading = ref(false)
const autoSwitchThreshold = ref(60)
const autoSwitchInterval = ref(30)
const autoSwitchSaving = ref(false)

const history = ref<SwitchHistory[]>([])
const historyLoading = ref(false)

// ---- Helpers ----
function formatTime(t: string | undefined): string {
  if (!t) return '-'
  try { return new Date(t).toLocaleString('zh-CN') } catch { return t }
}

function scoreClass(score: number): string {
  if (!score && score !== 0) return ''
  if (score >= 80) return 'score-good'
  if (score >= 50) return 'score-medium'
  return 'score-bad'
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

function onInstanceChange() {
  const inst = instances.value.find(i => i.instanceId === testForm.value.instanceId)
  if (inst?.publicIps) {
    testForm.value.target = inst.publicIps
  }
}

async function startTest() {
  if (!testForm.value.instanceId) {
    ElMessage.warning('请先选择实例')
    return
  }
  if (!testForm.value.target) {
    ElMessage.warning('请输入目标 IP 或域名')
    return
  }
  testLoading.value = true
  testResult.value = null
  try {
    const body: any = {
      instanceId: testForm.value.instanceId,
      testType: testForm.value.testType,
      target: testForm.value.target,
    }
    if (testForm.value.testType === 'tcp' || testForm.value.testType === 'http') {
      body.port = testForm.value.port
    }
    const res = await request.post('/oci/ip-quality/test', body) as any
    testResult.value = res
    ElMessage.success('测试完成')
  } catch (e: any) {
    ElMessage.error(e.message || '测试失败')
  } finally {
    testLoading.value = false
  }
}

async function loadAutoSwitchStatus() {
  try {
    const res = await request.get('/oci/ip-quality/auto-switch') as any
    autoSwitchEnabled.value = res?.enabled || false
    autoSwitchThreshold.value = res?.threshold || 60
    autoSwitchInterval.value = res?.intervalMinutes || 30
  } catch { /* ignore */ }
}

async function toggleAutoSwitch(val: boolean) {
  autoSwitchLoading.value = true
  try {
    await request.post('/oci/ip-quality/auto-switch', {
      enabled: val,
      threshold: autoSwitchThreshold.value,
      intervalMinutes: autoSwitchInterval.value,
    })
    ElMessage.success(val ? '自动切换已启用' : '自动切换已禁用')
  } catch (e: any) {
    autoSwitchEnabled.value = !val
    ElMessage.error(e.message || '操作失败')
  } finally {
    autoSwitchLoading.value = false
  }
}

async function saveAutoSwitchConfig() {
  autoSwitchSaving.value = true
  try {
    await request.post('/oci/ip-quality/auto-switch', {
      enabled: autoSwitchEnabled.value,
      threshold: autoSwitchThreshold.value,
      intervalMinutes: autoSwitchInterval.value,
    })
    ElMessage.success('配置已保存')
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    autoSwitchSaving.value = false
  }
}

async function loadHistory() {
  historyLoading.value = true
  try {
    const res = await request.get('/oci/ip-quality/history') as any
    history.value = Array.isArray(res) ? res : (res?.items || [])
  } catch {
    history.value = []
  } finally {
    historyLoading.value = false
  }
}

// ---- Init ----
onMounted(() => {
  loadInstances()
  loadAutoSwitchStatus()
  loadHistory()
})
</script>

<style scoped>
.ip-quality-page {
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

.toolbar-right {
  display: flex;
  gap: var(--space-2);
}

.form-card {
  margin-bottom: var(--space-4);
  border-radius: var(--radius-md);
}

.result-card {
  margin-bottom: var(--space-4);
  border-radius: var(--radius-md);
}

.switch-card {
  margin-bottom: var(--space-4);
  border-radius: var(--radius-md);
}

.switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.switch-info h4 {
  margin: 0 0 4px 0;
  font-size: var(--text-md);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

.switch-info p {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--text-secondary);
}

.switch-config {
  margin-top: var(--space-4);
  padding-top: var(--space-4);
  border-top: 1px solid var(--border-default);
}

.table-card {
  border-radius: var(--radius-md);
  overflow: hidden;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.score-value {
  font-size: var(--text-2xl);
  font-weight: var(--font-bold);
}

.score-unit {
  font-size: var(--text-sm);
  color: var(--text-secondary);
  margin-left: 4px;
}

.score-good {
  color: var(--status-up);
}

.score-medium {
  color: var(--status-warn);
}

.score-bad {
  color: var(--status-down);
}

:deep(.el-table) {
  border-radius: var(--radius-md);
  overflow: hidden;
  font-size: var(--text-sm);
}

:deep(.el-table th) {
  background: var(--bg-raised);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
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

  .switch-row {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-3);
  }
}
</style>
