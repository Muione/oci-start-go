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
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  if (!to.meta.requiresAuth) return true
  const user = useUserStore()
  if (!user.username) {
    try {
      await user.fetchUserInfo()
    } catch {
      user.clear()
    }
  }
  if (!user.username) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  return true
})

export default router
