<script setup lang="ts">
defineOptions({ name: 'ConfigureIPv6Modal' })

import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'

const props = defineProps<{ modelValue: boolean; vcnId: string }>()
const emit = defineEmits<{ 'update:modelValue': [val: boolean]; saved: [] }>()

const saving = ref(false)

async function handleConfigure() {
  saving.value = true
  try {
    await request.post('/api/vcn/configure-ipv6', { vcnId: props.vcnId })
    ElMessage.success('IPv6 安全规则配置成功')
    emit('saved')
    emit('update:modelValue', false)
  } catch (e: any) {
    ElMessage.error(e.message || '配置失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="配置 IPv6 安全规则" width="480px" destroy-on-close>
    <el-alert title="将在 VCN 默认安全列表中添加以下规则：" type="info" :closable="false" show-icon>
      <ul style="margin: 8px 0 0; padding-left: 20px;">
        <li>ICMPv6 (类型 128, 129, 133-137) from ::/0</li>
        <li>TCP 22 (SSH) from ::/0</li>
        <li>出站 all to ::/0</li>
      </ul>
    </el-alert>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="handleConfigure">确认配置</el-button>
    </template>
  </el-dialog>
</template>
