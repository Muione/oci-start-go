<script setup lang="ts">
defineOptions({ name: 'InstanceTrafficPanel' })

import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'

const props = defineProps<{ instanceId: string; tenantId: number }>()

interface InstanceTraffic {
  instanceId: string
  instanceName: string
  publicIp: string
  vnicCount: number
  egressGB: number
  egressBytes: number
  ingressBytes: number
  statsDate: string
  region: string
}

const loading = ref(false)
const ingressMonth = ref(0)
const egressMonth = ref(0)

async function loadTraffic() {
  loading.value = true
  try {
    const res = await request.get('/instances/traffic', { params: { tenantId: props.tenantId } }) as InstanceTraffic[]
    // Aggregate per-instance traffic into totals
    let totalIngress = 0
    let totalEgress = 0
    if (Array.isArray(res)) {
      for (const inst of res) {
        totalIngress += inst.ingressBytes ?? 0
        totalEgress += inst.egressBytes ?? 0
      }
    }
    ingressMonth.value = +(totalIngress / (1024 ** 3)).toFixed(2)
    egressMonth.value = +(totalEgress / (1024 ** 3)).toFixed(2)
  } catch (e: any) {
    ElMessage.error(e.message || '加载流量数据失败')
  } finally {
    loading.value = false
  }
}

onMounted(loadTraffic)
</script>

<template>
  <div class="traffic-panel" v-loading="loading">
    <el-row :gutter="16">
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header>入站流量</template>
          <el-statistic :value="ingressMonth" :precision="2" suffix="GB" />
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header>出站流量</template>
          <el-statistic :value="egressMonth" :precision="2" suffix="GB" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.traffic-panel {
  padding: var(--space-4);
}
</style>
