import { useEffect, useState } from 'react'
import { Table, type TableColumnsType } from 'antd'
import StatusBadge from '@/components/common/StatusBadge'
import { tenantDomainTenants, type Tenant } from '@/api/tenant'

interface Props {
  tenantId: number
}

export default function TenantDomains({ tenantId }: Props) {
  const [rows, setRows] = useState<Tenant[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setLoading(true)
    tenantDomainTenants(tenantId)
      .then((d) => setRows(d || []))
      .catch(() => setRows([]))
      .finally(() => setLoading(false))
  }, [tenantId])

  const columns: TableColumnsType<Tenant> = [
    { title: '用户名', dataIndex: 'userName', minWidth: 120 },
    { title: '自定义名称', dataIndex: 'tenancyDes', minWidth: 120, render: (v: string) => v || '—' },
    { title: '区域', dataIndex: 'region', width: 120 },
    {
      title: '状态',
      dataIndex: 'isActive',
      width: 90,
      align: 'center',
      render: (v: boolean) => <StatusBadge status={v ? 'up' : 'down'} />,
    },
  ]

  return (
    <div>
      <h4 className="font-semibold mb-3">域内其他租户</h4>
      <Table
        dataSource={rows}
        columns={columns}
        rowKey="id"
        loading={loading}
        bordered
        size="small"
        pagination={false}
      />
    </div>
  )
}
