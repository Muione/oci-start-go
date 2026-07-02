<template>
  <div style="padding:40px;text-align:center;color:var(--text-secondary)">
    <h2>租户详情 — {{ $route.params.id }}</h2>
    <p>Phase 2: 详情页（多 tab）将在此实现。</p>
    <el-button @click="$router.push('/tenants')">返回租户列表</el-button>
  </div>
</template>
<script setup lang="ts">
defineOptions({ name: 'tenant-detail' })
</script>
