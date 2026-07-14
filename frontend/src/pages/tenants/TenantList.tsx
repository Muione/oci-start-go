import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Table, Button, Input, Space, Tag, Modal, Form, Select, message,
  Dropdown, Progress, type TableColumnsType,
} from 'antd'
import {
  PlusOutlined, SearchOutlined, ExportOutlined, DeleteOutlined,
  SyncOutlined, LinkOutlined, MoreOutlined, CheckCircleOutlined,
} from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import PageHeader from '@/components/common/PageHeader'
import StatusBadge from '@/components/common/StatusBadge'
import {
  tenantList, tenantSave, tenantDelete, tenantSyncOci,
  tenantCheckBatch, tenantExport,
  type Tenant, type CheckResult,
} from '@/api/tenant'

function maskedName(n: string): string {
  if (!n || n.length <= 2) return n || '***'
  return n[0] + '***' + n[n.length - 1]
}

function accountTypeLabel(t?: string): string {
  if (!t) return '—'
  const m: Record<string, string> = {
    FREE_TIER: '免费', FREE: '免费', PAYG: '升级号',
    PERSONAL: '个人', CORPORATE: '企业',
  }
  return m[t] || t
}

function accountTypeColor(t?: string): string {
  if (!t) return 'default'
  const s = t.toLowerCase()
  if (s.includes('free') || s.includes('trial')) return 'warning'
  if (s.includes('payg') || s.includes('paid')) return 'success'
  return 'default'
}

function daysChipColor(days?: string): string {
  if (!days) return ''
  const n = parseInt(days, 10)
  if (isNaN(n)) return ''
  if (n > 365) return '#52c41a'
  if (n >= 30) return '#faad14'
  return '#ff4d4f'
}

