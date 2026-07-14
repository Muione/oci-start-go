import { useCallback, useEffect, useState } from 'react'
import { Table, Button, Space, Card, Modal, message } from 'antd'
import { PlusOutlined, ThunderboltOutlined } from '@ant-design/icons'
import { vnicSecurityRules, vnicDeleteSecurityRule, vnicEnableAll, vnicEnableIpv6 } from '@/api/vnic'
import type { SecurityRule } from '@/types/api'
import AddRuleModal from '../modals/AddRuleModal'

interface Props {
  tenantId: number
}

export default function SecurityRules({ tenantId }: Props) {
  const [loading, setLoading] = useState(false)
  const [ingressRules, setIngressRules] = useState<SecurityRule[]>([])
  const [egressRules, setEgressRules] = useState<SecurityRule[]>([])
  const [addOpen, setAddOpen] = useState(false)

  const loadRules = useCallback(async () => {
    setLoading(true)
    try {
      const [ingress, egress] = await Promise.all([
        vnicSecurityRules(tenantId, 'ingress'),
        vnicSecurityRules(tenantId, 'egress'),
      ])
      setIngressRules(ingress || [])
      setEgressRules(egress || [])
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setLoading(false)
    }
  }, [tenantId])

  useEffect(() => { loadRules() }, []) // eslint-disable-line react-hooks/exhaustive-deps

  async function handleDelete(rule: SecurityRule) {
    Modal.confirm({
      title: '确认删除',
      content: '确定删除此安全规则？',
      okType: 'danger',
      onOk: async () => {
        try {
          await vnicDeleteSecurityRule(rule.id!)
          message.success('规则已删除')
          loadRules()
        } catch (e: unknown) { message.error((e as Error).message) }
      },
    })
  }

  async function handleEnableAll() {
    Modal.confirm({
      title: '一键启用',
      content: '将为当前 Compartment 启用全部协议规则（IPv4 + IPv6 + ICMP），是否继续？',
      onOk: async () => {
        try {
          await vnicEnableAll(tenantId)
          message.success('全部规则已启用')
          loadRules()
        } catch (e: unknown) { message.error((e as Error).message) }
      },
    })
  }

  async function handleEnableIpv6() {
    Modal.confirm({
      title: '启用 IPv6 规则',
      content: '将启用 IPv6 入站/出站规则，是否继续？',
      onOk: async () => {
        try {
          await vnicEnableIpv6(tenantId)
          message.success('IPv6 规则已启用')
          loadRules()
        } catch (e: unknown) { message.error((e as Error).message) }
      },
    })
  }

  const ruleColumns = [
    { title: '协议', dataIndex: 'protocol', width: 80 },
    { title: 'CIDR', dataIndex: 'source' },
    { title: '端口', dataIndex: 'ports', width: 120, render: (v: string) => v || '全部' },
  ]

  return (
    <div className="p-4">
      <div className="mb-4">
        <Space>
          <Button danger icon={<ThunderboltOutlined />} onClick={handleEnableAll}>一键启用全部规则</Button>
          <Button onClick={handleEnableIpv6}>启用 IPv6 规则</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setAddOpen(true)}>添加规则</Button>
        </Space>
      </div>

      <Card title={`入站规则 (${ingressRules.length})`} size="small" className="mb-4">
        <Table dataSource={ingressRules} loading={loading} rowKey={(r) => r.id || Math.random().toString()} size="small" pagination={false}>
          {ruleColumns.map((col) => <Table.Column key={col.title} {...col} />)}
          <Table.Column title="ICMP Type" dataIndex="icmpType" width={100} render={(v: string) => v || '-'} />
          <Table.Column title="操作" width={80} render={(_: unknown, r: SecurityRule) => (
            <Button size="small" danger onClick={() => handleDelete(r)}>删除</Button>
          )} />
        </Table>
      </Card>

      <Card title={`出站规则 (${egressRules.length})`} size="small">
        <Table dataSource={egressRules} loading={loading} rowKey={(r) => r.id || Math.random().toString()} size="small" pagination={false}>
          {ruleColumns.map((col) => <Table.Column key={col.title} {...col} />)}
          <Table.Column title="操作" width={80} render={(_: unknown, r: SecurityRule) => (
            <Button size="small" danger onClick={() => handleDelete(r)}>删除</Button>
          )} />
        </Table>
      </Card>

      <AddRuleModal open={addOpen} onClose={() => setAddOpen(false)} tenantId={tenantId} onSuccess={loadRules} />
    </div>
  )
}
