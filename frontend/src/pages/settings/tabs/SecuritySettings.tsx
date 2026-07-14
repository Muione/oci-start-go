import { useCallback, useEffect, useState } from 'react'
import {
  Card, Row, Col, Button, Tag, Table, Modal, Input, Space, Switch, Form, message,
} from 'antd'
import {
  SafetyOutlined, DeleteOutlined, LogoutOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import {
  getMfaStatus, mfaTotpSetup, mfaTotpVerify, mfaDisable,
  getLoginHistory, getSessions, deleteSession, logoutAllSessions,
  getSystemConfig, saveSystemConfigField,
  type MfaStatus, type TotpSetup, type LoginHistoryItem, type SessionInfo,
} from '@/api/system'
import type { SystemConfig } from '@/types/api'

export default function SecuritySettings() {
  const { t } = useTranslation()

  // MFA state
  const [mfaStatus, setMfaStatus] = useState<MfaStatus>({ enabled: false, configured: false })
  const [totpOpen, setTotpOpen] = useState(false)
  const [totpData, setTotpData] = useState<TotpSetup | null>(null)
  const [totpCode, setTotpCode] = useState('')
  const [totpLoading, setTotpLoading] = useState(false)
  const [mfaDisabling, setMfaDisabling] = useState(false)

  // Login history
  const [loginHistory, setLoginHistory] = useState<LoginHistoryItem[]>([])
  const [historyLoading, setHistoryLoading] = useState(false)

  // Sessions
  const [sessions, setSessions] = useState<SessionInfo[]>([])
  const [sessionsLoading, setSessionsLoading] = useState(false)

  // Turnstile
  const [config, setConfig] = useState<SystemConfig>({ strings: {}, bools: {}, appVersion: '' })
  const [turnstileForm] = Form.useForm()
  const [turnstileSaving, setTurnstileSaving] = useState(false)

  // GitHub OAuth
  const [githubForm] = Form.useForm()
  const [githubSaving, setGithubSaving] = useState(false)

  // Google OAuth
  const [googleForm] = Form.useForm()
  const [googleSaving, setGoogleSaving] = useState(false)

  const loadMfa = useCallback(async () => {
    try {
      const data = await getMfaStatus()
      setMfaStatus(data || { enabled: false, configured: false })
    } catch { /* ignore */ }
  }, [])

  const loadHistory = useCallback(async () => {
    setHistoryLoading(true)
    try {
      const data = await getLoginHistory(20)
      setLoginHistory(data?.items || [])
    } catch { /* ignore */ }
    finally { setHistoryLoading(false) }
  }, [])

  const loadSessions = useCallback(async () => {
    setSessionsLoading(true)
    try {
      const data = await getSessions()
      setSessions(data?.sessions || [])
    } catch { /* ignore */ }
    finally { setSessionsLoading(false) }
  }, [])

  const loadConfig = useCallback(async () => {
    try {
      const data = await getSystemConfig()
      if (data) {
        setConfig(data)
        const strs = data.strings || {}
        turnstileForm.setFieldsValue({
          enabled: data.bools?.['turnstile.enabled'] || false,
          siteKey: strs['turnstile.site.key'] || '',
          secretKey: strs['turnstile.secret.key'] || '',
        })
        githubForm.setFieldsValue({
          enabled: data.bools?.['github.enabled'] || false,
          clientId: strs['github.client.id'] || '',
          clientSecret: strs['github.client.secret'] || '',
          redirectUri: strs['github.client.redirect.uri'] || '',
        })
        googleForm.setFieldsValue({
          enabled: data.bools?.['google.enabled'] || false,
          clientId: strs['google.client.id'] || '',
          clientSecret: strs['google.client.secret'] || '',
          redirectUri: strs['google.client.redirect.uri'] || '',
        })
      }
    } catch { /* ignore */ }
  }, [turnstileForm, githubForm, googleForm])

  useEffect(() => {
    loadMfa()
    loadHistory()
    loadSessions()
    loadConfig()
  }, [loadMfa, loadHistory, loadSessions, loadConfig])

  // ── MFA ────────────────────────────────────────────────────────────

  async function handleTotpSetup() {
    try {
      setTotpLoading(true)
      const data = await mfaTotpSetup()
      setTotpData(data)
      setTotpCode('')
      setTotpOpen(true)
    } catch (err: any) {
      message.error(err?.message || t('common.error'))
    } finally {
      setTotpLoading(false)
    }
  }

  async function handleTotpVerify() {
    if (!totpCode || totpCode.length !== 6) {
      message.warning(t('settings.mfaCodeRequired'))
      return
    }
    setTotpLoading(true)
    try {
      await mfaTotpVerify(totpCode)
      message.success(t('settings.mfaEnabled'))
      setTotpOpen(false)
      await loadMfa()
    } catch (err: any) {
      message.error(err?.message || t('common.error'))
    } finally {
      setTotpLoading(false)
    }
  }

  async function handleMfaDisable() {
    Modal.confirm({
      title: t('settings.mfaDisableConfirm'),
      onOk: async () => {
        setMfaDisabling(true)
        try {
          await mfaDisable()
          message.success(t('settings.mfaDisabled'))
          await loadMfa()
        } catch (err: any) {
          message.error(err?.message || t('common.error'))
        } finally {
          setMfaDisabling(false)
        }
      },
    })
  }

  // ── Sessions ───────────────────────────────────────────────────────

  async function handleDeleteSession(id: string) {
    try {
      await deleteSession(id)
      message.success(t('common.success'))
      await loadSessions()
    } catch (err: any) {
      message.error(err?.message || t('common.error'))
    }
  }

  async function handleLogoutAll() {
    Modal.confirm({
      title: t('settings.logoutAllConfirm'),
      onOk: async () => {
        try {
          await logoutAllSessions()
          message.success(t('settings.logoutAllSuccess'))
          await loadSessions()
        } catch (err: any) {
          message.error(err?.message || t('common.error'))
        }
      },
    })
  }

  // ── Turnstile ──────────────────────────────────────────────────────

  async function handleSaveTurnstile() {
    setTurnstileSaving(true)
    try {
      const values = turnstileForm.getFieldsValue()
      await saveSystemConfigField('turnstile.enabled', String(values.enabled))
      if (values.siteKey) await saveSystemConfigField('turnstile.site.key', values.siteKey)
      if (values.secretKey) await saveSystemConfigField('turnstile.secret.key', values.secretKey)
      message.success(t('common.success'))
      await loadConfig()
    } catch (err: any) {
      message.error(err?.message || t('common.error'))
    } finally {
      setTurnstileSaving(false)
    }
  }

  // ── GitHub OAuth ───────────────────────────────────────────────────

  async function handleSaveGithub() {
    setGithubSaving(true)
    try {
      const values = githubForm.getFieldsValue()
      await saveSystemConfigField('github.enabled', String(values.enabled))
      if (values.clientId) await saveSystemConfigField('github.client.id', values.clientId)
      if (values.clientSecret) await saveSystemConfigField('github.client.secret', values.clientSecret)
      if (values.redirectUri) await saveSystemConfigField('github.client.redirect.uri', values.redirectUri)
      message.success(t('common.success'))
      await loadConfig()
    } catch (err: any) {
      message.error(err?.message || t('common.error'))
    } finally {
      setGithubSaving(false)
    }
  }

  // ── Google OAuth ───────────────────────────────────────────────────

  async function handleSaveGoogle() {
    setGoogleSaving(true)
    try {
      const values = googleForm.getFieldsValue()
      await saveSystemConfigField('google.enabled', String(values.enabled))
      if (values.clientId) await saveSystemConfigField('google.client.id', values.clientId)
      if (values.clientSecret) await saveSystemConfigField('google.client.secret', values.clientSecret)
      if (values.redirectUri) await saveSystemConfigField('google.client.redirect.uri', values.redirectUri)
      message.success(t('common.success'))
      await loadConfig()
    } catch (err: any) {
      message.error(err?.message || t('common.error'))
    } finally {
      setGoogleSaving(false)
    }
  }

  // ── Tables ─────────────────────────────────────────────────────────

  const historyCols = [
    { title: t('settings.time'), dataIndex: 'created_at', key: 'time', width: 180 },
    { title: 'IP', dataIndex: 'ip_address', key: 'ip', width: 150 },
    {
      title: t('common.status'),
      dataIndex: 'success',
      key: 'status',
      width: 80,
      render: (v: boolean) => <Tag color={v ? 'success' : 'error'}>{v ? 'OK' : 'FAIL'}</Tag>,
    },
  ]

  const sessionCols = [
    { title: 'IP', dataIndex: 'ip_address', key: 'ip', width: 150 },
    { title: t('settings.userAgent'), dataIndex: 'user_agent', key: 'ua', ellipsis: true },
    { title: t('settings.lastActive'), dataIndex: 'last_active_at', key: 'active', width: 180 },
    {
      title: t('common.action'),
      key: 'action',
      width: 80,
      render: (_: any, record: SessionInfo) => (
        <Button
          size="small"
          danger
          icon={<DeleteOutlined />}
          onClick={() => handleDeleteSession(record.id)}
        />
      ),
    },
  ]

  return (
    <div>
      <Row gutter={[16, 16]}>
        {/* MFA */}
        <Col xs={24} sm={8}>
          <Card size="small" className="h-full">
            <div className="flex items-center justify-between mb-2">
              <span className="font-semibold">{t('settings.mfaTitle')}</span>
              <Tag color={mfaStatus.enabled ? 'success' : 'default'}>
                {mfaStatus.enabled ? t('common.enabled') : t('common.disabled')}
              </Tag>
            </div>
            <p className="text-xs text-gray-500 mb-3">{t('settings.mfaDesc')}</p>
            {mfaStatus.enabled ? (
              <Button danger size="small" loading={mfaDisabling} onClick={handleMfaDisable}>
                {t('common.disabled')}
              </Button>
            ) : (
              <Button type="primary" size="small" loading={totpLoading} onClick={handleTotpSetup}>
                {t('settings.setupTotp')}
              </Button>
            )}
          </Card>
        </Col>

        {/* Turnstile */}
        <Col xs={24} sm={8}>
          <Card size="small" className="h-full">
            <div className="flex items-center justify-between mb-2">
              <span className="font-semibold">Turnstile</span>
              <Tag color={config.bools?.['turnstile.enabled'] ? 'success' : 'default'}>
                {config.bools?.['turnstile.enabled'] ? t('common.enabled') : t('common.disabled')}
              </Tag>
            </div>
            <p className="text-xs text-gray-500 mb-3">{t('settings.turnstileDesc')}</p>
            <Form form={turnstileForm} layout="vertical" size="small">
              <Form.Item name="enabled" valuePropName="checked" className="mb-1">
                <Switch size="small" />
              </Form.Item>
              <Form.Item name="siteKey" className="mb-1">
                <Input placeholder="Site Key" size="small" />
              </Form.Item>
              <Form.Item name="secretKey" className="mb-1">
                <Input.Password placeholder="Secret Key" size="small" />
              </Form.Item>
            </Form>
            <Button type="primary" size="small" loading={turnstileSaving} onClick={handleSaveTurnstile}>
              {t('common.save')}
            </Button>
          </Card>
        </Col>

        {/* GitHub OAuth */}
        <Col xs={24} sm={8}>
          <Card size="small" className="h-full">
            <div className="flex items-center justify-between mb-2">
              <span className="font-semibold">GitHub OAuth</span>
              <Tag color={config.bools?.['github.enabled'] ? 'success' : 'default'}>
                {config.bools?.['github.enabled'] ? t('common.enabled') : t('common.disabled')}
              </Tag>
            </div>
            <p className="text-xs text-gray-500 mb-3">{t('settings.githubOauthDesc')}</p>
            <Form form={githubForm} layout="vertical" size="small">
              <Form.Item name="enabled" valuePropName="checked" className="mb-1">
                <Switch size="small" />
              </Form.Item>
              <Form.Item name="clientId" className="mb-1">
                <Input placeholder="Client ID" size="small" />
              </Form.Item>
              <Form.Item name="clientSecret" className="mb-1">
                <Input.Password placeholder="Client Secret" size="small" />
              </Form.Item>
              <Form.Item name="redirectUri" className="mb-1">
                <Input placeholder="Redirect URI" size="small" />
              </Form.Item>
            </Form>
            <Button type="primary" size="small" loading={githubSaving} onClick={handleSaveGithub}>
              {t('common.save')}
            </Button>
          </Card>
        </Col>
      </Row>

      {/* Google OAuth */}
      <Row gutter={[16, 16]} className="mt-4">
        <Col xs={24} sm={8}>
          <Card size="small" className="h-full">
            <div className="flex items-center justify-between mb-2">
              <span className="font-semibold">Google OAuth</span>
              <Tag color={config.bools?.['google.enabled'] ? 'success' : 'default'}>
                {config.bools?.['google.enabled'] ? t('common.enabled') : t('common.disabled')}
              </Tag>
            </div>
            <p className="text-xs text-gray-500 mb-3">{t('settings.googleOauthDesc')}</p>
            <Form form={googleForm} layout="vertical" size="small">
              <Form.Item name="enabled" valuePropName="checked" className="mb-1">
                <Switch size="small" />
              </Form.Item>
              <Form.Item name="clientId" className="mb-1">
                <Input placeholder="Client ID" size="small" />
              </Form.Item>
              <Form.Item name="clientSecret" className="mb-1">
                <Input.Password placeholder="Client Secret" size="small" />
              </Form.Item>
              <Form.Item name="redirectUri" className="mb-1">
                <Input placeholder="Redirect URI" size="small" />
              </Form.Item>
            </Form>
            <Button type="primary" size="small" loading={googleSaving} onClick={handleSaveGoogle}>
              {t('common.save')}
            </Button>
          </Card>
        </Col>
      </Row>

      {/* Login History */}
      <Card
        size="small"
        title={t('settings.loginHistory')}
        className="mt-4"
        extra={
          <Button size="small" type="text" icon={<SafetyOutlined />} loading={historyLoading} onClick={loadHistory}>
            {t('common.refresh')}
          </Button>
        }
      >
        <Table
          dataSource={loginHistory}
          columns={historyCols}
          rowKey={(_, i) => String(i)}
          size="small"
          pagination={false}
          locale={{ emptyText: t('common.noData') }}
        />
      </Card>

      {/* Sessions */}
      <Card
        size="small"
        title={
          <span>
            {t('settings.sessionManagement')}
            <Tag className="ml-2">{sessions.length}</Tag>
          </span>
        }
        className="mt-4"
        extra={
          <Space>
            <Button size="small" loading={sessionsLoading} onClick={loadSessions}>
              {t('common.refresh')}
            </Button>
            <Button size="small" danger icon={<LogoutOutlined />} onClick={handleLogoutAll}>
              {t('settings.logoutAll')}
            </Button>
          </Space>
        }
      >
        <Table
          dataSource={sessions}
          columns={sessionCols}
          rowKey="id"
          size="small"
          pagination={false}
          locale={{ emptyText: t('common.noData') }}
        />
      </Card>

      {/* TOTP Dialog */}
      <Modal
        title={t('settings.setupTotp')}
        open={totpOpen}
        onCancel={() => setTotpOpen(false)}
        onOk={handleTotpVerify}
        confirmLoading={totpLoading}
        destroyOnClose
      >
        {totpData?.qrCodeBase64 && (
          <div className="text-center">
            <p>{t('settings.scanQr')}</p>
            <img
              src={totpData.qrCodeBase64}
              alt="TOTP QR"
              className="w-[200px] h-[200px] mx-auto"
            />
            <p className="text-xs text-gray-500 mt-1">
              {t('settings.secret')}: <code>{totpData.secret}</code>
            </p>
            <Input
              placeholder={t('settings.enterCode')}
              maxLength={6}
              className="w-[200px] mt-3 mx-auto"
              value={totpCode}
              onChange={(e) => setTotpCode(e.target.value)}
              onPressEnter={handleTotpVerify}
            />
          </div>
        )}
      </Modal>
    </div>
  )
}
