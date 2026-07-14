import { useState } from 'react'
import { Layout, Menu } from 'antd'
import {
  DashboardOutlined,
  TeamOutlined,
  CloudServerOutlined,
  ApiOutlined,
  FolderOutlined,
  ThunderboltOutlined,
  GlobalOutlined,
  DownloadOutlined,
  WarningOutlined,
  CodeOutlined,
  DesktopOutlined,
  SettingOutlined,
  SwapOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate } from 'react-router-dom'

const { Sider } = Layout

const ROUTE_MAP: Record<string, string> = {
  dashboard: '/',
  tenants: '/tenants',
  instances: '/instances',
  vnic: '/vnic',
  storage: '/storage',
  boot: '/boot',
  dns: '/dns',
  proxies: '/proxies',
  migration: '/migration',
  rescue: '/rescue',
  ssh: '/terminal',
  console: '/console',
  settings: '/settings',
}

export default function Sidebar() {
  const { t } = useTranslation()
  const location = useLocation()
  const navigate = useNavigate()
  const [collapsed, setCollapsed] = useState(false)

  const menuItems = [
    {
      key: 'overview',
      label: t('nav.overview'),
      type: 'group' as const,
      children: [
        {
          key: 'dashboard',
          icon: <DashboardOutlined />,
          label: t('nav.dashboard'),
        },
      ],
    },
    {
      key: 'resources',
      label: t('nav.resources'),
      type: 'group' as const,
      children: [
        {
          key: 'tenants',
          icon: <TeamOutlined />,
          label: t('nav.tenants'),
        },
        {
          key: 'instances',
          icon: <CloudServerOutlined />,
          label: t('nav.instances'),
        },
        {
          key: 'vnic',
          icon: <ApiOutlined />,
          label: t('nav.vnic'),
        },
        {
          key: 'storage',
          icon: <FolderOutlined />,
          label: t('nav.storage'),
        },
      ],
    },
    {
      key: 'ops',
      label: t('nav.ops'),
      type: 'group' as const,
      children: [
        {
          key: 'boot',
          icon: <ThunderboltOutlined />,
          label: t('nav.boot'),
        },
        {
          key: 'dns',
          icon: <GlobalOutlined />,
          label: t('nav.dns'),
        },
        {
          key: 'proxies',
          icon: <SwapOutlined />,
          label: t('nav.proxies'),
        },
        {
          key: 'migration',
          icon: <DownloadOutlined />,
          label: t('nav.migration'),
        },
        {
          key: 'rescue',
          icon: <WarningOutlined />,
          label: t('nav.rescue'),
        },
      ],
    },
    {
      key: 'terminal',
      label: t('nav.terminal'),
      type: 'group' as const,
      children: [
        {
          key: 'ssh',
          icon: <CodeOutlined />,
          label: t('nav.ssh'),
        },
        {
          key: 'console',
          icon: <DesktopOutlined />,
          label: t('nav.console'),
        },
      ],
    },
  ]

  // Determine active key from current path
  const activeKey = Object.entries(ROUTE_MAP).find(
    ([, path]) => location.pathname === path,
  )?.[0] ?? 'dashboard'

  // Determine which submenus to open based on active key
  const groupKeys = ['overview', 'resources', 'ops', 'terminal']
  const openKeys = groupKeys.filter((group) =>
    menuItems
      .find((item) => item.key === group)
      ?.children?.some((child) => child.key === activeKey),
  )

  return (
    <Sider
      collapsible
      collapsed={collapsed}
      onCollapse={setCollapsed}
      theme="dark"
      width={220}
      className="min-h-screen"
    >
      <div className="flex items-center justify-center h-16 text-white text-lg font-bold tracking-wide">
        {collapsed ? 'OS' : 'oci-start'}
      </div>
      <Menu
        theme="dark"
        mode="inline"
        selectedKeys={[activeKey]}
        defaultOpenKeys={openKeys}
        items={menuItems}
        onClick={({ key }) => {
          const path = ROUTE_MAP[key]
          if (path) navigate(path)
        }}
      />
      <div className="absolute bottom-12 left-0 right-0">
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={activeKey === 'settings' ? ['settings'] : []}
          items={[
            {
              key: 'settings',
              icon: <SettingOutlined />,
              label: t('nav.settings'),
            },
          ]}
          onClick={() => navigate('/settings')}
        />
      </div>
    </Sider>
  )
}
