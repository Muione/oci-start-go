# Frontend Enhancement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enhance existing InstanceDetail, VnicManagement pages with disk management, IPv6, security rules, VCN, NAT/route table panels; fix Dashboard/SystemSettings bugs.

**Architecture:** Extract focused sub-components (`*Panel.vue`, `*Modal.vue`) from existing pages, organize via Element Plus Tabs. Each panel owns its data fetching and state. Modals use `el-dialog` with `destroy-on-close`.

**Tech Stack:** Vue 3 Composition API, Element Plus, Axios, TypeScript

## Global Constraints

- All API calls use `import request from '../utils/request'` — never raw axios
- All types defined in `frontend/src/types/api.ts`
- Modals: `el-dialog` + `v-model` + `destroy-on-close`
- Destructive confirmations: `ElMessageBox.confirm()`
- Feedback: `ElMessage.success/error/warning`
- Every async action has a dedicated `loading` ref
- CSS uses scoped styles with design tokens (`var(--text-sm)`, `var(--space-4)`, etc.)
- Component naming: `defineOptions({ name: 'ComponentName' })`

---

### Task 1: API Type Definitions

**Files:**
- Modify: `frontend/src/types/api.ts`

**Interfaces:**
- Produces: All types consumed by Tasks 3-10

- [ ] **Step 1: Add types to api.ts**

Append to `frontend/src/types/api.ts`:

```typescript
// ─── Traffic Monitoring ───────────────────────────────────────────────
export interface TrafficData {
  ingressToday: number    // GB
  egressToday: number     // GB
  ingressMonth: number    // GB
  egressMonth: number     // GB
}

// ─── Boot Volume ──────────────────────────────────────────────────────
export interface BootVolumeDetail {
  id: string
  displayName: string
  sizeInGBs: number
  vpusPerGB: number
  lifecycleState: string
  timeCreated: string
}

export interface BootVolumeBackup {
  id: string
  displayName: string
  bootVolumeId: string
  sizeInGBs: number
  lifecycleState: string
  timeCreated: string
}

export interface VpuLevel {
  value: number
  label: string
  description: string
}

// ─── Security Rules ───────────────────────────────────────────────────
// SecurityRule already exists in api.ts — skip if present

// ─── VCN ──────────────────────────────────────────────────────────────
export interface VcnInfo {
  id: string
  displayName: string
  cidrBlock: string
  dnsLabel: string
  defaultSecurityListId: string
  defaultRouteTableId: string
  timeCreated: string
}

// ─── NAT Gateway ──────────────────────────────────────────────────────
export interface NatGatewayInfo {
  id: string
  displayName: string
  vcnId: string
  lifecycleState: string
}

// ─── Route Table ──────────────────────────────────────────────────────
export interface RouteTableInfo {
  id: string
  displayName: string
  vcnId: string
  routeRules: RouteRuleInfo[]
}

export interface RouteRuleInfo {
  destination: string
  destinationType: string
  networkEntityId: string
}

// ─── IPv6 ─────────────────────────────────────────────────────────────
export interface Ipv6Info {
  id: string
  ipAddress: string
  vnicId: string
}
```

- [ ] **Step 2: Verify types compile**

Run: `cd frontend && npx vue-tsc --noEmit 2>&1 | head -20`
Expected: No errors related to new types

- [ ] **Step 3: Commit**

```bash
git add frontend/src/types/api.ts
git commit -m "feat: add API types for traffic, disk, VCN, NAT, IPv6"
```

---

### Task 2: Bug Fixes

**Files:**
- Modify: `frontend/src/views/Dashboard.vue`
- Modify: `frontend/src/views/SystemSettings.vue`

- [ ] **Step 1: Fix Dashboard backup count**

In `Dashboard.vue`, find the statistics cards section. Add a card for `backupCount` if not displayed. Locate where `stats` is rendered and add:

```html
<el-col :span="6">
  <el-card shadow="hover">
    <el-statistic title="备份数量" :value="stats.backupCount ?? 0">
      <template #prefix><el-icon><CopyDocument /></el-icon></template>
    </el-statistic>
  </el-card>
</el-col>
```

Ensure `CopyDocument` is imported from `@element-plus/icons-vue`.

- [ ] **Step 2: Fix notification history parsing**

In `SystemSettings.vue`, find where notification history is loaded. The response interceptor already unwraps `data`, so if the backend returns `{ history: [...] }`, the code should access `res.history` not `res` directly.

Find the load function and fix:

```typescript
// Before (broken):
const data = await request.get('/api/system/notification/history') as NotificationHistory[]
notificationHistory.value = data

// After (fixed):
const res = await request.get('/api/system/notification/history') as { history: NotificationHistory[] }
notificationHistory.value = res.history ?? []
```

- [ ] **Step 3: Add channel filter to notification history**

Add a channel dropdown above the notification history table:

```html
<el-select v-model="notifChannelFilter" placeholder="全部频道" clearable @change="loadNotificationHistory" style="width: 160px; margin-right: 12px;">
  <el-option label="全部" value="" />
  <el-option label="Email" value="email" />
  <el-option label="Webhook" value="webhook" />
</el-select>
```

Add ref and update load function:

```typescript
const notifChannelFilter = ref('')

async function loadNotificationHistory() {
  try {
    const params: any = {}
    if (notifChannelFilter.value) params.channel = notifChannelFilter.value
    const res = await request.get('/api/system/notification/history', { params }) as { history: NotificationHistory[] }
    notificationHistory.value = res.history ?? []
  } catch (e: any) {
    ElMessage.error(e.message || '加载通知历史失败')
  }
}
```

- [ ] **Step 4: Test bug fixes**

Run: `cd frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/Dashboard.vue frontend/src/views/SystemSettings.vue
git commit -m "fix: display backup count on dashboard, fix notification history parsing and add channel filter"
```

---

### Task 3: InstanceDetail Tab Framework

**Files:**
- Modify: `frontend/src/views/InstanceDetail.vue`

**Interfaces:**
- Produces: Tab structure consumed by Tasks 4, 5

