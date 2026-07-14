import { useCallback, useEffect, useState } from 'react'
import { Button, Card, Select, Space, Table, Tag, message, type TableColumnsType } from 'antd'
import { DeleteOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import PageHeader from '@/components/common/PageHeader'
import XtermTerminal from '@/components/terminal/XtermTerminal'
import {
  instanceList,
  instanceConsoleConnections,
  instanceDeleteConsoleConnection,
  type ConsoleConnection,
} from '@/api/instance'
import type { Instance } from '@/types/api'

export default function Console() {
  const { t } = useTranslation()
  const [instances, setInstances] = useState<Instance[]>([])
  const [loadingInstances, setLoadingInstances] = useState(false)
  const [selectedId, setSelectedId] = useState<number | undefined>()
  const [connections, setConnections] = useState<ConsoleConnection[]>([])
  const [loadingConns, setLoadingConns] = useState(false)
  const [deletingConn, setDeletingConn] = useState<string | null>(null)

  const [connected, setConnected] = useState(false)
  const [connecting, setConnecting] = useState(false)
  const [wsUrl, setWsUrl] = useState<string | null>(null)
  const [statusLog, setStatusLog] = useState('')

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

  useEffect(() => { loadInstances() }, [loadInstances])

  // ── Load console connections for selected instance ────────────
  const loadConnections = useCallback(async () => {
    if (!selectedId) { setConnections([]); return }
    setLoadingConns(true)
    try {
      const data = await instanceConsoleConnections(selectedId)
      setConnections(data || [])
    } catch (e: unknown) {
      message.error((e as Error).message || 'Failed to load connections')
      setConnections([])
    } finally {
      setLoadingConns(false)
    }
  }, [selectedId])

  useEffect(() => { loadConnections() }, [loadConnections])

  // ── Instance change ───────────────────────────────────────────
  const handleInstanceChange = useCallback(
    (id: number) => {
      if (connected || connecting) {
        handleDisconnect()
      }
      setSelectedId(id)
    },
    [connected, connecting], // eslint-disable-line react-hooks/exhaustive-deps
  )

  // ── Connect serial console ────────────────────────────────────
  const handleConnect = useCallback(() => {
    if (!selectedId) return
    setConnecting(true)
    setConnected(false)
    setStatusLog('Connecting to serial console...\n')

    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${proto}//${location.host}/ws/console/serial?instanceId=${encodeURIComponent(String(selectedId))}`
    setWsUrl(url)
  }, [selectedId])

  // ── Disconnect ────────────────────────────────────────────────
  const handleDisconnect = useCallback(() => {
    setWsUrl(null)
    setConnected(false)
    setConnecting(false)
    setStatusLog((prev) => prev + 'Disconnected\n')
  }, [])

  // ── Delete console connection ─────────────────────────────────
  const handleDeleteConnection = useCallback(
    async (connId: string) => {
      if (!selectedId) return
      setDeletingConn(connId)
      try {
        await instanceDeleteConsoleConnection(selectedId, connId)
        setConnections((prev) => prev.filter((c) => c.id !== connId))
        message.success('Connection deleted')
        setTimeout(loadConnections, 3000)
      } catch (e: unknown) {
        message.error((e as Error).message || 'Failed to delete')
      } finally {
        setDeletingConn(null)
      }
    },
    [selectedId, loadConnections],
  )

  // ── Status logging ────────────────────────────────────────────
  const appendStatus = useCallback((msg: string) => {
    setStatusLog((prev) => prev + msg + '\n')
  }, [])

  // ── Connection table columns ──────────────────────────────────
  const connectionColumns: TableColumnsType<ConsoleConnection> = [
    {
      title: 'Connection ID',
      dataIndex: 'id',
      ellipsis: true,
      render: (v: string) => (
        <span title={v} className="font-mono text-xs">
          {v.length > 28 ? v.slice(0, 28) + '…' : v}
        </span>
      ),
    },
    { title: 'State', dataIndex: 'state', width: 110 },
    { title: 'Type', dataIndex: 'connectionType', width: 100 },
    {
      title: 'Action',
      width: 90,
      render: (_: unknown, row: ConsoleConnection) => (
        <Button
          size="small"
          danger
          icon={<DeleteOutlined />}
          loading={deletingConn === row.id}
          onClick={() => handleDeleteConnection(row.id)}
        >
          {t('common.delete')}
        </Button>
      ),
    },
  ]

  return (
    <div>
      <PageHeader title={t('nav.console')} />

      {/* Connect form */}
      <Card className="mb-4">
        <Space wrap>
          <Select
            placeholder="Select instance"
            showSearch
            allowClear
            loading={loadingInstances}
            onFocus={loadInstances}
            onChange={handleInstanceChange}
            value={selectedId}
            style={{ width: 420 }}
            optionFilterProp="label"
            options={instances.map((i) => ({
              value: i.id,
              label: `${i.displayName} (${i.instanceId})`,
            }))}
          />
          <Button
            type="primary"
            loading={connecting}
            disabled={!selectedId}
            onClick={handleConnect}
          >
            {connecting ? 'Connecting...' : 'Connect Serial'}
          </Button>
          <Button
            danger
            disabled={!connected && !connecting}
            onClick={handleDisconnect}
          >
            Disconnect
          </Button>
        </Space>

        {/* Existing console connections */}
        {selectedId && (
          <div className="mt-4">
            <div className="flex items-center justify-between font-semibold text-gray-600 mb-2">
              <span>Existing Console Connections</span>
              <Button size="small" onClick={loadConnections}>
                {t('common.refresh')}
              </Button>
            </div>
            <Table
              dataSource={connections}
              columns={connectionColumns}
              rowKey="id"
              size="small"
              loading={loadingConns}
              pagination={false}
              locale={{ emptyText: 'No console connections (created automatically on connect)' }}
            />
          </div>
        )}

        {/* Status log */}
        {statusLog && (
          <div className="mt-3 p-3 bg-gray-900 text-gray-300 rounded font-mono text-xs max-h-36 overflow-y-auto whitespace-pre-wrap break-all">
            {statusLog}
          </div>
        )}
      </Card>

      {/* Terminal */}
      {wsUrl && (
        <Card
          title={
            <Space>
              <Tag color={connected ? 'green' : connecting ? 'gold' : 'default'}>
                {connected ? 'Connected' : connecting ? 'Connecting...' : 'Disconnected'}
              </Tag>
              <span>Serial Console — {selectedId}</span>
            </Space>
          }
          styles={{ body: { padding: 0 } }}
        >
          <XtermTerminal
            wsUrl={wsUrl}
            onConnect={() => {
              setConnecting(false)
              setConnected(true)
              appendStatus('Serial console connected')
            }}
            onDisconnect={() => {
              setConnected(false)
              setConnecting(false)
              appendStatus('Serial console disconnected')
              loadConnections()
            }}
            onError={(msg) => {
              setConnecting(false)
              appendStatus(`Error: ${msg}`)
            }}
            style={{ height: 560 }}
          />
        </Card>
      )}
    </div>
  )
}
