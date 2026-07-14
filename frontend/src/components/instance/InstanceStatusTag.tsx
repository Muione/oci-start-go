import { Tag } from 'antd'

interface InstanceStatusTagProps {
  state?: string
}

const STATE_MAP: Record<string, { color: string; label: string }> = {
  running: { color: 'success', label: '运行中' },
  stopped: { color: 'error', label: '已停止' },
  terminated: { color: 'default', label: '已终止' },
  starting: { color: 'processing', label: '启动中' },
  stopping: { color: 'warning', label: '停止中' },
  provisioning: { color: 'processing', label: '创建中' },
}

export default function InstanceStatusTag({ state }: InstanceStatusTagProps) {
  const key = (state || '').toLowerCase()
  const cfg = STATE_MAP[key] || { color: 'default', label: state || '未知' }
  return <Tag color={cfg.color}>{cfg.label}</Tag>
}
