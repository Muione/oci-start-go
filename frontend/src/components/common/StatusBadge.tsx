import { Badge } from 'antd'

interface StatusBadgeProps {
  status: 'up' | 'down' | 'idle' | 'warn'
  pulse?: boolean
  text?: string
}

const STATUS_MAP: Record<string, { color: string; label: string }> = {
  up: { color: '#52c41a', label: '正常' },
  down: { color: '#ff4d4f', label: '停用' },
  idle: { color: '#8c8c8c', label: '空闲' },
  warn: { color: '#faad14', label: '警告' },
}

export default function StatusBadge({ status, text }: StatusBadgeProps) {
  const cfg = STATUS_MAP[status] || STATUS_MAP.idle
  return (
    <span className="inline-flex items-center gap-1.5">
      <Badge color={cfg.color} />
      <span style={{ color: cfg.color, fontSize: 12 }}>{text || cfg.label}</span>
    </span>
  )
}
