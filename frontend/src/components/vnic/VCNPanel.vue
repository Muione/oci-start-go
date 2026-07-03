<script setup lang="ts">
defineOptions({ name: 'VCNPanel' })

import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'
import type { VcnInfo } from '../../types/api'
import ConfigureIPv6Modal from './ConfigureIPv6Modal.vue'
import ReassignIPModal from './ReassignIPModal.vue'

const props = defineProps<{ instanceId: string }>()

const loading = ref(false)
const vcnList = ref<VcnInfo[]>([])
const selectedVcn = ref<VcnInfo | null>(null)
const currentPublicIp = ref('')

// Modals
const configureIpv6Visible = ref(false)
const reassignIpVisible = ref(false)

async function loadVcns() {
  loading.value = true
  try {
    const res = await request.get('/api/vcn/list') as VcnInfo[]
    vcnList.value = res ?? []
  } catch (e: any) {
    ElMessage.error(e.message || '加载 VCN 列表失败')
  } finally {
    loading.value = false
  }
}

async function loadPublicIp() {
  try {
    const res = await request.get('/api/instances/detail', { params: { instanceId: props.instanceId } }) as any
    currentPublicIp.value = res?.publicIp ?? ''
  } catch {}
}

function selectVcn(vcn: VcnInfo) {
  selectedVcn.value = vcn
}

onMounted(() => {
  loadVcns()
  loadPublicIp()
})
</script>

<template>
  <div class="vcn-panel" v-loading="loading">
    <!-- VCN List -->
    <el-card shadow="hover" style="margin-bottom: 16px;">
      <template #header>VCN 列表</template>
      <el-table :data="vcnList" stripe highlight-current-row @current-change="selectVcn">
        <el-table-column prop="displayName" label="名称" />
        <el-table-column prop="cidrBlock" label="CIDR" />
        <el-table-column prop="dnsLabel" label="DNS 标签" />
        <el-table-column prop="timeCreated" label="创建时间" width="180" />
      </el-table>
      <el-empty v-if="vcnList.length === 0" description="暂无 VCN" />
    </el-card>

    <!-- VCN Detail -->
    <el-card v-if="selectedVcn" shadow="hover" style="margin-bottom: 16px;">
      <template #header>VCN 详情: {{ selectedVcn.displayName }}</template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="OCID"><el-text truncated>{{ selectedVcn.id }}</el-text></el-descriptions-item>
        <el-descriptions-item label="CIDR">{{ selectedVcn.cidrBlock }}</el-descriptions-item>
        <el-descriptions-item label="DNS 标签">{{ selectedVcn.dnsLabel || '-' }}</el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ selectedVcn.timeCreated }}</el-descriptions-item>
      </el-descriptions>
      <el-space style="margin-top: 12px;">
        <el-button @click="configureIpv6Visible = true">配置 IPv6 安全规则</el-button>
      </el-space>
    </el-card>

    <!-- Public IP -->
    <el-card shadow="hover">
      <template #header>公网 IP 管理</template>
      <div style="display: flex; align-items: center; gap: 16px;">
        <span>当前公网 IP: <strong>{{ currentPublicIp || '无' }}</strong></span>
        <el-button type="warning" @click="reassignIpVisible = true">重新分配公网 IP</el-button>
      </div>
    </el-card>

    <!-- Modals -->
    <ConfigureIPv6Modal v-model="configureIpv6Visible" :vcn-id="selectedVcn?.id ?? ''" @saved="loadVcns" />
    <ReassignIPModal v-model="reassignIpVisible" :instance-id="instanceId" @saved="(ip) => currentPublicIp = ip" />
  </div>
</template>

<style scoped>
.vcn-panel {
  padding: var(--space-4);
}
</style>
