<template>
  <div class="shape-select-wrapper">
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
        <el-icon @click.stop="refresh" style="cursor:pointer" title="刷新 Shape 列表">
          <Refresh :class="{ 'spin': loading }" />
        </el-icon>
      </template>
      <el-option-group label="ARM 架构">
        <el-option
          v-for="s in armShapes"
          :key="s.shape"
          :label="formatShapeLabel(s)"
          :value="s.shape"
        >
          <div class="shape-option">
            <div class="shape-option-main">
              <span class="shape-name">{{ s.shape }}</span>
              <el-tag type="success" size="small" effect="dark" class="arch-tag">ARM</el-tag>
            </div>
            <div class="shape-option-detail">
              {{ s.ocpus }}C / {{ s.memoryInGBs }}G
              <span v-if="s.isFlexible" class="flex-hint">(Flex)</span>
              <span v-if="s.networkingDescription" class="net-hint">{{ s.networkingDescription }}</span>
            </div>
          </div>
        </el-option>
      </el-option-group>
      <el-option-group label="AMD 架构">
        <el-option
          v-for="s in amdShapes"
          :key="s.shape"
          :label="formatShapeLabel(s)"
          :value="s.shape"
        >
          <div class="shape-option">
            <div class="shape-option-main">
              <span class="shape-name">{{ s.shape }}</span>
              <el-tag type="warning" size="small" effect="dark" class="arch-tag">AMD</el-tag>
            </div>
            <div class="shape-option-detail">
              {{ s.ocpus }}C / {{ s.memoryInGBs }}G
              <span v-if="s.isFlexible" class="flex-hint">(Flex)</span>
              <span v-if="s.networkingDescription" class="net-hint">{{ s.networkingDescription }}</span>
            </div>
          </div>
        </el-option>
      </el-option-group>
      <template #empty>
        <div v-if="loading" style="padding:12px;text-align:center;color:var(--text-muted)">
          加载中...
        </div>
        <div v-else style="padding:12px;text-align:center;color:var(--text-muted)">
          暂无可用 Shape
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
import type { ShapeInfo } from '../types/api'

const props = withDefaults(defineProps<{
  modelValue?: string
  tenantId?: number | null
  architecture?: string
  placeholder?: string
  disabled?: boolean
  autoLoad?: boolean
}>(), {
  placeholder: '选择 Shape',
  autoLoad: true,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  'change': [shape: ShapeInfo | undefined]
}>()

const loading = ref(false)
const shapes = ref<ShapeInfo[]>([])

const armShapes = computed(() =>
  shapes.value.filter(s => s.architecture === 'ARM')
)

const amdShapes = computed(() =>
  shapes.value.filter(s => s.architecture === 'AMD')
)

function formatShapeLabel(s: ShapeInfo): string {
  return `${s.shape} (${s.ocpus}C/${s.memoryInGBs}G)`
}

async function refresh() {
  if (!props.tenantId) return
  loading.value = true
  try {
    const params: any = { tenantId: props.tenantId }
    if (props.architecture) params.architecture = props.architecture
    shapes.value = await request.get('/oci/shapes', { params }) as ShapeInfo[]
  } catch (e: any) {
    ElMessage.error('加载 Shape 列表失败: ' + e.message)
    shapes.value = []
  } finally {
    loading.value = false
  }
}

watch(() => props.tenantId, (id) => {
  if (id && props.autoLoad) {
    refresh()
  } else {
    shapes.value = []
  }
})

watch(() => props.modelValue, (val) => {
  const found = shapes.value.find(s => s.shape === val)
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
.shape-select-wrapper {
  width: 100%;
}

.shape-option {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 2px 0;
}

.shape-option-main {
  display: flex;
  align-items: center;
  gap: 8px;
}

.shape-name {
  font-weight: var(--font-medium);
  font-size: var(--text-sm);
}

.arch-tag {
  font-size: 10px;
  padding: 0 4px;
  height: 18px;
  line-height: 18px;
}

.shape-option-detail {
  font-size: var(--text-xs);
  color: var(--text-muted);
}

.flex-hint {
  color: var(--accent);
  margin-left: 4px;
}

.net-hint {
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
