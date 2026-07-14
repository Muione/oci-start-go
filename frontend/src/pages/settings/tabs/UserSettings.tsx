import { useEffect, useState } from 'react'
import { Card, Descriptions, Tag, Button, Modal, Form, Input, message, Row, Col } from 'antd'
import { LockOutlined, SettingOutlined, UserOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useUserStore } from '@/store/useUserStore'
import { changePassword, getSystemVersion } from '@/api/system'

export default function UserSettings() {
  const { t } = useTranslation()
  const { username } = useUserStore()
  const [version, setVersion] = useState('')
  const [pwdOpen, setPwdOpen] = useState(false)
  const [pwdLoading, setPwdLoading] = useState(false)
  const [form] = Form.useForm()

  useEffect(() => {
    getSystemVersion()
      .then((d) => setVersion(d?.version || ''))
      .catch(() => {})
  }, [])

  async function handlePasswordChange() {
    try {
      const values = await form.validateFields()
      if (values.newPassword !== values.confirmPassword) {
        message.warning(t('auth.passwordMismatch'))
        return
      }
      setPwdLoading(true)
      await changePassword({
        currentPassword: values.currentPassword,
        newPassword: values.newPassword,
      })
      message.success(t('common.success'))
      setPwdOpen(false)
      form.resetFields()
    } catch (err: any) {
      if (err?.message) message.error(err.message)
    } finally {
      setPwdLoading(false)
    }
  }

  return (
    <div>
      <Row gutter={16}>
        <Col xs={24} sm={12}>
          <Card size="small" title={<><UserOutlined className="mr-1" />{t('settings.userInfo')}</>}>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label={t('auth.username')}>
                <Tag color="blue">{username || '—'}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label={t('settings.userRole')}>
                <Tag color="red">ADMIN</Tag>
              </Descriptions.Item>
              <Descriptions.Item label={t('settings.loginStatus')}>
                <Tag color="success">{t('settings.loggedIn')}</Tag>
              </Descriptions.Item>
            </Descriptions>
          </Card>
        </Col>

        <Col xs={24} sm={12}>
          <Card size="small" title={<><SettingOutlined className="mr-1" />{t('settings.quickActions')}</>}>
            <div className="flex flex-col gap-2">
              <Button
                type="primary"
                icon={<LockOutlined />}
                onClick={() => setPwdOpen(true)}
              >
                {t('settings.changePassword')}
              </Button>
            </div>
          </Card>
        </Col>
      </Row>

      <Card
        size="small"
        title={<><SettingOutlined className="mr-1" />{t('settings.systemInfo')}</>}
        className="mt-4"
      >
        <Descriptions column={2} size="small" bordered>
          <Descriptions.Item label={t('settings.appVersion')}>
            <Tag color="info">{version || '—'}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label={t('settings.lastSync')}>
            {new Date().toLocaleString()}
          </Descriptions.Item>
        </Descriptions>
      </Card>

      <Modal
        title={t('settings.changePassword')}
        open={pwdOpen}
        onCancel={() => setPwdOpen(false)}
        onOk={handlePasswordChange}
        confirmLoading={pwdLoading}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="currentPassword"
            label={t('settings.currentPassword')}
            rules={[{ required: true }]}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item
            name="newPassword"
            label={t('settings.newPassword')}
            rules={[{ required: true, min: 6 }]}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item
            name="confirmPassword"
            label={t('auth.passwordConfirm')}
            rules={[{ required: true }]}
          >
            <Input.Password />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
