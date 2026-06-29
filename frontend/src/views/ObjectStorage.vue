<template>
  <div class="storage-page">
    <!-- ================================================================ -->
    <!-- Bucket List View -->
    <!-- ================================================================ -->
    <template v-if="!selectedBucket">
      <div class="toolbar">
        <div class="toolbar-left">
          <h2>对象存储</h2>
          <el-tag type="info" size="small">{{ buckets.length }} 个存储桶</el-tag>
        </div>
        <div class="toolbar-right">
          <el-button type="primary" @click="openCreateBucket">
            <el-icon><Plus /></el-icon> 创建存储桶
          </el-button>
          <el-button @click="loadBuckets" :loading="loading">
            <el-icon><Refresh /></el-icon> 刷新
          </el-button>
        </div>
      </div>

      <!-- Tenant selector -->
      <div class="filter-bar">
        <el-select
          v-model="tenantId"
          placeholder="选择租户"
          filterable
          style="width: 260px"
          @change="onTenantChange"
        >
          <el-option
            v-for="t in tenantOptions"
            :key="t.id"
            :label="t.name"
            :value="t.id"
          />
        </el-select>
      </div>

      <el-card shadow="none" class="table-card">
        <el-table :data="buckets" v-loading="loading" border stripe style="width: 100%">
          <template #empty>
            <el-empty :description="!tenantId ? '请先选择租户' : '暂无存储桶'" :image-size="80">
              <el-button v-if="tenantId" type="primary" @click="openCreateBucket">创建存储桶</el-button>
            </el-empty>
          </template>
          <el-table-column type="index" label="#" width="50" align="center" />
          <el-table-column label="存储桶名称" min-width="180">
            <template #default="{ row }">
              <span class="cell-link" @click="openBucket(row)">{{ row.name }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="namespace" label="命名空间" width="140" />
          <el-table-column label="访问类型" width="150">
            <template #default="{ row }">
              <el-tag :type="accessTypeTag(row.publicAccess)" size="small">
                {{ accessTypeLabel(row.publicAccess) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="创建时间" width="180">
            <template #default="{ row }">
              <span class="data-mono" style="font-size:12px">{{ formatTime(row.timeCreated) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right" align="center">
            <template #default="{ row }">
              <el-button size="small" type="danger" @click="deleteBucket(row)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <!-- Bucket pagination -->
      <div v-if="bucketNextPage" class="pagination-bar">
        <el-button size="small" @click="loadBucketsNext">加载更多</el-button>
      </div>
    </template>

    <!-- ================================================================ -->
    <!-- Object List View (inside a bucket) -->
    <!-- ================================================================ -->
    <template v-else>
      <div class="toolbar">
        <div class="toolbar-left">
          <el-button size="small" @click="backToBuckets">
            <el-icon><ArrowLeft /></el-icon> 返回
          </el-button>
          <h2>{{ selectedBucket.name }}</h2>
          <el-tag type="info" size="small">namespace: {{ selectedBucket.namespace }}</el-tag>
        </div>
        <div class="toolbar-right">
          <el-button type="primary" @click="triggerUpload">
            <el-icon><Upload /></el-icon> 上传文件
          </el-button>
          <el-button @click="loadObjects" :loading="objLoading">
            <el-icon><Refresh /></el-icon> 刷新
          </el-button>
        </div>
      </div>

      <!-- Prefix filter -->
      <div class="filter-bar">
        <el-input
          v-model="prefixFilter"
          placeholder="按前缀过滤..."
          clearable
          style="width: 260px"
          @input="onPrefixChange"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-button size="small" @click="showResumableUploads">
          <el-icon><List /></el-icon> 未完成上传
        </el-button>
      </div>

      <!-- Upload drop zone -->
      <div
        class="upload-zone"
        :class="{ 'upload-zone--dragover': isDragover }"
        @dragover.prevent="isDragover = true"
        @dragleave="isDragover = false"
        @drop.prevent="onDrop"
        v-if="!uploading"
      >
        <el-icon :size="32"><Upload /></el-icon>
        <p>拖拽文件到此处上传，或 <span class="upload-zone-link" @click="triggerUpload">点击选择</span></p>
      </div>

      <!-- Upload progress -->
      <el-card v-if="uploading" shadow="none" class="upload-progress-card">
        <div class="upload-progress-header">
          <span class="upload-filename">{{ uploadingFileName }}</span>
          <el-button size="small" type="danger" @click="cancelUpload" v-if="uploadingMultipart">
            取消上传
          </el-button>
        </div>
        <el-progress
          :percentage="uploadProgress"
          :stroke-width="16"
          :text-inside="true"
          :status="uploadProgress >= 100 ? 'success' : ''"
        />
        <p class="upload-progress-info">
          {{ formatBytes(uploadBytesSent) }} / {{ formatBytes(uploadTotalSize) }}
          <span v-if="uploadSpeed"> &middot; {{ formatBytes(uploadSpeed) }}/s</span>
          <span v-if="uploadingMultipart"> &middot; 第 {{ uploadPartNum }}/{{ uploadTotalParts }} 分片</span>
        </p>
      </el-card>

      <!-- Hidden file input -->
      <input
        ref="fileInputRef"
        type="file"
        style="display:none"
        @change="onFileSelected"
        multiple
      />

      <!-- Object table -->
      <el-card shadow="none" class="table-card">
        <el-table :data="objects" v-loading="objLoading" border stripe style="width: 100%">
          <template #empty>
            <el-empty description="存储桶为空" :image-size="80" />
          </template>
          <el-table-column label="文件名" min-width="280">
            <template #default="{ row }">
              <span class="data-mono">{{ row.name }}</span>
            </template>
          </el-table-column>
          <el-table-column label="大小" width="120" align="right">
            <template #default="{ row }">
              <span class="data-mono">{{ formatBytes(row.size) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="类型" width="160">
            <template #default="{ row }">
              <el-tag size="small" effect="plain">{{ row.contentType || '-' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="最后修改" width="180">
            <template #default="{ row }">
              <span class="data-mono" style="font-size:12px">{{ formatTime(row.timeModified) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="MD5" width="150" show-overflow-tooltip>
            <template #default="{ row }">
              <span class="data-mono" style="font-size:11px">{{ row.md5 || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="280" fixed="right" align="center">
            <template #default="{ row }">
              <el-button size="small" @click="previewObject(row)">预览</el-button>
              <el-button size="small" @click="downloadObject(row)">下载</el-button>
              <el-button size="small" @click="generatePresignedUrl(row)">链接</el-button>
              <el-button size="small" type="danger" @click="deleteObject(row)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <!-- Object pagination -->
      <div v-if="objNextToken" class="pagination-bar">
        <el-button size="small" @click="loadObjectsNext">加载更多</el-button>
      </div>
    </template>

    <!-- ================================================================ -->
    <!-- Create Bucket Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="createBucketVisible" title="创建存储桶" width="480px" destroy-on-close>
      <el-form :model="createBucketForm" label-width="100px">
        <el-form-item label="存储桶名称" required>
          <el-input v-model="createBucketForm.name" placeholder="my-bucket" />
        </el-form-item>
        <el-form-item label="访问类型">
          <el-select v-model="createBucketForm.publicAccessType" style="width:100%">
            <el-option label="私有 (NoPublicAccess)" value="NoPublicAccess" />
            <el-option label="对象可读 (ObjectRead)" value="ObjectRead" />
            <el-option label="对象可读无列表 (ObjectReadWithoutList)" value="ObjectReadWithoutList" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createBucketVisible = false">取消</el-button>
        <el-button type="primary" :loading="createBucketSaving" @click="doCreateBucket">创建</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Presigned URL Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="presignedVisible" title="预签名链接" width="560px" destroy-on-close>
      <el-form :model="presignedForm" label-width="100px">
        <el-form-item label="文件">
          <el-input :model-value="presignedForm.objectName" disabled />
        </el-form-item>
        <el-form-item label="有效期(秒)">
          <el-input-number v-model="presignedForm.validitySeconds" :min="60" :max="604800" :step="3600" style="width:100%" />
        </el-form-item>
      </el-form>
      <div v-if="presignedUrl" style="margin-top:12px">
        <el-input v-model="presignedUrl" readonly>
          <template #append>
            <el-button @click="copyText(presignedUrl)">复制</el-button>
          </template>
        </el-input>
      </div>
      <template #footer>
        <el-button @click="presignedVisible = false">关闭</el-button>
        <el-button type="primary" :loading="presignedLoading" @click="doGeneratePresigned">生成链接</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Preview Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="previewVisible" title="文件预览" width="80%" destroy-on-close>
      <div v-if="previewLoading" style="text-align:center;padding:48px">
        <el-skeleton :rows="5" animated />
      </div>
      <template v-else>
        <div v-if="previewType === 'image'" class="preview-image-container">
          <img :src="previewSrc" alt="preview" class="preview-image" />
        </div>
        <div v-else-if="previewType === 'text'" class="preview-text-container">
          <pre class="preview-text">{{ previewContent }}</pre>
        </div>
        <div v-else style="text-align:center;padding:24px;color:var(--text-secondary)">
          <p>无法预览此文件类型: {{ previewContentType }}</p>
          <el-button type="primary" @click="downloadObject(previewTarget!)">下载文件</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Resumable Uploads Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="resumableVisible" title="未完成的上传" width="700px" destroy-on-close>
      <el-table :data="resumableUploads" border size="small" v-loading="resumableLoading">
        <template #empty><el-empty description="没有未完成的上传" :image-size="60" /></template>
        <el-table-column prop="objectName" label="文件名" min-width="160" show-overflow-tooltip />
        <el-table-column label="进度" width="160">
          <template #default="{ row }">
            <el-progress
              :percentage="row.totalParts > 0 ? Math.round(row.completedPartCount / row.totalParts * 100) : 0"
              :stroke-width="10"
              :text-inside="true"
            />
          </template>
        </el-table-column>
        <el-table-column label="大小" width="120">
          <template #default="{ row }">{{ formatBytes(row.totalSize) }}</template>
        </el-table-column>
        <el-table-column label="已上传分片" width="110" align="center">
          <template #default="{ row }">{{ row.completedPartCount }}/{{ row.totalParts }}</template>
        </el-table-column>
        <el-table-column label="创建时间" width="160">
          <template #default="{ row }">{{ row.createTime }}</template>
        </el-table-column>
        <el-table-column label="操作" width="120" align="center">
          <template #default="{ row }">
            <el-button size="small" type="danger" @click="abortResumableUpload(row)">放弃</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus, Refresh, Delete, Upload, Download, ArrowLeft, Search, List,
} from '@element-plus/icons-vue'
import request from '../utils/request'

// --- Types ---
interface Bucket {
  name: string
  namespace: string
  timeCreated: string
  publicAccess: string
}

interface StorageObject {
  name: string
  size: number
  timeModified: string
  contentType: string
  md5: string
}

interface MultipartUploadRecord {
  id: number
  uploadId: string
  objectName: string
  bucketName: string
  namespace: string
  totalSize: number
  chunkSize: number
  totalParts: number
  completedPartCount: number
  completedParts: Array<{ partNum: number; etag: string }>
  createTime: string
}

// --- Constants ---
const MULTIPART_THRESHOLD = 100 * 1024 * 1024 // 100MB
const DEFAULT_CHUNK_SIZE = 10 * 1024 * 1024 // 10MB

// --- State ---
const tenantId = ref<number | null>(null)
const tenantOptions = ref<Array<{ id: number; name: string }>>([])
const namespace = ref('')

// Bucket state
const buckets = ref<Bucket[]>([])
const loading = ref(false)
const bucketNextPage = ref('')
const selectedBucket = ref<Bucket | null>(null)

// Object state
const objects = ref<StorageObject[]>([])
const objLoading = ref(false)
const objNextToken = ref('')
const prefixFilter = ref('')
const isDragover = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)

// Upload state
const uploading = ref(false)
const uploadingFileName = ref('')
const uploadingMultipart = ref(false)
const uploadProgress = ref(0)
const uploadBytesSent = ref(0)
const uploadTotalSize = ref(0)
const uploadSpeed = ref(0)
const uploadPartNum = ref(0)
const uploadTotalParts = ref(0)
let uploadAbortController: AbortController | null = null

// Create bucket dialog
const createBucketVisible = ref(false)
const createBucketSaving = ref(false)
const createBucketForm = ref({ name: '', publicAccessType: 'NoPublicAccess' })

// Presigned URL dialog
const presignedVisible = ref(false)
const presignedLoading = ref(false)
const presignedForm = ref({ objectName: '', validitySeconds: 3600 })
const presignedUrl = ref('')

// Preview dialog
const previewVisible = ref(false)
const previewLoading = ref(false)
const previewType = ref<'image' | 'text' | 'unknown'>('unknown')
const previewSrc = ref('')
const previewContent = ref('')
const previewContentType = ref('')
const previewTarget = ref<StorageObject | null>(null)

// Resumable uploads dialog
const resumableVisible = ref(false)
const resumableLoading = ref(false)
const resumableUploads = ref<MultipartUploadRecord[]>([])

// --- Helpers ---
function formatBytes(bytes: number | undefined): string {
  if (!bytes || isNaN(bytes)) return '0 B'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1073741824) return (bytes / 1048576).toFixed(1) + ' MB'
  return (bytes / 1073741824).toFixed(2) + ' GB'
}

function formatTime(t: string | undefined): string {
  if (!t) return '-'
  try { return new Date(t).toLocaleString('zh-CN') } catch { return t }
}

function accessTypeTag(t: string): 'success' | 'warning' | 'info' | '' {
  if (!t || t === 'NoPublicAccess') return 'success'
  if (t === 'ObjectRead') return 'warning'
  return 'info'
}

function accessTypeLabel(t: string): string {
  if (!t || t === 'NoPublicAccess') return '私有'
  if (t === 'ObjectRead') return '对象可读'
  if (t === 'ObjectReadWithoutList') return '可读无列表'
  return t
}

function copyText(text: string) {
  navigator.clipboard?.writeText(text).then(() => ElMessage.success('已复制到剪贴板')).catch(() => {})
}

function getObjectNameFromPath(path: string): string {
  const parts = path.split('/')
  return parts[parts.length - 1] || path
}

function isPreviewableContentType(ct: string): boolean {
  if (!ct) return false
  const lower = ct.toLowerCase()
  return lower.startsWith('image/') || lower.startsWith('text/') ||
    lower.includes('pdf') || lower.includes('json') || lower.includes('xml')
}

function inferContentType(filename: string): string {
  const ext = filename.split('.').pop()?.toLowerCase() || ''
  const map: Record<string, string> = {
    png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', gif: 'image/gif',
    webp: 'image/webp', svg: 'image/svg+xml', bmp: 'image/bmp',
    pdf: 'application/pdf',
    json: 'application/json', xml: 'application/xml',
    txt: 'text/plain', log: 'text/plain', md: 'text/plain', csv: 'text/plain',
    html: 'text/html', htm: 'text/html', css: 'text/css', js: 'text/javascript',
    zip: 'application/zip', gz: 'application/gzip', tar: 'application/x-tar',
  }
  return map[ext] || 'application/octet-stream'
}

// --- Tenant loading ---
async function loadTenants() {
  try {
    const tenants = await request.get('/tenants/listAll') as any[]
    tenantOptions.value = tenants.map((t: any) => ({
      id: t.id,
      name: t.userName || t.tenancyName || `#${t.id}`,
    }))
    // Auto-select first tenant
    if (tenantOptions.value.length > 0 && !tenantId.value) {
      tenantId.value = tenantOptions.value[0].id
      await onTenantChange()
    }
  } catch { /* ignore */ }
}

async function onTenantChange() {
  selectedBucket.value = null
  buckets.value = []
  bucketNextPage.value = ''
  namespace.value = ''
  if (tenantId.value) {
    await loadNamespace()
    await loadBuckets()
  }
}

async function loadNamespace() {
  if (!tenantId.value) return
  try {
    const resp: any = await request.get('/oci/storage/namespace', { params: { tenantId: tenantId.value } })
    namespace.value = resp?.namespace || ''
  } catch (e: any) {
    ElMessage.error('获取命名空间失败: ' + (e?.message || e))
  }
}

// --- Bucket operations ---
async function loadBuckets() {
  if (!tenantId.value) return
  loading.value = true
  buckets.value = []
  bucketNextPage.value = ''
  try {
    const resp: any = await request.get('/oci/storage/buckets', {
      params: { tenantId: tenantId.value, limit: 100 },
    })
    buckets.value = resp?.items || []
    bucketNextPage.value = resp?.nextPage || ''
  } catch (e: any) {
    ElMessage.error('加载存储桶失败: ' + (e?.message || e))
  } finally {
    loading.value = false
  }
}

async function loadBucketsNext() {
  if (!tenantId.value || !bucketNextPage.value) return
  loading.value = true
  try {
    const resp: any = await request.get('/oci/storage/buckets', {
      params: { tenantId: tenantId.value, limit: 100, pageToken: bucketNextPage.value },
    })
    buckets.value = [...buckets.value, ...(resp?.items || [])]
    bucketNextPage.value = resp?.nextPage || ''
  } catch (e: any) {
    ElMessage.error('加载更多失败: ' + (e?.message || e))
  } finally {
    loading.value = false
  }
}

function openCreateBucket() {
  createBucketForm.value = { name: '', publicAccessType: 'NoPublicAccess' }
  createBucketVisible.value = true
}

async function doCreateBucket() {
  if (!createBucketForm.value.name.trim()) {
    ElMessage.warning('请输入存储桶名称')
    return
  }
  if (!tenantId.value) {
    ElMessage.warning('请先选择租户')
    return
  }
  createBucketSaving.value = true
  try {
    await request.post('/oci/storage/bucket/create', {
      tenantId: tenantId.value,
      bucketName: createBucketForm.value.name.trim(),
      publicAccessType: createBucketForm.value.publicAccessType,
    })
    ElMessage.success('存储桶创建成功')
    createBucketVisible.value = false
    await loadBuckets()
  } catch (e: any) {
    ElMessage.error('创建失败: ' + (e?.message || e))
  } finally {
    createBucketSaving.value = false
  }
}

async function deleteBucket(bucket: Bucket) {
  try {
    await ElMessageBox.confirm(
      `确定删除存储桶「${bucket.name}」？存储桶必须为空才能删除。`,
      '确认删除',
      { type: 'warning', confirmButtonText: '确定删除' }
    )
    await request.post('/oci/storage/bucket/delete', {
      tenantId: tenantId.value,
      namespace: bucket.namespace,
      bucketName: bucket.name,
    })
    ElMessage.success('存储桶已删除')
    await loadBuckets()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// --- Object operations ---
function openBucket(bucket: Bucket) {
  selectedBucket.value = bucket
  objects.value = []
  objNextToken.value = ''
  prefixFilter.value = ''
  loadObjects()
}

function backToBuckets() {
  selectedBucket.value = null
  objects.value = []
  objNextToken.value = ''
}

async function loadObjects() {
  if (!selectedBucket.value || !tenantId.value) return
  objLoading.value = true
  objects.value = []
  objNextToken.value = ''
  try {
    const params: any = {
      tenantId: tenantId.value,
      namespace: selectedBucket.value.namespace,
      bucketName: selectedBucket.value.name,
      limit: 100,
    }
    if (prefixFilter.value) params.prefix = prefixFilter.value
    const resp: any = await request.get('/oci/storage/objects', { params })
    objects.value = resp?.items || []
    objNextToken.value = resp?.nextStartWith || ''
  } catch (e: any) {
    ElMessage.error('加载对象失败: ' + (e?.message || e))
  } finally {
    objLoading.value = false
  }
}

async function loadObjectsNext() {
  if (!selectedBucket.value || !tenantId.value || !objNextToken.value) return
  objLoading.value = true
  try {
    const params: any = {
      tenantId: tenantId.value,
      namespace: selectedBucket.value.namespace,
      bucketName: selectedBucket.value.name,
      limit: 100,
      startToken: objNextToken.value,
    }
    if (prefixFilter.value) params.prefix = prefixFilter.value
    const resp: any = await request.get('/oci/storage/objects', { params })
    objects.value = [...objects.value, ...(resp?.items || [])]
    objNextToken.value = resp?.nextStartWith || ''
  } catch (e: any) {
    ElMessage.error('加载更多失败: ' + (e?.message || e))
  } finally {
    objLoading.value = false
  }
}

let prefixDebounceTimer: ReturnType<typeof setTimeout> | null = null
function onPrefixChange() {
  if (prefixDebounceTimer) clearTimeout(prefixDebounceTimer)
  prefixDebounceTimer = setTimeout(() => loadObjects(), 400)
}

async function deleteObject(obj: StorageObject) {
  if (!selectedBucket.value || !tenantId.value) return
  try {
    await ElMessageBox.confirm(
      `确定删除文件「${obj.name}」？`,
      '确认删除',
      { type: 'warning' }
    )
    await request.post('/oci/storage/object/delete', {
      tenantId: tenantId.value,
      namespace: selectedBucket.value.namespace,
      bucketName: selectedBucket.value.name,
      objectName: obj.name,
    })
    ElMessage.success('文件已删除')
    await loadObjects()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

function downloadObject(obj: StorageObject) {
  if (!selectedBucket.value || !tenantId.value) return
  const params = new URLSearchParams({
    tenantId: String(tenantId.value),
    namespace: selectedBucket.value.namespace,
    bucketName: selectedBucket.value.name,
    objectName: obj.name,
  })
  const a = document.createElement('a')
  a.href = `/oci/storage/object/download?${params.toString()}`
  a.download = getObjectNameFromPath(obj.name)
  document.body.appendChild(a)
  a.click()
  a.remove()
}

async function previewObject(obj: StorageObject) {
  if (!selectedBucket.value || !tenantId.value) return
  previewTarget.value = obj
  previewVisible.value = true
  previewLoading.value = true
  previewType.value = 'unknown'
  previewSrc.value = ''
  previewContent.value = ''
  previewContentType.value = obj.contentType || ''

  const baseUrl = '/oci/storage/object/preview'
  const params = new URLSearchParams({
    tenantId: String(tenantId.value),
    namespace: selectedBucket.value.namespace,
    bucketName: selectedBucket.value.name,
    objectName: obj.name,
  })
  const url = `${baseUrl}?${params.toString()}`

  try {
    const ct = (obj.contentType || '').toLowerCase()
    if (ct.startsWith('image/')) {
      previewType.value = 'image'
      previewSrc.value = url
    } else if (ct.startsWith('text/') || ct.includes('json') || ct.includes('xml') || ct.includes('javascript')) {
      previewType.value = 'text'
      const resp = await fetch(url, { credentials: 'include' })
      if (resp.ok) {
        previewContent.value = await resp.text()
      } else {
        previewContent.value = '加载失败'
      }
    } else {
      // Try to fetch and check content type
      const resp = await fetch(url, { credentials: 'include' })
      const respCt = resp.headers.get('content-type') || ''
      if (respCt.startsWith('image/')) {
        previewType.value = 'image'
        const blob = await resp.blob()
        previewSrc.value = URL.createObjectURL(blob)
      } else if (respCt.startsWith('text/') || respCt.includes('json') || respCt.includes('xml')) {
        previewType.value = 'text'
        previewContent.value = await resp.text()
      } else {
        previewContentType.value = respCt || obj.contentType || 'unknown'
      }
    }
  } catch (e: any) {
    ElMessage.error('预览失败: ' + (e?.message || e))
  } finally {
    previewLoading.value = false
  }
}

// --- Presigned URL ---
async function generatePresignedUrl(obj: StorageObject) {
  presignedForm.value = { objectName: obj.name, validitySeconds: 3600 }
  presignedUrl.value = ''
  presignedVisible.value = true
}

async function doGeneratePresigned() {
  if (!selectedBucket.value || !tenantId.value) return
  presignedLoading.value = true
  presignedUrl.value = ''
  try {
    const resp: any = await request.post('/oci/storage/object/presigned', {
      tenantId: tenantId.value,
      namespace: selectedBucket.value.namespace,
      bucketName: selectedBucket.value.name,
      objectName: presignedForm.value.objectName,
      validitySeconds: presignedForm.value.validitySeconds,
    })
    presignedUrl.value = resp?.url || ''
  } catch (e: any) {
    ElMessage.error('生成链接失败: ' + (e?.message || e))
  } finally {
    presignedLoading.value = false
  }
}

// --- File upload ---
function triggerUpload() {
  fileInputRef.value?.click()
}

function onFileSelected(e: Event) {
  const files = (e.target as HTMLInputElement).files
  if (!files || files.length === 0) return
  // Upload files sequentially
  uploadFiles(Array.from(files))
  // Reset input
  if (fileInputRef.value) fileInputRef.value.value = ''
}

function onDrop(e: DragEvent) {
  isDragover.value = false
  const files = e.dataTransfer?.files
  if (!files || files.length === 0) return
  uploadFiles(Array.from(files))
}

async function uploadFiles(files: File[]) {
  for (const file of files) {
    if (file.size > MULTIPART_THRESHOLD) {
      await multipartUpload(file)
    } else {
      await singleUpload(file)
    }
  }
}

// --- Single PUT upload ---
async function singleUpload(file: File) {
  if (!selectedBucket.value || !tenantId.value) return
  uploading.value = true
  uploadingFileName.value = file.name
  uploadingMultipart.value = false
  uploadProgress.value = 0
  uploadBytesSent.value = 0
  uploadTotalSize.value = file.size

  try {
    const fd = new FormData()
    fd.append('tenantId', String(tenantId.value))
    fd.append('namespace', selectedBucket.value.namespace)
    fd.append('bucketName', selectedBucket.value.name)
    fd.append('objectName', file.name)
    fd.append('file', file)

    // Use XMLHttpRequest for upload progress
    await new Promise<void>((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', '/oci/storage/object/upload')
      xhr.withCredentials = true

      xhr.upload.onprogress = (ev) => {
        if (ev.lengthComputable) {
          uploadBytesSent.value = ev.loaded
          uploadProgress.value = Math.round((ev.loaded / ev.total) * 100)
        }
      }

      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          uploadProgress.value = 100
          uploadBytesSent.value = file.size
          resolve()
        } else {
          let msg = '上传失败'
          try {
            const body = JSON.parse(xhr.responseText)
            msg = body.message || msg
          } catch { /* ignore */ }
          reject(new Error(msg))
        }
      }

      xhr.onerror = () => reject(new Error('网络错误'))
      xhr.send(fd)
    })

    ElMessage.success(`${file.name} 上传成功`)
    await loadObjects()
  } catch (e: any) {
    ElMessage.error('上传失败: ' + (e?.message || e))
  } finally {
    uploading.value = false
  }
}

// --- Multipart upload ---
async function multipartUpload(file: File) {
  if (!selectedBucket.value || !tenantId.value) return
  uploading.value = true
  uploadingFileName.value = file.name
  uploadingMultipart.value = true
  uploadProgress.value = 0
  uploadBytesSent.value = 0
  uploadTotalSize.value = file.size
  uploadSpeed.value = 0
  uploadAbortController = new AbortController()

  const chunkSize = DEFAULT_CHUNK_SIZE
  const totalParts = Math.ceil(file.size / chunkSize)
  uploadTotalParts.value = totalParts
  uploadPartNum.value = 0

  const bucket = selectedBucket.value
  const tid = tenantId.value
  const completedParts: Array<{ partNum: number; etag: string }> = []
  let uploadId = ''

  const startTime = Date.now()

  try {
    // 1. Initiate multipart upload
    const initResp: any = await request.post('/oci/storage/object/multipart/initiate', {
      tenantId: tid,
      namespace: bucket.namespace,
      bucketName: bucket.name,
      objectName: file.name,
      contentType: inferContentType(file.name),
      totalSize: file.size,
      chunkSize,
    })
    uploadId = initResp?.uploadId || ''

    // 2. Upload each part
    for (let i = 0; i < totalParts; i++) {
      if (uploadAbortController?.signal.aborted) {
        throw new Error('上传已取消')
      }

      const start = i * chunkSize
      const end = Math.min(start + chunkSize, file.size)
      const chunk = file.slice(start, end)

      uploadPartNum.value = i + 1

      const fd = new FormData()
      fd.append('tenantId', String(tid))
      fd.append('namespace', bucket.namespace)
      fd.append('bucketName', bucket.name)
      fd.append('objectName', file.name)
      fd.append('uploadId', uploadId)
      fd.append('partNumber', String(i + 1))
      fd.append('chunk', chunk)

      const partResp: any = await new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest()
        xhr.open('POST', '/oci/storage/object/multipart/part')
        xhr.withCredentials = true

        xhr.upload.onprogress = (ev) => {
          if (ev.lengthComputable) {
            const partBytes = ev.loaded
            uploadBytesSent.value = start + partBytes
            const elapsed = (Date.now() - startTime) / 1000
            uploadSpeed.value = elapsed > 0 ? uploadBytesSent.value / elapsed : 0
            uploadProgress.value = Math.round((uploadBytesSent.value / file.size) * 100)
          }
        }

        xhr.onload = () => {
          if (xhr.status >= 200 && xhr.status < 300) {
            try { resolve(JSON.parse(xhr.responseText)) } catch { resolve({}) }
          } else {
            let msg = '分片上传失败'
            try { msg = JSON.parse(xhr.responseText).message || msg } catch { /* */ }
            reject(new Error(msg))
          }
        }

        xhr.onerror = () => reject(new Error('网络错误'))

        if (uploadAbortController?.signal.aborted) {
          reject(new Error('上传已取消'))
          return
        }
        xhr.send(fd)
      })

      const etag = partResp?.data?.etag || partResp?.etag || ''
      completedParts.push({ partNum: i + 1, etag })
    }

    // 3. Commit multipart upload
    await request.post('/oci/storage/object/multipart/commit', {
      tenantId: tid,
      namespace: bucket.namespace,
      bucketName: bucket.name,
      objectName: file.name,
      uploadId,
      parts: completedParts,
    })

    uploadProgress.value = 100
    uploadBytesSent.value = file.size
    ElMessage.success(`${file.name} 上传完成`)
    await loadObjects()
  } catch (e: any) {
    if (uploadId) {
      // Abort the multipart upload on error
      try {
        await request.post('/oci/storage/object/multipart/abort', {
          tenantId: tid,
          namespace: bucket.namespace,
          bucketName: bucket.name,
          objectName: file.name,
          uploadId,
        })
      } catch { /* ignore abort error */ }
    }
    if (e?.message !== '上传已取消') {
      ElMessage.error('上传失败: ' + (e?.message || e))
    }
  } finally {
    uploading.value = false
    uploadAbortController = null
  }
}

function cancelUpload() {
  uploadAbortController?.abort()
}

// --- Resumable uploads ---
async function showResumableUploads() {
  if (!selectedBucket.value || !tenantId.value) return
  resumableVisible.value = true
  resumableLoading.value = true
  try {
    const resp: any = await request.get('/oci/storage/object/multipart/resumeable', {
      params: {
        tenantId: tenantId.value,
        bucketName: selectedBucket.value.name,
      },
    })
    resumableUploads.value = resp || []
  } catch (e: any) {
    ElMessage.error('加载失败: ' + (e?.message || e))
    resumableUploads.value = []
  } finally {
    resumableLoading.value = false
  }
}

async function abortResumableUpload(record: MultipartUploadRecord) {
  try {
    await ElMessageBox.confirm(
      `确定放弃上传「${record.objectName}」？已上传的 ${record.completedPartCount} 个分片将被丢弃。`,
      '确认放弃',
      { type: 'warning' }
    )
    await request.post('/oci/storage/object/multipart/abort', {
      tenantId: tenantId.value,
      namespace: record.namespace,
      bucketName: record.bucketName,
      objectName: record.objectName,
      uploadId: record.uploadId,
    })
    ElMessage.success('已放弃')
    resumableUploads.value = resumableUploads.value.filter(r => r.uploadId !== record.uploadId)
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// --- Init ---
onMounted(loadTenants)
</script>

<style scoped>
.storage-page {
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
  flex-wrap: wrap;
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

.filter-bar {
  display: flex;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
  flex-wrap: wrap;
  align-items: center;
}

.table-card {
  border-radius: var(--radius-md);
  overflow: hidden;
}

.table-card :deep(.el-card__body) {
  padding: 0;
}

.pagination-bar {
  display: flex;
  justify-content: center;
  margin-top: var(--space-4);
}

/* Clickable cell link */
.cell-link {
  cursor: pointer;
  color: var(--accent);
  font-weight: var(--font-medium);
}

.cell-link:hover {
  color: var(--accent-hover);
  text-decoration: underline;
}

/* Upload drop zone */
.upload-zone {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 32px;
  margin-bottom: var(--space-4);
  border: 2px dashed var(--border-default);
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  font-size: var(--text-sm);
  transition: border-color var(--transition-fast), background var(--transition-fast);
  cursor: pointer;
}

.upload-zone:hover,
.upload-zone--dragover {
  border-color: var(--accent);
  background: var(--accent-subtle);
}

.upload-zone-link {
  color: var(--accent);
  cursor: pointer;
  font-weight: var(--font-medium);
}

.upload-zone-link:hover {
  text-decoration: underline;
}

/* Upload progress */
.upload-progress-card {
  margin-bottom: var(--space-4);
}

.upload-progress-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.upload-filename {
  font-weight: var(--font-semibold);
  color: var(--text-primary);
  font-size: var(--text-sm);
}

.upload-progress-info {
  margin-top: 6px;
  font-size: var(--text-xs);
  color: var(--text-secondary);
}

/* Preview */
.preview-image-container {
  text-align: center;
  padding: 16px;
}

.preview-image {
  max-width: 100%;
  max-height: 70vh;
  border-radius: var(--radius-md);
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.preview-text-container {
  max-height: 70vh;
  overflow: auto;
  background: var(--bg-raised);
  border-radius: var(--radius-md);
  padding: 16px;
}

.preview-text {
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-primary);
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}

/* Element Plus deep overrides */
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

  .filter-bar {
    flex-direction: column;
  }
}
</style>
