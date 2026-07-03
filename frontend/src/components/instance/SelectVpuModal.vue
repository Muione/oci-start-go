<script setup lang="ts">
defineOptions({ name: 'SelectVpuModal' })

import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'

const props = defineProps<{ modelValue: boolean; instanceDbId: number; currentVpu: number }>()
const emit = defineEmits<{ 'update:modelValue': [val: boolean]; saved: [] }>()

const saving = ref(false)
const selectedVpu = ref(props.currentVpu)

const vpuOptions = [
  { value: 10, label: 'Balanced (10 VPUs/GB)', desc: '标准性能' },
  { value: 20, label: 'Higher Performance (20 VPUs/GB)', desc: '更高性能' },
  { value: 30, label: 'Ultra High (30 VPUs/GB)', desc: '超高性能' },
  { value: 60, label: 'Ultra High (60 VPUs/GB)', desc: '超高性能' },
  { value: 120, label: 'Ultra High (120 VPUs/GB)', desc: '最高性能' },
]

watch(() => props.modelValue, (val) => {
  if (val) selectedVpu.value = props.currentVpu
})

async function handleSave() {
  saving.value = true
  try {
    await request.post(`/instances/${props.instanceDbId}/vpu`, { vpusPerGB: selectedVpu.value })
    ElMessage.success('VPU 调整成功')
    emit('saved')
    emit('update:modelValue', false)
  } catch (e: any) {
    ElMessage.error(e.message || 'VPU 调整失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="调整启动卷性能级别" width="480px" destroy-on-close>
    <el-alert title="调整 VPU 需要实例处于停止状态" type="warning" :closable="false" show-icon style="margin-bottom: 16px" />
    <el-radio-group v-model="selectedVpu" style="display: flex; flex-direction: column; gap: 12px;">
      <el-radio v-for="opt in vpuOptions" :key="opt.value" :value="opt.value">
        {{ opt.label }}
        <span style="color: var(--text-secondary); margin-left: 8px;">{{ opt.desc }}</span>
      </el-radio>
    </el-radio-group>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="handleSave">确认</el-button>
    </template>
  </el-dialog>
</template>
