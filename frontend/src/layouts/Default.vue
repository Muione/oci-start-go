<template>
  <div class="shell">
    <!-- Sidebar -->
    <aside class="sidebar" :class="{ 'sidebar--collapsed': collapsed }">
      <!-- Accent line — the single decorative element in the chrome -->
      <div class="sidebar-accent"></div>

      <!-- Logo -->
      <div class="brand">
        <div class="brand-mark" :class="{ 'brand-mark--compact': collapsed }">
          <svg width="28" height="28" viewBox="0 0 28 28" fill="none">
            <rect x="2" y="2" width="10" height="10" rx="2" stroke="currentColor" stroke-width="1.5" />
            <rect x="16" y="2" width="10" height="10" rx="2" stroke="currentColor" stroke-width="1.5" />
            <rect x="2" y="16" width="10" height="10" rx="2" stroke="currentColor" stroke-width="1.5" />
            <rect x="16" y="16" width="10" height="10" rx="2" stroke="currentColor" stroke-width="1.5" />
            <circle cx="7" cy="7" r="2" fill="currentColor" class="pulse-node" />
            <circle cx="21" cy="21" r="2" fill="currentColor" />
          </svg>
        </div>
        <transition name="fade-text">
          <span v-if="!collapsed" class="brand-name">oci-start</span>
        </transition>
      </div>

      <!-- Navigation -->
      <nav class="nav">
        <router-link
          v-for="item in navItems"
          :key="item.path"
          :to="item.path"
          class="nav-item"
          :class="{ 'nav-item--active': isActive(item.path) }"
        >
          <span class="nav-icon">
            <el-icon :size="18"><component :is="item.icon" /></el-icon>
          </span>
          <transition name="fade-text">
            <span v-if="!collapsed" class="nav-label">{{ item.label }}</span>
          </transition>
          <transition name="fade-text">
            <span v-if="!collapsed && item.count !== undefined && item.count > 0" class="nav-badge">
              {{ item.count }}
            </span>
          </transition>
        </router-link>
      </nav>

      <!-- Sidebar footer -->
      <div class="sidebar-footer">
        <button class="collapse-trigger" @click="collapsed = !collapsed" :title="collapsed ? '展开' : '收起'">
          <el-icon :size="16">
            <DArrowLeft v-if="!collapsed" />
            <DArrowRight v-else />
          </el-icon>
        </button>
      </div>
    </aside>

    <!-- Main area -->
    <div class="main">
      <!-- Header -->
      <header class="topbar">
        <div class="topbar-left">
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{ path: '/' }">首页</el-breadcrumb-item>
            <el-breadcrumb-item v-if="pageTitle">{{ pageTitle }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        <div class="topbar-right">
          <span class="topbar-user">{{ user.username }}</span>
          <button class="topbar-logout" @click="logout" title="退出登录">
            <el-icon :size="16"><SwitchButton /></el-icon>
          </button>
        </div>
      </header>

      <!-- Content -->
      <main class="content">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Monitor, Connection, Setting, VideoPlay, Operation,
  Download, WarningFilled, DataBoard, Platform, Coin,
  SwitchButton, DArrowLeft, DArrowRight, Folder, Message,
} from '@element-plus/icons-vue'
import { useUserStore } from '../store/user'
import request from '../utils/request'

const route = useRoute()
const router = useRouter()
const user = useUserStore()
const collapsed = ref(false)

interface NavItem {
  path: string
  label: string
  icon: any
  count?: number
}

const navItems: NavItem[] = [
  { path: '/',           label: '仪表盘',     icon: DataBoard },
  { path: '/tenants',    label: '租户管理',   icon: Connection },
  { path: '/proxies',    label: '代理管理',   icon: Platform },
  { path: '/boot',       label: '抢机任务',   icon: VideoPlay },
  { path: '/instances',  label: '实例管理',   icon: Monitor },
  { path: '/vnic',       label: 'VNIC 管理',  icon: Connection },
  { path: '/email',      label: '邮件管理',   icon: Message },
  { path: '/nginx',      label: 'Nginx 管理', icon: Setting },
  { path: '/dns',        label: 'DNS 管理',   icon: Coin },
  { path: '/storage',    label: '对象存储',   icon: Folder },
  { path: '/terminal',   label: 'SSH 终端',   icon: Operation },
  { path: '/console',    label: 'VNC 控制台', icon: Monitor },
  { path: '/rescue',     label: '实例救援',   icon: WarningFilled },
  { path: '/migration',  label: '数据迁移',   icon: Download },
  { path: '/settings',   label: '系统设置',   icon: Setting },
]

function isActive(path: string): boolean {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}

