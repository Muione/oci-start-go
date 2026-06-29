<template>
  <div class="mysql-page">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>MySQL 数据库管理</h2>
      </div>
      <div class="toolbar-right">
        <el-button type="primary" @click="openCreateSystem">
          <el-icon><Plus /></el-icon> 创建 DB System
        </el-button>
        <el-button @click="loadSystems" :loading="loading">
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

    <!-- Tabs -->
    <el-tabs v-model="activeTab">
      <!-- DB Systems tab -->
      <el-tab-pane label="DB System 列表" name="systems">
        <el-card shadow="none" class="table-card">
          <el-table :data="systems" v-loading="loading" border stripe style="width: 100%">
            <template #empty>
              <el-empty :description="!tenantId ? '请先选择租户' : '暂无 DB System'" :image-size="80">
                <el-button v-if="tenantId" type="primary" @click="openCreateSystem">创建 DB System</el-button>
              </el-empty>
            </template>
            <el-table-column type="index" label="#" width="50" align="center" />
            <el-table-column prop="displayName" label="名称" min-width="160">
              <template #default="{ row }">
                <span class="cell-link" @click="showSystemDetail(row)">{{ row.displayName }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="shapeName" label="Shape" min-width="140" show-overflow-tooltip />
            <el-table-column prop="mysqlVersion" label="版本" width="90" align="center" />
            <el-table-column label="状态" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="stateTagType(row.lifecycleState)" size="small">
                  {{ row.lifecycleState || '-' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="availabilityDomain" label="可用域" min-width="140" show-overflow-tooltip />
            <el-table-column label="创建时间" width="180">
              <template #default="{ row }">
                <span class="data-mono" style="font-size:12px">{{ formatTime(row.timeCreated) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="260" fixed="right" align="center">
              <template #default="{ row }">
                <el-button-group>
                  <el-button
                    size="small"
                    :disabled="row.lifecycleState !== 'INACTIVE'"
                    @click="startSystem(row)"
                  >
                    启动
                  </el-button>
                  <el-button
                    size="small"
                    :disabled="row.lifecycleState !== 'ACTIVE'"
                    @click="stopSystem(row)"
                  >
                    停止
                  </el-button>
                  <el-button
                    size="small"
                    :disabled="row.lifecycleState !== 'ACTIVE'"
                    @click="restartSystem(row)"
                  >
                    重启
                  </el-button>
                  <el-popconfirm title="确定删除此 DB System？所有数据将被清除。" @confirm="deleteSystem(row)">
                    <template #reference>
                      <el-button size="small" type="danger">删除</el-button>
                    </template>
                  </el-popconfirm>
                </el-button-group>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- Backups tab -->
      <el-tab-pane label="备份管理" name="backups">
        <div class="tab-toolbar">
          <el-select
            v-model="backupSystemId"
            placeholder="选择 DB System"
            filterable
            style="width: 300px"
            @change="loadBackups"
          >
            <el-option
              v-for="s in systems"
              :key="s.id"
              :label="s.displayName"
              :value="s.id"
            />
          </el-select>
          <el-button type="primary" size="small" @click="openCreateBackup" :disabled="!backupSystemId">
            创建备份
          </el-button>
          <el-button size="small" @click="loadBackups" :loading="backupsLoading" :disabled="!backupSystemId">
            刷新
          </el-button>
        </div>
        <el-card shadow="none" class="table-card">
          <el-table :data="backups" v-loading="backupsLoading" border stripe style="width: 100%">
            <template #empty>
              <el-empty description="暂无备份" :image-size="80" />
            </template>
            <el-table-column type="index" label="#" width="50" align="center" />
            <el-table-column prop="displayName" label="名称" min-width="160" />
            <el-table-column label="状态" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="stateTagType(row.lifecycleState)" size="small">
                  {{ row.lifecycleState || '-' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="backupType" label="类型" width="100" align="center">
              <template #default="{ row }">
                <el-tag size="small" effect="plain">{{ row.backupType || '-' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="大小" width="100" align="right">
              <template #default="{ row }">
                <span class="data-mono">{{ formatBytes(row.sizeInBytes) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="创建时间" width="180">
              <template #default="{ row }">
                <span class="data-mono" style="font-size:12px">{{ formatTime(row.timeCreated) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="100" fixed="right" align="center">
              <template #default="{ row }">
                <el-popconfirm title="确定删除此备份？" @confirm="deleteBackup(row)">
                  <template #reference>
                    <el-button size="small" type="danger">删除</el-button>
                  </template>
                </el-popconfirm>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- Channels tab -->
      <el-tab-pane label="复制通道" name="channels">
        <div class="tab-toolbar">
          <el-button size="small" @click="loadChannels" :loading="channelsLoading">
            刷新
          </el-button>
        </div>
        <el-card shadow="none" class="table-card">
          <el-table :data="channels" v-loading="channelsLoading" border stripe style="width: 100%">
            <template #empty>
              <el-empty description="暂无复制通道" :image-size="80" />
            </template>
            <el-table-column type="index" label="#" width="50" align="center" />
            <el-table-column prop="displayName" label="名称" min-width="160" />
            <el-table-column prop="sourceDisplayName" label="源" min-width="140" show-overflow-tooltip />
            <el-table-column prop="targetDisplayName" label="目标" min-width="140" show-overflow-tooltip />
            <el-table-column label="状态" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="stateTagType(row.lifecycleState)" size="small">
                  {{ row.lifecycleState || '-' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="100" fixed="right" align="center">
              <template #default="{ row }">
                <el-popconfirm title="确定删除此复制通道？" @confirm="deleteChannel(row)">
                  <template #reference>
                    <el-button size="small" type="danger">删除</el-button>
                  </template>
                </el-popconfirm>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- ================================================================ -->
    <!-- Create DB System Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="createSystemVisible" title="创建 DB System" width="600px" destroy-on-close>
      <el-form :model="createSystemForm" label-width="140px">
        <el-form-item label="名称" required>
          <el-input v-model="createSystemForm.displayName" placeholder="my-mysql" />
        </el-form-item>
        <el-form-item label="Shape" required>
          <el-select v-model="createSystemForm.shapeName" style="width:100%">
            <el-option label="MySQL.VM.Standard.E3.1.8GB" value="MySQL.VM.Standard.E3.1.8GB" />
            <el-option label="MySQL.VM.Standard.E3.2.16GB" value="MySQL.VM.Standard.E3.2.16GB" />
            <el-option label="MySQL.VM.Standard.E3.4.32GB" value="MySQL.VM.Standard.E3.4.32GB" />
            <el-option label="MySQL.VM.Standard.E3.8.64GB" value="MySQL.VM.Standard.E3.8.64GB" />
            <el-option label="MySQL.VM.Standard.E4.1.8GB" value="MySQL.VM.Standard.E4.1.8GB" />
            <el-option label="MySQL.VM.Standard.E4.2.16GB" value="MySQL.VM.Standard.E4.2.16GB" />
            <el-option label="MySQL.VM.Standard.E4.4.32GB" value="MySQL.VM.Standard.E4.4.32GB" />
            <el-option label="MySQL.VM.Standard.E4.8.64GB" value="MySQL.VM.Standard.E4.8.64GB" />
          </el-select>
        </el-form-item>
        <el-form-item label="MySQL 版本" required>
          <el-select v-model="createSystemForm.mysqlVersion" style="width:100%">
            <el-option label="8.0" value="8.0" />
            <el-option label="8.4" value="8.4" />
            <el-option label="9.0" value="9.0" />
          </el-select>
        </el-form-item>
        <el-form-item label="管理员用户名" required>
          <el-input v-model="createSystemForm.adminUsername" placeholder="admin" />
        </el-form-item>
        <el-form-item label="管理员密码" required>
          <el-input v-model="createSystemForm.adminPassword" type="password" show-password placeholder="设置管理员密码" />
        </el-form-item>
        <el-form-item label="子网 OCID" required>
          <el-input v-model="createSystemForm.subnetId" placeholder="ocid1.subnet..." />
        </el-form-item>
        <el-form-item label="可用域">
          <el-input v-model="createSystemForm.availabilityDomain" placeholder="留空使用默认可用域" />
        </el-form-item>
        <el-form-item label="数据存储(GB)">
          <el-input-number v-model="createSystemForm.dataStorageSizeInGBs" :min="50" :max="32768" :step="50" controls-position="right" style="width:200px" />
        </el-form-item>
        <el-form-item label="主机名">
          <el-input v-model="createSystemForm.hostname" placeholder="mysql-host" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createSystemVisible = false">取消</el-button>
        <el-button type="primary" :loading="createSystemSaving" @click="doCreateSystem">创建</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- System Detail Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="detailVisible" title="DB System 详情" width="700px" destroy-on-close>
      <template v-if="detailSystem">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="名称" :span="2">
            <span style="font-weight:var(--font-semibold)">{{ detailSystem.displayName }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="OCID" :span="2">
            <span class="data-mono" style="font-size:11px">{{ detailSystem.id }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="Shape">{{ detailSystem.shapeName }}</el-descriptions-item>
          <el-descriptions-item label="MySQL 版本">{{ detailSystem.mysqlVersion }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="stateTagType(detailSystem.lifecycleState)" size="small">
              {{ detailSystem.lifecycleState }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="可用域">{{ detailSystem.availabilityDomain || '-' }}</el-descriptions-item>
          <el-descriptions-item label="数据存储">{{ detailSystem.dataStorageSizeInGBs || '-' }} GB</el-descriptions-item>
          <el-descriptions-item label="主机名">{{ detailSystem.hostname || '-' }}</el-descriptions-item>
          <el-descriptions-item label="IP 地址">{{ detailSystem.ipAddress || '-' }}</el-descriptions-item>
          <el-descriptions-item label="端口">{{ detailSystem.port || 3306 }}</el-descriptions-item>
          <el-descriptions-item label="创建时间" :span="2">{{ formatTime(detailSystem.timeCreated) }}</el-descriptions-item>
        </el-descriptions>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Create Backup Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="createBackupVisible" title="创建备份" width="480px" destroy-on-close>
      <el-form :model="createBackupForm" label-width="100px">
        <el-form-item label="备份名称" required>
          <el-input v-model="createBackupForm.displayName" placeholder="my-backup" />
        </el-form-item>
        <el-form-item label="备份类型">
          <el-select v-model="createBackupForm.backupType" style="width:100%">
            <el-option label="增量备份" value="INCREMENTAL" />
            <el-option label="全量备份" value="FULL" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createBackupVisible = false">取消</el-button>
        <el-button type="primary" :loading="createBackupSaving" @click="doCreateBackup">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh } from '@element-plus/icons-vue'
import request from '../utils/request'

// ---- Types ----
interface MysqlSystem {
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

interface MysqlBackup {
  id: string
  displayName: string
  lifecycleState: string
  backupType: string
  sizeInBytes: number
  timeCreated: string
}

interface MysqlChannel {
  id: string
  displayName: string
  sourceDisplayName: string
  targetDisplayName: string
  lifecycleState: string
}

// ---- State ----
const tenantId = ref<number | null>(null)
const tenantOptions = ref<Array<{ id: number; name: string }>>([])
const activeTab = ref('systems')

// DB Systems
const systems = ref<MysqlSystem[]>([])
const loading = ref(false)

// Backups
const backupSystemId = ref('')
const backups = ref<MysqlBackup[]>([])
const backupsLoading = ref(false)

// Channels
const channels = ref<MysqlChannel[]>([])
const channelsLoading = ref(false)

// Create DB System
const createSystemVisible = ref(false)
const createSystemSaving = ref(false)
const createSystemForm = ref({
  displayName: '',
  shapeName: 'MySQL.VM.Standard.E4.2.16GB',
  mysqlVersion: '8.0',
  adminUsername: 'admin',
  adminPassword: '',
  subnetId: '',
  availabilityDomain: '',
  dataStorageSizeInGBs: 50,
  hostname: '',
})

// Detail
const detailVisible = ref(false)
const detailSystem = ref<MysqlSystem | null>(null)

// Create Backup
const createBackupVisible = ref(false)
const createBackupSaving = ref(false)
const createBackupForm = ref({
  displayName: '',
  backupType: 'INCREMENTAL',
})

// ---- Helpers ----
function formatBytes(bytes: number | undefined): string {
  if (!bytes || isNaN(bytes)) return '0 B'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1073741824) return (bytes / 1048576).toFixed(1) + ' MB'
  return (bytes / 1073741824).toFixed(2) + ' GB'
}

function formatTime(t: string | undefined): string {
  if (!t) return '-'
  try { return new Date(t).toLocaleString('zh-CN') } catch { return t }
}

function stateTagType(state: string): '' | 'success' | 'warning' | 'danger' | 'info' {
  const s = (state || '').toLowerCase()
  if (s === 'active') return 'success'
  if (s === 'creating' || s === 'updating' || s === 'starting' || s === 'stopping') return 'warning'
  if (s === 'deleted' || s === 'failed' || s === 'inactive') return 'danger'
  return 'info'
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
  systems.value = []
  backups.value = []
  channels.value = []
  if (tenantId.value) {
    loadSystems()
    loadChannels()
  }
}

// ---- DB System operations ----
async function loadSystems() {
  if (!tenantId.value) return
  loading.value = true
  try {
    const res = await request.get('/oci/mysql/systems', { params: { tenantId: tenantId.value } }) as any
    systems.value = Array.isArray(res) ? res : (res?.items || [])
  } catch (e: any) {
    ElMessage.error('加载 DB System 失败: ' + (e?.message || e))
    systems.value = []
  } finally {
    loading.value = false
  }
}

function showSystemDetail(system: MysqlSystem) {
  detailSystem.value = system
  detailVisible.value = true
}

function openCreateSystem() {
  createSystemForm.value = {
    displayName: '',
    shapeName: 'MySQL.VM.Standard.E4.2.16GB',
    mysqlVersion: '8.0',
    adminUsername: 'admin',
    adminPassword: '',
    subnetId: '',
    availabilityDomain: '',
    dataStorageSizeInGBs: 50,
    hostname: '',
  }
  createSystemVisible.value = true
}

async function doCreateSystem() {
  if (!createSystemForm.value.displayName.trim()) {
    ElMessage.warning('请输入名称')
    return
  }
  if (!createSystemForm.value.subnetId.trim()) {
    ElMessage.warning('请输入子网 OCID')
    return
  }
  if (!createSystemForm.value.adminPassword) {
    ElMessage.warning('请设置管理员密码')
    return
  }
  if (!tenantId.value) {
    ElMessage.warning('请先选择租户')
    return
  }
  createSystemSaving.value = true
  try {
    await request.post('/oci/mysql/system/create', {
      tenantId: tenantId.value,
      ...createSystemForm.value,
    })
    ElMessage.success('DB System 创建请求已发送')
    createSystemVisible.value = false
    await loadSystems()
  } catch (e: any) {
    ElMessage.error('创建失败: ' + (e?.message || e))
  } finally {
    createSystemSaving.value = false
  }
}

async function startSystem(system: MysqlSystem) {
  try {
    await ElMessageBox.confirm(`确定启动 ${system.displayName}？`, '确认启动')
    await request.post('/oci/mysql/system/start', { tenantId: tenantId.value, systemId: system.id })
    ElMessage.success('启动请求已发送')
    await loadSystems()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

async function stopSystem(system: MysqlSystem) {
  try {
    await ElMessageBox.confirm(`确定停止 ${system.displayName}？`, '确认停止', { type: 'warning' })
    await request.post('/oci/mysql/system/stop', { tenantId: tenantId.value, systemId: system.id })
    ElMessage.success('停止请求已发送')
    await loadSystems()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

async function restartSystem(system: MysqlSystem) {
  try {
    await ElMessageBox.confirm(`确定重启 ${system.displayName}？`, '确认重启')
    await request.post('/oci/mysql/system/restart', { tenantId: tenantId.value, systemId: system.id })
    ElMessage.success('重启请求已发送')
    await loadSystems()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

async function deleteSystem(system: MysqlSystem) {
  try {
    await ElMessageBox.confirm(
      `确定删除 DB System「${system.displayName}」？此操作不可逆，所有数据将被清除！`,
      '确认删除',
      { type: 'error', confirmButtonText: '确认删除', confirmButtonClass: 'el-button--danger' }
    )
    await request.delete('/oci/mysql/system/delete', { data: { tenantId: tenantId.value, systemId: system.id } })
    ElMessage.success('删除请求已发送')
    await loadSystems()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// ---- Backups ----
async function loadBackups() {
  if (!backupSystemId.value || !tenantId.value) return
  backupsLoading.value = true
  try {
    const res = await request.get('/oci/mysql/backups', {
      params: { tenantId: tenantId.value, systemId: backupSystemId.value },
    }) as any
    backups.value = Array.isArray(res) ? res : (res?.items || [])
  } catch (e: any) {
    ElMessage.error('加载备份失败: ' + (e?.message || e))
    backups.value = []
  } finally {
    backupsLoading.value = false
  }
}

function openCreateBackup() {
  createBackupForm.value = { displayName: '', backupType: 'INCREMENTAL' }
  createBackupVisible.value = true
}

async function doCreateBackup() {
  if (!createBackupForm.value.displayName.trim()) {
    ElMessage.warning('请输入备份名称')
    return
  }
  if (!backupSystemId.value || !tenantId.value) return
  createBackupSaving.value = true
  try {
    await request.post('/oci/mysql/backup/create', {
      tenantId: tenantId.value,
      systemId: backupSystemId.value,
      displayName: createBackupForm.value.displayName.trim(),
      backupType: createBackupForm.value.backupType,
    })
    ElMessage.success('备份创建请求已发送')
    createBackupVisible.value = false
    await loadBackups()
  } catch (e: any) {
    ElMessage.error('创建备份失败: ' + (e?.message || e))
  } finally {
    createBackupSaving.value = false
  }
}

async function deleteBackup(backup: MysqlBackup) {
  try {
    await request.delete('/oci/mysql/backup/delete', {
      data: { tenantId: tenantId.value, backupId: backup.id },
    })
    ElMessage.success('备份已删除')
    await loadBackups()
  } catch (e: any) {
    ElMessage.error('删除失败: ' + (e?.message || e))
  }
}

// ---- Channels ----
async function loadChannels() {
  if (!tenantId.value) return
  channelsLoading.value = true
  try {
    const res = await request.get('/oci/mysql/channels', { params: { tenantId: tenantId.value } }) as any
    channels.value = Array.isArray(res) ? res : (res?.items || [])
  } catch (e: any) {
    ElMessage.error('加载复制通道失败: ' + (e?.message || e))
    channels.value = []
  } finally {
    channelsLoading.value = false
  }
}

async function deleteChannel(channel: MysqlChannel) {
  try {
    await ElMessageBox.confirm(`确定删除复制通道「${channel.displayName}」？`, '确认删除', { type: 'warning' })
    await request.delete('/oci/mysql/channel/delete', {
      data: { tenantId: tenantId.value, channelId: channel.id },
    })
    ElMessage.success('复制通道已删除')
    await loadChannels()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// ---- Init ----
onMounted(loadTenants)
</script>

<style scoped>
.mysql-page {
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

.tab-toolbar {
  display: flex;
  gap: var(--space-3);
  align-items: center;
  margin-bottom: var(--space-4);
  flex-wrap: wrap;
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

:deep(.el-descriptions) {
  background: transparent;
}

:deep(.el-descriptions__body) {
  background: transparent;
}

:deep(.el-tabs__item) {
  font-weight: var(--font-medium);
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

  .tab-toolbar {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