- [ ] **Step 1: Add Tab structure to InstanceDetail**

Wrap existing content in `el-tabs`. The existing instance info becomes the first tab.

```html
<template>
  <div class="instance-detail-page">
    <!-- Header with instance name and status -->
    <div class="page-header">
      <h2>{{ instance?.displayName }}</h2>
      <el-tag :type="stateTagType(instance?.state)">{{ instance?.state }}</el-tag>
    </div>

    <el-tabs v-model="activeTab" type="border-card">
      <el-tab-pane label="概览" name="overview">
        <!-- Move existing instance detail content here -->
      </el-tab-pane>

      <el-tab-pane label="流量监控" name="traffic">
        <InstanceTrafficPanel :instance-id="instanceId" />
      </el-tab-pane>

      <el-tab-pane label="磁盘管理" name="disk">
        <DiskManagementPanel :instance-id="instanceId" :boot-volume-id="instance?.bootVolumeId" />
      </el-tab-pane>

      <el-tab-pane label="控制台连接" name="console">
        <!-- Move existing console connection content here -->
      </el-tab-pane>
    </el-tabs>
  </div>
</template>
```

Add ref and imports:

```typescript
const activeTab = ref('overview')

// Import sub-components (will be created in later tasks)
// import InstanceTrafficPanel from '../components/instance/InstanceTrafficPanel.vue'
// import DiskManagementPanel from '../components/instance/DiskManagementPanel.vue'
```

- [ ] **Step 2: Verify page renders**

Run: `cd frontend && npm run dev`
Expected: InstanceDetail page shows tabs, existing content in "概览" tab

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/InstanceDetail.vue
git commit -m "refactor: add tab structure to InstanceDetail page"
```

---

### Task 4: InstanceTrafficPanel

**Files:**
- Create: `frontend/src/components/instance/InstanceTrafficPanel.vue`

**Interfaces:**
- Consumes: `TrafficData` from Task 1, `instanceId` prop
- Produces: None (leaf component)

- [ ] **Step 1: Create InstanceTrafficPanel.vue**

```vue
<script setup lang="ts">
defineOptions({ name: 'InstanceTrafficPanel' })

import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'
import type { TrafficData } from '../../types/api'

const props = defineProps<{ instanceId: string }>()

const loading = ref(false)
const traffic = ref<TrafficData | null>(null)

async function loadTraffic() {
  loading.value = true
  try {
    const res = await request.get('/api/instances/traffic', { params: { instanceId: props.instanceId } }) as TrafficData
    traffic.value = res
  } catch (e: any) {
    ElMessage.error(e.message || '加载流量数据失败')
  } finally {
    loading.value = false
  }
}

onMounted(loadTraffic)
</script>

<template>
  <div class="traffic-panel" v-loading="loading">
    <el-row :gutter="16">
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header>今日入站流量</template>
          <el-statistic :value="traffic?.ingressToday ?? 0" :precision="2" suffix="GB" />
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header>今日出站流量</template>
          <el-statistic :value="traffic?.egressToday ?? 0" :precision="2" suffix="GB" />
        </el-card>
      </el-col>
    </el-row>
    <el-row :gutter="16" style="margin-top: 16px;">
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header>本月入站流量</template>
          <el-statistic :value="traffic?.ingressMonth ?? 0" :precision="2" suffix="GB" />
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header>本月出站流量</template>
          <el-statistic :value="traffic?.egressMonth ?? 0" :precision="2" suffix="GB" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.traffic-panel {
  padding: var(--space-4);
}
</style>
```

- [ ] **Step 2: Register component in InstanceDetail**

In `InstanceDetail.vue`, uncomment the import:

```typescript
import InstanceTrafficPanel from '../components/instance/InstanceTrafficPanel.vue'
```

- [ ] **Step 3: Verify**

Run: `cd frontend && npm run dev`
Expected: Traffic tab shows 4 stat cards

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/instance/InstanceTrafficPanel.vue frontend/src/views/InstanceDetail.vue
git commit -m "feat: add InstanceTrafficPanel with daily/monthly traffic stats"
```

---

### Task 5: DiskManagementPanel + Modals

**Files:**
- Create: `frontend/src/components/instance/DiskManagementPanel.vue`
- Create: `frontend/src/components/instance/SelectVpuModal.vue`
- Create: `frontend/src/components/instance/ResizeVolumeModal.vue`
- Create: `frontend/src/components/instance/CreateBackupModal.vue`
- Create: `frontend/src/components/instance/CopyBackupModal.vue`

**Interfaces:**
- Consumes: `BootVolumeDetail`, `BootVolumeBackup`, `VpuLevel` from Task 1
- Props: `instanceId`, `bootVolumeId`

- [ ] **Step 1: Create SelectVpuModal.vue**

```vue
<script setup lang="ts">
defineOptions({ name: 'SelectVpuModal' })

import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'

const props = defineProps<{ modelValue: boolean; bootVolumeId: string; currentVpu: number }>()
const emit = defineEmits<{ 'update:modelValue': [val: boolean]; saved: [] }>()

const saving = ref(false)
const selectedVpu = ref(props.currentVpu)

const vpuOptions = [
  { value: 10, label: 'Balanced (10 VPUs/GB)', desc: '标准性能' },
  { value: 20, label: 'Higher Performance (20 VPUs/GB)', desc: '更高性能' },
  { value: 30, label: 'Ultra High (30 VPUs/GB)', desc: '超高性能' },
  { value: 60, label: 'Ultra High (60 VPUs/GB)', desc: '超高性能' },
  { value: 120, label: 'Ultra High (120 VPUs/GB)', desc: '最高性能' },
]

watch(() => props.modelValue, (val) => {
  if (val) selectedVpu.value = props.currentVpu
})

async function handleSave() {
  saving.value = true
  try {
    await request.post('/api/boot-volume/vpu', { bootVolumeId: props.bootVolumeId, vpusPerGB: selectedVpu.value })
    ElMessage.success('VPU 调整成功')
    emit('saved')
    emit('update:modelValue', false)
  } catch (e: any) {
    ElMessage.error(e.message || 'VPU 调整失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="调整启动卷性能级别" width="480px" destroy-on-close>
    <el-alert title="调整 VPU 需要实例处于停止状态" type="warning" :closable="false" show-icon style="margin-bottom: 16px" />
    <el-radio-group v-model="selectedVpu" style="display: flex; flex-direction: column; gap: 12px;">
      <el-radio v-for="opt in vpuOptions" :key="opt.value" :value="opt.value">
        {{ opt.label }}
        <span style="color: var(--text-secondary); margin-left: 8px;">{{ opt.desc }}</span>
      </el-radio>
    </el-radio-group>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="handleSave">确认</el-button>
    </template>
  </el-dialog>
</template>
```

