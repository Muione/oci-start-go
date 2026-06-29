<template>
  <div class="nosql-page">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>NoSQL 数据库管理</h2>
        <el-tag type="info" size="small">{{ tables.length }} 个表</el-tag>
      </div>
      <div class="toolbar-right">
        <el-button type="primary" @click="openCreateTable">
          <el-icon><Plus /></el-icon> 创建表
        </el-button>
        <el-button @click="loadTables" :loading="loading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </div>
    </div>

    <!-- Tenant selector -->
    <div class="filter-bar">
      <el-select
        v-model="tenantId"
        placeholder="选择租户"
        filterable
        style="width: 260px"
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

    <!-- Table list -->
    <el-card shadow="none" class="table-card">
      <el-table :data="tables" v-loading="loading" border stripe style="width: 100%">
        <template #empty>
          <el-empty :description="!tenantId ? '请先选择租户' : '暂无 NoSQL 表'" :image-size="80">
            <el-button v-if="tenantId" type="primary" @click="openCreateTable">创建表</el-button>
          </el-empty>
        </template>
        <el-table-column type="index" label="#" width="50" align="center" />
        <el-table-column prop="name" label="表名" min-width="180">
          <template #default="{ row }">
            <span class="cell-link" @click="openRowViewer(row)">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="compartmentName" label="分区" min-width="140" />
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="stateTagType(row.lifecycleState)" size="small">
              {{ row.lifecycleState || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="已用存储" width="120" align="right">
          <template #default="{ row }">
            <span class="data-mono">{{ formatBytes(row.storageUsed) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">
            <span class="data-mono" style="font-size:12px">{{ formatTime(row.timeCreated) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right" align="center">
          <template #default="{ row }">
            <el-button size="small" @click="openRowViewer(row)">查看数据</el-button>
            <el-popconfirm title="确定删除此表？所有数据将被清除。" @confirm="deleteTable(row)">
              <template #reference>
                <el-button size="small" type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- ================================================================ -->
    <!-- Create Table Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="createVisible" title="创建 NoSQL 表" width="600px" destroy-on-close>
      <el-form :model="createForm" label-width="120px">
        <el-form-item label="表名" required>
          <el-input v-model="createForm.name" placeholder="my_table" />
        </el-form-item>
        <el-form-item label="DDL 语句">
          <el-input
            v-model="createForm.ddl"
            type="textarea"
            :rows="6"
            placeholder="CREATE TABLE IF NOT EXISTS my_table (id INTEGER, name STRING, PRIMARY KEY (id))"
          />
        </el-form-item>
        <el-form-item label="读容量">
          <el-input-number v-model="createForm.readUnits" :min="1" :max="100000" controls-position="right" style="width: 200px" />
          <span style="margin-left:8px;font-size:var(--text-sm);color:var(--text-secondary)">RU/s</span>
        </el-form-item>
        <el-form-item label="写容量">
          <el-input-number v-model="createForm.writeUnits" :min="1" :max="100000" controls-position="right" style="width: 200px" />
          <span style="margin-left:8px;font-size:var(--text-sm);color:var(--text-secondary)">WU/s</span>
        </el-form-item>
        <el-form-item label="存储容量(GB)">
          <el-input-number v-model="createForm.storageGB" :min="1" :max="1000" controls-position="right" style="width: 200px" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="createSaving" @click="doCreateTable">创建</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Row Viewer Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="rowViewerVisible" :title="`查看数据 - ${rowViewerTable}`" width="90%" destroy-on-close>
      <div class="row-viewer-toolbar">
        <el-input
          v-model="queryInput"
          placeholder='输入查询语句，例如: SELECT * FROM my_table WHERE id = 1'
          style="flex:1"
        >
          <template #prepend>SQL</template>
        </el-input>
        <el-button type="primary" :loading="queryLoading" @click="executeQuery">
          查询
        </el-button>
      </div>

      <!-- Query results -->
      <el-table
        :data="queryResults"
        v-loading="queryLoading"
        border
        stripe
        size="small"
        style="width: 100%; margin-top: 12px"
        max-height="400"
      >
        <template #empty>
          <el-empty description="无查询结果" :image-size="60" />
        </template>
        <el-table-column
          v-for="col in queryColumns"
          :key="col"
          :prop="col"
          :label="col"
          min-width="140"
          show-overflow-tooltip
        >
          <template #default="{ row }">
            <span class="data-mono" style="font-size:12px">{{ formatCellValue(row[col]) }}</span>
          </template>
        </el-table-column>
      </el-table>

      <!-- Row CRUD buttons -->
      <div class="row-actions" style="margin-top: 12px">
        <el-button size="small" @click="openAddRow">添加行</el-button>
        <el-button size="small" @click="openEditRow" :disabled="selectedRows.length !== 1">编辑行</el-button>
        <el-popconfirm title="确定删除选中的行？" @confirm="deleteRows">
          <template #reference>
            <el-button size="small" type="danger" :disabled="selectedRows.length === 0">删除选中行</el-button>
          </template>
        </el-popconfirm>
      </div>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Add/Edit Row Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="rowEditVisible" :title="rowEditIsNew ? '添加行' : '编辑行'" width="600px" destroy-on-close>
      <el-form label-width="120px">
        <el-form-item v-for="(val, key) in rowEditData" :key="key" :label="String(key)">
          <el-input v-model="rowEditData[key]" :placeholder="`输入 ${key} 的值`" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rowEditVisible = false">取消</el-button>
        <el-button type="primary" :loading="rowEditSaving" @click="saveRow">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus, Refresh } from '@element-plus/icons-vue'
import request from '../utils/request'

// ---- Types ----
interface NosqlTable {
  name: string
  compartmentName: string
  compartmentId: string
  lifecycleState: string
  storageUsed: number
  timeCreated: string
}

// ---- State ----
const tenantId = ref<number | null>(null)
const tenantOptions = ref<Array<{ id: number; name: string }>>([])
const tables = ref<NosqlTable[]>([])
const loading = ref(false)

// Create table
const createVisible = ref(false)
const createSaving = ref(false)
const createForm = ref({
  name: '',
  ddl: '',
  readUnits: 10,
  writeUnits: 10,
  storageGB: 1,
})

// Row viewer
const rowViewerVisible = ref(false)
const rowViewerTable = ref('')
const queryInput = ref('')
const queryLoading = ref(false)
const queryResults = ref<any[]>([])
const queryColumns = ref<string[]>([])
const selectedRows = ref<any[]>([])

// Row edit
const rowEditVisible = ref(false)
const rowEditIsNew = ref(true)
const rowEditSaving = ref(false)
const rowEditData = ref<Record<string, any>>({})

// ---- Helpers ----
function formatBytes(bytes: number | undefined): string {
  if (!bytes || isNaN(bytes)) return '0 B'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1073741824) return (bytes / 1048576).toFixed(1) + ' MB'
  return (bytes / 1073741824).toFixed(2) + ' GB'
}

function formatTime(t: string | undefined): string {
  if (!t) return '-'
  try { return new Date(t).toLocaleString('zh-CN') } catch { return t }
}

function stateTagType(state: string): '' | 'success' | 'warning' | 'danger' | 'info' {
  const s = (state || '').toLowerCase()
  if (s === 'active') return 'success'
  if (s === 'creating' || s === 'updating') return 'warning'
  if (s === 'deleted' || s === 'failed') return 'danger'
  return 'info'
}

function formatCellValue(val: any): string {
  if (val === null || val === undefined) return 'NULL'
  if (typeof val === 'object') return JSON.stringify(val)
  return String(val)
}

// ---- Tenant ----
async function loadTenants() {
  try {
    const tenants = await request.get('/tenants/listAll') as any[]
    tenantOptions.value = tenants.map((t: any) => ({
      id: t.id,
      name: t.userName || t.tenancyName || `#${t.id}`,
    }))
    if (tenantOptions.value.length > 0 && !tenantId.value) {
      tenantId.value = tenantOptions.value[0].id
      await onTenantChange()
    }
  } catch { /* ignore */ }
}

function onTenantChange() {
  tables.value = []
  if (tenantId.value) {
    loadTables()
  }
}

// ---- Table operations ----
async function loadTables() {
  if (!tenantId.value) return
  loading.value = true
  try {
    const res = await request.get('/oci/nosql/tables', { params: { tenantId: tenantId.value } }) as any
    tables.value = Array.isArray(res) ? res : (res?.items || [])
  } catch (e: any) {
    ElMessage.error('加载表失败: ' + (e?.message || e))
    tables.value = []
  } finally {
    loading.value = false
  }
}

function openCreateTable() {
  createForm.value = { name: '', ddl: '', readUnits: 10, writeUnits: 10, storageGB: 1 }
  createVisible.value = true
}

async function doCreateTable() {
  if (!createForm.value.name.trim()) {
    ElMessage.warning('请输入表名')
    return
  }
  if (!tenantId.value) {
    ElMessage.warning('请先选择租户')
    return
  }
  createSaving.value = true
  try {
    await request.post('/oci/nosql/table/create', {
      tenantId: tenantId.value,
      name: createForm.value.name.trim(),
      ddl: createForm.value.ddl,
      readUnits: createForm.value.readUnits,
      writeUnits: createForm.value.writeUnits,
      storageGB: createForm.value.storageGB,
    })
    ElMessage.success('表创建成功')
    createVisible.value = false
    await loadTables()
  } catch (e: any) {
    ElMessage.error('创建失败: ' + (e?.message || e))
  } finally {
    createSaving.value = false
  }
}

async function deleteTable(table: NosqlTable) {
  if (!tenantId.value) return
  try {
    await request.delete('/oci/nosql/table/delete', {
      data: { tenantId: tenantId.value, tableName: table.name },
    })
    ElMessage.success('表已删除')
    await loadTables()
  } catch (e: any) {
    ElMessage.error('删除失败: ' + (e?.message || e))
  }
}

// ---- Row viewer ----
function openRowViewer(table: NosqlTable) {
  rowViewerTable.value = table.name
  queryInput.value = `SELECT * FROM ${table.name} LIMIT 50`
  queryResults.value = []
  queryColumns.value = []
  selectedRows.value = []
  rowViewerVisible.value = true
  executeQuery()
}

async function executeQuery() {
  if (!queryInput.value.trim() || !tenantId.value) return
  queryLoading.value = true
  queryResults.value = []
  queryColumns.value = []
  try {
    const res = await request.post('/oci/nosql/query', {
      tenantId: tenantId.value,
      statement: queryInput.value.trim(),
    }) as any
    const items = Array.isArray(res) ? res : (res?.items || [])
    queryResults.value = items
    // Extract column names from first row
    if (items.length > 0) {
      queryColumns.value = Object.keys(items[0])
    }
  } catch (e: any) {
    ElMessage.error('查询失败: ' + (e?.message || e))
  } finally {
    queryLoading.value = false
  }
}

function openAddRow() {
  rowEditIsNew.value = true
  rowEditData.value = {}
  // Pre-fill with column names from query results
  for (const col of queryColumns.value) {
    rowEditData.value[col] = ''
  }
  rowEditVisible.value = true
}

function openEditRow() {
  if (selectedRows.value.length !== 1) return
  rowEditIsNew.value = false
  rowEditData.value = { ...selectedRows.value[0] }
  rowEditVisible.value = true
}

async function saveRow() {
  if (!tenantId.value) return
  rowEditSaving.value = true
  try {
    if (rowEditIsNew.value) {
      await request.post('/oci/nosql/row/update', {
        tenantId: tenantId.value,
        tableName: rowViewerTable.value,
        row: rowEditData.value,
      })
      ElMessage.success('行已添加')
    } else {
      await request.post('/oci/nosql/row/update', {
        tenantId: tenantId.value,
        tableName: rowViewerTable.value,
        row: rowEditData.value,
      })
      ElMessage.success('行已更新')
    }
    rowEditVisible.value = false
    await executeQuery()
  } catch (e: any) {
    ElMessage.error('保存失败: ' + (e?.message || e))
  } finally {
    rowEditSaving.value = false
  }
}

async function deleteRows() {
  if (selectedRows.value.length === 0 || !tenantId.value) return
  try {
    for (const row of selectedRows.value) {
      await request.delete('/oci/nosql/row/delete', {
        data: {
          tenantId: tenantId.value,
          tableName: rowViewerTable.value,
          key: row,
        },
      })
    }
    ElMessage.success(`已删除 ${selectedRows.value.length} 行`)
    selectedRows.value = []
    await executeQuery()
  } catch (e: any) {
    ElMessage.error('删除失败: ' + (e?.message || e))
  }
}

// ---- Init ----
onMounted(loadTenants)
</script>

<style scoped>
.nosql-page {
  padding: 0;
}

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

.filter-bar {
  display: flex;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
  flex-wrap: wrap;
  align-items: center;
}

.table-card {
  border-radius: var(--radius-md);
  overflow: hidden;
}

.table-card :deep(.el-card__body) {
  padding: 0;
}

.cell-link {
  cursor: pointer;
  color: var(--accent);
  font-weight: var(--font-medium);
}

.cell-link:hover {
  color: var(--accent-hover);
  text-decoration: underline;
}

.row-viewer-toolbar {
  display: flex;
  gap: var(--space-3);
  align-items: center;
}

.row-actions {
  display: flex;
  gap: var(--space-2);
}

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

  .row-viewer-toolbar {
    flex-direction: column;
  }
}
</style>
