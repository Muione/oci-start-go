<template>
  <div class="vnic-page">
    <!-- ================================================================ -->
    <!-- Toolbar -->
    <!-- ================================================================ -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>VNIC 管理</h2>
        <el-tag v-if="selectedInstance" type="info" size="small">
          {{ vnicData?.statistics?.totalVnicCount ?? 0 }} 个 VNIC
        </el-tag>
      </div>
      <div class="toolbar-right">
        <el-button @click="refreshVnics" :loading="loading" :disabled="!selectedInstance">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </div>
    </div>

    <!-- ================================================================ -->
    <!-- Instance Selector -->
    <!-- ================================================================ -->
    <el-card shadow="none" class="selector-card">
      <div class="selector-row">
        <div class="selector-item">
          <span class="selector-label">租户</span>
          <el-select
            v-model="selectedTenantId"
            placeholder="选择租户"
            filterable
            clearable
            style="width: 240px"
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
        <div class="selector-item">
          <span class="selector-label">实例</span>
          <el-select
            v-model="selectedInstanceId"
            placeholder="选择实例"
            filterable
            clearable
            style="width: 360px"
            :disabled="!selectedTenantId"
            @change="onInstanceChange"
          >
            <el-option
              v-for="inst in instanceOptions"
              :key="inst.instanceId"
              :label="`${inst.displayName} (${inst.state})`"
              :value="inst.instanceId"
            />
          </el-select>
        </div>
      </div>

      <!-- Instance info summary -->
      <div v-if="selectedInstance" class="instance-summary">
        <el-descriptions :column="4" border size="small">
          <el-descriptions-item label="名称">
            <span style="font-weight:var(--font-semibold)">{{ selectedInstance.displayName }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="实例 OCID">
            <span class="data-mono" style="font-size:var(--text-xs)">{{ selectedInstance.instanceId }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="区域">{{ selectedInstance.tenantName || '-' }}</el-descriptions-item>
          <el-descriptions-item label="Shape">{{ selectedInstance.shape || '-' }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <div class="state-cell">
              <span class="status-dot" :class="stateDotClass(selectedInstance.state)"></span>
              <span>{{ selectedInstance.state || '-' }}</span>
            </div>
          </el-descriptions-item>
          <el-descriptions-item label="公网 IP">
            <span class="data-mono">{{ selectedInstance.publicIps || '-' }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="私网 IP">
            <span class="data-mono">{{ selectedInstance.privateIps || '-' }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="架构">{{ selectedInstance.architecture || '-' }}</el-descriptions-item>
        </el-descriptions>
      </div>
    </el-card>

    <!-- ================================================================ -->
    <!-- VNIC Statistics -->
    <!-- ================================================================ -->
    <div v-if="vnicData" class="stats-row">
      <el-card shadow="none" class="stat-card">
        <el-statistic title="总 VNIC 数" :value="vnicData.statistics?.totalVnicCount ?? 0" />
      </el-card>
      <el-card shadow="none" class="stat-card">
        <el-statistic title="活跃 VNIC" :value="vnicData.statistics?.activeVnicCount ?? 0" />
      </el-card>
      <el-card shadow="none" class="stat-card">
        <el-statistic title="辅助 VNIC" :value="vnicData.statistics?.secondaryVnicCount ?? 0" />
      </el-card>
      <el-card shadow="none" class="stat-card">
        <el-statistic title="总 IPv6" :value="vnicData.statistics?.totalIpv6Count ?? 0" />
      </el-card>
    </div>

    <!-- ================================================================ -->
    <!-- Operations Toolbar -->
    <!-- ================================================================ -->
    <div v-if="selectedInstance" class="ops-toolbar">
      <el-button type="primary" @click="openBatchCreate" :loading="batchCreating">
        <el-icon><Plus /></el-icon> 批量创建 VNIC
      </el-button>
      <el-button type="danger" @click="confirmDeleteAllSecondary" :loading="deletingAll" :disabled="!vnicData?.secondaryVnics?.length">
        <el-icon><Delete /></el-icon> 删除所有辅助 VNIC
      </el-button>
      <el-button @click="openIpSwitch" :loading="ipSwitching">
        <el-icon><Switch /></el-icon> 切换IP
      </el-button>
      <el-button @click="openConfigureLB" :loading="configuringLB">
        <el-icon><Connection /></el-icon> 配置负载均衡
      </el-button>
      <el-button @click="confirmRestoreNetwork" :loading="restoringNetwork">
        <el-icon><RefreshRight /></el-icon> 恢复网络配置
      </el-button>
    </div>

    <!-- ================================================================ -->
    <!-- VNIC Table -->
    <!-- ================================================================ -->
    <el-card v-if="selectedInstance" shadow="none" class="table-card">
      <el-table
        :data="allVnics"
        v-loading="loading"
        border
        stripe
        style="width: 100%"
        row-key="vnicId"
      >
        <template #empty>
          <el-empty description="暂无 VNIC 数据" :image-size="80" />
        </template>
        <el-table-column type="index" label="#" width="50" align="center" />
        <el-table-column label="名称" min-width="180">
          <template #default="{ row }">
            <span class="vnic-name">{{ row.vnicDisplayName || '-' }}</span>
            <el-tag v-if="row.isPrimary" type="success" size="small" effect="dark" style="margin-left:6px">
              主 VNIC
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="私网 IP" width="140">
          <template #default="{ row }">
            <span class="data-mono" style="font-size:var(--text-xs)">{{ row.privateIp || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="公网 IP" width="140">
          <template #default="{ row }">
            <span class="data-mono" style="font-size:var(--text-xs)">{{ row.publicIp || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="子网" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="data-mono" style="font-size:var(--text-xs)">{{ row.subnetId || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="IPv6 地址" min-width="200">
          <template #default="{ row }">
            <div v-if="row.ipv6Addresses?.length" class="ipv6-list">
              <el-tag
                v-for="(addr, idx) in row.ipv6Addresses"
                :key="idx"
                size="small"
                type="info"
                class="ipv6-tag"
                closable
                @close="confirmDeleteIpv6(row, addr)"
              >
                {{ truncateIpv6(addr) }}
              </el-tag>
            </div>
            <span v-else class="text-muted">无 IPv6</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <div class="state-cell">
              <span class="status-dot" :class="lifecycleDotClass(row.lifecycleState)"></span>
              <span>{{ row.lifecycleState || '-' }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button-group>
              <el-button size="small" @click="openCreateIpv6(row)" title="创建 IPv6">
                <el-icon><Plus /></el-icon> IPv6
              </el-button>
              <el-button
                size="small"
                type="danger"
                @click="confirmDeleteVnic(row)"
                :disabled="row.isPrimary"
                title="删除 VNIC"
              >
                <el-icon><Delete /></el-icon>
              </el-button>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Empty state when no instance selected -->
    <el-card v-else shadow="none" class="empty-card">
      <el-empty description="请先选择租户和实例以管理 VNIC" :image-size="120" />
    </el-card>

    <!-- ================================================================ -->
    <!-- Batch Create Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="batchCreateVisible" title="批量创建 VNIC" width="520px" destroy-on-close>
      <el-alert
        title="将在选定实例上创建辅助 VNIC，每个 VNIC 可配置 IPv6 地址数量"
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom:16px"
      />
      <el-form :model="batchCreateForm" label-width="120px">
        <el-form-item label="VNIC 数量" required>
          <el-input-number
            v-model="batchCreateForm.vnicCount"
            :min="1"
            :max="32"
            :step="1"
            controls-position="right"
            style="width:100%"
          />
          <div class="form-hint">最多 32 个（含主 VNIC）</div>
        </el-form-item>
        <el-form-item label="目标子网" required>
          <el-select
            v-model="batchCreateForm.subnetId"
            placeholder="选择子网"
            filterable
            style="width:100%"
          >
            <el-option
              v-for="s in subnetOptions"
              :key="s.subnetId"
              :label="`${s.displayName} (${s.cidrBlock})`"
              :value="s.subnetId"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="IPv6 地址数">
          <el-input-number
            v-model="batchCreateForm.ipv6CountPerVnic"
            :min="0"
            :max="32"
            :step="1"
            controls-position="right"
            style="width:100%"
          />
          <div class="form-hint">每个 VNIC 的 IPv6 地址数，0 表示不创建</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="batchCreateVisible = false">取消</el-button>
        <el-button type="primary" :loading="batchCreating" @click="doBatchCreate">
          开始创建
        </el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Create IPv6 Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="createIpv6Visible" title="创建 IPv6 地址" width="460px" destroy-on-close>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item label="VNIC">
          {{ ipv6Target?.vnicDisplayName || '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="当前 IPv6 数">
          {{ ipv6Target?.ipv6Addresses?.length ?? 0 }}
        </el-descriptions-item>
      </el-descriptions>
      <el-form :model="createIpv6Form" label-width="120px" style="margin-top:16px">
        <el-form-item label="IPv6 数量" required>
          <el-input-number
            v-model="createIpv6Form.count"
            :min="1"
            :max="32 - (ipv6Target?.ipv6Addresses?.length ?? 0)"
            :step="1"
            controls-position="right"
            style="width:100%"
          />
        </el-form-item>
      </el-form>
      <el-alert
        title="创建 IPv6 后实例将自动重启（停止+启动）以生效"
        type="warning"
        :closable="false"
        show-icon
        style="margin-top:12px"
      />
      <template #footer>
        <el-button @click="createIpv6Visible = false">取消</el-button>
        <el-button type="primary" :loading="creatingIpv6" @click="doCreateIpv6">
          确认创建
        </el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Delete IPv6 Confirm Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="deleteIpv6Visible" title="删除 IPv6 地址" width="460px" destroy-on-close>
      <el-alert
        title="删除 IPv6 地址后实例将自动重启以生效"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom:16px"
      />
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item label="VNIC">
          {{ deleteIpv6Target?.vnic?.vnicDisplayName || '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="IPv6 地址">
          <span class="data-mono">{{ deleteIpv6Target?.ipv6Address || '-' }}</span>
        </el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button @click="deleteIpv6Visible = false">取消</el-button>
        <el-button type="danger" :loading="deletingIpv6" @click="doDeleteIpv6">
          确认删除
        </el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- IP Switch Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="ipSwitchVisible" title="切换 IP" width="520px" destroy-on-close>
      <el-alert
        title="将反复重新分配公网 IP，直到 IP 落在指定 CIDR 范围内。此操作可能需要较长时间。"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom:16px"
      />
      <el-form :model="ipSwitchForm" label-width="120px">
        <el-form-item label="目标 VNIC">
          <el-select v-model="ipSwitchForm.vnicId" placeholder="选择 VNIC" style="width:100%">
            <el-option
              v-for="v in allVnics"
              :key="v.vnicId"
              :label="`${v.vnicDisplayName} (${v.publicIp || '无公网IP'})`"
              :value="v.vnicId"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="CIDR 范围" required>
          <el-input
            v-model="ipSwitchForm.cidrRanges"
            type="textarea"
            :rows="3"
            placeholder="每行一个 CIDR，例如：&#10;129.146.0.0/16&#10;130.35.0.0/16"
          />
          <div class="form-hint">支持多个 CIDR，每行一个</div>
        </el-form-item>
      </el-form>
      <div v-if="ipSwitching" style="margin-top:16px">
        <el-progress :percentage="0" :indeterminate="true" :stroke-width="16" :text-inside="true" status="warning">
          <span>正在切换 IP，请耐心等待...</span>
        </el-progress>
      </div>
      <template #footer>
        <el-button @click="ipSwitchVisible = false" :disabled="ipSwitching">取消</el-button>
        <el-button type="primary" :loading="ipSwitching" @click="doIpSwitch">
          开始切换
        </el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Configure Load Balancer Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="configLBVisible" title="配置负载均衡" width="520px" destroy-on-close>
      <el-alert
        title="将一键配置 NAT 网关 + 路由表 + 网络负载均衡器 (NLB)"
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom:16px"
      />
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item label="实例">
          {{ selectedInstance?.displayName || '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="操作说明">
          创建 NAT 网关和路由表，配置 NLB 转发 SSH 流量
        </el-descriptions-item>
      </el-descriptions>
      <div v-if="lbResult" style="margin-top:16px">
        <el-alert
          :title="lbResult.success ? '配置成功' : '配置失败'"
          :type="lbResult.success ? 'success' : 'error'"
          :closable="false"
          show-icon
        >
          <template #default>
            <p>{{ lbResult.message }}</p>
            <div v-if="lbResult.success" style="margin-top:8px;font-size:12px;color:var(--text-secondary)">
              <p>NAT 网关: {{ lbResult.natGatewayName || '-' }}</p>
              <p>路由表: {{ lbResult.routeTableName || '-' }}</p>
              <p>NLB: {{ lbResult.networkLoadBalancerName || '-' }}</p>
              <p>NLB IP: {{ lbResult.nlpIpAddress || '-' }}</p>
            </div>
          </template>
        </el-alert>
      </div>
      <template #footer>
        <el-button @click="configLBVisible = false">关闭</el-button>
        <el-button v-if="!lbResult" type="primary" :loading="configuringLB" @click="doConfigureLB">
          确认配置
        </el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Batch Create Result Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="batchResultVisible" title="批量创建结果" width="600px" destroy-on-close>
      <template v-if="batchResult">
        <el-alert
          :title="batchResult.allSuccessful ? '全部成功' : '部分失败'"
          :type="batchResult.allSuccessful ? 'success' : 'warning'"
          :closable="false"
          show-icon
          style="margin-bottom:16px"
        >
          <template #default>
            <p>{{ batchResult.summary }}</p>
            <p style="margin-top:4px;font-size:12px;color:var(--text-secondary)">
              耗时: {{ (batchResult.totalExecutionTimeMs / 1000).toFixed(1) }}s
            </p>
          </template>
        </el-alert>
        <el-table :data="batchResult.vnicResults" border size="small" max-height="300">
          <el-table-column type="index" label="#" width="50" align="center" />
          <el-table-column prop="vnicDisplayName" label="名称" min-width="140" />
          <el-table-column prop="privateIp" label="私网 IP" width="130" />
          <el-table-column prop="publicIp" label="公网 IP" width="130" />
          <el-table-column label="IPv6" min-width="160">
            <template #default="{ row }">
              <span v-if="row.ipv6Addresses?.length">{{ row.ipv6Addresses.length }} 个</span>
              <span v-else class="text-muted">-</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="80" align="center">
            <template #default="{ row }">
              <el-tag :type="row.success ? 'success' : 'danger'" size="small">
                {{ row.success ? '成功' : '失败' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="errorMessage" label="错误" min-width="160" show-overflow-tooltip />
        </el-table>
      </template>
      <template #footer>
        <el-button type="primary" @click="batchResultVisible = false">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Refresh, Plus, Delete, Connection, Switch, RefreshRight,
} from '@element-plus/icons-vue'
import request from '../utils/request'
import type {
  VnicInfo, VnicLoadData, BatchVnicResult, NetworkConfigResult,
  SubnetInfo,
} from '../types/api'

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

// Tenant/Instance selection
const tenantOptions = ref<Array<{ id: number; name: string }>>([])
const instanceOptions = ref<Instance[]>([])
const selectedTenantId = ref<number | null>(null)
const selectedInstanceId = ref('')
const selectedInstance = ref<Instance | null>(null)

// VNIC data
const vnicData = ref<VnicLoadData | null>(null)
const loading = ref(false)

// Batch create
const batchCreateVisible = ref(false)
const batchCreating = ref(false)
const batchCreateForm = ref({
  vnicCount: 1,
  subnetId: '',
  ipv6CountPerVnic: 0,
})
const subnetOptions = ref<SubnetInfo[]>([])
const batchResultVisible = ref(false)
const batchResult = ref<BatchVnicResult | null>(null)

// Delete all secondary
const deletingAll = ref(false)

// Create IPv6
const createIpv6Visible = ref(false)
const creatingIpv6 = ref(false)
const ipv6Target = ref<VnicInfo | null>(null)
const createIpv6Form = ref({ count: 1 })

// Delete IPv6
const deleteIpv6Visible = ref(false)
const deletingIpv6 = ref(false)
const deleteIpv6Target = ref<{ vnic: VnicInfo; ipv6Address: string } | null>(null)

// IP Switch
const ipSwitchVisible = ref(false)
const ipSwitching = ref(false)
const ipSwitchForm = ref({ vnicId: '', cidrRanges: '' })

// Configure LB
const configLBVisible = ref(false)
const configuringLB = ref(false)
const lbResult = ref<NetworkConfigResult | null>(null)

// Restore network
const restoringNetwork = ref(false)

// ---- Computed ----
const allVnics = computed(() => vnicData.value?.vnicList || [])

// ---- Helpers ----
function stateDotClass(state: string): string {
  const s = (state || '').toLowerCase()
  if (s === 'running') return 'status-dot--up status-dot--pulse'
  if (s === 'stopped' || s === 'terminated') return 'status-dot--down'
  if (s === 'starting' || s === 'stopping') return 'status-dot--warn'
  return 'status-dot--idle'
}

function lifecycleDotClass(state: string): string {
  const s = (state || '').toLowerCase()
  if (s === 'attached') return 'status-dot--up status-dot--pulse'
  if (s === 'detached') return 'status-dot--down'
  if (s === 'attaching' || s === 'detaching') return 'status-dot--warn'
  return 'status-dot--idle'
}

function truncateIpv6(addr: string): string {
  if (!addr) return '-'
  if (addr.length <= 28) return addr
  return addr.substring(0, 20) + '...' + addr.substring(addr.length - 8)
}

// ---- Data Loading ----
async function loadTenants() {
  try {
    const tenants = await request.get('/tenants/listAll') as any[]
    tenantOptions.value = tenants.map((t: any) => ({
      id: t.id,
      name: t.userName || t.tenancy || `#${t.id}`,
    }))
  } catch { /* ignore */ }
}

async function loadInstances(tenantId: number) {
  try {
    instanceOptions.value = await request.get(`/tenants/${tenantId}/instances`) as Instance[]
  } catch {
    instanceOptions.value = []
  }
}

async function loadVnicData(instanceId: string) {
  loading.value = true
  try {
    const data = await request.get('/oci/vnic/loadData', { params: { instanceId } }) as VnicLoadData
    vnicData.value = data
    // Extract subnet options from VNIC data for batch create
    const subnetIds = new Set<string>()
    const subs: SubnetInfo[] = []
    for (const vnic of data.vnicList || []) {
      if (vnic.subnetId && !subnetIds.has(vnic.subnetId)) {
        subnetIds.add(vnic.subnetId)
        subs.push({
          subnetId: vnic.subnetId,
          displayName: vnic.vnicDisplayName || '',
          cidrBlock: '',
          ipv6CidrBlock: '',
          vcnId: '',
          availabilityDomain: '',
          lifecycleState: '',
        })
      }
    }
    subnetOptions.value = subs
  } catch (e: any) {
    ElMessage.error(e.message || '加载 VNIC 数据失败')
    vnicData.value = null
  } finally {
    loading.value = false
  }
}

async function refreshVnics() {
  if (!selectedInstanceId.value) return
  loading.value = true
  try {
    const data = await request.get('/oci/vnic/refresh', { params: { instanceId: selectedInstanceId.value } }) as VnicLoadData
    vnicData.value = data
    ElMessage.success('VNIC 数据已刷新')
  } catch (e: any) {
    ElMessage.error(e.message || '刷新失败')
  } finally {
    loading.value = false
  }
}

// ---- Selection Handlers ----
function onTenantChange(tenantId: number | null) {
  selectedInstanceId.value = ''
  selectedInstance.value = null
  vnicData.value = null
  instanceOptions.value = []
  if (tenantId) {
    loadInstances(tenantId)
  }
}

function onInstanceChange(instanceId: string) {
  selectedInstance.value = instanceOptions.value.find(i => i.instanceId === instanceId) || null
  vnicData.value = null
  if (instanceId) {
    loadVnicData(instanceId)
  }
}

// ---- Batch Create ----
function openBatchCreate() {
  batchCreateForm.value = {
    vnicCount: 1,
    subnetId: subnetOptions.value.length > 0 ? subnetOptions.value[0].subnetId : '',
    ipv6CountPerVnic: 0,
  }
  batchResult.value = null
  batchCreateVisible.value = true
}

async function doBatchCreate() {
  if (!selectedInstanceId.value) return
  if (!batchCreateForm.value.subnetId) {
    ElMessage.warning('请选择目标子网')
    return
  }
  if (batchCreateForm.value.vnicCount < 1) {
    ElMessage.warning('VNIC 数量至少为 1')
    return
  }

  batchCreating.value = true
  try {
    const result = await request.post('/oci/vnic/create', {
      instanceId: selectedInstanceId.value,
      subnetId: batchCreateForm.value.subnetId,
      vnicCount: batchCreateForm.value.vnicCount,
      ipv6CountPerVnic: batchCreateForm.value.ipv6CountPerVnic,
    }) as BatchVnicResult
    batchResult.value = result
    batchCreateVisible.value = false
    batchResultVisible.value = true
    if (result.allSuccessful) {
      ElMessage.success(`创建完成: ${result.successfulVnicCount}/${result.requestedVnicCount} 个 VNIC`)
    } else {
      ElMessage.warning(`部分创建失败: ${result.successfulVnicCount}/${result.requestedVnicCount}`)
    }
    await loadVnicData(selectedInstanceId.value)
  } catch (e: any) {
    ElMessage.error(e.message || '批量创建失败')
  } finally {
    batchCreating.value = false
  }
}

// ---- Delete All Secondary ----
async function confirmDeleteAllSecondary() {
  if (!selectedInstanceId.value || !vnicData.value?.secondaryVnics?.length) return
  const count = vnicData.value.secondaryVnics.length
  try {
    await ElMessageBox.confirm(
      `确定删除所有 ${count} 个辅助 VNIC？此操作将同时删除所有关联的 IPv6 地址。`,
      '确认删除',
      { type: 'warning', confirmButtonText: '确认删除', confirmButtonClass: 'el-button--danger' }
    )
    deletingAll.value = true
    const result = await request.post('/oci/vnic/deleteAllSecondary', {
      instanceId: selectedInstanceId.value,
    }) as any
    const successCount = result ? Object.values(result).filter(v => v === true).length : 0
    ElMessage.success(`已删除 ${successCount} 个辅助 VNIC`)
    await loadVnicData(selectedInstanceId.value)
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  } finally {
    deletingAll.value = false
  }
}

// ---- Create IPv6 ----
function openCreateIpv6(vnic: VnicInfo) {
  ipv6Target.value = vnic
  createIpv6Form.value.count = 1
  createIpv6Visible.value = true
}

async function doCreateIpv6() {
  if (!ipv6Target.value || !selectedInstanceId.value) return
  creatingIpv6.value = true
  try {
    await request.post('/oci/vnic/createIpv6', {
      vnicId: ipv6Target.value.vnicId,
      ipv6Count: createIpv6Form.value.count,
      instanceId: selectedInstanceId.value,
    })
    ElMessage.success('IPv6 地址创建成功')
    createIpv6Visible.value = false
    await loadVnicData(selectedInstanceId.value)
  } catch (e: any) {
    ElMessage.error(e.message || '创建 IPv6 失败')
  } finally {
    creatingIpv6.value = false
  }
}

// ---- Delete IPv6 ----
function confirmDeleteIpv6(vnic: VnicInfo, ipv6Address: string) {
  deleteIpv6Target.value = { vnic, ipv6Address }
  deleteIpv6Visible.value = true
}

async function doDeleteIpv6() {
  if (!deleteIpv6Target.value || !selectedInstanceId.value) return
  deletingIpv6.value = true
  try {
    await request.post('/oci/vnic/deleteIpv6', {
      ipv6Address: deleteIpv6Target.value.ipv6Address,
      vnicId: deleteIpv6Target.value.vnic.vnicId,
      instanceId: selectedInstanceId.value,
    })
    ElMessage.success('IPv6 地址已删除')
    deleteIpv6Visible.value = false
    await loadVnicData(selectedInstanceId.value)
  } catch (e: any) {
    ElMessage.error(e.message || '删除 IPv6 失败')
  } finally {
    deletingIpv6.value = false
  }
}

// ---- Delete VNIC ----
async function confirmDeleteVnic(vnic: VnicInfo) {
  if (vnic.isPrimary) {
    ElMessage.warning('不能删除主 VNIC')
    return
  }
  try {
    await ElMessageBox.confirm(
      `确定删除 VNIC「${vnic.vnicDisplayName}」？此操作将同时删除所有关联的 IPv6 地址。`,
      '确认删除',
      { type: 'warning', confirmButtonText: '确认删除', confirmButtonClass: 'el-button--danger' }
    )
    await request.post('/oci/vnic/delete', {
      instanceId: selectedInstanceId.value,
      vnicId: vnic.vnicId,
    })
    ElMessage.success('VNIC 已删除')
    await loadVnicData(selectedInstanceId.value)
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// ---- IP Switch ----
function openIpSwitch() {
  ipSwitchForm.value = {
    vnicId: allVnics.value.length > 0 ? allVnics.value[0].vnicId : '',
    cidrRanges: '',
  }
  ipSwitchVisible.value = true
}

async function doIpSwitch() {
  if (!selectedInstanceId.value) return
  if (!ipSwitchForm.value.vnicId) {
    ElMessage.warning('请选择目标 VNIC')
    return
  }
  const ranges = ipSwitchForm.value.cidrRanges
    .split('\n')
    .map(s => s.trim())
    .filter(Boolean)
  if (ranges.length === 0) {
    ElMessage.warning('请填写至少一个 CIDR 范围')
    return
  }

  ipSwitching.value = true
  try {
    const result = await request.post('/oci/vnic/changeSpecIp', {
      instanceId: selectedInstanceId.value,
      vnicId: ipSwitchForm.value.vnicId,
      cidrRanges: ranges,
    }) as any
    if (result?.status === 'success') {
      ElMessage.success(`IP 切换成功: ${result.details?.oldIp} -> ${result.details?.newIp}`)
      ipSwitchVisible.value = false
      await loadVnicData(selectedInstanceId.value)
    } else {
      ElMessage.error(result?.message || 'IP 切换失败')
    }
  } catch (e: any) {
    ElMessage.error(e.message || 'IP 切换失败')
  } finally {
    ipSwitching.value = false
  }
}

// ---- Configure Load Balancer ----
function openConfigureLB() {
  lbResult.value = null
  configLBVisible.value = true
}

async function doConfigureLB() {
  if (!selectedInstanceId.value) return
  configuringLB.value = true
  try {
    const result = await request.post('/oci/vnic/network/configureLoadBalancer', {
      instanceId: selectedInstanceId.value,
    }) as NetworkConfigResult
    lbResult.value = result
    if (result.success) {
      ElMessage.success('负载均衡配置成功')
    } else {
      ElMessage.error(result.message || '配置失败')
    }
  } catch (e: any) {
    ElMessage.error(e.message || '配置负载均衡失败')
    lbResult.value = { success: false, message: e.message || '配置失败' } as NetworkConfigResult
  } finally {
    configuringLB.value = false
  }
}

// ---- Restore Network ----
async function confirmRestoreNetwork() {
  if (!selectedInstanceId.value) return
  try {
    await ElMessageBox.confirm(
      '确定恢复网络配置？此操作将删除 NAT 网关、路由表和网络负载均衡器。',
      '确认恢复',
      { type: 'warning', confirmButtonText: '确认恢复' }
    )
    restoringNetwork.value = true
    const result = await request.post('/oci/vnic/network/restoreNetwork', {
      instanceId: selectedInstanceId.value,
    }) as any
    if (result?.success) {
      ElMessage.success('网络配置已恢复')
    } else {
      ElMessage.error(result?.message || '恢复失败')
    }
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  } finally {
    restoringNetwork.value = false
  }
}

// ---- Init ----
onMounted(loadTenants)
</script>

<style scoped>
.vnic-page {
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

/* ---- Selector Card ---- */
.selector-card {
  border-radius: var(--radius-md);
  margin-bottom: var(--space-4);
}

.selector-row {
  display: flex;
  gap: var(--space-5);
  flex-wrap: wrap;
  align-items: flex-end;
}

.selector-item {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.selector-label {
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  color: var(--text-secondary);
}

.instance-summary {
  margin-top: var(--space-4);
}

/* ---- Stats Row ---- */
.stats-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-3);
  margin-bottom: var(--space-4);
}

.stat-card {
  border-radius: var(--radius-md);
  text-align: center;
}

.stat-card :deep(.el-card__body) {
  padding: var(--space-4);
}

/* ---- Operations Toolbar ---- */
.ops-toolbar {
  display: flex;
  gap: var(--space-2);
  margin-bottom: var(--space-4);
  flex-wrap: wrap;
}

/* ---- Table Card ---- */
.table-card {
  border-radius: var(--radius-md);
  overflow: hidden;
}

.table-card :deep(.el-card__body) {
  padding: 0;
}

/* ---- Empty Card ---- */
.empty-card {
  border-radius: var(--radius-md);
}

/* ---- VNIC Name ---- */
.vnic-name {
  font-weight: var(--font-medium);
  color: var(--text-primary);
}

/* ---- IPv6 List ---- */
.ipv6-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.ipv6-tag {
  font-family: var(--font-mono);
  font-size: 11px;
}

/* ---- State Cell ---- */
.state-cell {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-sm);
}

/* ---- Form Hint ---- */
.form-hint {
  font-size: var(--text-xs);
  color: var(--text-muted);
  margin-top: var(--space-1);
}

/* ---- Text Muted ---- */
.text-muted {
  color: var(--text-muted);
  font-size: var(--text-sm);
}

/* ---- Element Plus Deep Overrides ---- */
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

:deep(.el-statistic__head) {
  font-size: var(--text-xs);
  color: var(--text-secondary);
}

:deep(.el-statistic__content) {
  font-size: var(--text-xl);
}

@media (max-width: 768px) {
  .toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .toolbar-left h2 {
    font-size: var(--text-lg);
  }

  .selector-row {
    flex-direction: column;
  }

  .stats-row {
    grid-template-columns: repeat(2, 1fr);
  }

  .ops-toolbar {
    flex-direction: column;
  }
}
</style>
