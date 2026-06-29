<template>
  <div class="vision-page">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>AI 视觉分析</h2>
      </div>
      <div class="toolbar-right">
        <el-button @click="loadJobs" :loading="jobsLoading">
          <el-icon><Refresh /></el-icon> 刷新任务
        </el-button>
      </div>
    </div>

    <!-- Main Tabs -->
    <el-tabs v-model="activeTab" class="vision-tabs">
      <!-- ===================== Image Analysis Tab ===================== -->
      <el-tab-pane label="图像分析" name="image">
        <el-card shadow="none" class="form-card">
          <el-form label-width="120px">
            <el-form-item label="上传图片">
              <el-upload
                ref="imageUploadRef"
                :auto-upload="false"
                :limit="1"
                accept="image/*"
                :on-change="onImageChange"
                :on-remove="onImageRemove"
                list-type="picture-card"
              >
                <el-icon><Plus /></el-icon>
              </el-upload>
            </el-form-item>
            <el-form-item label="分析特征" required>
              <el-checkbox-group v-model="imageForm.features">
                <el-checkbox label="IMAGE_CLASSIFICATION" value="IMAGE_CLASSIFICATION">
                  图像分类
                </el-checkbox>
                <el-checkbox label="OBJECT_DETECTION" value="OBJECT_DETECTION">
                  目标检测
                </el-checkbox>
                <el-checkbox label="FACE_DETECTION" value="FACE_DETECTION">
                  人脸检测
                </el-checkbox>
                <el-checkbox label="TEXT_DETECTION" value="TEXT_DETECTION">
                  文字检测
                </el-checkbox>
              </el-checkbox-group>
            </el-form-item>
            <el-form-item label="语言">
              <el-select v-model="imageForm.language" style="width: 200px">
                <el-option label="自动检测" value="" />
                <el-option label="中文" value="zh" />
                <el-option label="英文" value="en" />
                <el-option label="日文" value="ja" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="imageAnalyzing" @click="analyzeImage">
                <el-icon><VideoPlay /></el-icon> 分析图像
              </el-button>
              <el-button @click="createImageJob" :loading="imageJobCreating">
                <el-icon><Operation /></el-icon> 创建异步任务
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <!-- Image Analysis Results -->
        <el-card v-if="imageResult" shadow="none" class="result-card">
          <template #header>
            <span>分析结果</span>
          </template>

          <!-- Image Classification -->
          <div v-if="imageResult.imageClassification" class="result-section">
            <h4>图像分类</h4>
            <el-table :data="imageResult.imageClassification.labels || []" border size="small">
              <el-table-column prop="name" label="分类名称" min-width="200" />
              <el-table-column label="置信度" width="120" align="center">
                <template #default="{ row }">
                  <el-progress
                    :percentage="Math.round((row.confidence || 0) * 100)"
                    :stroke-width="12"
                    :text-inside="true"
                  />
                </template>
              </el-table-column>
            </el-table>
          </div>

          <!-- Object Detection -->
          <div v-if="imageResult.objectDetection" class="result-section">
            <h4>目标检测</h4>
            <div class="detection-grid">
              <div
                v-for="(obj, idx) in (imageResult.objectDetection.detectedObjects || [])"
                :key="idx"
                class="detection-item"
              >
                <div class="detection-label">
                  <el-tag size="small">{{ obj.name || 'Object' }}</el-tag>
                  <span class="confidence">{{ Math.round((obj.confidence || 0) * 100) }}%</span>
                </div>
                <div v-if="obj.boundingPolygon" class="bounding-box">
                  <span class="data-mono" style="font-size: var(--text-xs)">
                    {{ formatBoundingPolygon(obj.boundingPolygon) }}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <!-- Face Detection -->
          <div v-if="imageResult.faceDetection" class="result-section">
            <h4>人脸检测</h4>
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="检测到人脸数">
                {{ imageResult.faceDetection.faceCount ?? imageResult.faceDetection.faces?.length ?? 0 }}
              </el-descriptions-item>
            </el-descriptions>
            <el-table
              v-if="imageResult.faceDetection.faces?.length"
              :data="imageResult.faceDetection.faces"
              border
              size="small"
              style="margin-top: 8px"
            >
              <el-table-column label="置信度" width="120" align="center">
                <template #default="{ row }">
                  {{ Math.round((row.confidence || 0) * 100) }}%
                </template>
              </el-table-column>
              <el-table-column label="位置" min-width="200">
                <template #default="{ row }">
                  <span class="data-mono" style="font-size: var(--text-xs)">
                    {{ formatBoundingPolygon(row.boundingPolygon) }}
                  </span>
                </template>
              </el-table-column>
            </el-table>
          </div>

          <!-- Text Detection -->
          <div v-if="imageResult.textDetection" class="result-section">
            <h4>文字检测</h4>
            <div v-if="imageResult.textDetection.text" class="text-output">
              <pre>{{ imageResult.textDetection.text }}</pre>
            </div>
            <el-table
              v-if="imageResult.textDetection.words?.length"
              :data="imageResult.textDetection.words"
              border
              size="small"
              style="margin-top: 8px"
            >
              <el-table-column prop="text" label="文字" min-width="160" />
              <el-table-column label="置信度" width="120" align="center">
                <template #default="{ row }">
                  {{ Math.round((row.confidence || 0) * 100) }}%
                </template>
              </el-table-column>
            </el-table>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- ===================== Document Analysis Tab ===================== -->
      <el-tab-pane label="文档分析" name="document">
        <el-card shadow="none" class="form-card">
          <el-form label-width="120px">
            <el-form-item label="上传文档">
              <el-upload
                ref="docUploadRef"
                :auto-upload="false"
                :limit="1"
                accept=".pdf,.jpg,.jpeg,.png,.tiff,.tif"
                :on-change="onDocChange"
                :on-remove="onDocRemove"
                drag
              >
                <el-icon class="el-icon--upload"><Upload /></el-icon>
                <div class="el-upload__text">拖拽文件到此处，或 <em>点击上传</em></div>
                <template #tip>
                  <div class="el-upload__tip">支持 PDF、JPG、PNG、TIFF 格式</div>
                </template>
              </el-upload>
            </el-form-item>
            <el-form-item label="分析特征" required>
              <el-checkbox-group v-model="docForm.features">
                <el-checkbox label="TEXT_EXTRACTION" value="TEXT_EXTRACTION">
                  文字提取
                </el-checkbox>
                <el-checkbox label="TABLE_EXTRACTION" value="TABLE_EXTRACTION">
                  表格提取
                </el-checkbox>
                <el-checkbox label="KEY_VALUE_EXTRACTION" value="KEY_VALUE_EXTRACTION">
                  键值对提取
                </el-checkbox>
                <el-checkbox label="LANGUAGE_DETECTION" value="LANGUAGE_DETECTION">
                  语言检测
                </el-checkbox>
              </el-checkbox-group>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="docAnalyzing" @click="analyzeDocument">
                <el-icon><VideoPlay /></el-icon> 分析文档
              </el-button>
              <el-button @click="createDocJob" :loading="docJobCreating">
                <el-icon><Operation /></el-icon> 创建异步任务
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <!-- Document Analysis Results -->
        <el-card v-if="docResult" shadow="none" class="result-card">
          <template #header>
            <span>分析结果</span>
          </template>

          <!-- Text Extraction -->
          <div v-if="docResult.textExtraction" class="result-section">
            <h4>文字提取</h4>
            <div class="text-output">
              <pre>{{ docResult.textExtraction.text || '-' }}</pre>
            </div>
          </div>

          <!-- Table Extraction -->
          <div v-if="docResult.tableExtraction" class="result-section">
            <h4>表格提取</h4>
            <div v-if="docResult.tableExtraction.tables?.length">
              <div v-for="(table, tIdx) in docResult.tableExtraction.tables" :key="tIdx" class="table-block">
                <h5>表格 {{ tIdx + 1 }}</h5>
                <el-table :data="table.rows || []" border size="small">
                  <el-table-column
                    v-for="(_, colIdx) in (table.headerRows?.[0] || table.rows?.[0] || [])"
                    :key="colIdx"
                    :label="table.headerRows?.[0]?.[colIdx] || `列 ${colIdx + 1}`"
                    min-width="120"
                  >
                    <template #default="{ row }">
                      {{ row[colIdx] || '-' }}
                    </template>
                  </el-table-column>
                </el-table>
              </div>
            </div>
            <div v-else class="text-output">
              <pre>{{ JSON.stringify(docResult.tableExtraction, null, 2) }}</pre>
            </div>
          </div>

          <!-- Key-Value Extraction -->
          <div v-if="docResult.keyValueExtraction" class="result-section">
            <h4>键值对提取</h4>
            <el-table :data="keyValuePairs" border size="small">
              <el-table-column prop="key" label="键" min-width="160" />
              <el-table-column prop="value" label="值" min-width="200" />
              <el-table-column label="置信度" width="120" align="center">
                <template #default="{ row }">
                  {{ row.confidence ? Math.round(row.confidence * 100) + '%' : '-' }}
                </template>
              </el-table-column>
            </el-table>
          </div>

          <!-- Language Detection -->
          <div v-if="docResult.languageDetection" class="result-section">
            <h4>语言检测</h4>
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="检测语言">
                {{ docResult.languageDetection.language || '-' }}
              </el-descriptions-item>
              <el-descriptions-item label="置信度">
                {{ docResult.languageDetection.confidence ? Math.round(docResult.languageDetection.confidence * 100) + '%' : '-' }}
              </el-descriptions-item>
            </el-descriptions>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- ===================== Job History Tab ===================== -->
      <el-tab-pane label="任务历史" name="jobs">
        <el-card shadow="none" class="table-card">
          <div style="margin-bottom: 12px; display: flex; justify-content: flex-end">
            <el-button @click="loadJobs" :loading="jobsLoading">
              <el-icon><Refresh /></el-icon> 刷新
            </el-button>
          </div>
          <el-table :data="jobs" v-loading="jobsLoading" border stripe>
            <template #empty>
              <el-empty description="暂无任务记录" :image-size="60" />
            </template>
            <el-table-column prop="jobId" label="任务ID" min-width="200" show-overflow-tooltip>
              <template #default="{ row }">
                <span class="data-mono" style="font-size: var(--text-xs)">{{ row.jobId }}</span>
              </template>
            </el-table-column>
            <el-table-column label="类型" width="120">
              <template #default="{ row }">
                <el-tag :type="row.jobType === 'IMAGE' ? '' : 'warning'" size="small">
                  {{ row.jobType === 'IMAGE' ? '图像分析' : '文档分析' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="100" align="center">
              <template #default="{ row }">
                <span class="status-dot" :class="jobStateDot(row.lifecycleState)"></span>
                <span class="state-text">{{ jobStateLabel(row.lifecycleState) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="特征" min-width="200">
              <template #default="{ row }">
                <el-tag
                  v-for="f in (row.features || [])"
                  :key="f"
                  size="small"
                  style="margin-right: 4px; margin-bottom: 2px"
                >{{ featureLabel(f) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="创建时间" width="160">
              <template #default="{ row }">{{ formatTime(row.timeCreated) }}</template>
            </el-table-column>
            <el-table-column label="完成时间" width="160">
              <template #default="{ row }">{{ formatTime(row.timeUpdated) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="100" fixed="right" align="center">
              <template #default="{ row }">
                <el-popconfirm
                  v-if="isJobCancellable(row.lifecycleState)"
                  title="确定取消此任务？"
                  @confirm="cancelJob(row)"
                >
                  <template #reference>
                    <el-button size="small" type="danger" link>
                      <el-icon><Close /></el-icon> 取消
                    </el-button>
                  </template>
                </el-popconfirm>
                <el-button size="small" type="primary" link @click="viewJobResult(row)" v-if="row.lifecycleState === 'SUCCEEDED'">
                  <el-icon><View /></el-icon> 结果
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Plus, VideoPlay, Operation, Upload, Close, View } from '@element-plus/icons-vue'
import request from '../utils/request'

// ---- Types ----
interface ImageForm {
  features: string[]
  language: string
  imageFile: File | null
}

interface DocForm {
  features: string[]
  docFile: File | null
}

interface VisionJob {
  jobId: string
  jobType: string
  lifecycleState: string
  features: string[]
  timeCreated: string
  timeUpdated: string
  outputLocation?: any
}

// ---- State ----
const activeTab = ref('image')

// Image analysis
const imageForm = ref<ImageForm>({
  features: ['IMAGE_CLASSIFICATION', 'OBJECT_DETECTION'],
  language: '',
  imageFile: null,
})
const imageAnalyzing = ref(false)
const imageJobCreating = ref(false)
const imageResult = ref<any>(null)
const imageUploadRef = ref<any>(null)

// Document analysis
const docForm = ref<DocForm>({
  features: ['TEXT_EXTRACTION'],
  docFile: null,
})
const docAnalyzing = ref(false)
const docJobCreating = ref(false)
const docResult = ref<any>(null)
const docUploadRef = ref<any>(null)

// Job history
const jobs = ref<VisionJob[]>([])
const jobsLoading = ref(false)

// ---- Computed ----
const keyValuePairs = computed(() => {
  if (!docResult.value?.keyValueExtraction) return []
  const kv = docResult.value.keyValueExtraction
  if (Array.isArray(kv.fields)) return kv.fields
  if (Array.isArray(kv.keyValuePairs)) return kv.keyValuePairs
  if (typeof kv === 'object') {
    return Object.entries(kv).map(([key, value]: [string, any]) => ({
      key,
      value: typeof value === 'object' ? value.value || JSON.stringify(value) : String(value),
      confidence: value.confidence,
    }))
  }
  return []
})

// ---- Helpers ----
function formatTime(t: string | undefined): string {
  if (!t) return '-'
  try { return new Date(t).toLocaleString('zh-CN') } catch { return t }
}

function formatBoundingPolygon(polygon: any): string {
  if (!polygon) return '-'
  if (Array.isArray(polygon.normalizedVertices)) {
    return polygon.normalizedVertices
      .map((v: any) => `(${(v.x || 0).toFixed(2)}, ${(v.y || 0).toFixed(2)})`)
      .join(' ')
  }
  if (Array.isArray(polygon.vertices)) {
    return polygon.vertices
      .map((v: any) => `(${v.x || 0}, ${v.y || 0})`)
      .join(' ')
  }
  return JSON.stringify(polygon)
}

function jobStateDot(state: string): string {
  const s = (state || '').toLowerCase()
  if (s === 'succeeded') return 'status-dot--up status-dot--pulse'
  if (s === 'in_progress' || s === 'accepted') return 'status-dot--warn'
  if (s === 'failed' || s === 'cancelled') return 'status-dot--down'
  return 'status-dot--idle'
}

function jobStateLabel(state: string): string {
  const map: Record<string, string> = {
    ACCEPTED: '已接受',
    IN_PROGRESS: '进行中',
    SUCCEEDED: '已完成',
    FAILED: '失败',
    CANCELLED: '已取消',
  }
  return map[state] || state || '-'
}

function featureLabel(f: string): string {
  const map: Record<string, string> = {
    IMAGE_CLASSIFICATION: '图像分类',
    OBJECT_DETECTION: '目标检测',
    FACE_DETECTION: '人脸检测',
    TEXT_DETECTION: '文字检测',
    TEXT_EXTRACTION: '文字提取',
    TABLE_EXTRACTION: '表格提取',
    KEY_VALUE_EXTRACTION: '键值对提取',
    LANGUAGE_DETECTION: '语言检测',
  }
  return map[f] || f
}

function isJobCancellable(state: string): boolean {
  const s = (state || '').toUpperCase()
  return s === 'ACCEPTED' || s === 'IN_PROGRESS'
}

// ---- File Handling ----
function onImageChange(file: any) {
  imageForm.value.imageFile = file?.raw || null
}

function onImageRemove() {
  imageForm.value.imageFile = null
}

function onDocChange(file: any) {
  docForm.value.docFile = file?.raw || null
}

function onDocRemove() {
  docForm.value.docFile = null
}

// ---- Image Analysis ----
async function analyzeImage() {
  if (!imageForm.value.imageFile) {
    ElMessage.warning('请先上传图片')
    return
  }
  if (imageForm.value.features.length === 0) {
    ElMessage.warning('请至少选择一个分析特征')
    return
  }
  imageAnalyzing.value = true
  imageResult.value = null
  try {
    const fd = new FormData()
    fd.append('image', imageForm.value.imageFile)
    fd.append('features', JSON.stringify(imageForm.value.features))
    if (imageForm.value.language) fd.append('language', imageForm.value.language)

    const res = await request.post('/oci/vision/image/analyze', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    imageResult.value = res
    ElMessage.success('图像分析完成')
  } catch (e: any) {
    ElMessage.error(e.message || '图像分析失败')
  } finally {
    imageAnalyzing.value = false
  }
}

async function createImageJob() {
  if (!imageForm.value.imageFile) {
    ElMessage.warning('请先上传图片')
    return
  }
  if (imageForm.value.features.length === 0) {
    ElMessage.warning('请至少选择一个分析特征')
    return
  }
  imageJobCreating.value = true
  try {
    const fd = new FormData()
    fd.append('image', imageForm.value.imageFile)
    fd.append('features', JSON.stringify(imageForm.value.features))
    if (imageForm.value.language) fd.append('language', imageForm.value.language)

    await request.post('/oci/vision/image/job', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    ElMessage.success('异步任务已创建，请在任务历史中查看进度')
    await loadJobs()
  } catch (e: any) {
    ElMessage.error(e.message || '创建任务失败')
  } finally {
    imageJobCreating.value = false
  }
}

// ---- Document Analysis ----
async function analyzeDocument() {
  if (!docForm.value.docFile) {
    ElMessage.warning('请先上传文档')
    return
  }
  if (docForm.value.features.length === 0) {
    ElMessage.warning('请至少选择一个分析特征')
    return
  }
  docAnalyzing.value = true
  docResult.value = null
  try {
    const fd = new FormData()
    fd.append('document', docForm.value.docFile)
    fd.append('features', JSON.stringify(docForm.value.features))

    const res = await request.post('/oci/vision/document/analyze', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    docResult.value = res
    ElMessage.success('文档分析完成')
  } catch (e: any) {
    ElMessage.error(e.message || '文档分析失败')
  } finally {
    docAnalyzing.value = false
  }
}

async function createDocJob() {
  if (!docForm.value.docFile) {
    ElMessage.warning('请先上传文档')
    return
  }
  if (docForm.value.features.length === 0) {
    ElMessage.warning('请至少选择一个分析特征')
    return
  }
  docJobCreating.value = true
  try {
    const fd = new FormData()
    fd.append('document', docForm.value.docFile)
    fd.append('features', JSON.stringify(docForm.value.features))

    await request.post('/oci/vision/document/job', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    ElMessage.success('异步任务已创建，请在任务历史中查看进度')
    await loadJobs()
  } catch (e: any) {
    ElMessage.error(e.message || '创建任务失败')
  } finally {
    docJobCreating.value = false
  }
}

// ---- Job Management ----
async function loadJobs() {
  jobsLoading.value = true
  try {
    // The API might return jobs from a list endpoint or we use the history
    const res = await request.get('/oci/vision/jobs') as any
    jobs.value = Array.isArray(res) ? res : (res?.items || [])
  } catch (e: any) {
    // If the endpoint doesn't exist yet, silently fail
    jobs.value = []
  } finally {
    jobsLoading.value = false
  }
}

async function cancelJob(job: VisionJob) {
  try {
    await request.delete('/oci/vision/job/cancel', { data: { jobId: job.jobId } })
    ElMessage.success('任务已取消')
    await loadJobs()
  } catch (e: any) {
    ElMessage.error(e.message || '取消任务失败')
  }
}

async function viewJobResult(job: VisionJob) {
  try {
    const res = await request.get('/oci/vision/job/get', { params: { jobId: job.jobId } }) as any
    if (job.jobType === 'IMAGE') {
      imageResult.value = res?.output || res
      activeTab.value = 'image'
    } else {
      docResult.value = res?.output || res
      activeTab.value = 'document'
    }
    ElMessage.success('已加载任务结果')
  } catch (e: any) {
    ElMessage.error(e.message || '获取任务结果失败')
  }
}

onMounted(loadJobs)
</script>

<style scoped>
.vision-page {
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

.vision-tabs {
  margin-top: var(--space-2);
}

.form-card {
  border-radius: var(--radius-md);
  margin-bottom: var(--space-4);
}

.result-card {
  border-radius: var(--radius-md);
  margin-top: var(--space-4);
}

.table-card {
  border-radius: var(--radius-md);
  overflow: hidden;
}

.table-card :deep(.el-card__body) {
  padding: 0;
}

.result-section {
  margin-bottom: var(--space-5);
  padding-bottom: var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
}

.result-section:last-child {
  margin-bottom: 0;
  padding-bottom: 0;
  border-bottom: none;
}

.result-section h4 {
  margin: 0 0 var(--space-3) 0;
  font-size: var(--text-base);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

.detection-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: var(--space-3);
}

.detection-item {
  padding: var(--space-3);
  background: var(--bg-raised);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
}

.detection-label {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin-bottom: var(--space-2);
}

.confidence {
  font-size: var(--text-xs);
  color: var(--text-secondary);
  font-weight: var(--font-medium);
}

.bounding-box {
  font-size: var(--text-xs);
  color: var(--text-muted);
}

.text-output {
  padding: var(--space-4);
  background: var(--bg-root);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  max-height: 400px;
  overflow-y: auto;
}

.text-output pre {
  margin: 0;
  font-family: 'Courier New', Courier, monospace;
  font-size: var(--text-sm);
  color: var(--text-primary);
  white-space: pre-wrap;
  word-break: break-all;
}

.table-block {
  margin-bottom: var(--space-4);
}

.table-block h5 {
  margin: 0 0 var(--space-2) 0;
  font-size: var(--text-sm);
  font-weight: var(--font-semibold);
  color: var(--text-secondary);
}

.state-text {
  font-size: var(--text-xs);
  color: var(--text-secondary);
  font-weight: var(--font-medium);
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

:deep(.el-tabs__item) {
  font-weight: var(--font-medium);
}

:deep(.el-upload--picture-card) {
  width: 120px;
  height: 120px;
}

:deep(.el-upload-list--picture-card .el-upload-list__item) {
  width: 120px;
  height: 120px;
}

@media (max-width: 768px) {
  .toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .toolbar-left h2 {
    font-size: var(--text-lg);
  }

  .detection-grid {
    grid-template-columns: 1fr;
  }
}
</style>