- [ ] **Step 2: Create ResizeVolumeModal.vue**

```vue
<script setup lang="ts">
defineOptions({ name: 'ResizeVolumeModal' })

import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'

const props = defineProps<{ modelValue: boolean; bootVolumeId: string; currentSizeGB: number }>()
const emit = defineEmits<{ 'update:modelValue': [val: boolean]; saved: [] }>()

const saving = ref(false)
const newSize = ref(props.currentSizeGB)

watch(() => props.modelValue, (val) => {
  if (val) newSize.value = props.currentSizeGB
})

async function handleSave() {
  if (newSize.value < props.currentSizeGB) {
    ElMessage.warning('磁盘只能增大不能缩小')
    return
  }
  saving.value = true
  try {
    await request.post('/api/boot-volume/resize', { bootVolumeId: props.bootVolumeId, sizeInGBs: newSize.value })
    ElMessage.success('磁盘大小调整成功')
    emit('saved')
    emit('update:modelValue', false)
  } catch (e: any) {
    ElMessage.error(e.message || '磁盘调整失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="调整启动卷大小" width="420px" destroy-on-close>
    <el-form label-width="100px">
      <el-form-item label="当前大小">
        <span>{{ currentSizeGB }} GB</span>
      </el-form-item>
      <el-form-item label="新大小">
        <el-input-number v-model="newSize" :min="currentSizeGB" :step="10" />
        <span style="margin-left: 8px;">GB</span>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="handleSave">确认</el-button>
    </template>
  </el-dialog>
</template>
```

- [ ] **Step 3: Create CreateBackupModal.vue**

```vue
<script setup lang="ts">
defineOptions({ name: 'CreateBackupModal' })

import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'

const props = defineProps<{ modelValue: boolean; bootVolumeId: string }>()
const emit = defineEmits<{ 'update:modelValue': [val: boolean]; saved: [] }>()

const saving = ref(false)
const backupName = ref('')

async function handleCreate() {
  if (!backupName.value.trim()) {
    ElMessage.warning('请输入备份名称')
    return
  }
  saving.value = true
  try {
    await request.post('/api/backup/create', { bootVolumeId: props.bootVolumeId, displayName: backupName.value.trim() })
    ElMessage.success('备份创建成功')
    emit('saved')
    emit('update:modelValue', false)
  } catch (e: any) {
    ElMessage.error(e.message || '备份创建失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="创建启动卷备份" width="420px" destroy-on-close>
    <el-form label-width="100px">
      <el-form-item label="备份名称">
        <el-input v-model="backupName" placeholder="输入备份名称" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="handleCreate">创建</el-button>
    </template>
  </el-dialog>
</template>
```

- [ ] **Step 4: Create CopyBackupModal.vue**

```vue
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
```

- [ ] **Step 5: Create DiskManagementPanel.vue**

```vue
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

const props = defineProps<{ instanceId: string; bootVolumeId?: string }>()

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
  if (!props.bootVolumeId) return
  loading.value = true
  try {
    const res = await request.get('/api/boot-volume/detail', { params: { bootVolumeId: props.bootVolumeId } }) as BootVolumeDetail
    bootVolume.value = res
  } catch (e: any) {
    ElMessage.error(e.message || '加载启动卷信息失败')
  } finally {
    loading.value = false
  }
}

async function loadBackups() {
  if (!props.bootVolumeId) return
  try {
    const res = await request.get('/api/backup/list', { params: { bootVolumeId: props.bootVolumeId } }) as BootVolumeBackup[]
    backups.value = res ?? []
  } catch (e: any) {
    ElMessage.error(e.message || '加载备份列表失败')
  }
}

async function deleteBackup(backup: BootVolumeBackup) {
  try {
    await ElMessageBox.confirm(`确定删除备份 "${backup.displayName}"？`, '删除备份', { type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger' })
    await request.get('/api/backup/delete', { params: { id: backup.id } })
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
    <SelectVpuModal v-model="vpuModalVisible" :boot-volume-id="bootVolumeId ?? ''" :current-vpu="bootVolume?.vpusPerGB ?? 10" @saved="onSaved" />
    <ResizeVolumeModal v-model="resizeModalVisible" :boot-volume-id="bootVolumeId ?? ''" :current-size-g-b="bootVolume?.sizeInGBs ?? 0" @saved="onSaved" />
    <CreateBackupModal v-model="createBackupVisible" :boot-volume-id="bootVolumeId ?? ''" @saved="onSaved" />
    <CopyBackupModal v-model="copyBackupVisible" :backup-id="copyBackupId" @saved="onSaved" />
  </div>
</template>

<style scoped>
.disk-panel {
  padding: var(--space-4);
}
</style>
```

- [ ] **Step 6: Register components in InstanceDetail**

In `InstanceDetail.vue`, add imports:

```typescript
import DiskManagementPanel from '../components/instance/DiskManagementPanel.vue'
```

- [ ] **Step 7: Verify**

Run: `cd frontend && npm run dev`
Expected: Disk Management tab shows boot volume info, actions, and backup table

- [ ] **Step 8: Commit**

```bash
git add frontend/src/components/instance/
git add frontend/src/views/InstanceDetail.vue
git commit -m "feat: add DiskManagementPanel with VPU, resize, backup CRUD"
```

---

