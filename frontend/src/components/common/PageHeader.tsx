import type { ReactNode } from 'react'
import { Button, Space, Typography } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'

const { Title } = Typography

interface PageHeaderProps {
  title: string
  extra?: ReactNode
  onRefresh?: () => void
  refreshing?: boolean
  children?: ReactNode
}

export default function PageHeader({ title, extra, onRefresh, refreshing, children }: PageHeaderProps) {
  return (
    <div className="flex items-center justify-between flex-wrap gap-3 mb-4">
      <Title level={4} className="!mb-0">{title}</Title>
      <Space>
        {extra}
        {onRefresh && (
          <Button icon={<ReloadOutlined />} loading={refreshing} onClick={onRefresh}>
            刷新
          </Button>
        )}
        {children}
      </Space>
    </div>
  )
}
