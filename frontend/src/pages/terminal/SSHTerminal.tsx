import { useCallback, useEffect, useRef, useState } from 'react'
import {
  Button, Card, Form, Input, InputNumber, Select, Space, Table, Tag,
  Modal, Radio, message, type TableColumnsType,
} from 'antd'
import { DeleteOutlined, KeyOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import PageHeader from '@/components/common/PageHeader'
import XtermTerminal from '@/components/terminal/XtermTerminal'
import { instanceList } from '@/api/instance'
import { sshKeyList, sshKeyAdd, sshKeyDelete, type SshKey } from '@/api/ssh'
import type { Instance } from '@/types/api'

interface SshForm {
  instanceId: number | undefined
  host: string
  port: number
  username: string
  authType: 'password' | 'key'
  password: string
  savedKeyId: number | undefined
  privateKey: string
  passphrase: string
}

const DEFAULT_FORM: SshForm = {
  instanceId: undefined,
  host: '',
  port: 22,
  username: 'root',
  authType: 'password',
  password: '',
  savedKeyId: undefined,
  privateKey: '',
  passphrase: '',
}

export default function SSHTerminal() {
  const { t } = useTranslation()
  const [form] = Form.useForm<SshForm>()
  const [sshForm, setSshForm] = useState<SshForm>({ ...DEFAULT_FORM })
  const [instances, setInstances] = useState<Instance[]>([])
  const [loadingInstances, setLoadingInstances] = useState(false)
  const [savedKeys, setSavedKeys] = useState<SshKey[]>([])
  const [keyDialogOpen, setKeyDialogOpen] = useState(false)
  const [keyForm, setKeyForm] = useState({ label: '', content: '', passphrase: '' })
  const [keySaving, setKeySaving] = useState(false)
  const [connected, setConnected] = useState(false)
  const [connecting, setConnecting] = useState(false)
  const [wsUrl, setWsUrl] = useState<string | null>(null)
  const rawSendRef = useRef<((data: string) => boolean) | null>(null)

  // ── Load instances ────────────────────────────────────────────
  const loadInstances = useCallback(async () => {
    setLoadingInstances(true)
    try {
      const res = await instanceList({ limit: 200, offset: 0 })
      setInstances(res.items || [])
    } catch (e: unknown) {
      message.error((e as Error).message || 'Failed to load instances')
    } finally {
      setLoadingInstances(false)
    }
  }, [])

  // ── Load SSH keys ─────────────────────────────────────────────
  const loadKeys = useCallback(async () => {
    try {
      const keys = await sshKeyList()
      setSavedKeys(keys || [])
    } catch (e: unknown) {
      message.error((e as Error).message || 'Failed to load keys')
    }
  }, [])

  useEffect(() => {
    loadInstances()
    loadKeys()
  }, [loadInstances, loadKeys])

  // ── Instance selection → auto-fill ────────────────────────────
  const handleInstanceChange = useCallback(
    (instanceId: number) => {
      const inst = instances.find((i) => i.id === instanceId)
      if (!inst) return
      const ip = firstIP(inst.publicIps) || firstIP(inst.privateIps)
      const updates: Partial<SshForm> = { instanceId }
      if (ip) updates.host = ip
      setSshForm((prev) => ({ ...prev, ...updates }))
      form.setFieldsValue(updates)
    },
    [instances, form],
  )

  // ── Connect SSH ───────────────────────────────────────────────
  const handleConnect = useCallback(() => {
    const values = form.getFieldsValue()
    if (!values.host) {
      message.warning('Please enter host address')
      return
    }
    setConnecting(true)
    setConnected(false)
    setSshForm((prev) => ({ ...prev, ...values }))

    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    setWsUrl(`${proto}//${location.host}/ws/ssh`)
  }, [form])

  // ── Send SSH credentials after WS connects ────────────────────
  const handleWsConnect = useCallback(() => {
    const send = rawSendRef.current
    if (!send) return

    const payload: Record<string, unknown> = {
      host: sshForm.host,
      port: sshForm.port,
      username: sshForm.username,
    }

    if (sshForm.authType === 'key') {
      if (sshForm.savedKeyId) {
        payload.keyId = sshForm.savedKeyId
      } else if (sshForm.privateKey) {
        payload.privateKey = sshForm.privateKey
        if (sshForm.passphrase) payload.passphrase = sshForm.passphrase
      } else {
        message.error('Please select a saved key or paste a private key')
        setConnecting(false)
        setWsUrl(null)
        return
      }
    } else {
      payload.password = sshForm.password
    }

    // Send the connect message directly (not wrapped in {type:'input'})
    send(JSON.stringify({ type: 'connect', data: payload }))
    setConnecting(false)
    setConnected(true)
  }, [sshForm])

  // ── Disconnect ────────────────────────────────────────────────
  const handleDisconnect = useCallback(() => {
    setWsUrl(null)
    setConnected(false)
    setConnecting(false)
  }, [])

  // ── SSH key management ────────────────────────────────────────
  const handleAddKey = useCallback(async () => {
    if (!keyForm.label || !keyForm.content) {
      message.warning('Please fill in label and private key')
      return
    }
    setKeySaving(true)
    try {
      await sshKeyAdd(keyForm)
      setKeyForm({ label: '', content: '', passphrase: '' })
      await loadKeys()
      message.success('Key saved')
    } catch (e: unknown) {
      message.error((e as Error).message || 'Failed to save key')
    } finally {
      setKeySaving(false)
    }
  }, [keyForm, loadKeys])

  const handleDeleteKey = useCallback(
    async (id: number) => {
      try {
        await sshKeyDelete(id)
        await loadKeys()
        message.success('Key deleted')
      } catch (e: unknown) {
        message.error((e as Error).message || 'Failed to delete key')
      }
    },
    [loadKeys],
  )

  const keyColumns: TableColumnsType<SshKey> = [
    { title: 'Label', dataIndex: 'label', minWidth: 140 },
    { title: 'Fingerprint', dataIndex: 'fingerprint', minWidth: 200 },
    {
      title: 'Action',
      width: 90,
      render: (_: unknown, row: SshKey) => (
        <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDeleteKey(row.id)}>
          {t('common.delete')}
        </Button>
      ),
    },
  ]

  return (
    <div>
      <PageHeader title={t('nav.ssh')} />

      {/* Connect form */}
      {!connected && (
        <Card className="mb-4">
          <Form form={form} layout="inline" initialValues={DEFAULT_FORM} className="flex flex-wrap gap-y-3">
            <Form.Item label="Instance" name="instanceId" className="!mb-0">
              <Select
                placeholder="Select instance (optional)"
                allowClear
                showSearch
                loading={loadingInstances}
                onFocus={loadInstances}
                onChange={handleInstanceChange}
                style={{ width: 340 }}
                optionFilterProp="label"
                options={instances.map((i) => ({
                  value: i.id,
                  label: `${i.displayName} (${i.instanceId})`,
                }))}
              />
            </Form.Item>
            <Form.Item label="Host" name="host" rules={[{ required: true, message: 'Required' }]} className="!mb-0">
              <Input placeholder="IP address" style={{ width: 180 }} />
            </Form.Item>
            <Form.Item label="Port" name="port" className="!mb-0">
              <InputNumber min={1} max={65535} style={{ width: 100 }} />
            </Form.Item>
            <Form.Item label="Username" name="username" className="!mb-0">
              <Input placeholder="root" style={{ width: 140 }} />
            </Form.Item>
            <Form.Item label="Auth" name="authType" className="!mb-0">
              <Radio.Group>
                <Radio value="password">Password</Radio>
                <Radio value="key">Key</Radio>
              </Radio.Group>
            </Form.Item>

            <Form.Item noStyle shouldUpdate={(prev, cur) => prev.authType !== cur.authType}>
              {() =>
                form.getFieldValue('authType') === 'password' ? (
                  <Form.Item label="Password" name="password" className="!mb-0">
                    <Input.Password placeholder="Password" style={{ width: 160 }} />
                  </Form.Item>
                ) : (
                  <>
                    <Form.Item label="Saved Key" name="savedKeyId" className="!mb-0">
                      <Select
                        placeholder="Select saved key"
                        allowClear
                        style={{ width: 200 }}
                        options={savedKeys.map((k) => ({ value: k.id, label: k.label }))}
                      />
                    </Form.Item>
                    <Form.Item label="Private Key" name="privateKey" className="!mb-0">
                      <Input.TextArea placeholder="Paste PEM private key" rows={2} style={{ width: 300 }} />
                    </Form.Item>
                    <Form.Item label="Passphrase" name="passphrase" className="!mb-0">
                      <Input.Password placeholder="Key passphrase (optional)" style={{ width: 160 }} />
                    </Form.Item>
                  </>
                )
              }
            </Form.Item>

            <Form.Item className="!mb-0">
              <Space>
                <Button type="primary" loading={connecting} onClick={handleConnect}>
                  Connect
                </Button>
                <Button icon={<KeyOutlined />} onClick={() => setKeyDialogOpen(true)}>
                  Manage Keys
                </Button>
              </Space>
            </Form.Item>
          </Form>
        </Card>
      )}

      {/* Terminal */}
      {wsUrl && (
        <Card
          title={
            <Space>
              <Tag color={connected ? 'green' : connecting ? 'gold' : 'default'}>
                {connected ? 'Connected' : connecting ? 'Connecting...' : 'Disconnected'}
              </Tag>
              <span>{sshForm.username}@{sshForm.host}:{sshForm.port}</span>
            </Space>
          }
          extra={
            <Space>
              <Button size="small" onClick={handleDisconnect} danger disabled={!connected && !connecting}>
                Disconnect
              </Button>
            </Space>
          }
          styles={{ body: { padding: 0 } }}
        >
          <XtermTerminal
            wsUrl={wsUrl}
            onConnect={handleWsConnect}
            onDisconnect={() => {
              setConnected(false)
              setConnecting(false)
            }}
            onSendReady={(sendFn) => { rawSendRef.current = sendFn }}
            style={{ height: 'calc(100vh - 280px)' }}
          />
        </Card>
      )}

      {/* Key management dialog */}
      <Modal
        title="Manage SSH Keys (Encrypted Storage)"
        open={keyDialogOpen}
        onCancel={() => setKeyDialogOpen(false)}
        footer={null}
        width={600}
        afterOpenChange={(open) => { if (open) loadKeys() }}
      >
        <Form layout="vertical" className="mb-4">
          <Form.Item label="Label">
            <Input
              value={keyForm.label}
              onChange={(e) => setKeyForm((prev) => ({ ...prev, label: e.target.value }))}
              placeholder="e.g. Instance A root key"
            />
          </Form.Item>
          <Form.Item label="Private Key">
            <Input.TextArea
              value={keyForm.content}
              onChange={(e) => setKeyForm((prev) => ({ ...prev, content: e.target.value }))}
              rows={5}
              placeholder="Paste PEM private key (encrypted at rest with AES-256-GCM)"
            />
          </Form.Item>
          <Form.Item label="Passphrase">
            <Input.Password
              value={keyForm.passphrase}
              onChange={(e) => setKeyForm((prev) => ({ ...prev, passphrase: e.target.value }))}
              placeholder="Key passphrase (optional)"
            />
          </Form.Item>
          <Form.Item>
            <Button type="primary" loading={keySaving} onClick={handleAddKey}>
              {t('common.save')}
            </Button>
          </Form.Item>
        </Form>

        <Table dataSource={savedKeys} columns={keyColumns} rowKey="id" size="small" pagination={false} />

        <div className="mt-2 text-xs text-gray-400">
          Private keys are stored encrypted with AES-256-GCM (master key). The backend decrypts them at connect time; content is never sent back to the frontend.
        </div>
      </Modal>
    </div>
  )
}

// ── Helpers ───────────────────────────────────────────────────

function firstIP(raw: string | null | undefined): string {
  if (!raw) return ''
  try {
    const arr = JSON.parse(raw)
    if (Array.isArray(arr)) return arr[0] || ''
  } catch { /* not JSON */ }
  return raw.split(',')[0].trim()
}
