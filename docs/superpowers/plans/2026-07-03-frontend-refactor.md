# Frontend Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor the oci-start-go frontend to improve UX, visual consistency, and code quality across 5 core pages.

**Architecture:** Extract 8 common components, optimize the layout with sidebar grouping, and refactor Dashboard, TenantList, TenantDetail, Instances, and Settings pages with improved information hierarchy and responsive design.

**Tech Stack:** Vue 3, Element Plus, TypeScript, Vite, CSS Variables (Design Tokens)

## Global Constraints

- Pure frontend refactor — no API changes
- Keep "Operator" dark theme and CSS tokens
- No changes to URL routing structure
- Follow existing code patterns and naming conventions
- Responsive design: desktop (>1024px), tablet (768-1024px), mobile (<768px)

---

## Phase 1: Common Components

### Task 1: Create PageHeader Component

**Files:**
- Create: `frontend/src/components/common/PageHeader.vue`
- Modify: `frontend/src/views/Dashboard.vue` (replace toolbar)

**Interfaces:**
- Consumes: Element Plus icons, CSS tokens
- Produces: `<PageHeader title="..." />` with optional slot for actions

- [ ] **Step 1: Create PageHeader.vue**

```vue
<template>
  <div class="page-header">
    <div class="page-header-left">
      <h2 class="page-header-title">{{ title }}</h2>
      <slot name="extra" />
    </div>
    <div class="page-header-right">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  title: string
}>()
</script>

<style scoped>
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-6);
  flex-wrap: wrap;
  gap: var(--space-4);
}

.page-header-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.page-header-title {
  margin: 0;
  font-size: var(--text-xl);
  font-weight: var(--font-bold);
  color: var(--text-primary);
  letter-spacing: var(--tracking-tight);
}

.page-header-right {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
</style>
```

- [ ] **Step 2: Update Dashboard.vue to use PageHeader**

Replace the `.toolbar` section in `Dashboard.vue`:

```vue
<template>
  <div class="dashboard">
    <!-- Page header -->
    <PageHeader title="仪表盘">
      <template #actions>
        <el-button @click="loadAll" :loading="loading" size="small">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </template>
    </PageHeader>
    <!-- ... rest of template -->
  </div>
</template>

<script setup lang="ts">
import PageHeader from '../components/common/PageHeader.vue'
// ... existing imports and logic
</script>
```

- [ ] **Step 3: Verify build passes**

Run: `cd frontend && npm run build`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/common/PageHeader.vue frontend/src/views/Dashboard.vue
git commit -m "feat: add PageHeader component and update Dashboard"
```

---

### Task 2: Create StatusBadge Component

**Files:**
- Create: `frontend/src/components/common/StatusBadge.vue`

**Interfaces:**
- Consumes: CSS tokens for status colors
- Produces: `<StatusBadge status="up|down|warn|idle" :pulse="true" />`

- [ ] **Step 1: Create StatusBadge.vue**

```vue
<template>
  <span class="status-badge" :class="badgeClass">
    <span class="status-badge-dot" :class="{ 'status-badge-dot--pulse': pulse }" />
    <span v-if="label" class="status-badge-label">{{ label }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  status: 'up' | 'down' | 'warn' | 'idle'
  label?: string
  pulse?: boolean
}>()

const badgeClass = computed(() => `status-badge--${props.status}`)
</script>

<style scoped>
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
}

.status-badge-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-badge--up .status-badge-dot {
  background: var(--status-up);
  box-shadow: 0 0 6px rgba(52, 168, 83, 0.4);
}

.status-badge--down .status-badge-dot {
  background: var(--status-down);
  box-shadow: 0 0 6px rgba(234, 67, 53, 0.4);
}

.status-badge--warn .status-badge-dot {
  background: var(--status-warn);
  box-shadow: 0 0 6px rgba(249, 171, 0, 0.4);
}

.status-badge--idle .status-badge-dot {
  background: var(--status-idle);
}

.status-badge-dot--pulse {
  animation: status-pulse 2s ease-in-out infinite;
}

@keyframes status-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.status-badge-label {
  font-size: var(--text-xs);
  color: var(--text-secondary);
  font-weight: var(--font-medium);
}
</style>
```

- [ ] **Step 2: Verify build passes**

Run: `cd frontend && npm run build`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/common/StatusBadge.vue
git commit -m "feat: add StatusBadge component"
```

---

### Task 3: Create MonoText Component

**Files:**
- Create: `frontend/src/components/common/MonoText.vue`

