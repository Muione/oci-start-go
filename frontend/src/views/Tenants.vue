<template>
  <div>
    <div class="toolbar">
      <h2>租户管理</h2>
      <el-button type="primary" @click="openAdd">新增租户</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table :data="rows" v-loading="loading" border style="width: 100%">
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
    <el-dialog v-model="addVisible" title="新增租户" width="640px">
      <el-form :model="form" label-width="120px">
        <el-form-item label="Tenancy OCID"><el-input v-model="form.tenancy" /></el-form-item>
        <el-form-item label="User OCID"><el-input v-model="form.tenantId" /></el-form-item>
        <el-form-item label="指纹"><el-input v-model="form.fingerprint" /></el-form-item>
        <el-form-item label="区域">
          <el-select v-model="form.region" filterable allow-create placeholder="选择或输入区域">
            <el-option v-for="r in regions" :key="r.code" :label="r.name" :value="r.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="用户名(可选)"><el-input v-model="form.userName" placeholder="留空自动生成 区域码_随机" /></el-form-item>
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

// A subset of common OCI regions (code ↔ friendly name); backend resolves
// the friendly name to the code via the full RegionEnum map.
const regions: Region[] = [
  { code: 'ap-tokyo-1', name: '东京' },
  { code: 'ap-osaka-1', name: '大阪' },
  { code: 'ap-singapore-1', name: '新加坡' },
  { code: 'ap-seoul-1', name: '首尔' },
  { code: 'us-ashburn-1', name: '阿什本' },
  { code: 'us-phoenix-1', name: '凤凰城' },
  { code: 'eu-frankfurt-1', name: '法兰克福' },
  { code: 'uk-london-1', name: '伦敦' },
]

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
.toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
.toolbar h2 { margin: 0; margin-right: auto; }
</style>
