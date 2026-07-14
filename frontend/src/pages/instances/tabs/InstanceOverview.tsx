import { useState } from 'react'
import { Descriptions, Button, Space, Card, Modal, Form, Input, InputNumber, message } from 'antd'
import {
  EditOutlined, SwapOutlined, LinkOutlined,
} from '@ant-design/icons'
import {
  instanceChangeIP, instanceIpv6, instanceModify,
  instanceGetSshConfig, instanceSaveSshConfig, instanceUpdateRemark,
  type ModifyInstancePayload,
} from '@/api/instance'
import ShapeSelect from '@/components/instance/ShapeSelect'
import type { Instance } from '@/types/api'

interface Props {
  instance: Instance
  onRefresh: () => void
}

export default function InstanceOverview({ instance, onRefresh }: Props) {
  // Modify dialog
  const [modifyOpen, setModifyOpen] = useState(false)
  const [modifySaving, setModifySaving] = useState(false)
  const [modifyForm] = Form.useForm()

  // Change IP
  const [changeIpOpen, setChangeIpOpen] = useState(false)
  const [changeIpLoading, setChangeIpLoading] = useState(false)

  // IPv6
  const [ipv6Open, setIpv6Open] = useState(false)
  const [ipv6Loading, setIpv6Loading] = useState(false)

  // SSH
  const [sshOpen, setSshOpen] = useState(false)
  const [sshSaving, setSshSaving] = useState(false)
  const [sshForm] = Form.useForm()

  // Remark
  const [remarkOpen, setRemarkOpen] = useState(false)
  const [remarkSaving, setRemarkSaving] = useState(false)
  const [remarkForm] = Form.useForm()

  // ── Modify ──────────────────────────────────────────────

  function openModify() {
    modifyForm.setFieldsValue({
      displayName: instance.displayName,
      shape: instance.shape,
      ocpus: instance.ocpus,
      memoryInGbs: instance.memoryInGbs,
    })
    setModifyOpen(true)
  }

  async function handleModify() {
    try {
      const values = await modifyForm.validateFields()
      const body: ModifyInstancePayload = {}
      if (values.displayName && values.displayName !== instance.displayName) body.displayName = values.displayName
      if (values.shape && values.shape !== instance.shape) body.shape = values.shape
      if (values.ocpus !== instance.ocpus) body.ocpus = values.ocpus
      if (values.memoryInGbs !== instance.memoryInGbs) body.memoryInGbs = values.memoryInGbs

      if (Object.keys(body).length === 0) {
        message.warning('没有需要修改的配置')
        return
      }
      setModifySaving(true)
      await instanceModify(instance.id, body)
      message.success('修改请求已提交')
      setModifyOpen(false)
      onRefresh()
    } catch (e: unknown) {
      if ((e as Error).message) message.error((e as Error).message || '修改失败')
    } finally {
      setModifySaving(false)
    }
  }

  // ── Change IP ───────────────────────────────────────────

  async function handleChangeIp() {
    setChangeIpLoading(true)
    try {
      const res = await instanceChangeIP(instance.id)
      message.success(`IP 已更换: ${res.oldIp} → ${res.newIp}`)
      setChangeIpOpen(false)
      onRefresh()
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setChangeIpLoading(false)
    }
  }

  // ── IPv6 ────────────────────────────────────────────────

  async function handleEnableIpv6() {
    setIpv6Loading(true)
    try {
      const res = await instanceIpv6(instance.id)
      message.success('IPv6 已启用: ' + (res?.ipv6Address || ''))
      setIpv6Open(false)
      onRefresh()
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setIpv6Loading(false)
    }
  }

  // ── SSH ─────────────────────────────────────────────────

  async function openSsh() {
    setSshOpen(true)
    try {
      const ssh = await instanceGetSshConfig(instance.id)
      sshForm.setFieldsValue({
        username: ssh.username || 'root',
        port: ssh.port || 22,
        password: ssh.password || '',
      })
    } catch {
      sshForm.setFieldsValue({ username: 'root', port: 22, password: '' })
    }
  }

  async function handleSaveSsh() {
    try {
      const values = await sshForm.validateFields()
      setSshSaving(true)
      await instanceSaveSshConfig(instance.id, values)
      message.success('SSH 配置已保存')
      setSshOpen(false)
    } catch (e: unknown) {
      if ((e as Error).message) message.error((e as Error).message)
    } finally {
      setSshSaving(false)
    }
  }

  // ── Remark ──────────────────────────────────────────────

  function openRemark() {
    remarkForm.setFieldsValue({ remark: '' })
    setRemarkOpen(true)
  }

  async function handleSaveRemark() {
    try {
      const values = await remarkForm.validateFields()
      setRemarkSaving(true)
      await instanceUpdateRemark(instance.id, values.remark)
      message.success('备注已更新')
      setRemarkOpen(false)
    } catch (e: unknown) {
      if ((e as Error).message) message.error((e as Error).message)
    } finally {
      setRemarkSaving(false)
    }
  }

  return (
    <div>
      {/* Instance Info */}
      <Card title="基本信息" size="small" className="mb-4">
        <Descriptions column={2} bordered size="small">
          <Descriptions.Item label="实例 ID" span={2}>
            <span className="font-mono text-xs break-all">{instance.instanceId || '—'}</span>
          </Descriptions.Item>
          <Descriptions.Item label="Shape">{instance.shape || '—'}</Descriptions.Item>
          <Descriptions.Item label="架构">{instance.architecture || '—'}</Descriptions.Item>
          <Descriptions.Item label="OCPU">{instance.ocpus ?? '—'}</Descriptions.Item>
          <Descriptions.Item label="内存 (GB)">{instance.memoryInGbs ?? '—'}</Descriptions.Item>
          <Descriptions.Item label="公网 IP">
            <span className="font-mono text-xs">{instance.publicIps || '—'}</span>
          </Descriptions.Item>
          <Descriptions.Item label="私网 IP">
            <span className="font-mono text-xs">{instance.privateIps || '—'}</span>
          </Descriptions.Item>
          <Descriptions.Item label="IPv6">
            <span className="font-mono text-xs">{instance.ipv6Addresses || '—'}</span>
          </Descriptions.Item>
          <Descriptions.Item label="可用域">{instance.availabilityDomain || '—'}</Descriptions.Item>
          <Descriptions.Item label="启动卷 ID" span={2}>
            <span className="font-mono text-xs break-all">{instance.bootVolumeId || '—'}</span>
          </Descriptions.Item>
          <Descriptions.Item label="VNIC IDs" span={2}>
            <span className="font-mono text-xs break-all">{instance.vnicIds || '—'}</span>
          </Descriptions.Item>
          <Descriptions.Item label="最后心跳">{instance.lastHeartbeat || '—'}</Descriptions.Item>
          <Descriptions.Item label="创建时间">{instance.createTime || '—'}</Descriptions.Item>
        </Descriptions>
      </Card>

      {/* Actions */}
      <Card title="操作" size="small" className="mb-4">
        <Space wrap>
          <Button icon={<EditOutlined />} onClick={openModify}>修改配置</Button>
          <Button icon={<SwapOutlined />} onClick={() => setChangeIpOpen(true)}>更换公网 IP</Button>
          <Button icon={<LinkOutlined />} disabled={!!instance.ipv6Addresses} onClick={() => setIpv6Open(true)}>启用 IPv6</Button>
          <Button onClick={openSsh}>SSH 配置</Button>
          <Button onClick={openRemark}>备注</Button>
        </Space>
      </Card>

      {/* Modify Dialog */}
      <Modal
        title="修改实例配置"
        open={modifyOpen}
        onCancel={() => setModifyOpen(false)}
        onOk={handleModify}
        confirmLoading={modifySaving}
        destroyOnClose
        width={520}
      >
        <div className="p-3 mb-4 bg-amber-50 text-amber-700 rounded text-sm">
          提示：修改 Shape 或资源规格可能需要先停止实例
        </div>
        <Form form={modifyForm} layout="vertical">
          <Form.Item label="当前 Shape">
            <Input value={instance.shape} disabled />
          </Form.Item>
          <Form.Item label="显示名称" name="displayName">
            <Input placeholder="修改实例显示名称" />
          </Form.Item>
          <Form.Item label="新 Shape" name="shape">
            <ShapeSelect
              tenantId={instance.tenantId}
              value={modifyForm.getFieldValue('shape')}
              onChange={(val) => {
                modifyForm.setFieldValue('shape', val)
              }}
            />
          </Form.Item>
          <Form.Item label="OCPU" name="ocpus">
            <InputNumber min={1} max={128} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="内存 (GB)" name="memoryInGbs">
            <InputNumber min={1} max={1024} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      {/* Change IP Dialog */}
      <Modal
        title="更换公网 IP"
        open={changeIpOpen}
        onCancel={() => setChangeIpOpen(false)}
        onOk={handleChangeIp}
        confirmLoading={changeIpLoading}
        destroyOnClose
      >
        <div className="p-3 mb-4 bg-amber-50 text-amber-700 rounded text-sm">
          更换 IP 会释放当前公网 IP 并分配新的临时公网 IP
        </div>
        <Descriptions column={1} bordered size="small">
          <Descriptions.Item label="实例">{instance.displayName}</Descriptions.Item>
          <Descriptions.Item label="当前 IP">{instance.publicIps || '—'}</Descriptions.Item>
        </Descriptions>
      </Modal>

      {/* IPv6 Dialog */}
      <Modal
        title="启用 IPv6"
        open={ipv6Open}
        onCancel={() => setIpv6Open(false)}
        onOk={handleEnableIpv6}
        confirmLoading={ipv6Loading}
        destroyOnClose
      >
        <Descriptions column={1} bordered size="small">
          <Descriptions.Item label="实例">{instance.displayName}</Descriptions.Item>
          <Descriptions.Item label="当前 IPv6">{instance.ipv6Addresses || '未启用'}</Descriptions.Item>
        </Descriptions>
      </Modal>

      {/* SSH Config Dialog */}
      <Modal
        title="SSH 配置"
        open={sshOpen}
        onCancel={() => setSshOpen(false)}
        onOk={handleSaveSsh}
        confirmLoading={sshSaving}
        destroyOnClose
        width={480}
      >
        <Form form={sshForm} layout="vertical">
          <Form.Item label="用户名" name="username" rules={[{ required: true }]}>
            <Input placeholder="root" />
          </Form.Item>
          <Form.Item label="端口" name="port" rules={[{ required: true }]}>
            <InputNumber min={1} max={65535} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="密码" name="password">
            <Input.Password placeholder="SSH 密码" />
          </Form.Item>
        </Form>
      </Modal>

      {/* Remark Dialog */}
      <Modal
        title="备注"
        open={remarkOpen}
        onCancel={() => setRemarkOpen(false)}
        onOk={handleSaveRemark}
        confirmLoading={remarkSaving}
        destroyOnClose
      >
        <Form form={remarkForm} layout="vertical">
          <Form.Item name="remark">
            <Input.TextArea rows={4} placeholder="添加备注信息..." />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