**Interfaces:**
- Consumes: CSS tokens for monospace font
- Produces: `<MonoText>1.2.3.4</MonoText>`

- [ ] **Step 1: Create MonoText.vue**

```vue
<template>
  <span class="mono-text">
    <slot />
  </span>
</template>

<style scoped>
.mono-text {
  font-family: var(--font-mono);
  font-size: 0.875em;
  letter-spacing: var(--tracking-mono);
}
</style>
```

- [ ] **Step 2: Verify build passes**

Run: `cd frontend && npm run build`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/common/MonoText.vue
git commit -m "feat: add MonoText component"
```

---

### Task 4: Create StatusCard Component

**Files:**
- Create: `frontend/src/components/common/StatusCard.vue`

**Interfaces:**
- Consumes: CSS tokens, StatusBadge component
- Produces: `<StatusCard :value="8" label="在线实例" status="up" />`

- [ ] **Step 1: Create StatusCard.vue**

```vue
<template>
  <component
    :is="link ? 'router-link' : 'div'"
    :to="link"
    class="status-card"
    :class="{ 'status-card--link': link }"
  >
    <div class="status-card-header">
      <span class="status-card-icon">
        <el-icon :size="16"><component :is="icon" /></el-icon>
      </span>
      <span class="status-card-label">{{ label }}</span>
    </div>
    <div class="status-card-body">
      <span class="status-card-value">{{ value }}</span>
      <span v-if="sub" class="status-card-sub">{{ sub }}</span>
    </div>
    <div class="status-card-rule" />
    <div class="status-card-status">
      <StatusBadge :status="status" />
      <span class="status-card-status-text">{{ statusText }}</span>
    </div>
  </component>
</template>

<script setup lang="ts">
import StatusBadge from './StatusBadge.vue'

defineProps<{
  value: number | string
  label: string
  icon: any
  sub?: string
  status: 'up' | 'down' | 'warn' | 'idle'
  statusText: string
  link?: string
}>()
</script>

<style scoped>
.status-card {
  background: var(--bg-surface);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  padding: var(--space-4) var(--space-5);
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
  text-decoration: none;
  color: inherit;
}

.status-card--link {
  cursor: pointer;
}

.status-card--link:hover {
  border-color: var(--accent);
  box-shadow: 0 0 0 1px var(--accent-subtle);
}

