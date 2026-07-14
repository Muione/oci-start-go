import { useCallback, useEffect, useState } from 'react'
import {
  Card, Row, Col, Descriptions, Input, Button, Tag, Table, Select, Space, message,
} from 'antd'
import { SendOutlined, ReloadOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import {
  getSystemConfig, saveSystemConfigField, testNotification, getNotificationHistory,
  type NotificationHistoryItem,
} from '@/api/system'
import type { SystemConfig } from '@/types/api'

interface FieldDef {
  configKey: string
  label: string
  password?: boolean
  placeholder?: string
}

const CHANNELS: { key: string; label: string; fields: FieldDef[] }[] = [
  {
    key: 'telegram',
    label: 'Telegram',
    fields: [
      { configKey: 'telegram.bot.token', label: 'Bot Token', password: true },
      { configKey: 'telegram.chat.id', label: 'Chat ID' },
    ],
  },
  {
    key: 'dingtalk',
    label: 'DingTalk',
    fields: [
      { configKey: 'dingtalk.webhook', label: 'Webhook URL' },
      { configKey: 'dingtalk.secret', label: 'Secret', password: true },
    ],
  },
  {
    key: 'bark',
    label: 'Bark (iOS)',
    fields: [
      { configKey: 'bark.key', label: 'Device Key' },
      { configKey: 'bark.server', label: 'Server', placeholder: 'https://api.day.app' },
    ],
  },
  {
    key: 'feishu',
    label: 'Feishu',
    fields: [
      { configKey: 'feishu.webhook', label: 'Webhook URL' },
      { configKey: 'feishu.secret', label: 'Secret', password: true },
    ],
  },
]

export default function NotificationSettings() {
  const { t } = useTranslation()
  const [config, setConfig] = useState<SystemConfig>({ strings: {}, bools: {}, appVersion: '' })
  const [editValues, setEditValues] = useState<Record<string, string>>({})
  const [savingKeys, setSavingKeys] = useState<Record<string, boolean>>({})
  const [testingChannel, setTestingChannel] = useState('')
  const [history, setHistory] = useState<NotificationHistoryItem[]>([])
  const [historyLoading, setHistoryLoading] = useState(false)
  const [channelFilter, setChannelFilter] = useState<string>('')

  const loadConfig = useCallback(async () => {
    try {
      const data = await getSystemConfig()
      if (data) {
        setConfig(data)
        const vals: Record<string, string> = {}
        for (const ch of CHANNELS) {
          for (const f of ch.fields) {
            vals[f.configKey] = data.strings?.[f.configKey] || ''
          }
        }
        setEditValues(vals)
      }
    } catch { /* ignore */ }
  }, [])

  const loadHistory = useCallback(async () => {
    setHistoryLoading(true)
    try {
      const params = channelFilter ? { channel: channelFilter } : undefined
      const res = await getNotificationHistory(params)
      setHistory(res?.history ?? [])
    } catch { /* ignore */ }
    finally { setHistoryLoading(false) }
  }, [channelFilter])

  useEffect(() => { loadConfig() }, [loadConfig])
  useEffect(() => { loadHistory() }, [loadHistory])

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

  async function handleTest(channel: string) {
    setTestingChannel(channel)
    try {
      await testNotification(channel)
      message.success(`${channel} test sent`)
    } catch (err: any) {
      message.error(err?.message || t('common.error'))
    } finally {
      setTestingChannel('')
    }
  }

  function isConfigured(key: string): boolean {
    return !!config.strings?.[key]
  }

  const historyColumns = [
    { title: t('settings.time'), dataIndex: 'time', key: 'time', width: 160 },
    {
      title: t('settings.channel'),
      dataIndex: 'channel',
      key: 'channel',
      width: 100,
      render: (v: string) => <Tag>{v}</Tag>,
    },
    { title: t('settings.message'), dataIndex: 'message', key: 'message', ellipsis: true },
    {
      title: t('common.status'),
      dataIndex: 'success',
      key: 'success',
      width: 80,
      render: (v: boolean) => (
        <Tag color={v ? 'success' : 'error'}>{v ? 'OK' : 'FAIL'}</Tag>
      ),
    },
  ]

  return (
    <div>
      {/* Quick actions */}
      <div className="flex items-center justify-between mb-4 p-3 bg-gray-50 rounded">
        <span className="text-sm font-medium text-gray-600">{t('settings.quickActions')}</span>
        <Space>
          <Button
            size="small"
            type="primary"
            icon={<SendOutlined />}
            onClick={() => CHANNELS.forEach((ch) => handleTest(ch.key))}
          >
            {t('settings.testAll')}
          </Button>
        </Space>
      </div>

      <Row gutter={[16, 16]}>
        {CHANNELS.map((ch) => (
          <Col xs={24} md={12} key={ch.key}>
            <Card
              size="small"
              title={
                <div className="flex items-center justify-between">
                  <span>{ch.label}</span>
                  <Tag color={isConfigured(ch.fields[0].configKey) ? 'success' : 'default'}>
                    {isConfigured(ch.fields[0].configKey) ? t('common.enabled') : t('common.disabled')}
                  </Tag>
                </div>
              }
              extra={
                <Button
                  size="small"
                  type="text"
                  icon={<SendOutlined />}
                  loading={testingChannel === ch.key}
                  onClick={() => handleTest(ch.key)}
                />
              }
            >
              <Descriptions column={1} size="small" bordered>
                {ch.fields.map((f) => (
                  <Descriptions.Item key={f.configKey} label={f.label}>
                    <div className="flex gap-2 items-center">
                      <Input
                        size="small"
                        type={f.password ? 'password' : 'text'}
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
          </Col>
        ))}
      </Row>

      {/* Notification History */}
      <Card
        size="small"
        title={t('settings.notifHistory')}
        className="mt-4"
        extra={
          <Space>
            <Select
              value={channelFilter}
              onChange={setChannelFilter}
              allowClear
              placeholder={t('settings.allChannels')}
              style={{ width: 140 }}
              size="small"
              options={[
                { label: t('settings.allChannels'), value: '' },
                { label: 'Email', value: 'email' },
                { label: 'Webhook', value: 'webhook' },
              ]}
            />
            <Button
              size="small"
              type="text"
              icon={<ReloadOutlined />}
              loading={historyLoading}
              onClick={loadHistory}
            />
          </Space>
        }
      >
        <Table
          dataSource={history}
          columns={historyColumns}
          rowKey={(_, i) => String(i)}
          size="small"
          pagination={false}
          locale={{ emptyText: t('common.noData') }}
        />
      </Card>
    </div>
  )
}
