<script setup lang="ts">
defineOptions({ name: 'ResizeVolumeModal' })

import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'

const props = defineProps<{ modelValue: boolean; instanceDbId: number; currentSizeGB: number }>()
const emit = defineEmits<{ 'update:modelValue': [val: boolean]; saved: [] }>()

const saving = ref(false)
const newSize = ref(props.currentSizeGB)

watch(() => props.modelValue, (val) => {
  if (val) newSize.value = props.currentSizeGB
})

async function handleSave() {
  if (newSize.value < props.currentSizeGB) {
    ElMessage.warning('磁盘只能增大不能缩小')
    return
  }
  saving.value = true
  try {
    await request.post(`/instances/${props.instanceDbId}/resize`, { sizeInGBs: newSize.value })
    ElMessage.success('磁盘大小调整成功')
    emit('saved')
    emit('update:modelValue', false)
  } catch (e: any) {
    ElMessage.error(e.message || '磁盘调整失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="调整启动卷大小" width="420px" destroy-on-close>
    <el-form label-width="100px">
      <el-form-item label="当前大小">
        <span>{{ currentSizeGB }} GB</span>
      </el-form-item>
      <el-form-item label="新大小">
        <el-input-number v-model="newSize" :min="currentSizeGB" :step="10" />
        <span style="margin-left: 8px;">GB</span>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="handleSave">确认</el-button>
    </template>
  </el-dialog>
</template>
