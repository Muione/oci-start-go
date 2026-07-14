import { useCallback, useEffect, useState } from 'react'
import {
  Table, Button, Tag, Space, Card, Modal, Form, Input, Select, InputNumber, Row, Col, message,
} from 'antd'
import { PlusOutlined, ReloadOutlined, EditOutlined, PauseCircleOutlined, PlayCircleOutlined, DeleteOutlined, SettingOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import PageHeader from '@/components/common/PageHeader'
import { bootList, bootSave, bootDelete, bootToggle, bootSystemStatus, bootTenants, type BootTenant } from '@/api/boot'
import type { BootTask, EngineStatus } from '@/types/api'

function statusTag(s: number) {
  return s === 1 ? 'processing' : s === 2 ? 'success' : 'default'
}

function statusText(s: number) {
  return s === 1 ? '运行中' : s === 2 ? '已完成' : s === 0 ? '已停用' : '未知'
}

const emptyForm = {
  bootId: '', tenantId: 0, ocpu: 4, memory: 24, disk: 100,
  loopTime: 6, instanceCount: 1, architecture: 'ARM', rootPassword: '',
  imageId: '', operatingSystem: 'Canonical Ubuntu', operatingSystemVersion: '22.04',
  dataGap: '', notifyFlag: 'NO', remark: '', cloudType: 1,
}

export default function BootTasks() {
  const { t } = useTranslation()
  const [rows, setRows] = useState<BootTask[]>([])
  const [loading, setLoading] = useState(false)
  const [sysStatus, setSysStatus] = useState<EngineStatus | null>(null)
  const [tenantList, setTenantList] = useState<BootTenant[]>([])
  const [addOpen, setAddOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [editing, setEditing] = useState(false)
  const [form] = Form.useForm()

  const engineActive = (sysStatus?.running ?? false) && (sysStatus?.totalTasks ?? 0) > 0

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [tasks, status] = await Promise.all([bootList(), bootSystemStatus()])
      setRows(tasks || [])
      setSysStatus(status)
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setLoading(false)
    }
  }, [])

  const loadTenants = useCallback(async () => {
    try { setTenantList(await bootTenants()) } catch { /* ignore */ }
  }, [])

  useEffect(() => { load(); loadTenants() }, []) // eslint-disable-line react-hooks/exhaustive-deps

  function openAdd() {
    setEditing(false)
    form.setFieldsValue({ ...emptyForm })
    setAddOpen(true)
  }

  function openEdit(row: BootTask) {
    setEditing(true)
    form.setFieldsValue({
      bootId: row.bootId, tenantId: row.tenantId,
      ocpu: row.ocpu, memory: row.memory, disk: row.disk,
      loopTime: row.loopTime, instanceCount: row.instanceCount,
      architecture: row.architecture, rootPassword: '',
      imageId: row.imageId || '', operatingSystem: row.operatingSystem || '',
      operatingSystemVersion: row.operatingSystemVersion || '',
      dataGap: row.dataGap || '', remark: row.remark || '',
      cloudType: row.cloudType || 1,
    })
    setAddOpen(true)
  }

  async function handleSave() {
    setSaving(true)
    try {
      await bootSave(form.getFieldsValue())
      message.success(editing ? '更新成功' : '创建成功')
      setAddOpen(false)
      load()
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  async function handleToggle(row: BootTask) {
    try {
      await bootToggle(row.bootId, row.status !== 1)
      message.success(row.status === 1 ? '已暂停' : '已启用')
      load()
    } catch (e: unknown) { message.error((e as Error).message) }
  }

  async function handleDelete(row: BootTask) {
    Modal.confirm({
      title: '确认删除',
      content: '确定删除此任务？',
      okType: 'danger',
      onOk: async () => {
        try {
          await bootDelete(row.bootId)
          message.success('已删除')
          load()
        } catch (e: unknown) { message.error((e as Error).message) }
      },
    })
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <PageHeader title={t('nav.boot')} />
        <Space>
          <Tag color={engineActive ? 'success' : 'default'}>{engineActive ? '引擎运行中' : '引擎已停止'}</Tag>
          <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>新建任务</Button>
          <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>刷新</Button>
        </Space>
      </div>

      {/* Engine Status */}
      {sysStatus && (
        <Card size="small" className="mb-4" title={<><SettingOutlined className="mr-2" />引擎状态</>} extra={<Tag color={sysStatus.running ? 'success' : 'error'}>{sysStatus.running ? '活跃' : '停止'}</Tag>}>
          <div className="grid grid-cols-3 md:grid-cols-6 gap-3">
            {[
              { label: '总任务', value: sysStatus.totalTasks ?? 0 },
              { label: '运行中', value: sysStatus.runningTasks ?? 0, color: '#52c41a' },
              { label: '活跃 Key', value: sysStatus.activeKeyCount ?? 0 },
              { label: '批次大小', value: sysStatus.batchSize ?? '-' },
              { label: '父池', value: `${sysStatus.parentPool?.active ?? 0}/${sysStatus.parentPool?.queue ?? 0}` },
              { label: 'API池', value: `${sysStatus.apiPool?.active ?? 0}/${sysStatus.apiPool?.completed ?? 0}` },
            ].map((s) => (
              <div key={s.label} className="text-center p-2 border rounded">
                <div className="text-xl font-bold" style={{ color: (s as any).color }}>{s.value}</div>
                <div className="text-xs text-gray-500">{s.label}</div>
              </div>
            ))}
          </div>
        </Card>
      )}

      {/* Task Table */}
      <Table dataSource={rows} loading={loading} rowKey="bootId" size="small">
        <Table.Column title="任务ID" dataIndex="bootId" ellipsis render={(v: string) => <span className="font-mono text-xs">{v?.substring(0, 20)}...</span>} />
        <Table.Column title="租户" dataIndex="tenantId" width={80} render={(v: number) => <Tag>#{v}</Tag>} />
        <Table.Column title="架构" dataIndex="architecture" width={80} render={(v: string) => <Tag color={v === 'ARM' ? 'green' : 'orange'}>{v}</Tag>} />
        <Table.Column title="规格" width={150} render={(_: unknown, r: BootTask) => <span className="font-mono text-xs">{r.ocpu}C / {r.memory}G / {r.disk}GB</span>} />
        <Table.Column title="镜像" ellipsis render={(_: unknown, r: BootTask) => `${r.operatingSystem || '-'} ${r.operatingSystemVersion || ''}`} />
        <Table.Column title="间隔(s)" dataIndex="loopTime" width={80} align="center" />
        <Table.Column title="进度" width={100} align="center" render={(_: unknown, r: BootTask) => (
          <span><span className="text-green-600">{r.successCount || 0}</span> / {r.instanceCount || 0}</span>
        )} />
        <Table.Column title="状态" dataIndex="status" width={90} align="center" render={(v: number) => <Tag color={statusTag(v)}>{statusText(v)}</Tag>} />
        <Table.Column title="失败" dataIndex="failCount" width={60} align="center" render={(v: number) => <span style={{ color: v > 0 ? '#ff4d4f' : undefined }}>{v || 0}</span>} />
        <Table.Column title="下次执行" dataIndex="nextExecutionTime" width={155} />
        <Table.Column
          title="操作"
          width={200}
          render={(_: unknown, row: BootTask) => (
            <Space>
              <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(row)} />
              <Button size="small" type={row.status === 1 ? 'default' : 'primary'} icon={row.status === 1 ? <PauseCircleOutlined /> : <PlayCircleOutlined />} onClick={() => handleToggle(row)}>
                {row.status === 1 ? '暂停' : '启用'}
              </Button>
              <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(row)} />
            </Space>
          )}
        />
      </Table>

      {/* Add/Edit Dialog */}
      <Modal open={addOpen} title={editing ? '编辑任务' : '新建任务'} onCancel={() => setAddOpen(false)} onOk={handleSave} confirmLoading={saving} width={680} destroyOnClose>
        <Form form={form} layout="vertical" initialValues={{ ...emptyForm }}>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="tenantId" label="租户" rules={[{ required: true }]}>
                <Select showSearch optionFilterProp="label" options={tenantList.map((t) => ({ label: `${t.name} (${t.region})`, value: t.id }))} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="architecture" label="架构" rules={[{ required: true }]}>
                <Select options={[{ label: 'ARM (Ampere)', value: 'ARM' }, { label: 'AMD (Intel/AMD)', value: 'AMD' }]} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={8}><Form.Item name="ocpu" label="OCPU"><InputNumber min={1} max={128} style={{ width: '100%' }} /></Form.Item></Col>
            <Col span={8}><Form.Item name="memory" label="内存 (GB)"><InputNumber min={1} max={1024} style={{ width: '100%' }} /></Form.Item></Col>
            <Col span={8}><Form.Item name="disk" label="磁盘 (GB)"><InputNumber min={50} max={32768} style={{ width: '100%' }} /></Form.Item></Col>
          </Row>
          <Row gutter={16}>
            <Col span={8}><Form.Item name="loopTime" label="循环间隔(秒)"><InputNumber min={3} max={3600} style={{ width: '100%' }} /></Form.Item></Col>
            <Col span={8}><Form.Item name="instanceCount" label="目标实例数"><InputNumber min={1} max={100} style={{ width: '100%' }} /></Form.Item></Col>
            <Col span={8}><Form.Item name="cloudType" label="云类型"><Select options={[{ label: 'Oracle Cloud', value: 1 }]} /></Form.Item></Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}><Form.Item name="imageId" label="镜像ID"><Input placeholder="留空自动选择" /></Form.Item></Col>
            <Col span={12}><Form.Item name="operatingSystem" label="操作系统"><Input placeholder="如: Canonical Ubuntu" /></Form.Item></Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}><Form.Item name="operatingSystemVersion" label="系统版本"><Input placeholder="如: 22.04" /></Form.Item></Col>
            <Col span={12}><Form.Item name="rootPassword" label="Root密码"><Input.Password placeholder="留空自动生成" /></Form.Item></Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}><Form.Item name="dataGap" label="时间窗口" extra="留空表示全天候运行"><Input placeholder="如: 00:00-23:59" /></Form.Item></Col>
            <Col span={12}><Form.Item name="remark" label="备注"><Input placeholder="可选" /></Form.Item></Col>
          </Row>
        </Form>
      </Modal>
    </div>
  )
}
