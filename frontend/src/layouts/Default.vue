<template>
  <el-container class="layout">
    <el-aside :width="collapsed ? '64px' : '220px'" class="aside">
      <div class="logo">
        <span v-if="!collapsed" class="logo-text">☁️ oci-start</span>
        <span v-else class="logo-icon">☁️</span>
      </div>
      <el-menu
        :collapse="collapsed"
        :default-active="route.path"
        router
        background-color="#001529"
        text-color="#a3b1cc"
        active-text-color="#fff"
        class="side-menu"
      >
        <el-menu-item index="/">
          <el-icon><DataBoard /></el-icon>
          <template #title>仪表盘</template>
        </el-menu-item>
        <el-menu-item index="/tenants">
          <el-icon><Connection /></el-icon>
          <template #title>租户管理</template>
        </el-menu-item>
        <el-menu-item index="/proxies">
          <el-icon><Platform /></el-icon>
          <template #title>代理管理</template>
        </el-menu-item>
        <el-menu-item index="/boot">
          <el-icon><VideoPlay /></el-icon>
          <template #title>抢机任务</template>
        </el-menu-item>
        <el-menu-item index="/instances">
          <el-icon><Monitor /></el-icon>
          <template #title>实例管理</template>
        </el-menu-item>
        <el-menu-item index="/dns">
          <el-icon><Coin /></el-icon>
          <template #title>DNS 管理</template>
        </el-menu-item>
        <el-menu-item index="/terminal">
          <el-icon><Operation /></el-icon>
          <template #title>SSH终端</template>
        </el-menu-item>
        <el-menu-item index="/console">
          <el-icon><Monitor /></el-icon>
          <template #title>VNC控制台</template>
        </el-menu-item>
        <el-menu-item index="/rescue">
          <el-icon><WarningFilled /></el-icon>
          <template #title>实例救援</template>
        </el-menu-item>
        <el-menu-item index="/migration">
          <el-icon><Download /></el-icon>
          <template #title>数据迁移</template>
        </el-menu-item>
        <el-menu-item index="/settings">
          <el-icon><Setting /></el-icon>
          <template #title>系统设置</template>
        </el-menu-item>
      </el-menu>
    </el-aside>

    <el-container class="main-container">
      <el-header class="header">
        <el-button text class="collapse-btn" @click="collapsed = !collapsed">
          <el-icon :size="18"><Fold /></el-icon>
        </el-button>
        <el-breadcrumb separator="/" class="breadcrumb">
          <el-breadcrumb-item :to="{ path: '/' }">首页</el-breadcrumb-item>
          <el-breadcrumb-item v-if="pageTitle">{{ pageTitle }}</el-breadcrumb-item>
        </el-breadcrumb>
        <div class="header-right">
          <el-tag type="primary" size="small" effect="plain">{{ user.username }}</el-tag>
          <el-button text @click="logout" class="logout-btn">
            <el-icon><SwitchButton /></el-icon>
          </el-button>
        </div>
      </el-header>
      <el-main class="main-content">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Monitor, Fold, Connection, Setting, VideoPlay, Operation,
  Download, WarningFilled, DataBoard, Platform, Coin, SwitchButton,
} from '@element-plus/icons-vue'
import { useUserStore } from '../store/user'
import request from '../utils/request'

const route = useRoute()
const router = useRouter()
const user = useUserStore()
const collapsed = ref(false)

const pageTitle = computed(() => {
  const titles: Record<string, string> = {
    dashboard: '仪表盘', tenants: '租户管理', proxies: '代理管理',
    boot: '抢机任务', instances: '实例管理', dns: 'DNS 管理',
    terminal: 'SSH终端', console: 'VNC控制台', rescue: '实例救援',
    migration: '数据迁移', settings: '系统设置',
  }
  return (route.name && titles[route.name as string]) || ''
})

async function logout() {
  try { await request.post('/api/logout') } catch { /* ignore */ }
  user.clear()
  router.push('/login')
}
</script>

<style scoped>
.layout { height: 100vh; }
.aside {
  background: #001529;
  transition: width 0.25s ease;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  border-right: 1px solid rgba(255,255,255,.05);
}
.logo {
  display: flex; align-items: center; justify-content: center;
  height: 56px; color: #fff; font-size: 17px; font-weight: 700;
  user-select: none; border-bottom: 1px solid rgba(255,255,255,.08);
}
.logo-text { letter-spacing: 1px; }
.logo-icon { font-size: 22px; }
.side-menu { border-right: none; flex: 1; overflow-y: auto; }
.side-menu :deep(.el-menu-item) {
  margin: 2px 8px; border-radius: 8px; transition: all 0.15s;
}
.side-menu :deep(.el-menu-item.is-active) {
  background: linear-gradient(135deg, #1890ff, #096dd9) !important;
  color: #fff !important;
}
.side-menu :deep(.el-menu-item:hover) {
  background: rgba(255,255,255,.06) !important;
}

.main-container { flex-direction: column; }
.header {
  display: flex; align-items: center; gap: 16px;
  background: #fff; border-bottom: 1px solid #f0f0f0;
  height: 56px; padding: 0 20px; box-shadow: 0 1px 4px rgba(0,0,0,.04);
  z-index: 10;
}
.collapse-btn { font-size: 18px; }
.breadcrumb { flex: 1; }
.header-right { display: flex; align-items: center; gap: 12px; }
.logout-btn { font-size: 17px; color: #909399; }
.logout-btn:hover { color: #f56c6c; }

.main-content {
  background: #f0f2f5; padding: 20px;
  overflow-y: auto; height: calc(100vh - 56px);
}
</style>
