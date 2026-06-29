<template>
  <div class="resmgr-page">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>Resource Manager</h2>
        <el-tag type="info" size="small">{{ stacks.length }} 个 Stack</el-tag>
      </div>
      <div class="toolbar-right">
        <el-button type="primary" @click="openCreateStack">
          <el-icon><Plus /></el-icon> 创建 Stack
        </el-button>
        <el-button @click="loadStacks" :loading="stacksLoading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </div>
    </div>

    <!-- Tenant selector -->
    <div class="filter-bar">
      <el-select
        v-model="tenantId"
        placeholder="选择租户"
        filterable
        style="width: 260px"
        @change="onTenantChange"
      >
        <el-option
          v-for="t in tenantOptions"
          :key="t.id"
          :label="t.name"
          :value="t.id"
        />
      </el-select>
    </div>

    <!-- Stack list -->
    <el-card shadow="none" class="table-card">
      <el-table :data="stacks" v-loading="stacksLoading" border stripe style="width: 100%">
        <template #empty>
          <el-empty :description="!tenantId ? '请先选择租户' : '暂无 Stack'" :image-size="80">
            <el-button v-if="tenantId" type="primary" @click="openCreateStack">创建 Stack</el-button>
          </el-empty>
        </template>
        <el-table-column type="index" label="#" width="50" align="center" />
        <el-table-column prop="displayName" label="名称" min-width="180">
          <template #default="{ row }">
            <span class="cell-link" @click="openJobList(row)">{{ row.displayName }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="stateTagType(row.lifecycleState)" size="small">
              {{ row.lifecycleState || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">
            <span class="data-mono" style="font-size:12px">{{ formatTime(row.timeCreated) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="300" fixed="right" align="center">
          <template #default="{ row }">
            <el-button-group>
              <el-button size="small" @click="openCreateJob(row, 'PLAN')">Plan</el-button>
              <el-button size="small" @click="openCreateJob(row, 'APPLY')">Apply</el-button>
              <el-button size="small" type="warning" @click="openCreateJob(row, 'DESTROY')">Destroy</el-button>
              <el-button size="small" @click="openJobList(row)">Jobs</el-button>
              <el-popconfirm title="确定删除此 Stack 及所有关联资源？" @confirm="deleteStack(row)">
                <template #reference>
                  <el-button size="small" type="danger">删除</el-button>
                </template>
              </el-popconfirm>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- ================================================================ -->
    <!-- Create Stack Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="createStackVisible" title="创建 Stack" width="650px" destroy-on-close>
      <el-form :model="createStackForm" label-width="120px">
        <el-form-item label="名称" required>
          <el-input v-model="createStackForm.displayName" placeholder="my-stack" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="createStackForm.description" placeholder="Stack 描述" />
        </el-form-item>
        <el-form-item label="来源方式">
          <el-radio-group v-model="createStackForm.sourceType">
            <el-radio label="zip">上传 .tf 文件</el-radio>
            <el-radio label="git">Git 仓库</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="createStackForm.sourceType === 'zip'" label="上传文件">
          <el-upload
            ref="tfUploadRef"
            :auto-upload="false"
            :limit="1"
            accept=".zip,.tar,.tar.gz,.tgz"
            :on-change="onTfFileChange"
            :on-exceed="() => ElMessage.warning('只能上传一个文件')"
          >
            <el-button size="small">选择文件</el-button>
            <template #tip>
              <div class="el-upload__tip">上传包含 .tf 文件的 zip/tar 包</div>
            </template>
          </el-upload>
        </el-form-item>
        <el-form-item v-if="createStackForm.sourceType === 'git'" label="Git URL" required>
          <el-input v-model="createStackForm.gitUrl" placeholder="https://github.com/user/repo.git" />
        </el-form-item>
        <el-form-item v-if="createStackForm.sourceType === 'git'" label="Git 分支">
          <el-input v-model="createStackForm.gitBranch" placeholder="main" />
        </el-form-item>
        <el-form-item v-if="createStackForm.sourceType === 'git'" label="工作目录">
          <el-input v-model="createStackForm.workingDirectory" placeholder="/path/to/tf/files" />
        </el-form-item>
        <el-form-item label="变量 (JSON)">
          <el-input
            v-model="createStackForm.variables"
            type="textarea"
            :rows="4"
            placeholder='{"key": "value", ...}'
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createStackVisible = false">取消</el-button>
        <el-button type="primary" :loading="createStackSaving" @click="doCreateStack">创建</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Job List Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="jobListVisible" :title="`Jobs - ${jobListStackName}`" width="90%" destroy-on-close>
      <el-table :data="jobs" v-loading="jobsLoading" border stripe size="small" style="width: 100%">
        <template #empty>
          <el-empty description="暂无 Job 记录" :image-size="60" />
        </template>
        <el-table-column type="index" label="#" width="50" align="center" />
        <el-table-column prop="operation" label="操作" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="operationTagType(row.operation)" size="small">{{ row.operation }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="stateTagType(row.lifecycleState)" size="small">
              {{ row.lifecycleState || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">
            <span class="data-mono" style="font-size:12px">{{ formatTime(row.timeCreated) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="耗时" width="100">
          <template #default="{ row }">
            <span class="data-mono">{{ formatDuration(row.timeFinished, row.timeCreated) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="failureDetails" label="错误信息" min-width="200" show-overflow-tooltip />
        <el-table-column label="操作" width="260" fixed="right" align="center">
          <template #default="{ row }">
            <el-button size="small" @click="viewJobLogs(row)">日志</el-button>
            <el-button size="small" @click="getTfState(row)" :disabled="row.operation !== 'APPLY'">State</el-button>
            <el-button
              size="small"
              type="danger"
              :disabled="!isJobCancellable(row.lifecycleState)"
              @click="cancelJob(row)"
            >
              取消
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Create Job Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="createJobVisible" :title="`创建 Job - ${createJobOperation}`" width="500px" destroy-on-close>
      <el-alert
        :title="jobAlertMessage"
        :type="createJobOperation === 'DESTROY' ? 'error' : 'info'"
        :closable="false"
        show-icon
        style="margin-bottom:16px"
      />
      <el-form :model="createJobForm" label-width="120px">
        <el-form-item label="操作">
          <el-tag :type="operationTagType(createJobOperation)" size="default">{{ createJobOperation }}</el-tag>
        </el-form-item>
        <el-form-item label="变量 (JSON)">
          <el-input
            v-model="createJobForm.variables"
            type="textarea"
            :rows="4"
            placeholder='{"key": "value"} — 留空使用 Stack 默认变量'
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createJobVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="createJobSaving"
          @click="doCreateJob"
        >
          执行 {{ createJobOperation }}
        </el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Job Logs Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="logsVisible" title="Job 日志" width="80%" destroy-on-close>
      <div class="logs-toolbar">
        <el-tag :type="stateTagType(logsJobState)" size="small">{{ logsJobState }}</el-tag>
        <el-button size="small" @click="refreshLogs" :loading="logsLoading">刷新</el-button>
      </div>
      <div class="logs-container" v-loading="logsLoading">
        <pre class="logs-content">{{ logsContent }}</pre>
      </div>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Terraform State Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="tfStateVisible" title="Terraform State" width="80%" destroy-on-close>
      <div class="tfstate-toolbar">
        <el-button size="small" @click="copyText(tfStateContent)">复制</el-button>
      </div>
      <div class="tfstate-container" v-loading="tfStateLoading">
        <pre class="tfstate-content">{{ tfStateContent }}</pre>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh } from '@element-plus/icons-vue'
import request from '../utils/request'

// ---- Types ----
interface Stack {
  id: string
  displayName: string
  description: string
  lifecycleState: string
  timeCreated: string
  compartmentId: string
}

interface Job {
  id: string
  stackId: string
  operation: string
  lifecycleState: string
  timeCreated: string
  timeFinished: string
  failureDetails?: string
}

// ---- State ----
const tenantId = ref<number | null>(null)
const tenantOptions = ref<Array<{ id: number; name: string }>>([])

// Stacks
const stacks = ref<Stack[]>([])
const stacksLoading = ref(false)

// Create Stack
const createStackVisible = ref(false)
const createStackSaving = ref(false)
const tfUploadRef = ref<any>(null)
const createStackForm = ref({
  displayName: '',
  description: '',
  sourceType: 'zip' as 'zip' | 'git',
  gitUrl: '',
  gitBranch: '',
  workingDirectory: '',
  variables: '',
})
let tfFile: File | null = null

// Jobs
const jobListVisible = ref(false)
const jobListStackId = ref('')
const jobListStackName = ref('')
const jobs = ref<Job[]>([])
const jobsLoading = ref(false)

// Create Job
const createJobVisible = ref(false)
const createJobSaving = ref(false)
const createJobOperation = ref('PLAN')
const createJobForm = ref({ variables: '' })
const createJobStackId = ref('')

// Job Logs
const logsVisible = ref(false)
const logsLoading = ref(false)
const logsContent = ref('')
const logsJobId = ref('')
const logsJobState = ref('')

// Terraform State
const tfStateVisible = ref(false)
const tfStateLoading = ref(false)
const tfStateContent = ref('')

// ---- Computed ----
const jobAlertMessage = computed(() => {
  switch (createJobOperation.value) {
    case 'PLAN': return 'Plan 会生成执行计划，预览将要进行的变更。'
    case 'APPLY': return 'Apply 会执行 Terraform 代码，创建或修改云资源。'
    case 'DESTROY': return 'WARNING: Destroy 会删除 Stack 创建的所有资源，此操作不可逆！'
    default: return ''
  }
})

// ---- Helpers ----
function formatTime(t: string | undefined): string {
  if (!t) return '-'
  try { return new Date(t).toLocaleString('zh-CN') } catch { return t }
}

function formatDuration(endTime: string | undefined, startTime: string | undefined): string {
  if (!endTime || !startTime) return '-'
  try {
    const ms = new Date(endTime).getTime() - new Date(startTime).getTime()
    if (ms < 0) return '-'
    const s = Math.floor(ms / 1000)
    if (s < 60) return `${s}s`
    const m = Math.floor(s / 60)
    const rs = s % 60
    if (m < 60) return `${m}m ${rs}s`
    const h = Math.floor(m / 60)
    const rm = m % 60
    return `${h}h ${rm}m`
  } catch { return '-' }
}

function stateTagType(state: string): '' | 'success' | 'warning' | 'danger' | 'info' {
  const s = (state || '').toLowerCase()
  if (s === 'active' || s === 'succeeded') return 'success'
  if (s === 'creating' || s === 'updating' || s === 'in_progress' || s === 'accepted') return 'warning'
  if (s === 'deleted' || s === 'failed' || s === 'canceled') return 'danger'
  return 'info'
}

function operationTagType(op: string): '' | 'success' | 'warning' | 'danger' | 'info' {
  const s = (op || '').toUpperCase()
  if (s === 'PLAN') return 'info'
  if (s === 'APPLY') return 'success'
  if (s === 'DESTROY') return 'danger'
  return ''
}

function isJobCancellable(state: string): boolean {
  const s = (state || '').toLowerCase()
  return s === 'accepted' || s === 'in_progress'
}

function copyText(text: string) {
  navigator.clipboard?.writeText(text).then(() => ElMessage.success('已复制到剪贴板')).catch(() => {})
}

// ---- Tenant ----
async function loadTenants() {
  try {
    const tenants = await request.get('/tenants/listAll') as any[]
    tenantOptions.value = tenants.map((t: any) => ({
      id: t.id,
      name: t.userName || t.tenancyName || `#${t.id}`,
    }))
    if (tenantOptions.value.length > 0 && !tenantId.value) {
      tenantId.value = tenantOptions.value[0].id
      await onTenantChange()
    }
  } catch { /* ignore */ }
}

function onTenantChange() {
  stacks.value = []
  if (tenantId.value) {
    loadStacks()
  }
}

// ---- Stack operations ----
async function loadStacks() {
  if (!tenantId.value) return
  stacksLoading.value = true
  try {
    const res = await request.get('/oci/resmgr/stacks', { params: { tenantId: tenantId.value } }) as any
    stacks.value = Array.isArray(res) ? res : (res?.items || [])
  } catch (e: any) {
    ElMessage.error('加载 Stack 失败: ' + (e?.message || e))
    stacks.value = []
  } finally {
    stacksLoading.value = false
  }
}

function openCreateStack() {
  createStackForm.value = {
    displayName: '',
    description: '',
    sourceType: 'zip',
    gitUrl: '',
    gitBranch: '',
    workingDirectory: '',
    variables: '',
  }
  tfFile = null
  createStackVisible.value = true
}

function onTfFileChange(file: any) {
  tfFile = file.raw || file
}

async function doCreateStack() {
  if (!createStackForm.value.displayName.trim()) {
    ElMessage.warning('请输入名称')
    return
  }
  if (!tenantId.value) {
    ElMessage.warning('请先选择租户')
    return
  }
  if (createStackForm.value.sourceType === 'zip' && !tfFile) {
    ElMessage.warning('请上传 Terraform 文件')
    return
  }
  if (createStackForm.value.sourceType === 'git' && !createStackForm.value.gitUrl.trim()) {
    ElMessage.warning('请输入 Git URL')
    return
  }

  createStackSaving.value = true
  try {
    const fd = new FormData()
    fd.append('tenantId', String(tenantId.value))
    fd.append('displayName', createStackForm.value.displayName.trim())
    fd.append('description', createStackForm.value.description)
    fd.append('sourceType', createStackForm.value.sourceType)

    if (createStackForm.value.sourceType === 'zip' && tfFile) {
      fd.append('file', tfFile)
    } else {
      fd.append('gitUrl', createStackForm.value.gitUrl)
      fd.append('gitBranch', createStackForm.value.gitBranch || 'main')
      fd.append('workingDirectory', createStackForm.value.workingDirectory || '')
    }

    if (createStackForm.value.variables) {
      fd.append('variables', createStackForm.value.variables)
    }

    await new Promise<void>((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', '/oci/resmgr/stack/create')
      xhr.withCredentials = true
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve()
        } else {
          let msg = '创建失败'
          try { msg = JSON.parse(xhr.responseText).message || msg } catch { /* */ }
          reject(new Error(msg))
        }
      }
      xhr.onerror = () => reject(new Error('网络错误'))
      xhr.send(fd)
    })

    ElMessage.success('Stack 创建成功')
    createStackVisible.value = false
    await loadStacks()
  } catch (e: any) {
    ElMessage.error('创建失败: ' + (e?.message || e))
  } finally {
    createStackSaving.value = false
  }
}

async function deleteStack(stack: Stack) {
  try {
    await ElMessageBox.confirm(
      `确定删除 Stack「${stack.displayName}」？这会同时删除 Stack 记录。`,
      '确认删除',
      { type: 'warning', confirmButtonText: '确认删除' }
    )
    await request.delete('/oci/resmgr/stack/delete', {
      data: { tenantId: tenantId.value, stackId: stack.id },
    })
    ElMessage.success('Stack 已删除')
    await loadStacks()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// ---- Job operations ----
async function openJobList(stack: Stack) {
  jobListStackId.value = stack.id
  jobListStackName.value = stack.displayName
  jobListVisible.value = true
  await loadJobs()
}

async function loadJobs() {
  if (!jobListStackId.value || !tenantId.value) return
  jobsLoading.value = true
  try {
    // We get jobs from the stack detail or a dedicated endpoint
    const res = await request.get('/oci/resmgr/stack/jobs', {
      params: { tenantId: tenantId.value, stackId: jobListStackId.value },
    }) as any
    jobs.value = Array.isArray(res) ? res : (res?.items || [])
  } catch {
    jobs.value = []
  } finally {
    jobsLoading.value = false
  }
}

function openCreateJob(stack: Stack, operation: string) {
  createJobStackId.value = stack.id
  createJobOperation.value = operation
  createJobForm.value = { variables: '' }
  createJobVisible.value = true
}

async function doCreateJob() {
  if (!createJobStackId.value || !tenantId.value) return
  createJobSaving.value = true
  try {
    const body: any = {
      tenantId: tenantId.value,
      stackId: createJobStackId.value,
      operation: createJobOperation.value,
    }
    if (createJobForm.value.variables) {
      try {
        body.variables = JSON.parse(createJobForm.value.variables)
      } catch {
        ElMessage.error('变量 JSON 格式不正确')
        createJobSaving.value = false
        return
      }
    }
    await request.post('/oci/resmgr/job/create', body)
    ElMessage.success(`${createJobOperation.value} Job 已创建`)
    createJobVisible.value = false
    // Refresh jobs if job list is open
    if (jobListVisible.value && jobListStackId.value === createJobStackId.value) {
      await loadJobs()
    }
  } catch (e: any) {
    ElMessage.error('创建 Job 失败: ' + (e?.message || e))
  } finally {
    createJobSaving.value = false
  }
}

async function viewJobLogs(job: Job) {
  logsJobId.value = job.id
  logsJobState.value = job.lifecycleState
  logsContent.value = ''
  logsVisible.value = true
  await refreshLogs()
}

async function refreshLogs() {
  if (!logsJobId.value || !tenantId.value) return
  logsLoading.value = true
  try {
    const res = await request.get('/oci/resmgr/job/logs', {
      params: { tenantId: tenantId.value, jobId: logsJobId.value },
    }) as any
    logsContent.value = typeof res === 'string' ? res : (res?.logs || res?.content || JSON.stringify(res, null, 2))
  } catch (e: any) {
    logsContent.value = '加载日志失败: ' + (e?.message || e)
  } finally {
    logsLoading.value = false
  }
}

async function getTfState(job: Job) {
  if (!tenantId.value) return
  tfStateContent.value = ''
  tfStateVisible.value = true
  tfStateLoading.value = true
  try {
    const res = await request.get('/oci/resmgr/job/tfstate', {
      params: { tenantId: tenantId.value, jobId: job.id },
    }) as any
    tfStateContent.value = typeof res === 'string' ? res : JSON.stringify(res, null, 2)
  } catch (e: any) {
    tfStateContent.value = '加载 State 失败: ' + (e?.message || e)
  } finally {
    tfStateLoading.value = false
  }
}

async function cancelJob(job: Job) {
  try {
    await ElMessageBox.confirm('确定取消此 Job？', '确认取消', { type: 'warning' })
    await request.delete('/oci/resmgr/job/cancel', {
      data: { tenantId: tenantId.value, jobId: job.id },
    })
    ElMessage.success('取消请求已发送')
    await loadJobs()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// ---- Init ----
onMounted(loadTenants)
</script>

<style scoped>
.resmgr-page {
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

.filter-bar {
  display: flex;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
  flex-wrap: wrap;
  align-items: center;
}

.table-card {
  border-radius: var(--radius-md);
  overflow: hidden;
}

.table-card :deep(.el-card__body) {
  padding: 0;
}

.cell-link {
  cursor: pointer;
  color: var(--accent);
  font-weight: var(--font-medium);
}

.cell-link:hover {
  color: var(--accent-hover);
  text-decoration: underline;
}

.logs-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-3);
}

.logs-container,
.tfstate-container {
  max-height: 60vh;
  overflow: auto;
  background: var(--bg-raised);
  border-radius: var(--radius-md);
  padding: var(--space-4);
  border: 1px solid var(--border-default);
}

.logs-content,
.tfstate-content {
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  font-size: 12px;
  line-height: 1.6;
  color: var(--text-primary);
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}

.tfstate-toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: var(--space-3);
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

:deep(.el-dialog) {
  border-radius: var(--radius-lg);
}

:deep(.el-dialog__title) {
  font-size: var(--text-lg);
  font-weight: var(--font-semibold);
}

:deep(.el-upload) {
  width: 100%;
}

@media (max-width: 768px) {
  .toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .toolbar-left h2 {
    font-size: var(--text-lg);
  }

  .filter-bar {
    flex-direction: column;
  }
}
</style>
