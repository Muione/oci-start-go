<template>
  <div>
    <div class="toolbar">
      <h2>租户管理</h2>
      <el-button type="primary" @click="openAdd">新增租户</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table :data="rows" v-loading="loading" border style="width: 100%">
      <template #empty>
        <el-empty description="暂无租户，请新增" :image-size="80" />
      </template>
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="userName" label="用户名" min-width="160" />
      <el-table-column prop="tenancy" label="Tenancy OCID" min-width="220" show-overflow-tooltip />
      <el-table-column prop="region" label="区域" width="100">
        <template #default="{ row }">{{ row.regionName || row.region }}</template>
      </el-table-column>
      <el-table-column prop="fingerprint" label="指纹" min-width="180" show-overflow-tooltip />
      <el-table-column label="已同步" width="90">
        <template #default="{ row }">
          <el-tag :type="row.apiSynced ? 'success' : 'info'">{{ row.apiSynced ? '是' : '否' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="syncOci(row)">同步</el-button>
          <el-button size="small" @click="showInstances(row)">实例</el-button>
          <el-button size="small" type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- add dialog -->
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
        <el-form-item label="Tenancy OCID">
          <el-input v-model="form.tenancy" placeholder="ocid1.tenancy.oc1..xxxxx" />
        </el-form-item>
        <el-form-item label="User OCID">
          <el-input v-model="form.tenantId" placeholder="ocid1.user.oc1..xxxxx" />
        </el-form-item>
        <el-form-item label="指纹">
          <el-input v-model="form.fingerprint" placeholder="3a:37:17:38:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx" />
        </el-form-item>
        <el-form-item label="区域">
          <el-select v-model="form.region" filterable allow-create placeholder="选择或输入区域代码">
            <el-option v-for="r in regions" :key="r.code" :label="`${r.name} (${r.code})`" :value="r.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="用户名(可选)">
          <el-input v-model="form.userName" placeholder="留空自动生成 区域码_随机" />
        </el-form-item>
        <el-form-item label="API 私钥">
          <input ref="keyFile" type="file" @change="onFile" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <!-- instances dialog -->
    <el-dialog v-model="instVisible" :title="`实例列表 (租户 ${instTenantId})`" width="90%">
      <el-table :data="instances" border>
        <el-table-column prop="displayName" label="名称" min-width="160" />
        <el-table-column prop="shape" label="Shape" min-width="160" />
        <el-table-column prop="state" label="状态" width="120" />
        <el-table-column prop="publicIps" label="公网IP" width="140" />
        <el-table-column prop="architecture" label="架构" width="80" />
        <el-table-column prop="availabilityDomain" label="AD" min-width="120" show-overflow-tooltip />
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../utils/request'

interface Tenant { id: number; userName: string; tenancy: string; region: string; regionName: string; fingerprint: string; apiSynced: boolean }
interface Region { code: string; name: string }

const rows = ref<Tenant[]>([])
const loading = ref(false)
const addVisible = ref(false)
const saving = ref(false)
const keyFile = ref<HTMLInputElement | null>(null)
const fileBytes = ref<File | null>(null)
const instVisible = ref(false)
const instTenantId = ref<number>(0)
const instances = ref<any[]>([])

const form = ref({ tenancy: '', tenantId: '', fingerprint: '', region: '', userName: '' })

// OCI config import
const configCollapse = ref<string[]>([])
const ociConfigText = ref('')

// Full OCI region list (code ↔ friendly name).
const regions: Region[] = [
  // Asia Pacific
  { code: 'ap-tokyo-1', name: '东京' },
  { code: 'ap-osaka-1', name: '大阪' },
  { code: 'ap-seoul-1', name: '首尔' },
  { code: 'ap-singapore-1', name: '新加坡' },
  { code: 'ap-singapore-2', name: '新加坡(西)' },
  { code: 'ap-mumbai-1', name: '孟买' },
  { code: 'ap-hyderabad-1', name: '海得拉巴' },
  { code: 'ap-sydney-1', name: '悉尼' },
  { code: 'ap-melbourne-1', name: '墨尔本' },
  { code: 'ap-chuncheon-1', name: '春川' },
  { code: 'ap-osaka-2', name: '大阪(第2)' },
  // North America
  { code: 'us-ashburn-1', name: '阿什本' },
  { code: 'us-phoenix-1', name: '凤凰城' },
  { code: 'us-sanjose-1', name: '圣何塞' },
  { code: 'us-sanjose-2', name: '圣何塞(第2)' },
  { code: 'us-chicago-1', name: '芝加哥' },
  { code: 'us-phoenix-2', name: '凤凰城(第2)' },
  { code: 'us-ashburn-2', name: '阿什本(第2)' },
  // Latin America
  { code: 'sa-saopaulo-1', name: '圣保罗' },
  { code: 'sa-vinhedo-1', name: '维涅杜' },
  { code: 'mx-queretaro-1', name: '克雷塔罗' },
  { code: 'mx-queretaro-2', name: '克雷塔罗(第2)' },
  // Europe
  { code: 'eu-frankfurt-1', name: '法兰克福' },
  { code: 'eu-frankfurt-2', name: '法兰克福(第2)' },
  { code: 'uk-london-1', name: '伦敦' },
  { code: 'uk-cardiff-1', name: '加的夫' },
  { code: 'eu-zurich-1', name: '苏黎世' },
  { code: 'eu-amsterdam-1', name: '阿姆斯特丹' },
  { code: 'eu-madrid-1', name: '马德里' },
  { code: 'eu-milan-1', name: '米兰' },
  // Middle East & Africa
  { code: 'me-jeddah-1', name: '吉达' },
  { code: 'me-dubai-1', name: '迪拜' },
  { code: 'me-abudhabi-1', name: '阿布扎比' },
  { code: 'af-johannesburg-1', name: '约翰内斯堡' },
]

// OCI region code → friendly name lookup
const regionCodeToName: Record<string, string> = {}
regions.forEach(r => { regionCodeToName[r.code] = r.name })

// Parse OCI config text and fill the form.
// Expected format (INI-style):
//   [DEFAULT]
//   user=ocid1.user.oc1..xxx
//   fingerprint=3a:37:...
//   tenancy=ocid1.tenancy.oc1..xxx
//   region=ap-singapore-2
function parseOciConfig() {
  const text = ociConfigText.value.trim()
  if (!text) {
    ElMessage.warning('请先粘贴 OCI 配置文件内容')
    return
  }

  // Parse key=value pairs from [DEFAULT] section
  const lines = text.split('\n')
  let inDefault = false
  const kv: Record<string, string> = {}

  for (const raw of lines) {
    const line = raw.trim()
    if (!line || line.startsWith('#') || line.startsWith(';')) continue
    // Section header
    if (line.startsWith('[') && line.endsWith(']')) {
      inDefault = line.toLowerCase().includes('default')
      continue
    }
    if (!inDefault && text.includes('[')) continue // skip non-DEFAULT sections
    // key = value (also handle key=value without spaces)
    const eq = line.indexOf('=')
    if (eq === -1) continue
    const key = line.substring(0, eq).trim().toLowerCase()
    const value = line.substring(eq + 1).trim()
    if (key && value) kv[key] = value
  }

  // Map OCI config keys to form fields
  let filled = 0
  if (kv['user'] && !form.value.tenantId) { form.value.tenantId = kv['user']; filled++ }
  if (kv['fingerprint'] && !form.value.fingerprint) { form.value.fingerprint = kv['fingerprint']; filled++ }
  if (kv['tenancy'] && !form.value.tenancy) { form.value.tenancy = kv['tenancy']; filled++ }
  if (kv['region'] && !form.value.region) {
    const code = kv['region']
    // Try to resolve region code to friendly name
    form.value.region = regionCodeToName[code] || code
    filled++
  }
  // Auto-generate username from region if not set
  if (kv['region'] && !form.value.userName) {
    const code = kv['region']
    const suffix = Math.random().toString(36).substring(2, 6)
    form.value.userName = `${code.replace(/-/g, '_')}_${suffix}`
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
    rows.value = await request.get('/tenants/listAll') as Tenant[]
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
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
  fileBytes.value = t.files && t.files[0] ? t.files[0] : null
}

async function save() {
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
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function remove(row: Tenant) {
  try {
    await ElMessageBox.confirm(`删除租户 ${row.userName}?`, '确认', { type: 'warning' })
    await request.get('/tenants/deleteApi', { params: { tenantId: row.id } })
    ElMessage.success('已删除')
    await load()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

async function syncOci(row: Tenant) {
  try {
    await ElMessageBox.confirm(`从 OCI 同步租户 ${row.userName} 的实例?`, '确认', { type: 'info' })
    ElMessage.info('同步中…（可能需要数秒）')
    await request.get('/tenants/syncOci', { params: { tenantId: row.id } })
    ElMessage.success('同步完成')
    await load()
  } catch (e: any) {
    if (e?.message) ElMessage.error('同步失败: ' + e.message)
  }
}

async function showInstances(row: Tenant) {
  instTenantId.value = row.id
  instVisible.value = true
  try {
    instances.value = await request.get(`/tenants/${row.id}/instances`) as any[]
  } catch (e: any) {
    ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 24px;
  flex-wrap: wrap;
}

.toolbar h2 {
  margin: 0;
  margin-right: auto;
  font-size: 24px;
  font-weight: 700;
  color: #1e293b;
  background: linear-gradient(135deg, #0066ff, #00bcd4);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

:deep(.el-collapse-item__header) {
  font-size: 14px;
  font-weight: 600;
  color: #0066ff;
  transition: all 0.3s ease;
}

:deep(.el-collapse-item__header:hover) {
  color: #00bcd4;
}

:deep(.el-table) {
  border-radius: 12px;
  overflow: hidden;
  background: #ffffff;
  border: 1px solid rgba(0, 102, 255, 0.1);
}

:deep(.el-table__header) {
  background: linear-gradient(90deg, rgba(0, 102, 255, 0.03), rgba(0, 188, 212, 0.03));
}

:deep(.el-table__header th) {
  background: transparent;
  color: #1e293b;
  font-weight: 600;
  border-bottom: 2px solid rgba(0, 102, 255, 0.1);
}

:deep(.el-table__body tr:hover > td) {
  background-color: rgba(0, 102, 255, 0.05);
}

:deep(.el-dialog) {
  border-radius: 16px;
}

:deep(.el-dialog__header) {
  border-bottom: 1px solid rgba(0, 102, 255, 0.1);
}

:deep(.el-dialog__title) {
  font-size: 18px;
  font-weight: 700;
  color: #1e293b;
}

:deep(.el-form-item__label) {
  color: #1e293b;
  font-weight: 600;
}

:deep(.el-alert) {
  border-radius: 12px;
  border: none;
  box-shadow: 0 4px 12px rgba(64, 158, 255, 0.15);
}

:deep(.el-button--small) {
  border-radius: 6px;
  transition: all 0.3s ease;
}

:deep(.el-button--primary:hover) {
  box-shadow: 0 8px 16px rgba(0, 102, 255, 0.3);
  transform: translateY(-1px);
}

:deep(.el-button--danger:hover) {
  box-shadow: 0 8px 16px rgba(239, 68, 68, 0.3);
  transform: translateY(-1px);
}

:deep(.el-tag) {
  border-radius: 8px;
  border: none;
  font-weight: 600;
}

:deep(.el-collapse) {
  border: 1px solid rgba(0, 102, 255, 0.1);
  border-radius: 12px;
  background: linear-gradient(90deg, rgba(0, 102, 255, 0.02), rgba(0, 188, 212, 0.02));
}

:deep(.el-collapse-item) {
  border-bottom: 1px solid rgba(0, 102, 255, 0.1);
}

:deep(.el-collapse-item:last-child) {
  border-bottom: none;
}

:deep(.el-collapse-item__wrap) {
  background: #ffffff;
}
</style>
