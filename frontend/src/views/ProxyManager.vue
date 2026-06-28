<template>
  <div class="proxy-page">
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>代理管理</h2>
        <el-tag type="info" size="small">{{ rows.length }} 个代理</el-tag>
      </div>
      <div class="toolbar-right">
        <el-button type="primary" @click="openAddDialog">
          <el-icon><Plus /></el-icon> 新增代理
        </el-button>
        <el-button @click="load" :loading="loading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </div>
    </div>

    <el-card shadow="none" class="table-card">
      <el-table :data="rows" v-loading="loading" border stripe>
        <template #empty>
          <el-empty description="暂无代理" :image-size="80">
            <el-button type="primary" @click="openAddDialog">新增代理</el-button>
          </el-empty>
        </template>
        <el-table-column prop="id" label="ID" width="60" align="center" />
        <el-table-column label="类型" width="100" align="center">
          <template #default="{ row }">
            <el-tag
              :type="row.proxyType === 'SOCKS5' ? 'success' : row.proxyType === 'HTTP' ? 'warning' : 'info'"
              size="small"
              effect="dark"
            >
              {{ row.proxyType }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="proxyHost" label="地址" min-width="180" />
        <el-table-column prop="proxyPort" label="端口" width="80" align="center" />
        <el-table-column label="认证" width="100" align="center">
          <template #default="{ row }">
            <el-icon v-if="row.proxyUsername" color="var(--status-up)"><Lock /></el-icon>
            <el-icon v-else color="var(--text-muted)"><Unlock /></el-icon>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="120" align="center">
          <template #default="{ row }">
            <el-switch
              :model-value="row.availableStatus === 1"
              @change="(val: boolean) => toggleStatus(row, val)"
              active-text="启用"
              inactive-text="禁用"
              size="small"
            />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openEditDialog(row)">
              <el-icon><Edit /></el-icon>
            </el-button>
            <el-popconfirm title="确定删除此代理？" @confirm="removeProxy(row)">
              <template #reference>
                <el-button size="small" type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Add/Edit Dialog -->
    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑代理' : '新增代理'" width="480px" destroy-on-close>
      <el-form :model="form" label-width="80px">
        <el-form-item label="类型">
          <el-select v-model="form.proxyType" style="width:100%">
            <el-option label="SOCKS5（推荐）" value="SOCKS5" />
            <el-option label="HTTP" value="HTTP" />
            <el-option label="HTTPS" value="HTTPS" />
          </el-select>
        </el-form-item>
        <el-form-item label="地址" required>
          <el-input v-model="form.proxyHost" placeholder="代理服务器 IP 或域名" />
        </el-form-item>
        <el-form-item label="端口" required>
          <el-input-number v-model="form.proxyPort" :min="1" :max="65535" style="width:100%" />
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model="form.proxyUsername" placeholder="选填" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.proxyPassword" placeholder="选填" show-password />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch
            v-model="form.availableStatus"
            :active-value="1"
            :inactive-value="0"
            active-text="启用"
            inactive-text="禁用"
          />
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
import { ElMessage } from 'element-plus'
import { Plus, Refresh, Edit, Lock, Unlock } from '@element-plus/icons-vue'
import request from '../utils/request'

interface Proxy {
  id: number; proxyType: string; proxyHost: string; proxyPort: number
  proxyUsername: string; proxyPassword: string; availableStatus: number
}

const rows = ref<Proxy[]>([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const editingId = ref(0)
const form = reactive({
  proxyType: 'SOCKS5', proxyHost: '', proxyPort: 1080,
  proxyUsername: '', proxyPassword: '', availableStatus: 1,
})

async function load() {
  loading.value = true
  try { rows.value = await request.get('/proxies/list') as Proxy[] }
  catch (e: any) { ElMessage.error(e.message) }
  finally { loading.value = false }
}

function resetForm() {
  editingId.value = 0
  Object.assign(form, {
    proxyType: 'SOCKS5', proxyHost: '', proxyPort: 1080,
    proxyUsername: '', proxyPassword: '', availableStatus: 1,
  })
}

function openAddDialog() { resetForm(); dialogVisible.value = true }

function openEditDialog(row: Proxy) {
  editingId.value = row.id
  Object.assign(form, {
    proxyType: row.proxyType || 'SOCKS5',
    proxyHost: row.proxyHost,
    proxyPort: row.proxyPort,
    proxyUsername: row.proxyUsername || '',
    proxyPassword: row.proxyPassword || '',
    availableStatus: row.availableStatus,
  })
  dialogVisible.value = true
}

async function saveProxy() {
  if (!form.proxyHost) { ElMessage.warning('请输入代理地址'); return }
  saving.value = true
  try {
    const fd = new FormData()
    if (editingId.value) fd.append('id', String(editingId.value))
    fd.append('proxyType', form.proxyType)
    fd.append('proxyHost', form.proxyHost)
    fd.append('proxyPort', String(form.proxyPort))
    fd.append('proxyUsername', form.proxyUsername)
    fd.append('proxyPassword', form.proxyPassword)
    fd.append('availableStatus', String(form.availableStatus))
    await request.post('/proxies/save', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    ElMessage.success('保存成功')
    dialogVisible.value = false
    await load()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { saving.value = false }
}

async function removeProxy(row: Proxy) {
  try {
    await request.get('/proxies/delete', { params: { id: row.id } })
    ElMessage.success('已删除')
    await load()
  } catch (e: any) { ElMessage.error(e.message) }
}

async function toggleStatus(row: Proxy, val: boolean) {
  try {
    const fd = new FormData()
    fd.append('id', String(row.id))
    fd.append('proxyType', row.proxyType)
    fd.append('proxyHost', row.proxyHost)
    fd.append('proxyPort', String(row.proxyPort))
    fd.append('proxyUsername', row.proxyUsername || '')
    fd.append('proxyPassword', row.proxyPassword || '')
    fd.append('availableStatus', val ? '1' : '0')
    await request.post('/proxies/save', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    row.availableStatus = val ? 1 : 0
    ElMessage.success(val ? '已启用' : '已禁用')
  } catch (e: any) { ElMessage.error(e.message) }
}

onMounted(load)
</script>

<style scoped>
.proxy-page {
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
  margin-top: var(--space-6);
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