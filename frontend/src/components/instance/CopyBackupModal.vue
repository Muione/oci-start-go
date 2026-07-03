<script setup lang="ts">
defineOptions({ name: 'CopyBackupModal' })

import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'
import type { RegionSubInfo } from '../../types/api'

const props = defineProps<{ modelValue: boolean; backupId: string }>()
const emit = defineEmits<{ 'update:modelValue': [val: boolean]; saved: [] }>()

const saving = ref(false)
const targetRegion = ref('')
const backupName = ref('')
const regions = ref<RegionSubInfo[]>([])

async function loadRegions() {
  try {
    const res = await request.get('/api/regions/subscribed') as RegionSubInfo[]
    regions.value = res ?? []
  } catch (e: any) {
    ElMessage.error(e.message || '加载区域列表失败')
  }
}

async function handleCopy() {
  if (!targetRegion.value) {
    ElMessage.warning('请选择目标区域')
    return
  }
  saving.value = true
  try {
    await request.post('/api/backup/copy', { backupId: props.backupId, targetRegion: targetRegion.value, displayName: backupName.value || undefined })
    ElMessage.success('备份复制任务已提交')
    emit('saved')
    emit('update:modelValue', false)
  } catch (e: any) {
    ElMessage.error(e.message || '备份复制失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadRegions)
</script>

<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="跨区域复制备份" width="480px" destroy-on-close>
    <el-form label-width="100px">
      <el-form-item label="目标区域">
        <el-select v-model="targetRegion" placeholder="选择目标区域" style="width: 100%;">
          <el-option v-for="r in regions" :key="r.regionKey" :label="`${r.regionName} (${r.regionKey})`" :value="r.regionKey" />
        </el-select>
      </el-form-item>
      <el-form-item label="备份名称">
        <el-input v-model="backupName" placeholder="可选，留空使用默认名称" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="handleCopy">复制</el-button>
    </template>
  </el-dialog>
</template>
