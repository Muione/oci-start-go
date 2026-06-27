import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '../store/user'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('../views/Login.vue') },
    { path: '/first-user', name: 'first-user', component: () => import('../views/FirstUser.vue') },
    {
      path: '/',
      component: () => import('../layouts/Default.vue'),
      meta: { requiresAuth: true },
      children: [
        { path: '', name: 'dashboard', component: () => import('../views/Dashboard.vue') },
        { path: 'tenants', name: 'tenants', component: () => import('../views/Tenants.vue') },
        { path: 'proxies', name: 'proxies', component: () => import('../views/ProxyManager.vue') },
        { path: 'boot', name: 'boot', component: () => import('../views/BootTasks.vue') },
        { path: 'instances', name: 'instances', component: () => import('../views/Instances.vue') },
        { path: 'terminal', name: 'terminal', component: () => import('../views/SSHTerminal.vue') },
        { path: 'settings', name: 'settings', component: () => import('../views/SystemSettings.vue') },
        { path: 'migration', name: 'migration', component: () => import('../views/Migration.vue') },
        { path: 'console', name: 'console', component: () => import('../views/Console.vue') },
        { path: 'dns', name: 'dns', component: () => import('../views/DnsRecords.vue') },
        { path: 'rescue', name: 'rescue', component: () => import('../views/Rescue.vue') },
      ],
    },
  ],
})

const guestRoutes = ['/login', '/first-user']

router.beforeEach(async (to) => {
  const user = useUserStore()

  // Fetch user info if not already loaded.
  if (!user.username) {
    try {
      await user.fetchUserInfo()
    } catch {
      user.clear()
    }
  }

  // Already logged in — skip guest pages (login, first-user).
  if (user.username && guestRoutes.includes(to.path)) {
    return { path: '/' }
  }

  // Auth-required routes — redirect to login if not authenticated.
  if (to.meta.requiresAuth && !user.username) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }

  return true
})

export default router
