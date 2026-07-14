import { useState } from 'react'
import { Modal, Form, InputNumber, Alert, message } from 'antd'
import { vnicBatchCreate } from '@/api/vnic'
import type { BatchVnicResult } from '@/types/api'

interface Props {
  open: boolean
  onClose: () => void
  instanceId: string
  onSuccess: () => void
}

export default function BatchCreateVnicModal({ open, onClose, instanceId, onSuccess }: Props) {
  const [saving, setSaving] = useState(false)
  const [result, setResult] = useState<BatchVnicResult | null>(null)
  const [form] = Form.useForm()

  async function handleCreate(values: { vnicCount: number; ipv6Count: number }) {
    setSaving(true)
    setResult(null)
    try {
      const res = await vnicBatchCreate(instanceId, '', values.vnicCount, values.ipv6Count)
      setResult(res)
      if (res.allSuccessful) {
        message.success(`成功创建 ${res.successfulVnicCount} 个 VNIC`)
      } else {
        message.warning(res.summary || '部分 VNIC 创建失败')
      }
      onSuccess()
    } catch (e: unknown) {
      message.error((e as Error).message || '批量创建失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open={open}
      title="批量创建 VNIC + IPv6"
      onCancel={onClose}
      onOk={() => form.submit()}
      confirmLoading={saving}
      destroyOnClose
    >
      <Form form={form} layout="vertical" onFinish={handleCreate} initialValues={{ vnicCount: 1, ipv6Count: 0 }}>
        <Form.Item name="vnicCount" label="VNIC 数量">
          <InputNumber min={1} max={32} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="ipv6Count" label="每 VNIC IPv6 数">
          <InputNumber min={0} max={32} style={{ width: '100%' }} />
        </Form.Item>
      </Form>
      {result && (
        <Alert
          message={result.summary}
          type={result.allSuccessful ? 'success' : 'warning'}
          showIcon
          className="mt-3"
        />
      )}
    </Modal>
  )
}
