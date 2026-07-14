import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Tabs, Table, Button, Input, Select, Tag, Modal, Form, Switch, InputNumber, Space, message, Empty, Spin,
} from 'antd'
import { ReloadOutlined, PlusOutlined, SearchOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import PageHeader from '@/components/common/PageHeader'
import {
  cfListZones, cfListRecords, cfCreateRecord,
  eoListRecords, eoCreateRecord,
  type CfCreateRecordPayload, type EoCreateRecordPayload,
} from '@/api/dns'
import type { CfZone, CfRecord, EoRecord } from '@/types/api'

const DNS_TYPES = ['A', 'AAAA', 'CNAME', 'MX', 'TXT', 'SRV']

function typeColor(type: string) {
  const map: Record<string, string> = { A: 'blue', AAAA: 'green', CNAME: 'orange', MX: 'red', TXT: 'geekblue', SRV: 'purple' }
  return map[type] || 'default'
}

// ─── Cloudflare Tab ────────────────────────────────────────────────────

function CloudflareTab() {
  const [zones, setZones] = useState<CfZone[]>([])
  const [zoneId, setZoneId] = useState('')
  const [records, setRecords] = useState<CfRecord[]>([])
  const [loading, setLoading] = useState(false)
  const [zonesLoading, setZonesLoading] = useState(false)
  const [configured, setConfigured] = useState<boolean | null>(null)
  const [typeFilter, setTypeFilter] = useState<string | undefined>()
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [addOpen, setAddOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm()

  const zoneName = useMemo(() => zones.find((z) => z.id === zoneId)?.name || '', [zones, zoneId])

  const loadZones = useCallback(async () => {
    setZonesLoading(true)
    try {
      const data = await cfListZones()
      setZones(data || [])
      setConfigured(true)
      if (!zoneId && data.length > 0) {
        setZoneId(data[0].id)
      }
    } catch (e: unknown) {
      const msg = (e as Error).message || ''
      if (msg.includes('not configured') || msg.includes('未配置')) {
        setConfigured(false)
      } else {
        message.error(msg)
      }
    } finally {
      setZonesLoading(false)
    }
  }, [zoneId])

  const loadRecords = useCallback(async () => {
    if (!zoneId) return
    setLoading(true)
    try {
      const res = await cfListRecords(zoneId, { page, perPage: 20, type: typeFilter })
      setRecords(res.records || [])
      setTotal(res.totalCount || 0)
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setLoading(false)
    }
  }, [zoneId, page, typeFilter])

  useEffect(() => { loadZones() }, []) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { if (zoneId) loadRecords() }, [zoneId, page, typeFilter]) // eslint-disable-line react-hooks/exhaustive-deps

  const filtered = useMemo(() => {
    const q = search.toLowerCase().trim()
    if (!q) return records
    return records.filter((r) => r.name.toLowerCase().includes(q) || r.content.toLowerCase().includes(q))
  }, [records, search])

  async function handleAdd(values: CfCreateRecordPayload) {
    if (!zoneId) return
    setSaving(true)
    try {
      let name = values.name.trim()
      if (name === '@') name = zoneName
      else if (!name.includes('.')) name = `${name}.${zoneName}`
      await cfCreateRecord(zoneId, { ...values, name })
      message.success('记录已创建')
      setAddOpen(false)
      form.resetFields()
      loadRecords()
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  if (configured === null) return <Spin style={{ display: 'block', margin: '48px auto' }} />
  if (configured === false) {
    return (
      <Empty description="Cloudflare 未配置，请在系统设置中配置 cloudflare.api.token">
        <Button type="primary" href="/settings">前往配置</Button>
      </Empty>
    )
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-3 flex-wrap gap-2">
        <Space>
          <Select
            value={zoneId || undefined}
            onChange={(v) => { setZoneId(v); setPage(1) }}
            placeholder="选择 Zone"
            style={{ width: 300 }}
            loading={zonesLoading}
            showSearch
            optionFilterProp="label"
            options={zones.map((z) => ({ label: z.name, value: z.id }))}
          />
          <Select
            allowClear
            placeholder="全部类型"
            style={{ width: 110 }}
            value={typeFilter}
            onChange={(v) => { setTypeFilter(v); setPage(1) }}
            options={DNS_TYPES.map((t) => ({ label: t, value: t }))}
          />
          {zoneName && <Tag>{total} 条记录</Tag>}
        </Space>
        <Space>
          <Input
            placeholder="搜索..."
            prefix={<SearchOutlined />}
            allowClear
            style={{ width: 200 }}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <Button icon={<ReloadOutlined />} onClick={() => { loadRecords() }} loading={loading}>刷新</Button>
          {zoneId && (
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setAddOpen(true)}>
              添加记录
            </Button>
          )}
        </Space>
      </div>

      <Table
        dataSource={filtered}
        loading={loading}
        rowKey="id"
        size="small"
        pagination={total > 20 ? { current: page, pageSize: 20, total, onChange: setPage } : false}
      >
        <Table.Column title="类型" dataIndex="type" width={80} render={(v: string) => <Tag color={typeColor(v)}>{v}</Tag>} />
        <Table.Column title="名称" dataIndex="name" ellipsis />
        <Table.Column title="内容" dataIndex="content" ellipsis />
        <Table.Column title="TTL" dataIndex="ttl" width={80} align="center" render={(v: number) => v === 1 ? 'Auto' : v} />
        <Table.Column title="代理" dataIndex="proxied" width={70} align="center" render={(v: boolean) => v ? <Tag color="green">✓</Tag> : <Tag>✗</Tag>} />
      </Table>

      <Modal
        open={addOpen}
        title="添加 Cloudflare 记录"
        onCancel={() => setAddOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={saving}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={handleAdd} initialValues={{ type: 'A', ttl: 1, proxied: false }}>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}>
            <Select options={DNS_TYPES.map((t) => ({ label: t, value: t }))} />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true }]} extra={`输入子域名（如 www），系统自动补全 .${zoneName}`}>
            <Input placeholder="www 或 @" />
          </Form.Item>
          <Form.Item name="content" label="内容" rules={[{ required: true }]}>
            <Input placeholder="IP 地址或目标域名" />
          </Form.Item>
          <Form.Item name="ttl" label="TTL" extra="设为 1 表示自动">
            <InputNumber min={1} max={86400} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="proxied" label="代理" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

// ─── EdgeOne Tab ───────────────────────────────────────────────────────

function EdgeOneTab() {
  const [records, setRecords] = useState<EoRecord[]>([])
  const [loading, setLoading] = useState(false)
  const [configured, setConfigured] = useState<boolean | null>(null)
  const [typeFilter, setTypeFilter] = useState<string | undefined>()
  const [search, setSearch] = useState('')
  const [addOpen, setAddOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm()

  const loadRecords = useCallback(async () => {
    setLoading(true)
    try {
      const data = await eoListRecords()
      setRecords(data || [])
      setConfigured(true)
    } catch (e: unknown) {
      const msg = (e as Error).message || ''
      if (msg.includes('not configured') || msg.includes('未配置')) {
        setConfigured(false)
      } else {
        message.error(msg)
      }
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { loadRecords() }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const filtered = useMemo(() => {
    let items = records
    if (typeFilter) items = items.filter((r) => (r.Type || r.type) === typeFilter)
    const q = search.toLowerCase().trim()
    if (q) items = items.filter((r) => (r.Name || r.name || '').toLowerCase().includes(q) || (r.Content || r.content || '').toLowerCase().includes(q))
    return items
  }, [records, typeFilter, search])

  async function handleAdd(values: EoCreateRecordPayload) {
    setSaving(true)
    try {
      await eoCreateRecord(values)
      message.success('记录已创建')
      setAddOpen(false)
      form.resetFields()
      loadRecords()
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  if (configured === null) return <Spin style={{ display: 'block', margin: '48px auto' }} />
  if (configured === false) {
    return (
      <Empty description="EdgeOne 未配置，请在系统设置中配置 edgeone.secretId、edgeone.secretKey 和 edgeone.zoneId">
        <Button type="primary" href="/settings">前往配置</Button>
      </Empty>
    )
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-3 flex-wrap gap-2">
        <Space>
          <Select
            allowClear
            placeholder="全部类型"
            style={{ width: 110 }}
            value={typeFilter}
            onChange={setTypeFilter}
            options={DNS_TYPES.map((t) => ({ label: t, value: t }))}
          />
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setAddOpen(true)}>添加记录</Button>
        </Space>
        <Space>
          <Input placeholder="搜索..." prefix={<SearchOutlined />} allowClear style={{ width: 200 }} value={search} onChange={(e) => setSearch(e.target.value)} />
          <Button icon={<ReloadOutlined />} onClick={loadRecords} loading={loading}>刷新</Button>
        </Space>
      </div>

      <Table dataSource={filtered} loading={loading} rowKey={(r) => r.RecordId || r.recordId || Math.random().toString()} size="small">
        <Table.Column title="类型" width={80} render={(_: unknown, r: EoRecord) => <Tag color={typeColor(r.Type || r.type || '')}>{r.Type || r.type}</Tag>} />
        <Table.Column title="名称" render={(_: unknown, r: EoRecord) => r.Name || r.name} />
        <Table.Column title="内容" render={(_: unknown, r: EoRecord) => r.Content || r.content} />
        <Table.Column title="TTL" width={80} align="center" render={(_: unknown, r: EoRecord) => r.TTL || r.ttl || '-'} />
        <Table.Column title="优先级" width={80} align="center" render={(_: unknown, r: EoRecord) => r.Priority || r.priority || '-'} />
      </Table>

      <Modal
        open={addOpen}
        title="添加 EdgeOne 记录"
        onCancel={() => setAddOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={saving}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={handleAdd} initialValues={{ type: 'A', ttl: 600, priority: 0 }}>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}>
            <Select options={DNS_TYPES.map((t) => ({ label: t, value: t }))} />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input placeholder="www 或 @" />
          </Form.Item>
          <Form.Item name="content" label="内容" rules={[{ required: true }]}>
            <Input placeholder="IP 地址或目标域名" />
          </Form.Item>
          <Form.Item name="ttl" label="TTL">
            <InputNumber min={1} max={86400} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="priority" label="优先级 (MX)">
            <InputNumber min={0} max={65535} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

// ─── Main Component ────────────────────────────────────────────────────

export default function DnsRecords() {
  const { t } = useTranslation()

  return (
    <div>
      <PageHeader title={t('nav.dns')} />
      <Tabs
        items={[
          { key: 'cloudflare', label: 'Cloudflare', children: <CloudflareTab /> },
          { key: 'edgeone', label: 'EdgeOne', children: <EdgeOneTab /> },
        ]}
      />
    </div>
  )
}
