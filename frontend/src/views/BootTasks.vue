<template>
  <div>
    <div class="toolbar">
      <h2>抢机任务</h2>
      <el-button type="primary" @click="openAdd">新建任务</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <!-- System Status Card -->
    <el-card v-if="sysStatus" class="status-card" shadow="hover">
      <template #header><span>引擎状态</span></template>
      <el-row :gutter="20">
        <el-col :span="4">
          <el-statistic title="总任务" :value="sysStatus.totalTasks" />
        </el-col>
        <el-col :span="4">
          <el-statistic title="运行中" :value="sysStatus.runningTasks" />
        </el-col>
        <el-col :span="4">
          <el-statistic title="活跃Key" :value="sysStatus.activeKeyCount" />
        </el-col>
        <el-col :span="4">
          <el-statistic title="批次大小" :value="sysStatus.batchSize" />
        </el-col>
        <el-col :span="4">
          <el-tag :type="sysStatus.running ? 'success' : 'danger'">
            {{ sysStatus.running ? '运行中' : '已停止' }}
          </el-tag>
        </el-col>
        <el-col :span="4">
          <el-statistic title="父池活跃" :value="sysStatus.parentPool?.active ?? 0" />
        </el-col>
      </el-row>
      <el-row :gutter="20" style="margin-top: 12px">
        <el-col :span="4">
          <el-statistic title="父池队列" :value="sysStatus.parentPool?.queue ?? 0" />
        </el-col>
        <el-col :span="4">
          <el-statistic title="API池活跃" :value="sysStatus.apiPool?.active ?? 0" />
        </el-col>
        <el-col :span="4">
          <el-statistic title="API池完成" :value="sysStatus.apiPool?.completed ?? 0" />
        </el-col>
      </el-row>
    </el-card>

    <!-- Task Table -->
    <el-table :data="rows" v-loading="loading" border stripe style="width: 100%; margin-top: 16px">
      <template #empty>
        <el-empty description="暂无抢机任务，请新建" :image-size="80" />
      </template>
      <el-table-column prop="bootId" label="Boot ID" min-width="200" show-overflow-tooltip />
      <el-table-column prop="tenantId" label="租户ID" width="80" />
      <el-table-column prop="architecture" label="架构" width="70">
        <template #default="{ row }">
          <el-tag size="small" :type="row.architecture === 'ARM' ? '' : 'warning'">
            {{ row.architecture || '-' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="规格" width="160">
        <template #default="{ row }">
          {{ row.ocpu }}C / {{ row.memory }}G / {{ row.disk }}GB
        </template>
      </el-table-column>
      <el-table-column prop="loopTime" label="间隔(s)" width="80" />
      <el-table-column prop="instanceCount" label="目标数" width="80" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="statusTag(row.status)">{{ statusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="publicIp" label="公网IP" width="140" />
      <el-table-column prop="failCount" label="失败次数" width="90" />
      <el-table-column prop="successCount" label="成功次数" width="90" />
      <el-table-column prop="nextExecutionTime" label="下次执行" width="160" />
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="edit(row)">编辑</el-button>
          <el-button size="small" :type="row.status === 1 ? 'warning' : 'success'"
            @click="toggle(row)">
            {{ row.status === 1 ? '暂停' : '启用' }}
          </el-button>
          <el-button size="small" type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Add/Edit Dialog -->
    <el-dialog v-model="addVisible" :title="editing ? '编辑任务' : '新建任务'" width="680px">
      <el-form :model="form" label-width="130px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="租户">
              <el-select v-model="form.tenantId" filterable placeholder="选择租户" style="width:100%">
                <el-option v-for="t in tenantList" :key="t.id" :label="`${t.name} (${t.region})`"
                  :value="t.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="架构">
              <el-select v-model="form.architecture" placeholder="选择架构" style="width:100%">
                <el-option label="ARM (Ampere)" value="ARM" />
                <el-option label="AMD (Intel/AMD)" value="AMD" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="OCPU">
              <el-input-number v-model="form.ocpu" :min="1" :max="128" style="width:100%" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="内存 (GB)">
              <el-input-number v-model="form.memory" :min="1" :max="1024" style="width:100%" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="磁盘 (GB)">
              <el-input-number v-model="form.disk" :min="50" :max="32768" style="width:100%" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="循环间隔(秒)">
              <el-input-number v-model="form.loopTime" :min="3" :max="3600" style="width:100%" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="目标实例数">
              <el-input-number v-model="form.instanceCount" :min="1" :max="100" style="width:100%" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="云类型">
              <el-select v-model="form.cloudType" style="width:100%">
                <el-option label="Oracle Cloud" :value="1" />
                <el-option label="GCP" :value="2" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="镜像ID">
              <el-input v-model="form.imageId" placeholder="留空自动选择" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="操作系统">
              <el-input v-model="form.operatingSystem" placeholder="如: Canonical Ubuntu" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="系统版本">
              <el-input v-model="form.operatingSystemVersion" placeholder="如: 22.04" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Root密码">
              <el-input v-model="form.rootPassword" placeholder="留空自动生成" show-password />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="时间窗口">
              <el-input v-model="form.dataGap" placeholder="如: 00:00-23:59 或留空" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="备注">
              <el-input v-model="form.remark" placeholder="可选" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="addVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../utils/request'

interface BootTask {
  id: number; bootId: string; tenantId: number; ocpu: number; memory: number; disk: number
  loopTime: number; instanceCount: number; status: number; architecture: string
  rootPassword: string; publicIp: string; imageId: string
  operatingSystem: string; operatingSystemVersion: string
  dataGap: string; notifyFlag: string; nextExecutionTime: string
  failCount: number; successCount: number; remark: string; cloudType: number
}
interface SysStatus { totalTasks: number; runningTasks: number; activeKeyCount: number
  batchSize: number; running: boolean; parentPool: any; apiPool: any }
interface Tenant { id: number; name: string; region: string; tenancy: string }

const rows = ref<BootTask[]>([])
const loading = ref(false)
const addVisible = ref(false)
const saving = ref(false)
const editing = ref(false)
const sysStatus = ref<SysStatus | null>(null)
const tenantList = ref<Tenant[]>([])

const emptyForm = {
  bootId: '', tenantId: 0, ocpu: 4, memory: 24, disk: 100,
  loopTime: 6, instanceCount: 1, architecture: 'ARM', rootPassword: '',
  imageId: '', operatingSystem: 'Canonical Ubuntu', operatingSystemVersion: '22.04',
  dataGap: '', notifyFlag: 'NO', remark: '', cloudType: 1,
}
const form = ref({ ...emptyForm })

function statusTag(s: number) { return s === 1 ? 'warning' : s === 2 ? 'success' : s === 0 ? 'info' : '' }
function statusText(s: number) { return s === 1 ? '运行中' : s === 2 ? '已完成' : s === 0 ? '已停用' : '未知' }

async function load() {
  loading.value = true
  try {
    rows.value = await request.get('/boot/list') as BootTask[]
    sysStatus.value = await request.get('/boot/systemStatus') as SysStatus
  } catch (e: any) { ElMessage.error(e.message)
  } finally { loading.value = false }
}

async function loadTenants() {
  try { tenantList.value = await request.get('/boot/tenants') as Tenant[]
  } catch { /* ignore */ }
}

function openAdd() {
  editing.value = false
  form.value = { ...emptyForm }
  addVisible.value = true
}

function edit(row: BootTask) {
  editing.value = true
  form.value = {
    bootId: row.bootId, tenantId: row.tenantId,
    ocpu: row.ocpu, memory: row.memory, disk: row.disk,
    loopTime: row.loopTime, instanceCount: row.instanceCount,
    architecture: row.architecture, rootPassword: '',
    imageId: row.imageId, operatingSystem: row.operatingSystem,
    operatingSystemVersion: row.operatingSystemVersion,
    dataGap: row.dataGap, notifyFlag: row.notifyFlag,
    remark: row.remark, cloudType: row.cloudType,
  }
  addVisible.value = true
}

async function save() {
  if (!form.value.tenantId) { ElMessage.warning('请选择租户'); return }
  if (!form.value.architecture) { ElMessage.warning('请选择架构'); return }
  saving.value = true
  try {
    await request.post('/boot/save', form.value)
    ElMessage.success(editing.value ? '更新成功' : '创建成功')
    addVisible.value = false
    await load()
  } catch (e: any) { ElMessage.error(e.message)
  } finally { saving.value = false }
}

async function remove(row: BootTask) {
  try {
    await ElMessageBox.confirm(`删除任务 ${row.bootId}?`, '确认', { type: 'warning' })
    await request.get('/boot/delete', { params: { bootId: row.bootId } })
    ElMessage.success('已删除')
    await load()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

async function toggle(row: BootTask) {
  const enable = row.status !== 1
  try {
    await request.get('/boot/toggle', { params: { bootId: row.bootId, enable: enable ? 1 : 0 } })
    ElMessage.success(enable ? '已启用' : '已暂停')
    await load()
  } catch (e: any) { ElMessage.error(e.message) }
}

onMounted(() => { load(); loadTenants() })
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

.status-card {
  margin-bottom: 20px;
  border-radius: 12px;
  border: 1px solid rgba(0, 102, 255, 0.1);
  background: linear-gradient(135deg, #ffffff, #f8fafc);
  transition: all 0.3s ease;
}

.status-card:hover {
  box-shadow: 0 8px 16px rgba(0, 102, 255, 0.15);
  transform: translateY(-2px);
  border-color: rgba(0, 102, 255, 0.2);
}

.status-card :deep(.el-card__header) {
  padding: 16px 20px;
  font-weight: 700;
  background: linear-gradient(90deg, rgba(0, 102, 255, 0.03), rgba(0, 188, 212, 0.03));
  border-bottom: 1px solid rgba(0, 102, 255, 0.1);
  color: #1e293b;
}

.status-card :deep(.el-card__body) {
  padding: 20px;
  background: #ffffff;
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

:deep(.el-button--small) {
  border-radius: 6px;
  transition: all 0.3s ease;
}

:deep(.el-button--primary:hover) {
  box-shadow: 0 8px 16px rgba(0, 102, 255, 0.3);
  transform: translateY(-1px);
}

:deep(.el-button--success:hover) {
  box-shadow: 0 8px 16px rgba(16, 185, 129, 0.3);
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

:deep(.el-pagination) {
  text-align: center;
  margin-top: 24px;
}

:deep(.el-pagination__item.active) {
  background: linear-gradient(135deg, #0066ff, #00bcd4);
  color: #fff;
}

@media (max-width: 768px) {
  .toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .toolbar h2 {
    font-size: 20px;
  }
}
</style>
