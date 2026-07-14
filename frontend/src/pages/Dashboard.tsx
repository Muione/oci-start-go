import { useCallback, useEffect, useState } from 'react'
import { Card, Col, Row, Statistic, Typography, Button, Space, Tag } from 'antd'
import {
  ApiOutlined,
  TeamOutlined,
  CheckCircleOutlined,
  SafetyOutlined,
  CopyOutlined,
  ReloadOutlined,
  RocketOutlined,
  DesktopOutlined,
  SettingOutlined,
  LinkOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import client from '@/api/client'
import type { DashboardStats, EngineStatus, MessageChannels, SystemConfig } from '@/types/api'

const { Title, Text } = Typography

export default function Dashboard() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [engine, setEngine] = useState<EngineStatus | null>(null)
  const [channels, setChannels] = useState<MessageChannels | null>(null)
  const [sysConfig, setSysConfig] = useState<SystemConfig | null>(null)
  const [loading, setLoading] = useState(false)

  const loadAll = useCallback(async () => {
    setLoading(true)
    await Promise.all([
      client.get('/api/stats').then((d) => setStats(d as DashboardStats)).catch(() => {}),
      client.get('/boot/systemStatus').then((d) => setEngine(d as EngineStatus)).catch(() => {}),
      client.get('/api/config/message-enabled').then((d) => setChannels(d as MessageChannels)).catch(() => {}),
      client.get('/system/config').then((d) => setSysConfig(d as SystemConfig)).catch(() => {}),
    ])
    setLoading(false)
  }, [])

  useEffect(() => {
    loadAll()
  }, [loadAll])

  const engineRunning = (engine?.parentActive ?? 0) > 0 || (engine?.registeredJobs ?? 0) > 0
  const onlineRate = stats?.instanceCount
    ? Math.round(((stats.onlineCount ?? 0) / stats.instanceCount) * 100)
    : 0

  const quickLinks = [
    { label: '抢机任务', icon: <RocketOutlined />, path: '/boot' },
    { label: '实例列表', icon: <DesktopOutlined />, path: '/instances' },
    { label: '租户管理', icon: <TeamOutlined />, path: '/tenants' },
    { label: '系统设置', icon: <SettingOutlined />, path: '/settings' },
  ]

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <Title level={4} className="!mb-0">{t('dashboard.welcome')}</Title>
        <Button icon={<ReloadOutlined />} loading={loading} onClick={loadAll}>
          {t('common.refresh')}
        </Button>
      </div>

      {/* ── Stat cards ─────────────────────────────────────── */}
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={6}>
          <Card hoverable onClick={() => navigate('/instances')}>
            <Statistic
              title="在线实例"
              value={stats?.onlineCount ?? 0}
              suffix={stats?.instanceCount ? `/ ${stats.instanceCount}` : undefined}
              prefix={<CheckCircleOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
            <Text type="secondary" className="text-xs">{onlineRate}% 在线率</Text>
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card hoverable onClick={() => navigate('/tenants')}>
            <Statistic
              title={t('dashboard.tenantCount')}
              value={stats?.tenantCount ?? 0}
              prefix={<TeamOutlined />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card hoverable onClick={() => navigate('/boot')}>
            <Statistic
              title="抢机引擎"
              value={engine?.runningTasks ?? 0}
              suffix={`/ ${engine?.totalTasks ?? 0}`}
              prefix={<RocketOutlined />}
              valueStyle={{ color: engineRunning ? '#52c41a' : '#ff4d4f' }}
            />
            <Tag color={engineRunning ? 'success' : 'error'} className="mt-1">
              {engineRunning ? '运行中' : '已停止'}
            </Tag>
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card hoverable>
            <Statistic
              title={t('dashboard.backupCount')}
              value={stats?.backupCount ?? 0}
              prefix={<CopyOutlined />}
            />
          </Card>
        </Col>
      </Row>

      {/* ── Second row: channels + system info ─────────────── */}
      <Row gutter={[16, 16]} className="mt-4">
        <Col xs={24} lg={12}>
          <Card
            title={<><ApiOutlined className="mr-2" />通知渠道</>}
            size="small"
          >
            {channels ? (
              <div className="flex flex-col gap-2">
                {[
                  { key: 'telegram' as const, label: 'Telegram' },
                  { key: 'dingtalk' as const, label: '钉钉' },
                  { key: 'bark' as const, label: 'Bark' },
                  { key: 'feishu' as const, label: '飞书' },
                ].map((ch) => (
                  <div key={ch.key} className="flex items-center justify-between px-2 py-1 rounded hover:bg-gray-50">
                    <Text>{ch.label}</Text>
                    <Tag color={channels[ch.key] ? 'success' : 'default'}>
                      {channels[ch.key] ? '已配置' : '未配置'}
                    </Tag>
                  </div>
                ))}
              </div>
            ) : (
              <Text type="secondary">{t('common.noData')}</Text>
            )}
          </Card>
        </Col>

        <Col xs={24} lg={12}>
          <Card
            title={<><SafetyOutlined className="mr-2" />系统概览</>}
            size="small"
          >
            <div className="flex flex-col gap-2">
              <div className="flex items-center justify-between px-2 py-1">
                <Text>应用版本</Text>
                <Text code>{sysConfig?.appVersion || '-'}</Text>
              </div>
              <div className="flex items-center justify-between px-2 py-1 border-t border-gray-100">
                <Text>MFA</Text>
                <Tag color={sysConfig?.bools?.['mfa.enabled'] ? 'success' : 'default'}>
                  {sysConfig?.bools?.['mfa.enabled'] ? '已启用' : '已禁用'}
                </Tag>
              </div>
              <div className="flex items-center justify-between px-2 py-1 border-t border-gray-100">
                <Text>SSL 证书</Text>
                <Tag color={sysConfig?.strings?.['ssl.domain'] ? 'success' : 'default'}>
                  {sysConfig?.strings?.['ssl.domain'] ? '已配置' : '未配置'}
                </Tag>
              </div>
            </div>
          </Card>
        </Col>
      </Row>

      {/* ── Quick links ────────────────────────────────────── */}
      <Card title={<><LinkOutlined className="mr-2" />快捷入口</>} size="small" className="mt-4">
        <Space wrap>
          {quickLinks.map((link) => (
            <Button key={link.path} icon={link.icon} onClick={() => navigate(link.path)}>
              {link.label}
            </Button>
          ))}
        </Space>
      </Card>
    </div>
  )
}
