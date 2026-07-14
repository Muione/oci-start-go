import { useEffect, useState } from 'react'
import { Form, Input, Switch, Button, Space, message } from 'antd'
import {
  tenantEmailGet, tenantEmailSave, tenantEmailDelete,
  tenantEmailEnable, tenantEmailDisable,
  type EmailConfig,
} from '@/api/tenant'

interface Props {
  tenantId: number
}

const EMPTY_FORM: EmailConfig = {
  domainName: '',
  smtpHost: '',
  smtpPort: '587',
  smtpUsername: '',
  smtpPassword: '',
  senderEmail: '',
  active: false,
}

export default function TenantEmail({ tenantId }: Props) {
  const [form, setForm] = useState<EmailConfig>(EMPTY_FORM)
  const [configId, setConfigId] = useState<number | null>(null)
  const [saving, setSaving] = useState(false)
  const [enabling, setEnabling] = useState(false)
  const [disabling, setDisabling] = useState(false)

  useEffect(() => {
    tenantEmailGet(tenantId)
      .then((cfg) => {
        if (cfg) {
          setConfigId(cfg.id ?? null)
          setForm({
            domainName: cfg.domainName || '',
            smtpHost: cfg.smtpHost || '',
            smtpPort: cfg.smtpPort || '587',
            smtpUsername: cfg.smtpUsername || '',
            smtpPassword: cfg.smtpPassword || '',
            senderEmail: cfg.senderEmail || '',
            active: cfg.active === true || (cfg.active as unknown as number) === 1,
          })
        }
      })
      .catch(() => setConfigId(null))
  }, [tenantId])

  function updateField<K extends keyof EmailConfig>(key: K, value: EmailConfig[K]) {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  async function handleSave() {
    setSaving(true)
    try {
      await tenantEmailSave(tenantId, form)
      message.success('邮件配置已保存')
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete() {
    try {
      await tenantEmailDelete(tenantId)
      message.success('已删除')
      setForm(EMPTY_FORM)
      setConfigId(null)
    } catch (e: unknown) {
      message.error((e as Error).message)
    }
  }

  async function handleEnable() {
    if (!form.domainName) {
      message.warning('请先填写域名')
      return
    }
    setEnabling(true)
    try {
      await tenantEmailEnable(tenantId, form.domainName)
      message.success('邮件服务已启用')
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setEnabling(false)
    }
  }

  async function handleDisable() {
    if (!configId) {
      message.warning('未找到邮件配置 ID')
      return
    }
    setDisabling(true)
    try {
      await tenantEmailDisable(configId)
      message.success('邮件服务已禁用')
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setDisabling(false)
    }
  }

  return (
    <div>
      <Form layout="vertical" style={{ maxWidth: 500 }}>
        <Form.Item label="域名">
          <Input value={form.domainName} onChange={(e) => updateField('domainName', e.target.value)} placeholder="example.com" />
        </Form.Item>
        <Form.Item label="SMTP 主机">
          <Input value={form.smtpHost} onChange={(e) => updateField('smtpHost', e.target.value)} placeholder="smtp.example.com" />
        </Form.Item>
        <Form.Item label="SMTP 端口">
          <Input value={form.smtpPort} onChange={(e) => updateField('smtpPort', e.target.value)} placeholder="587" />
        </Form.Item>
        <Form.Item label="用户名">
          <Input value={form.smtpUsername} onChange={(e) => updateField('smtpUsername', e.target.value)} placeholder="邮箱用户名" />
        </Form.Item>
        <Form.Item label="密码">
          <Input.Password value={form.smtpPassword} onChange={(e) => updateField('smtpPassword', e.target.value)} placeholder="邮箱密码" />
        </Form.Item>
        <Form.Item label="发件人">
          <Input value={form.senderEmail} onChange={(e) => updateField('senderEmail', e.target.value)} placeholder="noreply@example.com" />
        </Form.Item>
        <Form.Item label="启用">
          <Switch checked={form.active} onChange={(v) => updateField('active', v)} />
        </Form.Item>
        <Form.Item>
          <Space>
            <Button type="primary" loading={saving} onClick={handleSave}>保存</Button>
            <Button danger onClick={handleDelete}>删除配置</Button>
          </Space>
        </Form.Item>
      </Form>

      <div className="flex gap-2 mt-4">
        <Button type="primary" loading={enabling} onClick={handleEnable}>
          启用邮件服务（OCI + DNS）
        </Button>
        <Button loading={disabling} onClick={handleDisable}>
          禁用邮件服务
        </Button>
      </div>
      <p className="text-xs text-gray-400 mt-2">
        启用：配置 OCI Email Delivery 域名 + Cloudflare DNS 记录。禁用：拆除 OCI 邮件资源 + DNS 记录。
      </p>
    </div>
  )
}
