import { useCallback, useEffect, useState } from 'react'
import {
  Card, Descriptions, Input, Button, Tag, Table, Space, message, Form,
} from 'antd'
import { LockOutlined, ReloadOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import {
  getSystemConfig, saveSystemConfigField, sslList, sslIssue,
  type SslCertInfo,
} from '@/api/system'
import type { SystemConfig } from '@/types/api'

export default function SslSettings() {
  const { t } = useTranslation()
  const [config, setConfig] = useState<SystemConfig>({ strings: {}, bools: {}, appVersion: '' })
  const [editValues, setEditValues] = useState<Record<string, string>>({})
  const [savingKeys, setSavingKeys] = useState<Record<string, boolean>>({})
  const [certs, setCerts] = useState<SslCertInfo[]>([])
  const [certsLoading, setCertsLoading] = useState(false)
  const [issuing, setIssuing] = useState(false)
  const [issueForm] = Form.useForm()

  const loadConfig = useCallback(async () => {
    try {
      const data = await getSystemConfig()
      if (data) {
        setConfig(data)
        setEditValues({
          'ssl.domain': data.strings?.['ssl.domain'] || '',
          'ssl.email': data.strings?.['ssl.email'] || '',
        })
      }
    } catch { /* ignore */ }
  }, [])

  const loadCerts = useCallback(async () => {
    setCertsLoading(true)
    try {
      const data = await sslList()
      setCerts(data || [])
    } catch { /* ignore */ }
    finally { setCertsLoading(false) }
  }, [])

  useEffect(() => {
    loadConfig()
    loadCerts()
  }, [loadConfig, loadCerts])

  async function handleSave(key: string) {
    setSavingKeys((s) => ({ ...s, [key]: true }))
    try {
      await saveSystemConfigField(key, editValues[key] || '')
      message.success(t('common.success'))
      await loadConfig()
    } catch (err: any) {
      message.error(err?.message || t('common.error'))
    } finally {
      setSavingKeys((s) => ({ ...s, [key]: false }))
    }
  }

  async function handleIssue() {
    try {
      const values = await issueForm.validateFields()
      setIssuing(true)
      await sslIssue({ domain: values.domain, email: values.email })
      message.success('Certificate issued')
      issueForm.resetFields()
      await loadCerts()
    } catch (err: any) {
      if (err?.message) message.error(err.message)
    } finally {
      setIssuing(false)
    }
  }

  const domain = config.strings?.['ssl.domain'] || ''
  const notAfter = config.strings?.['ssl.notAfter'] || ''
  const isStaging = config.bools?.['ssl.staging']

  let sslStatus = 'not-configured'
  if (domain) {
    if (!notAfter) sslStatus = 'configured'
    else {
      const days = Math.ceil((new Date(notAfter).getTime() - Date.now()) / 86400000)
      if (days < 0) sslStatus = 'expired'
      else if (days < 30) sslStatus = 'expiring'
      else sslStatus = 'valid'
    }
  }

  const statusColor: Record<string, string> = {
    valid: 'success', expiring: 'warning', expired: 'error', configured: 'processing', 'not-configured': 'default',
  }
  const statusLabel: Record<string, string> = {
    valid: t('settings.sslValid'),
    expiring: t('settings.sslExpiring'),
    expired: t('settings.sslExpired'),
    configured: t('settings.sslConfigured'),
    'not-configured': t('settings.sslNotConfigured'),
  }

  const certColumns = [
    { title: t('settings.domain'), dataIndex: 'domain', key: 'domain' },
    {
      title: t('common.status'),
      dataIndex: 'certificateStatus',
      key: 'status',
      render: (v: string) => <Tag color={v === 'valid' ? 'success' : v === 'expired' ? 'error' : 'processing'}>{v}</Tag>,
    },
    { title: t('settings.issueDate'), dataIndex: 'issueDate', key: 'issueDate' },
    { title: t('settings.expireDate'), dataIndex: 'expireDate', key: 'expireDate' },
  ]

  return (
    <div>
      <Card
        size="small"
        title={
          <div className="flex items-center justify-between">
            <span><LockOutlined className="mr-1" />{t('settings.certStatus')}</span>
            <Tag color={statusColor[sslStatus]}>{statusLabel[sslStatus]}</Tag>
          </div>
        }
      >
        <Descriptions column={2} size="small" bordered>
          <Descriptions.Item label={t('settings.domain')}>
            <div className="flex gap-2 items-center">
              <Input
                size="small"
                value={editValues['ssl.domain'] || ''}
                onChange={(e) => setEditValues((s) => ({ ...s, 'ssl.domain': e.target.value }))}
              />
              <Button size="small" type="primary" loading={savingKeys['ssl.domain']} onClick={() => handleSave('ssl.domain')}>
                {t('common.save')}
              </Button>
            </div>
          </Descriptions.Item>
          <Descriptions.Item label="Email">
            <div className="flex gap-2 items-center">
              <Input
                size="small"
                value={editValues['ssl.email'] || ''}
                onChange={(e) => setEditValues((s) => ({ ...s, 'ssl.email': e.target.value }))}
              />
              <Button size="small" type="primary" loading={savingKeys['ssl.email']} onClick={() => handleSave('ssl.email')}>
                {t('common.save')}
              </Button>
            </div>
          </Descriptions.Item>
          <Descriptions.Item label={t('settings.sslMode')}>
            <Tag color={isStaging ? 'warning' : 'success'}>
              {isStaging ? 'Staging' : 'Production'}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label={t('settings.autoRenew')}>
            <Tag color="info">04:00 daily</Tag>
          </Descriptions.Item>
        </Descriptions>

        <Space className="mt-3">
          <Button size="small" icon={<ReloadOutlined />} onClick={loadCerts}>
            {t('common.refresh')}
          </Button>
        </Space>
      </Card>

      {/* Certificate list */}
      <Card size="small" title={t('settings.certList')} className="mt-4">
        <Table
          dataSource={certs}
          columns={certColumns}
          rowKey="id"
          size="small"
          loading={certsLoading}
          pagination={false}
          locale={{ emptyText: t('common.noData') }}
        />
      </Card>

      {/* Issue new cert */}
      <Card size="small" title={t('settings.issueCert')} className="mt-4">
        <Form form={issueForm} layout="inline">
          <Form.Item name="domain" rules={[{ required: true }]}>
            <Input placeholder={t('settings.domain')} />
          </Form.Item>
          <Form.Item name="email" rules={[{ required: true, type: 'email' }]}>
            <Input placeholder="Email" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" loading={issuing} onClick={handleIssue}>
              {t('settings.issueCert')}
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}
