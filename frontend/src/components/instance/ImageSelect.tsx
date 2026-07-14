import { useEffect, useState, useMemo } from 'react'
import { Select, Spin, message } from 'antd'
import { instanceImages } from '@/api/instance'
import type { ImageInfo } from '@/types/api'

interface ImageSelectProps {
  tenantId?: number
  architecture?: string
  shape?: string
  value?: string
  onChange?: (value: string, image?: ImageInfo) => void
  disabled?: boolean
  placeholder?: string
}

export default function ImageSelect({
  tenantId,
  architecture,
  shape,
  value,
  onChange,
  disabled,
  placeholder = '选择镜像',
}: ImageSelectProps) {
  const [images, setImages] = useState<ImageInfo[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!tenantId) {
      setImages([])
      return
    }
    setLoading(true)
    instanceImages({ tenantId, architecture, shape })
      .then((d) => setImages(d || []))
      .catch((e: unknown) => {
        message.error('加载镜像列表失败: ' + (e as Error).message)
        setImages([])
      })
      .finally(() => setLoading(false))
  }, [tenantId, architecture, shape])

  const grouped = useMemo(() => {
    const groups: Record<string, ImageInfo[]> = {}
    for (const img of images) {
      const os = img.operatingSystem || 'Other'
      if (!groups[os]) groups[os] = []
      groups[os].push(img)
    }
    return groups
  }, [images])

  const options = useMemo(() => {
    const items: { label: React.ReactNode; value: string; key: string }[] = []
    for (const [, imgs] of Object.entries(grouped)) {
      for (const img of imgs) {
        items.push({
          key: img.id,
          value: img.id,
          label: (
            <div className="flex items-center justify-between">
              <span className="font-medium text-sm">{img.displayName}</span>
              <span className="text-gray-400 text-xs ml-2">
                {img.operatingSystem} {img.operatingSystemVersion}
                {img.sizeInGBs ? ` (${img.sizeInGBs}GB)` : ''}
              </span>
            </div>
          ),
        })
      }
    }
    return items
  }, [grouped])

  function handleChange(val: string) {
    const found = images.find((img) => img.id === val)
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
      notFoundContent={loading ? <Spin size="small" /> : '暂无可用镜像'}
      style={{ width: '100%' }}
      optionLabelProp="value"
    />
  )
}
