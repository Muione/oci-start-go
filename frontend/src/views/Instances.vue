<template>
  <div>
    <div class="toolbar">
      <h2>实例管理</h2>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table :data="rows" v-loading="loading" border stripe style="width: 100%">
      <template #empty>
        <el-empty description="暂无实例数据" :image-size="80" />
      </template>
      <el-table-column prop="displayName" label="名称" min-width="160" />
      <el-table-column prop="tenantName" label="所属租户" min-width="140" />
      <el-table-column prop="instanceId" label="实例ID" min-width="220" show-overflow-tooltip />
      <el-table-column prop="shape" label="Shape" min-width="140" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.state === 'Running' ? 'success' : row.state === 'Stopped' ? 'danger' : 'info'">
            {{ row.state || '-' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="publicIps" label="公网IP" width="140" />
      <el-table-column prop="architecture" label="架构" width="80" />
      <el-table-column prop="ocpus" label="OCPU" width="70" />
      <el-table-column prop="memoryInGbs" label="内存(G)" width="80" />
      <el-table-column prop="availabilityDomain" label="AD" min-width="120" show-overflow-tooltip />
      <el-table-column label="在线" width="80">
        <template #default="{ row }">
          <el-tag :type="row.onLineEnable ? 'success' : 'info'" size="small">
            {{ row.onLineEnable ? '在线' : '离线' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="240" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="showDetail(row)">详情</el-button>
          <el-button size="small" type="warning" @click="openModify(row)">修改配置</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-if="total > 20"
      :total="total" :page-size="20" :current-page="page"
      layout="total, prev, pager, next"
      @current-change="onPage"
      style="margin-top: 16px; justify-content: center"
    />

    <!-- Detail Dialog -->
    <el-dialog v-model="detailVisible" title="实例详情" width="720px">
      <el-descriptions v-if="detail" :column="2" border>
        <el-descriptions-item label="显示名称">{{ detail.displayName }}</el-descriptions-item>
        <el-descriptions-item label="所属租户">{{ detail.tenantName || '-' }}</el-descriptions-item>
        <el-descriptions-item label="实例ID">{{ detail.instanceId }}</el-descriptions-item>
        <el-descriptions-item label="Shape">{{ detail.shape }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ detail.state }}</el-descriptions-item>
        <el-descriptions-item label="OCPU">{{ detail.ocpus }}</el-descriptions-item>
        <el-descriptions-item label="内存(GB)">{{ detail.memoryInGbs }}</el-descriptions-item>
        <el-descriptions-item label="启动卷(GB)">{{ detail.bootVolumeSizeInGbs }}</el-descriptions-item>
        <el-descriptions-item label="架构">{{ detail.architecture }}</el-descriptions-item>
        <el-descriptions-item label="公网IP">{{ detail.publicIps || '-' }}</el-descriptions-item>
        <el-descriptions-item label="私网IP">{{ detail.privateIps || '-' }}</el-descriptions-item>
        <el-descriptions-item label="可用域">{{ detail.availabilityDomain }}</el-descriptions-item>
        <el-descriptions-item label="启动卷ID" :span="2">{{ detail.bootVolumeId || '-' }}</el-descriptions-item>
        <el-descriptions-item label="IPv6">{{ detail.ipv6Addresses || '-' }}</el-descriptions-item>
        <el-descriptions-item label="VNIC IDs">{{ detail.vnicIds || '-' }}</el-descriptions-item>
        <el-descriptions-item label="VPU/GB">{{ detail.vpusPerGb || '-' }}</el-descriptions-item>
        <el-descriptions-item label="最后心跳">{{ detail.lastHeartbeat || '-' }}</el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ detail.createTime || '-' }}</el-descriptions-item>
        <el-descriptions-item label="备注" :span="2">
          <el-input v-model="remark" placeholder="添加备注" @keyup.enter="saveRemark(detail)" />
          <el-button size="small" style="margin-left:8px" @click="saveRemark(detail)">保存</el-button>
        </el-descriptions-item>
        <el-descriptions-item label="操作" :span="2">
          <el-button size="small" type="warning" @click="goRescue(detail)">救援模式</el-button>
          <el-button size="small" type="primary" @click="goConsole(detail)">VNC 控制台</el-button>
        </el-descriptions-item>
      </el-descriptions>
    </el-dialog>

    <!-- Modify Config Dialog -->
    <el-dialog v-model="modifyVisible" title="修改实例配置" width="520px" destroy-on-close>
      <el-alert
        title="提示：修改 Shape 可能需要先停止实例，请在 OCI 控制台确认实例状态"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom:16px"
      />
      <el-form :model="modifyForm" label-width="120px">
        <el-form-item label="当前 Shape">
          <el-input :model-value="modifyTarget?.shape" disabled />
        </el-form-item>
        <el-form-item label="新 Shape">
          <el-input v-model="modifyForm.shape" placeholder="例如: VM.Standard.E4.Flex" />
        </el-form-item>
        <el-form-item label="OCPU">
          <el-input-number v-model="modifyForm.ocpus" :min="1" :max="128" :step="1" controls-position="right" style="width:100%" />
        </el-form-item>
        <el-form-item label="内存(GB)">
          <el-input-number v-model="modifyForm.memoryInGbs" :min="1" :max="1024" :step="1" controls-position="right" style="width:100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="modifyVisible = false">取消</el-button>
        <el-button type="primary" :loading="modifySaving" @click="doModify">确认修改</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import request from '../utils/request'

interface Instance {
  id: number; tenantId: number; tenantName: string
  instanceId: string; displayName: string; shape: string; state: string
  ocpus: number; memoryInGbs: number; bootVolumeSizeInGbs: number
  publicIps: string; privateIps: string; availabilityDomain: string
  compartmentId: string; bootVolumeId: string; bootVolumeName: string
  vpusPerGb: string; ipv6Addresses: string; vnicIds: string
  architecture: string; onLineEnable: number; lastHeartbeat: string; createTime: string
}

const rows = ref<Instance[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const detailVisible = ref(false)
const detail = ref<Instance | null>(null)
const router = useRouter()
const remark = ref('')
// Modify config state
const modifyVisible = ref(false)
const modifySaving = ref(false)
const modifyTarget = ref<Instance | null>(null)
const modifyForm = ref({ shape: '', ocpus: 4, memoryInGbs: 24, displayName: '' })

function openModify(row: Instance) {
  modifyTarget.value = row
  modifyForm.value = {
    shape: row.shape || '',
    ocpus: row.ocpus || 4,
    memoryInGbs: row.memoryInGbs || 24,
    displayName: row.displayName || '',
  }
  modifyVisible.value = true
}

async function doModify() {
  if (!modifyTarget.value) return
  modifySaving.value = true
  try {
    const body: any = {}
    if (modifyForm.value.shape && modifyForm.value.shape !== modifyTarget.value.shape) {
      body.shape = modifyForm.value.shape
    }
    if (modifyForm.value.ocpus > 0 && modifyForm.value.ocpus !== modifyTarget.value.ocpus) {
      body.ocpus = modifyForm.value.ocpus
    }
    if (modifyForm.value.memoryInGbs > 0 && modifyForm.value.memoryInGbs !== modifyTarget.value.memoryInGbs) {
      body.memoryInGbs = modifyForm.value.memoryInGbs
    }
    if (modifyForm.value.displayName && modifyForm.value.displayName !== modifyTarget.value.displayName) {
      body.displayName = modifyForm.value.displayName
    }
    if (!body.shape && !body.displayName && body.ocpus === undefined && body.memoryInGbs === undefined) {
      ElMessage.warning('没有需要修改的配置')
      modifySaving.value = false
      return
    }
    await request.post(`/instances/${modifyTarget.value.id}/modify`, body)
    ElMessage.success('修改请求已提交，实例正在更新中')
    modifyVisible.value = false
    await load()
  } catch (e: any) {
    ElMessage.error(e.message || '修改失败')
  } finally {
    modifySaving.value = false
  }
}

async function load() {
  loading.value = true
  try {
    const offset = (page.value - 1) * 20
    const res = await request.get('/instances/list', { params: { limit: 20, offset } }) as any
    rows.value = res.items
    total.value = res.total
  } catch (e: any) { ElMessage.error(e.message)
  } finally { loading.value = false }
}

function onPage(p: number) { page.value = p; load() }

function showDetail(row: Instance) {
  detail.value = row
  remark.value = ''
  detailVisible.value = true
}

function goRescue(row: Instance) {
  router.push({ path: '/rescue', query: { instanceId: row.instanceId } })
}
function goConsole(row: Instance) {
  router.push({ path: '/console', query: { instanceId: row.instanceId } })
}

async function saveRemark(row: Instance) {
  try {
    await request.post(`/instances/${row.id}/remark`, { remark: remark.value })
    ElMessage.success('备注已更新')
    remark.value = ''
  } catch (e: any) { ElMessage.error(e.message) }
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

:deep(.el-pagination) {
  text-align: center;
  margin-top: 24px;
}

:deep(.el-pagination__item.active) {
  background: linear-gradient(135deg, #0066ff, #00bcd4);
  color: #fff;
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

:deep(.el-descriptions) {
  background: transparent;
}

:deep(.el-descriptions__body) {
  background: transparent;
}

:deep(.el-button--small) {
  border-radius: 6px;
  transition: all 0.3s ease;
}

:deep(.el-button--primary:hover) {
  box-shadow: 0 8px 16px rgba(0, 102, 255, 0.3);
  transform: translateY(-1px);
}

:deep(.el-tag) {
  border-radius: 8px;
  border: none;
  font-weight: 600;
}
</style>
