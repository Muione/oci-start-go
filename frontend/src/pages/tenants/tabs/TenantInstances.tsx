import { useEffect, useState } from 'react'
import { Table, Tag, message, type TableColumnsType } from 'antd'
import { tenantInstances } from '@/api/tenant'
import type { Instance } from '@/types/api'

interface Props {
  tenantId: number
}

function stateColor(state?: string): string {
  if (!state) return 'default'
  const s = state.toLowerCase()
  if (s === 'running') return 'success'
  if (s === 'stopped' || s === 'terminated') return 'error'
  if (s === 'starting' || s === 'stopping') return 'warning'
  return 'default'
}

export default function TenantInstances({ tenantId }: Props) {
  const [rows, setRows] = useState<Instance[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setLoading(true)
    tenantInstances(tenantId)
      .then((d) => setRows(d || []))
      .catch((e: unknown) => message.error((e as Error).message))
      .finally(() => setLoading(false))
  }, [tenantId])

  const columns: TableColumnsType<Instance> = [
    {
      title: '名称',
      dataIndex: 'displayName',
      minWidth: 160,
    },
    {
      title: '实例 ID',
      dataIndex: 'instanceId',
      minWidth: 200,
      ellipsis: true,
      render: (v: string) => <span className="font-mono text-xs">{v}</span>,
    },
    {
      title: 'Shape',
      dataIndex: 'shape',
      minWidth: 140,
      ellipsis: true,
    },
    {
      title: '状态',
      dataIndex: 'state',
      width: 100,
      render: (v: string) => <Tag color={stateColor(v)}>{v || '—'}</Tag>,
    },
    {
      title: '公网 IP',
      dataIndex: 'publicIps',
      width: 140,
      render: (v: string) => <span className="font-mono text-xs">{v || '—'}</span>,
    },
    {
      title: '架构',
      dataIndex: 'architecture',
      width: 80,
    },
    {
      title: '规格',
      width: 120,
      render: (_: unknown, r: Instance) => `${r.ocpus || 0}C / ${r.memoryInGbs || 0}G`,
    },
    {
      title: 'AD',
      dataIndex: 'availabilityDomain',
      minWidth: 120,
      ellipsis: true,
    },
    {
      title: '创建时间',
      dataIndex: 'createTime',
      width: 160,
    },
  ]

  return (
    <Table
      dataSource={rows}
      columns={columns}
      rowKey="instanceId"
      loading={loading}
      bordered
      size="small"
      scroll={{ x: 1000 }}
      pagination={false}
    />
  )
}