export default function TenantList() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [rows, setRows] = useState<Tenant[]>([])
  const [loading, setLoading] = useState(false)
  const [search, setSearch] = useState('')

  // Add dialog
  const [addOpen, setAddOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm()

  // Batch check
  const [batchOpen, setBatchOpen] = useState(false)
  const [batchChecking, setBatchChecking] = useState(false)
  const [batchProgress, setBatchProgress] = useState(0)
  const [batchResults, setBatchResults] = useState<CheckResult[]>([])

  // Export

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await tenantList()
      setRows(data || [])
    } catch (e: unknown) {
      message.error((e as Error).message || '加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const filtered = useMemo(() => {
    const q = search.toLowerCase().trim()
    if (!q) return rows
    return rows.filter(
      (r) =>
        (r.tenancyName || '').toLowerCase().includes(q) ||
        (r.userName || '').toLowerCase().includes(q) ||
        (r.tenancyDes || '').toLowerCase().includes(q) ||
        (r.region || '').toLowerCase().includes(q),
    )
  }, [rows, search])

  // ── Actions ─────────────────────────────────────────────

  async function handleSync(row: Tenant) {
    Modal.confirm({
      title: '确认同步',
      content: `从 OCI 同步租户 ${row.userName} 的全部信息？`,
      onOk: async () => {
        try {
          await tenantSyncOci(row.id)
          message.success('同步完成')
          await load()
        } catch (e: unknown) {
          message.error((e as Error).message)
        }
      },
    })
  }

  async function handleDelete(row: Tenant) {
    Modal.confirm({
      title: '确认删除',
      content: `确定删除租户「${row.userName || row.tenancyName}」？此操作不可恢复。`,
      okType: 'danger',
      onOk: async () => {
        try {
          await tenantDelete(row.id)
          message.success('已删除')
          await load()
        } catch (e: unknown) {
          message.error((e as Error).message)
        }
      },
    })
  }

  async function handleExport(row: Tenant) {
    try {
      const blob = await tenantExport(row.id)
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `tenant_${row.id}_export.json`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
      message.success('导出成功')
    } catch (e: unknown) {
      message.error((e as Error).message)
    }
  }

  async function handleBatchCheck() {
    setBatchOpen(true)
    setBatchChecking(true)
    setBatchProgress(0)
    setBatchResults([])
    try {
      setBatchProgress(10)
      const results = await tenantCheckBatch(rows.map((r) => r.id))
      setBatchProgress(100)
      setBatchResults(results || [])
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setBatchChecking(false)
    }
  }

  async function handleAdd(values: Record<string, string>) {
    setSaving(true)
    try {
      const fd = new FormData()
      fd.append('tenancy', values.tenancy)
      fd.append('tenantId', values.tenantId)
      fd.append('fingerprint', values.fingerprint)
      fd.append('region', values.region)
      fd.append('userName', values.userName || '')
      fd.append('cloudType', '1')
      fd.append('isHomeRegion', 'true')
      // Note: keyFileStr must be provided via file upload in production
      await tenantSave(fd)
      message.success('保存成功')
      setAddOpen(false)
      form.resetFields()
      await load()
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  // ── Columns ─────────────────────────────────────────────

  const columns: TableColumnsType<Tenant> = [
    {
      title: '#',
      width: 50,
      align: 'center',
      render: (_: unknown, __: Tenant, index: number) => index + 1,
    },
    {
      title: '租户名',
      dataIndex: 'tenancyName',
      minWidth: 120,
      render: (_: unknown, row: Tenant) => (
        <span className="text-blue-500 cursor-pointer hover:underline">
          {maskedName(row.tenancyName || row.userName)}
        </span>
      ),
    },
    {
      title: '自定义名称',
      dataIndex: 'tenancyDes',
      minWidth: 130,
      render: (v: string) => v || '—',
    },
    {
      title: '区域',
      dataIndex: 'regionName',
      width: 120,
      render: (_: unknown, row: Tenant) => (
        <span className="font-mono text-xs">{row.regionName || row.region || '—'}</span>
      ),
    },
    {
      title: '多区',
      dataIndex: 'hasChildren',
      width: 60,
      align: 'center',
      render: (v: boolean) => (
        <Tag color={v ? 'success' : 'default'}>{v ? '是' : '否'}</Tag>
      ),
    },
    {
      title: '订阅天数',
      dataIndex: 'activeDays',
      width: 90,
      align: 'center',
      render: (v: string) => (
        <span
          className="inline-block px-2 py-0.5 rounded text-xs font-semibold"
          style={{ backgroundColor: daysChipColor(v) ? daysChipColor(v) + '20' : undefined, color: daysChipColor(v) || undefined }}
        >
          {v || '—'}
        </span>
      ),
    },
    {
      title: '账号类型',
      dataIndex: 'planType',
      width: 90,
      align: 'center',
      render: (v: string) => <Tag color={accountTypeColor(v)}>{accountTypeLabel(v)}</Tag>,
    },
    {
      title: '实例',
      dataIndex: 'instanceCount',
      width: 70,
      align: 'center',
      render: (v: number) => v ?? 0,
    },
    {
      title: t('common.status'),
      dataIndex: 'isActive',
      width: 90,
      align: 'center',
      render: (v: boolean) => <StatusBadge status={v ? 'up' : 'down'} />,
    },
    {
      title: t('common.action'),
      width: 60,
      fixed: 'right',
      align: 'center',
      render: (_: unknown, row: Tenant) => (
        <Dropdown
          menu={{
            items: [
              { key: 'detail', label: '详情', icon: <CheckCircleOutlined /> },
              { key: 'sync', label: '同步 OCI', icon: <SyncOutlined /> },
              { key: 'export', label: '导出', icon: <ExportOutlined /> },
              { key: 'delete', label: '删除', icon: <DeleteOutlined />, danger: true },
            ],
            onClick: ({ key }) => {
              if (key === 'detail') navigate(`/tenants/${row.id}`)
              else if (key === 'sync') handleSync(row)
              else if (key === 'export') handleExport(row)
              else if (key === 'delete') handleDelete(row)
            },
          }}
          trigger={['click']}
        >
          <Button type="text" size="small" icon={<MoreOutlined />} />
        </Dropdown>
      ),
    },
  ]

  return (
    <div>
      <PageHeader
        title="租户管理"
        onRefresh={load}
        refreshing={loading}
        extra={
          <>
            <Tag>{rows.length} 个租户</Tag>
            <Input
              placeholder="搜索租户名称..."
              prefix={<SearchOutlined />}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              allowClear
              style={{ width: 200 }}
            />
          </>
        }
      >
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setAddOpen(true)}>
          新增租户
        </Button>
        <Button icon={<LinkOutlined />} onClick={handleBatchCheck} disabled={rows.length === 0}>
          批量检测
        </Button>
      </PageHeader>

      <Table
        dataSource={filtered}
        columns={columns}
        rowKey="id"
        loading={loading}
        bordered
        size="small"
        scroll={{ x: 900 }}
        pagination={{ pageSize: 20, showSizeChanger: true, showTotal: (n) => `共 ${n} 条` }}
        onRow={(row) => ({ onClick: () => navigate(`/tenants/${row.id}`) })}
        style={{ cursor: 'pointer' }}
      />

      {/* ── Add Tenant Dialog ── */}
      <Modal
        title="新增租户"
        open={addOpen}
        onCancel={() => setAddOpen(false)}
        footer={null}
        destroyOnClose
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={handleAdd}>
          <Form.Item name="tenancy" label="Tenancy OCID" rules={[{ required: true }]}>
            <Input placeholder="ocid1.tenancy.oc1..xxxxx" />
          </Form.Item>
          <Form.Item name="tenantId" label="User OCID" rules={[{ required: true }]}>
            <Input placeholder="ocid1.user.oc1..xxxxx" />
          </Form.Item>
          <Form.Item name="fingerprint" label="指纹" rules={[{ required: true }]}>
            <Input placeholder="3a:37:17:38:xx:xx:xx" />
          </Form.Item>
          <Form.Item name="region" label="区域" rules={[{ required: true }]}>
            <Select
              showSearch
              placeholder="选择区域"
              options={[
                { value: '东京 (ap-tokyo-1)', label: '东京 (ap-tokyo-1)' },
                { value: '大阪 (ap-osaka-1)', label: '大阪 (ap-osaka-1)' },
                { value: '首尔 (ap-seoul-1)', label: '首尔 (ap-seoul-1)' },
                { value: '新加坡 (ap-singapore-1)', label: '新加坡 (ap-singapore-1)' },
                { value: '悉尼 (ap-sydney-1)', label: '悉尼 (ap-sydney-1)' },
                { value: '孟买 (ap-mumbai-1)', label: '孟买 (ap-mumbai-1)' },
                { value: '法兰克福 (eu-frankfurt-1)', label: '法兰克福 (eu-frankfurt-1)' },
                { value: '伦敦 (eu-london-1)', label: '伦敦 (eu-london-1)' },
                { value: '阿什本 (us-ashburn-1)', label: '阿什本 (us-ashburn-1)' },
                { value: '凤凰城 (us-phoenix-1)', label: '凤凰城 (us-phoenix-1)' },
              ]}
            />
          </Form.Item>
          <Form.Item name="userName" label="用户名">
            <Input placeholder="留空自动生成" />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button onClick={() => setAddOpen(false)}>{t('common.cancel')}</Button>
              <Button type="primary" htmlType="submit" loading={saving}>{t('common.save')}</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* ── Batch Check Dialog ── */}
      <Modal
        title="批量检测租户状态"
        open={batchOpen}
        onCancel={() => setBatchOpen(false)}
        footer={<Button onClick={() => setBatchOpen(false)}>关闭</Button>}
        width={600}
      >
        {batchChecking && <Progress percent={batchProgress} />}
        {batchResults.length > 0 && (
          <Table
            dataSource={batchResults}
            rowKey={(_r, i) => String(i)}
            size="small"
            bordered
            pagination={false}
            scroll={{ y: 400 }}
            columns={[
              {
                title: '租户',
                minWidth: 120,
                render: (_: unknown, record: CheckResult) => record.userName || record.tenancyName || `#${record.tenantId}`,
              },
              {
                title: '状态',
                width: 80,
                align: 'center',
                render: (_: unknown, r: CheckResult) => (
                  <Tag color={r.alive ? 'success' : 'error'}>{r.alive ? '存活' : '异常'}</Tag>
                ),
              },
              { title: '错误信息', dataIndex: 'error', minWidth: 180, render: (v: string) => v || '—' },
            ]}
          />
        )}
      </Modal>
    </div>
  )
}