### Task 6: VnicManagement Tab Framework + IPv6 Row

**Files:**
- Modify: `frontend/src/views/VnicManagement.vue`
- Create: `frontend/src/components/vnic/VnicIpv6Row.vue`
- Create: `frontend/src/components/vnic/BatchCreateModal.vue`

**Interfaces:**
- Consumes: `VnicInfo`, `Ipv6Info` from Task 1
- Produces: Tab structure consumed by Tasks 7-9

- [ ] **Step 1: Create VnicIpv6Row.vue**

```vue
<script setup lang="ts">
defineOptions({ name: 'VnicIpv6Row' })

import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../../utils/request'
import type { Ipv6Info } from '../../types/api'

const props = defineProps<{ vnicId: string; ipv6Addresses: string[] }>()
const emit = defineEmits<{ refresh: [] }>()

const assigning = ref(false)

async function assignIpv6() {
  assigning.value = true
  try {
    await request.post('/api/vnic/ipv6/assign', { vnicId: props.vnicId })
    ElMessage.success('IPv6 地址分配成功')
    emit('refresh')
  } catch (e: any) {
    ElMessage.error(e.message || 'IPv6 分配失败')
  } finally {
    assigning.value = false
  }
}

async function deleteAllIpv6() {
  try {
    await ElMessageBox.confirm('确定删除该 VNIC 的所有 IPv6 地址？', '删除 IPv6', { type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger' })
    await request.post('/api/vnic/ipv6/delete', { vnicId: props.vnicId })
    ElMessage.success('IPv6 地址已删除')
    emit('refresh')
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '删除失败')
  }
}
</script>

<template>
  <div class="ipv6-row">
    <div v-if="ipv6Addresses.length > 0" style="margin-bottom: 8px;">
      <div v-for="addr in ipv6Addresses" :key="addr" style="display: flex; align-items: center; gap: 8px; margin-bottom: 4px;">
        <el-tag type="info" size="small">{{ addr }}</el-tag>
      </div>
    </div>
    <el-empty v-else description="无 IPv6 地址" :image-size="40" />
    <el-space style="margin-top: 8px;">
      <el-button size="small" type="primary" :loading="assigning" @click="assignIpv6">分配 IPv6</el-button>
      <el-button v-if="ipv6Addresses.length > 0" size="small" type="danger" @click="deleteAllIpv6">删除全部 IPv6</el-button>
    </el-space>
  </div>
</template>

<style scoped>
.ipv6-row {
  padding: 12px 16px;
  background: var(--bg-color-page);
  border-radius: var(--radius-md);
}
</style>
```

- [ ] **Step 2: Create BatchCreateModal.vue**

```vue
<script setup lang="ts">
defineOptions({ name: 'BatchCreateModal' })

import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'
import type { BatchVnicResult } from '../../types/api'

const props = defineProps<{ modelValue: boolean; instanceId: string; subnetId?: string }>()
const emit = defineEmits<{ 'update:modelValue': [val: boolean]; saved: [] }>()

const saving = ref(false)
const vnicCount = ref(1)
const ipv6Count = ref(0)
const result = ref<BatchVnicResult | null>(null)

async function handleCreate() {
  saving.value = true
  result.value = null
  try {
    const res = await request.post('/api/vnic/batch-create', {
      instanceId: props.instanceId,
      subnetId: props.subnetId ?? '',
      vnicCount: vnicCount.value,
      ipv6CountPerVnic: ipv6Count.value,
    }) as BatchVnicResult
    result.value = res
    if (res.allSuccessful) {
      ElMessage.success(`成功创建 ${res.successfulVnicCount} 个 VNIC`)
    } else {
      ElMessage.warning(res.summary || '部分 VNIC 创建失败')
    }
    emit('saved')
  } catch (e: any) {
    ElMessage.error(e.message || '批量创建失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="批量创建 VNIC + IPv6" width="520px" destroy-on-close>
    <el-form label-width="120px">
      <el-form-item label="VNIC 数量">
        <el-input-number v-model="vnicCount" :min="1" :max="32" />
      </el-form-item>
      <el-form-item label="每 VNIC IPv6 数">
        <el-input-number v-model="ipv6Count" :min="0" :max="32" />
      </el-form-item>
    </el-form>

    <el-alert v-if="result" :title="result.summary" :type="result.allSuccessful ? 'success' : 'warning'" :closable="false" show-icon style="margin-top: 12px;" />

    <template #footer>
      <el-button @click="emit('update:modelValue', false)">关闭</el-button>
      <el-button type="primary" :loading="saving" @click="handleCreate">创建</el-button>
    </template>
  </el-dialog>
</template>
```

- [ ] **Step 3: Refactor VnicManagement.vue with Tabs**

Wrap existing content in `el-tabs`, add VnicIpv6Row as expandable row:

```html
<el-tabs v-model="activeTab" type="border-card">
  <el-tab-pane label="VNIC 管理" name="vnic">
    <div style="margin-bottom: 12px;">
      <el-button type="primary" @click="batchCreateVisible = true">批量创建 VNIC+IPv6</el-button>
    </div>
    <el-table :data="vnicList" stripe row-key="vnicId">
      <el-table-column type="expand">
        <template #default="{ row }">
          <VnicIpv6Row :vnic-id="row.vnicId" :ipv6-addresses="row.ipv6Addresses ?? []" @refresh="loadVnics" />
        </template>
      </el-table-column>
      <el-table-column prop="vnicDisplayName" label="名称" />
      <el-table-column prop="privateIP" label="私有 IP" />
      <el-table-column prop="publicIP" label="公网 IP">
        <template #default="{ row }">{{ row.publicIP || '-' }}</template>
      </el-table-column>
      <el-table-column label="IPv6 数" width="100">
        <template #default="{ row }">{{ row.ipv6Addresses?.length ?? 0 }}</template>
      </el-table-column>
      <el-table-column label="操作" width="100" fixed="right">
        <template #default="{ row }">
          <el-button size="small" type="danger" @click="deleteVnic(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-tab-pane>

  <el-tab-pane label="安全规则" name="security">
    <!-- Task 7 -->
  </el-tab-pane>

  <el-tab-pane label="VCN 管理" name="vcn">
    <!-- Task 8 -->
  </el-tab-pane>

  <el-tab-pane label="网络配置" name="network">
    <!-- Task 9 -->
  </el-tab-pane>
</el-tabs>

<BatchCreateModal v-model="batchCreateVisible" :instance-id="currentInstanceId" @saved="loadVnics" />
```

