import { useState, type ReactNode } from 'react'
import { Tabs } from 'antd'
import {
  UserOutlined,
  BellOutlined,
  CloudOutlined,
  LockOutlined,
  GlobalOutlined,
  SafetyOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import PageHeader from '@/components/common/PageHeader'
import UserSettings from './tabs/UserSettings'
import NotificationSettings from './tabs/NotificationSettings'
import DnsSettings from './tabs/DnsSettings'
import SslSettings from './tabs/SslSettings'
import ProxySettings from './tabs/ProxySettings'
import SecuritySettings from './tabs/SecuritySettings'

const TAB_ITEMS = [
  { key: 'user', icon: <UserOutlined />, labelKey: 'settings.tabUser' },
  { key: 'notification', icon: <BellOutlined />, labelKey: 'settings.tabNotification' },
  { key: 'dns', icon: <CloudOutlined />, labelKey: 'settings.tabDns' },
  { key: 'ssl', icon: <LockOutlined />, labelKey: 'settings.tabSsl' },
  { key: 'proxy', icon: <GlobalOutlined />, labelKey: 'settings.tabProxy' },
  { key: 'security', icon: <SafetyOutlined />, labelKey: 'settings.tabSecurity' },
]

const TAB_PANES: Record<string, () => ReactNode> = {
  user: UserSettings,
  notification: NotificationSettings,
  dns: DnsSettings,
  ssl: SslSettings,
  proxy: ProxySettings,
  security: SecuritySettings,
}

export default function SystemSettings() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState('user')

  const items = TAB_ITEMS.map((tab) => ({
    key: tab.key,
    label: (
      <span>
        {tab.icon}
        <span className="ml-1">{t(tab.labelKey)}</span>
      </span>
    ),
    children: (() => {
      const Component = TAB_PANES[tab.key]
      return Component ? <Component /> : null
    })(),
  }))

  return (
    <div>
      <PageHeader title={t('settings.title')} />
      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        tabPosition="left"
        items={items}
        className="settings-tabs"
      />
    </div>
  )
}
