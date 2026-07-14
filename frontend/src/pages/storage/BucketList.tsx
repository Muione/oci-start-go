import { useCallback, useEffect, useState } from 'react'
import { Table, Button, Tag, Space, Modal, Form, Select, Input, message } from 'antd'
import { PlusOutlined, ReloadOutlined, DeleteOutlined } from '@ant-design/icons'
import { listBuckets, createBucket, deleteBucket } from '@/api/storage'
import { tenantList } from '@/api/tenant'
import type { Bucket, Tenant } from '@/types/api'

interface Props {
  onSelect: (bucket: Bucket) => void
}

function accessTypeTag(t: string) {
  if (!t || t === 'NoPublicAccess') return 'success'
  if (t === 'ObjectRead') return 'warning'
  return 'default'
}

function accessTypeLabel(t: string) {
  if (!t || t === 'NoPublicAccess') return '私有'
  if (t === 'ObjectRead') return '对象可读'
  if (t === 'ObjectReadWithoutList') return '可读无列表'
  return t
}

export default function BucketList({ onSelect }: Props) {
  const [buckets, setBuckets] = useState<Bucket[]>([])
  const [loading, setLoading] = useState(false)
  const [tenantId, setTenantId] = useState<number | undefined>()
  const [tenantOptions, setTenantOptions] = useState<Tenant[]>([])
  const [nextPage, setNextPage] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [createSaving, setCreateSaving] = useState(false)
  const [form] = Form.useForm()

  const loadTenants = useCallback(async () => {
    try {
      const data = await tenantList()
      setTenantOptions(data || [])
      if (data.length > 0 && !tenantId) setTenantId(data[0].id)
    } catch { /* ignore */ }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const loadBuckets = useCallback(async () => {
    if (!tenantId) return
    setLoading(true)
    try {
      const res = await listBuckets(tenantId)
      setBuckets(res.items || [])
      setNextPage(res.nextPage || '')
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setLoading(false)
    }
  }, [tenantId])

  useEffect(() => { loadTenants() }, []) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { if (tenantId) loadBuckets() }, [tenantId]) // eslint-disable-line react-hooks/exhaustive-deps

  async function handleCreate(values: { name: string; publicAccessType: string }) {
    if (!tenantId) return
    setCreateSaving(true)
    try {
      await createBucket(tenantId, values.name.trim(), values.publicAccessType)
      message.success('存储桶创建成功')
      setCreateOpen(false)
      form.resetFields()
      loadBuckets()
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setCreateSaving(false)
    }
  }

  async function handleDelete(bucket: Bucket) {
    if (!tenantId) return
    Modal.confirm({
      title: '确认删除',
      content: `确定删除存储桶「${bucket.name}」？存储桶必须为空才能删除。`,
      okType: 'danger',
      onOk: async () => {
        try {
          await deleteBucket(tenantId, bucket.namespace, bucket.name)
          message.success('存储桶已删除')
          loadBuckets()
        } catch (e: unknown) {
          message.error((e as Error).message)
        }
      },
    })
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4 flex-wrap gap-2">
        <Space>
          <Select
            value={tenantId}
            onChange={setTenantId}
            placeholder="选择租户"
            style={{ width: 260 }}
            showSearch
            optionFilterProp="label"
            options={tenantOptions.map((t) => ({ label: t.userName || t.tenancy, value: t.id }))}
          />
          <Tag>{buckets.length} 个存储桶</Tag>
        </Space>
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)} disabled={!tenantId}>
            创建存储桶
          </Button>
          <Button icon={<ReloadOutlined />} onClick={loadBuckets} loading={loading}>刷新</Button>
        </Space>
      </div>

      <Table
        dataSource={buckets}
        loading={loading}
        rowKey="name"
        size="small"
        locale={{ emptyText: !tenantId ? '请先选择租户' : '暂无存储桶' }}
      >
        <Table.Column title="#" render={(_: unknown, __: unknown, i: number) => i + 1} width={50} />
        <Table.Column
          title="存储桶名称"
          dataIndex="name"
          render={(v: string, row: Bucket) => <a onClick={() => onSelect(row)}>{v}</a>}
        />
        <Table.Column title="命名空间" dataIndex="namespace" width={150} />
        <Table.Column title="访问类型" dataIndex="publicAccess" width={130} render={(v: string) => <Tag color={accessTypeTag(v)}>{accessTypeLabel(v)}</Tag>} />
        <Table.Column title="创建时间" dataIndex="timeCreated" width={180} render={(v: string) => v ? new Date(v).toLocaleString('zh-CN') : '-'} />
        <Table.Column
          title="操作"
          width={80}
          align="center"
          render={(_: unknown, row: Bucket) => (
            <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(row)} />
          )}
        />
      </Table>

      {nextPage && (
        <div className="text-center mt-3">
          <Button size="small" onClick={loadBuckets}>加载更多</Button>
        </div>
      )}

      <Modal open={createOpen} title="创建存储桶" onCancel={() => setCreateOpen(false)} onOk={() => form.submit()} confirmLoading={createSaving} destroyOnClose>
        <Form form={form} layout="vertical" onFinish={handleCreate} initialValues={{ publicAccessType: 'NoPublicAccess' }}>
          <Form.Item name="name" label="存储桶名称" rules={[{ required: true }]}>
            <Input placeholder="my-bucket" />
          </Form.Item>
          <Form.Item name="publicAccessType" label="访问类型">
            <Select options={[
              { label: '私有 (NoPublicAccess)', value: 'NoPublicAccess' },
              { label: '对象可读 (ObjectRead)', value: 'ObjectRead' },
              { label: '对象可读无列表 (ObjectReadWithoutList)', value: 'ObjectReadWithoutList' },
            ]} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
