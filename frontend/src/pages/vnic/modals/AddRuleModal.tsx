import { useState } from 'react'
import { Modal, Form, Select, Input, Radio, message } from 'antd'
import { vnicAddSecurityRule } from '@/api/vnic'

interface Props {
  open: boolean
  onClose: () => void
  tenantId: number
  onSuccess: () => void
}

export default function AddRuleModal({ open, onClose, tenantId, onSuccess }: Props) {
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm()

  const protocol = Form.useWatch('protocol', form) || 'all'
  const ruleType = Form.useWatch('type', form) || 'ingress'

  async function handleAdd(values: { type: string; protocol: string; source: string; ports?: string; icmpType?: string }) {
    setSaving(true)
    try {
      await vnicAddSecurityRule({
        tenantId,
        type: values.type,
        protocol: values.protocol,
        source: values.source,
        ports: values.ports || undefined,
        icmpType: values.protocol === 'icmp' ? values.icmpType : undefined,
      })
      message.success('规则添加成功')
      onSuccess()
      onClose()
      form.resetFields()
    } catch (e: unknown) {
      message.error((e as Error).message || '规则添加失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal open={open} title="添加安全规则" onCancel={onClose} onOk={() => form.submit()} confirmLoading={saving} destroyOnClose>
      <Form form={form} layout="vertical" onFinish={handleAdd} initialValues={{ type: 'ingress', protocol: 'all', source: '0.0.0.0/0' }}>
        <Form.Item name="type" label="类型">
          <Radio.Group options={[
            { label: '入站', value: 'ingress' },
            { label: '出站', value: 'egress' },
          ]} />
        </Form.Item>
        <Form.Item name="protocol" label="协议">
          <Select options={[
            { label: '全部 (all)', value: 'all' },
            { label: 'TCP', value: 'tcp' },
            { label: 'UDP', value: 'udp' },
            { label: 'ICMP', value: 'icmp' },
          ]} />
        </Form.Item>
        <Form.Item name="source" label={ruleType === 'ingress' ? '源 CIDR' : '目标 CIDR'}>
          <Input placeholder="0.0.0.0/0" />
        </Form.Item>
        {(protocol === 'tcp' || protocol === 'udp') && (
          <Form.Item name="ports" label="端口">
            <Input placeholder="80 或 8080-9090" />
          </Form.Item>
        )}
        {protocol === 'icmp' && (
          <Form.Item name="icmpType" label="ICMP Type">
            <Input placeholder="8, 0" />
          </Form.Item>
        )}
      </Form>
    </Modal>
  )
}
