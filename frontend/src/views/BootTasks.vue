<template>
  <div class="boot-page">
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>抢机任务</h2>
        <el-tag :type="engineActive ? 'success' : 'info'" size="small">
          {{ engineActive ? '引擎运行中' : '引擎已停止' }}
        </el-tag>
      </div>
      <div class="toolbar-right">
        <el-button type="primary" @click="openAdd">
          <el-icon><Plus /></el-icon> 新建任务
        </el-button>
        <el-button @click="load" :loading="loading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </div>
    </div>

    <!-- System Status -->
    <el-card v-if="sysStatus" class="status-card" shadow="none">
      <template #header>
        <div class="card-header">
          <span class="card-title">
            <el-icon><SetUp /></el-icon> 引擎状态
          </span>
          <el-tag :type="sysStatus.running ? 'success' : 'danger'" size="small" effect="dark">
            {{ sysStatus.running ? '活跃' : '停止' }}
          </el-tag>
        </div>
      </template>
      <div class="status-grid">
        <div class="status-item">
          <span class="status-value">{{ sysStatus.totalTasks || 0 }}</span>
          <span class="status-label">总任务</span>
        </div>
        <div class="status-item">
          <span class="status-value" style="color:var(--status-up)">{{ sysStatus.runningTasks || 0 }}</span>
          <span class="status-label">运行中</span>
        </div>
        <div class="status-item">
          <span class="status-value">{{ sysStatus.activeKeyCount || 0 }}</span>
          <span class="status-label">活跃 Key</span>
        </div>
        <div class="status-item">
          <span class="status-value">{{ sysStatus.batchSize || '-' }}</span>
          <span class="status-label">批次大小</span>
        </div>
        <div class="status-item">
          <span class="status-value">{{ sysStatus.parentPool?.active ?? 0 }} / {{ sysStatus.parentPool?.queue ?? 0 }}</span>
          <span class="status-label">父池 活跃/队列</span>
        </div>
        <div class="status-item">
          <span class="status-value">{{ sysStatus.apiPool?.active ?? 0 }} / {{ sysStatus.apiPool?.completed ?? 0 }}</span>
          <span class="status-label">API池 活跃/完成</span>
        </div>
      </div>
    </el-card>

    <!-- Task Table -->
    <el-card shadow="none" class="table-card">
      <el-table :data="rows" v-loading="loading" border stripe size="default">
        <template #empty>
          <el-empty description="暂无抢机任务" :image-size="80">
            <el-button type="primary" @click="openAdd">新建任务</el-button>
          </el-empty>
        </template>
        <el-table-column label="任务ID" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="data-mono" style="font-size:var(--text-xs)">{{ row.bootId?.substring(0, 20) }}...</span>
          </template>
        </el-table-column>
        <el-table-column label="租户" width="100">
          <template #default="{ row }">
            <el-tag size="small" effect="plain" type="info">#{{ row.tenantId }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="架构" width="80" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="row.architecture === 'ARM' ? 'success' : 'warning'" effect="dark">
              {{ row.architecture || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="规格" min-width="150">
          <template #default="{ row }">
            <span class="data-mono">{{ row.ocpu }}C / {{ row.memory }}G / {{ row.disk }}GB</span>
          </template>
        </el-table-column>
        <el-table-column label="镜像" min-width="130" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.operatingSystem || '-' }} {{ row.operatingSystemVersion || '' }}
          </template>
        </el-table-column>
        <el-table-column prop="loopTime" label="间隔(s)" width="80" align="center" />
        <el-table-column label="进度" width="110" align="center">
          <template #default="{ row }">
            <span class="data-mono" style="color:var(--status-up)">{{ row.successCount || 0 }}</span>
            <span style="color:var(--text-muted)"> / </span>
            <span class="data-mono">{{ row.instanceCount || 0 }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status)" size="small" effect="dark">
              {{ statusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="失败" width="70" align="center">
          <template #default="{ row }">
            <span :style="{ color: row.failCount > 0 ? 'var(--status-down)' : 'var(--text-muted)' }">
              {{ row.failCount || 0 }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="nextExecutionTime" label="下次执行" width="155" />
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="edit(row)">
              <el-icon><Edit /></el-icon>
            </el-button>
            <el-button
              size="small"
              :type="row.status === 1 ? 'warning' : 'success'"
              @click="toggle(row)"
            >
              {{ row.status === 1 ? '暂停' : '启用' }}
            </el-button>
            <el-popconfirm title="确定删除此任务？" @confirm="remove(row)">
              <template #reference>
                <el-button size="small" type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- ================================================================ -->
    <!-- Add/Edit Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="addVisible" :title="editing ? '编辑任务' : '新建任务'" width="680px" destroy-on-close>
      <el-form :model="form" label-width="130px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="租户" required>
              <el-select v-model="form.tenantId" filterable placeholder="选择租户" style="width:100%">
                <el-option v-for="t in tenantList" :key="t.id" :label="`${t.name} (${t.region})`" :value="t.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="架构" required>
              <el-select v-model="form.architecture" placeholder="选择架构" style="width:100%">
                <el-option label="ARM (Ampere)" value="ARM" />
                <el-option label="AMD (Intel/AMD)" value="AMD" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="OCPU" required>
              <el-input-number v-model="form.ocpu" :min="1" :max="128" style="width:100%" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="内存 (GB)" required>
              <el-input-number v-model="form.memory" :min="1" :max="1024" style="width:100%" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="磁盘 (GB)" required>
              <el-input-number v-model="form.disk" :min="50" :max="32768" style="width:100%" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="循环间隔(秒)">
              <el-input-number v-model="form.loopTime" :min="3" :max="3600" style="width:100%" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="目标实例数">
              <el-input-number v-model="form.instanceCount" :min="1" :max="100" style="width:100%" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="云类型">
              <el-select v-model="form.cloudType" style="width:100%">
                <el-option label="Oracle Cloud" :value="1" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="镜像ID">
              <el-input v-model="form.imageId" placeholder="留空自动选择" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="操作系统">
              <el-input v-model="form.operatingSystem" placeholder="如: Canonical Ubuntu" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="系统版本">
              <el-input v-model="form.operatingSystemVersion" placeholder="如: 22.04" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Root密码">
              <el-input v-model="form.rootPassword" placeholder="留空自动生成" show-password />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="时间窗口">
              <el-input v-model="form.dataGap" placeholder="如: 00:00-23:59" />
              <div style="font-size:12px;color:var(--text-muted);margin-top:2px">留空表示全天候运行</div>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="备注">
              <el-input v-model="form.remark" placeholder="可选" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="addVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, SetUp, Edit } from '@element-plus/icons-vue'
import request from '../utils/request'

interface BootTask {
  id: number; bootId: string; tenantId: number; ocpu: number; memory: number; disk: number
  loopTime: number; instanceCount: number; status: number; architecture: string
  rootPassword: string; publicIp: string; imageId: string
  operatingSystem: string; operatingSystemVersion: string
  dataGap: string; notifyFlag: string; nextExecutionTime: string
  failCount: number; successCount: number; remark: string; cloudType: number
}
interface SysStatus {
  totalTasks: number; runningTasks: number; activeKeyCount: number
  batchSize: number; running: boolean; parentPool: any; apiPool: any
}
interface Tenant { id: number; name: string; region: string; tenancy: string }

const rows = ref<BootTask[]>([])
const loading = ref(false)
const addVisible = ref(false)
const saving = ref(false)
const editing = ref(false)
const sysStatus = ref<SysStatus | null>(null)
const tenantList = ref<Tenant[]>([])

const emptyForm = {
  bootId: '', tenantId: 0, ocpu: 4, memory: 24, disk: 100,
  loopTime: 6, instanceCount: 1, architecture: 'ARM', rootPassword: '',
  imageId: '', operatingSystem: 'Canonical Ubuntu', operatingSystemVersion: '22.04',
  dataGap: '', notifyFlag: 'NO', remark: '', cloudType: 1,
}
const form = ref({ ...emptyForm })

const engineActive = computed(() => {
  // Engine is active if it's running AND there are actually tasks
  return (sysStatus.value?.running ?? false) && (sysStatus.value?.totalTasks ?? 0) > 0
})

function statusTag(s: number) {
  return s === 1 ? 'warning' : s === 2 ? 'success' : s === 0 ? 'info' : ''
}
function statusText(s: number) {
  return s === 1 ? '运行中' : s === 2 ? '已完成' : s === 0 ? '已停用' : '未知'
}

async function load() {
  loading.value = true
  try {
    const [tasks, status] = await Promise.all([
      request.get('/boot/list') as Promise<BootTask[]>,
      request.get('/boot/systemStatus') as Promise<SysStatus>,
    ])
    rows.value = tasks
    sysStatus.value = status
  } catch (e: any) { ElMessage.error(e.message) }
  finally { loading.value = false }
}

async function loadTenants() {
  try { tenantList.value = await request.get('/boot/tenants') as Tenant[] }
  catch { /* ignore */ }
}

function openAdd() {
  editing.value = false
  form.value = { ...emptyForm }
  addVisible.value = true
}

function edit(row: BootTask) {
  editing.value = true
  form.value = {
    bootId: row.bootId || '',
    tenantId: row.tenantId,
    ocpu: row.ocpu, memory: row.memory, disk: row.disk,
    loopTime: row.loopTime, instanceCount: row.instanceCount,
    architecture: row.architecture,
    rootPassword: '',
    imageId: row.imageId || '',
    operatingSystem: row.operatingSystem || '',
    operatingSystemVersion: row.operatingSystemVersion || '',
    dataGap: row.dataGap || '',
    notifyFlag: row.notifyFlag || 'NO',
    remark: row.remark || '',
    cloudType: row.cloudType || 1,
  }
  addVisible.value = true
}

async function save() {
  if (!form.value.tenantId) { ElMessage.warning('请选择租户'); return }
  if (!form.value.architecture) { ElMessage.warning('请选择架构'); return }
  saving.value = true
  try {
    await request.post('/boot/save', form.value)
    ElMessage.success(editing.value ? '更新成功' : '创建成功')
    addVisible.value = false
    await load()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { saving.value = false }
}

async function remove(row: BootTask) {
  try {
    await request.get('/boot/delete', { params: { bootId: row.bootId } })
    ElMessage.success('已删除')
    await load()
  } catch (e: any) { ElMessage.error(e.message) }
}

async function toggle(row: BootTask) {
  const enable = row.status !== 1
  try {
    await request.get('/boot/toggle', { params: { bootId: row.bootId, enable: enable ? 1 : 0 } })
    ElMessage.success(enable ? '已启用' : '已暂停')
    await load()
  } catch (e: any) { ElMessage.error(e.message) }
}

onMounted(() => { load(); loadTenants() })
</script>

<style scoped>
.boot-page {
  padding: 0;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-5);
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

/* ---- Status Card ---- */
.status-card {
  margin-bottom: var(--space-5);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-default);
  background: var(--bg-surface);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.card-title {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-md);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

.status-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: var(--space-3);
}

.status-item {
  display: flex;
  flex-direction: column;
  padding: var(--space-3);
  background: var(--bg-raised);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
}

.status-value {
  font-size: var(--text-xl);
  font-weight: var(--font-bold);
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
}

.status-label {
  font-size: var(--text-2xs);
  color: var(--text-muted);
  margin-top: var(--space-1);
  text-transform: uppercase;
  letter-spacing: var(--tracking-wide);
}

/* ---- Table Card ---- */
.table-card {
  border-radius: var(--radius-md);
  overflow: hidden;
  border: 1px solid var(--border-default);
  background: var(--bg-surface);
}

.table-card :deep(.el-card__body) {
  padding: 0;
}

:deep(.el-table) {
  border-radius: var(--radius-md);
  overflow: hidden;
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

:deep(.el-pagination) {
  justify-content: center;
  margin-top: var(--space-5);
}

@media (max-width: 768px) {
  .toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .toolbar-left h2 {
    font-size: var(--text-lg);
  }

  .status-grid {
    grid-template-columns: 1fr 1fr;
  }
}
</style>