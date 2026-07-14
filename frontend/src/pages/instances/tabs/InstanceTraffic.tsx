import { useEffect, useState } from 'react'
import { Card, Row, Col, Statistic, Table, Spin, message, type TableColumnsType } from 'antd'
import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend,
} from 'recharts'
import { instanceTraffic, type InstanceTraffic as TrafficRecord } from '@/api/instance'
import type { Instance } from '@/types/api'

interface Props {
  instance: Instance
}

export default function InstanceTraffic({ instance }: Props) {
  const [loading, setLoading] = useState(false)
  const [records, setRecords] = useState<TrafficRecord[]>([])

  useEffect(() => {
    if (!instance.tenantId) return
    setLoading(true)
    instanceTraffic(instance.tenantId)
      .then((data) => {
        // Filter for this instance
        const filtered = (data || []).filter(
          (r) => r.instanceId === instance.instanceId || !instance.instanceId,
        )
        setRecords(filtered)
      })
      .catch((e: unknown) => message.error((e as Error).message || '加载流量数据失败'))
      .finally(() => setLoading(false))
  }, [instance.tenantId, instance.instanceId])

  // Aggregate totals
  const totalIngress = records.reduce((sum, r) => sum + (r.ingressBytes ?? 0), 0)
  const totalEgress = records.reduce((sum, r) => sum + (r.egressBytes ?? 0), 0)
  const ingressGB = +(totalIngress / 1024 ** 3).toFixed(2)
  const egressGB = +(totalEgress / 1024 ** 3).toFixed(2)

  // Chart data: group by date
  const chartData = records.reduce<Array<{ date: string; ingress: number; egress: number }>>((acc, r) => {
    const date = r.statsDate || 'unknown'
    const existing = acc.find((d) => d.date === date)
    if (existing) {
      existing.ingress += r.ingressBytes ?? 0
      existing.egress += r.egressBytes ?? 0
    } else {
      acc.push({ date, ingress: r.ingressBytes ?? 0, egress: r.egressBytes ?? 0 })
    }
    return acc
  }, []).map((d) => ({
    date: d.date,
    ingress: +(d.ingress / 1024 ** 3).toFixed(2),
    egress: +(d.egress / 1024 ** 3).toFixed(2),
  })).sort((a, b) => a.date.localeCompare(b.date))

  const columns: TableColumnsType<TrafficRecord> = [
    { title: '日期', dataIndex: 'statsDate', width: 120 },
    { title: '区域', dataIndex: 'region', width: 120 },
    {
      title: '入站 (GB)',
      width: 120,
      render: (_: unknown, r: TrafficRecord) => ((r.ingressBytes ?? 0) / 1024 ** 3).toFixed(2),
    },
    {
      title: '出站 (GB)',
      width: 120,
      render: (_: unknown, r: TrafficRecord) => ((r.egressBytes ?? 0) / 1024 ** 3).toFixed(2),
    },
    { title: 'VNIC 数', dataIndex: 'vnicCount', width: 80 },
  ]

  if (loading) {
    return <div className="flex justify-center py-8"><Spin /></div>
  }

  return (
    <div>
      {/* Summary cards */}
      <Row gutter={16} className="mb-4">
        <Col span={12}>
          <Card size="small">
            <Statistic title="入站流量" value={ingressGB} suffix="GB" precision={2} />
          </Card>
        </Col>
        <Col span={12}>
          <Card size="small">
            <Statistic title="出站流量" value={egressGB} suffix="GB" precision={2} />
          </Card>
        </Col>
      </Row>

      {/* Chart */}
      {chartData.length > 0 && (
        <Card title="流量趋势" size="small" className="mb-4">
          <ResponsiveContainer width="100%" height={250}>
            <AreaChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="date" />
              <YAxis />
              <Tooltip />
              <Legend />
              <Area type="monotone" dataKey="ingress" name="入站 (GB)" stroke="#1890ff" fill="#1890ff" fillOpacity={0.1} />
              <Area type="monotone" dataKey="egress" name="出站 (GB)" stroke="#52c41a" fill="#52c41a" fillOpacity={0.1} />
            </AreaChart>
          </ResponsiveContainer>
        </Card>
      )}

      {/* Table */}
      <Card title="流量明细" size="small">
        <Table
          dataSource={records}
          columns={columns}
          rowKey={(_, i) => String(i)}
          size="small"
          bordered
          pagination={false}
          scroll={{ x: 600 }}
        />
      </Card>
    </div>
  )
}
