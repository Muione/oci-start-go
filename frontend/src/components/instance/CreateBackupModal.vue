<script setup lang="ts">
defineOptions({ name: 'CreateBackupModal' })

import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'

const props = defineProps<{ modelValue: boolean; bootVolumeId: string }>()
const emit = defineEmits<{ 'update:modelValue': [val: boolean]; saved: [] }>()

const saving = ref(false)
const backupName = ref('')

async function handleCreate() {
  if (!backupName.value.trim()) {
    ElMessage.warning('请输入备份名称')
    return
  }
  saving.value = true
  try {
    await request.post('/api/backup/create', { bootVolumeId: props.bootVolumeId, displayName: backupName.value.trim() })
    ElMessage.success('备份创建成功')
    emit('saved')
    emit('update:modelValue', false)
  } catch (e: any) {
    ElMessage.error(e.message || '备份创建失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="创建启动卷备份" width="420px" destroy-on-close>
    <el-form label-width="100px">
      <el-form-item label="备份名称">
        <el-input v-model="backupName" placeholder="输入备份名称" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="handleCreate">创建</el-button>
    </template>
  </el-dialog>
</template>