.status-card-header {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.status-card-icon {
  display: flex;
  align-items: center;
  color: var(--text-muted);
}

.status-card-label {
  font-size: var(--text-xs);
  font-weight: var(--font-medium);
  color: var(--text-muted);
  letter-spacing: var(--tracking-wide);
}

.status-card-body {
  display: flex;
  align-items: baseline;
  gap: var(--space-1);
}

.status-card-value {
  font-size: var(--text-3xl);
  font-weight: var(--font-bold);
  color: var(--text-primary);
  line-height: 1;
  letter-spacing: var(--tracking-tight);
  font-variant-numeric: tabular-nums;
}

.status-card-sub {
  font-size: var(--text-sm);
  color: var(--text-muted);
  font-weight: var(--font-medium);
}

.status-card-rule {
  height: 1px;
  background: var(--border-subtle);
}

.status-card-status {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.status-card-status-text {
  font-size: var(--text-xs);
  color: var(--text-muted);
  font-weight: var(--font-medium);
}
</style>
```

- [ ] **Step 2: Verify build passes**

Run: `cd frontend && npm run build`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/common/StatusCard.vue
git commit -m "feat: add StatusCard component"
```

---

## Phase 2: Layout Optimization

### Task 5: Update Default Layout with Sidebar Groups

**Files:**
- Modify: `frontend/src/layouts/Default.vue`

**Interfaces:**
- Consumes: CSS tokens, Element Plus icons
- Produces: Grouped sidebar navigation

- [ ] **Step 1: Update navItems with groups**

Replace the `navItems` array and update the template:

```vue
<script setup lang="ts">
// ... existing imports

interface NavGroup {
  label: string
  items: NavItem[]
}

interface NavItem {
  path: string
  label: string
  icon: any
}

const navGroups: NavGroup[] = [
  {
    label: '概览',
    items: [
      { path: '/', label: '仪表盘', icon: DataBoard },
    ]
  },
  {
    label: '资源管理',
    items: [
      { path: '/tenants', label: '租户管理', icon: Connection },
      { path: '/instances', label: '实例管理', icon: Monitor },
      { path: '/vnic', label: 'VNIC 管理', icon: Connection },
      { path: '/storage', label: '对象存储', icon: Folder },
    ]
  },
  {
    label: '运维工具',
    items: [
      { path: '/boot', label: '抢机任务', icon: VideoPlay },
      { path: '/dns', label: 'DNS 管理', icon: Coin },
      { path: '/migration', label: '数据迁移', icon: Download },
      { path: '/rescue', label: '实例救援', icon: WarningFilled },
    ]
  },
  {
    label: '访问终端',
    items: [
      { path: '/terminal', label: 'SSH 终端', icon: Operation },
      { path: '/console', label: 'VNC 控制台', icon: Monitor },
    ]
  },
]
</script>
```

- [ ] **Step 2: Update sidebar template with grouped navigation**

```vue
<!-- Navigation -->
<nav class="nav">
  <template v-for="group in navGroups" :key="group.label">
    <div v-if="!collapsed" class="nav-group-label">{{ group.label }}</div>
    <router-link
      v-for="item in group.items"
      :key="item.path"
      :to="item.path"
      class="nav-item"
      :class="{ 'nav-item--active': isActive(item.path) }"
      @click="onNavClick"
    >
      <span class="nav-icon">
        <el-icon :size="18"><component :is="item.icon" /></el-icon>
      </span>
      <transition name="fade-text">
        <span v-if="!collapsed" class="nav-label">{{ item.label }}</span>
      </transition>
    </router-link>
  </template>
</nav>
```

- [ ] **Step 3: Add nav group label styles**

```css
.nav-group-label {
  font-size: var(--text-2xs);
  font-weight: var(--font-semibold);
  color: var(--text-muted);
  letter-spacing: var(--tracking-wide);
  text-transform: uppercase;
  padding: var(--space-3) var(--space-3) var(--space-1);
  white-space: nowrap;
}
```

- [ ] **Step 4: Update system settings nav item**

Add settings to the navGroups as a standalone item at the bottom:

```vue
<!-- Add after the nav groups loop -->
<router-link
  to="/settings"
  class="nav-item"
  :class="{ 'nav-item--active': isActive('/settings') }"
  @click="onNavClick"
>
  <span class="nav-icon">
    <el-icon :size="18"><Setting /></el-icon>
  </span>
  <transition name="fade-text">
    <span v-if="!collapsed" class="nav-label">系统设置</span>
  </transition>
</router-link>
```

- [ ] **Step 5: Verify build passes**

Run: `cd frontend && npm run build`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add frontend/src/layouts/Default.vue
git commit -m "feat: add sidebar navigation groups"
```

---

## Phase 3: Dashboard Refactoring

### Task 6: Refactor Dashboard with StatusCards

**Files:**
- Modify: `frontend/src/views/Dashboard.vue`

**Interfaces:**
- Consumes: PageHeader, StatusCard components
- Produces: 3 stat cards, 2 panel grid

- [ ] **Step 1: Import new components**

```vue
<script setup lang="ts">
import PageHeader from '../components/common/PageHeader.vue'
import StatusCard from '../components/common/StatusCard.vue'
// ... existing imports
</script>
```

- [ ] **Step 2: Replace stat cards section**

```vue
<template>
  <div class="dashboard">
    <PageHeader title="仪表盘">
      <template #actions>
        <el-button @click="loadAll" :loading="loading" size="small">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </template>
    </PageHeader>

    <!-- Error alert -->
    <el-alert
      v-if="loadError"
      :title="loadError"
      type="error"
      show-icon
      :closable="true"
      @close="loadError = ''"
      style="margin-bottom: 20px"
    />

    <!-- Stat cards -->
    <div class="stat-grid">
      <StatusCard
        :value="stats.onlineCount"
        label="在线实例"
        :icon="Monitor"
        :sub="stats.instanceCount ? `/ ${stats.instanceCount}` : undefined"
        status="up"
        :status-text="onlineRateText"
        link="/instances"
      />
      <StatusCard
        :value="stats.tenantCount"
        label="租户数量"
        :icon="User"
        status="idle"
        status-text="已配置"
        link="/tenants"
      />
      <StatusCard
        :value="engine.runningTasks ?? 0"
        label="抢机引擎"
        :icon="SetUp"
        :sub="`/ ${engine.totalTasks ?? 0}`"
        :status="engineRunning ? 'up' : 'down'"
        :status-text="engineRunning ? '运行中' : '已停止'"
        link="/boot"
      />
    </div>

    <!-- ... rest of template -->
  </div>
</template>
```

- [ ] **Step 3: Add computed for online rate text**

```vue
<script setup lang="ts">
const onlineRateText = computed(() => {
  if (stats.value.instanceCount) {
    return `${Math.round((stats.value.onlineCount / stats.value.instanceCount) * 100)}% 在线率`
  }
  return '暂无数据'
})
</script>
```

- [ ] **Step 4: Update stat-grid styles for 3 columns**

```css
.stat-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-4);
  margin-bottom: var(--space-6);
}

