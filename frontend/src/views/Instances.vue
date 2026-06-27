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
      <el-table-column label="操作" width="140" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="showDetail(row)">详情</el-button>
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
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import request from '../utils/request'

interface Instance {
  id: number; instanceId: string; displayName: string; shape: string; state: string
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
.toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
.toolbar h2 { margin: 0; margin-right: auto; }
</style>
