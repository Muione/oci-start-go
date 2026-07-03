<script setup lang="ts">
defineOptions({ name: 'InstanceTrafficPanel' })

import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'
import type { TrafficData } from '../../types/api'

const props = defineProps<{ instanceId: string; tenantId: number }>()

const loading = ref(false)
const traffic = ref<TrafficData | null>(null)

async function loadTraffic() {
  loading.value = true
  try {
    const res = await request.get('/instances/traffic', { params: { tenantId: props.tenantId } }) as TrafficData
    traffic.value = res
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
          <template #header>今日入站流量</template>
          <el-statistic :value="traffic?.ingressToday ?? 0" :precision="2" suffix="GB" />
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header>今日出站流量</template>
          <el-statistic :value="traffic?.egressToday ?? 0" :precision="2" suffix="GB" />
        </el-card>
      </el-col>
    </el-row>
    <el-row :gutter="16" style="margin-top: 16px;">
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header>本月入站流量</template>
          <el-statistic :value="traffic?.ingressMonth ?? 0" :precision="2" suffix="GB" />
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header>本月出站流量</template>
          <el-statistic :value="traffic?.egressMonth ?? 0" :precision="2" suffix="GB" />
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