@media (max-width: 768px) {
  .stat-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 480px) {
  .stat-grid {
    grid-template-columns: 1fr;
  }
}
```

- [ ] **Step 5: Verify build passes**

Run: `cd frontend && npm run build`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add frontend/src/views/Dashboard.vue
git commit -m "refactor: Dashboard with 3 StatusCards and grouped layout"
```

---

## Phase 4: TenantList Refactoring

### Task 7: Refactor TenantList Table Columns

**Files:**
- Modify: `frontend/src/views/TenantList.vue`

**Interfaces:**
- Consumes: PageHeader, StatusBadge, MonoText components
- Produces: 9-column table with improved styling

- [ ] **Step 1: Import new components**

```vue
<script setup lang="ts">
import PageHeader from '../components/common/PageHeader.vue'
import StatusBadge from '../components/common/StatusBadge.vue'
import MonoText from '../components/common/MonoText.vue'
// ... existing imports
</script>
```

- [ ] **Step 2: Update template with PageHeader**

```vue
<template>
  <div class="tenants-page">
    <!-- Toolbar -->
    <PageHeader title="租户管理">
      <template #extra>
        <el-tag type="info" size="small">{{ rows.length }} 个租户</el-tag>
        <el-input v-model="searchText" placeholder="搜索租户名称..." size="small" clearable
          style="width: 200px" :prefix-icon="Search" />
      </template>
      <template #actions>
        <el-button type="primary" @click="openAdd"><el-icon><Plus /></el-icon> 新增租户</el-button>
        <el-button @click="startBatchCheck" :disabled="rows.length === 0"><el-icon><Connection /></el-icon> 批量检测</el-button>
        <el-button @click="load" :loading="loading"><el-icon><Refresh /></el-icon> 刷新</el-button>
      </template>
    </PageHeader>

    <!-- Table -->
    <!-- ... -->
  </div>
</template>
```

- [ ] **Step 3: Update table columns**

Remove unnecessary columns and update the remaining ones:

```vue
<el-table :data="filteredRows" v-loading="loading" border stripe style="width: 100%">
  <template #empty>
    <el-empty description="暂无租户，请新增" :image-size="80">
      <el-button type="primary" @click="openAdd">新增租户</el-button>
    </el-empty>
  </template>
  <el-table-column type="index" label="#" width="50" align="center" />
  <el-table-column label="租户名" min-width="110">
    <template #default="{ row }">
      <span class="spoiler-link" @click="showName = showName === row.id ? 0 : row.id">
        <template v-if="showName === row.id">{{ row.tenancyName || row.userName }}</template>
        <template v-else>{{ maskedName(row.tenancyName || row.userName) }}</template>
      </span>
    </template>
  </el-table-column>
  <el-table-column label="自定义名称" min-width="120">
    <template #default="{ row }">
      <span class="cell-edit-link" @click="router.push({name:'tenant-detail', params:{id:row.id}})" :title="row.tenancyDes || '点击设置'">{{ row.tenancyDes || '—' }}</span>
    </template>
  </el-table-column>
  <el-table-column label="区域" width="100">
    <template #default="{ row }"><MonoText>{{ row.regionName || row.region || '—' }}</MonoText></template>
  </el-table-column>
  <el-table-column label="订阅天数" width="90" align="center">
    <template #default="{ row }">
      <span class="days-chip" :class="daysChipClass(row.activeDays)">{{ row.activeDays || '—' }}</span>
    </template>
  </el-table-column>
  <el-table-column label="账号类型" width="90" align="center">
    <template #default="{ row }">
      <el-tag size="small" :type="row.planType === 'PAYG' ? '' : 'info'">
        {{ row.planType || '—' }}
      </el-tag>
    </template>
  </el-table-column>
  <el-table-column label="实例" width="80" align="center">
    <template #default="{ row }">{{ row.instanceCount ?? 0 }}</template>
  </el-table-column>
  <el-table-column label="状态" width="80" align="center">
    <template #default="{ row }">
      <StatusBadge :status="row.isActive ? 'up' : 'down'" :pulse="row.isActive" />
      <span class="status-text">{{ row.isActive ? '正常' : '停用' }}</span>
    </template>
  </el-table-column>
  <el-table-column label="操作" width="60" fixed="right" align="center">
    <template #default="{ row }">
      <el-dropdown trigger="click" @command="(cmd: string) => handleAction(cmd, row)">
        <el-button size="small" text><el-icon><MoreFilled /></el-icon></el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="detail"><el-icon><InfoFilled /></el-icon> 详情</el-dropdown-item>
            <el-dropdown-item command="sync"><el-icon><Connection /></el-icon> 同步 OCI</el-dropdown-item>
            <el-dropdown-item command="export" divided><el-icon><Download /></el-icon> 导出租户</el-dropdown-item>
            <el-dropdown-item command="delete" divided style="color:var(--status-down)"><el-icon><Delete /></el-icon> 删除</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </template>
  </el-table-column>
</el-table>
```

