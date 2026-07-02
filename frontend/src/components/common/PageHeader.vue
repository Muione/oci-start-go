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
