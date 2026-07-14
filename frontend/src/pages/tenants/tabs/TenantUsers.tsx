import { useEffect, useState } from 'react'
import {
  Table, Button, Space, Tag, Modal, Form, Input, Select,
  Switch, InputNumber, message, type TableColumnsType,
} from 'antd'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import {
  tenantUsersList, tenantUsersCreate, tenantUsersDelete,
  tenantUsersResetPassword, tenantGroupsList, tenantPasswordPolicyGet,
  tenantPasswordPolicySave,
  type IamUser, type IamGroup, type PasswordPolicy,
} from '@/api/tenant'

interface Props {
  tenantId: number
}

export default function TenantUsers({ tenantId }: Props) {
  const [users, setUsers] = useState<IamUser[]>([])
  const [loading, setLoading] = useState(false)
  const [groups, setGroups] = useState<IamGroup[]>([])
  const [groupsLoading, setGroupsLoading] = useState(false)
  const [pwPolicy, setPwPolicy] = useState<PasswordPolicy>({ isPasswordExpiryEnabled: false, passwordExpiryDays: 90 })

  // Add user dialog
  const [addOpen, setAddOpen] = useState(false)
  const [addSaving, setAddSaving] = useState(false)
  const [createdPwd, setCreatedPwd] = useState('')
  const [form] = Form.useForm()

  function loadUsers() {
    setLoading(true)
    tenantUsersList(tenantId)
      .then((d) => setUsers(d || []))
      .catch(() => setUsers([]))
      .finally(() => setLoading(false))
  }

  function loadGroups() {
    setGroupsLoading(true)
    tenantGroupsList(tenantId)
      .then((d) => setGroups(d || []))
      .catch(() => setGroups([]))
      .finally(() => setGroupsLoading(false))
  }

  function loadPolicy() {
    tenantPasswordPolicyGet(tenantId)
      .then((d) => {
        if (d) setPwPolicy(d)
      })
      .catch(() => {})
  }

  useEffect(() => {
    loadUsers()
    loadGroups()
    loadPolicy()
  }, [tenantId])

  async function handleCreate(values: { username: string; email: string; groupName?: string }) {
    setAddSaving(true)
    setCreatedPwd('')
    try {
      const r = await tenantUsersCreate(tenantId, values)
      setCreatedPwd(r?.password || '')
      message.success('用户已创建')
      loadUsers()
      form.resetFields()
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setAddSaving(false)
    }
  }

  async function handleDelete(row: IamUser) {
    Modal.confirm({
      title: '确认删除',
      content: `确定删除用户「${row.name}」？`,
      okType: 'danger',
      onOk: async () => {
        try {
          await tenantUsersDelete(tenantId, row.ocid)
          message.success('已删除')
          loadUsers()
        } catch (e: unknown) {
          message.error((e as Error).message)
        }
      },
    })
  }

  async function handleResetPassword(row: IamUser) {
    try {
      const r = await tenantUsersResetPassword(tenantId, row.ocid)
      message.success('密码已重置: ' + (r?.password || ''))
    } catch (e: unknown) {
      message.error((e as Error).message)
    }
  }

  async function handleSavePolicy() {
    try {
      await tenantPasswordPolicySave(tenantId, {
        enableExpiry: pwPolicy.isPasswordExpiryEnabled,
        expiryDays: pwPolicy.passwordExpiryDays,
      })
      message.success('密码策略已更新')
    } catch (e: unknown) {
      message.error((e as Error).message)
    }
  }

  const userColumns: TableColumnsType<IamUser> = [
    { title: '用户名', dataIndex: 'name', minWidth: 140 },
    { title: '描述', dataIndex: 'description', minWidth: 160, ellipsis: true },
    {
      title: '状态',
      dataIndex: 'lifecycleState',
      width: 90,
      align: 'center',
      render: (v: string) => <Tag color={v === 'ACTIVE' ? 'success' : 'warning'}>{v || '—'}</Tag>,
    },
    { title: '邮箱', dataIndex: 'email', minWidth: 180, ellipsis: true },
    {
      title: '操作',
      width: 180,
      align: 'center',
      render: (_: unknown, row: IamUser) => (
        <Space>
          <Button size="small" onClick={() => handleResetPassword(row)}>重置密码</Button>
          <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(row)} />
        </Space>
      ),
    },
  ]

  const groupColumns: TableColumnsType<IamGroup> = [
    { title: '组名', dataIndex: 'name', minWidth: 200 },
    { title: 'OCID', dataIndex: 'ocid', minWidth: 300, ellipsis: true },
  ]

  return (
    <div>
      <div className="mb-3">
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setCreatedPwd(''); setAddOpen(true) }}>
          创建用户
        </Button>
      </div>

      <Table
        dataSource={users}
        columns={userColumns}
        rowKey="ocid"
        loading={loading}
        bordered
        size="small"
        pagination={false}
      />

      {/* User groups */}
      <h4 className="mt-6 mb-2 font-semibold">用户组</h4>
      <Table
        dataSource={groups}
        columns={groupColumns}
        rowKey="ocid"
        loading={groupsLoading}
        bordered
        size="small"
        pagination={false}
      />

      {/* Password policy */}
      <h4 className="mt-6 mb-2 font-semibold">密码策略</h4>
      <div className="flex items-center gap-4 mb-4">
        <span>密码过期:</span>
        <Switch
          checked={pwPolicy.isPasswordExpiryEnabled}
          onChange={(v) => setPwPolicy({ ...pwPolicy, isPasswordExpiryEnabled: v })}
        />
        {pwPolicy.isPasswordExpiryEnabled && (
          <>
            <span>过期天数:</span>
            <InputNumber
              min={1}
              max={365}
              value={pwPolicy.passwordExpiryDays}
              onChange={(v) => setPwPolicy({ ...pwPolicy, passwordExpiryDays: v || 90 })}
            />
          </>
        )}
        <Button type="primary" size="small" onClick={handleSavePolicy}>保存</Button>
      </div>

      {/* Add user dialog */}
      <Modal
        title="创建 IAM 用户"
        open={addOpen}
        onCancel={() => setAddOpen(false)}
        footer={null}
        destroyOnClose
        width={460}
      >
        <Form form={form} layout="vertical" onFinish={handleCreate}>
          <Form.Item name="username" label="用户名" rules={[{ required: true }]}>
            <Input placeholder="IAM 用户名" />
          </Form.Item>
          <Form.Item name="email" label="邮箱" rules={[{ required: true }]}>
            <Input placeholder="用户邮箱" />
          </Form.Item>
          <Form.Item name="groupName" label="用户组">
            <Select placeholder="选择用户组（可选）" allowClear>
              {groups.map((g) => (
                <Select.Option key={g.ocid} value={g.name}>{g.name}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          {createdPwd && (
            <div className="p-3 bg-green-50 rounded mb-3">
              <span className="text-green-700">用户创建成功！一次性密码: </span>
              <code className="select-all font-mono">{createdPwd}</code>
            </div>
          )}
          <Form.Item>
            <Space>
              <Button onClick={() => setAddOpen(false)}>关闭</Button>
              <Button type="primary" htmlType="submit" loading={addSaving}>创建</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
