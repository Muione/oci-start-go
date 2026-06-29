<template>
  <div class="bastion-page">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>堡垒机管理</h2>
        <el-tag type="info" size="small">{{ bastions.length }} 个堡垒机</el-tag>
      </div>
      <div class="toolbar-right">
        <el-button @click="loadBastions" :loading="loading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </div>
    </div>

    <!-- Bastion List -->
    <el-card shadow="none" class="table-card">
      <el-table :data="bastions" v-loading="loading" border stripe style="width: 100%" @expand-change="onBastionExpand">
        <template #empty>
          <el-empty description="暂无堡垒机数据" :image-size="80" />
        </template>
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="expand-content">
              <div class="expand-header">
                <h4>会话列表 — {{ row.displayName }}</h4>
                <el-button type="primary" size="small" @click="openCreateSession(row)">
                  <el-icon><Plus /></el-icon> 创建会话
                </el-button>
              </div>
              <el-table :data="row._sessions || []" v-loading="row._sessionsLoading" border size="small">
                <template #empty>
                  <el-empty description="暂无会话" :image-size="40" />
                </template>
                <el-table-column prop="displayName" label="会话名称" min-width="140" show-overflow-tooltip />
                <el-table-column label="会话类型" width="120">
                  <template #default="{ row: s }">
                    <el-tag :type="s.sessionType === 'MANAGED_SSH' ? '' : 'warning'" size="small">
                      {{ sessionTypeLabel(s.sessionType) }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="targetResourceDetails" label="目标资源" min-width="200" show-overflow-tooltip />
                <el-table-column label="状态" width="100" align="center">
                  <template #default="{ row: s }">
                    <span class="status-dot" :class="sessionStateDot(s.lifecycleState)"></span>
                    <span class="state-text">{{ s.lifecycleState || '-' }}</span>
                  </template>
                </el-table-column>
                <el-table-column label="TTL (分钟)" width="100" align="center">
                  <template #default="{ row: s }">{{ s.sessionTtlInSeconds ? Math.round(s.sessionTtlInSeconds / 60) : '-' }}</template>
                </el-table-column>
                <el-table-column label="创建时间" width="160">
                  <template #default="{ row: s }">{{ formatTime(s.timeCreated) }}</template>
                </el-table-column>
                <el-table-column label="过期时间" width="160">
                  <template #default="{ row: s }">{{ formatTime(s.timeUpdated) }}</template>
                </el-table-column>
                <el-table-column label="操作" width="160" fixed="right">
                  <template #default="{ row: s }">
                    <el-button size="small" type="primary" link @click="copySshCommand(s)" :disabled="!s.sshMetadata">
                      <el-icon><CopyDocument /></el-icon> 复制SSH
                    </el-button>
                    <el-popconfirm title="确定删除此会话？" @confirm="deleteSession(s)">
                      <template #reference>
                        <el-button size="small" type="danger" link>
                          <el-icon><Delete /></el-icon> 删除
                        </el-button>
                      </template>
                    </el-popconfirm>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="displayName" label="名称" min-width="160" sortable show-overflow-tooltip />
        <el-table-column label="类型" width="120">
          <template #default="{ row }">
            <el-tag size="small">{{ bastionTypeLabel(row.bastionType) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="targetSubnetDisplayName" label="目标子网" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">{{ row.targetSubnetDisplayName || row.targetSubnetId || '-' }}</template>
        </el-table-column>
        <el-table-column label="最大会话数" width="110" align="center">
          <template #default="{ row }">{{ row.maxSessionsAllowed || '-' }}</template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <span class="status-dot" :class="bastionStateDot(row.lifecycleState)"></span>
            <span class="state-text">{{ row.lifecycleState || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="160">
          <template #default="{ row }">{{ formatTime(row.timeCreated) }}</template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create Session Dialog -->
    <el-dialog v-model="sessionDialogVisible" title="创建堡垒机会话" width="600px" destroy-on-close>
      <el-alert
        title="创建会话后，将生成 SSH 连接命令，可通过堡垒机安全访问目标资源"
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
      />
      <el-form :model="sessionForm" label-width="120px">
        <el-form-item label="堡垒机">
          <el-input :model-value="sessionForm.bastionName" disabled />
        </el-form-item>
        <el-form-item label="会话类型" required>
          <el-select v-model="sessionForm.sessionType" style="width: 100%">
            <el-option label="托管 SSH (Managed SSH)" value="MANAGED_SSH" />
            <el-option label="端口转发 (Port Forwarding)" value="PORT_FORWARDING" />
          </el-select>
        </el-form-item>
        <el-form-item label="目标资源ID" required>
          <el-input v-model="sessionForm.targetResourceId" placeholder="ocid1.instance.oc1..xxxxx" />
        </el-form-item>
        <el-form-item v-if="sessionForm.sessionType === 'PORT_FORWARDING'" label="目标端口" required>
          <el-input-number v-model="sessionForm.targetPort" :min="1" :max="65535" style="width: 100%" />
        </el-form-item>
        <el-form-item label="TTL (分钟)" required>
          <el-input-number v-model="sessionForm.ttl" :min="5" :max="1440" :step="5" style="width: 100%" />
        </el-form-item>
        <el-form-item v-if="sessionForm.sessionType === 'MANAGED_SSH'" label="SSH 公钥" required>
          <el-input
            v-model="sessionForm.publicKey"
            type="textarea"
            :rows="4"
            placeholder="ssh-rsa AAAA... user@host"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="sessionDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="sessionSaving" @click="doCreateSession">创建会话</el-button>
      </template>
    </el-dialog>

    <!-- SSH Command Dialog -->
    <el-dialog v-model="sshDialogVisible" title="SSH 连接命令" width="680px" destroy-on-close>
      <el-alert
        title="请在本地终端执行以下命令连接到目标资源"
        type="success"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
      />
      <div class="ssh-command-box">
        <code>{{ sshCommand }}</code>
      </div>
      <template #footer>
        <el-button @click="sshDialogVisible = false">关闭</el-button>
        <el-button type="primary" @click="copyText(sshCommand)">
          <el-icon><CopyDocument /></el-icon> 复制命令
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Plus, Delete, CopyDocument } from '@element-plus/icons-vue'
import request from '../utils/request'

// ---- Types ----
interface Bastion {
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
  _sessions?: BastionSession[]
  _sessionsLoading?: boolean
}

interface BastionSession {
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

// ---- State ----
const bastions = ref<Bastion[]>([])
const loading = ref(false)

// Session dialog
const sessionDialogVisible = ref(false)
const sessionSaving = ref(false)
const sessionForm = ref({
  bastionId: '',
  bastionName: '',
  sessionType: 'MANAGED_SSH',
  targetResourceId: '',
  targetPort: 22,
  ttl: 30,
  publicKey: '',
})

// SSH command dialog
const sshDialogVisible = ref(false)
const sshCommand = ref('')

// ---- Helpers ----
function bastionTypeLabel(t: string): string {
  const map: Record<string, string> = {
    STANDARD: '标准',
    'STANDARD_ACK': '标准(ACK)',
  }
  return map[t] || t || '-'
}

function bastionStateDot(state: string): string {
  const s = (state || '').toLowerCase()
  if (s === 'active') return 'status-dot--up status-dot--pulse'
  if (s === 'creating' || s === 'updating') return 'status-dot--warn'
  if (s === 'deleted' || s === 'failed') return 'status-dot--down'
  return 'status-dot--idle'
}

function sessionStateDot(state: string): string {
  const s = (state || '').toLowerCase()
  if (s === 'active') return 'status-dot--up status-dot--pulse'
  if (s === 'creating') return 'status-dot--warn'
  if (s === 'ended' || s === 'deleted' || s === 'failed') return 'status-dot--down'
  return 'status-dot--idle'
}

function sessionTypeLabel(t: string): string {
  const map: Record<string, string> = {
    MANAGED_SSH: '托管SSH',
    PORT_FORWARDING: '端口转发',
    DYNAMIC_PORT_FORWARDING: '动态端口转发',
  }
  return map[t] || t || '-'
}

function formatTime(t: string | undefined): string {
  if (!t) return '-'
  try { return new Date(t).toLocaleString('zh-CN') } catch { return t }
}

function copyText(text: string) {
  navigator.clipboard?.writeText(text).then(() => ElMessage.success('已复制到剪贴板')).catch(() => {})
}

// ---- Data Loading ----
async function loadBastions() {
  loading.value = true
  try {
    const res = await request.get('/oci/bastion/list') as any
    bastions.value = (Array.isArray(res) ? res : (res?.items || [])).map((b: any) => ({
      ...b,
      _sessions: undefined,
      _sessionsLoading: false,
    }))
  } catch (e: any) {
    ElMessage.error(e.message || '加载堡垒机列表失败')
  } finally {
    loading.value = false
  }
}

async function onBastionExpand(row: Bastion, expandedRows: Bastion[]) {
  const isExpanded = expandedRows.some(r => r.bastionId === row.bastionId)
  if (!isExpanded || row._sessions) return
  row._sessionsLoading = true
  try {
    const res = await request.get('/oci/bastion/session/list', { params: { bastionId: row.bastionId } }) as any
    row._sessions = Array.isArray(res) ? res : (res?.items || [])
  } catch (e: any) {
    ElMessage.error(e.message || '加载会话列表失败')
    row._sessions = []
  } finally {
    row._sessionsLoading = false
  }
}

// ---- Create Session ----
function openCreateSession(bastion: Bastion) {
  sessionForm.value = {
    bastionId: bastion.bastionId,
    bastionName: bastion.displayName,
    sessionType: 'MANAGED_SSH',
    targetResourceId: '',
    targetPort: 22,
    ttl: 30,
    publicKey: '',
  }
  sessionDialogVisible.value = true
}

async function doCreateSession() {
  if (!sessionForm.value.targetResourceId) {
    ElMessage.warning('请填写目标资源ID')
    return
  }
  if (sessionForm.value.sessionType === 'MANAGED_SSH' && !sessionForm.value.publicKey) {
    ElMessage.warning('请填写 SSH 公钥')
    return
  }
  sessionSaving.value = true
  try {
    const body: any = {
      bastionId: sessionForm.value.bastionId,
      sessionType: sessionForm.value.sessionType,
      targetResourceId: sessionForm.value.targetResourceId,
      sessionTtlInSeconds: sessionForm.value.ttl * 60,
    }
    if (sessionForm.value.sessionType === 'PORT_FORWARDING') {
      body.targetPort = sessionForm.value.targetPort
    }
    if (sessionForm.value.sessionType === 'MANAGED_SSH') {
      body.publicKeyContent = sessionForm.value.publicKey
    }
    const res = await request.post('/oci/bastion/session/create', body) as any
    ElMessage.success('会话创建成功')
    sessionDialogVisible.value = false

    // Show SSH command if available
    if (res?.sshMetadata?.command) {
      sshCommand.value = res.sshMetadata.command
      sshDialogVisible.value = true
    }

    // Refresh sessions for the bastion
    const bastion = bastions.value.find(b => b.bastionId === sessionForm.value.bastionId)
    if (bastion) {
      bastion._sessions = undefined
      onBastionExpand(bastion, [bastion])
    }
  } catch (e: any) {
    ElMessage.error(e.message || '创建会话失败')
  } finally {
    sessionSaving.value = false
  }
}

// ---- Copy SSH Command ----
async function copySshCommand(session: BastionSession) {
  if (!session.sshMetadata) return
  const command = session.sshMetadata.command || session.sshMetadata['command']
  if (command) {
    sshCommand.value = command
    sshDialogVisible.value = true
  } else {
    // Try to fetch session details
    try {
      const res = await request.get('/oci/bastion/session/get', { params: { sessionId: session.sessionId } }) as any
      if (res?.sshMetadata?.command) {
        sshCommand.value = res.sshMetadata.command
        sshDialogVisible.value = true
      } else {
        ElMessage.warning('未找到 SSH 连接命令')
      }
    } catch (e: any) {
      ElMessage.error(e.message || '获取会话详情失败')
    }
  }
}

// ---- Delete Session ----
async function deleteSession(session: BastionSession) {
  try {
    await request.delete('/oci/bastion/session/delete', { data: { sessionId: session.sessionId } })
    ElMessage.success('会话已删除')
    // Refresh sessions
    const bastion = bastions.value.find(b => b.bastionId === session.bastionId)
    if (bastion) {
      bastion._sessions = undefined
      onBastionExpand(bastion, [bastion])
    }
  } catch (e: any) {
    ElMessage.error(e.message || '删除会话失败')
  }
}

onMounted(loadBastions)
</script>

<style scoped>
.bastion-page {
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

.table-card {
  border-radius: var(--radius-md);
  overflow: hidden;
}

.table-card :deep(.el-card__body) {
  padding: 0;
}

.state-cell {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.state-text {
  font-size: var(--text-xs);
  color: var(--text-secondary);
  font-weight: var(--font-medium);
}

.expand-content {
  padding: var(--space-4);
  background: var(--bg-raised);
}

.expand-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-3);
}

.expand-header h4 {
  margin: 0;
  font-size: var(--text-base);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

.ssh-command-box {
  padding: var(--space-4);
  background: var(--bg-root);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  overflow-x: auto;
}

.ssh-command-box code {
  font-family: 'Courier New', Courier, monospace;
  font-size: var(--text-sm);
  color: var(--text-primary);
  word-break: break-all;
  white-space: pre-wrap;
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
