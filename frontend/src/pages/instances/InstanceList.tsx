import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Table, Button, Input, Tag, Select, message,
  Dropdown, type TableColumnsType,
} from 'antd'
import {
  SearchOutlined, ExportOutlined,
  PlayCircleOutlined, PauseCircleOutlined, MoreOutlined,
} from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import PageHeader from '@/components/common/PageHeader'
import InstanceStatusTag from '@/components/instance/InstanceStatusTag'
import {
  instanceList, instanceStart, instanceStop, instanceExportUrl,
} from '@/api/instance'
import { tenantList, type Tenant } from '@/api/tenant'
import type { Instance } from '@/types/api'

export default function InstanceList() {
  const navigate = useNavigate()
  const [rows, setRows] = useState<Instance[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [search, setSearch] = useState('')
  const [tenantFilter, setTenantFilter] = useState<number | undefined>()
  const [stateFilter, setStateFilter] = useState<string | undefined>()
  const [tenantOptions, setTenantOptions] = useState<Tenant[]>([])
  const [selectedRowKeys, setSelectedRowKeys] = useState<number[]>([])

  const load = useCallback(async (p = page, ps = pageSize) => {
    setLoading(true)
    try {
      const offset = (p - 1) * ps
      const res = await instanceList({ limit: ps, offset })
      setRows(res.items || [])
      setTotal(res.total || 0)
    } catch (e: unknown) {
      message.error((e as Error).message || '加载失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize])

  const loadTenants = useCallback(async () => {
    try {
      const data = await tenantList()
      setTenantOptions(data || [])
    } catch { /* ignore */ }
  }, [])

  useEffect(() => {
    load()
    loadTenants()
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // Re-load when filters change
  useEffect(() => {
    load(1, pageSize)
    setPage(1)
  }, [tenantFilter, stateFilter]) // eslint-disable-line react-hooks/exhaustive-deps

  const filtered = useMemo(() => {
    const q = search.toLowerCase().trim()
    let items = rows
    if (tenantFilter) {
      items = items.filter((r) => r.tenantId === tenantFilter)
    }
    if (stateFilter) {
      items = items.filter((r) => r.state?.toLowerCase() === stateFilter.toLowerCase())
    }
    if (q) {
      items = items.filter(
        (r) =>
          (r.displayName || '').toLowerCase().includes(q) ||
          (r.publicIps || '').toLowerCase().includes(q) ||
          (r.instanceId || '').toLowerCase().includes(q),
      )
    }
    return items
  }, [rows, search, tenantFilter, stateFilter])

  // ── Actions ─────────────────────────────────────────────

  async function handleBatchStart() {
    if (selectedRowKeys.length === 0) return
    try {
      for (const id of selectedRowKeys) {
        try { await instanceStart(id) } catch { /* continue */ }
      }
      message.success('批量启动请求已发送')
      setSelectedRowKeys([])
      load()
    } catch { /* ignore */ }
  }

  async function handleBatchStop() {
    if (selectedRowKeys.length === 0) return
    try {
      for (const id of selectedRowKeys) {
        try { await instanceStop(id) } catch { /* continue */ }
      }
      message.success('批量停止请求已发送')
      setSelectedRowKeys([])
      load()
    } catch { /* ignore */ }
  }

  function handleExport(tenantId?: number) {
    const url = instanceExportUrl(tenantId)
    const a = document.createElement('a')
    a.href = url
    a.download = ''
    document.body.appendChild(a)
    a.click()
    a.remove()
    message.success('导出已开始')
  }

  // ── Columns ─────────────────────────────────────────────

  const columns: TableColumnsType<Instance> = [
    {
      title: '状态',
      dataIndex: 'state',
      width: 100,
      render: (v: string) => <InstanceStatusTag state={v} />,
    },
    {
      title: '名称',
      dataIndex: 'displayName',
      minWidth: 160,
      sorter: (a, b) => (a.displayName || '').localeCompare(b.displayName || ''),
      render: (v: string, row: Instance) => (
        <span
          className="text-blue-500 cursor-pointer hover:underline"
          onClick={() => navigate(`/instances/${row.id}`)}
        >
          {v}
        </span>
      ),
    },
    {
      title: '租户',
      dataIndex: 'tenantName',
      minWidth: 120,
    },
    {
      title: '公网 IP',
      dataIndex: 'publicIps',
      width: 140,
      render: (v: string) => <span className="font-mono text-xs">{v || '—'}</span>,
    },
    {
      title: '规格',
      width: 120,
      render: (_: unknown, r: Instance) => (
        <span className="font-mono text-xs">{r.ocpus || 0}C / {r.memoryInGbs || 0}G</span>
      ),
    },
    {
      title: '架构',
      dataIndex: 'architecture',
      width: 80,
    },
    {
      title: '区域',
      dataIndex: 'availabilityDomain',
      minWidth: 120,
      ellipsis: true,
    },
    {
      title: '操作',
      width: 80,
      fixed: 'right',
      align: 'center',
      render: (_: unknown, row: Instance) => (
        <Dropdown
          menu={{
            items: [
              { key: 'detail', label: '详情' },
              { key: 'start', label: '启动', icon: <PlayCircleOutlined />, disabled: row.state?.toLowerCase() === 'running' },
              { key: 'stop', label: '停止', icon: <PauseCircleOutlined />, disabled: row.state?.toLowerCase() !== 'running' },
            ],
            onClick: ({ key }) => {
              if (key === 'detail') navigate(`/instances/${row.id}`)
              else if (key === 'start') {
                instanceStart(row.id).then(() => { message.success('启动请求已发送'); load() }).catch((e: unknown) => message.error((e as Error).message))
              } else if (key === 'stop') {
                instanceStop(row.id).then(() => { message.success('停止请求已发送'); load() }).catch((e: unknown) => message.error((e as Error).message))
              }
            },
          }}
          trigger={['click']}
        >
          <Button type="text" size="small" icon={<MoreOutlined />} onClick={(e) => e.stopPropagation()} />
        </Dropdown>
      ),
    },
  ]

  return (
    <div>
      <PageHeader
        title="实例管理"
        onRefresh={() => { load() }}
        refreshing={loading}
        extra={
          <>
            <Tag>{total} 个实例</Tag>
            <Input
              placeholder="搜索名称 / IP / ID..."
              prefix={<SearchOutlined />}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              allowClear
              style={{ width: 220 }}
            />
            <Select
              placeholder="按租户筛选"
              value={tenantFilter}
              onChange={setTenantFilter}
              allowClear
              showSearch
              style={{ width: 160 }}
              optionFilterProp="label"
              options={tenantOptions.map((t) => ({
                value: t.id,
                label: t.userName || t.tenancy || `#${t.id}`,
              }))}
            />
            <Select
              placeholder="按状态筛选"
              value={stateFilter}
              onChange={setStateFilter}
              allowClear
              style={{ width: 120 }}
              options={[
                { value: 'Running', label: '运行中' },
                { value: 'Stopped', label: '已停止' },
                { value: 'Starting', label: '启动中' },
                { value: 'Stopping', label: '停止中' },
                { value: 'Terminated', label: '已终止' },
              ]}
            />
          </>
        }
      >
        <Button icon={<ExportOutlined />} onClick={() => handleExport(tenantFilter)}>
          导出
        </Button>
      </PageHeader>

      <Table
        dataSource={filtered}
        columns={columns}
        rowKey="id"
        loading={loading}
        bordered
        size="small"
        scroll={{ x: 1100 }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (n) => `共 ${n} 条`,
          onChange: (p, ps) => { setPage(p); setPageSize(ps); load(p, ps) },
        }}
        rowSelection={{
          selectedRowKeys,
          onChange: (keys) => setSelectedRowKeys(keys as number[]),
        }}
        onRow={(row) => ({
          onClick: () => navigate(`/instances/${row.id}`),
          style: { cursor: 'pointer' },
        })}
      />

      {/* Batch bar */}
      {selectedRowKeys.length > 0 && (
        <div className="flex items-center gap-3 mt-3 p-3 bg-gray-50 rounded border border-gray-200 text-sm text-gray-600">
          <span>已选择 {selectedRowKeys.length} 个实例</span>
          <Button size="small" icon={<PlayCircleOutlined />} onClick={handleBatchStart}>批量启动</Button>
          <Button size="small" icon={<PauseCircleOutlined />} onClick={handleBatchStop}>批量停止</Button>
          <Button size="small" onClick={() => setSelectedRowKeys([])}>取消选择</Button>
        </div>
      )}
    </div>
  )
}
