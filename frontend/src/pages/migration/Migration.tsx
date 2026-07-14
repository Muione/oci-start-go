import { useState, useMemo } from 'react'
import {
  Card, Row, Col, Statistic, Tabs, Upload, Button, Input, Alert, Steps, Divider, Table, message,
} from 'antd'
import {
  UploadOutlined, CheckCircleOutlined, WarningOutlined, CloseCircleOutlined,
  DatabaseOutlined, CodeOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import PageHeader from '@/components/common/PageHeader'
import { migrationImportPlain, migrationImportEncrypted, type MigrationResult } from '@/api/system'

export default function Migration() {
  const { t } = useTranslation()
  const [importing, setImporting] = useState(false)
  const [stats, setStats] = useState<MigrationResult | null>(null)
  const [plainFile, setPlainFile] = useState<File | null>(null)
  const [encFile, setEncFile] = useState<File | null>(null)
  const [masterKey, setMasterKey] = useState('')

  const tableStats = useMemo(() => {
    if (!stats?.tablesFound) return []
    return Object.entries(stats.tablesFound).map(([table, count]) => ({ table, count }))
  }, [stats])

  async function handleImportPlain() {
    if (!plainFile) return
    setImporting(true)
    try {
      const result = await migrationImportPlain(plainFile)
      setStats(result)
      message.success(result?.message || t('common.success'))
    } catch (err: any) {
      message.error(err?.message || t('common.error'))
    } finally {
      setImporting(false)
    }
  }

  async function handleImportEncrypted() {
    if (!encFile || !masterKey) return
    setImporting(true)
    try {
      const result = await migrationImportEncrypted(encFile, masterKey)
      setStats(result)
      message.success(result?.message || t('common.success'))
    } catch (err: any) {
      message.error(err?.message || t('common.error'))
    } finally {
      setImporting(false)
    }
  }

  const tabItems = [
    {
      key: 'plain',
      label: t('migration.plainSql'),
      children: (
        <div>
          <p className="text-gray-500 mb-3">
            {t('migration.plainHint')}
          </p>
          <Alert
            message={t('migration.importWarning')}
            type="warning"
            showIcon
            className="mb-4"
          />
          <Upload.Dragger
            accept=".sql"
            maxCount={1}
            beforeUpload={(file) => { setPlainFile(file); return false }}
            onRemove={() => setPlainFile(null)}
            fileList={plainFile ? [plainFile as any] : []}
          >
            <p className="ant-upload-drag-icon">
              <UploadOutlined />
            </p>
            <p className="ant-upload-text">{t('migration.dragDrop')}</p>
            <p className="ant-upload-hint">.sql</p>
          </Upload.Dragger>
          <Button
            type="primary"
            icon={<UploadOutlined />}
            loading={importing}
            disabled={!plainFile}
            onClick={handleImportPlain}
            className="mt-4"
          >
            {t('migration.startImport')}
          </Button>
        </div>
      ),
    },
    {
      key: 'encrypted',
      label: t('migration.encrypted'),
      children: (
        <div>
          <p className="text-gray-500 mb-3">
            {t('migration.encryptedHint')}
          </p>
          <div className="mb-3">
            <label className="block mb-1 font-medium">Master Key</label>
            <Input.TextArea
              rows={2}
              value={masterKey}
              onChange={(e) => setMasterKey(e.target.value)}
              placeholder={t('migration.masterKeyPlaceholder')}
            />
          </div>
          <Upload.Dragger
            accept=".enc"
            maxCount={1}
            beforeUpload={(file) => { setEncFile(file); return false }}
            onRemove={() => setEncFile(null)}
            fileList={encFile ? [encFile as any] : []}
          >
            <p className="ant-upload-drag-icon">
              <UploadOutlined />
            </p>
            <p className="ant-upload-text">{t('migration.dragDropEnc')}</p>
            <p className="ant-upload-hint">.enc</p>
          </Upload.Dragger>
          <Button
            type="primary"
            icon={<UploadOutlined />}
            loading={importing}
            disabled={!encFile || !masterKey}
            onClick={handleImportEncrypted}
            className="mt-4"
          >
            {t('migration.decryptImport')}
          </Button>
        </div>
      ),
    },
  ]

  const stepItems = [
    { title: t('migration.stepExport'), description: t('migration.stepExportDesc') },
    { title: t('migration.stepBackup'), description: t('migration.stepBackupDesc') },
    { title: t('migration.stepImport'), description: t('migration.stepImportDesc') },
    { title: t('migration.stepVerify'), description: t('migration.stepVerifyDesc') },
    { title: t('migration.stepSwitch'), description: t('migration.stepSwitchDesc') },
  ]

  return (
    <div>
      <PageHeader title={t('migration.title')} />

      {/* Stats */}
      {stats && (
        <Row gutter={16} className="mb-4">
          <Col xs={12} sm={6}>
            <Card size="small">
              <Statistic title={t('migration.totalLines')} value={stats.totalLines ?? 0} />
            </Card>
          </Col>
          <Col xs={12} sm={6}>
            <Card size="small">
              <Statistic
                title={t('migration.inserted')}
                value={stats.inserted ?? 0}
                valueStyle={{ color: '#52c41a' }}
                prefix={<CheckCircleOutlined />}
              />
            </Card>
          </Col>
          <Col xs={12} sm={6}>
            <Card size="small">
              <Statistic
                title={t('migration.skipped')}
                value={(stats.skipped ?? 0) + (stats.skippedDups ?? 0) + (stats.skippedUser ?? 0)}
                valueStyle={{ color: '#faad14' }}
                prefix={<WarningOutlined />}
              />
            </Card>
          </Col>
          <Col xs={12} sm={6}>
            <Card size="small">
              <Statistic
                title={t('migration.errors')}
                value={stats.errors ?? 0}
                valueStyle={{ color: (stats.errors ?? 0) > 0 ? '#ff4d4f' : '#52c41a' }}
                prefix={(stats.errors ?? 0) > 0 ? <CloseCircleOutlined /> : <CheckCircleOutlined />}
              />
            </Card>
          </Col>
        </Row>
      )}

      {/* Import */}
      <Card size="small" title={<><DatabaseOutlined className="mr-1" />{t('migration.importDb')}</>} className="mb-4">
        <Tabs items={tabItems} />
      </Card>

      {/* CLI guide */}
      <Card size="small" title={<><CodeOutlined className="mr-1" />{t('migration.cliTool')}</>} className="mb-4">
        <p className="text-gray-500 mb-3">{t('migration.cliHint')}</p>

        <Divider>{t('migration.plainSql')}</Divider>
        <pre className="bg-gray-900 text-green-400 p-4 rounded text-xs overflow-x-auto">
{`# Export from Java version
curl -o backup.sql http://old-server:9856/migration/export

# Import with CLI
./migrate -db /path/to/oci-start.db -file backup.sql`}
        </pre>

        <Divider>{t('migration.encrypted')}</Divider>
        <pre className="bg-gray-900 text-green-400 p-4 rounded text-xs overflow-x-auto">
{`# Export encrypted backup
curl -o backup.enc http://old-server:9856/migration/exportEncrypted
# Response header X-MASTER-KEY contains the key (shown once!)

# Import with CLI
./migrate -db /path/to/oci-start.db -file backup.enc -key <master-key>`}
        </pre>

        <Divider>{t('migration.steps')}</Divider>
        <Steps direction="vertical" size="small" items={stepItems} />
      </Card>

      {/* Table stats */}
      {tableStats.length > 0 && (
        <Card size="small" title={t('migration.tableStats')}>
          <Table
            dataSource={tableStats}
            columns={[
              { title: t('migration.tableName'), dataIndex: 'table', key: 'table' },
              { title: t('migration.importRows'), dataIndex: 'count', key: 'count', align: 'right' },
            ]}
            rowKey="table"
            size="small"
            pagination={false}
          />
        </Card>
      )}
    </div>
  )
}
