<template>
  <div class="dns-page">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>DNS 管理</h2>
      </div>
      <div class="toolbar-right">
        <el-button @click="loadCfRecords" :loading="cfLoading">
          <el-icon><RefreshRight /></el-icon> 刷新
        </el-button>
      </div>
    </div>

    <!-- Provider tabs -->
    <el-tabs v-model="activeTab" @tab-change="onTabChange" type="border-card">
      <!-- Cloudflare Tab -->
      <el-tab-pane label="Cloudflare" name="cloudflare">
        <div v-if="!cfConfigured" style="text-align:center;padding:48px 0">
          <el-empty description="Cloudflare 未配置" :image-size="80">
            <template #extra>
              <p style="color:var(--text-muted);font-size:13px;margin-bottom:12px">
                请在系统设置中配置 cloudflare.api.token（API Token）
              </p>
              <el-button type="primary" @click="$router.push('/settings')">前往配置</el-button>
            </template>
          </el-empty>
        </div>

        <template v-else>
          <!-- Zone selector -->
          <div class="provider-toolbar">
            <div class="provider-left">
              <span class="provider-label">Zone:</span>
              <el-select v-model="cfZoneId" placeholder="选择 Cloudflare 区域" style="width: 360px" @change="onCfZoneChange">
                <el-option
                  v-for="z in cfZones"
                  :key="z.id"
                  :label="z.name"
                  :value="z.id"
                />
              </el-select>
            </div>
            <div class="provider-right">
              <el-button size="small" @click="loadCfZones" :loading="cfLoadingZones">
                刷新 Zone 列表
              </el-button>
              <el-button v-if="cfZoneId" size="small" type="primary" @click="openCfAdd">
                <el-icon><Plus /></el-icon> 添加记录
              </el-button>
            </div>
          </div>

          <!-- CF Records Table -->
          <el-card shadow="none" class="table-card" style="margin-top:12px">
            <el-table :data="cfRecords" v-loading="cfLoading" border stripe size="default">
              <template #empty>
                <el-empty description="请选择一个 Zone 并加载记录" :image-size="60" />
              </template>
              <el-table-column label="类型" width="80">
                <template #default="{ row }">
                  <el-tag :type="typeTag(row.type)" size="small" effect="dark">{{ row.type }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="name" label="名称" min-width="200" show-overflow-tooltip />
              <el-table-column prop="content" label="内容" min-width="200" show-overflow-tooltip />
              <el-table-column label="TTL" width="80" align="center">
                <template #default="{ row }">{{ row.ttl === 1 ? 'Auto' : row.ttl }}</template>
              </el-table-column>
              <el-table-column label="代理" width="70" align="center">
                <template #default="{ row }">
                  <el-icon v-if="row.proxied" color="var(--status-up)"><CircleCheck /></el-icon>
                  <el-icon v-else color="var(--text-muted)"><Remove /></el-icon>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="180" fixed="right">
                <template #default="{ row }">
                  <el-button size="small" type="primary" link @click="openCfEdit(row)">编辑</el-button>
                  <el-popconfirm title="确定删除此 DNS 记录？" @confirm="deleteCfRecord(row)">
                    <template #reference>
                      <el-button size="small" type="danger" link>删除</el-button>
                    </template>
                  </el-popconfirm>
                </template>
              </el-table-column>
            </el-table>

            <!-- CF Pagination -->
            <div v-if="cfTotalPages > 1" style="display:flex;justify-content:center;margin-top:16px">
              <el-pagination
                v-model:current-page="cfPage"
                :page-size="cfPageSize"
                :total="cfTotalCount"
                layout="prev, pager, next, total"
                @current-change="loadCfRecords"
              />
            </div>
          </el-card>
        </template>
      </el-tab-pane>

      <!-- EdgeOne Tab -->
      <el-tab-pane label="EdgeOne" name="edgeone">
        <div v-if="!eoConfigured" style="text-align:center;padding:48px 0">
          <el-empty description="EdgeOne 未配置" :image-size="80">
            <template #extra>
              <p style="color:var(--text-muted);font-size:13px;margin-bottom:12px">
                请在系统设置中配置 edgeone.secretId、edgeone.secretKey 和 edgeone.zoneId
              </p>
              <el-button type="primary" @click="$router.push('/settings')">前往配置</el-button>
            </template>
          </el-empty>
        </div>

        <template v-else>
          <div class="provider-toolbar">
            <div class="provider-left">
              <el-button size="small" type="primary" @click="openEoAdd">
                <el-icon><Plus /></el-icon> 添加记录
              </el-button>
            </div>
            <el-button size="small" @click="loadEoRecords" :loading="eoLoading">刷新</el-button>
          </div>

          <el-card shadow="none" class="table-card" style="margin-top:12px">
            <el-table :data="eoRecords" v-loading="eoLoading" border stripe size="default">
              <template #empty>
                <el-empty description="暂无 EdgeOne 记录" :image-size="60" />
              </template>
              <el-table-column label="类型" width="80">
                <template #default="{ row }">
                  <el-tag :type="typeTag(row.type)" size="small" effect="dark">{{ row.type }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="name" label="名称" min-width="200" show-overflow-tooltip />
              <el-table-column prop="content" label="内容" min-width="200" show-overflow-tooltip />
              <el-table-column label="TTL" width="80" align="center">
                <template #default="{ row }">{{ row.ttl || '-' }}</template>
              </el-table-column>
              <el-table-column label="优先级" width="80" align="center">
                <template #default="{ row }">{{ row.priority || '-' }}</template>
              </el-table-column>
              <el-table-column label="操作" width="180" fixed="right">
                <template #default="{ row }">
                  <el-button size="small" type="primary" link @click="openEoEdit(row)">编辑</el-button>
                  <el-popconfirm title="确定删除此记录？" @confirm="deleteEoRecord(row)">
                    <template #reference>
                      <el-button size="small" type="danger" link>删除</el-button>
                    </template>
                  </el-popconfirm>
                </template>
              </el-table-column>
            </el-table>
          </el-card>
        </template>
      </el-tab-pane>
    </el-tabs>

    <!-- ================================================================ -->
    <!-- Cloudflare Add/Edit Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="cfDialogVisible" :title="cfIsEdit ? '编辑 Cloudflare 记录' : '添加 Cloudflare 记录'" width="520px" destroy-on-close>
      <el-form :model="cfForm" label-width="100px">
        <el-form-item label="类型" required>
          <el-select v-model="cfForm.type" style="width:100%">
            <el-option v-for="t in dnsTypes" :key="t" :label="t" :value="t" />
          </el-select>
        </el-form-item>
        <el-form-item label="名称" required>
          <el-input v-model="cfForm.name" placeholder="www 或 @ 等" />
        </el-form-item>
        <el-form-item label="内容" required>
          <el-input v-model="cfForm.content" placeholder="IP 地址或目标域名" />
        </el-form-item>
        <el-form-item label="TTL">
          <el-input-number v-model="cfForm.ttl" :min="1" :max="86400" style="width:100%" />
          <div style="font-size:12px;color:var(--text-muted);margin-top:4px">设为 1 表示自动</div>
        </el-form-item>
        <el-form-item label="代理">
          <el-switch v-model="cfForm.proxied" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="cfDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="cfSaving" @click="doCfSave">
          {{ cfIsEdit ? '更新' : '添加' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- EdgeOne Add/Edit Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="eoDialogVisible" :title="eoIsEdit ? '编辑 EdgeOne 记录' : '添加 EdgeOne 记录'" width="520px" destroy-on-close>
      <el-form :model="eoForm" label-width="100px">
        <el-form-item label="类型" required>
          <el-select v-model="eoForm.type" style="width:100%">
            <el-option v-for="t in dnsTypes" :key="t" :label="t" :value="t" />
          </el-select>
        </el-form-item>
        <el-form-item label="名称" required>
          <el-input v-model="eoForm.name" placeholder="www 或 @ 等" />
        </el-form-item>
        <el-form-item label="内容" required>
          <el-input v-model="eoForm.content" placeholder="IP 地址或目标域名" />
        </el-form-item>
        <el-form-item label="TTL">
          <el-input-number v-model="eoForm.ttl" :min="1" :max="86400" style="width:100%" />
        </el-form-item>
        <el-form-item label="优先级 (MX)">
          <el-input-number v-model="eoForm.priority" :min="0" :max="65535" style="width:100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="eoDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="eoSaving" @click="doEoSave">
          {{ eoIsEdit ? '更新' : '添加' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus, RefreshRight, CircleCheck, Remove } from '@element-plus/icons-vue'
import request from '../utils/request'

// ---- Types ----
interface CfRecord {
  id: string; zone_id: string; zone_name?: string
  type: string; name: string; content: string
  ttl: number; proxied: boolean; created_on?: string; modified_on?: string
}

interface CfZone {
  id: string; name: string; status: string; paused: boolean
}

interface EoRecord {
  RecordId?: string; Name?: string; Type?: string
  Content?: string; TTL?: number; Priority?: number
  recordId?: string; name?: string; type?: string
  content?: string; ttl?: number; priority?: number
}

const dnsTypes = ['A', 'AAAA', 'CNAME', 'MX', 'TXT', 'NS', 'SRV', 'CAA', 'PTR']

// ---- Cloudflare State ----
const cfConfigured = ref(false)
const cfZones = ref<CfZone[]>([])
const cfZoneId = ref('')
const cfRecords = ref<CfRecord[]>([])
const cfLoading = ref(false)
const cfLoadingZones = ref(false)
const cfSaving = ref(false)
const cfPage = ref(1)
const cfPageSize = ref(20)
const cfTotalPages = ref(0)
const cfTotalCount = ref(0)
const cfDialogVisible = ref(false)
const cfIsEdit = ref(false)
const cfForm = ref({ type: 'A', name: '', content: '', ttl: 1, proxied: false })
const cfEditId = ref('')

// ---- EdgeOne State ----
const eoConfigured = ref(false)
const eoRecords = ref<EoRecord[]>([])
const eoLoading = ref(false)
const eoSaving = ref(false)
const eoDialogVisible = ref(false)
const eoIsEdit = ref(false)
const eoForm = ref({ type: 'A', name: '', content: '', ttl: 600, priority: 0 })
const eoEditId = ref('')

const activeTab = ref('cloudflare')

function typeTag(type: string): string {
  const map: Record<string, string> = {
    A: '', AAAA: 'success', CNAME: 'warning', MX: 'danger',
    TXT: 'info', NS: '', SRV: '', CAA: '', PTR: '',
  }
  return map[type] || ''
}

async function onTabChange(tab: string) {
  if (tab === 'cloudflare') { await loadCfZones() }
  else if (tab === 'edgeone') { await loadEoRecords() }
}

// ---- Cloudflare Operations ----
async function loadCfZones() {
  cfLoadingZones.value = true
  try {
    cfZones.value = await request.get('/dns/cloudflare/zones') as CfZone[]
    cfConfigured.value = true
    // Auto-select first zone
    if (!cfZoneId.value && cfZones.value.length > 0) {
      cfZoneId.value = cfZones.value[0].id
      await loadCfRecords()
    }
  } catch (e: any) {
    if (e.message?.includes('not configured') || e.message?.includes('未配置')) {
      cfConfigured.value = false
    } else {
      ElMessage.error(e.message)
    }
  }
  finally { cfLoadingZones.value = false }
}

// onCfZoneChange is called when the user switches zones.
// Resets pagination to page 1 and reloads records — fixes the bug where
// switching zones with a stale page number returned blank results.
function onCfZoneChange() {
  cfPage.value = 1
  loadCfRecords()
}

async function loadCfRecords() {
  if (!cfZoneId.value) return
  cfLoading.value = true
  try {
    const params: any = { page: cfPage.value, perPage: cfPageSize.value }
    const res = await request.get(`/dns/cloudflare/zones/${cfZoneId.value}/records`, { params }) as any
    cfRecords.value = res.records || []
    cfTotalPages.value = res.totalPages || 0
    cfTotalCount.value = res.totalCount || 0
  } catch (e: any) { ElMessage.error(e.message) }
  finally { cfLoading.value = false }
}

function openCfAdd() {
  cfIsEdit.value = false
  cfEditId.value = ''
  cfForm.value = { type: 'A', name: '', content: '', ttl: 1, proxied: false }
  cfDialogVisible.value = true
}

function openCfEdit(row: CfRecord) {
  cfIsEdit.value = true
  cfEditId.value = row.id
  cfForm.value = {
    type: row.type, name: row.name, content: row.content,
    ttl: row.ttl, proxied: row.proxied,
  }
  cfDialogVisible.value = true
}

async function doCfSave() {
  if (!cfForm.value.name || !cfForm.value.content) {
    ElMessage.warning('请填写名称和内容'); return
  }
  cfSaving.value = true
  try {
    if (cfIsEdit.value) {
      await request.put(`/dns/cloudflare/zones/${cfZoneId.value}/records/${cfEditId.value}`, cfForm.value)
      ElMessage.success('记录已更新')
    } else {
      await request.post(`/dns/cloudflare/zones/${cfZoneId.value}/records`, cfForm.value)
      ElMessage.success('记录已创建')
    }
    cfDialogVisible.value = false
    await loadCfRecords()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { cfSaving.value = false }
}

async function deleteCfRecord(row: CfRecord) {
  try {
    await request.delete(`/dns/cloudflare/zones/${cfZoneId.value}/records/${row.id}`)
    ElMessage.success('记录已删除')
    await loadCfRecords()
  } catch (e: any) { ElMessage.error(e.message) }
}

// ---- EdgeOne Operations ----
async function loadEoRecords() {
  eoLoading.value = true
  try {
    eoRecords.value = await request.get('/dns/edgeone/records') as EoRecord[]
    eoConfigured.value = true
  } catch (e: any) {
    if (e.message?.includes('not configured') || e.message?.includes('未配置')) {
      eoConfigured.value = false
    } else {
      ElMessage.error(e.message)
    }
  }
  finally { eoLoading.value = false }
}

function openEoAdd() {
  eoIsEdit.value = false
  eoEditId.value = ''
  eoForm.value = { type: 'A', name: '', content: '', ttl: 600, priority: 0 }
  eoDialogVisible.value = true
}

function openEoEdit(row: EoRecord) {
  eoIsEdit.value = true
  eoEditId.value = row.RecordId || row.recordId || ''
  eoForm.value = {
    type: row.Type || row.type || 'A',
    name: row.Name || row.name || '',
    content: row.Content || row.content || '',
    ttl: row.TTL || row.ttl || 600,
    priority: row.Priority || row.priority || 0,
  }
  eoDialogVisible.value = true
}

async function doEoSave() {
  if (!eoForm.value.name || !eoForm.value.content) {
    ElMessage.warning('请填写名称和内容'); return
  }
  eoSaving.value = true
  try {
    if (eoIsEdit.value) {
      await request.put(`/dns/edgeone/records/${eoEditId.value}`, eoForm.value)
      ElMessage.success('记录已更新')
    } else {
      await request.post('/dns/edgeone/records', eoForm.value)
      ElMessage.success('记录已创建')
    }
    eoDialogVisible.value = false
    await loadEoRecords()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { eoSaving.value = false }
}

async function deleteEoRecord(row: EoRecord) {
  const id = row.RecordId || row.recordId
  if (!id) { ElMessage.error('无法获取记录ID'); return }
  try {
    await request.delete(`/dns/edgeone/records/${id}`)
    ElMessage.success('记录已删除')
    await loadEoRecords()
  } catch (e: any) { ElMessage.error(e.message) }
}

onMounted(() => { loadCfZones() })
</script>

<style scoped>
.dns-page {
  padding: 0;
}

/* ---- Toolbar ---- */
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

/* ---- Provider Toolbar ---- */
.provider-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  flex-wrap: wrap;
}

.provider-left {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.provider-label {
  font-size: var(--text-sm);
  font-weight: var(--font-semibold);
  color: var(--text-secondary);
}

.provider-right {
  display: flex;
  gap: var(--space-2);
}

/* ---- Table card ---- */
.table-card {
  border-radius: var(--radius-md);
  overflow: hidden;
}

.table-card :deep(.el-card__body) {
  padding: 0;
}

/* ---- Element Plus overrides ---- */
:deep(.el-tabs--border-card) {
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  background: var(--bg-surface);
  box-shadow: none;
}

:deep(.el-tabs--border-card > .el-tabs__header) {
  background: var(--bg-raised);
  border-bottom: 1px solid var(--border-default);
}

:deep(.el-tabs--border-card > .el-tabs__content) {
  padding: var(--space-4);
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
  margin-top: var(--space-5);
}

@media (max-width: 768px) {
  .toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .toolbar-left h2 {
    font-size: var(--text-lg);
  }

  .provider-toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .toolbar-right {
    width: 100%;
    justify-content: flex-start;
    flex-wrap: wrap;
  }
}
</style>
