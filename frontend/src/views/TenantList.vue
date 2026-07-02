<template>
  <div class="tenants-page">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>租户管理</h2>
        <el-tag type="info" size="small">{{ rows.length }} 个租户</el-tag>
        <el-input v-model="searchText" placeholder="搜索租户名称..." size="small" clearable
          style="width: 200px" :prefix-icon="Search" />
      </div>
      <div class="toolbar-right">
        <el-button type="primary" @click="openAdd"><el-icon><Plus /></el-icon> 新增租户</el-button>
        <el-button @click="startBatchCheck" :disabled="rows.length === 0"><el-icon><Connection /></el-icon> 批量检测</el-button>
        <el-button @click="load" :loading="loading"><el-icon><Refresh /></el-icon> 刷新</el-button>
      </div>
    </div>

    <!-- Table -->
    <el-card shadow="none" class="table-card">
      <el-table :data="filteredRows" v-loading="loading" border stripe style="width: 100%">
        <template #empty>
          <el-empty description="暂无租户，请新增" :image-size="80">
            <el-button type="primary" @click="openAdd">新增租户</el-button>
          </el-empty>
        </template>
        <el-table-column type="index" label="#" width="50" align="center" />
        <el-table-column label="租户名" min-width="110">
          <template #default="{ row }">
            <span class="spoiler-link" @click="showName = showName === row.id ? 0 : row.id">
              <template v-if="showName === row.id">{{ row.tenancyName || row.userName }}</template>
              <template v-else>{{ maskedName(row.tenancyName || row.userName) }}</template>
            </span>
          </template>
        </el-table-column>
        <el-table-column label="自定义名称" min-width="120">
          <template #default="{ row }">
            <span class="cell-edit-link" @click="openEditCustomName(row)" :title="row.tenancyDes || '点击设置'">{{ row.tenancyDes || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="账号成本" width="100" align="center">
          <template #default="{ row }">
            <span class="cell-edit-link data-mono" @click="openEditCost(row)">{{ row.accountCost || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="存活天数" width="90" align="center">
          <template #default="{ row }"><span class="days-chip">{{ row.activeDays || '0' }}</span></template>
        </el-table-column>
        <el-table-column label="开机任务" width="100" align="center">
          <template #default="{ row }">
            <span class="status-badge" :class="row.hasBootTask ? 'status-running' : 'status-idle'">
              <el-icon v-if="row.hasBootTask" class="is-loading" :size="10"><Operation /></el-icon>
              {{ row.hasBootTask ? '有任务' : '无任务' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="主区域" width="100">
          <template #default="{ row }"><span class="data-mono">{{ row.regionName || row.region || '—' }}</span></template>
        </el-table-column>
        <el-table-column label="多区" width="60" align="center">
          <template #default="{ row }">
            <span :class="row.hasChildren ? 'home-region-badge is-home' : 'home-region-badge not-home'">{{ row.hasChildren ? '是' : '否' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="账号类型" width="110">
          <template #default="{ row }">
            <el-tag size="small" :type="accountTypeTag(row.accountType)">{{ accountTypeLabel(row.accountType) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="150">
          <template #default="{ row }"><span class="data-mono" style="font-size:12px">{{ row.createdAt || '—' }}</span></template>
        </el-table-column>
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <span class="status-dot" :class="row.isActive ? 'status-dot--up status-dot--pulse' : 'status-dot--down'" />
            {{ row.isActive ? '正常' : '停用' }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="60" fixed="right" align="center">
          <template #default="{ row }">
            <el-dropdown trigger="click" @command="(cmd: string) => handleAction(cmd, row)">
              <el-button size="small" text><el-icon><MoreFilled /></el-icon></el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="detail"><el-icon><InfoFilled /></el-icon> 详情</el-dropdown-item>
                  <el-dropdown-item command="sync"><el-icon><Connection /></el-icon> 同步 OCI</el-dropdown-item>
                  <el-dropdown-item command="export" divided><el-icon><Download /></el-icon> 导出租户</el-dropdown-item>
                  <el-dropdown-item command="delete" divided style="color:var(--status-down)"><el-icon><Delete /></el-icon> 删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- ======================== dialogs ======================== -->

    <!-- Add tenant -->
    <el-dialog v-model="addVisible" title="新增租户" width="660px" destroy-on-close>
      <el-collapse v-model="configCollapse" style="margin-bottom:16px">
        <el-collapse-item title="从 OCI Config 文件导入" name="config">
          <el-alert title="粘贴 OCI CLI 配置文件内容，自动填写下方表单" type="info" :closable="false" show-icon style="margin-bottom:12px"/>
          <el-input v-model="ociConfigText" type="textarea" :rows="6" placeholder="[DEFAULT]\nuser=ocid1.user.oc1..xxx\nfingerprint=xx:xx:xx\ntenancy=ocid1.tenancy.oc1..xxx\nregion=ap-singapore-2"/>
          <div style="margin-top:12px;display:flex;gap:8px">
            <el-button type="primary" size="small" @click="parseOciConfig" :disabled="!ociConfigText.trim()">解析并填写</el-button>
            <el-button size="small" @click="ociConfigText='';configCollapse=[]">清空</el-button>
          </div>
        </el-collapse-item>
      </el-collapse>
      <el-form :model="form" label-width="120px">
        <el-form-item label="Tenancy OCID" required><el-input v-model="form.tenancy" placeholder="ocid1.tenancy.oc1..xxxxx"/></el-form-item>
        <el-form-item label="User OCID" required><el-input v-model="form.tenantId" placeholder="ocid1.user.oc1..xxxxx"/></el-form-item>
        <el-form-item label="指纹" required><el-input v-model="form.fingerprint" placeholder="3a:37:17:38:xx:xx:xx"/></el-form-item>
        <el-form-item label="区域" required>
          <el-select v-model="form.region" filterable allow-create placeholder="选择或输入区域">
            <el-option v-for="r in regions" :key="r.code" :label="`${r.name} (${r.code})`" :value="r.name"/>
          </el-select>
        </el-form-item>
        <el-form-item label="用户名"><el-input v-model="form.userName" placeholder="留空自动生成"/></el-form-item>
        <el-form-item label="API 私钥" required><input ref="keyFile" type="file" @change="onFile" style="display:block"/></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addVisible=false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <!-- Edit custom name -->
    <el-dialog v-model="editNameVisible" title="设置自定义名称" width="460px" destroy-on-close>
      <el-form label-width="100px">
        <el-form-item label="租户名"><el-input :model-value="editTarget?.userName || editTarget?.tenancyName" disabled/></el-form-item>
        <el-form-item label="自定义名称"><el-input v-model="editNameValue" placeholder="输入自定义名称"/></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editNameVisible=false">取消</el-button>
        <el-button type="primary" :loading="editSaving" @click="saveCustomName">保存</el-button>
      </template>
    </el-dialog>

    <!-- Edit account cost -->
    <el-dialog v-model="editCostVisible" title="设置账号成本" width="460px" destroy-on-close>
      <el-form label-width="100px">
        <el-form-item label="租户名"><el-input :model-value="editTarget?.userName || editTarget?.tenancyName" disabled/></el-form-item>
        <el-form-item label="账号成本"><el-input v-model="editCostValue" placeholder="例如: $29.99/月"/></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editCostVisible=false">取消</el-button>
        <el-button type="primary" :loading="editSaving" @click="saveAccountCost">保存</el-button>
      </template>
    </el-dialog>

    <!-- Export tenant -->
    <el-dialog v-model="exportVisible" title="导出租户数据" width="460px" destroy-on-close>
      <p style="color:var(--text-secondary);font-size:var(--text-sm)">租户: <strong>{{ exportTarget?.userName || exportTarget?.tenancyName }}</strong></p>
      <template #footer>
        <el-button @click="exportVisible=false">取消</el-button>
        <el-button type="primary" :loading="exporting" @click="doExport">确认导出</el-button>
      </template>
    </el-dialog>

    <!-- Batch check -->
    <el-dialog v-model="batchCheckVisible" title="批量检测租户状态" width="600px" destroy-on-close>
      <el-progress v-if="batchChecking" :percentage="batchProgress" :stroke-width="10" style="margin-bottom:16px"/>
      <el-table v-if="batchResults.length" :data="batchResults" size="small" max-height="400" border>
        <el-table-column label="租户" min-width="120">
          <template #default="{ row }">{{ row.userName || row.tenancyName || '#' + row.tenantId }}</template>
        </el-table-column>
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.alive ? 'success' : 'danger'" size="small">{{ row.alive ? '存活' : '异常' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="错误信息" min-width="180">
          <template #default="{ row }">{{ row.error || '—' }}</template>
        </el-table-column>
      </el-table>
      <template #footer><el-button @click="batchCheckVisible=false">关闭</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus, Refresh, Connection, InfoFilled, Download, Delete, Search,
  Operation, MoreFilled
} from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import request from '../utils/request'
import { maskedName, accountTypeTag, accountTypeLabel } from '../utils/tenant-utils'

defineOptions({ name: 'tenants' })

const router = useRouter()

// --- types ---
interface Tenant {
  id: number; tenantId?: string; userName: string; tenancy: string; region: string
  regionName: string; fingerprint: string; apiSynced: boolean
  tenancyName?: string; tenancyDes?: string; accountType?: string
  cloudType?: number; emailAddress?: string; emailEnable?: boolean
  isActive?: boolean; isHomeRegion?: boolean; createdAt?: string
  enableIcmp?: boolean; enableAllProtocol?: boolean
  parenId?: number; regionEn?: string; idStr?: string
  transferStatus?: number; transferAmount?: string
  instanceCount?: number; accountCost?: string
  hasBootTask?: boolean; hasChildren?: boolean; activeDays?: string
}
interface RegionItem { code: string; name: string }

// --- state ---
const rows = ref<Tenant[]>([])
const loading = ref(false)
const searchText = ref('')
const showName = ref(0)

// add dialog
const addVisible = ref(false)
const saving = ref(false)
const keyFile = ref<HTMLInputElement | null>(null)
const fileBytes = ref<File | null>(null)
const form = ref({ tenancy: '', tenantId: '', fingerprint: '', region: '', userName: '' })
const configCollapse = ref<string[]>([])
const ociConfigText = ref('')

// edit name / cost
const editTarget = ref<Tenant | null>(null)
const editNameVisible = ref(false)
const editNameValue = ref('')
const editCostVisible = ref(false)
const editCostValue = ref('')
const editSaving = ref(false)

// export
const exportVisible = ref(false)
const exportTarget = ref<Tenant | null>(null)
const exporting = ref(false)

// batch check
const batchCheckVisible = ref(false)
const batchChecking = ref(false)
const batchProgress = ref(0)
const batchResults = ref<any[]>([])

// --- computed ---
const filteredRows = computed(() => {
  const q = searchText.value.toLowerCase().trim()
  if (!q) return rows.value
  return rows.value.filter(r =>
    (r.tenancyName || '').toLowerCase().includes(q) ||
    (r.userName || '').toLowerCase().includes(q) ||
    (r.tenancyDes || '').toLowerCase().includes(q) ||
    (r.region || '').toLowerCase().includes(q)
  )
})

// --- regions ---
const regions: RegionItem[] = [
  { code: 'ap-tokyo-1', name: '东京 (ap-tokyo-1)' },
  { code: 'ap-osaka-1', name: '大阪 (ap-osaka-1)' },
  { code: 'ap-seoul-1', name: '首尔 (ap-seoul-1)' },
  { code: 'ap-chuncheon-1', name: '春川 (ap-chuncheon-1)' },
  { code: 'ap-mumbai-1', name: '孟买 (ap-mumbai-1)' },
  { code: 'ap-hyderabad-1', name: '海得拉巴 (ap-hyderabad-1)' },
  { code: 'ap-singapore-1', name: '新加坡 (ap-singapore-1)' },
  { code: 'ap-sydney-1', name: '悉尼 (ap-sydney-1)' },
  { code: 'ap-melbourne-1', name: '墨尔本 (ap-melbourne-1)' },
  { code: 'me-jeddah-1', name: '吉达 (me-jeddah-1)' },
  { code: 'me-abudhabi-1', name: '阿布扎比 (me-abudhabi-1)' },
  { code: 'me-dubai-1', name: '迪拜 (me-dubai-1)' },
  { code: 'eu-frankfurt-1', name: '法兰克福 (eu-frankfurt-1)' },
  { code: 'eu-amsterdam-1', name: '阿姆斯特丹 (eu-amsterdam-1)' },
  { code: 'eu-london-1', name: '伦敦 (eu-london-1)' },
  { code: 'eu-paris-1', name: '巴黎 (eu-paris-1)' },
  { code: 'eu-zurich-1', name: '苏黎世 (eu-zurich-1)' },
  { code: 'eu-marseille-1', name: '马赛 (eu-marseille-1)' },
  { code: 'eu-stockholm-1', name: '斯德哥尔摩 (eu-stockholm-1)' },
  { code: 'eu-milan-1', name: '米兰 (eu-milan-1)' },
  { code: 'eu-madrid-1', name: '马德里 (eu-madrid-1)' },
  { code: 'sa-saopaulo-1', name: '圣保罗 (sa-saopaulo-1)' },
  { code: 'sa-santiago-1', name: '圣地亚哥 (sa-santiago-1)' },
  { code: 'sa-vinhedo-1', name: '因赫道 (sa-vinhedo-1)' },
  { code: 'sa-bogota-1', name: '波哥大 (sa-bogota-1)' },
  { code: 'ca-montreal-1', name: '蒙特利尔 (ca-montreal-1)' },
  { code: 'ca-toronto-1', name: '多伦多 (ca-toronto-1)' },
  { code: 'us-ashburn-1', name: '阿什本 (us-ashburn-1)' },
  { code: 'us-chicago-1', name: '芝加哥 (us-chicago-1)' },
  { code: 'us-phoenix-1', name: '凤凰城 (us-phoenix-1)' },
  { code: 'us-sanjose-1', name: '圣何塞 (us-sanjose-1)' },
  { code: 'af-johannesburg-1', name: '约翰内斯堡 (af-johannesburg-1)' },
  { code: 'ap-dcc-canberra-1', name: '堪培拉 (ap-dcc-canberra-1)' },
  { code: 'eu-dcc-dublin-1', name: '都柏林 (eu-dcc-dublin-1)' },
  { code: 'eu-dcc-milan-1', name: '米兰DCC (eu-dcc-milan-1)' },
  { code: 'eu-dcc-rating-2', name: '莱廷 (eu-dcc-rating-2)' },
  { code: 'uk-london-1', name: '伦敦UK (uk-london-1)' },
  { code: 'uk-cardiff-1', name: '卡迪夫 (uk-cardiff-1)' },
  { code: 'il-jerusalem-1', name: '耶路撒冷 (il-jerusalem-1)' },
  { code: 'mx-queretaro-1', name: '克雷塔罗 (mx-queretaro-1)' },
  { code: 'ap-singapore-2', name: '新加坡2 (ap-singapore-2)' },
  { code: 'us-langley-1', name: '兰利 (us-langley-1)' },
  { code: 'us-luke-1', name: '卢克 (us-luke-1)' },
  { code: 'us-gov-ashburn-1', name: '阿什本Gov (us-gov-ashburn-1)' },
  { code: 'us-gov-chicago-1', name: '芝加哥Gov (us-gov-chicago-1)' },
  { code: 'us-gov-phoenix-1', name: '凤凰城Gov (us-gov-phoenix-1)' },
]
const regionCodeToName: Record<string, string> = {}
regions.forEach(r => { regionCodeToName[r.code] = r.name })

// --- actions ---
function handleAction(cmd: string, row: Tenant) {
  switch (cmd) {
    case 'detail': router.push({ name: 'tenant-detail', params: { id: row.id } }); break
    case 'sync': syncOci(row); break
    case 'export': exportTarget.value = row; exportVisible.value = true; break
    case 'delete': remove(row); break
  }
}

// --- load ---
async function load() {
  loading.value = true
  try {
    const tenants = await request.get('/tenants/listAll') as Tenant[]
    const countResults = await Promise.allSettled(
      tenants.map(t => request.get(`/tenants/${t.id}/instances`) as Promise<any[]>)
    )
    countResults.forEach((r, i) => {
      tenants[i].instanceCount = r.status === 'fulfilled' ? (r.value?.length ?? 0) : 0
    })
    rows.value = tenants
  } catch (e: any) { ElMessage.error(e.message) }
  finally { loading.value = false }
}

// --- sync ---
async function syncOci(row: Tenant) {
  try {
    await ElMessageBox.confirm(`从 OCI 同步租户 ${row.userName} 的实例？`, '确认同步', { type: 'info' })
    await request.get('/tenants/syncOci', { params: { tenantId: row.id } })
    ElMessage.success('同步完成')
    await load()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error('同步失败: ' + e.message) }
}

// --- add ---
function openAdd() {
  form.value = { tenancy: '', tenantId: '', fingerprint: '', region: '', userName: '' }
  fileBytes.value = null; ociConfigText.value = ''; configCollapse.value = []
  if (keyFile.value) keyFile.value.value = ''
  addVisible.value = true
}
function onFile(e: Event) { fileBytes.value = (e.target as HTMLInputElement).files?.[0] || null }

async function save() {
  if (!form.value.tenancy || !form.value.tenantId || !form.value.fingerprint || !form.value.region) {
    ElMessage.warning('请填写所有必填字段'); return
  }
  if (!fileBytes.value) { ElMessage.warning('请选择 API 私钥文件'); return }
  saving.value = true
  try {
    const fd = new FormData()
    fd.append('tenancy', form.value.tenancy)
    fd.append('tenantId', form.value.tenantId)
    fd.append('fingerprint', form.value.fingerprint)
    fd.append('region', form.value.region)
    fd.append('userName', form.value.userName)
    fd.append('cloudType', '1')
    fd.append('isHomeRegion', 'true')
    fd.append('keyFileStr', fileBytes.value)
    await request.post('/tenants/save', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    ElMessage.success('保存成功')
    addVisible.value = false
    await load()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { saving.value = false }
}

function parseOciConfig() {
  const text = ociConfigText.value.trim()
  if (!text) { ElMessage.warning('请先粘贴 OCI 配置文件内容'); return }
  const lines = text.split('\n')
  let inDefault = false
  const kv: Record<string, string> = {}
  for (const raw of lines) {
    const line = raw.trim()
    if (!line || line.startsWith('#') || line.startsWith(';')) continue
    if (line.startsWith('[') && line.endsWith(']')) { inDefault = line.toLowerCase().includes('default'); continue }
    if (!inDefault && text.includes('[')) continue
    const eq = line.indexOf('=')
    if (eq === -1) continue
    const key = line.substring(0, eq).trim().toLowerCase()
    const value = line.substring(eq + 1).trim()
    if (key && value) kv[key] = value
  }
  let filled = 0
  if (kv['user'] && !form.value.tenantId) { form.value.tenantId = kv['user']; filled++ }
  if (kv['fingerprint'] && !form.value.fingerprint) { form.value.fingerprint = kv['fingerprint']; filled++ }
  if (kv['tenancy'] && !form.value.tenancy) { form.value.tenancy = kv['tenancy']; filled++ }
  if (kv['region'] && !form.value.region) { form.value.region = regionCodeToName[kv['region']] || kv['region']; filled++ }
  if (kv['region'] && !form.value.userName) { form.value.userName = `${kv['region'].replace(/-/g, '_')}_${Math.random().toString(36).substring(2, 6)}` }
  if (filled > 0) { ElMessage.success(`已自动填写 ${filled} 个字段`); configCollapse.value = [] }
  else { ElMessage.info('未找到可识别的配置项') }
}

// --- remove ---
async function remove(row: Tenant) {
  try {
    await ElMessageBox.confirm(`确定删除租户「${row.userName || row.tenancyName}」？此操作将同时删除该租户下的所有实例记录，不可恢复。`, '确认删除', { confirmButtonText: '确定删除', cancelButtonText: '取消', type: 'warning' })
    await request.get('/tenants/deleteApi', { params: { tenantId: row.id } })
    ElMessage.success('已删除')
    await load()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
}

// --- edit custom name ---
function openEditCustomName(row: Tenant) {
  editTarget.value = row; editNameValue.value = row.tenancyDes || ''; editNameVisible.value = true
}
async function saveCustomName() {
  if (!editTarget.value) return
  editSaving.value = true
  try {
    await request.put(`/tenants/${editTarget.value.id}`, {
      tenancyName: editTarget.value.tenancyName || editTarget.value.userName,
      tenancyDes: editNameValue.value,
      accountType: editTarget.value.accountType || '',
      emailAddress: editTarget.value.emailAddress || '',
      isActive: editTarget.value.isActive ?? true,
    })
    ElMessage.success('已更新')
    editTarget.value.tenancyDes = editNameValue.value
    editNameVisible.value = false
  } catch (e: any) { ElMessage.error(e.message) }
  finally { editSaving.value = false }
}

// --- edit cost ---
function openEditCost(row: Tenant) {
  editTarget.value = row; editCostValue.value = row.accountCost || ''; editCostVisible.value = true
}
async function saveAccountCost() {
  if (!editTarget.value) return
  editSaving.value = true
  try {
    await request.put(`/tenants/${editTarget.value.id}`, {
      tenancyName: editTarget.value.tenancyName || editTarget.value.userName,
      tenancyDes: editTarget.value.tenancyDes || '',
      accountType: editTarget.value.accountType || '',
      emailAddress: editTarget.value.emailAddress || '',
      isActive: editTarget.value.isActive ?? true,
    })
    ElMessage.success('已更新')
    editTarget.value.accountCost = editCostValue.value
    editCostVisible.value = false
  } catch (e: any) { ElMessage.error(e.message) }
  finally { editSaving.value = false }
}

// --- export ---
async function doExport() {
  if (!exportTarget.value) return
  exporting.value = true
  try {
    const data = await request.get(`/tenants/${exportTarget.value.id}/export`, { responseType: 'blob' }) as any
    const url = URL.createObjectURL(data)
    const a = document.createElement('a'); a.href = url; a.download = `tenant_${exportTarget.value.id}_export.json`
    document.body.appendChild(a); a.click(); document.body.removeChild(a); URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
    exportVisible.value = false
  } catch (e: any) { ElMessage.error(e.message) }
  finally { exporting.value = false }
}

// --- batch check ---
async function startBatchCheck() {
  batchCheckVisible.value = true; batchChecking.value = true; batchProgress.value = 0; batchResults.value = []
  try {
    const ids = rows.value.map(r => r.id)
    const results: any[] = []
    for (let i = 0; i < ids.length; i++) {
      try { results.push(await request.get(`/tenants/${ids[i]}/check`)) }
      catch { results.push({ tenantId: ids[i], userName: rows.value[i]?.userName || '', alive: false, error: '请求失败' }) }
      batchProgress.value = Math.min(100, Math.round((i + 1) / ids.length * 100))
    }
    batchResults.value = results
  } catch (e: any) { ElMessage.error(e.message) }
  finally { batchChecking.value = false }
}

onMounted(load)
</script>

<style scoped>
.tenants-page { padding: 20px; }
.toolbar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; flex-wrap: wrap; gap: 12px; }
.toolbar-left { display: flex; align-items: center; gap: 12px; }
.toolbar-left h2 { margin: 0; font-size: var(--text-xl); font-weight: var(--font-bold); }
.toolbar-right { display: flex; align-items: center; gap: 8px; }
.table-card { margin-bottom: 16px; }

.spoiler-link { cursor: pointer; color: var(--accent); }
.spoiler-link:hover { text-decoration: underline; }
.cell-edit-link { cursor: pointer; color: var(--accent); }
.cell-edit-link:hover { text-decoration: underline; }
.data-mono { font-family: 'JetBrains Mono', monospace; font-size: var(--text-sm); }

.status-badge {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 2px 8px; border-radius: var(--radius-sm);
  font-size: var(--text-xs); font-weight: var(--font-medium);
}
.status-badge.status-running { background: color-mix(in srgb, var(--status-up) 15%, transparent); color: var(--status-up); }
.status-badge.status-idle { background: var(--bg-raised); color: var(--text-secondary); }

.days-chip {
  display: inline-block; padding: 2px 8px; border-radius: var(--radius-sm);
  background: var(--bg-raised); font-size: var(--text-sm); font-weight: var(--font-semibold); color: var(--text-primary);
}

.home-region-badge {
  display: inline-block; padding: 2px 8px; border-radius: var(--radius-sm);
  font-size: var(--text-xs); font-weight: var(--font-medium);
}
.home-region-badge.is-home { background: color-mix(in srgb, var(--status-up) 15%, transparent); color: var(--status-up); }
.home-region-badge.not-home { background: var(--bg-raised); color: var(--text-secondary); }

:deep(.el-table) { border-radius: var(--radius-md); overflow: hidden; }
:deep(.el-table th) { background: var(--bg-raised); font-weight: var(--font-semibold); color: var(--text-primary); }
:deep(.el-dialog) { border-radius: var(--radius-lg); }
:deep(.el-dialog__title) { font-size: var(--text-lg); font-weight: var(--font-semibold); }
:deep(.el-collapse) { border: 1px solid var(--border-default); border-radius: var(--radius-md); background: var(--bg-surface); }
:deep(.el-collapse-item) { border-bottom: 1px solid var(--border-subtle); }
:deep(.el-collapse-item:last-child) { border-bottom: none; }
:deep(.el-collapse-item__header) { font-size: var(--text-base); font-weight: var(--font-semibold); color: var(--accent); }
:deep(.el-collapse-item__header:hover) { color: var(--accent-hover); }

@media (max-width: 768px) {
  .toolbar { flex-direction: column; align-items: flex-start; }
  .toolbar-left h2 { font-size: var(--text-lg); }
  .toolbar-right { width: 100%; justify-content: flex-start; }
}
</style>
