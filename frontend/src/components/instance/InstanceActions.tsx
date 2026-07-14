import { Button, Space, Modal, message } from 'antd'
import {
  PlayCircleOutlined, PauseCircleOutlined, ReloadOutlined, DeleteOutlined,
} from '@ant-design/icons'
import { instanceStart, instanceStop, instanceRestart, instanceTerminate } from '@/api/instance'
import type { Instance } from '@/types/api'

interface InstanceActionsProps {
  instance: Instance
  onRefresh?: () => void
  size?: 'small' | 'middle' | 'large'
}

export default function InstanceActions({ instance, onRefresh, size = 'small' }: InstanceActionsProps) {
  const isRunning = instance.state?.toLowerCase() === 'running'
  const isStopped = instance.state?.toLowerCase() === 'stopped'
  const isTerminated = instance.state?.toLowerCase() === 'terminated'

  function confirmAction(action: string, title: string, content: string, danger: boolean, fn: () => Promise<unknown>) {
    Modal.confirm({
      title,
      content,
      okType: danger ? 'primary' : undefined,
      okButtonProps: danger ? { danger: true } : undefined,
      onOk: async () => {
        try {
          await fn()
          message.success(`${action}请求已发送`)
          onRefresh?.()
        } catch (e: unknown) {
          message.error((e as Error).message || `${action}失败`)
        }
      },
    })
  }

  return (
    <Space size="small">
      <Button
        size={size}
        type="primary"
        icon={<PlayCircleOutlined />}
        disabled={!isStopped}
        onClick={() => confirmAction(
          '启动',
          '确认启动',
          `确定启动实例 ${instance.displayName}？`,
          false,
          () => instanceStart(instance.id),
        )}
      >
        启动
      </Button>
      <Button
        size={size}
        icon={<PauseCircleOutlined />}
        disabled={!isRunning}
        onClick={() => confirmAction(
          '停止',
          '确认停止',
          `确定停止实例 ${instance.displayName}？这将中断所有正在运行的服务。`,
          true,
          () => instanceStop(instance.id),
        )}
      >
        停止
      </Button>
      <Button
        size={size}
        icon={<ReloadOutlined />}
        disabled={!isRunning}
        onClick={() => confirmAction(
          '重启',
          '确认重启',
          `确定重启实例 ${instance.displayName}？实例将先停止再启动。`,
          false,
          () => instanceRestart(instance.id),
        )}
      >
        重启
      </Button>
      <Button
        size={size}
        danger
        icon={<DeleteOutlined />}
        disabled={isTerminated}
        onClick={() => confirmAction(
          '终止',
          '确认终止',
          `确定终止实例 ${instance.displayName}？此操作不可逆！`,
          true,
          () => instanceTerminate(instance.id),
        )}
      >
        终止
      </Button>
    </Space>
  )
}
