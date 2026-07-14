import { useEffect, useState } from 'react'
import { Button, Card, Form, Input, message, Typography } from 'antd'
import { LockOutlined, UserOutlined } from '@ant-design/icons'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { JSEncrypt } from 'jsencrypt'
import client from '@/api/client'

const { Title, Text } = Typography

// ── Schema ──────────────────────────────────────────────────────────────

function schema(t: (k: string) => string) {
  return z
    .object({
      username: z.string().min(2, t('auth.usernameMinLength')),
      password: z.string().min(6, t('auth.passwordMinLength')),
      confirmPassword: z.string().min(6, t('auth.passwordConfirmRequired')),
    })
    .refine((d) => d.password === d.confirmPassword, {
      message: t('auth.passwordMismatch'),
      path: ['confirmPassword'],
    })
}

type FormValues = z.infer<ReturnType<typeof schema>>

// ── Component ───────────────────────────────────────────────────────────

export default function FirstUser() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [loading, setLoading] = useState(false)
  const [checking, setChecking] = useState(true)

  const {
    control,
    handleSubmit,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema(t)),
    defaultValues: { username: '', password: '', confirmPassword: '' },
  })

  // Check if system is already initialized
  useEffect(() => {
    client
      .get('/api/config/initialized')
      .then((resp) => {
        const data = resp as unknown as { initialized: boolean }
        if (data.initialized) {
          navigate('/login', { replace: true })
        }
      })
      .catch(() => {
        // If we can't check, allow the form to proceed
      })
      .finally(() => setChecking(false))
  }, [navigate])

  const onSubmit = async (values: FormValues) => {
    setLoading(true)
    try {
      // Fetch login init for RSA public key
      const resp = await client.get('/api/login/init')
      const init = resp as unknown as { preLoginToken: string; publicKey: string }
      const encrypt = new JSEncrypt()
      encrypt.setPublicKey(init.publicKey)
      const encrypted = encrypt.encrypt(values.password)
      if (!encrypted) {
        message.error(t('auth.loginFailed'))
        return
      }

      await client.post('/api/register-first-user', {
        preLoginToken: init.preLoginToken,
        username: values.username,
        password: encrypted,
      })
      message.success(t('auth.firstUserSuccess'))
      navigate('/login', { replace: true })
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('auth.loginFailed')
      message.error(msg)
    } finally {
      setLoading(false)
    }
  }

  if (checking) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <Card loading className="w-96" />
      </div>
    )
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <Card className="w-full max-w-md shadow-md">
        <div className="text-center mb-6">
          <Title level={3} className="!mb-1">
            {t('auth.firstUserTitle')}
          </Title>
          <Text type="secondary">{t('auth.firstUserSubtitle')}</Text>
        </div>

        <Form layout="vertical" onFinish={handleSubmit(onSubmit)} autoComplete="off">
          <Form.Item
            label={t('auth.username')}
            validateStatus={errors.username ? 'error' : ''}
            help={errors.username?.message}
          >
            <Controller
              name="username"
              control={control}
              render={({ field }) => (
                <Input
                  {...field}
                  prefix={<UserOutlined />}
                  placeholder={t('auth.username')}
                  size="large"
                />
              )}
            />
          </Form.Item>

          <Form.Item
            label={t('auth.password')}
            validateStatus={errors.password ? 'error' : ''}
            help={errors.password?.message}
          >
            <Controller
              name="password"
              control={control}
              render={({ field }) => (
                <Input.Password
                  {...field}
                  prefix={<LockOutlined />}
                  placeholder={t('auth.password')}
                  size="large"
                />
              )}
            />
          </Form.Item>

          <Form.Item
            label={t('auth.passwordConfirm')}
            validateStatus={errors.confirmPassword ? 'error' : ''}
            help={errors.confirmPassword?.message}
          >
            <Controller
              name="confirmPassword"
              control={control}
              render={({ field }) => (
                <Input.Password
                  {...field}
                  prefix={<LockOutlined />}
                  placeholder={t('auth.passwordConfirm')}
                  size="large"
                />
              )}
            />
          </Form.Item>

          <Form.Item>
            <Button
              type="primary"
              htmlType="submit"
              block
              size="large"
              loading={loading}
            >
              {t('auth.firstUserButton')}
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}
