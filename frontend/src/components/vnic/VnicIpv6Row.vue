<script setup lang="ts">
defineOptions({ name: 'VnicIpv6Row' })

import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../../utils/request'
import type { Ipv6Info } from '../../types/api'

const props = defineProps<{ vnicId: string; ipv6Addresses: string[] }>()
const emit = defineEmits<{ refresh: [] }>()

const assigning = ref(false)

async function assignIpv6() {
  assigning.value = true
  try {
    await request.post('/api/vnic/ipv6/assign', { vnicId: props.vnicId })
    ElMessage.success('IPv6 地址分配成功')
    emit('refresh')
  } catch (e: any) {
    ElMessage.error(e.message || 'IPv6 分配失败')
  } finally {
    assigning.value = false
  }
}

async function deleteAllIpv6() {
  try {
    await ElMessageBox.confirm('确定删除该 VNIC 的所有 IPv6 地址？', '删除 IPv6', { type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger' })
    await request.post('/api/vnic/ipv6/delete', { vnicId: props.vnicId })
    ElMessage.success('IPv6 地址已删除')
    emit('refresh')
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '删除失败')
  }
}
</script>

<template>
  <div class="ipv6-row">
    <div v-if="ipv6Addresses.length > 0" style="margin-bottom: 8px;">
      <div v-for="addr in ipv6Addresses" :key="addr" style="display: flex; align-items: center; gap: 8px; margin-bottom: 4px;">
        <el-tag type="info" size="small">{{ addr }}</el-tag>
      </div>
    </div>
    <el-empty v-else description="无 IPv6 地址" :image-size="40" />
    <el-space style="margin-top: 8px;">
      <el-button size="small" type="primary" :loading="assigning" @click="assignIpv6">分配 IPv6</el-button>
      <el-button v-if="ipv6Addresses.length > 0" size="small" type="danger" @click="deleteAllIpv6">删除全部 IPv6</el-button>
    </el-space>
  </div>
</template>

<style scoped>
.ipv6-row {
  padding: 12px 16px;
  background: var(--bg-color-page);
  border-radius: var(--radius-md);
}
</style>
