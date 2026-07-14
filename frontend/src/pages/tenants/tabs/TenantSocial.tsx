import { useEffect, useState } from 'react'
import { Table, Button, Space, Tag, Modal, Form, Input, Select, message, type TableColumnsType } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import {
  tenantSocialList, tenantSocialSave, tenantSocialToggle, tenantSocialDelete,
  type SocialConfig,
} from '@/api/tenant'

interface Props {
  tenantId: number
}

const SOCIAL_TYPES = ['GITHUB', 'GOOGLE']

export default function TenantSocial({ tenantId }: Props) {
  const [rows, setRows] = useState<SocialConfig[]>([])
  const [loading, setLoading] = useState(false)

  // Edit dialog
  const [editOpen, setEditOpen] = useState(false)
  const [editSaving, setEditSaving] = useState(false)
  const [editId, setEditId] = useState<string>('')
  const [form] = Form.useForm()

  function load() {
    setLoading(true)
    tenantSocialList(tenantId)
      .then((d) => setRows(d || []))
      .catch(() => setRows([]))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load()
  }, [tenantId])

  function openAdd() {
    setEditId('')
    form.resetFields()
    setEditOpen(true)
  }

  function openEdit(row: SocialConfig) {
    setEditId(row.id)
    form.setFieldsValue({
      socialType: row.socialTypeStr || 'GITHUB',
      clientId: row.clientId,
      clientSecret: '',
      redirectUrl: row.redirectUrl,
      loginUrl: row.loginUrl,
    })
    setEditOpen(true)
  }

  async function handleSave(values: Record<string, string>) {
    setEditSaving(true)
    try {
      const payload = editId ? { ...values, id: editId } : values
      await tenantSocialSave(tenantId, payload as Partial<SocialConfig>)
      message.success('已保存')
      setEditOpen(false)
      load()
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setEditSaving(false)
    }
  }

  async function handleToggle(row: SocialConfig) {
    try {
      await tenantSocialToggle(tenantId, row.id)
      message.success('已切换')
      load()
    } catch (e: unknown) {
      message.error((e as Error).message)
    }
  }

  async function handleDelete(row: SocialConfig) {
    Modal.confirm({
      title: '确认删除',
      content: `删除「${row.socialTypeStr}」配置？`,
      okType: 'danger',
      onOk: async () => {
        try {
          await tenantSocialDelete(tenantId, row.id)
          message.success('已删除')
          load()
        } catch (e: unknown) {
          message.error((e as Error).message)
        }
      },
    })
  }

  const columns: TableColumnsType<SocialConfig> = [
    { title: '平台', dataIndex: 'socialTypeStr', width: 100 },
    { title: 'Client ID', dataIndex: 'clientId', minWidth: 160, ellipsis: true },
    {
      title: '状态',
      dataIndex: 'socialStatus',
      width: 90,
      align: 'center',
      render: (v: string) => <Tag color={v === 'enabled' ? 'success' : 'default'}>{v === 'enabled' ? '启用' : '禁用'}</Tag>,
    },
    {
      title: '操作',
      width: 200,
      align: 'center',
      render: (_: unknown, row: SocialConfig) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(row)} />
          <Button size="small" onClick={() => handleToggle(row)}>
            {row.socialStatus === 'enabled' ? '禁用' : '启用'}
          </Button>
          <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(row)} />
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div className="mb-3">
        <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>添加配置</Button>
      </div>

      <Table
        dataSource={rows}
        columns={columns}
        rowKey="id"
        loading={loading}
        bordered
        size="small"
        pagination={false}
      />

      <Modal
        title={editId ? '编辑社媒配置' : '添加社媒配置'}
        open={editOpen}
        onCancel={() => setEditOpen(false)}
        footer={null}
        destroyOnClose
        width={460}
      >
        <Form form={form} layout="vertical" onFinish={handleSave}>
          <Form.Item name="socialType" label="平台类型" initialValue="GITHUB">
            <Select options={SOCIAL_TYPES.map((t) => ({ value: t, label: t }))} />
          </Form.Item>
          <Form.Item name="clientId" label="Client ID" rules={[{ required: true }]}>
            <Input placeholder="OAuth Client ID" />
          </Form.Item>
          <Form.Item name="clientSecret" label="Secret" rules={[{ required: !editId }]}>
            <Input.Password placeholder="OAuth Client Secret" />
          </Form.Item>
          <Form.Item name="redirectUrl" label="回调地址">
            <Input placeholder="https://your-domain.com/oauth/callback" />
          </Form.Item>
          <Form.Item name="loginUrl" label="登录地址">
            <Input placeholder="https://platform.com/login" />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button onClick={() => setEditOpen(false)}>取消</Button>
              <Button type="primary" htmlType="submit" loading={editSaving}>保存</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
