import { useCallback, useEffect, useState } from 'react'
import { Table, Button, Card, Descriptions, message } from 'antd'
import { vcnList, vcnConfigureIpv6, vnicReassignIp } from '@/api/vnic'
import type { VcnInfo } from '@/types/api'

interface Props {
  tenantId: number
  compartmentId: string
  instanceId: string
  instanceDbId: number
  currentPublicIp: string
}

export default function VcnPanel({ tenantId, compartmentId, instanceId, instanceDbId, currentPublicIp }: Props) {
  const [loading, setLoading] = useState(false)
  const [vcnList2, setVcnList] = useState<VcnInfo[]>([])
  const [selectedVcn, setSelectedVcn] = useState<VcnInfo | null>(null)
  const [publicIp, setPublicIp] = useState(currentPublicIp)
  const [reassigning, setReassigning] = useState(false)

  const loadVcns = useCallback(async () => {
    if (!compartmentId || !tenantId) return
    setLoading(true)
    try {
      const data = await vcnList(tenantId, compartmentId)
      setVcnList(data || [])
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setLoading(false)
    }
  }, [tenantId, compartmentId])

  useEffect(() => { loadVcns() }, []) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { setPublicIp(currentPublicIp) }, [currentPublicIp])

  async function handleConfigureIpv6() {
    if (!selectedVcn) return
    try {
      await vcnConfigureIpv6(tenantId, selectedVcn.id)
      message.success('IPv6 安全规则配置成功')
    } catch (e: unknown) {
      message.error((e as Error).message)
    }
  }

  async function handleReassignIp() {
    setReassigning(true)
    try {
      const res = await vnicReassignIp(instanceId, instanceDbId)
      if (res.publicIp) {
        setPublicIp(res.publicIp)
        message.success(`公网 IP 已重新分配: ${res.publicIp}`)
      }
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setReassigning(false)
    }
  }

  return (
    <div className="p-4 space-y-4">
      {/* VCN List */}
      <Card title="VCN 列表" size="small">
        <Table
          dataSource={vcnList2}
          loading={loading}
          rowKey="id"
          size="small"
          onRow={(record) => ({ onClick: () => setSelectedVcn(record) })}
          rowClassName={(record) => record.id === selectedVcn?.id ? 'bg-blue-50' : ''}
          pagination={false}
        >
          <Table.Column title="名称" dataIndex="displayName" />
          <Table.Column title="CIDR" dataIndex="cidrBlock" />
          <Table.Column title="DNS 标签" dataIndex="dnsLabel" />
          <Table.Column title="创建时间" dataIndex="timeCreated" width={180} />
        </Table>
      </Card>

      {/* VCN Detail */}
      {selectedVcn && (
        <Card title={`VCN 详情: ${selectedVcn.displayName}`} size="small">
          <Descriptions column={2} size="small" bordered>
            <Descriptions.Item label="OCID"><span className="font-mono text-xs truncate block max-w-[300px]">{selectedVcn.id}</span></Descriptions.Item>
            <Descriptions.Item label="CIDR">{selectedVcn.cidrBlock}</Descriptions.Item>
            <Descriptions.Item label="DNS 标签">{selectedVcn.dnsLabel || '-'}</Descriptions.Item>
            <Descriptions.Item label="创建时间">{selectedVcn.timeCreated}</Descriptions.Item>
          </Descriptions>
          <Button className="mt-3" onClick={handleConfigureIpv6}>配置 IPv6 安全规则</Button>
        </Card>
      )}

      {/* Public IP */}
      <Card title="公网 IP 管理" size="small">
        <div className="flex items-center gap-4">
          <span>当前公网 IP: <strong>{publicIp || '无'}</strong></span>
          <Button type="primary" danger loading={reassigning} onClick={handleReassignIp}>重新分配公网 IP</Button>
        </div>
      </Card>
    </div>
  )
}
