<script setup lang="ts">
defineOptions({ name: 'BatchCreateModal' })

import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'
import type { BatchVnicResult } from '../../types/api'

const props = defineProps<{ modelValue: boolean; instanceId: string; subnetId?: string }>()
const emit = defineEmits<{ 'update:modelValue': [val: boolean]; saved: [] }>()

const saving = ref(false)
const vnicCount = ref(1)
const ipv6Count = ref(0)
const result = ref<BatchVnicResult | null>(null)

async function handleCreate() {
  saving.value = true
  result.value = null
  try {
    const res = await request.post('/oci/vnic/create', {
      instanceId: props.instanceId,
      subnetId: props.subnetId ?? '',
      vnicCount: vnicCount.value,
      ipv6CountPerVnic: ipv6Count.value,
    }) as BatchVnicResult
    result.value = res
    if (res.allSuccessful) {
      ElMessage.success(`成功创建 ${res.successfulVnicCount} 个 VNIC`)
    } else {
      ElMessage.warning(res.summary || '部分 VNIC 创建失败')
    }
    emit('saved')
  } catch (e: any) {
    ElMessage.error(e.message || '批量创建失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="批量创建 VNIC + IPv6" width="520px" destroy-on-close>
    <el-form label-width="120px">
      <el-form-item label="VNIC 数量">
        <el-input-number v-model="vnicCount" :min="1" :max="32" />
      </el-form-item>
      <el-form-item label="每 VNIC IPv6 数">
        <el-input-number v-model="ipv6Count" :min="0" :max="32" />
      </el-form-item>
    </el-form>

    <el-alert v-if="result" :title="result.summary" :type="result.allSuccessful ? 'success' : 'warning'" :closable="false" show-icon style="margin-top: 12px;" />

    <template #footer>
      <el-button @click="emit('update:modelValue', false)">关闭</el-button>
      <el-button type="primary" :loading="saving" @click="handleCreate">创建</el-button>
    </template>
  </el-dialog>
</template>
