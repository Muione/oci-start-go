import { useCallback, useEffect, useRef, useState } from 'react'
import {
  Table, Button, Tag, Space, Input, Modal, Form, InputNumber, Progress, message,
} from 'antd'
import {
  ArrowLeftOutlined, UploadOutlined, ReloadOutlined, DeleteOutlined,
  DownloadOutlined, LinkOutlined, EyeOutlined, SearchOutlined, UnorderedListOutlined,
} from '@ant-design/icons'
import {
  listObjects, deleteObject, generatePresignedUrl, downloadObjectUrl, previewObjectUrl,
  uploadObjectUrl, multipartInitiate, multipartUploadPartUrl, multipartCommit, multipartAbort,
  listResumableUploads,
} from '@/api/storage'
import type { Bucket, StorageObject, MultipartUpload } from '@/types/api'

const MULTIPART_THRESHOLD = 100 * 1024 * 1024 // 100MB
const DEFAULT_CHUNK_SIZE = 10 * 1024 * 1024 // 10MB

function formatBytes(bytes?: number) {
  if (!bytes || isNaN(bytes)) return '0 B'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1073741824) return (bytes / 1048576).toFixed(1) + ' MB'
  return (bytes / 1073741824).toFixed(2) + ' GB'
}

function inferContentType(filename: string): string {
  const ext = filename.split('.').pop()?.toLowerCase() || ''
  const map: Record<string, string> = {
    png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', gif: 'image/gif',
    webp: 'image/webp', svg: 'image/svg+xml', pdf: 'application/pdf',
    json: 'application/json', xml: 'application/xml', txt: 'text/plain',
    html: 'text/html', css: 'text/css', js: 'text/javascript',
    zip: 'application/zip', gz: 'application/gzip',
  }
  return map[ext] || 'application/octet-stream'
}

interface Props {
  bucket: Bucket
  tenantId: number
  onBack: () => void
}

