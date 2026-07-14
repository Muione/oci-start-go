import { useEffect, useState, useMemo } from 'react'
import { Select, Spin, message } from 'antd'
import { instanceShapes } from '@/api/instance'
import type { ShapeInfo } from '@/types/api'

interface ShapeSelectProps {
  tenantId?: number
  architecture?: string
  value?: string
  onChange?: (value: string, shape?: ShapeInfo) => void
  disabled?: boolean
  placeholder?: string
}

export default function ShapeSelect({
  tenantId,
  architecture,
  value,
  onChange,
  disabled,
  placeholder = '选择 Shape',
}: ShapeSelectProps) {
  const [shapes, setShapes] = useState<ShapeInfo[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!tenantId) {
      setShapes([])
      return
    }
    setLoading(true)
    instanceShapes({ tenantId, architecture })
      .then((d) => setShapes(d || []))
      .catch((e: unknown) => {
        message.error('加载 Shape 列表失败: ' + (e as Error).message)
        setShapes([])
      })
      .finally(() => setLoading(false))
  }, [tenantId, architecture])

  const grouped = useMemo(() => {
    const arm: ShapeInfo[] = []
    const amd: ShapeInfo[] = []
    for (const s of shapes) {
      if (s.architecture?.toUpperCase() === 'ARM') arm.push(s)
      else amd.push(s)
    }
    return { arm, amd }
  }, [shapes])

  const options = useMemo(() => {
    const items: { label: React.ReactNode; value: string; key: string }[] = []
    function addGroup(_label: string, list: ShapeInfo[]) {
      for (const s of list) {
        items.push({
          key: s.shape,
          value: s.shape,
          label: (
            <div className="flex items-center justify-between">
              <span className="font-medium text-sm">{s.shape}</span>
              <span className="text-gray-400 text-xs ml-2">
                {s.ocpus}C / {s.memoryInGBs}G
                {s.isFlexible ? ' (Flex)' : ''}
              </span>
            </div>
          ),
        })
      }
    }
    if (grouped.arm.length > 0) addGroup('ARM', grouped.arm)
    if (grouped.amd.length > 0) addGroup('AMD', grouped.amd)
    return items
  }, [grouped])

  function handleChange(val: string) {
    const found = shapes.find((s) => s.shape === val)
    onChange?.(val, found)
  }

  return (
    <Select
      value={value}
      onChange={handleChange}
      options={options}
      placeholder={placeholder}
      disabled={disabled || !tenantId}
      loading={loading}
      showSearch
      allowClear
      filterOption={(input, option) =>
        (option?.value as string)?.toLowerCase().includes(input.toLowerCase()) ?? false
      }
      notFoundContent={loading ? <Spin size="small" /> : '暂无可用 Shape'}
      style={{ width: '100%' }}
      optionLabelProp="value"
    />
  )
}
