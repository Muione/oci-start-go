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
