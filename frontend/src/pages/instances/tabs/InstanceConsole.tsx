import { useEffect, useState } from 'react'
import { Card, Table, Button, Tag, Space, Empty, message, type TableColumnsType } from 'antd'
import { instanceConsoleConnections, instanceDeleteConsoleConnection, type ConsoleConnection } from '@/api/instance'
import type { Instance } from '@/types/api'

interface Props {
  instance: Instance
}

export default function InstanceConsole({ instance }: Props) {
  const [connections, setConnections] = useState<ConsoleConnection[]>([])
  const [loading, setLoading] = useState(false)

  const loadConnections = () => {
    if (!instance.id) return
    setLoading(true)
    instanceConsoleConnections(instance.id)
      .then((d) => setConnections(d || []))
      .catch((e: unknown) => message.error((e as Error).message || '加载控制台连接失败'))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    loadConnections()
  }, [instance.id]) // eslint-disable-line react-hooks/exhaustive-deps

  function handleDelete(conn: ConsoleConnection) {
    instanceDeleteConsoleConnection(instance.id, conn.id)
      .then(() => {
        message.success('已删除')
        loadConnections()
      })
      .catch((e: unknown) => message.error((e as Error).message || '删除失败'))
  }

  const columns: TableColumnsType<ConsoleConnection> = [
    {
      title: '类型',
      dataIndex: 'connectionType',
      width: 120,
      render: (v: string) => <Tag color="blue">{v || '—'}</Tag>,
    },
    {
      title: '状态',
      dataIndex: 'state',
      width: 100,
      render: (v: string) => (
        <Tag color={v?.toLowerCase() === 'active' ? 'success' : 'default'}>{v || '—'}</Tag>
      ),
    },
    {
      title: '控制台 URL',
      dataIndex: 'consoleUrl',
      minWidth: 200,
      ellipsis: true,
      render: (v: string) =>
        v ? (
          <a href={v} target="_blank" rel="noopener noreferrer" className="font-mono text-xs text-blue-500">
            {v}
          </a>
        ) : (
          '—'
        ),
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 180,
    },
    {
      title: '操作',
      width: 120,
      fixed: 'right',
      render: (_: unknown, row: ConsoleConnection) => (
        <Space size="small">
          {row.consoleUrl && (
            <Button size="small" type="link" href={row.consoleUrl} target="_blank">
              打开
            </Button>
          )}
          <Button size="small" danger onClick={() => handleDelete(row)}>
            删除
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <Card
        title="控制台连接"
        size="small"
        extra={<Button size="small" onClick={loadConnections}>刷新</Button>}
      >
        {connections.length === 0 && !loading ? (
          <Empty description="暂无控制台连接" />
        ) : (
          <Table
            dataSource={connections}
            columns={columns}
            rowKey="id"
            loading={loading}
            size="small"
            bordered
            pagination={false}
            scroll={{ x: 700 }}
          />
        )}
      </Card>
    </div>
  )
}
