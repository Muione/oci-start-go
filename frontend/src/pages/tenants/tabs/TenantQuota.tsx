import { useEffect, useState } from 'react'
import { Table, Select, Progress, Descriptions, Tabs, Tag, message, type TableColumnsType } from 'antd'
import {
  tenantQuotaServices, tenantQuota, tenantRegionsSummary,
  tenantRegionsSubscribed, tenantRegionsUnsubscribed, tenantRegionsSubscribe,
  type QuotaService, type QuotaItem, type RegionSummary,
  type RegionSubInfo, type RegionUnsubInfo,
} from '@/api/tenant'
import { Button } from 'antd'

interface Props {
  tenantId: number
}

function quotaUsageColor(used: number, total: number): string {
  if (total <= 0) return '#8c8c8c'
  const pct = (used / total) * 100
  if (pct >= 90) return '#ff4d4f'
  if (pct >= 70) return '#faad14'
  return '#52c41a'
}

export default function TenantQuota({ tenantId }: Props) {
  // Quota
  const [services, setServices] = useState<QuotaService[]>([])
  const [selectedService, setSelectedService] = useState('')
  const [quotaItems, setQuotaItems] = useState<QuotaItem[]>([])
  const [quotaLoading, setQuotaLoading] = useState(false)

  // Regions
  const [regionSummary, setRegionSummary] = useState<RegionSummary | null>(null)
  const [subscribed, setSubscribed] = useState<RegionSubInfo[]>([])
  const [unsubscribed, setUnsubscribed] = useState<RegionUnsubInfo[]>([])
  const [regionsLoading, setRegionsLoading] = useState(false)
  const [selectedKeys, setSelectedKeys] = useState<string[]>([])
  const [subscribing, setSubscribing] = useState(false)

  function loadServices() {
    tenantQuotaServices(tenantId)
      .then((d) => {
        setServices(d || [])
        if (d?.length && !d.find((s) => s.name === selectedService)) {
          setSelectedService(d[0].name)
        }
      })
      .catch(() => setServices([]))
  }

  function loadQuota(service?: string) {
    const svc = service || selectedService
    if (!svc) return
    setQuotaLoading(true)
    tenantQuota(tenantId, svc)
      .then((d) => setQuotaItems(d?.items || []))
      .catch(() => setQuotaItems([]))
      .finally(() => setQuotaLoading(false))
  }

  function loadRegions() {
    setRegionsLoading(true)
    Promise.all([
      tenantRegionsSummary(tenantId),
      tenantRegionsSubscribed(tenantId),
      tenantRegionsUnsubscribed(tenantId),
    ])
      .then(([sum, sub, unsub]) => {
        setRegionSummary(sum)
        setSubscribed(sub || [])
        setUnsubscribed(unsub || [])
      })
      .catch(() => {
        setRegionSummary(null)
        setSubscribed([])
        setUnsubscribed([])
      })
      .finally(() => setRegionsLoading(false))
  }

  useEffect(() => {
    loadServices()
    loadRegions()
  }, [tenantId])

  useEffect(() => {
    if (selectedService) loadQuota(selectedService)
  }, [selectedService])

  async function handleSubscribe() {
    if (!selectedKeys.length) return
    setSubscribing(true)
    try {
      const r = await tenantRegionsSubscribe(tenantId, selectedKeys)
      const ok = r?.details?.filter((d) => d.success).length || 0
      const fail = r?.details?.filter((d) => !d.success).length || 0
      if (fail) message.warning(`订阅完成: ${ok} 成功, ${fail} 失败`)
      else message.success(`已订阅 ${ok} 个区域`)
      setSelectedKeys([])
      loadRegions()
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setSubscribing(false)
    }
  }

  const quotaColumns: TableColumnsType<QuotaItem> = [
    { title: '配额名称', dataIndex: 'name', minWidth: 220, ellipsis: true },
    {
      title: '使用率',
      width: 160,
      render: (_: unknown, row: QuotaItem) =>
        row.total > 0 ? (
          <Progress
            percent={Math.min(100, Math.round((row.used / row.total) * 100))}
            strokeColor={quotaUsageColor(row.used, row.total)}
            size="small"
          />
        ) : (
          <span className="text-xs text-gray-400">无限制</span>
        ),
    },
    {
      title: '已用',
      dataIndex: 'used',
      width: 90,
      align: 'right',
      render: (v: number) => <span className="font-mono">{v}</span>,
    },
    {
      title: '可用',
      dataIndex: 'available',
      width: 90,
      align: 'right',
      render: (v: number) => (
        <span className="font-mono" style={{ color: v > 0 ? undefined : '#ff4d4f' }}>{v}</span>
      ),
    },
    {
      title: '总量',
      dataIndex: 'total',
      width: 90,
      align: 'right',
      render: (v: number) => <span className="font-mono">{v || '—'}</span>,
    },
  ]

  const subColumns: TableColumnsType<RegionSubInfo> = [
    { title: '区域代码', dataIndex: 'regionKey', minWidth: 160 },
    { title: '区域名称', dataIndex: 'regionName', minWidth: 160 },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (v: string) => <Tag color={v === 'READY' ? 'success' : 'warning'}>{v || '—'}</Tag>,
    },
    {
      title: '主区域',
      dataIndex: 'isHomeRegion',
      width: 80,
      align: 'center',
      render: (v: boolean) => (v ? '是' : '否'),
    },
  ]

  const unsubColumns: TableColumnsType<RegionUnsubInfo> = [
    { title: '区域代码', dataIndex: 'key', minWidth: 160 },
    { title: '区域名称', dataIndex: 'name', minWidth: 160 },
    { title: '中文名', dataIndex: 'cnName', minWidth: 120 },
  ]

  return (
    <div>
      {/* Quota section */}
      <h4 className="font-semibold mb-2">配额</h4>
      {services.length > 0 && (
        <div className="mb-3">
          <Select
            value={selectedService}
            onChange={(v) => setSelectedService(v)}
            style={{ width: 200 }}
            options={services.map((s) => ({ value: s.name, label: s.description || s.name }))}
          />
        </div>
      )}
      <Table
        dataSource={quotaItems}
        columns={quotaColumns}
        rowKey="name"
        loading={quotaLoading}
        bordered
        size="small"
        pagination={false}
      />

      {/* Regions section */}
      <h4 className="font-semibold mt-8 mb-2">区域管理</h4>
      {regionSummary && (
        <Descriptions bordered size="small" column={3} className="mb-4">
          <Descriptions.Item label="总区域">{regionSummary.totalRegions}</Descriptions.Item>
          <Descriptions.Item label="已订阅">{regionSummary.subscribedRegions}</Descriptions.Item>
          <Descriptions.Item label="未订阅">{regionSummary.unsubscribedRegions}</Descriptions.Item>
        </Descriptions>
      )}

      <Tabs
        items={[
          {
            key: 'subscribed',
            label: '已订阅',
            children: (
              <Table
                dataSource={subscribed}
                columns={subColumns}
                rowKey="regionKey"
                loading={regionsLoading}
                bordered
                size="small"
                pagination={false}
              />
            ),
          },
          {
            key: 'unsubscribed',
            label: '可订阅',
            children: (
              <>
                <div className="mb-3">
                  <Button
                    type="primary"
                    disabled={!selectedKeys.length}
                    loading={subscribing}
                    onClick={handleSubscribe}
                  >
                    订阅选中 ({selectedKeys.length})
                  </Button>
                </div>
                <Table
                  dataSource={unsubscribed}
                  columns={unsubColumns}
                  rowKey="key"
                  loading={regionsLoading}
                  bordered
                  size="small"
                  pagination={false}
                  rowSelection={{
                    selectedRowKeys: selectedKeys,
                    onChange: (keys) => setSelectedKeys(keys as string[]),
                  }}
                />
              </>
            ),
          },
        ]}
      />
    </div>
  )
}
