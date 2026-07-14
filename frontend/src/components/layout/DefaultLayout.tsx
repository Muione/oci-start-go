import { useEffect } from 'react'
import { Layout, Breadcrumb, Button, Dropdown } from 'antd'
import { LogoutOutlined, UserOutlined } from '@ant-design/icons'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useUserStore } from '@/store/useUserStore'
import { useAuth } from '@/hooks/useAuth'
import Sidebar from './Sidebar'

const { Header, Content } = Layout

const ROUTE_LABELS: Record<string, string> = {
  '/': 'nav.dashboard',
  '/tenants': 'nav.tenants',
  '/instances': 'nav.instances',
  '/vnic': 'nav.vnic',
  '/storage': 'nav.storage',
  '/boot': 'nav.boot',
  '/dns': 'nav.dns',
  '/migration': 'nav.migration',
  '/rescue': 'nav.rescue',
  '/terminal': 'nav.ssh',
  '/console': 'nav.console',
  '/settings': 'nav.settings',
}

export default function DefaultLayout() {
  const { t } = useTranslation()
  const location = useLocation()
  const navigate = useNavigate()
  const { username } = useUserStore()
  const { logout, fetchUserInfo } = useAuth()

  useEffect(() => {
    fetchUserInfo().catch(() => {
      // Will be caught by 401 interceptor
    })
  }, [])

  // Listen for 401 events from axios interceptor
  useEffect(() => {
    const handler = () => navigate('/login')
    window.addEventListener('auth:unauthorized', handler)
    return () => window.removeEventListener('auth:unauthorized', handler)
  }, [navigate])

  const currentLabel = ROUTE_LABELS[location.pathname]
  const breadcrumbItems = [
    { title: t('common.dashboard'), href: '/' },
    ...(currentLabel && currentLabel !== 'nav.dashboard'
      ? [{ title: t(currentLabel) }]
      : []),
  ]

  const handleLogout = async () => {
    await logout()
  }

  const userMenuItems = [
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: t('common.logout'),
      onClick: handleLogout,
    },
  ]

  return (
    <Layout className="min-h-screen">
      <Sidebar />
      <Layout>
        <Header className="bg-white px-6 flex items-center justify-between shadow-sm">
          <Breadcrumb items={breadcrumbItems} />
          <div className="flex items-center gap-4">
            <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
              <Button type="text" icon={<UserOutlined />}>
                {username || t('common.login')}
              </Button>
            </Dropdown>
          </div>
        </Header>
        <Content className="m-6">
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