- [ ] **Step 4: Add days chip color function**

```vue
<script setup lang="ts">
function daysChipClass(days: string | undefined): string {
  if (!days) return ''
  const num = parseInt(days)
  if (isNaN(num)) return ''
  if (num > 365) return 'days-chip--green'
  if (num >= 90) return ''
  if (num >= 30) return 'days-chip--yellow'
  return 'days-chip--red'
}
</script>
```

- [ ] **Step 5: Add days chip styles**

```css
.days-chip {
  display: inline-block;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  background: var(--bg-raised);
  font-size: var(--text-sm);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

.days-chip--green {
  background: color-mix(in srgb, var(--status-up) 15%, transparent);
  color: var(--status-up);
}

.days-chip--yellow {
  background: color-mix(in srgb, var(--status-warn) 15%, transparent);
  color: var(--status-warn);
}

.days-chip--red {
  background: color-mix(in srgb, var(--status-down) 15%, transparent);
  color: var(--status-down);
}

.status-text {
  font-size: var(--text-xs);
  color: var(--text-secondary);
  margin-left: var(--space-1);
}
```

- [ ] **Step 6: Verify build passes**

Run: `cd frontend && npm run build`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add frontend/src/views/TenantList.vue
git commit -m "refactor: TenantList with 9 columns and improved styling"
```

---

## Phase 5: TenantDetail Refactoring

### Task 8: Create TenantDetail Fold Panel Layout

**Files:**
- Modify: `frontend/src/views/TenantDetail.vue`

**Interfaces:**
- Consumes: CSS tokens, Element Plus components
- Produces: 6 fold panels replacing 8 tabs

- [ ] **Step 1: Replace tabs with el-collapse**

```vue
<template>
  <div class="tenant-detail-page">
    <!-- Header -->
    <div class="detail-header">
      <div class="header-left">
        <el-button text @click="router.push('/tenants')"><el-icon><ArrowLeft /></el-icon> 返回</el-button>
        <h2>{{ tenant.userName || tenant.tenancyName || `租户 #${tenant.id}` }}</h2>
        <el-tag v-if="tenant.accountType" :type="tenant.accountType === 'PAYG' ? '' : 'info'" size="small">
          {{ tenant.accountType }}
        </el-tag>
        <el-tag v-if="tenant.isActive !== undefined" :type="tenant.isActive ? 'success' : 'danger'" size="small">
          {{ tenant.isActive ? '正常' : '停用' }}
        </el-tag>
      </div>
      <div class="header-right">
        <el-button size="small" @click="syncOci" :loading="syncing"><el-icon><Connection /></el-icon> 同步 OCI</el-button>
        <el-button size="small" @click="checkTenant" :loading="checking"><el-icon><Monitor /></el-icon> 测试存活</el-button>
      </div>
    </div>

    <el-alert v-if="checkResult" :type="checkResult.alive ? 'success' : 'error'" show-icon style="margin-bottom:12px"
      :title="checkResult.alive ? 'OCI 认证成功' : '异常: ' + (checkResult.error || '未知错误')" />

    <!-- Fold panels -->
    <el-collapse v-model="activePanels" accordion>
      <!-- 基本信息 -->
      <el-collapse-item name="overview">
        <template #title>
          <span class="panel-title">基本信息</span>
        </template>
        <!-- Overview content -->
      </el-collapse-item>

      <!-- 实例列表 -->
      <el-collapse-item name="instances">
        <template #title>
          <span class="panel-title">实例列表 ({{ instances.length }})</span>
        </template>
        <!-- Instances content -->
      </el-collapse-item>

      <!-- 费用账单 -->
      <el-collapse-item name="costs">
        <template #title>
          <span class="panel-title">费用账单</span>
        </template>
        <!-- Costs content -->
      </el-collapse-item>

      <!-- IAM 用户 -->
      <el-collapse-item name="users">
        <template #title>
          <span class="panel-title">IAM 用户 ({{ userList.length }})</span>
        </template>
        <!-- Users content -->
      </el-collapse-item>

      <!-- 安全与合规 -->
      <el-collapse-item name="security">
        <template #title>
          <span class="panel-title">安全与合规</span>
        </template>
        <!-- Security content -->
      </el-collapse-item>

      <!-- 区域管理 -->
      <el-collapse-item name="regions">
        <template #title>
          <span class="panel-title">区域管理</span>
        </template>
        <!-- Regions content -->
      </el-collapse-item>
    </el-collapse>
  </div>
