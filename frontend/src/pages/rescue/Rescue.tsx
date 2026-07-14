import { useCallback, useEffect, useRef, useState } from 'react'
import { Card, Select, Button, Radio, Input, Descriptions, Progress, Steps, Tag, Alert, Space, message } from 'antd'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router-dom'
import PageHeader from '@/components/common/PageHeader'
import { instanceList } from '@/api/instance'
import type { Instance, RescueProgress } from '@/types/api'

interface HistoryItem {
  time: string
  msg: string
  ok: boolean
}

export default function Rescue() {
  const { t } = useTranslation()
  const [searchParams] = useSearchParams()
  const [instances, setInstances] = useState<Instance[]>([])
  const [loadingInstances, setLoadingInstances] = useState(false)
  const [instanceId, setInstanceId] = useState(searchParams.get('instanceId') || '')
  const [rescueType, setRescueType] = useState(0)
  const [rescueImageId, setRescueImageId] = useState('')
  const [starting, setStarting] = useState(false)
  const [active, setActive] = useState(false)
  const [error, setError] = useState('')
  const [status, setStatus] = useState<RescueProgress>({ step: '', message: '', progress: 0, instanceId: '' })
  const [history, setHistory] = useState<HistoryItem[]>([])
  const wsRef = useRef<WebSocket | null>(null)

  const selectedInstance = instances.find((i) => i.instanceId === instanceId) || null

  const loadInstances = useCallback(async () => {
    setLoadingInstances(true)
    try {
      const res = await instanceList({ limit: 200 })
      setInstances(res.items || [])
    } catch (e: unknown) {
      message.error('加载实例列表失败')
    } finally {
      setLoadingInstances(false)
    }
  }, [])

  useEffect(() => { loadInstances() }, []) // eslint-disable-line react-hooks/exhaustive-deps

  function getWsUrl() {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${proto}//${location.host}/ws/rescue`
  }

  function addHistory(msg: string, ok: boolean) {
    setHistory((prev) => [{ time: new Date().toLocaleTimeString(), msg, ok }, ...prev])
  }

  function startRescue() {
    if (!instanceId.trim()) { message.warning('请先选择一个实例'); return }
    if (rescueType === 1 && !rescueImageId.trim()) { message.warning('NetBoot 模式需要填写救援卷 ID'); return }

    setStarting(true)
    setError('')
    setHistory([])

    try {
      const ws = new WebSocket(getWsUrl())
      wsRef.current = ws

      ws.onopen = () => {
        ws.send(JSON.stringify({
          type: 'init',
          data: { instanceId, rescueType, rescueImageId, tenantId: 0 },
        }))
      }

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data)
          switch (msg.type) {
            case 'info':
              setStarting(false)
              message.info(msg.message)
              break
            case 'error':
              setStarting(false)
              setActive(false)
              setError(msg.message)
              addHistory(`错误: ${msg.message}`, false)
              message.error(msg.message)
              break
            case 'cancelled':
              setActive(false)
              addHistory('操作已取消', false)
              message.warning('救援已取消')
              break
            default:
              setStarting(false)
              setActive(true)
              setStatus(msg)
              if (msg.progress > 0) addHistory(`[${msg.step}] ${msg.message}`, true)
              if (msg.step === 'complete') {
                setActive(false)
                message.success('救援流程完成！')
              }
              break
          }
        } catch { /* ignore */ }
      }

      ws.onclose = () => {
        setStarting(false)
        if (active) { setActive(false); message.warning('WebSocket 连接已断开') }
      }

      ws.onerror = () => {
        setStarting(false)
        setError('WebSocket 连接失败')
        message.error('WebSocket 连接失败')
      }
    } catch (err: unknown) {
      setStarting(false)
      setError((err as Error).message || '连接失败')
    }
  }

  function cancelRescue() {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'cancel', data: { instanceId } }))
      addHistory('已发送取消请求', false)
    }
  }

  function completeRescue() {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'complete', data: { instanceId } }))
      message.success('已发送完成指令，开始还原引导卷')
      addHistory('用户确认救援完成，开始还原', true)
    }
  }

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (wsRef.current) {
        cancelRescue()
        wsRef.current.close()
      }
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div>
      <PageHeader title={t('nav.rescue')} />

      {/* Connection card */}
      <Card size="small" className="mb-4">
        <div className="flex flex-col gap-4">
          <div className="flex items-end gap-3">
            <div>
              <div className="text-xs text-gray-500 mb-1">选择实例</div>
              <Select
                value={instanceId || undefined}
                onChange={setInstanceId}
                placeholder="请选择要救援的实例"
                style={{ width: 420 }}
                showSearch
                optionFilterProp="label"
                loading={loadingInstances}
                allowClear
                options={instances.map((i) => ({
                  label: `${i.displayName} (${i.instanceId})`,
                  value: i.instanceId,
                }))}
              />
            </div>
          </div>

          <div>
            <div className="text-xs text-gray-500 mb-1">救援类型</div>
            <Radio.Group value={rescueType} onChange={(e) => setRescueType(e.target.value)}>
              <Radio value={0}>DD 重建</Radio>
              <Radio value={1}>NetBoot</Radio>
            </Radio.Group>
          </div>

          {rescueType === 1 && (
            <div>
              <div className="text-xs text-gray-500 mb-1">救援卷 ID</div>
              <Input
                value={rescueImageId}
                onChange={(e) => setRescueImageId(e.target.value)}
                placeholder="预创建的急救引导卷 OCID"
                style={{ width: 360 }}
              />
              <div className="text-xs text-gray-400 mt-1">请输入已通过 OCI 控制台或 API 预创建的救援引导卷 OCID</div>
            </div>
          )}

          <Space>
            <Button type="primary" loading={starting} disabled={!instanceId} onClick={startRescue}>
              {starting ? '启动中...' : '开始救援'}
            </Button>
            <Button disabled={!active} onClick={cancelRescue}>取消</Button>
            <Button disabled={!active} type="primary" onClick={completeRescue}>完成救援</Button>
          </Space>
        </div>
      </Card>

      {/* Instance info */}
      {selectedInstance && (
        <Card size="small" title="实例信息" className="mb-4">
          <Descriptions column={3} size="small" bordered>
            <Descriptions.Item label="名称">{selectedInstance.displayName}</Descriptions.Item>
            <Descriptions.Item label="状态">
              <Tag color={selectedInstance.state === 'RUNNING' ? 'green' : 'default'}>{selectedInstance.state}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="规格">{selectedInstance.shape}</Descriptions.Item>
            <Descriptions.Item label="公网 IP"><span className="font-mono">{selectedInstance.publicIps || '-'}</span></Descriptions.Item>
            <Descriptions.Item label="私网 IP"><span className="font-mono">{selectedInstance.privateIps || '-'}</span></Descriptions.Item>
            <Descriptions.Item label="可用域">{selectedInstance.availabilityDomain}</Descriptions.Item>
          </Descriptions>
        </Card>
      )}

      {/* Error */}
      {error && (
        <Alert type="error" message={error} closable onClose={() => setError('')} className="mb-4" />
      )}

      {/* Progress */}
      {active && (
        <Card size="small" title={`救援进度 — ${status.step}`} className="mb-4" extra={<Tag color="warning">进行中</Tag>}>
          <Progress percent={status.progress} status={status.progress === 100 ? 'success' : 'active'} />
          <p className="mt-2 text-sm">{status.message}</p>
        </Card>
      )}

      {/* History */}
      {history.length > 0 && (
        <Card size="small" title="操作历史" className="mb-4">
          <div className="space-y-2 max-h-[300px] overflow-auto">
            {history.map((item, idx) => (
              <div key={idx} className="flex items-start gap-2 text-sm">
                <Tag color={item.ok ? 'success' : 'error'} className="shrink-0">{item.time}</Tag>
                <span>{item.msg}</span>
              </div>
            ))}
          </div>
        </Card>
      )}

      {/* Steps guide (when idle) */}
      {!active && history.length === 0 && (
        <Card size="small" title="救援流程说明">
          <Steps
            direction="vertical"
            current={1}
            status="finish"
            items={[
              { title: '选择实例', description: '从下拉列表中选择要救援的目标实例' },
              { title: '停止实例', description: '安全停止目标实例' },
              { title: '卸载引导卷', description: '分离原始引导卷' },
              { title: '挂载急救卷', description: '挂载急救/恢复引导卷' },
              { title: '启动急救系统', description: '启动实例进入急救模式' },
              { title: 'SSH 救援操作', description: '通过 SSH 连接实例执行 DD 重建或其他修复操作' },
              { title: '完成救援', description: '点击「完成救援」后自动还原引导卷并重启实例' },
            ]}
          />
        </Card>
      )}
    </div>
  )
}
