<template>
  <div class="registry-page">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>容器镜像仓库</h2>
        <el-tag type="info" size="small">{{ repos.length }} 个仓库</el-tag>
      </div>
      <div class="toolbar-right">
        <el-button type="warning" @click="openCleanupDialog">
          <el-icon><Delete /></el-icon> 清理旧镜像
        </el-button>
        <el-button @click="loadRepos" :loading="loading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </div>
    </div>

    <!-- Repository List -->
    <el-card shadow="none" class="table-card">
      <el-table :data="repos" v-loading="loading" border stripe style="width: 100%" @row-click="showImages">
        <template #empty>
          <el-empty description="暂无镜像仓库" :image-size="80" />
        </template>
        <el-table-column prop="name" label="仓库名称" min-width="200" sortable show-overflow-tooltip>
          <template #default="{ row }">
            <span class="repo-name" @click="showImages(row)">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="namespace" label="命名空间" min-width="140" show-overflow-tooltip />
        <el-table-column prop="compartmentName" label="分区" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">{{ row.compartmentName || row.compartmentId || '-' }}</template>
        </el-table-column>
        <el-table-column label="镜像数量" width="100" align="center">
          <template #default="{ row }">
            <el-tag size="small">{{ row.imageCount ?? '-' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="160">
          <template #default="{ row }">{{ formatTime(row.timeCreated) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right" align="center">
          <template #default="{ row }">
            <el-button size="small" type="primary" link @click.stop="showImages(row)">
              <el-icon><View /></el-icon> 镜像
            </el-button>
            <el-popconfirm title="确定删除此仓库？仓库内所有镜像将一并删除！" @confirm="deleteRepo(row)">
              <template #reference>
                <el-button size="small" type="danger" link @click.stop>
                  <el-icon><Delete /></el-icon>
                </el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Image List Dialog -->
    <el-dialog
      v-model="imageDialogVisible"
      :title="`镜像列表 — ${selectedRepo}`"
      width="90%"
      destroy-on-close
    >
      <div style="margin-bottom: 12px; display: flex; align-items: center; justify-content: space-between">
        <el-tag type="info" size="small">{{ images.length }} 个镜像</el-tag>
        <el-button @click="refreshImages" :loading="imagesLoading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </div>
      <el-table :data="images" v-loading="imagesLoading" border stripe size="small">
        <template #empty>
          <el-empty description="暂无镜像" :image-size="60" />
        </template>
        <el-table-column prop="digest" label="Digest" min-width="240" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="data-mono" style="font-size: var(--text-xs)">{{ truncateDigest(row.digest) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="标签" min-width="200">
          <template #default="{ row }">
            <el-tag
              v-for="tag in (row.tags || [])"
              :key="tag"
              size="small"
              style="margin-right: 4px; margin-bottom: 2px"
            >{{ tag }}</el-tag>
            <span v-if="!row.tags || row.tags.length === 0" style="color: var(--text-muted)">-</span>
          </template>
        </el-table-column>
        <el-table-column label="大小" width="120" align="right">
          <template #default="{ row }">
            <span class="data-mono">{{ formatBytes(row.sizeInBytes) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="层数" width="70" align="center">
          <template #default="{ row }">{{ row.layersCount ?? '-' }}</template>
        </el-table-column>
        <el-table-column label="创建时间" width="160">
          <template #default="{ row }">{{ formatTime(row.timeCreated) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right" align="center">
          <template #default="{ row }">
            <el-popconfirm title="确定删除此镜像？" @confirm="deleteImage(row)">
              <template #reference>
                <el-button size="small" type="danger" link>
                  <el-icon><Delete /></el-icon> 删除
                </el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- Cleanup Dialog -->
    <el-dialog v-model="cleanupDialogVisible" title="清理旧镜像" width="500px" destroy-on-close>
      <el-alert
        title="将清理指定天数前的旧镜像，请谨慎操作"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
      />
      <el-form :model="cleanupForm" label-width="120px">
        <el-form-item label="分区">
          <el-input v-model="cleanupForm.compartmentName" placeholder="分区名称" disabled />
        </el-form-item>
        <el-form-item label="保留天数" required>
          <el-input-number v-model="cleanupForm.keepDays" :min="1" :max="365" :step="1" style="width: 100%" />
        </el-form-item>
        <el-form-item label="仓库名称">
          <el-input v-model="cleanupForm.repositoryName" placeholder="留空清理所有仓库" clearable />
        </el-form-item>
      </el-form>
      <el-alert
        :title="`将删除 ${cleanupForm.keepDays} 天前的所有镜像${cleanupForm.repositoryName ? '（仅限 ' + cleanupForm.repositoryName + '）' : ''}`"
        type="info"
        :closable="false"
        style="margin-top: 12px"
      />
      <template #footer>
        <el-button @click="cleanupDialogVisible = false">取消</el-button>
        <el-button type="warning" :loading="cleanupLoading" @click="doCleanup">确认清理</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Delete, View } from '@element-plus/icons-vue'
import request from '../utils/request'

// ---- Types ----
interface Repository {
  name: string
  namespace: string
  compartmentId: string
  compartmentName?: string
  imageCount?: number
  timeCreated: string
}

interface Image {
  digest: string
  tags: string[]
  sizeInBytes: number
  layersCount: number
  timeCreated: string
  repositoryName: string
}

// ---- State ----
const repos = ref<Repository[]>([])
const loading = ref(false)

// Image dialog
const imageDialogVisible = ref(false)
const imagesLoading = ref(false)
const selectedRepo = ref('')
const selectedCompartmentId = ref('')
const selectedCompartmentName = ref('')
const images = ref<Image[]>([])

// Cleanup dialog
const cleanupDialogVisible = ref(false)
const cleanupLoading = ref(false)
const cleanupForm = ref({
  compartmentId: '',
  compartmentName: '',
  keepDays: 30,
  repositoryName: '',
})

// ---- Helpers ----
function formatTime(t: string | undefined): string {
  if (!t) return '-'
  try { return new Date(t).toLocaleString('zh-CN') } catch { return t }
}

function formatBytes(bytes: number | undefined): string {
  if (!bytes && bytes !== 0) return '-'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1073741824) return (bytes / 1048576).toFixed(1) + ' MB'
  return (bytes / 1073741824).toFixed(2) + ' GB'
}

function truncateDigest(digest: string): string {
  if (!digest) return '-'
  if (digest.length <= 40) return digest
  return digest.substring(0, 20) + '...' + digest.substring(digest.length - 16)
}

// ---- Data Loading ----
async function loadRepos() {
  loading.value = true
  try {
    const res = await request.get('/oci/registry/repos') as any
    repos.value = Array.isArray(res) ? res : (res?.items || [])
  } catch (e: any) {
    ElMessage.error(e.message || '加载仓库列表失败')
  } finally {
    loading.value = false
  }
}

async function showImages(repo: Repository) {
  selectedRepo.value = repo.name
  selectedCompartmentId.value = repo.compartmentId
  selectedCompartmentName.value = repo.compartmentName || ''
  imageDialogVisible.value = true
  await loadImages()
}

async function loadImages() {
  imagesLoading.value = true
  try {
    const params: any = { repositoryName: selectedRepo.value }
    if (selectedCompartmentId.value) params.compartmentId = selectedCompartmentId.value
    if (selectedCompartmentName.value) params.compartmentName = selectedCompartmentName.value
    const res = await request.get('/oci/registry/images', { params }) as any
    images.value = Array.isArray(res) ? res : (res?.items || [])
  } catch (e: any) {
    ElMessage.error(e.message || '加载镜像列表失败')
    images.value = []
  } finally {
    imagesLoading.value = false
  }
}

async function refreshImages() {
  await loadImages()
}

// ---- Delete Image ----
async function deleteImage(image: Image) {
  try {
    await request.post('/oci/registry/image/delete', {
      repositoryName: selectedRepo.value,
      digest: image.digest,
    })
    ElMessage.success('镜像已删除')
    await loadImages()
    await loadRepos()
  } catch (e: any) {
    ElMessage.error(e.message || '删除镜像失败')
  }
}

// ---- Delete Repository ----
async function deleteRepo(repo: Repository) {
  try {
    await request.post('/oci/registry/repo/delete', {
      repositoryName: repo.name,
      compartmentId: repo.compartmentId,
    })
    ElMessage.success('仓库已删除')
    await loadRepos()
  } catch (e: any) {
    ElMessage.error(e.message || '删除仓库失败')
  }
}

// ---- Cleanup ----
function openCleanupDialog() {
  cleanupForm.value = {
    compartmentId: '',
    compartmentName: '',
    keepDays: 30,
    repositoryName: '',
  }
  cleanupDialogVisible.value = true
}

async function doCleanup() {
  cleanupLoading.value = true
  try {
    await request.post('/oci/registry/cleanup', {
      keepDays: cleanupForm.value.keepDays,
      repositoryName: cleanupForm.value.repositoryName || undefined,
      compartmentName: cleanupForm.value.compartmentName || undefined,
    })
    ElMessage.success('清理任务已提交')
    cleanupDialogVisible.value = false
    await loadRepos()
  } catch (e: any) {
    ElMessage.error(e.message || '清理失败')
  } finally {
    cleanupLoading.value = false
  }
}

onMounted(loadRepos)
</script>

<style scoped>
.registry-page {
  padding: 0;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-4);
  flex-wrap: wrap;
  gap: var(--space-4);
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.toolbar-left h2 {
  margin: 0;
  font-size: var(--text-xl);
  font-weight: var(--font-bold);
  color: var(--text-primary);
  letter-spacing: var(--tracking-tight);
}

.toolbar-right {
  display: flex;
  gap: var(--space-2);
}

.table-card {
  border-radius: var(--radius-md);
  overflow: hidden;
}

.table-card :deep(.el-card__body) {
  padding: 0;
}

.repo-name {
  cursor: pointer;
  color: var(--accent);
  font-weight: var(--font-medium);
}

.repo-name:hover {
  text-decoration: underline;
}

:deep(.el-table) {
  border-radius: var(--radius-md);
  overflow: hidden;
  font-size: var(--text-sm);
}

:deep(.el-table th) {
  background: var(--bg-raised);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

:deep(.el-dialog) {
  border-radius: var(--radius-lg);
}

:deep(.el-dialog__title) {
  font-size: var(--text-lg);
  font-weight: var(--font-semibold);
}

@media (max-width: 768px) {
  .toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .toolbar-left h2 {
    font-size: var(--text-lg);
  }
}
</style>
