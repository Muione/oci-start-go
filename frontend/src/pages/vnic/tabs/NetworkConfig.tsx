import { useCallback, useEffect, useState } from 'react'
import { Table, Button, Card, Space, Modal, Form, Input, Tag, message } from 'antd'
import { PlusOutlined, DeleteOutlined, UndoOutlined } from '@ant-design/icons'
import { natList, natCreate, natDelete, routeTableList, routeTableCreate, routeTableDelete, routeTableReset } from '@/api/vnic'
import type { NatGatewayInfo, RouteTableInfo } from '@/types/api'

interface Props {
  tenantId: number
  compartmentId: string
  vcnId: string
  instanceId: string
}

export default function NetworkConfig({ tenantId, compartmentId, vcnId, instanceId }: Props) {
  const [loading] = useState(false)
  const [natGateways, setNatGateways] = useState<NatGatewayInfo[]>([])
  const [routeTables, setRouteTables] = useState<RouteTableInfo[]>([])
  const [natModalOpen, setNatModalOpen] = useState(false)
  const [rtModalOpen, setRtModalOpen] = useState(false)
  const [natForm] = Form.useForm()
  const [rtForm] = Form.useForm()

  const hasParams = !!(compartmentId && vcnId && tenantId)

  const loadNat = useCallback(async () => {
    if (!hasParams) return
    try {
      const data = await natList(tenantId, compartmentId, vcnId)
      setNatGateways(data || [])
    } catch (e: unknown) { message.error((e as Error).message) }
  }, [tenantId, compartmentId, vcnId, hasParams])

  const loadRouteTables = useCallback(async () => {
    if (!hasParams) return
    try {
      const data = await routeTableList(tenantId, compartmentId, vcnId)
      setRouteTables(data || [])
    } catch (e: unknown) { message.error((e as Error).message) }
  }, [tenantId, compartmentId, vcnId, hasParams])

  useEffect(() => {
    if (hasParams) { loadNat(); loadRouteTables() }
  }, [hasParams]) // eslint-disable-line react-hooks/exhaustive-deps

  async function handleCreateNat(values: { displayName: string }) {
    try {
      await natCreate(tenantId, compartmentId, vcnId, values.displayName)
      message.success('NAT 网关创建成功')
      setNatModalOpen(false)
      natForm.resetFields()
      loadNat()
    } catch (e: unknown) { message.error((e as Error).message) }
  }

  async function handleDeleteNat(nat: NatGatewayInfo) {
    Modal.confirm({
      title: '确认删除',
      content: `确定删除 NAT 网关 "${nat.displayName}"？`,
      okType: 'danger',
      onOk: async () => {
        try {
          await natDelete(tenantId, nat.id)
          message.success('NAT 网关已删除')
          loadNat()
        } catch (e: unknown) { message.error((e as Error).message) }
      },
    })
  }

  async function handleCreateRouteTable(values: { displayName: string }) {
    try {
      await routeTableCreate(tenantId, compartmentId, vcnId, values.displayName)
      message.success('路由表创建成功')
      setRtModalOpen(false)
      rtForm.resetFields()
      loadRouteTables()
    } catch (e: unknown) { message.error((e as Error).message) }
  }

  async function handleDeleteRouteTable(rt: RouteTableInfo) {
    Modal.confirm({
      title: '确认删除',
      content: `确定删除路由表 "${rt.displayName}"？`,
      okType: 'danger',
      onOk: async () => {
        try {
          await routeTableDelete(tenantId, rt.id)
          message.success('路由表已删除')
          loadRouteTables()
        } catch (e: unknown) { message.error((e as Error).message) }
      },
    })
  }

  async function handleResetRouteTable() {
    Modal.confirm({
      title: '重置路由表',
      content: '将重置实例主 VNIC 的路由表为 VCN 默认路由表，是否继续？',
      onOk: async () => {
        try {
          await routeTableReset(tenantId, instanceId, compartmentId)
          message.success('路由表已重置为默认')
        } catch (e: unknown) { message.error((e as Error).message) }
      },
    })
  }

  return (
    <div className="p-4 space-y-4">
      {/* NAT Gateways */}
      <Card
        title="NAT 网关"
        size="small"
        extra={<Button size="small" type="primary" icon={<PlusOutlined />} onClick={() => setNatModalOpen(true)}>创建 NAT 网关</Button>}
      >
        <Table dataSource={natGateways} loading={loading} rowKey="id" size="small" pagination={false}>
          <Table.Column title="名称" dataIndex="displayName" />
          <Table.Column title="OCID" dataIndex="id" render={(v: string) => <span className="font-mono text-xs truncate block max-w-[200px]">{v}</span>} />
          <Table.Column title="状态" dataIndex="lifecycleState" width={120} render={(v: string) => <Tag color={v === 'AVAILABLE' ? 'green' : 'default'}>{v}</Tag>} />
          <Table.Column title="操作" width={80} render={(_: unknown, r: NatGatewayInfo) => (
            <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDeleteNat(r)} />
          )} />
        </Table>
      </Card>

      {/* Route Tables */}
      <Card
        title="路由表"
        size="small"
        extra={
          <Space>
            <Button size="small" type="primary" icon={<PlusOutlined />} onClick={() => setRtModalOpen(true)}>创建路由表</Button>
            <Button size="small" icon={<UndoOutlined />} onClick={handleResetRouteTable}>重置为默认</Button>
          </Space>
        }
      >
        <Table dataSource={routeTables} loading={loading} rowKey="id" size="small" pagination={false}>
          <Table.Column title="名称" dataIndex="displayName" />
          <Table.Column title="OCID" dataIndex="id" render={(v: string) => <span className="font-mono text-xs truncate block max-w-[200px]">{v}</span>} />
          <Table.Column title="规则数" width={100} render={(_: unknown, r: RouteTableInfo) => r.routeRules?.length ?? 0} />
          <Table.Column title="操作" width={80} render={(_: unknown, r: RouteTableInfo) => (
            <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDeleteRouteTable(r)} />
          )} />
        </Table>
      </Card>

      {/* Create NAT Modal */}
      <Modal open={natModalOpen} title="创建 NAT 网关" onCancel={() => setNatModalOpen(false)} onOk={() => natForm.submit()} destroyOnClose>
        <Form form={natForm} layout="vertical" onFinish={handleCreateNat}>
          <Form.Item name="displayName" label="名称" rules={[{ required: true }]}>
            <Input placeholder="my-nat-gateway" />
          </Form.Item>
        </Form>
      </Modal>

      {/* Create Route Table Modal */}
      <Modal open={rtModalOpen} title="创建路由表" onCancel={() => setRtModalOpen(false)} onOk={() => rtForm.submit()} destroyOnClose>
        <Form form={rtForm} layout="vertical" onFinish={handleCreateRouteTable}>
          <Form.Item name="displayName" label="名称" rules={[{ required: true }]}>
            <Input placeholder="my-route-table" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
