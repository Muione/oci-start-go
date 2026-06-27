<template>
  <div class="dns-page">
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>DNS 记录管理</h2>
        <el-tag type="info" size="small">{{ rows.length }} 条记录</el-tag>
      </div>
      <div class="toolbar-right">
        <el-button @click="syncAll" :loading="syncing">
          <el-icon><Refresh /></el-icon> 同步 Cloudflare
        </el-button>
        <el-button type="primary" @click="openAdd">
          <el-icon><Plus /></el-icon> 添加记录
        </el-button>
        <el-button @click="load">
          <el-icon><RefreshRight /></el-icon> 刷新
        </el-button>
      </div>
    </div>

    <el-card shadow="hover" class="table-card">
      <el-table :data="rows" v-loading="loading" border stripe style="width: 100%" size="default">
        <template #empty>
          <el-empty description="暂无 DNS 记录，请点击「同步 Cloudflare」或手动添加" :image-size="80">
            <el-button type="primary" @click="syncAll">立即同步</el-button>
          </el-empty>
        </template>
        <el-table-column prop="domainName" label="域名" min-width="180" show-overflow-tooltip />
        <el-table-column prop="recordName" label="记录名称" min-width="160" show-overflow-tooltip />
        <el-table-column label="类型" width="80">
          <template #default="{ row }">
            <el-tag :type="typeColor(row.recordType)" size="small" effect="dark">{{ row.recordType }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="recordValue" label="记录值" min-width="200" show-overflow-tooltip />
        <el-table-column label="TTL" width="80" align="center">
          <template #default="{ row }">{{ row.ttl || '-' }}</template>
        </el-table-column>
        <el-table-column label="代理" width="70" align="center">
          <template #default="{ row }">
            <el-icon v-if="row.proxied" color="#67c23a"><CircleCheck /></el-icon>
            <el-icon v-else color="#c0c4cc"><Remove /></el-icon>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'warning'" size="small">
              {{ row.status || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="providerType" label="来源" width="100" />
        <el-table-column prop="updateTime" label="更新时间" width="150" />
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="primary" link @click="openEdit(row)">编辑</el-button>
            <el-popconfirm title="确定删除此 DNS 记录？" @confirm="doDelete(row)">
              <template #reference>
                <el-button size="small" type="danger" link>删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Add/Edit Dialog -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑 DNS 记录' : '添加 DNS 记录'" width="560px" destroy-on-close>
      <el-form :model="form" label-width="100px">
        <el-form-item label="记录名称" required>
          <el-input v-model="form.recordName" placeholder="www 或 @ 等" />
        </el-form-item>
        <el-form-item label="域名" required>
          <el-input v-model="form.domainName" placeholder="example.com" />
        </el-form-item>
        <el-form-item label="记录类型" required>
          <el-select v-model="form.recordType" style="width:100%">
            <el-option label="A" value="A" />
            <el-option label="AAAA" value="AAAA" />
            <el-option label="CNAME" value="CNAME" />
            <el-option label="MX" value="MX" />
            <el-option label="TXT" value="TXT" />
            <el-option label="NS" value="NS" />
            <el-option label="SRV" value="SRV" />
          </el-select>
        </el-form-item>
        <el-form-item label="记录值" required>
          <el-input v-model="form.recordValue" placeholder="IP 地址或目标域名" />
        </el-form-item>
        <el-form-item label="TTL">
          <el-input-number v-model="form.ttl" :min="1" :max="86400" style="width:100%" />
        </el-form-item>
        <el-form-item label="Cloudflare 代理">
          <el-switch v-model="form.proxied" />
        </el-form-item>
        <el-form-item label="来源">
          <el-select v-model="form.providerType" style="width:100%">
            <el-option label="手动" value="manual" />
            <el-option label="Cloudflare" value="cloudflare" />
            <el-option label="EdgeOne" value="edgeone" />
          </el-select>
        </el-form-item>
        <el-form-item label="Zone ID" v-if="form.providerType === 'cloudflare'">
          <el-input v-model="form.zoneId" placeholder="Cloudflare Zone ID" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="form.status" style="width:100%">
            <el-option label="激活" value="active" />
            <el-option label="停用" value="inactive" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="doSave">
          {{ isEdit ? '保存修改' : '添加记录' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Plus, RefreshRight, CircleCheck, Remove } from '@element-plus/icons-vue'
import request from '../utils/request'

interface DnsRecord {
  id: number
  providerType: string
  domainName: string
  recordName: string
  recordType: string
  recordValue: string
  ttl: number
  proxied: boolean
  status: string
  zoneId: string
  createTime: string
  updateTime: string
}

const rows = ref<DnsRecord[]>([])
const loading = ref(false)
const syncing = ref(false)
const saving = ref(false)
const isEdit = ref(false)
const dialogVisible = ref(false)
const form = ref<DnsRecord>(emptyForm())

function emptyForm(): DnsRecord {
  return {
    id: 0, providerType: 'manual', domainName: '', recordName: '',
    recordType: 'A', recordValue: '', ttl: 120, proxied: false,
    status: 'active', zoneId: '', createTime: '', updateTime: '',
  }
}

function typeColor(type: string): string {
  const map: Record<string, string> = { A: '', AAAA: 'success', CNAME: 'warning', MX: 'danger', TXT: 'info', NS: '', SRV: '' }
  return map[type] || ''
}

async function load() {
  loading.value = true
  try {
    const data = await request.get('/dns/list') as any
    rows.value = Array.isArray(data) ? data : (data?.records || data?.items || [])
  } catch (e: any) { ElMessage.error(e.message)
  } finally { loading.value = false }
}

function openAdd() {
  isEdit.value = false
  form.value = emptyForm()
  dialogVisible.value = true
}

function openEdit(row: DnsRecord) {
  isEdit.value = true
  form.value = { ...row }
  dialogVisible.value = true
}

async function doSave() {
  if (!form.value.recordName || !form.value.domainName || !form.value.recordValue) {
    ElMessage.warning('请填写记录名称、域名和记录值')
    return
  }
  saving.value = true
  try {
    await request.post('/dns/save', form.value)
    ElMessage.success(isEdit.value ? 'DNS 记录已更新' : 'DNS 记录已添加')
    dialogVisible.value = false
    await load()
  } catch (e: any) { ElMessage.error(e.message)
  } finally { saving.value = false }
}

async function doDelete(row: DnsRecord) {
  try {
    await request.get('/dns/delete', { params: { id: row.id } })
    ElMessage.success('DNS 记录已删除')
    await load()
  } catch (e: any) { ElMessage.error(e.message) }
}

async function syncAll() {
  syncing.value = true
  try {
    const res = await request.post('/dns/sync', {}) as any
    ElMessage.success(`同步完成：${res.synced || 0} 条记录`)
    await load()
  } catch (e: any) { ElMessage.error(e.message)
  } finally { syncing.value = false }
}

onMounted(load)
</script>

<style scoped>
.dns-page { padding: 4px 0; }
.toolbar {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 20px; flex-wrap: wrap; gap: 12px;
}
.toolbar-left { display: flex; align-items: center; gap: 12px; }
.toolbar-left h2 { margin: 0; }
.toolbar-right { display: flex; gap: 8px; }
.table-card { border-radius: 8px; }
</style>
