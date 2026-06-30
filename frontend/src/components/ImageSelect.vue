<template>
  <div class="image-select-wrapper">
    <el-select
      :model-value="modelValue"
      :placeholder="placeholder"
      filterable
      clearable
      :loading="loading"
      :disabled="disabled"
      style="width: 100%"
      @update:model-value="$emit('update:modelValue', $event)"
    >
      <template #prefix>
        <el-icon @click.stop="refresh" style="cursor:pointer" title="刷新镜像列表">
          <Refresh :class="{ 'spin': loading }" />
        </el-icon>
      </template>
      <el-option-group
        v-for="(imgs, os) in groupedImages"
        :key="os"
        :label="os"
      >
        <el-option
          v-for="img in imgs"
          :key="img.id"
          :label="formatImageLabel(img)"
          :value="img.id"
        >
          <div class="image-option">
            <div class="image-option-main">
              <span class="image-name">{{ img.displayName }}</span>
              <el-tag
                :type="img.architecture === 'ARM' ? 'success' : 'warning'"
                size="small"
                effect="dark"
                class="arch-tag"
              >
                {{ img.architecture }}
              </el-tag>
            </div>
            <div class="image-option-detail">
              {{ img.operatingSystem }} {{ img.operatingSystemVersion }}
              <span v-if="img.sizeInGBs" class="size-hint">{{ img.sizeInGBs }}GB</span>
            </div>
          </div>
        </el-option>
      </el-option-group>
      <template #empty>
        <div v-if="loading" style="padding:12px;text-align:center;color:var(--text-muted)">
          加载中...
        </div>
        <div v-else style="padding:12px;text-align:center;color:var(--text-muted)">
          暂无可用镜像
        </div>
      </template>
    </el-select>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import request from '../utils/request'
import type { ImageInfo } from '../types/api'

const props = withDefaults(defineProps<{
  modelValue?: string
  tenantId?: number | null
  architecture?: string
  shape?: string
  placeholder?: string
  disabled?: boolean
  autoLoad?: boolean
}>(), {
  placeholder: '选择镜像',
  autoLoad: true,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  'change': [image: ImageInfo | undefined]
}>()

const loading = ref(false)
const images = ref<ImageInfo[]>([])

const groupedImages = computed(() => {
  const groups: Record<string, ImageInfo[]> = {}
  for (const img of filteredImages.value) {
    const os = img.operatingSystem || 'Other'
    if (!groups[os]) groups[os] = []
    groups[os].push(img)
  }
  return groups
})

const filteredImages = computed(() => {
  if (!props.architecture) return images.value
  return images.value.filter(img =>
    img.architecture?.toUpperCase() === props.architecture?.toUpperCase()
  )
})

function formatImageLabel(img: ImageInfo): string {
  return `${img.displayName} (${img.operatingSystem} ${img.operatingSystemVersion})`
}

async function refresh() {
  if (!props.tenantId) return
  loading.value = true
  try {
    const params: any = { tenantId: props.tenantId }
    if (props.architecture) params.architecture = props.architecture
    if (props.shape) params.shape = props.shape
    images.value = await request.get('/oci/images', { params }) as ImageInfo[]
  } catch (e: any) {
    ElMessage.error('加载镜像列表失败: ' + e.message)
    images.value = []
  } finally {
    loading.value = false
  }
}

watch(() => [props.tenantId, props.architecture, props.shape], () => {
  if (props.tenantId && props.autoLoad) {
    refresh()
  } else {
    images.value = []
  }
})

watch(() => props.modelValue, (val) => {
  const found = images.value.find(img => img.id === val)
  emit('change', found)
})

onMounted(() => {
  if (props.tenantId && props.autoLoad) {
    refresh()
  }
})

defineExpose({ refresh })
</script>

<style scoped>
.image-select-wrapper {
  width: 100%;
}

.image-option {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 2px 0;
}

.image-option-main {
  display: flex;
  align-items: center;
  gap: 8px;
}

.image-name {
  font-weight: var(--font-medium);
  font-size: var(--text-sm);
}

.arch-tag {
  font-size: 10px;
  padding: 0 4px;
  height: 18px;
  line-height: 18px;
}

.image-option-detail {
  font-size: var(--text-xs);
  color: var(--text-muted);
}

.size-hint {
  margin-left: 8px;
  color: var(--text-secondary);
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

:deep(.el-select) {
  width: 100%;
}
</style>
