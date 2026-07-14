import { useCallback, useEffect, useState } from 'react'
import {
  Table, Button, Space, Tag, Modal, Form, Input, InputNumber, Select, Switch, message,
} from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, LockOutlined, UnlockOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import PageHeader from '@/components/common/PageHeader'
import { proxyList, proxySave, proxyDelete } from '@/api/system'
import type { Proxy } from '@/types/api'

export default function ProxyManager() {
  const { t } = useTranslation()
  const [rows, setRows] = useState<Proxy[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editingId, setEditingId] = useState(0)
  const [form] = Form.useForm()

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await proxyList()
      setRows(data || [])
    } catch (err: any) {
      message.error(err?.message || t('common.error'))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => { load() }, [load])

  function openAdd() {
    setEditingId(0)
    form.resetFields()
    form.setFieldsValue({ proxyType: 'SOCKS5', proxyPort: 1080, availableStatus: 1 })
    setModalOpen(true)
  }

  function openEdit(row: Proxy) {
    setEditingId(row.id)
    form.setFieldsValue({
      proxyType: row.proxyType || 'SOCKS5',
      proxyHost: row.proxyHost,
      proxyPort: row.proxyPort,
      proxyUsername: row.proxyUsername || '',
      proxyPassword: row.proxyPassword || '',
      availableStatus: row.availableStatus,
    })
    setModalOpen(true)
  }

  async function handleSave() {
    try {
      const values = await form.validateFields()
      if (!values.proxyHost) {
        message.warning(t('proxy.hostRequired'))
        return
      }
      setSaving(true)
      const fd = new FormData()
      if (editingId) fd.append('id', String(editingId))
      fd.append('proxyType', values.proxyType)
      fd.append('proxyHost', values.proxyHost)
      fd.append('proxyPort', String(values.proxyPort))
      fd.append('proxyUsername', values.proxyUsername || '')
      fd.append('proxyPassword', values.proxyPassword || '')
      fd.append('availableStatus', String(values.availableStatus ?? 1))
      await proxySave(fd)
      message.success(t('common.success'))
      setModalOpen(false)
      await load()
    } catch (err: any) {
      if (err?.message) message.error(err.message)
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(row: Proxy) {
    Modal.confirm({
      title: t('common.confirmDelete'),
      onOk: async () => {
        try {
          await proxyDelete(row.id)
          message.success(t('common.success'))
          await load()
        } catch (err: any) {
          message.error(err?.message || t('common.error'))
        }
      },
    })
  }

  async function handleToggle(row: Proxy, val: boolean) {
    try {
      const fd = new FormData()
      fd.append('id', String(row.id))
      fd.append('proxyType', row.proxyType)
      fd.append('proxyHost', row.proxyHost)
      fd.append('proxyPort', String(row.proxyPort))
      fd.append('proxyUsername', row.proxyUsername || '')
      fd.append('proxyPassword', row.proxyPassword || '')
      fd.append('availableStatus', val ? '1' : '0')
      await proxySave(fd)
      message.success(val ? t('common.enabled') : t('common.disabled'))
      await load()
    } catch (err: any) {
      message.error(err?.message || t('common.error'))
    }
  }

  const columns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 60, align: 'center' as const },
    {
      title: t('proxy.type'),
      dataIndex: 'proxyType',
      key: 'type',
      width: 100,
      align: 'center' as const,
      render: (v: string) => (
        <Tag color={v === 'SOCKS5' ? 'success' : v === 'HTTP' ? 'warning' : 'processing'}>
          {v}
        </Tag>
      ),
    },
    { title: t('proxy.host'), dataIndex: 'proxyHost', key: 'host', minWidth: 180 },
    { title: t('proxy.port'), dataIndex: 'proxyPort', key: 'port', width: 80, align: 'center' as const },
    {
      title: t('proxy.auth'),
      key: 'auth',
      width: 80,
      align: 'center' as const,
      render: (_: any, row: Proxy) =>
        row.proxyUsername ? <LockOutlined style={{ color: '#52c41a' }} /> : <UnlockOutlined style={{ color: '#aaa' }} />,
    },
    {
      title: t('common.status'),
      key: 'status',
      width: 120,
      align: 'center' as const,
      render: (_: any, row: Proxy) => (
        <Switch
          checked={row.availableStatus === 1}
          checkedChildren={t('common.enabled')}
          unCheckedChildren={t('common.disabled')}
          onChange={(val) => handleToggle(row, val)}
        />
      ),
    },
    {
      title: t('common.action'),
      key: 'action',
      width: 130,
      fixed: 'right' as const,
      render: (_: any, row: Proxy) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(row)} />
          <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(row)} />
        </Space>
      ),
    },
  ]

  return (
    <div>
      <PageHeader
        title={t('proxy.title')}
        extra={<Tag>{rows.length} {t('proxy.count')}</Tag>}
        onRefresh={load}
        refreshing={loading}
      >
        <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>
          {t('proxy.add')}
        </Button>
      </PageHeader>

      <Table
        dataSource={rows}
        columns={columns}
        rowKey="id"
        loading={loading}
        bordered
        size="small"
        scroll={{ x: 700 }}
        locale={{ emptyText: t('common.noData') }}
      />

      <Modal
        title={editingId ? t('proxy.edit') : t('proxy.add')}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        confirmLoading={saving}
        destroyOnClose
        width={480}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="proxyType" label={t('proxy.type')}>
            <Select
              options={[
                { label: 'SOCKS5 (Recommended)', value: 'SOCKS5' },
                { label: 'HTTP', value: 'HTTP' },
                { label: 'HTTPS', value: 'HTTPS' },
              ]}
            />
          </Form.Item>
          <Form.Item name="proxyHost" label={t('proxy.host')} rules={[{ required: true }]}>
            <Input placeholder={t('proxy.hostPlaceholder')} />
          </Form.Item>
          <Form.Item name="proxyPort" label={t('proxy.port')} rules={[{ required: true }]}>
            <InputNumber min={1} max={65535} className="w-full" />
          </Form.Item>
          <Form.Item name="proxyUsername" label={t('proxy.username')}>
            <Input placeholder={t('settings.optional')} />
          </Form.Item>
          <Form.Item name="proxyPassword" label={t('proxy.password')}>
            <Input.Password placeholder={t('settings.optional')} />
          </Form.Item>
          <Form.Item name="availableStatus" label={t('common.status')}>
            <Select
              options={[
                { label: t('common.enabled'), value: 1 },
                { label: t('common.disabled'), value: 0 },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
