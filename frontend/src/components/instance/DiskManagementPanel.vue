<script setup lang="ts">
defineOptions({ name: 'DiskManagementPanel' })

import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../../utils/request'
import type { BootVolumeDetail, BootVolumeBackup } from '../../types/api'
import SelectVpuModal from './SelectVpuModal.vue'
import ResizeVolumeModal from './ResizeVolumeModal.vue'
import CreateBackupModal from './CreateBackupModal.vue'
import CopyBackupModal from './CopyBackupModal.vue'

const props = defineProps<{ dbId: number; instanceId: string; bootVolumeId?: string; tenantId?: number }>()

const loading = ref(false)
const bootVolume = ref<BootVolumeDetail | null>(null)
const backups = ref<BootVolumeBackup[]>([])

// Modal visibility
const vpuModalVisible = ref(false)
const resizeModalVisible = ref(false)
const createBackupVisible = ref(false)
const copyBackupVisible = ref(false)
const copyBackupId = ref('')

async function loadBootVolume() {
  if (!props.dbId) return
  loading.value = true
  try {
    const res = await request.get(`/instances/${props.dbId}`) as any
    // Map instance detail response to boot volume display format
    bootVolume.value = {
      id: res.bootVolumeId ?? '',
      displayName: res.bootVolumeName ?? res.displayName ?? '-',
      sizeInGBs: res.bootVolumeSizeInGbs ?? 0,
      vpusPerGB: Number(res.vpusPerGb) || 10,
    } as BootVolumeDetail
  } catch (e: any) {
    ElMessage.error(e.message || '加载启动卷信息失败')
  } finally {
    loading.value = false
  }
}

async function loadBackups() {
  if (!props.tenantId) return
  try {
    const res = await request.get('/backup/list', { params: { tenantId: props.tenantId } }) as BootVolumeBackup[]
    backups.value = res ?? []
  } catch (e: any) {
    ElMessage.error(e.message || '加载备份列表失败')
  }
}

async function deleteBackup(backup: BootVolumeBackup) {
  try {
    await ElMessageBox.confirm(`确定删除备份 "${backup.displayName}"？`, '删除备份', { type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger' })
    await request.delete('/backup/delete', { params: { id: backup.id } })
    ElMessage.success('备份已删除')
    loadBackups()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '删除失败')
  }
}

function openCopyBackup(backupId: string) {
  copyBackupId.value = backupId
  copyBackupVisible.value = true
}

function onSaved() {
  loadBootVolume()
  loadBackups()
}

onMounted(() => {
  loadBootVolume()
  loadBackups()
})
</script>

<template>
  <div class="disk-panel" v-loading="loading">
    <!-- Boot Volume Info -->
    <el-card shadow="hover" style="margin-bottom: 16px;">
      <template #header>启动卷信息</template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="名称">{{ bootVolume?.displayName ?? '-' }}</el-descriptions-item>
        <el-descriptions-item label="OCID"><el-text truncated>{{ bootVolume?.id ?? '-' }}</el-text></el-descriptions-item>
        <el-descriptions-item label="大小">{{ bootVolume?.sizeInGBs ?? '-' }} GB</el-descriptions-item>
        <el-descriptions-item label="VPU/GB">{{ bootVolume?.vpusPerGB ?? '-' }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- Boot Volume Actions -->
    <el-card shadow="hover" style="margin-bottom: 16px;">
      <template #header>启动卷操作</template>
      <el-space>
        <el-button @click="vpuModalVisible = true">调整 VPU</el-button>
        <el-button @click="resizeModalVisible = true">调整大小</el-button>
        <el-button type="primary" @click="createBackupVisible = true">创建备份</el-button>
      </el-space>
    </el-card>

    <!-- Backup List -->
    <el-card shadow="hover">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <span>备份列表</span>
          <el-button size="small" @click="loadBackups">刷新</el-button>
        </div>
      </template>
      <el-table :data="backups" stripe>
        <el-table-column prop="displayName" label="名称" />
        <el-table-column prop="sizeInGBs" label="大小 (GB)" width="120" />
        <el-table-column prop="lifecycleState" label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.lifecycleState === 'AVAILABLE' ? 'success' : 'info'" size="small">{{ row.lifecycleState }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="timeCreated" label="创建时间" width="180" />
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openCopyBackup(row.id)">复制</el-button>
            <el-button size="small" type="danger" @click="deleteBackup(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="backups.length === 0" description="暂无备份" />
    </el-card>

    <!-- Modals -->
    <SelectVpuModal v-model="vpuModalVisible" :instance-db-id="dbId" :current-vpu="bootVolume?.vpusPerGB ?? 10" @saved="onSaved" />
    <ResizeVolumeModal v-model="resizeModalVisible" :instance-db-id="dbId" :current-size-g-b="bootVolume?.sizeInGBs ?? 0" @saved="onSaved" />
    <CreateBackupModal v-model="createBackupVisible" :instance-id="instanceId" :tenant-id="tenantId ?? 0" @saved="onSaved" />
    <CopyBackupModal v-model="copyBackupVisible" :backup-id="copyBackupId" :tenant-id="tenantId ?? 0" @saved="onSaved" />
  </div>
</template>

<style scoped>
.disk-panel {
  padding: var(--space-4);
}
</style>
