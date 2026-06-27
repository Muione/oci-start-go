<template>
  <div>
    <div class="toolbar">
      <h2 class="page-title">代理管理</h2>
      <el-button type="primary" @click="openAddDialog">新增代理</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table :data="rows" border empty-text="暂无代理，请新增">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="proxyType" label="类型" width="90">
        <template #default="{ row }">
          <el-tag :type="row.proxyType === 'SOCKS5' ? 'success' : 'warning'" size="small">{{ row.proxyType }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="proxyHost" label="地址" min-width="160" />
      <el-table-column prop="proxyPort" label="端口" width="80" />
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.availableStatus === 1 ? 'success' : 'info'" size="small">{{ row.availableStatus === 1 ? '启用' : '禁用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="160">
        <template #default="{ row }">
          <el-button size="small" @click="openEditDialog(row)">编辑</el-button>
          <el-button size="small" type="danger" plain @click="removeProxy(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑代理' : '新增代理'" width="480px" destroy-on-close>
      <el-form :model="form" label-width="80px">
        <el-form-item label="类型">
          <el-select v-model="form.proxyType" style="width:100%">
            <el-option label="SOCKS5（推荐）" value="SOCKS5" />
            <el-option label="HTTP" value="HTTP" />
            <el-option label="HTTPS" value="HTTPS" />
          </el-select>
        </el-form-item>
        <el-form-item label="地址">
          <el-input v-model="form.proxyHost" placeholder="代理服务器 IP 或域名" />
        </el-form-item>
        <el-form-item label="端口">
          <el-input-number v-model="form.proxyPort" :min="1" :max="65535" style="width:100%" />
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model="form.proxyUsername" placeholder="选填" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.proxyPassword" placeholder="选填" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveProxy">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../utils/request'

interface Proxy {
  id: number; proxyType: string; proxyHost: string; proxyPort: number;
  proxyUsername: string; proxyPassword: string; availableStatus: number;
}

const rows = ref<Proxy[]>([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const editingId = ref(0)
const form = reactive({ proxyType: 'SOCKS5', proxyHost: '', proxyPort: 1080, proxyUsername: '', proxyPassword: '' })

async function load() {
  loading.value = true
  try { rows.value = (await request.get('/proxies/list')) as Proxy[] } catch (e: any) { ElMessage.error(e.message) }
  finally { loading.value = false }
}

function resetForm() {
  editingId.value = 0
  Object.assign(form, { proxyType: 'SOCKS5', proxyHost: '', proxyPort: 1080, proxyUsername: '', proxyPassword: '' })
}

function openAddDialog() { resetForm(); dialogVisible.value = true }
function openEditDialog(row: Proxy) {
  editingId.value = row.id
  Object.assign(form, { proxyType: row.proxyType, proxyHost: row.proxyHost, proxyPort: row.proxyPort, proxyUsername: row.proxyUsername || '', proxyPassword: row.proxyPassword || '' })
  dialogVisible.value = true
}

async function saveProxy() {
  if (!form.proxyHost) { ElMessage.warning('请输入代理地址'); return }
  saving.value = true
  try {
    const fd = new FormData()
    if (editingId.value) fd.append('id', String(editingId.value))
    fd.append('proxyType', form.proxyType); fd.append('proxyHost', form.proxyHost)
    fd.append('proxyPort', String(form.proxyPort))
    fd.append('proxyUsername', form.proxyUsername); fd.append('proxyPassword', form.proxyPassword)
    await request.post('/proxies/save', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    ElMessage.success('保存成功'); dialogVisible.value = false; await load()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { saving.value = false }
}

async function removeProxy(row: Proxy) {
  try { await ElMessageBox.confirm(`确定删除代理 ${row.proxyHost}:${row.proxyPort}？`, '确认删除', { type: 'warning' }) } catch { return }
  try {
    await request.get('/proxies/delete', { params: { id: row.id } })
    ElMessage.success('已删除'); await load()
  } catch (e: any) { ElMessage.error(e.message) }
}

onMounted(load)
</script>

<style scoped>
.toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
.page-title { margin: 0; margin-right: auto; font-size: 22px; color: #303133; }
</style>
