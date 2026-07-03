<script setup lang="ts">
defineOptions({ name: 'CreateRouteTableModal' })

import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'
import type { NatGatewayInfo } from '../../types/api'

const props = defineProps<{ modelValue: boolean; compartmentId: string; vcnId: string }>()
const emit = defineEmits<{ 'update:modelValue': [val: boolean]; saved: [] }>()

const saving = ref(false)
const displayName = ref('')
const natGatewayId = ref('')
const natGateways = ref<NatGatewayInfo[]>([])

async function loadNatGateways() {
  try {
    const res = await request.get('/api/nat/list', { params: { compartmentId: props.compartmentId, vcnId: props.vcnId } }) as NatGatewayInfo[]
    natGateways.value = res ?? []
  } catch {}
}

async function handleCreate() {
  if (!displayName.value.trim()) {
    ElMessage.warning('请输入名称')
    return
  }
  saving.value = true
  try {
    await request.post('/api/route-table/create', {
      compartmentId: props.compartmentId,
      vcnId: props.vcnId,
      displayName: displayName.value.trim(),
      natGatewayId: natGatewayId.value || undefined,
    })
    ElMessage.success('路由表创建成功')
    emit('saved')
    emit('update:modelValue', false)
  } catch (e: any) {
    ElMessage.error(e.message || '创建失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadNatGateways)
</script>

<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="创建路由表" width="480px" destroy-on-close>
    <el-form label-width="100px">
      <el-form-item label="名称">
        <el-input v-model="displayName" placeholder="路由表名称" />
      </el-form-item>
      <el-form-item label="NAT 网关">
        <el-select v-model="natGatewayId" placeholder="选择 NAT 网关（可选）" clearable style="width: 100%;">
          <el-option v-for="gw in natGateways" :key="gw.id" :label="gw.displayName" :value="gw.id" />
        </el-select>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="handleCreate">创建</el-button>
    </template>
  </el-dialog>
</template>
