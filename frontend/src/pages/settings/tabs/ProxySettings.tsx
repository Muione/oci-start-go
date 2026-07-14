import { useCallback, useEffect, useState } from 'react'
import {
  Card, Form, Input, InputNumber, Select, Switch, Button, Alert, message,
} from 'antd'
import { GlobalOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import {
  getOutboundProxy, updateOutboundProxy, testOutboundProxy,
  type OutboundProxyConfig,
} from '@/api/system'

export default function ProxySettings() {
  const { t } = useTranslation()
  const [form] = Form.useForm()
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [loading, setLoading] = useState(false)

  const loadConfig = useCallback(async () => {
    setLoading(true)
    try {
      const data = await getOutboundProxy()
      if (data) {
        form.setFieldsValue({
          type: data.type || 'SOCKS5',
          host: data.host || '',
          port: data.port || 1080,
          username: data.username || '',
          password: data.password || '',
          enabled: data.enabled || false,
        })
      }
    } catch { /* ignore */ }
    finally { setLoading(false) }
  }, [form])

  useEffect(() => { loadConfig() }, [loadConfig])

  async function handleSave() {
    try {
      const values = await form.validateFields()
      setSaving(true)
      await updateOutboundProxy(values as OutboundProxyConfig)
      message.success(t('common.success'))
    } catch (err: any) {
      if (err?.message) message.error(err.message)
    } finally {
      setSaving(false)
    }
  }

  async function handleTest() {
    const host = form.getFieldValue('host')
    const port = form.getFieldValue('port')
    if (!host || !port) {
      message.warning(t('settings.proxyHostRequired'))
      return
    }
    setTesting(true)
    try {
      const res = await testOutboundProxy({
        type: form.getFieldValue('type'),
        host,
        port,
        username: form.getFieldValue('username'),
        password: form.getFieldValue('password'),
      })
      if (res?.reachable) {
        message.success(res.message || t('settings.proxyTestSuccess'))
      } else {
        message.error(t('settings.proxyTestFail'))
      }
    } catch (err: any) {
      message.error(err?.message || t('settings.proxyTestFail'))
    } finally {
      setTesting(false)
    }
  }

  return (
    <div>
      <Alert
        message={t('settings.proxyHint')}
        type="info"
        showIcon
        className="mb-4"
      />

      <Card size="small" title={<><GlobalOutlined className="mr-1" />{t('settings.outboundProxy')}</>} loading={loading}>
        <Form form={form} layout="vertical" initialValues={{ type: 'SOCKS5', port: 1080, enabled: false }}>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Form.Item name="type" label={t('settings.proxyType')} rules={[{ required: true }]}>
              <Select
                options={[
                  { label: 'HTTP', value: 'HTTP' },
                  { label: 'HTTPS', value: 'HTTPS' },
                  { label: 'SOCKS5 (Recommended)', value: 'SOCKS5' },
                ]}
              />
            </Form.Item>
            <Form.Item name="enabled" label={t('common.enabled')} valuePropName="checked">
              <Switch />
            </Form.Item>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Form.Item name="host" label={t('settings.proxyHost')} rules={[{ required: true }]} className="md:col-span-2">
              <Input placeholder={t('settings.proxyHostPlaceholder')} />
            </Form.Item>
            <Form.Item name="port" label={t('settings.proxyPort')} rules={[{ required: true }]}>
              <InputNumber min={1} max={65535} className="w-full" />
            </Form.Item>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Form.Item name="username" label={t('settings.proxyUsername')}>
              <Input placeholder={t('settings.optional')} />
            </Form.Item>
            <Form.Item name="password" label={t('settings.proxyPassword')}>
              <Input.Password placeholder={t('settings.optional')} />
            </Form.Item>
          </div>

          <div className="flex gap-2">
            <Button type="primary" loading={saving} onClick={handleSave}>
              {t('common.save')}
            </Button>
            <Button loading={testing} onClick={handleTest}>
              {t('settings.testConnection')}
            </Button>
            <Button onClick={loadConfig}>
              {t('common.reset')}
            </Button>
          </div>
        </Form>
      </Card>
    </div>
  )
}
