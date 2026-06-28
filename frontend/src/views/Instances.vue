<template>
  <div class="instances-page">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>实例管理</h2>
        <el-tag type="info" size="small">{{ total }} 个实例</el-tag>
      </div>
      <div class="toolbar-right">
        <el-button @click="load" :loading="loading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
        <el-dropdown @command="handleExport">
          <el-button>
            <el-icon><Download /></el-icon> 导出 <el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="all">导出全部实例</el-dropdown-item>
              <el-dropdown-item command="current-tenant" v-if="tenantFilter">导出当前租户</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <!-- Filter bar -->
    <div class="filter-bar">
      <el-select
        v-model="tenantFilter"
        placeholder="按租户筛选"
        clearable
        filterable
        style="width: 200px"
        @change="onFilterChange"
      >
        <el-option
          v-for="t in tenantOptions"
          :key="t.id"
          :label="t.name"
          :value="t.id"
        />
      </el-select>
      <el-select
        v-model="stateFilter"
        placeholder="按状态筛选"
        clearable
        style="width: 150px"
        @change="onFilterChange"
      >
        <el-option label="Running" value="Running" />
        <el-option label="Stopped" value="Stopped" />
        <el-option label="Starting" value="Starting" />
        <el-option label="Stopping" value="Stopping" />
        <el-option label="Terminated" value="Terminated" />
      </el-select>
      <el-input
        v-model="searchText"
        placeholder="搜索名称 / IP / ID..."
        clearable
        style="width: 260px"
        @input="onFilterChange"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
    </div>

    <!-- Table -->
    <el-card shadow="none" class="table-card">
      <el-table
        :data="rows"
        v-loading="loading"
        border
        stripe
        style="width: 100%"
        @selection-change="onSelectionChange"
      >
        <template #empty>
          <el-empty description="暂无实例数据" :image-size="80" />
        </template>
        <el-table-column type="selection" width="40" />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <div class="state-cell">
              <span class="status-dot" :class="stateDotClass(row.state)"></span>
              <span class="state-text">{{ row.state || '-' }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="displayName" label="名称" min-width="160" sortable>
          <template #default="{ row }">
            <span class="instance-name" @click="showDetail(row)" style="cursor:pointer;color:var(--accent)">
              {{ row.displayName }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="tenantName" label="租户" min-width="120" />
        <el-table-column prop="instanceId" label="实例ID" min-width="200" show-overflow-tooltip />
        <el-table-column prop="shape" label="Shape" min-width="140" show-overflow-tooltip />
        <el-table-column prop="publicIps" label="公网IP" width="140">
          <template #default="{ row }">
            <span class="data-mono" style="font-size:var(--text-xs)">{{ row.publicIps || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="规格" width="140">
          <template #default="{ row }">
            <span class="data-mono">{{ row.ocpus }}C / {{ row.memoryInGbs }}G</span>
          </template>
        </el-table-column>
        <el-table-column prop="architecture" label="架构" width="70" align="center">
          <template #default="{ row }">
            <el-tag :type="row.architecture?.startsWith('ARM') ? '' : 'warning'" size="small" effect="dark">
              {{ row.architecture || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="在线" width="70" align="center">
          <template #default="{ row }">
            <span class="status-dot" :class="row.onLineEnable ? 'status-dot--up' : 'status-dot--idle'"></span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button-group>
              <el-button size="small" @click="showDetail(row)" title="详情">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
              <el-dropdown trigger="click" @command="(cmd: string) => handleAction(cmd, row)">
                <el-button size="small">
                  操作 <el-icon class="el-icon--right"><ArrowDown /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="start" :disabled="row.state === 'Running'">
                      <el-icon><VideoPlay /></el-icon> 启动
                    </el-dropdown-item>
                    <el-dropdown-item command="stop" :disabled="row.state === 'Stopped'">
                      <el-icon><VideoPause /></el-icon> 停止
                    </el-dropdown-item>
                    <el-dropdown-item command="modify">
                      <el-icon><Edit /></el-icon> 修改配置
                    </el-dropdown-item>
                    <el-dropdown-item command="change-ip">
                      <el-icon><Connection /></el-icon> 更换IP
                    </el-dropdown-item>
                    <el-dropdown-item command="enable-ipv6" :disabled="!!row.ipv6Addresses">
                      <el-icon><Link /></el-icon> 启用 IPv6
                    </el-dropdown-item>
                    <el-dropdown-item command="ssh-config">
                      <el-icon><Key /></el-icon> SSH 配置
                    </el-dropdown-item>
                    <el-dropdown-item command="rescue">
                      <el-icon><Warning /></el-icon> 救援模式
                    </el-dropdown-item>
                    <el-dropdown-item command="console">
                      <el-icon><Monitor /></el-icon> VNC 控制台
                    </el-dropdown-item>
                    <el-dropdown-item command="terminal">
                      <el-icon><Operation /></el-icon> SSH 终端
                    </el-dropdown-item>
                    <el-divider style="margin:4px 0" />
                    <el-dropdown-item command="terminate" divided>
                      <span style="color:var(--status-down)">
                        <el-icon><Delete /></el-icon> 终止实例
                      </span>
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Batch operations -->
    <div v-if="selectedIds.length > 0" class="batch-bar">
      <span>已选择 {{ selectedIds.length }} 个实例</span>
      <el-button size="small" @click="batchStart">批量启动</el-button>
      <el-button size="small" @click="batchStop">批量停止</el-button>
      <el-button size="small" @click="selectedIds = []">取消选择</el-button>
    </div>

    <!-- Pagination -->
    <el-pagination
      v-if="total > pageSize"
      v-model:current-page="page"
      v-model:page-size="pageSize"
      :total="total"
      :page-sizes="[20, 50, 100]"
      layout="total, sizes, prev, pager, next"
      @change="load"
      style="margin-top: var(--space-5); justify-content: center"
    />

    <!-- ================================================================ -->
    <!-- Detail Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="detailVisible" title="实例详情" width="780px" destroy-on-close @opened="onDetailOpened">
      <template v-if="detail">
        <el-tabs v-model="detailTab">
          <el-tab-pane label="基本信息" name="info">
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="显示名称" :span="2">
                <span style="font-weight:var(--font-semibold)">{{ detail.displayName }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="实例ID" :span="2">{{ detail.instanceId }}</el-descriptions-item>
              <el-descriptions-item label="租户">{{ detail.tenantName || '-' }}</el-descriptions-item>
              <el-descriptions-item label="状态">
                <el-tag :type="stateTagType(detail.state)" size="small">
                  {{ detail.state }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="Shape">{{ detail.shape }}</el-descriptions-item>
              <el-descriptions-item label="架构">{{ detail.architecture }}</el-descriptions-item>
              <el-descriptions-item label="OCPU">{{ detail.ocpus }}</el-descriptions-item>
              <el-descriptions-item label="内存(GB)">{{ detail.memoryInGbs }}</el-descriptions-item>
              <el-descriptions-item label="启动卷(GB)">{{ detail.bootVolumeSizeInGbs }}</el-descriptions-item>
              <el-descriptions-item label="VPU/GB">{{ detail.vpusPerGb || '-' }}</el-descriptions-item>
              <el-descriptions-item label="公网IP">{{ detail.publicIps || '-' }}</el-descriptions-item>
              <el-descriptions-item label="私网IP">{{ detail.privateIps || '-' }}</el-descriptions-item>
              <el-descriptions-item label="IPv6">{{ detail.ipv6Addresses || '-' }}</el-descriptions-item>
              <el-descriptions-item label="可用域">{{ detail.availabilityDomain }}</el-descriptions-item>
              <el-descriptions-item label="启动卷ID" :span="2">{{ detail.bootVolumeId || '-' }}</el-descriptions-item>
              <el-descriptions-item label="VNIC IDs" :span="2">{{ detail.vnicIds || '-' }}</el-descriptions-item>
              <el-descriptions-item label="最后心跳">{{ detail.lastHeartbeat || '-' }}</el-descriptions-item>
              <el-descriptions-item label="创建时间">{{ detail.createTime || '-' }}</el-descriptions-item>
            </el-descriptions>
            <!-- Quick actions -->
            <div style="margin-top:var(--space-4);display:flex;gap:var(--space-3);flex-wrap:wrap">
              <el-button size="small" @click="handleAction('rescue', detail)">救援模式</el-button>
              <el-button size="small" @click="handleAction('console', detail)">VNC 控制台</el-button>
              <el-button size="small" @click="handleAction('terminal', detail)">SSH 终端</el-button>
              <el-button size="small" type="warning" @click="handleAction('modify', detail)">修改配置</el-button>
            </div>
          </el-tab-pane>

          <el-tab-pane label="备注" name="remark">
            <el-input
              v-model="remarkText"
              type="textarea"
              :rows="4"
              placeholder="添加备注信息..."
            />
            <el-button type="primary" size="small" style="margin-top:12px" @click="saveDetailRemark">
              保存备注
            </el-button>
          </el-tab-pane>

          <el-tab-pane label="流量统计" name="traffic" v-if="trafficData">
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="统计日期">{{ trafficData.statsDate || '-' }}</el-descriptions-item>
              <el-descriptions-item label="区域">{{ trafficData.region || '-' }}</el-descriptions-item>
              <el-descriptions-item label="入站流量">
                <span class="data-mono">{{ formatBytes(trafficData.ingressBytes) }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="出站流量">
                <span class="data-mono">{{ formatBytes(trafficData.egressBytes) }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="流量阈值">
                <span class="data-mono">{{ trafficData.threshold ? formatBytes(trafficData.threshold) : '未设置' }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="自动关机">
                <el-tag :type="trafficData.autoShutdown ? 'warning' : 'info'" size="small">
                  {{ trafficData.autoShutdown ? '启用' : '禁用' }}
                </el-tag>
              </el-descriptions-item>
            </el-descriptions>
          </el-tab-pane>

          <el-tab-pane label="备份记录" name="backups">
            <div v-if="backups.length === 0" style="text-align:center;padding:24px;color:var(--text-muted)">
              暂无备份记录
            </div>
            <el-table v-else :data="backups" size="small" border>
              <el-table-column prop="displayName" label="名称" min-width="140" />
              <el-table-column prop="shape" label="Shape" min-width="120" />
              <el-table-column prop="state" label="状态" width="80" />
              <el-table-column prop="publicIps" label="公网IP" width="130" />
              <el-table-column label="操作" width="80">
                <template #default="{ row: bk }">
                  <el-popconfirm title="确定删除此备份记录？" @confirm="deleteBackup(bk.id)">
                    <template #reference>
                      <el-button size="small" type="danger" link>删除</el-button>
                    </template>
                  </el-popconfirm>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>

          <el-tab-pane label="SSH 配置" name="ssh">
            <el-form :model="sshForm" label-width="100px" size="small">
              <el-form-item label="用户名">
                <el-input v-model="sshForm.username" placeholder="root" />
              </el-form-item>
              <el-form-item label="端口">
                <el-input-number v-model="sshForm.port" :min="1" :max="65535" style="width:100%" />
              </el-form-item>
              <el-form-item label="密码">
                <el-input v-model="sshForm.password" type="password" show-password placeholder="SSH 密码" />
              </el-form-item>
              <el-form-item>
                <el-button type="primary" :loading="sshSaving" @click="saveSshConfig">
                  保存 SSH 配置
                </el-button>
              </el-form-item>
            </el-form>
          </el-tab-pane>
        </el-tabs>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Modify Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="modifyVisible" title="修改实例配置" width="520px" destroy-on-close>
      <el-alert
        title="提示：修改 Shape 或资源规格可能需要先停止实例"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom:16px"
      />
      <el-form :model="modifyForm" label-width="100px">
        <el-form-item label="当前 Shape">
          <el-input :model-value="modifyTarget?.shape" disabled />
        </el-form-item>
        <el-form-item label="显示名称">
          <el-input v-model="modifyForm.displayName" placeholder="修改实例显示名称" />
        </el-form-item>
        <el-form-item label="新 Shape">
          <el-input v-model="modifyForm.shape" placeholder="例如: VM.Standard.E4.Flex" />
        </el-form-item>
        <el-form-item label="OCPU">
          <el-input-number v-model="modifyForm.ocpus" :min="1" :max="128" :step="1" controls-position="right" style="width:100%" />
        </el-form-item>
        <el-form-item label="内存(GB)">
          <el-input-number v-model="modifyForm.memoryInGbs" :min="1" :max="1024" :step="1" controls-position="right" style="width:100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="modifyVisible = false">取消</el-button>
        <el-button type="primary" :loading="modifySaving" @click="doModify">确认修改</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Change IP Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="changeIpVisible" title="更换公网 IP" width="460px" destroy-on-close>
      <el-alert
        title="更换 IP 会释放当前公网 IP 并分配新的临时公网 IP"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom:16px"
      />
      <el-descriptions :column="1" border>
        <el-descriptions-item label="实例">{{ changeIpTarget?.displayName }}</el-descriptions-item>
        <el-descriptions-item label="当前 IP">{{ changeIpTarget?.publicIps || '-' }}</el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button @click="changeIpVisible = false">取消</el-button>
        <el-button type="primary" :loading="changeIpLoading" @click="doChangeIp">确认更换</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Enable IPv6 Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="ipv6Visible" title="启用 IPv6" width="460px" destroy-on-close>
      <el-descriptions :column="1" border>
        <el-descriptions-item label="实例">{{ ipv6Target?.displayName }}</el-descriptions-item>
        <el-descriptions-item label="当前 IPv6">{{ ipv6Target?.ipv6Addresses || '未启用' }}</el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button @click="ipv6Visible = false">取消</el-button>
        <el-button type="primary" :loading="ipv6Loading" @click="doEnableIpv6">确认启用</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Refresh, Download, ArrowDown, Search, InfoFilled,
  VideoPlay, VideoPause, Edit, Connection, Link, Key,
  Warning, Monitor, Operation, Delete,
} from '@element-plus/icons-vue'
import request from '../utils/request'

// ---- Types ----
interface Instance {
  id: number; tenantId: number; tenantName: string
  instanceId: string; displayName: string; shape: string; state: string
  ocpus: number; memoryInGbs: number; bootVolumeSizeInGbs: number
  publicIps: string; privateIps: string; availabilityDomain: string
  compartmentId: string; bootVolumeId: string; bootVolumeName: string
  vpusPerGb: string; ipv6Addresses: string; vnicIds: string
  architecture: string; onLineEnable: number; lastHeartbeat: string; createTime: string
}

// ---- State ----
const router = useRouter()
const rows = ref<Instance[]>([])
const allInstances = ref<Instance[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const selectedIds = ref<number[]>([])
const tenantOptions = ref<Array<{id:number;name:string}>>([])

// Filters
const tenantFilter = ref<number | null>(null)
const stateFilter = ref('')
const searchText = ref('')

// Detail dialog
const detailVisible = ref(false)
const detail = ref<Instance | null>(null)
const detailTab = ref('info')
const remarkText = ref('')

// Traffic
const trafficData = ref<any>(null)

// Backups
const backups = ref<any[]>([])

// SSH config
const sshForm = ref({ username: 'root', port: 22, password: '' })
const sshSaving = ref(false)

// Modify
const modifyVisible = ref(false)
const modifySaving = ref(false)
const modifyTarget = ref<Instance | null>(null)
const modifyForm = ref({ shape: '', ocpus: 4, memoryInGbs: 24, displayName: '' })

// Change IP
const changeIpVisible = ref(false)
const changeIpLoading = ref(false)
const changeIpTarget = ref<Instance | null>(null)

// IPv6
const ipv6Visible = ref(false)
const ipv6Loading = ref(false)
const ipv6Target = ref<Instance | null>(null)

// ---- Computed ----
function stateDotClass(state: string): string {
  const s = (state || '').toLowerCase()
  if (s === 'running') return 'status-dot--up status-dot--pulse'
  if (s === 'stopped' || s === 'terminated') return 'status-dot--down'
  if (s === 'starting' || s === 'stopping') return 'status-dot--warn'
  return 'status-dot--idle'
}

function stateTagType(state: string): string {
  const s = (state || '').toLowerCase()
  if (s === 'running') return 'success'
  if (s === 'stopped' || s === 'terminated') return 'danger'
  if (s === 'starting' || s === 'stopping') return 'warning'
  return 'info'
}

function formatBytes(bytes: number | undefined): string {
  if (!bytes && bytes !== 0) return '-'
  const gb = bytes / (1024 * 1024 * 1024)
  if (gb >= 1) return gb.toFixed(2) + ' GB'
  const mb = bytes / (1024 * 1024)
  if (mb >= 1) return mb.toFixed(2) + ' MB'
  return bytes.toFixed(0) + ' B'
}

// ---- Data loading ----
async function load() {
  loading.value = true
  try {
    const offset = (page.value - 1) * pageSize.value
    const res = await request.get('/instances/list', { params: { limit: pageSize.value, offset } }) as any
    rows.value = res.items || []
    total.value = res.total || 0
    // Also load all instances for export / tenant list
    if (allInstances.value.length === 0) {
      loadAllInstances()
      loadTenants()
    }
  } catch (e: any) { ElMessage.error(e.message) }
  finally { loading.value = false }
}

async function loadAllInstances() {
  try {
    const res = await request.get('/instances/list', { params: { limit: 9999, offset: 0 } }) as any
    allInstances.value = res.items || []
  } catch { /* ignore */ }
}

async function loadTenants() {
  try {
    const tenants = await request.get('/tenants/listAll') as any[]
    tenantOptions.value = tenants.map((t: any) => ({ id: t.id, name: t.userName || t.tenancy || `#${t.id}` }))
  } catch { /* ignore */ }
}

function onFilterChange() {
  load()
}

function onSelectionChange(selection: Instance[]) {
  selectedIds.value = selection.map(s => s.id)
}

// ---- Detail ----
function showDetail(row: Instance) {
  detail.value = { ...row }
  detailTab.value = 'info'
  remarkText.value = ''
  detailVisible.value = true
}

async function onDetailOpened() {
  if (!detail.value) return
  // Load traffic
  try {
    trafficData.value = await request.get('/instances/traffic', { params: { tenantId: detail.value.tenantId } })
  } catch { trafficData.value = null }
  // Load backups
  try {
    backups.value = await request.get('/backup/list', { params: { tenantId: detail.value.tenantId } }) as any[]
  } catch { backups.value = [] }
  // Load SSH config
  try {
    const ssh = await request.get(`/instances/${detail.value.id}/ssh-config`) as any
    sshForm.value = {
      username: ssh.username || 'root',
      port: ssh.port || 22,
      password: '',
    }
  } catch {
    sshForm.value = { username: 'root', port: 22, password: '' }
  }
}

async function saveDetailRemark() {
  if (!detail.value) return
  try {
    await request.post(`/instances/${detail.value.id}/remark`, { remark: remarkText.value })
    ElMessage.success('备注已更新')
  } catch (e: any) { ElMessage.error(e.message) }
}

async function deleteBackup(id: number) {
  try {
    await request.get('/backup/delete', { params: { id } })
    ElMessage.success('备份已删除')
    backups.value = backups.value.filter(b => b.id !== id)
  } catch (e: any) { ElMessage.error(e.message) }
}

// ---- Actions ----
function handleAction(cmd: string, row: Instance) {
  switch (cmd) {
    case 'start': return confirmStart(row)
    case 'stop': return confirmStop(row)
    case 'modify': return openModify(row)
    case 'change-ip': return openChangeIp(row)
    case 'enable-ipv6': return openEnableIpv6(row)
    case 'ssh-config':
      detail.value = row
      detailTab.value = 'ssh'
      detailVisible.value = true
      onDetailOpened()
      return
    case 'rescue':
      router.push({ path: '/rescue', query: { instanceId: row.instanceId } })
      return
    case 'console':
      router.push({ path: '/console', query: { instanceId: row.instanceId } })
      return
    case 'terminal':
      router.push({ path: '/terminal', query: { host: row.publicIps || '' } })
      return
    case 'terminate': return confirmTerminate(row)
  }
}

async function confirmStart(row: Instance) {
  try {
    await ElMessageBox.confirm(`确定启动实例 ${row.displayName}？`, '确认启动', { type: 'info' })
    await request.post(`/instances/${row.id}/start`)
    ElMessage.success('启动请求已发送')
    await load()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

async function confirmStop(row: Instance) {
  try {
    await ElMessageBox.confirm(`确定停止实例 ${row.displayName}？`, '确认停止', { type: 'warning' })
    await request.post(`/instances/${row.id}/stop`)
    ElMessage.success('停止请求已发送')
    await load()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

async function confirmTerminate(row: Instance) {
  try {
    await ElMessageBox.confirm(
      `确定终止实例 ${row.displayName}？此操作不可逆，将删除 OCI 实例及本地记录！`,
      '确认终止',
      { type: 'error', confirmButtonText: '确认终止', confirmButtonClass: 'el-button--danger' }
    )
    await request.post(`/instances/${row.id}/terminate`, { preserveBootVolume: false })
    ElMessage.success('终止请求已发送')
    await load()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// ---- Modify ----
function openModify(row: Instance) {
  modifyTarget.value = row
  modifyForm.value = {
    shape: row.shape || '',
    ocpus: row.ocpus || 4,
    memoryInGbs: row.memoryInGbs || 24,
    displayName: row.displayName || '',
  }
  modifyVisible.value = true
}

async function doModify() {
  if (!modifyTarget.value) return
  modifySaving.value = true
  try {
    const body: any = {}
    if (modifyForm.value.shape && modifyForm.value.shape !== modifyTarget.value.shape) body.shape = modifyForm.value.shape
    if (modifyForm.value.ocpus !== modifyTarget.value.ocpus) body.ocpus = modifyForm.value.ocpus
    if (modifyForm.value.memoryInGbs !== modifyTarget.value.memoryInGbs) body.memoryInGbs = modifyForm.value.memoryInGbs
    if (modifyForm.value.displayName && modifyForm.value.displayName !== modifyTarget.value.displayName) body.displayName = modifyForm.value.displayName

    if (Object.keys(body).length === 0) {
      ElMessage.warning('没有需要修改的配置')
      modifySaving.value = false
      return
    }
    await request.post(`/instances/${modifyTarget.value.id}/modify`, body)
    ElMessage.success('修改请求已提交，实例正在更新中')
    modifyVisible.value = false
    await load()
  } catch (e: any) {
    ElMessage.error(e.message || '修改失败')
  } finally { modifySaving.value = false }
}

// ---- Change IP ----
function openChangeIp(row: Instance) {
  changeIpTarget.value = row
  changeIpVisible.value = true
}

async function doChangeIp() {
  if (!changeIpTarget.value) return
  changeIpLoading.value = true
  try {
    const res = await request.post(`/instances/${changeIpTarget.value.id}/change-ip`) as any
    ElMessage.success(`IP 已更换: ${res.oldIp} → ${res.newIp}`)
    changeIpVisible.value = false
    await load()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally { changeIpLoading.value = false }
}

// ---- Enable IPv6 ----
function openEnableIpv6(row: Instance) {
  ipv6Target.value = row
  ipv6Visible.value = true
}

async function doEnableIpv6() {
  if (!ipv6Target.value) return
  ipv6Loading.value = true
  try {
    const res = await request.post(`/instances/${ipv6Target.value.id}/enable-ipv6`)
    ElMessage.success('IPv6 已启用: ' + (res as any)?.ipv6Address)
    ipv6Visible.value = false
    await load()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally { ipv6Loading.value = false }
}

// ---- SSH Config ----
async function saveSshConfig() {
  if (!detail.value) return
  if (!sshForm.value.username) { ElMessage.warning('用户名不能为空'); return }
  sshSaving.value = true
  try {
    await request.post(`/instances/${detail.value.id}/ssh-config`, {
      username: sshForm.value.username,
      port: sshForm.value.port,
      password: sshForm.value.password,
    })
    ElMessage.success('SSH 配置已保存')
  } catch (e: any) { ElMessage.error(e.message) }
  finally { sshSaving.value = false }
}

// ---- Export ----
async function handleExport(cmd: string) {
  try {
    let url = '/instances/export'
    if (cmd === 'current-tenant' && tenantFilter.value) {
      url += `?tenantId=${tenantFilter.value}`
    }
    const a = document.createElement('a')
    a.href = url
    a.download = ''
    document.body.appendChild(a)
    a.click()
    a.remove()
    ElMessage.success('导出已开始')
  } catch (e: any) {
    ElMessage.error('导出失败')
  }
}

// ---- Batch operations ----
async function batchStart() {
  try {
    await ElMessageBox.confirm(`确定批量启动 ${selectedIds.value.length} 个实例？`, '确认', { type: 'info' })
    for (const id of selectedIds.value) {
      try { await request.post(`/instances/${id}/start`) } catch { /* continue */ }
    }
    ElMessage.success('批量启动请求已发送')
    await load()
  } catch { /* cancel */ }
}

async function batchStop() {
  try {
    await ElMessageBox.confirm(`确定批量停止 ${selectedIds.value.length} 个实例？`, '确认', { type: 'warning' })
    for (const id of selectedIds.value) {
      try { await request.post(`/instances/${id}/stop`) } catch { /* continue */ }
    }
    ElMessage.success('批量停止请求已发送')
    await load()
  } catch { /* cancel */ }
}

onMounted(load)
</script>

<style scoped>
.instances-page {
  padding: 0;
}

/* ---- Toolbar ---- */
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

/* ---- Filter bar ---- */
.filter-bar {
  display: flex;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
  flex-wrap: wrap;
}

/* ---- State cell ---- */
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

.instance-name:hover {
  text-decoration: underline;
}

/* ---- Table card ---- */
.table-card {
  border-radius: var(--radius-md);
  overflow: hidden;
}

.table-card :deep(.el-card__body) {
  padding: 0;
}

/* ---- Batch bar ---- */
.batch-bar {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  margin-top: var(--space-3);
  padding: var(--space-3) var(--space-4);
  background: var(--bg-surface);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  font-size: var(--text-sm);
  color: var(--text-secondary);
}

/* ---- Element Plus deep overrides ---- */
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

:deep(.el-pagination) {
  justify-content: center;
  margin-top: var(--space-5);
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

  .batch-bar {
    flex-wrap: wrap;
  }
}
</style>