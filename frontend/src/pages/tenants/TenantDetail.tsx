import { useCallback, useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Tabs, Button, Space, Tag, Spin, message } from 'antd'
import {
  ArrowLeftOutlined, SyncOutlined, CheckCircleOutlined,
} from '@ant-design/icons'
import { tenantGet, tenantSyncOci, tenantCheck, type Tenant, type CheckResult } from '@/api/tenant'
import TenantOverview from './tabs/TenantOverview'
import TenantInstances from './tabs/TenantInstances'
import TenantUsers from './tabs/TenantUsers'
import TenantEmail from './tabs/TenantEmail'
import TenantSocial from './tabs/TenantSocial'
import TenantQuota from './tabs/TenantQuota'
import TenantDomains from './tabs/TenantDomains'
import TenantAudit from './tabs/TenantAudit'

export default function TenantDetail() {
  const { id } = useParams<{ id: string }>()
  const tenantId = Number(id)
  const navigate = useNavigate()

  const [tenant, setTenant] = useState<Tenant | null>(null)
  const [loading, setLoading] = useState(true)
  const [syncing, setSyncing] = useState(false)
  const [checking, setChecking] = useState(false)
  const [checkResult, setCheckResult] = useState<CheckResult | null>(null)

  const loadTenant = useCallback(async () => {
    setLoading(true)
    try {
      const data = await tenantGet(tenantId)
      setTenant(data)
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setLoading(false)
    }
  }, [tenantId])

  useEffect(() => {
    loadTenant()
  }, [loadTenant])

  async function handleSync() {
    setSyncing(true)
    try {
      await tenantSyncOci(tenantId)
      message.success('同步完成')
      await loadTenant()
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setSyncing(false)
    }
  }

  async function handleCheck() {
    setChecking(true)
    setCheckResult(null)
    try {
      const r = await tenantCheck(tenantId)
      setCheckResult(r)
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setChecking(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Spin size="large" />
      </div>
    )
  }

  if (!tenant) return null

  const tabItems = [
    { key: 'overview', label: '基本信息', children: <TenantOverview tenant={tenant} onRefresh={loadTenant} /> },
    { key: 'instances', label: `实例列表`, children: <TenantInstances tenantId={tenantId} /> },
    { key: 'users', label: 'IAM 用户', children: <TenantUsers tenantId={tenantId} /> },
    { key: 'email', label: '邮件服务', children: <TenantEmail tenantId={tenantId} /> },
    { key: 'social', label: '社交登录', children: <TenantSocial tenantId={tenantId} /> },
    { key: 'quota', label: '配额', children: <TenantQuota tenantId={tenantId} /> },
    { key: 'domains', label: '域内租户', children: <TenantDomains tenantId={tenantId} /> },
    { key: 'audit', label: '审计日志', children: <TenantAudit tenantId={tenantId} /> },
  ]

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between flex-wrap gap-3 mb-4">
        <div className="flex items-center gap-3">
          <Button type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate('/tenants')} />
          <span className="text-xl font-bold">
            {tenant.userName || tenant.tenancyName || `租户 #${tenant.id}`}
          </span>
          {tenant.accountType && <Tag>{tenant.accountType}</Tag>}
          {tenant.isActive !== undefined && (
            <Tag color={tenant.isActive ? 'success' : 'error'}>
              {tenant.isActive ? '正常' : '停用'}
            </Tag>
          )}
        </div>
        <Space>
          <Button icon={<SyncOutlined />} loading={syncing} onClick={handleSync}>同步 OCI</Button>
          <Button icon={<CheckCircleOutlined />} loading={checking} onClick={handleCheck}>测试存活</Button>
        </Space>
      </div>

      {checkResult && (
        <div
          className={`p-3 rounded mb-4 ${checkResult.alive ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'}`}
        >
          {checkResult.alive ? 'OCI 认证成功' : `异常: ${checkResult.error || '未知错误'}`}
        </div>
      )}

      <Tabs defaultActiveKey="overview" items={tabItems} />
    </div>
  )
}
