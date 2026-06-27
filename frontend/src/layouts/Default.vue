<template>
  <el-container class="layout">
    <el-aside :width="collapsed ? '64px' : '210px'" class="aside">
      <div class="logo">oci-start</div>
      <el-menu :collapse="collapsed" :default-active="route.path" router background-color="#001529" text-color="#cfd8e6" active-text-color="#fff">
        <el-menu-item index="/">
          <el-icon><Monitor /></el-icon>
          <template #title>仪表盘</template>
        </el-menu-item>
        <el-menu-item index="/tenants">
          <el-icon><Connection /></el-icon>
          <template #title>租户管理</template>
        </el-menu-item>
        <el-menu-item index="/proxies">
          <el-icon><Setting /></el-icon>
          <template #title>代理管理</template>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <el-button text @click="collapsed = !collapsed">
          <el-icon><Fold /></el-icon>
        </el-button>
        <span class="user">{{ user.username }}</span>
        <el-button text @click="logout">退出</el-button>
      </el-header>
      <el-main>
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Monitor, Fold, Connection, Setting } from '@element-plus/icons-vue'
import { useUserStore } from '../store/user'
import request from '../utils/request'

const route = useRoute()
const router = useRouter()
const user = useUserStore()
const collapsed = ref(false)

async function logout() {
  try {
    await request.post('/api/logout')
  } catch {
    /* ignore */
  }
  user.clear()
  router.push('/login')
}
</script>

<style scoped>
.layout { height: 100vh; }
.aside { background: #001529; transition: width 0.2s; overflow: hidden; }
.logo { color: #fff; font-size: 18px; font-weight: 600; padding: 16px; text-align: center; white-space: nowrap; }
.header { display: flex; align-items: center; gap: 12px; border-bottom: 1px solid #eee; background: #fff; }
.user { margin-left: auto; color: #333; }
</style>
