import { useState } from 'react'
import { Table, Button, Tag, Space, Modal, message } from 'antd'
import { PlusOutlined, DeleteOutlined, ReloadOutlined } from '@ant-design/icons'
import { vnicDelete, vnicDeleteAllSecondary, vnicCreateIpv6, vnicDeleteIpv6 } from '@/api/vnic'
import type { VnicInfo, VnicLoadData } from '@/types/api'
import BatchCreateVnicModal from '../modals/BatchCreateVnic'

interface Props {
  instanceId: string
  data: VnicLoadData | null
  loading: boolean
  onRefresh: () => void
}

export default function VnicList({ instanceId, data, loading, onRefresh }: Props) {
  const [batchOpen, setBatchOpen] = useState(false)
  const [deletingAll, setDeletingAll] = useState(false)
  const [ipv6Modal, setIpv6Modal] = useState<{ vnic: VnicInfo; count: number } | null>(null)
  const [creatingIpv6, setCreatingIpv6] = useState(false)

  const allVnics = data?.vnicList || []
  const stats = data?.statistics

  async function handleDeleteVnic(vnic: VnicInfo) {
    if (vnic.isPrimary) { message.warning('不能删除主 VNIC'); return }
    Modal.confirm({
      title: '确认删除',
      content: `确定删除 VNIC「${vnic.vnicDisplayName}」？此操作将同时删除所有关联的 IPv6 地址。`,
      okType: 'danger',
      onOk: async () => {
        try {
          await vnicDelete(instanceId, vnic.vnicId)
          message.success('VNIC 已删除')
          onRefresh()
        } catch (e: unknown) { message.error((e as Error).message) }
      },
    })
  }

  async function handleDeleteAllSecondary() {
    const count = data?.secondaryVnics?.length || 0
    if (count === 0) return
    Modal.confirm({
      title: '确认删除',
      content: `确定删除所有 ${count} 个辅助 VNIC？此操作将同时删除所有关联的 IPv6 地址。`,
      okType: 'danger',
      onOk: async () => {
        setDeletingAll(true)
        try {
          await vnicDeleteAllSecondary(instanceId)
          message.success(`已删除辅助 VNIC`)
          onRefresh()
        } catch (e: unknown) { message.error((e as Error).message) }
        finally { setDeletingAll(false) }
      },
    })
  }

  async function handleCreateIpv6() {
    if (!ipv6Modal) return
    setCreatingIpv6(true)
    try {
      await vnicCreateIpv6(ipv6Modal.vnic.vnicId, ipv6Modal.count, instanceId)
      message.success('IPv6 地址创建成功')
      setIpv6Modal(null)
      onRefresh()
    } catch (e: unknown) { message.error((e as Error).message) }
    finally { setCreatingIpv6(false) }
  }

  async function handleDeleteIpv6(vnic: VnicInfo, addr: string) {
    try {
      await vnicDeleteIpv6(addr, vnic.vnicId, instanceId)
      message.success('IPv6 地址已删除')
      onRefresh()
    } catch (e: unknown) { message.error((e as Error).message) }
  }

  return (
    <div>
      {/* Stats */}
      {stats && (
        <div className="grid grid-cols-4 gap-3 mb-4">
          {[
            { label: '总 VNIC 数', value: stats.totalVnicCount },
            { label: '活跃 VNIC', value: stats.activeVnicCount },
            { label: '辅助 VNIC', value: stats.secondaryVnicCount },
            { label: '总 IPv6', value: stats.totalIpv6Count },
          ].map((s) => (
            <div key={s.label} className="border rounded-lg p-3 text-center">
              <div className="text-2xl font-bold">{s.value ?? 0}</div>
              <div className="text-xs text-gray-500 mt-1">{s.label}</div>
            </div>
          ))}
        </div>
      )}

      {/* Ops toolbar */}
      <div className="flex gap-2 mb-3 flex-wrap">
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setBatchOpen(true)}>批量创建 VNIC+IPv6</Button>
        <Button danger icon={<DeleteOutlined />} onClick={handleDeleteAllSecondary} loading={deletingAll} disabled={!data?.secondaryVnics?.length}>
          删除所有辅助 VNIC
        </Button>
        <Button icon={<ReloadOutlined />} onClick={onRefresh} loading={loading}>刷新</Button>
      </div>

      {/* VNIC Table */}
      <Table dataSource={allVnics} loading={loading} rowKey="vnicId" size="small" expandable={{
        expandedRowRender: (row: VnicInfo) => (
          <div className="p-3 bg-gray-50 rounded">
            {row.ipv6Addresses && row.ipv6Addresses.length > 0 ? (
              <div className="flex flex-wrap gap-1">
                {row.ipv6Addresses.map((addr) => (
                  <Tag key={addr} className="font-mono text-xs">
                    {addr}
                    <span className="ml-1 cursor-pointer text-red-500" onClick={() => handleDeleteIpv6(row, addr)}>×</span>
                  </Tag>
                ))}
              </div>
            ) : (
              <span className="text-gray-400 text-sm">无 IPv6 地址</span>
            )}
            <Button size="small" type="link" onClick={() => setIpv6Modal({ vnic: row, count: 1 })}>分配 IPv6</Button>
          </div>
        ),
      }}>
        <Table.Column title="#" render={(_: unknown, __: unknown, i: number) => i + 1} width={50} />
        <Table.Column title="名称" render={(_: unknown, r: VnicInfo) => (
          <span>
            {r.vnicDisplayName || '-'}
            {r.isPrimary && <Tag color="green" className="ml-1">主 VNIC</Tag>}
          </span>
        )} />
        <Table.Column title="私网 IP" dataIndex="privateIp" width={140} render={(v: string) => <span className="font-mono text-xs">{v || '-'}</span>} />
        <Table.Column title="公网 IP" dataIndex="publicIp" width={140} render={(v: string) => <span className="font-mono text-xs">{v || '-'}</span>} />
        <Table.Column title="子网" dataIndex="subnetId" ellipsis render={(v: string) => <span className="font-mono text-xs">{v || '-'}</span>} />
        <Table.Column title="IPv6 数" width={80} render={(_: unknown, r: VnicInfo) => r.ipv6Addresses?.length ?? 0} />
        <Table.Column title="状态" dataIndex="lifecycleState" width={100} render={(v: string) => (
          <Tag color={v === 'ATTACHED' ? 'green' : v === 'DETACHED' ? 'red' : 'default'}>{v || '-'}</Tag>
        )} />
        <Table.Column
          title="操作"
          width={160}
          render={(_: unknown, r: VnicInfo) => (
            <Space>
              <Button size="small" icon={<PlusOutlined />} onClick={() => setIpv6Modal({ vnic: r, count: 1 })}>IPv6</Button>
              <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDeleteVnic(r)} disabled={r.isPrimary} />
            </Space>
          )}
        />
      </Table>

      {/* Batch create modal */}
      <BatchCreateVnicModal open={batchOpen} onClose={() => setBatchOpen(false)} instanceId={instanceId} onSuccess={onRefresh} />

      {/* Create IPv6 modal */}
      <Modal
        open={!!ipv6Modal}
        title="创建 IPv6 地址"
        onCancel={() => setIpv6Modal(null)}
        onOk={handleCreateIpv6}
        confirmLoading={creatingIpv6}
        destroyOnClose
      >
        {ipv6Modal && (
          <div>
            <p>VNIC: {ipv6Modal.vnic.vnicDisplayName || '-'}</p>
            <p>当前 IPv6 数: {ipv6Modal.vnic.ipv6Addresses?.length ?? 0}</p>
            <p className="text-orange-500 mt-2">⚠️ 创建 IPv6 后实例将自动重启以生效</p>
          </div>
        )}
      </Modal>
    </div>
  )
}
