<script setup lang="ts">
defineOptions({ name: 'ReassignIPModal' })

import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'

const props = defineProps<{ modelValue: boolean; instanceId: string }>()
const emit = defineEmits<{ 'update:modelValue': [val: boolean]; saved: [newIp: string] }>()

const saving = ref(false)

async function handleReassign() {
  saving.value = true
  try {
    const res = await request.post('/api/vcn/reassign-ip', { instanceId: props.instanceId }) as { publicIp: string }
    ElMessage.success(`新公网 IP: ${res.publicIp}`)
    emit('saved', res.publicIp)
    emit('update:modelValue', false)
  } catch (e: any) {
    ElMessage.error(e.message || '重新分配失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="重新分配公网 IP" width="420px" destroy-on-close>
    <el-alert title="旧公网 IP 将立即失效，请确保没有依赖该 IP 的服务。" type="warning" :closable="false" show-icon />
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="warning" :loading="saving" @click="handleReassign">确认重新分配</el-button>
    </template>
  </el-dialog>
</template>
