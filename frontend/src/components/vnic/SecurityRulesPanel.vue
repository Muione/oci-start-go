<script setup lang="ts">
defineOptions({ name: 'SecurityRulesPanel' })

import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../../utils/request'
import type { SecurityRule } from '../../types/api'
import AddRuleModal from './AddRuleModal.vue'

const props = defineProps<{ compartmentId: string }>()

const loading = ref(false)
const ingressRules = ref<SecurityRule[]>([])
const egressRules = ref<SecurityRule[]>([])
const addRuleVisible = ref(false)

async function loadRules() {
  loading.value = true
  try {
    const [ingress, egress] = await Promise.all([
      request.get('/api/security/rules', { params: { compartmentId: props.compartmentId, type: 'ingress' } }) as Promise<SecurityRule[]>,
      request.get('/api/security/rules', { params: { compartmentId: props.compartmentId, type: 'egress' } }) as Promise<SecurityRule[]>,
    ])
    ingressRules.value = ingress ?? []
    egressRules.value = egress ?? []
  } catch (e: any) {
    ElMessage.error(e.message || '加载安全规则失败')
  } finally {
    loading.value = false
  }
}

async function deleteRule(rule: SecurityRule) {
  try {
    await ElMessageBox.confirm('确定删除此安全规则？', '删除规则', { type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger' })
    await request.get('/api/security/rules/delete', { params: { compartmentId: props.compartmentId, id: rule.id } })
    ElMessage.success('规则已删除')
    loadRules()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '删除失败')
  }
}

async function enableAll() {
  try {
    await ElMessageBox.confirm('将为当前 Compartment 启用全部协议规则（IPv4 + IPv6 + ICMP），是否继续？', '一键启用', { type: 'warning', confirmButtonText: '确认启用' })
    await request.post('/api/security/rules/enable-all', { compartmentId: props.compartmentId })
    ElMessage.success('全部规则已启用')
    loadRules()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '操作失败')
  }
}

async function enableIpv6() {
  try {
    await ElMessageBox.confirm('将启用 IPv6 入站/出站规则，是否继续？', '启用 IPv6 规则', { type: 'info', confirmButtonText: '确认' })
    await request.post('/api/security/rules/enable-ipv6', { compartmentId: props.compartmentId })
    ElMessage.success('IPv6 规则已启用')
    loadRules()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '操作失败')
  }
}

onMounted(loadRules)
</script>

<template>
  <div class="security-panel" v-loading="loading">
    <div style="margin-bottom: 16px;">
      <el-space>
        <el-button type="danger" @click="enableAll">一键启用全部规则</el-button>
        <el-button @click="enableIpv6">启用 IPv6 规则</el-button>
        <el-button type="primary" @click="addRuleVisible = true">添加规则</el-button>
      </el-space>
    </div>

    <el-card shadow="hover" style="margin-bottom: 16px;">
      <template #header>入站规则 ({{ ingressRules.length }})</template>
      <el-table :data="ingressRules" stripe size="small">
        <el-table-column prop="protocol" label="协议" width="80" />
        <el-table-column prop="source" label="源 CIDR" />
        <el-table-column prop="ports" label="端口" width="120">
          <template #default="{ row }">{{ row.ports || '全部' }}</template>
        </el-table-column>
        <el-table-column prop="icmpType" label="ICMP Type" width="100">
          <template #default="{ row }">{{ row.icmpType || '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="danger" @click="deleteRule(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card shadow="hover">
      <template #header>出站规则 ({{ egressRules.length }})</template>
      <el-table :data="egressRules" stripe size="small">
        <el-table-column prop="protocol" label="协议" width="80" />
        <el-table-column prop="source" label="目标 CIDR" />
        <el-table-column prop="ports" label="端口" width="120">
          <template #default="{ row }">{{ row.ports || '全部' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="danger" @click="deleteRule(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <AddRuleModal v-model="addRuleVisible" :compartment-id="compartmentId" @saved="loadRules" />
  </div>
</template>

<style scoped>
.security-panel {
  padding: var(--space-4);
}
</style>
