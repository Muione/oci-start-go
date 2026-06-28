<template>
  <div class="tenants-page">
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>租户管理</h2>
        <el-tag type="info" size="small">{{ rows.length }} 个租户</el-tag>
      </div>
      <div class="toolbar-right">
        <el-button type="primary" @click="openAdd">
          <el-icon><Plus /></el-icon> 新增租户
        </el-button>
        <el-button @click="load" :loading="loading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </div>
    </div>

    <el-card shadow="none" class="table-card">
      <el-table :data="rows" v-loading="loading" border stripe style="width: 100%">
        <template #empty>
          <el-empty description="暂无租户，请新增" :image-size="80">
            <el-button type="primary" @click="openAdd">新增租户</el-button>
          </el-empty>
        </template>
        <el-table-column prop="id" label="ID" width="60" align="center" />
        <el-table-column prop="userName" label="用户名" min-width="140">
          <template #default="{ row }">
            <span class="tenant-name" @click="showInstances(row)" style="cursor:pointer;color:var(--accent)">
              {{ row.userName }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="tenancy" label="Tenancy OCID" min-width="220" show-overflow-tooltip />
        <el-table-column label="区域" width="120">
          <template #default="{ row }">
            <span class="data-mono">{{ row.regionName || row.region || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="实例数" width="90" align="center">
          <template #default="{ row }">
            <el-tag
              :type="(row.instanceCount || 0) > 0 ? 'success' : 'info'"
              size="small"
              effect="dark"
            >
              {{ row.instanceCount ?? '...' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="已同步" width="80" align="center">
          <template #default="{ row }">
            <span class="status-dot" :class="row.apiSynced ? 'status-dot--up' : 'status-dot--idle'"></span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="syncOci(row)">
              <el-icon><Refresh /></el-icon> 同步
            </el-button>
            <el-button size="small" @click="showInstances(row)">
              <el-icon><Monitor /></el-icon> 实例
            </el-button>
            <el-popconfirm title="确定删除该租户？" @confirm="remove(row)">
              <template #reference>
                <el-button size="small" type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- ================================================================ -->
    <!-- Add Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="addVisible" title="新增租户" width="660px" destroy-on-close>
      <!-- OCI Config Import -->
      <el-collapse v-model="configCollapse" style="margin-bottom:16px">
        <el-collapse-item title="从 OCI Config 文件导入" name="config">
          <el-alert
            title="粘贴 OCI CLI 配置文件内容，自动填写下方表单"
            type="info"
            :closable="false"
            show-icon
            style="margin-bottom:12px"
          />
          <el-input
            v-model="ociConfigText"
            type="textarea"
            :rows="6"
            placeholder="[DEFAULT]&#10;user=ocid1.user.oc1..aaaaaaaaxxx&#10;fingerprint=3a:37:17:38:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx&#10;tenancy=ocid1.tenancy.oc1..aaaaaaaaxxx&#10;region=ap-singapore-2"
          />
          <div style="margin-top:12px; display:flex; gap:8px">
            <el-button type="primary" size="small" @click="parseOciConfig" :disabled="!ociConfigText.trim()">
              解析并填写
            </el-button>
            <el-button size="small" @click="ociConfigText = ''; configCollapse = []">清空</el-button>
          </div>
        </el-collapse-item>
      </el-collapse>

      <el-form :model="form" label-width="120px">
        <el-form-item label="Tenancy OCID" required>
          <el-input v-model="form.tenancy" placeholder="ocid1.tenancy.oc1..xxxxx" />
        </el-form-item>
        <el-form-item label="User OCID" required>
          <el-input v-model="form.tenantId" placeholder="ocid1.user.oc1..xxxxx" />
        </el-form-item>
        <el-form-item label="指纹" required>
          <el-input v-model="form.fingerprint" placeholder="3a:37:17:38:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx" />
        </el-form-item>
        <el-form-item label="区域" required>
          <el-select v-model="form.region" filterable allow-create placeholder="选择或输入区域代码">
            <el-option v-for="r in regions" :key="r.code" :label="`${r.name} (${r.code})`" :value="r.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model="form.userName" placeholder="留空自动生成" />
        </el-form-item>
        <el-form-item label="API 私钥" required>
          <input ref="keyFile" type="file" @change="onFile" style="display:block" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Instances Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="instVisible" :title="`实例列表 — 租户 ${instTenantName}`" width="85%" destroy-on-close>
      <template v-if="instLoading">
        <el-skeleton :rows="5" animated />
      </template>
      <el-table v-else :data="instances" border stripe size="small">
        <template #empty>
          <el-empty description="该租户下暂无实例" :image-size="60" />
        </template>
        <el-table-column prop="displayName" label="名称" min-width="160" />
        <el-table-column prop="instanceId" label="实例ID" min-width="200" show-overflow-tooltip />
        <el-table-column prop="shape" label="Shape" min-width="140" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <div class="state-cell">
              <span class="status-dot" :class="instStateDot(row.state)"></span>
              {{ row.state || '-' }}
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="publicIps" label="公网IP" width="140" />
        <el-table-column prop="architecture" label="架构" width="80" />
        <el-table-column label="规格" width="120">
          <template #default="{ row }">{{ row.ocpus || 0 }}C / {{ row.memoryInGbs || 0 }}G</template>
        </el-table-column>
        <el-table-column prop="availabilityDomain" label="AD" min-width="120" show-overflow-tooltip />
        <el-table-column prop="createTime" label="创建时间" width="160" />
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, Monitor } from '@element-plus/icons-vue'
import request from '../utils/request'

interface Tenant {
  id: number; userName: string; tenancy: string; region: string
  regionName: string; fingerprint: string; apiSynced: boolean
  instanceCount?: number
}
interface Region { code: string; name: string }

const rows = ref<Tenant[]>([])
const loading = ref(false)
const addVisible = ref(false)
const saving = ref(false)
const keyFile = ref<HTMLInputElement | null>(null)
const fileBytes = ref<File | null>(null)
const instVisible = ref(false)
const instLoading = ref(false)
const instTenantId = ref<number>(0)
const instTenantName = ref('')
const instances = ref<any[]>([])

const form = ref({ tenancy: '', tenantId: '', fingerprint: '', region: '', userName: '' })

const configCollapse = ref<string[]>([])
const ociConfigText = ref('')

const regions: Region[] = [
  { code: 'ap-tokyo-1', name: '东京' }, { code: 'ap-osaka-1', name: '大阪' },
  { code: 'ap-seoul-1', name: '首尔' }, { code: 'ap-singapore-1', name: '新加坡' },
  { code: 'ap-singapore-2', name: '新加坡(西)' }, { code: 'ap-mumbai-1', name: '孟买' },
  { code: 'ap-hyderabad-1', name: '海得拉巴' }, { code: 'ap-sydney-1', name: '悉尼' },
  { code: 'ap-melbourne-1', name: '墨尔本' }, { code: 'ap-chuncheon-1', name: '春川' },
  { code: 'ap-osaka-2', name: '大阪(第2)' },
  { code: 'us-ashburn-1', name: '阿什本' }, { code: 'us-phoenix-1', name: '凤凰城' },
  { code: 'us-sanjose-1', name: '圣何塞' }, { code: 'us-sanjose-2', name: '圣何塞(第2)' },
  { code: 'us-chicago-1', name: '芝加哥' }, { code: 'us-phoenix-2', name: '凤凰城(第2)' },
  { code: 'us-ashburn-2', name: '阿什本(第2)' },
  { code: 'sa-saopaulo-1', name: '圣保罗' }, { code: 'sa-vinhedo-1', name: '维涅杜' },
  { code: 'mx-queretaro-1', name: '克雷塔罗' }, { code: 'mx-queretaro-2', name: '克雷塔罗(第2)' },
  { code: 'eu-frankfurt-1', name: '法兰克福' }, { code: 'eu-frankfurt-2', name: '法兰克福(第2)' },
  { code: 'uk-london-1', name: '伦敦' }, { code: 'uk-cardiff-1', name: '加的夫' },
  { code: 'eu-zurich-1', name: '苏黎世' }, { code: 'eu-amsterdam-1', name: '阿姆斯特丹' },
  { code: 'eu-madrid-1', name: '马德里' }, { code: 'eu-milan-1', name: '米兰' },
  { code: 'me-jeddah-1', name: '吉达' }, { code: 'me-dubai-1', name: '迪拜' },
  { code: 'me-abudhabi-1', name: '阿布扎比' }, { code: 'af-johannesburg-1', name: '约翰内斯堡' },
]

const regionCodeToName: Record<string, string> = {}
regions.forEach(r => { regionCodeToName[r.code] = r.name })

function instStateDot(state: string): string {
  if (!state) return 'status-dot--idle'
  const s = state.toLowerCase()
  if (s === 'running') return 'status-dot--up status-dot--pulse'
  if (s === 'stopped' || s === 'terminated') return 'status-dot--down'
  if (s === 'starting' || s === 'stopping') return 'status-dot--warn'
  return 'status-dot--idle'
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
    if (line.startsWith('[') && line.endsWith(']')) {
      inDefault = line.toLowerCase().includes('default')
      continue
    }
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
  if (kv['region'] && !form.value.region) {
    form.value.region = regionCodeToName[kv['region']] || kv['region']
    filled++
  }
  if (kv['region'] && !form.value.userName) {
    form.value.userName = `${kv['region'].replace(/-/g, '_')}_${Math.random().toString(36).substring(2, 6)}`
  }

  if (filled > 0) {
    ElMessage.success(`已自动填写 ${filled} 个字段`)
    configCollapse.value = []
  } else {
    ElMessage.info('未找到可识别的配置项（需要 [DEFAULT] 段下的 user/fingerprint/tenancy/region）')
  }
}

async function load() {
  loading.value = true
  try {
    const tenants = await request.get('/tenants/listAll') as Tenant[]
    // Load instance counts in parallel
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

function openAdd() {
  form.value = { tenancy: '', tenantId: '', fingerprint: '', region: '', userName: '' }
  fileBytes.value = null
  ociConfigText.value = ''
  configCollapse.value = []
  if (keyFile.value) keyFile.value.value = ''
  addVisible.value = true
}

function onFile(e: Event) {
  const t = e.target as HTMLInputElement
  fileBytes.value = t.files?.[0] || null
}

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

async function remove(row: Tenant) {
  try {
    await request.get('/tenants/deleteApi', { params: { tenantId: row.id } })
    ElMessage.success('已删除')
    await load()
  } catch (e: any) { ElMessage.error(e.message) }
}

async function syncOci(row: Tenant) {
  try {
    await ElMessageBox.confirm(`从 OCI 同步租户 ${row.userName} 的实例？`, '确认同步', { type: 'info' })
    const msg = ElMessage.info('同步中…（可能需要数秒，请稍候）')
    await request.get('/tenants/syncOci', { params: { tenantId: row.id } })
    msg.close()
    ElMessage.success('同步完成')
    await load()
  } catch (e: any) {
    if (e?.message && e !== 'cancel') ElMessage.error('同步失败: ' + e.message)
  }
}

async function showInstances(row: Tenant) {
  instTenantId.value = row.id
  instTenantName.value = row.userName || `#${row.id}`
  instVisible.value = true
  instLoading.value = true
  try {
    instances.value = await request.get(`/tenants/${row.id}/instances`) as any[]
  } catch (e: any) { ElMessage.error(e.message) }
  finally { instLoading.value = false }
}

onMounted(load)
</script>

<style scoped>
.tenants-page {
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

.table-card {
  border-radius: var(--radius-md);
  overflow: hidden;
}

.table-card :deep(.el-card__body) {
  padding: 0;
}

.tenant-name:hover {
  text-decoration: underline;
}

.state-cell {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-sm);
}

:deep(.el-collapse-item__header) {
  font-size: var(--text-base);
  font-weight: var(--font-semibold);
  color: var(--accent);
}

:deep(.el-collapse-item__header:hover) {
  color: var(--accent-hover);
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

:deep(.el-collapse) {
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  background: var(--bg-surface);
}

:deep(.el-collapse-item) {
  border-bottom: 1px solid var(--border-subtle);
}

:deep(.el-collapse-item:last-child) {
  border-bottom: none;
}

@media (max-width: 768px) {
  .toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .toolbar-left h2 {
    font-size: var(--text-lg);
  }

  .toolbar-right {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>