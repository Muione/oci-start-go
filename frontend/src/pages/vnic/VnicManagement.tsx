import { useCallback, useEffect, useState } from 'react'
import { Tabs, Select, Descriptions, Tag, Card, Empty, message } from 'antd'
import { useTranslation } from 'react-i18next'
import PageHeader from '@/components/common/PageHeader'
import { vnicLoadData, vnicRefresh, vcnList } from '@/api/vnic'
import { tenantList } from '@/api/tenant'
import { instanceList } from '@/api/instance'
import type { VnicLoadData, Tenant, Instance } from '@/types/api'
import VnicList from './tabs/VnicList'
import SecurityRules from './tabs/SecurityRules'
import VcnPanel from './tabs/VcnPanel'
import NetworkConfig from './tabs/NetworkConfig'

export default function VnicManagement() {
  const { t } = useTranslation()
  const [tenantOptions, setTenantOptions] = useState<Tenant[]>([])
  const [instanceOptions, setInstanceOptions] = useState<Instance[]>([])
  const [tenantId, setTenantId] = useState<number | undefined>()
  const [instanceId, setInstanceId] = useState('')
  const [selectedInstance, setSelectedInstance] = useState<Instance | null>(null)
  const [vnicData, setVnicData] = useState<VnicLoadData | null>(null)
  const [loading, setLoading] = useState(false)
  const [vcnId, setVcnId] = useState('')
  const [activeTab, setActiveTab] = useState('vnic')

  const loadTenants = useCallback(async () => {
    try {
      const data = await tenantList()
      setTenantOptions(data || [])
      if (data.length === 1) {
        setTenantId(data[0].id)
      }
    } catch { /* ignore */ }
  }, [])

  const loadInstances = useCallback(async (tid: number) => {
    try {
      const data = await instanceList({ limit: 200 })
      const filtered = (data.items || []).filter((i) => i.tenantId === tid)
      setInstanceOptions(filtered)
      if (filtered.length === 1) {
        setInstanceId(filtered[0].instanceId)
        setSelectedInstance(filtered[0])
      }
    } catch { setInstanceOptions([]) }
  }, [])

  const loadVnicData = useCallback(async (instId: string) => {
    setLoading(true)
    try {
      const data = await vnicLoadData(instId)
      setVnicData(data)
    } catch (e: unknown) {
      message.error((e as Error).message)
      setVnicData(null)
    } finally {
      setLoading(false)
    }
  }, [])

  const loadVcnId = useCallback(async (compartmentId: string) => {
    if (!compartmentId || !tenantId) return
    try {
      const data = await vcnList(tenantId, compartmentId)
      if (data?.length > 0) setVcnId(data[0].id)
    } catch { /* ignore */ }
  }, [tenantId])

  useEffect(() => { loadTenants() }, []) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { if (tenantId) loadInstances(tenantId) }, [tenantId]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (instanceId) {
      loadVnicData(instanceId)
      const inst = instanceOptions.find((i) => i.instanceId === instanceId)
      setSelectedInstance(inst || null)
      if (inst?.compartmentId) loadVcnId(inst.compartmentId)
    } else {
      setSelectedInstance(null)
      setVnicData(null)
    }
  }, [instanceId]) // eslint-disable-line react-hooks/exhaustive-deps

  async function handleRefresh() {
    if (!instanceId) return
    setLoading(true)
    try {
      const data = await vnicRefresh(instanceId)
      setVnicData(data)
      message.success('VNIC 数据已刷新')
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div>
      <PageHeader title={t('nav.vnic')} />

      {/* Instance selector */}
      <Card size="small" className="mb-4">
        <div className="flex items-end gap-4 flex-wrap">
          <div>
            <div className="text-xs text-gray-500 mb-1">租户</div>
            <Select
              value={tenantId}
              onChange={(v) => { setTenantId(v); setInstanceId(''); setSelectedInstance(null); setVnicData(null) }}
              placeholder="选择租户"
              style={{ width: 240 }}
              showSearch
              optionFilterProp="label"
              allowClear
              options={tenantOptions.map((t) => ({ label: t.userName || t.tenancy, value: t.id }))}
            />
          </div>
          <div>
            <div className="text-xs text-gray-500 mb-1">实例</div>
            <Select
              value={instanceId || undefined}
              onChange={setInstanceId}
              placeholder="选择实例"
              style={{ width: 360 }}
              showSearch
              optionFilterProp="label"
              allowClear
              disabled={!tenantId}
              options={instanceOptions.map((i) => ({ label: `${i.displayName} (${i.state})`, value: i.instanceId }))}
            />
          </div>
        </div>

        {selectedInstance && (
          <Descriptions column={4} size="small" bordered className="mt-3">
            <Descriptions.Item label="名称"><strong>{selectedInstance.displayName}</strong></Descriptions.Item>
            <Descriptions.Item label="实例 OCID"><span className="font-mono text-xs">{selectedInstance.instanceId}</span></Descriptions.Item>
            <Descriptions.Item label="区域">{selectedInstance.tenantName || '-'}</Descriptions.Item>
            <Descriptions.Item label="Shape">{selectedInstance.shape || '-'}</Descriptions.Item>
            <Descriptions.Item label="状态">
              <Tag color={selectedInstance.state === 'RUNNING' ? 'green' : selectedInstance.state === 'STOPPED' ? 'red' : 'default'}>
                {selectedInstance.state || '-'}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="公网 IP"><span className="font-mono">{selectedInstance.publicIps || '-'}</span></Descriptions.Item>
            <Descriptions.Item label="私网 IP"><span className="font-mono">{selectedInstance.privateIps || '-'}</span></Descriptions.Item>
            <Descriptions.Item label="架构">{selectedInstance.architecture || '-'}</Descriptions.Item>
          </Descriptions>
        )}
      </Card>

      {!selectedInstance ? (
        <Empty description="请先选择租户和实例以管理 VNIC" />
      ) : (
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            { key: 'vnic', label: 'VNIC 管理', children: <VnicList instanceId={instanceId} data={vnicData} loading={loading} onRefresh={handleRefresh} /> },
            { key: 'security', label: '安全规则', children: <SecurityRules tenantId={tenantId!} /> },
            { key: 'vcn', label: 'VCN 管理', children: <VcnPanel tenantId={tenantId!} compartmentId={selectedInstance.compartmentId} instanceId={instanceId} instanceDbId={selectedInstance.id} currentPublicIp={selectedInstance.publicIps || ''} /> },
            { key: 'network', label: '网络配置', children: <NetworkConfig tenantId={tenantId!} compartmentId={selectedInstance.compartmentId} vcnId={vcnId} instanceId={instanceId} /> },
          ]}
        />
      )}
    </div>
  )
}
