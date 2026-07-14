import { useEffect, useState } from 'react'
import { Table, Button, DatePicker, Input, Tag, message, type TableColumnsType } from 'antd'
import { SearchOutlined } from '@ant-design/icons'
import {
  tenantAuditLog, tenantNotifRecipients, tenantNotifRecipientsUpdate,
  type AuditEvent, type NotifRecipient,
} from '@/api/tenant'

interface Props {
  tenantId: number
}

export default function TenantAudit({ tenantId }: Props) {
  // Audit
  const [auditLogs, setAuditLogs] = useState<AuditEvent[]>([])
  const [auditLoading, setAuditLoading] = useState(false)
  const [auditDays, setAuditDays] = useState(1)
  const [dateRange, setDateRange] = useState<[string, string] | null>(null)

  // Notification recipients
  const [recipients, setRecipients] = useState<NotifRecipient[]>([])
  const [recipientsLoading, setRecipientsLoading] = useState(false)
  const [emailInput, setEmailInput] = useState('')
  const [notifSaving, setNotifSaving] = useState(false)

  async function loadAudit(days: number) {
    setAuditDays(days)
    setDateRange(null)
    setAuditLoading(true)
    try {
      const r = await tenantAuditLog(tenantId, { days })
      setAuditLogs(r?.data || [])
    } catch (e: unknown) {
      message.error((e as Error).message)
      setAuditLogs([])
    } finally {
      setAuditLoading(false)
    }
  }

  async function loadAuditCustom() {
    if (!dateRange || dateRange.length !== 2) {
      message.warning('请选择日期范围')
      return
    }
    setAuditDays(0)
    setAuditLoading(true)
    try {
      const r = await tenantAuditLog(tenantId, { startDate: dateRange[0], endDate: dateRange[1] })
      setAuditLogs(r?.data || [])
    } catch (e: unknown) {
      message.error((e as Error).message)
      setAuditLogs([])
    } finally {
      setAuditLoading(false)
    }
  }

  function loadRecipients() {
    setRecipientsLoading(true)
    tenantNotifRecipients(tenantId)
      .then((d) => setRecipients(d || []))
      .catch((e: unknown) => {
        message.error((e as Error).message)
        setRecipients([])
      })
      .finally(() => setRecipientsLoading(false))
  }

  async function handleUpdateRecipients() {
    const emails = emailInput.split(',').map((e) => e.trim()).filter(Boolean)
    if (!emails.length) {
      message.warning('请输入至少一个邮箱')
      return
    }
    setNotifSaving(true)
    try {
      await tenantNotifRecipientsUpdate(tenantId, emails)
      message.success('已更新')
      loadRecipients()
      setEmailInput('')
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setNotifSaving(false)
    }
  }

  async function handleDeleteRecipient(email: string) {
    const remaining = recipients.map((r) => r.email).filter((e) => e !== email)
    setNotifSaving(true)
    try {
      await tenantNotifRecipientsUpdate(tenantId, remaining)
      message.success('已删除')
      loadRecipients()
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setNotifSaving(false)
    }
  }

  useEffect(() => {
    loadAudit(1)
    loadRecipients()
  }, [tenantId])

  const auditColumns: TableColumnsType<AuditEvent> = [
    { title: '时间', dataIndex: 'eventTime', width: 170 },
    { title: '事件', dataIndex: 'eventType', minWidth: 200, ellipsis: true },
    { title: '用户', dataIndex: 'userName', width: 140 },
    { title: '类型', dataIndex: 'userType', width: 80 },
    { title: 'IP', dataIndex: 'ipAddress', width: 130 },
    { title: '状态', dataIndex: 'responseStatus', width: 80 },
  ]

  const recipientColumns: TableColumnsType<NotifRecipient> = [
    { title: '邮箱', dataIndex: 'email', minWidth: 200 },
    {
      title: '状态',
      dataIndex: 'state',
      width: 100,
      render: (v: string) => <Tag color={v === 'VERIFIED' ? 'success' : 'warning'}>{v || '—'}</Tag>,
    },
    {
      title: '操作',
      width: 80,
      align: 'center',
      render: (_: unknown, row: NotifRecipient) => (
        <Button size="small" danger onClick={() => handleDeleteRecipient(row.email)}>删除</Button>
      ),
    },
  ]

  return (
    <div>
      {/* Audit log */}
      <h4 className="font-semibold mb-3">审计日志</h4>
      <div className="flex items-center gap-2 mb-3 flex-wrap">
        <Button size="small" type={auditDays === 1 ? 'primary' : 'default'} onClick={() => loadAudit(1)}>今日</Button>
        <Button size="small" type={auditDays === 7 ? 'primary' : 'default'} onClick={() => loadAudit(7)}>7 天</Button>
        <Button size="small" type={auditDays === 30 ? 'primary' : 'default'} onClick={() => loadAudit(30)}>30 天</Button>
        <DatePicker.RangePicker
          size="small"
          onChange={(dates, dateStrings) => {
            if (dates) setDateRange(dateStrings as [string, string])
            else setDateRange(null)
          }}
        />
        <Button size="small" type="primary" icon={<SearchOutlined />} onClick={loadAuditCustom} loading={auditLoading}>
          查询
        </Button>
      </div>
      <Table
        dataSource={auditLogs}
        columns={auditColumns}
        rowKey={(_r, i) => String(i)}
        loading={auditLoading}
        bordered
        size="small"
        scroll={{ x: 800 }}
        pagination={{ pageSize: 20 }}
      />

      {/* Notification recipients */}
      <h4 className="font-semibold mt-8 mb-3">通知接收人</h4>
      <Table
        dataSource={recipients}
        columns={recipientColumns}
        rowKey="email"
        loading={recipientsLoading}
        bordered
        size="small"
        pagination={false}
      />
      <div className="flex items-center gap-2 mt-3">
        <Input
          value={emailInput}
          onChange={(e) => setEmailInput(e.target.value)}
          placeholder="输入邮箱地址，多个用逗号分隔"
          style={{ maxWidth: 400 }}
        />
        <Button type="primary" size="small" onClick={handleUpdateRecipients} loading={notifSaving}>
          更新接收人
        </Button>
      </div>
    </div>
  )
}