</template>
```

- [ ] **Step 2: Update script setup**

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  ArrowLeft, Connection, Refresh, Monitor, Plus, Edit, Download, Delete, Search
} from '@element-plus/icons-vue'
import request from '../utils/request'

defineOptions({ name: 'tenant-detail' })

const route = useRoute()
const router = useRouter()
const tenantId = Number(route.params.id)

// Active panels (accordion mode)
const activePanels = ref('overview')

// ... rest of existing state and logic
</script>
```

- [ ] **Step 3: Add panel title styles**

```css
<style scoped>
.tenant-detail-page {
  padding: 0;
}

.detail-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-6);
  flex-wrap: wrap;
  gap: var(--space-3);
}

.header-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.header-left h2 {
  margin: 0;
  font-size: var(--text-xl);
  font-weight: var(--font-bold);
}

.header-right {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.panel-title {
  font-size: var(--text-md);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

:deep(.el-collapse-item__header) {
  font-size: var(--text-md);
  font-weight: var(--font-semibold);
}

:deep(.el-collapse-item__content) {
  padding-bottom: var(--space-4);
}
</style>
```

- [ ] **Step 4: Verify build passes**

Run: `cd frontend && npm run build`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/TenantDetail.vue
git commit -m "refactor: TenantDetail with fold panel layout"
```

---

### Task 9: Add Credential User Selector to TenantDetail

**Files:**
- Modify: `frontend/src/views/TenantDetail.vue`

**Interfaces:**
- Consumes: User list from TenantDetail
- Produces: Credential section with user selector

- [ ] **Step 1: Add credential user selector in security section**

```vue
<!-- 安全与合规 section -->
<el-collapse-item name="security">
  <template #title>
    <span class="panel-title">安全与合规</span>
  </template>

  <!-- 凭证管理 -->
  <div class="section-block">
    <h4 class="section-title">凭证管理</h4>
    <div class="credential-user-select">
      <span class="select-label">选择 IAM 用户:</span>
      <el-select v-model="credUserOcid" filterable placeholder="选择用户" @change="loadCredentials" style="width: 300px" size="small">
        <el-option v-for="u in userList" :key="u.ocid" :label="u.name" :value="u.ocid"/>
      </el-select>
    </div>

    <!-- API Keys -->
    <h5 class="subsection-title">API 密钥</h5>
    <el-table :data="apiKeys" v-loading="credLoading" border stripe size="small" max-height="200">
      <template #empty><el-empty description="暂无 API 密钥" :image-size="40"/></template>
      <el-table-column prop="fingerprint" label="指纹" min-width="200" show-overflow-tooltip/>
      <el-table-column label="操作" width="80" align="center">
        <template #default="{ row }">
          <el-button size="small" type="danger" text @click="deleteApiKey(row)">
            <el-icon><Delete/></el-icon>
          </el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-button size="small" type="primary" @click="apiKeyAddVisible=true" style="margin-top:8px">
      <el-icon><Plus/></el-icon> 添加 API 密钥
    </el-button>

    <!-- Auth Tokens -->
    <h5 class="subsection-title">Auth 令牌</h5>
    <!-- ... similar structure -->

    <!-- SMTP Credentials -->
    <h5 class="subsection-title">SMTP 凭证</h5>
    <!-- ... similar structure -->

    <!-- Customer Secret Keys -->
    <h5 class="subsection-title">Customer Secret Keys</h5>
    <!-- ... similar structure -->
  </div>

  <!-- ... other security sections -->
