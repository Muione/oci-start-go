import { useState } from 'react'
import { Descriptions, Button, Space, Modal, Input, message, Tag } from 'antd'
import { SyncOutlined, DownloadOutlined, DeleteOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import {
  tenantUpdate, tenantDelete, tenantExport, tenantUpdateDetail,
  type Tenant,
} from '@/api/tenant'

interface Props {
  tenant: Tenant
  onRefresh: () => Promise<void>
}

function cloudTypeLabel(ct?: number): string {
  return ct === 1 ? 'OCI' : ct === 2 ? 'AWS' : ct === 4 ? 'Azure' : String(ct || '—')
}

export default function TenantOverview({ tenant, onRefresh }: Props) {
  const navigate = useNavigate()
  const [editNameOpen, setEditNameOpen] = useState(false)
  const [editName, setEditName] = useState(tenant.tenancyDes || '')
  const [editSaving, setEditSaving] = useState(false)
  const [updateLoading, setUpdateLoading] = useState(false)

  async function handleSaveName() {
    setEditSaving(true)
    try {
      await tenantUpdate(tenant.id, { ...tenant, tenancyDes: editName })
      message.success('已更新')
      setEditNameOpen(false)
      await onRefresh()
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setEditSaving(false)
    }
  }

  async function handleUpdateDetail() {
    setUpdateLoading(true)
    try {
      await tenantUpdateDetail(tenant.id)
      message.success('已从 OCI 获取')
      await onRefresh()
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setUpdateLoading(false)
    }
  }

  async function handleExport() {
    try {
      const blob = await tenantExport(tenant.id)
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `tenant_${tenant.id}_export.json`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
      message.success('导出成功')
    } catch (e: unknown) {
      message.error((e as Error).message)
    }
  }

  async function handleDelete() {
    Modal.confirm({
      title: '确认删除',
      content: `确定删除租户「${tenant.userName || tenant.tenancyName}」？不可恢复，将删除所有实例记录。`,
      okType: 'danger',
      onOk: async () => {
        try {
          await tenantDelete(tenant.id)
          message.success('已删除')
          navigate('/tenants')
        } catch (e: unknown) {
          message.error((e as Error).message)
        }
      },
    })
  }

  return (
    <div>
      <Descriptions bordered size="small" column={2}>
        <Descriptions.Item label="租户 ID">{tenant.id}</Descriptions.Item>
        <Descriptions.Item label="用户名">{tenant.userName}</Descriptions.Item>
        <Descriptions.Item label="Tenancy OCID" span={2}>
          <span className="font-mono text-xs break-all">{tenant.tenancy}</span>
        </Descriptions.Item>
        <Descriptions.Item label="User OCID" span={2}>
          <span className="font-mono text-xs break-all">{tenant.tenantId}</span>
        </Descriptions.Item>
        <Descriptions.Item label="指纹" span={2}>
          <span className="font-mono text-xs">{tenant.fingerprint}</span>
        </Descriptions.Item>
        <Descriptions.Item label="区域">{tenant.regionName || tenant.region}</Descriptions.Item>
        <Descriptions.Item label="区域代码">{tenant.region}</Descriptions.Item>
        <Descriptions.Item label="云厂商"><Tag>{cloudTypeLabel(tenant.cloudType)}</Tag></Descriptions.Item>
        <Descriptions.Item label="自定义名称">
          {tenant.tenancyDes || '—'}
          <Button type="link" size="small" onClick={() => { setEditName(tenant.tenancyDes || ''); setEditNameOpen(true) }}>
            编辑
          </Button>
        </Descriptions.Item>
        <Descriptions.Item label="账号成本">{tenant.accountCost || '—'}</Descriptions.Item>
        <Descriptions.Item label="邮箱">{tenant.emailAddress || '—'}</Descriptions.Item>
        <Descriptions.Item label="创建时间">{tenant.createdAt || '—'}</Descriptions.Item>
        <Descriptions.Item label="订阅天数">{tenant.activeDays || '—'}</Descriptions.Item>
        <Descriptions.Item label="API 同步">
          <Tag color={tenant.apiSynced ? 'success' : 'default'}>
            {tenant.apiSynced ? '已同步' : '未同步'}
          </Tag>
        </Descriptions.Item>
        <Descriptions.Item label="父租户 ID">{tenant.parenId || '无 (主租户)'}</Descriptions.Item>
      </Descriptions>

      {/* Actions */}
      <div className="mt-6">
        <Space direction="vertical" className="w-full">
          <div className="flex items-center gap-3">
            <Button icon={<SyncOutlined />} loading={updateLoading} onClick={handleUpdateDetail}>
              从 OCI 获取
            </Button>
          </div>
          <div className="flex items-center gap-3">
            <Button icon={<DownloadOutlined />} onClick={handleExport}>导出租户数据</Button>
          </div>
          <div className="flex items-center gap-3">
            <Button danger icon={<DeleteOutlined />} onClick={handleDelete}>删除此租户</Button>
            <span className="text-xs text-gray-400">不可恢复，将删除所有实例记录</span>
          </div>
        </Space>
      </div>

      {/* Edit name dialog */}
      <Modal
        title="设置自定义名称"
        open={editNameOpen}
        onCancel={() => setEditNameOpen(false)}
        footer={null}
        destroyOnClose
      >
        <div className="mb-3">
          <span className="text-gray-500">租户名: </span>
          <span className="font-semibold">{tenant.userName || tenant.tenancyName}</span>
        </div>
        <Input
          value={editName}
          onChange={(e) => setEditName(e.target.value)}
          placeholder="输入自定义名称"
          className="mb-4"
        />
        <Space>
          <Button onClick={() => setEditNameOpen(false)}>取消</Button>
          <Button type="primary" loading={editSaving} onClick={handleSaveName}>保存</Button>
        </Space>
      </Modal>
    </div>
  )
}
