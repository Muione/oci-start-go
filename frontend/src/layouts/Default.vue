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
.layout {
  height: 100vh;
  background: #f5f7fa;
}

.aside {
  background: linear-gradient(135deg, #0a0e27 0%, #001529 100%);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  border-right: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 2px 0 12px rgba(0, 0, 0, 0.15);
}

.logo {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 60px;
  color: #fff;
  font-size: 17px;
  font-weight: 700;
  user-select: none;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  margin-bottom: 8px;
  transition: all 0.3s ease;
}

.logo-text {
  letter-spacing: 1px;
  background: linear-gradient(135deg, #3385ff, #00bcd4);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  font-size: 18px;
  font-weight: 800;
}

.logo-icon {
  font-size: 28px;
  animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
}

@keyframes pulse {
  0%, 100% {
    opacity: 1;
  }
  50% {
    opacity: 0.8;
  }
}

.side-menu {
  border-right: none;
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
}

.side-menu :deep(.el-menu-item) {
  margin: 4px 8px;
  border-radius: 10px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  color: #a3b1cc;
  font-weight: 500;
}

.side-menu :deep(.el-menu-item:hover) {
  background: rgba(255, 255, 255, 0.1) !important;
  color: #fff;
  transform: translateX(4px);
}

.side-menu :deep(.el-menu-item.is-active) {
  background: linear-gradient(90deg, rgba(0, 102, 255, 0.3), rgba(0, 188, 212, 0.1)) !important;
  color: #3385ff !important;
  border-right: 3px solid #0066ff;
  font-weight: 600;
}

.side-menu :deep(.el-icon) {
  margin-right: 8px;
  transition: all 0.3s ease;
}

.main-container {
  flex-direction: column;
  background: #f5f7fa;
}

.header {
  display: flex;
  align-items: center;
  gap: 16px;
  background: linear-gradient(90deg, #ffffff, #f8fafc);
  border-bottom: 1px solid rgba(0, 102, 255, 0.1);
  height: 60px;
  padding: 0 24px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  z-index: 10;
  transition: all 0.3s ease;
}

.collapse-btn {
  font-size: 18px;
  color: #0066ff;
  transition: all 0.3s ease;
  flex-shrink: 0;
}

.collapse-btn:hover {
  color: #00bcd4;
  transform: rotate(90deg);
}

.breadcrumb {
  flex: 1;
  margin-left: 16px;
}

.breadcrumb :deep(.el-breadcrumb__inner) {
  color: #1e293b;
  font-weight: 500;
}

.breadcrumb :deep(.el-breadcrumb__separator) {
  color: #cbd5e1;
  font-weight: 600;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-shrink: 0;
}

.header-right :deep(.el-tag) {
  border-radius: 8px;
  font-weight: 600;
  background: rgba(0, 102, 255, 0.1);
  color: #0066ff;
  border: none;
  padding: 6px 12px;
}

.logout-btn {
  font-size: 18px;
  color: #94a3b8;
  transition: all 0.3s ease;
}

.logout-btn:hover {
  color: #ef4444;
  transform: scale(1.1);
}

.main-content {
  background: linear-gradient(135deg, #f5f7fa 0%, #e9ecef 100%);
  padding: 24px;
  overflow-y: auto;
  height: calc(100vh - 60px);
  flex: 1;
}

.main-content::-webkit-scrollbar {
  width: 8px;
}

.main-content::-webkit-scrollbar-track {
  background: transparent;
}

.main-content::-webkit-scrollbar-thumb {
  background: rgba(0, 102, 255, 0.3);
  border-radius: 4px;
  transition: background 0.3s ease;
}

.main-content::-webkit-scrollbar-thumb:hover {
  background: rgba(0, 102, 255, 0.5);
}

/* Responsive */
@media (max-width: 768px) {
  .logo {
    height: 50px;
  }

  .header {
    height: 50px;
    padding: 0 16px;
  }

  .main-content {
    height: calc(100vh - 50px);
    padding: 16px;
  }

  .header-right {
    gap: 8px;
  }
}
</style>
