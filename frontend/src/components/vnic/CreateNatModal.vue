<script setup lang="ts">
defineOptions({ name: 'CreateNatModal' })

import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'

const props = defineProps<{ modelValue: boolean; compartmentId: string; vcnId: string }>()
const emit = defineEmits<{ 'update:modelValue': [val: boolean]; saved: [] }>()

const saving = ref(false)
const displayName = ref('')

async function handleCreate() {
  if (!displayName.value.trim()) {
    ElMessage.warning('请输入名称')
    return
  }
  saving.value = true
  try {
    await request.post('/api/nat/create', { compartmentId: props.compartmentId, vcnId: props.vcnId, displayName: displayName.value.trim() })
    ElMessage.success('NAT 网关创建成功')
    emit('saved')
    emit('update:modelValue', false)
  } catch (e: any) {
    ElMessage.error(e.message || '创建失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="创建 NAT 网关" width="420px" destroy-on-close>
    <el-form label-width="80px">
      <el-form-item label="名称">
        <el-input v-model="displayName" placeholder="NAT 网关名称" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="handleCreate">创建</el-button>
    </template>
  </el-dialog>
</template>
