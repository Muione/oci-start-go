<script setup lang="ts">
defineOptions({ name: 'AddRuleModal' })

import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'

const props = defineProps<{ modelValue: boolean; tenantId: number }>()
const emit = defineEmits<{ 'update:modelValue': [val: boolean]; saved: [] }>()

const saving = ref(false)
const form = ref({
  type: 'ingress' as 'ingress' | 'egress',
  protocol: 'all',
  source: '0.0.0.0/0',
  ports: '',
  icmpType: '8, 0',
})

const showPorts = computed(() => form.value.protocol === 'tcp' || form.value.protocol === 'udp')
const showIcmp = computed(() => form.value.protocol === 'icmp')

async function handleAdd() {
  saving.value = true
  try {
    await request.post('/tenants/security-rules', {
      tenantId: props.tenantId,
      type: form.value.type,
      protocol: form.value.protocol,
      source: form.value.source,
      ports: form.value.ports || undefined,
      icmpType: showIcmp.value ? form.value.icmpType : undefined,
    })
    ElMessage.success('规则添加成功')
    emit('saved')
    emit('update:modelValue', false)
  } catch (e: any) {
    ElMessage.error(e.message || '规则添加失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="添加安全规则" width="520px" destroy-on-close>
    <el-form :model="form" label-width="100px">
      <el-form-item label="类型">
        <el-radio-group v-model="form.type">
          <el-radio value="ingress">入站</el-radio>
          <el-radio value="egress">出站</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="协议">
        <el-select v-model="form.protocol" style="width: 100%;">
          <el-option label="全部 (all)" value="all" />
          <el-option label="TCP" value="tcp" />
          <el-option label="UDP" value="udp" />
          <el-option label="ICMP" value="icmp" />
        </el-select>
      </el-form-item>
      <el-form-item :label="form.type === 'ingress' ? '源 CIDR' : '目标 CIDR'">
        <el-input v-model="form.source" placeholder="0.0.0.0/0" />
      </el-form-item>
      <el-form-item v-if="showPorts" label="端口">
        <el-input v-model="form.ports" placeholder="80 或 8080-9090" />
      </el-form-item>
      <el-form-item v-if="showIcmp" label="ICMP Type">
        <el-input v-model="form.icmpType" placeholder="8, 0" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="handleAdd">添加</el-button>
    </template>
  </el-dialog>
</template>
