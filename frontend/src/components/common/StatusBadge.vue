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
