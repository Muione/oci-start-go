import { useCallback, useEffect, useState } from 'react'
import { Card, Row, Col, Descriptions, Input, Button, Tag, message } from 'antd'
import { CloudOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { getSystemConfig, saveSystemConfigField } from '@/api/system'
import type { SystemConfig } from '@/types/api'

interface FieldDef {
  configKey: string
  label: string
  password?: boolean
  placeholder?: string
}

const CLOUDFLARE_FIELDS: FieldDef[] = [
  { configKey: 'cloudflare.api.token', label: 'API Token', password: true },
]

const EDGEONE_FIELDS: FieldDef[] = [
  { configKey: 'edgeone.secretId', label: 'Secret ID', password: true },
  { configKey: 'edgeone.zoneId', label: 'Zone ID' },
]

export default function DnsSettings() {
  const { t } = useTranslation()
  const [config, setConfig] = useState<SystemConfig>({ strings: {}, bools: {}, appVersion: '' })
  const [editValues, setEditValues] = useState<Record<string, string>>({})
  const [savingKeys, setSavingKeys] = useState<Record<string, boolean>>({})

  const loadConfig = useCallback(async () => {
    try {
      const data = await getSystemConfig()
      if (data) {
        setConfig(data)
        const allKeys = [...CLOUDFLARE_FIELDS, ...EDGEONE_FIELDS].map((f) => f.configKey)
        const vals: Record<string, string> = {}
        for (const k of allKeys) vals[k] = data.strings?.[k] || ''
        setEditValues(vals)
      }
    } catch { /* ignore */ }
  }, [])

  useEffect(() => { loadConfig() }, [loadConfig])

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

  function isConfigured(key: string): boolean {
    return !!config.strings?.[key]
  }

  function renderCard(title: string, icon: React.ReactNode, fields: FieldDef[], configStatusKey: string) {
    return (
      <Card
        size="small"
        title={
          <div className="flex items-center justify-between">
            <span>{icon}{title}</span>
            <Tag color={isConfigured(configStatusKey) ? 'success' : 'default'}>
              {isConfigured(configStatusKey) ? t('common.enabled') : t('dns.notConfigured')}
            </Tag>
          </div>
        }
      >
        <Descriptions column={1} size="small" bordered>
          {fields.map((f) => (
            <Descriptions.Item key={f.configKey} label={f.label}>
              <div className="flex gap-2 items-center">
                <Input
                  size="small"
                  type={f.password ? 'password' : undefined}
                  placeholder={f.placeholder}
                  value={editValues[f.configKey] || ''}
                  onChange={(e) =>
                    setEditValues((s) => ({ ...s, [f.configKey]: e.target.value }))
                  }
                />
                <Button
                  size="small"
                  type="primary"
                  loading={savingKeys[f.configKey]}
                  onClick={() => handleSave(f.configKey)}
                >
                  {t('common.save')}
                </Button>
              </div>
            </Descriptions.Item>
          ))}
        </Descriptions>
      </Card>
    )
  }

  return (
    <Row gutter={[16, 16]}>
      <Col xs={24} md={12}>
        {renderCard(
          t('dns.cloudflare'),
          <CloudOutlined className="mr-1" />,
          CLOUDFLARE_FIELDS,
          'cloudflare.api.token',
        )}
      </Col>
      <Col xs={24} md={12}>
        {renderCard(
          t('dns.edgeone'),
          <CloudOutlined className="mr-1" />,
          EDGEONE_FIELDS,
          'edgeone.secretId',
        )}
      </Col>
    </Row>
  )
}