</el-collapse-item>
```

- [ ] **Step 2: Add section styles**

```css
.section-block {
  margin-bottom: var(--space-6);
}

.section-title {
  font-size: var(--text-md);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--space-4) 0;
  padding-bottom: var(--space-2);
  border-bottom: 1px solid var(--border-subtle);
}

.subsection-title {
  font-size: var(--text-sm);
  font-weight: var(--font-semibold);
  color: var(--text-secondary);
  margin: var(--space-4) 0 var(--space-2) 0;
}

.credential-user-select {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
}

.select-label {
  font-size: var(--text-sm);
  color: var(--text-secondary);
  font-weight: var(--font-medium);
}
```

- [ ] **Step 3: Verify build passes**

Run: `cd frontend && npm run build`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add frontend/src/views/TenantDetail.vue
git commit -m "feat: add credential user selector to TenantDetail"
```

---

## Phase 6: Instances Refactoring

### Task 10: Refactor Instances Table

**Files:**
- Modify: `frontend/src/views/Instances.vue`

**Interfaces:**
- Consumes: PageHeader, StatusBadge, MonoText components
- Produces: 7-column table with improved styling

- [ ] **Step 1: Import new components**

```vue
<script setup lang="ts">
import PageHeader from '../components/common/PageHeader.vue'
import StatusBadge from '../components/common/StatusBadge.vue'
import MonoText from '../components/common/MonoText.vue'
// ... existing imports
</script>
```

- [ ] **Step 2: Update template with PageHeader**

```vue
<template>
  <div class="instances-page">
    <!-- Toolbar -->
    <PageHeader title="实例管理">
      <template #extra>
        <el-tag type="info" size="small">{{ total }} 个实例</el-tag>
      </template>
      <template #actions>
        <el-button @click="load" :loading="loading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
        <el-dropdown @command="handleExport">
          <el-button>
            <el-icon><Download /></el-icon> 导出 <el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="all">导出全部实例</el-dropdown-item>
              <el-dropdown-item command="current-tenant" v-if="tenantFilter">导出当前租户</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </template>
    </PageHeader>

    <!-- Filter bar -->
    <!-- ... -->

    <!-- Table -->
    <el-card shadow="none" class="table-card">
      <el-table :data="rows" v-loading="loading" border stripe style="width: 100%">
        <template #empty>
          <el-empty description="暂无实例数据" :image-size="80" />
        </template>
        <el-table-column type="selection" width="40" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <StatusBadge
              :status="getStateStatus(row.state)"
              :label="row.state || '-'"
              :pulse="row.state === 'Running'"
            />
          </template>
        </el-table-column>
        <el-table-column prop="displayName" label="名称" min-width="160" sortable>
          <template #default="{ row }">
            <span class="instance-name" @click="showDetail(row)" style="cursor:pointer;color:var(--accent)">
              {{ row.displayName }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="tenantName" label="租户" min-width="120" />
        <el-table-column prop="publicIps" label="公网IP" width="140">
          <template #default="{ row }">
            <MonoText>{{ row.publicIps || '-' }}</MonoText>
          </template>
        </el-table-column>
        <el-table-column label="规格" width="120">
          <template #default="{ row }">
            <MonoText>{{ row.ocpus }}C / {{ row.memoryInGbs }}G</MonoText>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="showDetail(row)" title="详情">
              <el-icon><InfoFilled /></el-icon>
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- ... rest of template -->
  </div>
</template>
```

- [ ] **Step 3: Add state status function**

```vue
<script setup lang="ts">
function getStateStatus(state: string): 'up' | 'down' | 'warn' | 'idle' {
  const s = (state || '').toLowerCase()
  if (s === 'running') return 'up'
  if (s === 'stopped' || s === 'terminated') return 'down'
  if (s === 'starting' || s === 'stopping') return 'warn'
  return 'idle'
}
</script>
```

- [ ] **Step 4: Verify build passes**