export default function ObjectList({ bucket, tenantId, onBack }: Props) {
  const [objects, setObjects] = useState<StorageObject[]>([])
  const [loading, setLoading] = useState(false)
  const [nextToken, setNextToken] = useState('')
  const [prefix, setPrefix] = useState('')
  const [uploading, setUploading] = useState(false)
  const [uploadProgress, setUploadProgress] = useState(0)
  const [uploadFileName, setUploadFileName] = useState('')
  const [uploadMultipart, setUploadMultipart] = useState(false)
  const [uploadPartNum, setUploadPartNum] = useState(0)
  const [uploadTotalParts, setUploadTotalParts] = useState(0)
  const [uploadBytesSent, setUploadBytesSent] = useState(0)
  const [uploadTotalSize, setUploadTotalSize] = useState(0)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const uploadAbortRef = useRef<AbortController | null>(null)

  // Presigned URL dialog
  const [presignedOpen, setPresignedOpen] = useState(false)
  const [presignedLoading, setPresignedLoading] = useState(false)
  const [presignedUrl, setPresignedUrl] = useState('')
  const [presignedForm] = Form.useForm()

  // Resumable uploads dialog
  const [resumableOpen, setResumableOpen] = useState(false)
  const [resumableList, setResumableList] = useState<MultipartUpload[]>([])
  const [resumableLoading, setResumableLoading] = useState(false)

  // Preview dialog
  const [previewOpen, setPreviewOpen] = useState(false)
  const [previewSrc, setPreviewSrc] = useState('')
  const [previewType, setPreviewType] = useState<'image' | 'text' | 'unknown'>('unknown')
  const [previewContent, setPreviewContent] = useState('')

  const loadObjects = useCallback(async () => {
    setLoading(true)
    try {
      const res = await listObjects(tenantId, bucket.namespace, bucket.name, 100, prefix || undefined)
      setObjects(res.items || [])
      setNextToken(res.nextStartWith || '')
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setLoading(false)
    }
  }, [tenantId, bucket, prefix])

  useEffect(() => { loadObjects() }, [prefix]) // eslint-disable-line react-hooks/exhaustive-deps

  // Debounced prefix
  const prefixTimer = useRef<ReturnType<typeof setTimeout>>(undefined)
  function onPrefixChange(v: string) {
    if (prefixTimer.current) clearTimeout(prefixTimer.current)
    prefixTimer.current = setTimeout(() => setPrefix(v), 400)
  }

  function handleDownload(obj: StorageObject) {
    const url = downloadObjectUrl(tenantId, bucket.namespace, bucket.name, obj.name)
    const a = document.createElement('a')
    a.href = url
    a.download = obj.name.split('/').pop() || obj.name
    document.body.appendChild(a)
    a.click()
    a.remove()
  }

  async function handleDelete(obj: StorageObject) {
    Modal.confirm({
      title: '确认删除',
      content: `确定删除文件「${obj.name}」？`,
      okType: 'danger',
      onOk: async () => {
        try {
          await deleteObject(tenantId, bucket.namespace, bucket.name, obj.name)
          message.success('文件已删除')
          loadObjects()
        } catch (e: unknown) {
          message.error((e as Error).message)
        }
      },
    })
  }

  async function handlePresigned(values: { validitySeconds: number }) {
    const objName = presignedForm.getFieldValue('objectName')
    setPresignedLoading(true)
    try {
      const res = await generatePresignedUrl(tenantId, bucket.namespace, bucket.name, objName, values.validitySeconds)
      setPresignedUrl(res.url || '')
    } catch (e: unknown) {
      message.error((e as Error).message)
    } finally {
      setPresignedLoading(false)
    }
  }

  function openPresigned(obj: StorageObject) {
    presignedForm.setFieldsValue({ objectName: obj.name, validitySeconds: 3600 })
    setPresignedUrl('')
    setPresignedOpen(true)
  }

  async function handlePreview(obj: StorageObject) {
    setPreviewOpen(true)
    setPreviewType('unknown')
    setPreviewSrc('')
    setPreviewContent('')
    const url = previewObjectUrl(tenantId, bucket.namespace, bucket.name, obj.name)
    const ct = (obj.contentType || '').toLowerCase()
    if (ct.startsWith('image/')) {
      setPreviewType('image')
      setPreviewSrc(url)
    } else if (ct.startsWith('text/') || ct.includes('json') || ct.includes('xml')) {
      setPreviewType('text')
      try {
        const resp = await fetch(url, { credentials: 'include' })
        setPreviewContent(resp.ok ? await resp.text() : '加载失败')
      } catch { setPreviewContent('加载失败') }
    } else {
      try {
        const resp = await fetch(url, { credentials: 'include' })
        const rct = resp.headers.get('content-type') || ''
        if (rct.startsWith('image/')) {
          setPreviewType('image')
          setPreviewSrc(URL.createObjectURL(await resp.blob()))
        } else if (rct.startsWith('text/') || rct.includes('json')) {
          setPreviewType('text')
          setPreviewContent(await resp.text())
        }
      } catch { /* ignore */ }
    }
  }

  // ── Upload ────────────────────────────────────────────

  function triggerUpload() { fileInputRef.current?.click() }

  function onFileSelected(e: React.ChangeEvent<HTMLInputElement>) {
    const files = e.target.files
    if (!files || files.length === 0) return
    uploadFiles(Array.from(files))
    e.target.value = ''
  }

  function onDrop(e: React.DragEvent<HTMLElement>) {
    e.preventDefault()
    const files = e.dataTransfer?.files
    if (!files || files.length === 0) return
    uploadFiles(Array.from(files))
  }

  async function uploadFiles(files: File[]) {
    for (const file of files) {
      if (file.size > MULTIPART_THRESHOLD) {
        await multipartUpload(file)
      } else {
        await singleUpload(file)
      }
    }
  }

  async function singleUpload(file: File) {
    setUploading(true)
    setUploadFileName(file.name)
    setUploadMultipart(false)
    setUploadProgress(0)
    setUploadBytesSent(0)
    setUploadTotalSize(file.size)

    const fd = new FormData()
    fd.append('tenantId', String(tenantId))
    fd.append('namespace', bucket.namespace)
    fd.append('bucketName', bucket.name)
    fd.append('objectName', file.name)
    fd.append('file', file)

    try {
      await new Promise<void>((resolve, reject) => {
        const xhr = new XMLHttpRequest()
        xhr.open('POST', uploadObjectUrl())
        xhr.withCredentials = true
        xhr.upload.onprogress = (ev) => {
          if (ev.lengthComputable) {
            setUploadBytesSent(ev.loaded)
            setUploadProgress(Math.round((ev.loaded / ev.total) * 100))
          }
        }
        xhr.onload = () => {
          if (xhr.status >= 200 && xhr.status < 300) {
            setUploadProgress(100)
            setUploadBytesSent(file.size)
            resolve()
          } else {
            let msg = '上传失败'
            try { msg = JSON.parse(xhr.responseText).message || msg } catch { /* */ }
            reject(new Error(msg))
          }
        }
        xhr.onerror = () => reject(new Error('网络错误'))
        xhr.send(fd)
      })
      message.success(`${file.name} 上传成功`)
      loadObjects()
    } catch (e: unknown) {
      message.error('上传失败: ' + (e as Error).message)
    } finally {
      setUploading(false)
    }
  }

  async function multipartUpload(file: File) {
    setUploading(true)
    setUploadFileName(file.name)
    setUploadMultipart(true)
    setUploadProgress(0)
    setUploadBytesSent(0)
    setUploadTotalSize(file.size)
    uploadAbortRef.current = new AbortController()

    const chunkSize = DEFAULT_CHUNK_SIZE
    const totalParts = Math.ceil(file.size / chunkSize)
    setUploadTotalParts(totalParts)
    setUploadPartNum(0)

    const completedParts: Array<{ partNum: number; etag: string }> = []
    let uploadId = ''

    try {
      const initResp = await multipartInitiate(tenantId, bucket.namespace, bucket.name, file.name, inferContentType(file.name), file.size, chunkSize)
      uploadId = initResp.uploadId

      for (let i = 0; i < totalParts; i++) {
        if (uploadAbortRef.current?.signal.aborted) throw new Error('上传已取消')
        const start = i * chunkSize
        const end = Math.min(start + chunkSize, file.size)
        const chunk = file.slice(start, end)
        setUploadPartNum(i + 1)

        const fd = new FormData()
        fd.append('tenantId', String(tenantId))
        fd.append('namespace', bucket.namespace)
        fd.append('bucketName', bucket.name)
        fd.append('objectName', file.name)
        fd.append('uploadId', uploadId)
        fd.append('partNumber', String(i + 1))
        fd.append('chunk', chunk)

        const partResp: { etag?: string; data?: { etag?: string } } = await new Promise((resolve, reject) => {
          const xhr = new XMLHttpRequest()
          xhr.open('POST', multipartUploadPartUrl())
          xhr.withCredentials = true
          xhr.upload.onprogress = (ev) => {
            if (ev.lengthComputable) {
              setUploadBytesSent(start + ev.loaded)
              setUploadProgress(Math.round(((start + ev.loaded) / file.size) * 100))
            }
          }
          xhr.onload = () => {
            if (xhr.status >= 200 && xhr.status < 300) {
              try { resolve(JSON.parse(xhr.responseText)) } catch { resolve({}) }
            } else {
              let msg = '分片上传失败'
              try { msg = JSON.parse(xhr.responseText).message || msg } catch { /* */ }
              reject(new Error(msg))
            }
          }
          xhr.onerror = () => reject(new Error('网络错误'))
          if (uploadAbortRef.current?.signal.aborted) { reject(new Error('上传已取消')); return }
          xhr.send(fd)
        })

        const etag = partResp?.data?.etag || partResp?.etag || ''
        completedParts.push({ partNum: i + 1, etag })
      }

      await multipartCommit(tenantId, bucket.namespace, bucket.name, file.name, uploadId, completedParts)
      setUploadProgress(100)
      setUploadBytesSent(file.size)
      message.success(`${file.name} 上传完成`)
      loadObjects()
    } catch (e: unknown) {
      if (uploadId) {
        try { await multipartAbort(tenantId, bucket.namespace, bucket.name, file.name, uploadId) } catch { /* */ }
      }
      if ((e as Error).message !== '上传已取消') {
        message.error('上传失败: ' + (e as Error).message)
      }
    } finally {
      setUploading(false)
      uploadAbortRef.current = null
    }
  }

  function cancelUpload() { uploadAbortRef.current?.abort() }

  async function showResumable() {
    setResumableOpen(true)
    setResumableLoading(true)
    try {
      const data = await listResumableUploads(tenantId, bucket.name)
      setResumableList(data || [])
    } catch (e: unknown) {
      message.error((e as Error).message)
      setResumableList([])
    } finally {
      setResumableLoading(false)
    }
  }

  async function abortResumable(record: MultipartUpload) {
    Modal.confirm({
      title: '确认放弃',
      content: `确定放弃上传「${record.objectName}」？已上传的 ${record.completedPartCount} 个分片将被丢弃。`,
      okType: 'danger',
      onOk: async () => {
        try {
          await multipartAbort(tenantId, record.namespace, record.bucketName, record.objectName, record.uploadId)
          message.success('已放弃')
          setResumableList((prev) => prev.filter((r) => r.uploadId !== record.uploadId))
        } catch (e: unknown) {
          message.error((e as Error).message)
        }
      },
    })
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4 flex-wrap gap-2">
        <Space>
          <Button icon={<ArrowLeftOutlined />} onClick={onBack}>返回</Button>
          <span className="text-lg font-semibold">{bucket.name}</span>
          <Tag>namespace: {bucket.namespace}</Tag>
        </Space>
        <Space>
          <Button type="primary" icon={<UploadOutlined />} onClick={triggerUpload}>上传文件</Button>
          <Button icon={<UnorderedListOutlined />} onClick={showResumable}>未完成上传</Button>
          <Button icon={<ReloadOutlined />} onClick={loadObjects} loading={loading}>刷新</Button>
        </Space>
      </div>

      <div className="mb-3">
        <Input
          placeholder="按前缀过滤..."
          prefix={<SearchOutlined />}
          allowClear
          style={{ width: 260 }}
          onChange={(e) => onPrefixChange(e.target.value)}
        />
      </div>

      {/* Drop zone */}
      {!uploading && (
        <div
          className="border-2 border-dashed rounded-lg p-8 text-center text-gray-500 mb-4 cursor-pointer hover:border-blue-400 hover:bg-blue-50 transition"
          onDragOver={(e) => e.preventDefault()}
          onDrop={onDrop}
          onClick={triggerUpload}
        >
          <UploadOutlined style={{ fontSize: 32 }} />
          <p className="mt-2">拖拽文件到此处上传，或 <span className="text-blue-500">点击选择</span></p>
        </div>
      )}

      {/* Upload progress */}
      {uploading && (
        <div className="border rounded-lg p-4 mb-4">
          <div className="flex items-center justify-between mb-2">
            <span className="font-medium">{uploadFileName}</span>
            {uploadMultipart && <Button size="small" danger onClick={cancelUpload}>取消上传</Button>}
          </div>
          <Progress percent={uploadProgress} status={uploadProgress >= 100 ? 'success' : 'active'} />
          <p className="text-xs text-gray-500 mt-1">
            {formatBytes(uploadBytesSent)} / {formatBytes(uploadTotalSize)}
            {uploadMultipart && ` · 第 ${uploadPartNum}/${uploadTotalParts} 分片`}
          </p>
        </div>
      )}

      <input ref={fileInputRef} type="file" style={{ display: 'none' }} onChange={onFileSelected} multiple />

      <Table dataSource={objects} loading={loading} rowKey="name" size="small">
        <Table.Column title="文件名" dataIndex="name" ellipsis render={(v: string) => <span className="font-mono text-xs">{v}</span>} />
        <Table.Column title="大小" dataIndex="size" width={100} align="right" render={(v: number) => formatBytes(v)} />
        <Table.Column title="类型" dataIndex="contentType" width={150} render={(v: string) => <Tag>{v || '-'}</Tag>} />
        <Table.Column title="最后修改" dataIndex="timeModified" width={170} render={(v: string) => v ? new Date(v).toLocaleString('zh-CN') : '-'} />
        <Table.Column
          title="操作"
          width={260}
          render={(_: unknown, row: StorageObject) => (
            <Space>
              <Button size="small" icon={<EyeOutlined />} onClick={() => handlePreview(row)}>预览</Button>
              <Button size="small" icon={<DownloadOutlined />} onClick={() => handleDownload(row)}>下载</Button>
              <Button size="small" icon={<LinkOutlined />} onClick={() => openPresigned(row)}>链接</Button>
              <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(row)} />
            </Space>
          )}
        />
      </Table>

      {nextToken && (
        <div className="text-center mt-3">
          <Button size="small" onClick={async () => {
            setLoading(true)
            try {
              const res = await listObjects(tenantId, bucket.namespace, bucket.name, 100, prefix || undefined, nextToken)
              setObjects((prev) => [...prev, ...(res.items || [])])
              setNextToken(res.nextStartWith || '')
            } catch (e: unknown) { message.error((e as Error).message) }
            finally { setLoading(false) }
          }}>加载更多</Button>
        </div>
      )}

      {/* Presigned URL Dialog */}
      <Modal open={presignedOpen} title="预签名链接" onCancel={() => setPresignedOpen(false)} onOk={() => presignedForm.submit()} confirmLoading={presignedLoading} destroyOnClose>
        <Form form={presignedForm} layout="vertical" onFinish={handlePresigned} initialValues={{ validitySeconds: 3600 }}>
          <Form.Item name="objectName" label="文件"><Input disabled /></Form.Item>
          <Form.Item name="validitySeconds" label="有效期(秒)"><InputNumber min={60} max={604800} style={{ width: '100%' }} /></Form.Item>
        </Form>
        {presignedUrl && (
          <div className="mt-2">
            <Input
              value={presignedUrl}
              readOnly
              addonAfter={<Button size="small" type="link" onClick={() => { navigator.clipboard?.writeText(presignedUrl); message.success('已复制') }}>复制</Button>}
            />
          </div>
        )}
      </Modal>

      {/* Preview Dialog */}
      <Modal open={previewOpen} title="文件预览" onCancel={() => setPreviewOpen(false)} footer={null} width="80%">
        {previewType === 'image' && <div className="text-center"><img src={previewSrc} alt="preview" style={{ maxWidth: '100%', maxHeight: '70vh' }} /></div>}
        {previewType === 'text' && <pre className="bg-gray-50 p-4 rounded overflow-auto max-h-[70vh] text-sm font-mono whitespace-pre-wrap">{previewContent}</pre>}
        {previewType === 'unknown' && <p className="text-center text-gray-500">无法预览此文件类型</p>}
      </Modal>

      {/* Resumable Uploads Dialog */}
      <Modal open={resumableOpen} title="未完成的上传" onCancel={() => setResumableOpen(false)} footer={null} width={700}>
        <Table dataSource={resumableList} loading={resumableLoading} rowKey="uploadId" size="small" locale={{ emptyText: '没有未完成的上传' }}>
          <Table.Column title="文件名" dataIndex="objectName" ellipsis />
          <Table.Column title="进度" width={120} render={(_: unknown, r: MultipartUpload) => (
            <Progress percent={r.totalParts > 0 ? Math.round(r.completedPartCount / r.totalParts * 100) : 0} size="small" />
          )} />
          <Table.Column title="大小" width={100} render={(_: unknown, r: MultipartUpload) => formatBytes(r.totalSize)} />
          <Table.Column title="分片" width={80} align="center" render={(_: unknown, r: MultipartUpload) => `${r.completedPartCount}/${r.totalParts}`} />
          <Table.Column title="操作" width={80} render={(_: unknown, r: MultipartUpload) => (
            <Button size="small" danger onClick={() => abortResumable(r)}>放弃</Button>
          )} />
        </Table>
      </Modal>
    </div>
  )
}
