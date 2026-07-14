import { lazy, Suspense, useEffect, type ReactNode } from 'react'
import { Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { Spin } from 'antd'
import { useUserStore } from '@/store/useUserStore'
import DefaultLayout from '@/components/layout/DefaultLayout'
import Login from '@/pages/auth/Login'
import FirstUser from '@/pages/auth/FirstUser'

// ── Lazy-loaded page components (code splitting) ──────────────────────
const Dashboard = lazy(() => import('@/pages/Dashboard'))
const TenantList = lazy(() => import('@/pages/tenants/TenantList'))
const TenantDetail = lazy(() => import('@/pages/tenants/TenantDetail'))
const InstanceList = lazy(() => import('@/pages/instances/InstanceList'))
const InstanceDetail = lazy(() => import('@/pages/instances/InstanceDetail'))
const SSHTerminal = lazy(() => import('@/pages/terminal/SSHTerminal'))
const Console = lazy(() => import('@/pages/terminal/Console'))
const DnsRecords = lazy(() => import('@/pages/dns/DnsRecords'))
const ObjectStorage = lazy(() => import('@/pages/storage/ObjectStorage'))
const VnicManagement = lazy(() => import('@/pages/vnic/VnicManagement'))
const BootTasks = lazy(() => import('@/pages/boot/BootTasks'))
const Rescue = lazy(() => import('@/pages/rescue/Rescue'))
const SystemSettings = lazy(() => import('@/pages/settings/SystemSettings'))
const ProxyManager = lazy(() => import('@/pages/proxy/ProxyManager'))
const Migration = lazy(() => import('@/pages/migration/Migration'))

function PageLoader() {
  return (
    <div className="flex items-center justify-center h-64">
      <Spin size="large" />
    </div>
  )
}

// Route guard: redirect to login if not authenticated
function ProtectedRoute({ children }: { children: ReactNode }) {
  const { isLoggedIn, authChecked } = useUserStore()
  const location = useLocation()

  if (!authChecked) {
    return null
  }

  if (!isLoggedIn) {
    return <Navigate to={`/login?redirect=${encodeURIComponent(location.pathname)}`} replace />
  }

  return <>{children}</>
}

// Route guard: redirect to home if already authenticated
function GuestRoute({ children }: { children: ReactNode }) {
  const { isLoggedIn, authChecked } = useUserStore()

  if (!authChecked) {
    return null
  }

  if (isLoggedIn) {
    return <Navigate to="/" replace />
  }

  return <>{children}</>
}

export default function App() {
  const checkAuth = useUserStore((s) => s.checkAuth)

  useEffect(() => {
    checkAuth()
  }, [checkAuth])

  return (
    <Suspense fallback={<PageLoader />}>
      <Routes>
        {/* Public routes */}
        <Route
          path="/login"
          element={
            <GuestRoute>
              <Login />
            </GuestRoute>
          }
        />
        {/* First-user page is always accessible (not guarded) */}
        <Route path="/first-user" element={<FirstUser />} />

        {/* Protected routes */}
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <DefaultLayout />
            </ProtectedRoute>
          }
        >
          <Route index element={<Dashboard />} />
          <Route path="tenants" element={<TenantList />} />
          <Route path="tenants/:id" element={<TenantDetail />} />
          <Route path="instances" element={<InstanceList />} />
          <Route path="instances/:id" element={<InstanceDetail />} />
          <Route path="vnic" element={<VnicManagement />} />
          <Route path="storage" element={<ObjectStorage />} />
          <Route path="boot" element={<BootTasks />} />
          <Route path="dns" element={<DnsRecords />} />
          <Route path="migration" element={<Migration />} />
          <Route path="rescue" element={<Rescue />} />
          <Route path="terminal" element={<SSHTerminal />} />
          <Route path="console" element={<Console />} />
          <Route path="settings" element={<SystemSettings />} />
          <Route path="proxies" element={<ProxyManager />} />
        </Route>

        {/* Fallback */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Suspense>
  )
}