Run: `cd frontend && npm run build`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/Instances.vue
git commit -m "refactor: Instances with 7 columns and improved styling"
```

---

## Phase 7: Settings Refactoring

### Task 11: Refactor Settings with Left Tabs

**Files:**
- Modify: `frontend/src/views/SystemSettings.vue`

**Interfaces:**
- Consumes: CSS tokens, Element Plus components
- Produces: Left tab + right content layout

- [ ] **Step 1: Restructure layout with el-tabs**

```vue
<template>
  <div class="settings-page">
    <PageHeader title="系统设置">
      <template #extra>
        <el-tag v-if="config.appVersion" type="info" size="small">v{{ config.appVersion }}</el-tag>
      </template>
      <template #actions>
        <el-button type="primary" @click="loadConfig" :loading="loading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </template>
    </PageHeader>

    <div class="settings-layout">
      <!-- Left tabs -->
      <el-tabs v-model="activeTab" tab-position="left" class="settings-tabs">
        <!-- 用户管理 -->
        <el-tab-pane label="👤 用户" name="user">
          <div class="tab-content">
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="当前用户">
                <el-tag type="primary" size="small">{{ user.username }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="用户角色">
                <el-tag type="danger" size="small">ADMIN</el-tag>
              </el-descriptions-item>
            </el-descriptions>
            <div style="margin-top: 16px;">
              <el-button type="warning" @click="openChangePassword">
                <el-icon><Lock /></el-icon> 修改密码
              </el-button>
            </div>
          </div>
        </el-tab-pane>

        <!-- 通知渠道 -->
        <el-tab-pane label="📢 通知" name="notification">
          <div class="tab-content">
            <el-row :gutter="16">
              <!-- Telegram, DingTalk, Bark, Feishu cards -->
            </el-row>
          </div>
        </el-tab-pane>

        <!-- DNS 服务 -->
        <el-tab-pane label="🌐 DNS" name="dns">
          <div class="tab-content">
            <el-row :gutter="16">
              <!-- Cloudflare, EdgeOne cards -->
            </el-row>
          </div>
        </el-tab-pane>

        <!-- SSL 证书 -->
        <el-tab-pane label="🔒 SSL" name="ssl">
          <div class="tab-content">
            <!-- SSL configuration -->
          </div>
        </el-tab-pane>

        <!-- 网络代理 -->
        <el-tab-pane label="🌐 代理" name="proxy">
          <div class="tab-content">
            <!-- Proxy configuration -->
          </div>
        </el-tab-pane>

        <!-- 安全与认证 -->
        <el-tab-pane label="🔐 安全" name="security">
          <div class="tab-content">
            <!-- MFA, Turnstile, GitHub OAuth -->
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Add layout styles**

```css
.settings-page {
  padding: 0;
}

.settings-layout {
  display: flex;
  gap: var(--space-6);
}

.settings-tabs {
  flex: 1;
}

.settings-tabs :deep(.el-tabs__header) {
  margin-right: var(--space-6);
}

.settings-tabs :deep(.el-tabs__item) {
  height: 44px;
  line-height: 44px;
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
}

.tab-content {
  min-height: 400px;
}

@media (max-width: 768px) {
  .settings-layout {
    flex-direction: column;
  }

  .settings-tabs :deep(.el-tabs__header) {
    margin-right: 0;
    margin-bottom: var(--space-4);
  }

  .settings-tabs :deep(.el-tabs__nav-wrap) {
    overflow-x: auto;
  }
}
```

- [ ] **Step 3: Add script setup for activeTab**

```vue
<script setup lang="ts">
import { ref } from 'vue'
import PageHeader from '../components/common/PageHeader.vue'
// ... existing imports

const activeTab = ref('user')
// ... rest of existing logic
</script>
```

- [ ] **Step 4: Verify build passes**

Run: `cd frontend && npm run build`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/SystemSettings.vue
git commit -m "refactor: Settings with left tab layout"
```

---

## Summary

| Task | Component/Page | Description |
|------|----------------|-------------|
| 1 | PageHeader | Extract page header component |
| 2 | StatusBadge | Extract status badge component |
| 3 | MonoText | Extract monospace text component |
| 4 | StatusCard | Extract stat card component |
| 5 | Default Layout | Add sidebar navigation groups |
| 6 | Dashboard | Refactor with StatusCards |
| 7 | TenantList | Refactor table columns |
| 8 | TenantDetail | Create fold panel layout |
| 9 | TenantDetail | Add credential user selector |
| 10 | Instances | Refactor table columns |
| 11 | Settings | Create left tab layout |

**Estimated Total Time:** 10-14 days

**Dependencies:**
- Tasks 1-4 can be done in parallel
- Task 5 depends on nothing
- Tasks 6-7 depend on Tasks 1-4
- Tasks 8-9 depend on nothing (can be parallel with 6-7)
- Task 10 depends on Tasks 1-4
- Task 11 depends on Task 1

**Testing:**
- Visual verification after each task
- Responsive testing at 3 breakpoints
- Cross-browser testing (Chrome, Firefox, Safari)
- Accessibility testing (keyboard navigation, screen reader)
