import { useEffect, useState } from 'react'
import {
  Card, Descriptions, Button, Space, Table, Tag, Modal, Form,
  InputNumber, Input, Select, message, type TableColumnsType,
} from 'antd'
import { instanceUpdateVpu, instanceResizeBootVolume } from '@/api/instance'
import { backupList, backupCreate, backupDelete, backupCopy } from '@/api/backup'
import { tenantRegionsSubscribed } from '@/api/tenant'
import type { Instance, BootVolumeBackup, RegionSubInfo } from '@/types/api'

interface Props {
  instance: Instance
  onRefresh: () => void
}

const VPU_OPTIONS = [
  { value: 10, label: 'Balanced (10 VPUs/GB)', description: '标准性能' },
  { value: 20, label: 'Higher Performance (20 VPUs/GB)', description: '更高性能' },
  { value: 30, label: 'Ultra High (30 VPUs/GB)', description: '超高性能' },
  { value: 60, label: 'Ultra High (60 VPUs/GB)', description: '超高性能' },
  { value: 120, label: 'Ultra High (120 VPUs/GB)', description: '最高性能' },
]

export default function InstanceDisk({ instance, onRefresh }: Props) {
  const [backups, setBackups] = useState<BootVolumeBackup[]>([])
  const [loadingBackups, setLoadingBackups] = useState(false)

  // VPU modal
  const [vpuOpen, setVpuOpen] = useState(false)
  const [vpuSaving, setVpuSaving] = useState(false)
  const [selectedVpu, setSelectedVpu] = useState(Number(instance.vpusPerGb) || 10)

  // Resize modal
  const [resizeOpen, setResizeOpen] = useState(false)
  const [resizeSaving, setResizeSaving] = useState(false)
  const [newSize, setNewSize] = useState(instance.bootVolumeSizeInGbs || 50)

  // Create backup modal
  const [createBackupOpen, setCreateBackupOpen] = useState(false)
  const [createBackupSaving, setCreateBackupSaving] = useState(false)
  const [backupName, setBackupName] = useState('')

  // Copy backup modal
  const [copyBackupOpen, setCopyBackupOpen] = useState(false)
  const [copyBackupSaving, setCopyBackupSaving] = useState(false)
  const [copyBackupId, setCopyBackupId] = useState('')
  const [targetRegion, setTargetRegion] = useState('')
  const [copyBackupName, setCopyBackupName] = useState('')
  const [regions, setRegions] = useState<RegionSubInfo[]>([])

  const loadBackups = () => {
    if (!instance.tenantId) return
    setLoadingBackups(true)
    backupList(instance.tenantId)
      .then((d) => setBackups(d || []))
      .catch((e: unknown) => message.error((e as Error).message || '加载备份失败'))
      .finally(() => setLoadingBackups(false))
  }

  useEffect(() => {
    loadBackups()
  }, [instance.tenantId]) // eslint-disable-line react-hooks/exhaustive-deps

  // ── VPU ─────────────────────────────────────────────────

  async function handleSaveVpu() {
    setVpuSaving(true)
    try {
      await instanceUpdateVpu(instance.id, selectedVpu)
      message.success('VPU 调整成功')
      setVpuOpen(false)
      onRefresh()
    } catch (e: unknown) {
      message.error((e as Error).message || 'VPU 调整失败')
    } finally {
      setVpuSaving(false)
    }
  }

  // ── Resize ──────────────────────────────────────────────

  async function handleResize() {
    if (newSize < (instance.bootVolumeSizeInGbs || 0)) {
      message.warning('磁盘只能增大不能缩小')
      return
    }
    setResizeSaving(true)
    try {
      await instanceResizeBootVolume(instance.id, newSize)
      message.success('磁盘大小调整成功')
      setResizeOpen(false)
      onRefresh()
    } catch (e: unknown) {
      message.error((e as Error).message || '磁盘调整失败')
    } finally {
      setResizeSaving(false)
    }
  }

  // ── Create Backup ───────────────────────────────────────

  async function handleCreateBackup() {
    if (!backupName.trim()) {
      message.warning('请输入备份名称')
      return
    }
    setCreateBackupSaving(true)
    try {
      await backupCreate({
        tenantId: instance.tenantId,
        instanceId: instance.instanceId,
        displayName: backupName.trim(),
      })
      message.success('备份创建成功')
      setCreateBackupOpen(false)
      setBackupName('')
      loadBackups()
    } catch (e: unknown) {
      message.error((e as Error).message || '备份创建失败')
    } finally {
      setCreateBackupSaving(false)
    }
  }

  // ── Delete Backup ───────────────────────────────────────

  function confirmDeleteBackup(backup: BootVolumeBackup) {
    Modal.confirm({
      title: '删除备份',
      content: `确定删除备份 "${backup.displayName}"？`,
      okType: 'primary',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await backupDelete(backup.id)
          message.success('备份已删除')
          loadBackups()
        } catch (e: unknown) {
          message.error((e as Error).message || '删除失败')
        }
      },
    })
  }

  // ── Copy Backup ─────────────────────────────────────────

  async function openCopyBackup(backupId: string) {
    setCopyBackupId(backupId)
    setCopyBackupName('')
    setTargetRegion('')
    setCopyBackupOpen(true)
    if (instance.tenantId) {
      try {
        const data = await tenantRegionsSubscribed(instance.tenantId)
        setRegions(data || [])
      } catch { /* ignore */ }
    }
  }

  async function handleCopyBackup() {
    if (!targetRegion) {
      message.warning('请选择目标区域')
      return
    }
    setCopyBackupSaving(true)
    try {
      await backupCopy({
        tenantId: instance.tenantId,
        backupId: copyBackupId,
        targetRegion,
        displayName: copyBackupName || undefined,
      })
      message.success('备份复制任务已提交')
      setCopyBackupOpen(false)
      loadBackups()
    } catch (e: unknown) {
      message.error((e as Error).message || '备份复制失败')
    } finally {
      setCopyBackupSaving(false)
    }
  }

  const backupColumns: TableColumnsType<BootVolumeBackup> = [
    { title: '名称', dataIndex: 'displayName', minWidth: 160 },
    { title: '大小 (GB)', dataIndex: 'sizeInGBs', width: 100 },
    {
      title: '状态',
      dataIndex: 'lifecycleState',
      width: 100,
      render: (v: string) => (
        <Tag color={v === 'AVAILABLE' ? 'success' : 'info'}>{v}</Tag>
      ),
    },
    { title: '创建时间', dataIndex: 'timeCreated', width: 180 },
    {
      title: '操作',
      width: 160,
      fixed: 'right',
      render: (_: unknown, row: BootVolumeBackup) => (
        <Space size="small">
          <Button size="small" onClick={() => openCopyBackup(row.id)}>复制</Button>
          <Button size="small" danger onClick={() => confirmDeleteBackup(row)}>删除</Button>
        </Space>
      ),
    },
  ]

  return (
    <div>
      {/* Boot Volume Info */}
      <Card title="启动卷信息" size="small" className="mb-4">
        <Descriptions column={2} bordered size="small">
          <Descriptions.Item label="名称">{instance.bootVolumeName || instance.displayName || '—'}</Descriptions.Item>
          <Descriptions.Item label="OCID">
            <span className="font-mono text-xs break-all">{instance.bootVolumeId || '—'}</span>
          </Descriptions.Item>
          <Descriptions.Item label="大小">{instance.bootVolumeSizeInGbs ?? '—'} GB</Descriptions.Item>
          <Descriptions.Item label="VPU/GB">{instance.vpusPerGb ?? '—'}</Descriptions.Item>
        </Descriptions>
      </Card>

      {/* Boot Volume Actions */}
      <Card title="启动卷操作" size="small" className="mb-4">
        <Space>
          <Button onClick={() => { setSelectedVpu(Number(instance.vpusPerGb) || 10); setVpuOpen(true) }}>
            调整 VPU
          </Button>
          <Button onClick={() => { setNewSize(instance.bootVolumeSizeInGbs || 50); setResizeOpen(true) }}>
            调整大小
          </Button>
          <Button type="primary" onClick={() => { setBackupName(''); setCreateBackupOpen(true) }}>
            创建备份
          </Button>
        </Space>
      </Card>

      {/* Backup List */}
      <Card
        title="备份列表"
        size="small"
        extra={<Button size="small" onClick={loadBackups}>刷新</Button>}
      >
        <Table
          dataSource={backups}
          columns={backupColumns}
          rowKey="id"
          loading={loadingBackups}
          size="small"
          bordered
          pagination={false}
          scroll={{ x: 700 }}
          locale={{ emptyText: '暂无备份' }}
        />
      </Card>

      {/* VPU Modal */}
      <Modal
        title="调整启动卷性能级别"
        open={vpuOpen}
        onCancel={() => setVpuOpen(false)}
        onOk={handleSaveVpu}
        confirmLoading={vpuSaving}
        destroyOnClose
      >
        <div className="p-3 mb-4 bg-amber-50 text-amber-700 rounded text-sm">
          调整 VPU 需要实例处于停止状态
        </div>
        <Select
          value={selectedVpu}
          onChange={setSelectedVpu}
          style={{ width: '100%' }}
          options={VPU_OPTIONS.map((opt) => ({
            value: opt.value,
            label: (
              <div>
                <span>{opt.label}</span>
                <span className="text-gray-400 ml-2">{opt.description}</span>
              </div>
            ),
          }))}
        />
      </Modal>

      {/* Resize Modal */}
      <Modal
        title="调整启动卷大小"
        open={resizeOpen}
        onCancel={() => setResizeOpen(false)}
        onOk={handleResize}
        confirmLoading={resizeSaving}
        destroyOnClose
      >
        <Form layout="vertical">
          <Form.Item label="当前大小">
            <span>{instance.bootVolumeSizeInGbs ?? 0} GB</span>
          </Form.Item>
          <Form.Item label="新大小">
            <div className="flex items-center gap-2">
              <InputNumber
                value={newSize}
                onChange={(v) => setNewSize(v ?? instance.bootVolumeSizeInGbs ?? 0)}
                min={instance.bootVolumeSizeInGbs || 0}
                step={10}
              />
              <span>GB</span>
            </div>
          </Form.Item>
        </Form>
      </Modal>

      {/* Create Backup Modal */}
      <Modal
        title="创建启动卷备份"
        open={createBackupOpen}
        onCancel={() => setCreateBackupOpen(false)}
        onOk={handleCreateBackup}
        confirmLoading={createBackupSaving}
        destroyOnClose
      >
        <Form layout="vertical">
          <Form.Item label="备份名称" required>
            <Input
              value={backupName}
              onChange={(e) => setBackupName(e.target.value)}
              placeholder="输入备份名称"
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* Copy Backup Modal */}
      <Modal
        title="跨区域复制备份"
        open={copyBackupOpen}
        onCancel={() => setCopyBackupOpen(false)}
        onOk={handleCopyBackup}
        confirmLoading={copyBackupSaving}
        destroyOnClose
      >
        <Form layout="vertical">
          <Form.Item label="目标区域" required>
            <Select
              value={targetRegion || undefined}
              onChange={setTargetRegion}
              placeholder="选择目标区域"
              options={regions.map((r) => ({
                value: r.regionKey,
                label: `${r.regionName} (${r.regionKey})`,
              }))}
            />
          </Form.Item>
          <Form.Item label="备份名称">
            <Input
              value={copyBackupName}
              onChange={(e) => setCopyBackupName(e.target.value)}
              placeholder="可选，留空使用默认名称"
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
