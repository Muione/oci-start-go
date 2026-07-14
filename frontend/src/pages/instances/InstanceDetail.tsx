import { useCallback, useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Tabs, Button, Spin, Tag, message } from 'antd'
import { ArrowLeftOutlined } from '@ant-design/icons'
import { instanceGet } from '@/api/instance'
import InstanceStatusTag from '@/components/instance/InstanceStatusTag'
import InstanceActions from '@/components/instance/InstanceActions'
import InstanceOverview from './tabs/InstanceOverview'
import InstanceTraffic from './tabs/InstanceTraffic'
import InstanceDisk from './tabs/InstanceDisk'
import InstanceConsole from './tabs/InstanceConsole'
import type { Instance } from '@/types/api'

export default function InstanceDetail() {
  const { id } = useParams<{ id: string }>()
  const instanceId = Number(id)
  const navigate = useNavigate()

  const [instance, setInstance] = useState<Instance | null>(null)
  const [loading, setLoading] = useState(true)

  const loadInstance = useCallback(async () => {
    setLoading(true)
    try {
      const data = await instanceGet(instanceId)
      setInstance(data)
    } catch (e: unknown) {
      message.error((e as Error).message || '加载实例失败')
    } finally {
      setLoading(false)
    }
  }, [instanceId])

  useEffect(() => {
    loadInstance()
  }, [loadInstance])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Spin size="large" />
      </div>
    )
  }

  if (!instance) return null

  const tabItems = [
    {
      key: 'overview',
      label: '概览',
      children: <InstanceOverview instance={instance} onRefresh={loadInstance} />,
    },
    {
      key: 'traffic',
      label: '流量监控',
      children: <InstanceTraffic instance={instance} />,
    },
    {
      key: 'disk',
      label: '磁盘管理',
      children: <InstanceDisk instance={instance} onRefresh={loadInstance} />,
    },
    {
      key: 'console',
      label: '控制台连接',
      children: <InstanceConsole instance={instance} />,
    },
  ]

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between flex-wrap gap-3 mb-4">
        <div className="flex items-center gap-3">
          <Button type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate('/instances')} />
          <span className="text-xl font-bold">
            {instance.displayName || `实例 #${instance.id}`}
          </span>
          <InstanceStatusTag state={instance.state} />
          {instance.tenantName && <Tag>{instance.tenantName}</Tag>}
          {instance.availabilityDomain && <Tag color="blue">{instance.availabilityDomain}</Tag>}
        </div>
        <InstanceActions instance={instance} onRefresh={loadInstance} />
      </div>

      <Tabs defaultActiveKey="overview" items={tabItems} />
    </div>
  )
}