Add imports and refs:

```typescript
import VnicIpv6Row from '../components/vnic/VnicIpv6Row.vue'
import BatchCreateModal from '../components/vnic/BatchCreateModal.vue'

const activeTab = ref('vnic')
const batchCreateVisible = ref(false)
```

- [ ] **Step 4: Verify**

Run: `cd frontend && npm run dev`
Expected: VNIC table with expandable rows showing IPv6 addresses

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/vnic/VnicIpv6Row.vue frontend/src/components/vnic/BatchCreateModal.vue
git add frontend/src/views/VnicManagement.vue
git commit -m "feat: add tab structure to VnicManagement with IPv6 expandable rows"
```

---

### Task 7: SecurityRulesPanel + AddRuleModal

**Files:**
- Create: `frontend/src/components/vnic/SecurityRulesPanel.vue`
- Create: `frontend/src/components/vnic/AddRuleModal.vue`

**Interfaces:**
- Consumes: `SecurityRule` from api.ts
- Props: `compartmentId`

- [ ] **Step 1: Create AddRuleModal.vue**

```vue
<script setup lang="ts">
defineOptions({ name: 'AddRuleModal' })

import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'

const props = defineProps<{ modelValue: boolean; compartmentId: string }>()
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
    await request.post('/api/security/rules/add', {
      compartmentId: props.compartmentId,
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
```

- [ ] **Step 2: Create SecurityRulesPanel.vue**

```vue
<script setup lang="ts">
defineOptions({ name: 'SecurityRulesPanel' })

import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../../utils/request'
import type { SecurityRule } from '../../types/api'
import AddRuleModal from './AddRuleModal.vue'

const props = defineProps<{ compartmentId: string }>()

const loading = ref(false)
const ingressRules = ref<SecurityRule[]>([])
const egressRules = ref<SecurityRule[]>([])
const addRuleVisible = ref(false)

async function loadRules() {
  loading.value = true
  try {
    const [ingress, egress] = await Promise.all([
      request.get('/api/security/rules', { params: { compartmentId: props.compartmentId, type: 'ingress' } }) as Promise<SecurityRule[]>,
      request.get('/api/security/rules', { params: { compartmentId: props.compartmentId, type: 'egress' } }) as Promise<SecurityRule[]>,
    ])
    ingressRules.value = ingress ?? []
    egressRules.value = egress ?? []
  } catch (e: any) {
    ElMessage.error(e.message || '加载安全规则失败')
  } finally {
    loading.value = false
  }
}

async function deleteRule(rule: SecurityRule) {
  try {
    await ElMessageBox.confirm('确定删除此安全规则？', '删除规则', { type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger' })
    await request.get('/api/security/rules/delete', { params: { compartmentId: props.compartmentId, id: rule.id } })
    ElMessage.success('规则已删除')
    loadRules()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '删除失败')
  }
}

async function enableAll() {
  try {
    await ElMessageBox.confirm('将为当前 Compartment 启用全部协议规则（IPv4 + IPv6 + ICMP），是否继续？', '一键启用', { type: 'warning', confirmButtonText: '确认启用' })
    await request.post('/api/security/rules/enable-all', { compartmentId: props.compartmentId })
    ElMessage.success('全部规则已启用')
    loadRules()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '操作失败')
  }
}

async function enableIpv6() {
  try {
    await ElMessageBox.confirm('将启用 IPv6 入站/出站规则，是否继续？', '启用 IPv6 规则', { type: 'info', confirmButtonText: '确认' })
    await request.post('/api/security/rules/enable-ipv6', { compartmentId: props.compartmentId })
    ElMessage.success('IPv6 规则已启用')
    loadRules()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '操作失败')
  }
}

onMounted(loadRules)
</script>

<template>
  <div class="security-panel" v-loading="loading">
    <div style="margin-bottom: 16px;">
      <el-space>
        <el-button type="danger" @click="enableAll">一键启用全部规则</el-button>
        <el-button @click="enableIpv6">启用 IPv6 规则</el-button>
        <el-button type="primary" @click="addRuleVisible = true">添加规则</el-button>
      </el-space>
    </div>

    <el-card shadow="hover" style="margin-bottom: 16px;">
      <template #header>入站规则 ({{ ingressRules.length }})</template>
      <el-table :data="ingressRules" stripe size="small">
        <el-table-column prop="protocol" label="协议" width="80" />
        <el-table-column prop="source" label="源 CIDR" />
        <el-table-column prop="ports" label="端口" width="120">
          <template #default="{ row }">{{ row.ports || '全部' }}</template>
        </el-table-column>
        <el-table-column prop="icmpType" label="ICMP Type" width="100">
          <template #default="{ row }">{{ row.icmpType || '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="danger" @click="deleteRule(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card shadow="hover">
      <template #header>出站规则 ({{ egressRules.length }})</template>
      <el-table :data="egressRules" stripe size="small">
        <el-table-column prop="protocol" label="协议" width="80" />
        <el-table-column prop="source" label="目标 CIDR" />
        <el-table-column prop="ports" label="端口" width="120">
          <template #default="{ row }">{{ row.ports || '全部' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="danger" @click="deleteRule(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <AddRuleModal v-model="addRuleVisible" :compartment-id="compartmentId" @saved="loadRules" />
  </div>
</template>

<style scoped>
.security-panel {
  padding: var(--space-4);
}
</style>
```

- [ ] **Step 3: Register in VnicManagement.vue**

```typescript
import SecurityRulesPanel from '../components/vnic/SecurityRulesPanel.vue'
```

Replace the security tab placeholder:

```html
<el-tab-pane label="安全规则" name="security">
  <SecurityRulesPanel :compartment-id="currentCompartmentId" />
</el-tab-pane>
```

- [ ] **Step 4: Verify**

Run: `cd frontend && npm run dev`
Expected: Security rules tab shows ingress/egress tables with add/delete

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/vnic/SecurityRulesPanel.vue frontend/src/components/vnic/AddRuleModal.vue
git add frontend/src/views/VnicManagement.vue
git commit -m "feat: add SecurityRulesPanel with add/delete and enable-all"
```

---

### Task 8: VCNPanel

**Files:**
- Create: `frontend/src/components/vnic/VCNPanel.vue`
- Create: `frontend/src/components/vnic/ConfigureIPv6Modal.vue`
- Create: `frontend/src/components/vnic/ReassignIPModal.vue`

**Interfaces:**
- Consumes: `VcnInfo` from Task 1

- [ ] **Step 1: Create ConfigureIPv6Modal.vue**

```vue
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
```

- [ ] **Step 2: Create ReassignIPModal.vue**

```vue
<script setup lang="ts">
defineOptions({ name: 'ReassignIPModal' })

import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'

const props = defineProps<{ modelValue: boolean; instanceId: string }>()
const emit = defineEmits<{ 'update:modelValue': [val: boolean]; saved: [newIp: string] }>()

const saving = ref(false)

async function handleReassign() {
  saving.value = true
  try {
    const res = await request.post('/api/vcn/reassign-ip', { instanceId: props.instanceId }) as { publicIp: string }
    ElMessage.success(`新公网 IP: ${res.publicIp}`)
    emit('saved', res.publicIp)
    emit('update:modelValue', false)
  } catch (e: any) {
    ElMessage.error(e.message || '重新分配失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="重新分配公网 IP" width="420px" destroy-on-close>
    <el-alert title="旧公网 IP 将立即失效，请确保没有依赖该 IP 的服务。" type="warning" :closable="false" show-icon />
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="warning" :loading="saving" @click="handleReassign">确认重新分配</el-button>
    </template>
  </el-dialog>
</template>
```

- [ ] **Step 3: Create VCNPanel.vue**

```vue
<script setup lang="ts">
defineOptions({ name: 'VCNPanel' })

import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'
import type { VcnInfo } from '../../types/api'
import ConfigureIPv6Modal from './ConfigureIPv6Modal.vue'
import ReassignIPModal from './ReassignIPModal.vue'

const props = defineProps<{ instanceId: string }>()

const loading = ref(false)
const vcnList = ref<VcnInfo[]>([])
const selectedVcn = ref<VcnInfo | null>(null)
const currentPublicIp = ref('')

// Modals
const configureIpv6Visible = ref(false)
const reassignIpVisible = ref(false)

async function loadVcns() {
  loading.value = true
  try {
    const res = await request.get('/api/vcn/list') as VcnInfo[]
    vcnList.value = res ?? []
  } catch (e: any) {
    ElMessage.error(e.message || '加载 VCN 列表失败')
  } finally {
    loading.value = false
  }
}

async function loadPublicIp() {
  try {
    const res = await request.get('/api/instances/detail', { params: { instanceId: props.instanceId } }) as any
    currentPublicIp.value = res?.publicIp ?? ''
  } catch {}
}

function selectVcn(vcn: VcnInfo) {
  selectedVcn.value = vcn
}

onMounted(() => {
  loadVcns()
  loadPublicIp()
})
</script>

<template>
  <div class="vcn-panel" v-loading="loading">
    <!-- VCN List -->
    <el-card shadow="hover" style="margin-bottom: 16px;">
      <template #header>VCN 列表</template>
      <el-table :data="vcnList" stripe highlight-current-row @current-change="selectVcn">
        <el-table-column prop="displayName" label="名称" />
        <el-table-column prop="cidrBlock" label="CIDR" />
        <el-table-column prop="dnsLabel" label="DNS 标签" />
        <el-table-column prop="timeCreated" label="创建时间" width="180" />
      </el-table>
      <el-empty v-if="vcnList.length === 0" description="暂无 VCN" />
    </el-card>

    <!-- VCN Detail -->
    <el-card v-if="selectedVcn" shadow="hover" style="margin-bottom: 16px;">
      <template #header>VCN 详情: {{ selectedVcn.displayName }}</template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="OCID"><el-text truncated>{{ selectedVcn.id }}</el-text></el-descriptions-item>
        <el-descriptions-item label="CIDR">{{ selectedVcn.cidrBlock }}</el-descriptions-item>
        <el-descriptions-item label="DNS 标签">{{ selectedVcn.dnsLabel || '-' }}</el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ selectedVcn.timeCreated }}</el-descriptions-item>
      </el-descriptions>
      <el-space style="margin-top: 12px;">
        <el-button @click="configureIpv6Visible = true">配置 IPv6 安全规则</el-button>
      </el-space>
    </el-card>

    <!-- Public IP -->
    <el-card shadow="hover">
      <template #header>公网 IP 管理</template>
      <div style="display: flex; align-items: center; gap: 16px;">
        <span>当前公网 IP: <strong>{{ currentPublicIp || '无' }}</strong></span>
        <el-button type="warning" @click="reassignIpVisible = true">重新分配公网 IP</el-button>
      </div>
    </el-card>

    <!-- Modals -->
    <ConfigureIPv6Modal v-model="configureIpv6Visible" :vcn-id="selectedVcn?.id ?? ''" @saved="loadVcns" />
    <ReassignIPModal v-model="reassignIpVisible" :instance-id="instanceId" @saved="(ip) => currentPublicIp = ip" />
  </div>
</template>

<style scoped>
.vcn-panel {
  padding: var(--space-4);
}
</style>
```

- [ ] **Step 4: Register in VnicManagement.vue**

```typescript
import VCNPanel from '../components/vnic/VCNPanel.vue'
```

Replace the VCN tab placeholder:

```html
<el-tab-pane label="VCN 管理" name="vcn">
  <VCNPanel :instance-id="currentInstanceId" />
</el-tab-pane>
```

- [ ] **Step 5: Verify**

Run: `cd frontend && npm run dev`
Expected: VCN tab shows VCN list, details, and public IP management

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/vnic/VCNPanel.vue frontend/src/components/vnic/ConfigureIPv6Modal.vue frontend/src/components/vnic/ReassignIPModal.vue
git add frontend/src/views/VnicManagement.vue
git commit -m "feat: add VCNPanel with IPv6 config and public IP reassignment"
```

---

### Task 9: NetworkConfigPanel (NAT + Route Table)

**Files:**
- Create: `frontend/src/components/vnic/NetworkConfigPanel.vue`
- Create: `frontend/src/components/vnic/CreateNatModal.vue`
- Create: `frontend/src/components/vnic/CreateRouteTableModal.vue`

**Interfaces:**
- Consumes: `NatGatewayInfo`, `RouteTableInfo` from Task 1

- [ ] **Step 1: Create CreateNatModal.vue**

```vue
<script setup lang="ts">
defineOptions({ name: 'CreateNatModal' })

import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../../utils/request'

const props = defineProps<{ modelValue: boolean; compartmentId: string; vcnId: string }>()
const emit = defineEmits<{ 'update:modelValue': [val: boolean]; saved: [] }>()

const saving = ref(false)
const displayName = ref('')

async function handleCreate() {
  if (!displayName.value.trim()) {
    ElMessage.warning('请输入名称')
    return
  }
  saving.value = true
  try {
    await request.post('/api/nat/create', { compartmentId: props.compartmentId, vcnId: props.vcnId, displayName: displayName.value.trim() })
    ElMessage.success('NAT 网关创建成功')
    emit('saved')
    emit('update:modelValue', false)
  } catch (e: any) {
    ElMessage.error(e.message || '创建失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" title="创建 NAT 网关" width="420px" destroy-on-close>
    <el-form label-width="80px">
      <el-form-item label="名称">
        <el-input v-model="displayName" placeholder="NAT 网关名称" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="handleCreate">创建</el-button>
    </template>
  </el-dialog>
</template>
```

- [ ] **Step 2: Create CreateRouteTableModal.vue**

```vue
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
```

- [ ] **Step 3: Create NetworkConfigPanel.vue**

```vue
<script setup lang="ts">
defineOptions({ name: 'NetworkConfigPanel' })

import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../../utils/request'
import type { NatGatewayInfo, RouteTableInfo } from '../../types/api'
import CreateNatModal from './CreateNatModal.vue'
import CreateRouteTableModal from './CreateRouteTableModal.vue'

const props = defineProps<{ compartmentId: string; vcnId: string; instanceId: string }>()

const loading = ref(false)
const natGateways = ref<NatGatewayInfo[]>([])
const routeTables = ref<RouteTableInfo[]>([])

// Modals
const createNatVisible = ref(false)
const createRouteTableVisible = ref(false)

async function loadNatGateways() {
  try {
    const res = await request.get('/api/nat/list', { params: { compartmentId: props.compartmentId, vcnId: props.vcnId } }) as NatGatewayInfo[]
    natGateways.value = res ?? []
  } catch (e: any) {
    ElMessage.error(e.message || '加载 NAT 网关失败')
  }
}

async function loadRouteTables() {
  try {
    const res = await request.get('/api/route-table/list', { params: { compartmentId: props.compartmentId, vcnId: props.vcnId } }) as RouteTableInfo[]
    routeTables.value = res ?? []
  } catch (e: any) {
    ElMessage.error(e.message || '加载路由表失败')
  }
}

async function deleteNat(nat: NatGatewayInfo) {
  try {
    await ElMessageBox.confirm(`确定删除 NAT 网关 "${nat.displayName}"？`, '删除 NAT 网关', { type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger' })
    await request.get('/api/nat/delete', { params: { natGatewayId: nat.id } })
    ElMessage.success('NAT 网关已删除')
    loadNatGateways()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '删除失败')
  }
}

async function deleteRouteTable(rt: RouteTableInfo) {
  try {
    await ElMessageBox.confirm(`确定删除路由表 "${rt.displayName}"？`, '删除路由表', { type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger' })
    await request.get('/api/route-table/delete', { params: { routeTableId: rt.id } })
    ElMessage.success('路由表已删除')
    loadRouteTables()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '删除失败')
  }
}

async function resetToDefault() {
  try {
    await ElMessageBox.confirm('将重置实例主 VNIC 的路由表为 VCN 默认路由表，是否继续？', '重置路由表', { type: 'info', confirmButtonText: '确认重置' })
    await request.post('/api/route-table/reset', { instanceId: props.instanceId })
    ElMessage.success('路由表已重置为默认')
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '重置失败')
  }
}

function loadData() {
  loadNatGateways()
  loadRouteTables()
}

onMounted(loadData)
</script>

<template>
  <div class="network-config-panel" v-loading="loading">
    <!-- NAT Gateways -->
    <el-card shadow="hover" style="margin-bottom: 16px;">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <span>NAT 网关</span>
          <el-button size="small" type="primary" @click="createNatVisible = true">创建 NAT 网关</el-button>
        </div>
      </template>
      <el-table :data="natGateways" stripe>
        <el-table-column prop="displayName" label="名称" />
        <el-table-column prop="id" label="OCID">
          <template #default="{ row }"><el-text truncated>{{ row.id }}</el-text></template>
        </el-table-column>
        <el-table-column prop="lifecycleState" label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.lifecycleState === 'AVAILABLE' ? 'success' : 'info'" size="small">{{ row.lifecycleState }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="danger" @click="deleteNat(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="natGateways.length === 0" description="暂无 NAT 网关" />
    </el-card>

    <!-- Route Tables -->
    <el-card shadow="hover">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <span>路由表</span>
          <el-space>
            <el-button size="small" type="primary" @click="createRouteTableVisible = true">创建路由表</el-button>
            <el-button size="small" @click="resetToDefault">重置为默认路由表</el-button>
          </el-space>
        </div>
      </template>
      <el-table :data="routeTables" stripe>
        <el-table-column prop="displayName" label="名称" />
        <el-table-column prop="id" label="OCID">
          <template #default="{ row }"><el-text truncated>{{ row.id }}</el-text></template>
        </el-table-column>
        <el-table-column label="规则数" width="100">
          <template #default="{ row }">{{ row.routeRules?.length ?? 0 }}</template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="danger" @click="deleteRouteTable(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="routeTables.length === 0" description="暂无路由表" />
    </el-card>

    <!-- Modals -->
    <CreateNatModal v-model="createNatVisible" :compartment-id="compartmentId" :vcn-id="vcnId" @saved="loadNatGateways" />
    <CreateRouteTableModal v-model="createRouteTableVisible" :compartment-id="compartmentId" :vcn-id="vcnId" @saved="loadRouteTables" />
  </div>
</template>

<style scoped>
.network-config-panel {
  padding: var(--space-4);
}
</style>
```

- [ ] **Step 4: Register in VnicManagement.vue**

```typescript
import NetworkConfigPanel from '../components/vnic/NetworkConfigPanel.vue'
```

Replace the network tab placeholder:

```html
<el-tab-pane label="网络配置" name="network">
  <NetworkConfigPanel :compartment-id="currentCompartmentId" :vcn-id="currentVcnId" :instance-id="currentInstanceId" />
</el-tab-pane>
```

- [ ] **Step 5: Verify**

Run: `cd frontend && npm run dev`
Expected: Network config tab shows NAT gateways and route tables

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/vnic/NetworkConfigPanel.vue frontend/src/components/vnic/CreateNatModal.vue frontend/src/components/vnic/CreateRouteTableModal.vue
git add frontend/src/views/VnicManagement.vue
git commit -m "feat: add NetworkConfigPanel with NAT gateway and route table CRUD"
```

---

### Task 10: Backend Handler Audit & Missing Handlers

**Files:**
- Modify: `internal/httpapi/server.go` (if routes missing)
- Create/Modify: handler files as needed

**Interfaces:**
- Produces: API endpoints consumed by Tasks 4-9

- [ ] **Step 1: Audit existing routes**

Check `internal/httpapi/server.go` for each required endpoint:

| Endpoint | Handler | Status |
|----------|---------|--------|
| `GET /api/instances/traffic` | Check if exists | Audit |
| `GET /api/boot-volume/detail` | Check if exists | Audit |
| `POST /api/boot-volume/vpu` | Check if exists | Audit |
| `POST /api/boot-volume/resize` | Likely missing | **Create** |
| `GET /api/backup/list` | Check if exists | Audit |
| `POST /api/backup/create` | Check if exists | Audit |
| `GET /api/backup/delete` | Check if exists | Audit |
| `POST /api/backup/copy` | Check if exists | Audit |
| `POST /api/vnic/ipv6/assign` | Check if exists | Audit |
| `POST /api/vnic/ipv6/delete` | Check if exists | Audit |
| `POST /api/vcn/list` | Check if exists | Audit |
| `POST /api/vcn/configure-ipv6` | Check if exists | Audit |
| `POST /api/vcn/reassign-ip` | Check if exists | Audit |
| `POST /api/security/rules/enable-all` | Check if exists | Audit |
| `POST /api/security/rules/enable-ipv6` | Check if exists | Audit |
| `GET /api/nat/list` | Check if exists | Audit |
| `POST /api/nat/create` | Check if exists | Audit |
| `GET /api/nat/delete` | Check if exists | Audit |
| `GET /api/route-table/list` | Check if exists | Audit |
| `POST /api/route-table/create` | Check if exists | Audit |
| `GET /api/route-table/delete` | Check if exists | Audit |
| `POST /api/route-table/update-vnic` | Check if exists | Audit |
| `POST /api/route-table/reset` | Check if exists | Audit |

- [ ] **Step 2: Create missing boot-volume/resize handler**

If `POST /api/boot-volume/resize` is missing, add to the appropriate handler file:

```go
// In handler file for boot volume operations
func (h *Handler) HandleBootVolumeResize(c *gin.Context) {
    var req struct {
        BootVolumeID string `json:"bootVolumeId" binding:"required"`
        SizeInGBs    int64  `json:"sizeInGBs" binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, err.Error())
        return
    }

    // Get tenant context
    tenantID, exists := c.Get("tenantId")
    if !exists {
        response.Unauthorized(c, "tenant not found")
        return
    }

    // Build OCI clients
    // ... (follow existing pattern in the handler)

    // Call SDK
    // resp, err := blockstorageClient.UpdateBootVolume(ctx, core.UpdateBootVolumeRequest{...})

    // Validate: newSize must be > currentSize
    // ...

    response.Success(c, nil)
}
```

Register in `server.go`:

```go
api.POST("/boot-volume/resize", h.HandleBootVolumeResize)
```

- [ ] **Step 3: Verify all routes compile**

Run: `go build ./...`
Expected: No compilation errors

- [ ] **Step 4: Commit**

```bash
git add internal/httpapi/
git commit -m "feat: add missing backend handlers for disk/network management"
```

---

### Task 11: Final Integration Test

- [ ] **Step 1: Build frontend**

Run: `cd frontend && npm run build`
Expected: Build succeeds with no errors

- [ ] **Step 2: Build backend**

Run: `go build ./...`
Expected: Build succeeds

- [ ] **Step 3: Manual smoke test**

Start the app and verify:
1. InstanceDetail → 流量监控 tab shows traffic stats
2. InstanceDetail → 磁盘管理 tab shows boot volume info and backups
3. VnicManagement → VNIC table has expandable IPv6 rows
4. VnicManagement → 安全规则 tab shows rules with add/delete
5. VnicManagement → VCN 管理 tab shows VCN list
6. VnicManagement → 网络配置 tab shows NAT/route tables
7. Dashboard shows backup count
8. SystemSettings notification history loads correctly

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "feat: complete frontend enhancement for instance/disk/network management"
```