const pageTitle = computed(() => {
  const titles: Record<string, string> = {
    dashboard: '仪表盘', tenants: '租户管理', proxies: '代理管理',
    boot: '抢机任务', instances: '实例管理', vnic: 'VNIC 管理',
    email: '邮件管理', nginx: 'Nginx 管理', dns: 'DNS 管理', storage: '对象存储', terminal: 'SSH 终端', console: 'VNC 控制台',
    rescue: '实例救援', migration: '数据迁移', settings: '系统设置',
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
/* ============================================================
   Shell — full-height layout
   ============================================================ */
.shell {
  display: flex;
  height: 100vh;
  overflow: hidden;
  background: var(--bg-page);
}

/* ============================================================
   Sidebar
   ============================================================ */

.sidebar {
  width: 220px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background:
    linear-gradient(180deg, var(--bg-root) 0%, #111418 100%);
  border-right: 1px solid var(--border-subtle);
  transition: width var(--transition-slow);
  overflow: hidden;
  position: relative;
}

.sidebar--collapsed {
  width: 60px;
}

/* The single decorative accent in the entire chrome */
.sidebar-accent {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 2px;
  background: linear-gradient(90deg, var(--accent), rgba(129, 153, 240, 0.1));
  opacity: 0.8;
}

/* --- Brand --- */
.brand {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  height: 56px;
  padding: 0 var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
  flex-shrink: 0;
}

.brand-mark {
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--accent);
  flex-shrink: 0;
  transition: color var(--transition-normal);
}

.brand-mark--compact {
  margin: 0 auto;
}

.brand-name {
  font-size: var(--text-md);
  font-weight: var(--font-bold);
  color: var(--text-primary);
  letter-spacing: var(--tracking-tight);
  white-space: nowrap;
}

/* Subtle pulse on the first node of the logo */
.pulse-node {
  animation: node-pulse 3s ease-in-out infinite;
}

@keyframes node-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

/* --- Navigation --- */
.nav {
  flex: 1;
  padding: var(--space-2) var(--space-2);
  overflow-y: auto;
  overflow-x: hidden;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  height: 40px;
  padding: 0 var(--space-3);
  margin-bottom: 2px;
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  text-decoration: none;
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  transition: background var(--transition-fast),
              color var(--transition-fast),
              transform var(--transition-fast);
  position: relative;
  white-space: nowrap;
}

.nav-item:hover {
  background: var(--bg-raised);
  color: var(--text-primary);
}

.nav-item--active {
  background: var(--accent-subtle);
  color: var(--accent);
}

.nav-item--active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 8px;
  bottom: 8px;
  width: 3px;
  border-radius: 0 2px 2px 0;
  background: var(--accent);
}

.nav-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  flex-shrink: 0;
}

.nav-label {
  flex: 1;
}

.nav-badge {
  font-size: var(--text-2xs);
  font-weight: var(--font-bold);
  color: var(--text-on-accent);
  background: var(--accent);
  min-width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-full);
  padding: 0 5px;
  line-height: 1;
  letter-spacing: var(--tracking-normal);
}

/* --- Sidebar footer --- */
.sidebar-footer {
  padding: var(--space-2);
  border-top: 1px solid var(--border-subtle);
  flex-shrink: 0;
}

.collapse-trigger {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 34px;
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  transition: background var(--transition-fast), color var(--transition-fast);
}

.collapse-trigger:hover {
  background: var(--bg-raised);
  color: var(--text-secondary);
}

/* ============================================================
   Main area
   ============================================================ */

.main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  background: var(--bg-page);
}

/* --- Topbar --- */
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 48px;
  padding: 0 var(--space-5);
  background: var(--bg-root);
  border-bottom: 1px solid var(--border-subtle);
  flex-shrink: 0;
  z-index: 10;
}

.topbar-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.topbar-user {
  font-size: var(--text-sm);
  color: var(--text-secondary);
  font-weight: var(--font-medium);
}

.topbar-logout {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  transition: background var(--transition-fast), color var(--transition-fast);
}

.topbar-logout:hover {
  background: var(--bg-raised);
  color: var(--status-down);
}

/* --- Content --- */
.content {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: var(--space-6);
  background: var(--bg-page);
}

/* ============================================================
   Transitions
   ============================================================ */

.fade-text-enter-active,
.fade-text-leave-active {
  transition: opacity var(--transition-normal);
}
.fade-text-enter-from,
.fade-text-leave-to {
  opacity: 0;
}

/* ============================================================
   Responsive
   ============================================================ */

@media (max-width: 768px) {
  .sidebar {
    width: 60px;
    position: fixed;
    left: 0;
    top: 0;
    bottom: 0;
    z-index: 50;
  }

  .sidebar--collapsed {
    width: 0;
    overflow: hidden;
  }

  .content {
    padding: var(--space-4);
  }

  .topbar {
    padding: 0 var(--space-4);
  }

  .topbar-user {
    display: none;
  }
}
</style>
