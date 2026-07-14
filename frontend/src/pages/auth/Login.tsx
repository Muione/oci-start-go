import { useEffect, useState } from 'react'
import { Button, Card, Divider, Form, Input, message, Steps, Typography } from 'antd'
import {
  GithubOutlined,
  GoogleOutlined,
  LockOutlined,
  SafetyCertificateOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useTranslation } from 'react-i18next'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { JSEncrypt } from 'jsencrypt'
import client from '@/api/client'
import { useAuth } from '@/hooks/useAuth'

const { Title, Text, Link } = Typography

// ── Login form schema ───────────────────────────────────────────────────

function loginSchema(t: (k: string) => string) {
  return z.object({
    username: z.string().min(1, t('auth.usernameMinLength')),
    password: z.string().min(1, t('auth.passwordMinLength')),
    mfaCode: z.string().optional(),
  })
}

type LoginForm = z.infer<ReturnType<typeof loginSchema>>

// ── Password-reset schemas ──────────────────────────────────────────────

function resetStep1Schema(t: (k: string) => string) {
  return z.object({
    username: z.string().min(2, t('auth.usernameMinLength')),
  })
}

function resetStep2Schema() {
  return z.object({
    code: z.string().min(1, 'auth.codeRequired'),
  })
}

function resetStep3Schema(t: (k: string) => string) {
  return z.object({
    newPassword: z.string().min(6, t('auth.passwordMinLength')),
    confirmPassword: z.string().min(6, t('auth.passwordConfirmRequired')),
  }).refine((d) => d.newPassword === d.confirmPassword, {
    message: t('auth.passwordMismatch'),
    path: ['confirmPassword'],
  })
}

type ResetStep1 = z.infer<ReturnType<typeof resetStep1Schema>>
type ResetStep2 = z.infer<ReturnType<typeof resetStep2Schema>>
type ResetStep3 = z.infer<ReturnType<typeof resetStep3Schema>>

// ── Init response shape ─────────────────────────────────────────────────

interface LoginInitResp {
  preLoginToken: string
  publicKey: string
  turnstile: { enabled: boolean; siteKey: string }
  mfaEnabled: boolean
  githubEnabled: boolean
  googleEnabled?: boolean
  firstUserRegistered: boolean
}

// ── Component ───────────────────────────────────────────────────────────

export default function Login() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { login } = useAuth()

  // Login-init state
  const [initData, setInitData] = useState<LoginInitResp | null>(null)
  const [loading, setLoading] = useState(false)

  // Password-reset flow: 'login' | 0 | 1 | 2
  const [mode, setMode] = useState<'login' | 0 | 1 | 2>('login')
  const [resetUsername, setResetUsername] = useState('')
  const [resetLoading, setResetLoading] = useState(false)

  // Fetch login init on mount
  useEffect(() => {
    client
      .get('/api/login/init')
      .then((resp) => {
        const data = resp as unknown as LoginInitResp
        if (!data.firstUserRegistered) {
          navigate('/first-user', { replace: true })
          return
        }
        setInitData(data)
      })
      .catch(() => {
        // Backend may be down — still show form
        setInitData({
          preLoginToken: '',
          publicKey: '',
          turnstile: { enabled: false, siteKey: '' },
          mfaEnabled: false,
          githubEnabled: false,
          firstUserRegistered: true,
        })
      })
  }, [navigate])

  // ── Login form ────────────────────────────────────────────────────────

  const loginForm = useForm<LoginForm>({
    resolver: zodResolver(loginSchema(t)),
    defaultValues: { username: '', password: '', mfaCode: '' },
  })

  const onLogin = async (values: LoginForm) => {
    if (!initData) return
    setLoading(true)
    try {
      // RSA-encrypt the password
      const encrypt = new JSEncrypt()
      encrypt.setPublicKey(initData.publicKey)
      const encrypted = encrypt.encrypt(values.password)
      if (!encrypted) {
        message.error(t('auth.loginFailed'))
        return
      }

      await login(values.username, encrypted, initData.preLoginToken, {
        rememberMe: false,
        mfaCode: values.mfaCode,
      })
      message.success(t('auth.loginSuccess'))
      const redirect = searchParams.get('redirect') || '/'
      navigate(redirect, { replace: true })
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('auth.loginFailed')
      message.error(msg)
    } finally {
      setLoading(false)
    }
  }

  // ── OAuth handlers ────────────────────────────────────────────────────

  const handleOAuth = async (provider: 'github' | 'google') => {
    try {
      const data = await client.get(`/api/${provider}/login/url`) as { url: string }
      window.location.href = data.url
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('auth.loginFailed')
      message.error(msg)
    }
  }

  // ── Password-reset forms ──────────────────────────────────────────────

  const reset1Form = useForm<ResetStep1>({
    resolver: zodResolver(resetStep1Schema(t)),
    defaultValues: { username: '' },
  })

  const reset2Form = useForm<ResetStep2>({
    resolver: zodResolver(resetStep2Schema()),
    defaultValues: { code: '' },
  })

  const reset3Form = useForm<ResetStep3>({
    resolver: zodResolver(resetStep3Schema(t)),
    defaultValues: { newPassword: '', confirmPassword: '' },
  })

  const onResetStep1 = async (values: ResetStep1) => {
    setResetLoading(true)
    try {
      await client.post('/api/send-reset-code', { username: values.username })
      setResetUsername(values.username)
      message.success(t('auth.codeSent'))
      setMode(1)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('auth.loginFailed')
      message.error(msg)
    } finally {
      setResetLoading(false)
    }
  }

  const onResetStep2 = async (values: ResetStep2) => {
    setResetLoading(true)
    try {
      await client.post('/api/verify-reset-code', {
        username: resetUsername,
        code: values.code,
      })
      message.success(t('auth.codeVerified'))
      setMode(2)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('auth.loginFailed')
      message.error(msg)
    } finally {
      setResetLoading(false)
    }
  }

  const onResetStep3 = async (values: ResetStep3) => {
    setResetLoading(true)
    try {
      await client.post('/api/reset-password', {
        username: resetUsername,
        code: reset2Form.getValues('code'),
        newPassword: values.newPassword,
      })
      message.success(t('auth.resetSuccess'))
      setMode('login')
      reset1Form.reset()
      reset2Form.reset()
      reset3Form.reset()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('auth.loginFailed')
      message.error(msg)
    } finally {
      setResetLoading(false)
    }
  }

  // ── Render ────────────────────────────────────────────────────────────

  if (!initData) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <Card loading className="w-96" />
      </div>
    )
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <Card className="w-full max-w-md shadow-md">
        {/* Header */}
        <div className="text-center mb-6">
          <Title level={3} className="!mb-1">
            {t('auth.loginTitle')}
          </Title>
          <Text type="secondary">{t('auth.loginSubtitle')}</Text>
        </div>

        {/* Login form */}
        {mode === 'login' && (
          <>
            <Form
              layout="vertical"
              onFinish={loginForm.handleSubmit(onLogin)}
              autoComplete="off"
            >
              <Form.Item
                label={t('auth.username')}
                validateStatus={loginForm.formState.errors.username ? 'error' : ''}
                help={loginForm.formState.errors.username?.message}
              >
                <Controller
                  name="username"
                  control={loginForm.control}
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
                validateStatus={loginForm.formState.errors.password ? 'error' : ''}
                help={loginForm.formState.errors.password?.message}
              >
                <Controller
                  name="password"
                  control={loginForm.control}
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

              {initData.mfaEnabled && (
                <Form.Item
                  label={t('auth.mfaCode')}
                  validateStatus={loginForm.formState.errors.mfaCode ? 'error' : ''}
                  help={loginForm.formState.errors.mfaCode?.message}
                >
                  <Controller
                    name="mfaCode"
                    control={loginForm.control}
                    render={({ field }) => (
                      <Input
                        {...field}
                        prefix={<SafetyCertificateOutlined />}
                        placeholder={t('auth.mfaPlaceholder')}
                        size="large"
                        maxLength={6}
                      />
                    )}
                  />
                </Form.Item>
              )}

              <Form.Item className="!mb-2">
                <Button
                  type="primary"
                  htmlType="submit"
                  block
                  size="large"
                  loading={loading}
                >
                  {t('auth.loginButton')}
                </Button>
              </Form.Item>

              <div className="text-right mb-4">
                <Link onClick={() => setMode(0)}>
                  {t('auth.forgotPassword')}
                </Link>
              </div>
            </Form>

            {/* OAuth buttons */}
            {(initData.githubEnabled || initData.googleEnabled) && (
              <>
                <Divider plain>
                  <Text type="secondary" className="text-xs">
                    OR
                  </Text>
                </Divider>
                <div className="flex flex-col gap-2">
                  {initData.githubEnabled && (
                    <Button
                      block
                      icon={<GithubOutlined />}
                      size="large"
                      onClick={() => handleOAuth('github')}
                    >
                      {t('auth.githubLogin')}
                    </Button>
                  )}
                  {initData.googleEnabled && (
                    <Button
                      block
                      icon={<GoogleOutlined />}
                      size="large"
                      onClick={() => handleOAuth('google')}
                    >
                      {t('auth.googleLogin')}
                    </Button>
                  )}
                </div>
              </>
            )}
          </>
        )}

        {/* Password reset flow */}
        {mode !== 'login' && (
          <>
            <div className="text-center mb-4">
              <Title level={4} className="!mb-1">
                {t('auth.resetPassword')}
              </Title>
              <Text type="secondary">{t('auth.resetSubtitle')}</Text>
            </div>

            <Steps
              current={typeof mode === 'number' ? mode : 0}
              size="small"
              className="mb-6"
              items={[
                { title: t('auth.resetStep1') },
                { title: t('auth.resetStep2') },
                { title: t('auth.resetStep3') },
              ]}
            />

            {/* Step 1: Send code */}
            {mode === 0 && (
              <Form
                layout="vertical"
                onFinish={reset1Form.handleSubmit(onResetStep1)}
              >
                <Form.Item
                  label={t('auth.username')}
                  validateStatus={reset1Form.formState.errors.username ? 'error' : ''}
                  help={reset1Form.formState.errors.username?.message}
                >
                  <Controller
                    name="username"
                    control={reset1Form.control}
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
                <Form.Item className="!mb-2">
                  <Button
                    type="primary"
                    htmlType="submit"
                    block
                    size="large"
                    loading={resetLoading}
                  >
                    {t('auth.sendCode')}
                  </Button>
                </Form.Item>
                <Link onClick={() => setMode('login')}>
                  {t('auth.backToLogin')}
                </Link>
              </Form>
            )}

            {/* Step 2: Verify code */}
            {mode === 1 && (
              <Form
                layout="vertical"
                onFinish={reset2Form.handleSubmit(onResetStep2)}
              >
                <Form.Item
                  label={t('auth.resetCode')}
                  validateStatus={reset2Form.formState.errors.code ? 'error' : ''}
                  help={reset2Form.formState.errors.code?.message}
                >
                  <Controller
                    name="code"
                    control={reset2Form.control}
                    render={({ field }) => (
                      <Input
                        {...field}
                        prefix={<SafetyCertificateOutlined />}
                        placeholder={t('auth.resetCodePlaceholder')}
                        size="large"
                        maxLength={6}
                      />
                    )}
                  />
                </Form.Item>
                <Form.Item className="!mb-2">
                  <Button
                    type="primary"
                    htmlType="submit"
                    block
                    size="large"
                    loading={resetLoading}
                  >
                    {t('auth.verifyCode')}
                  </Button>
                </Form.Item>
                <Link onClick={() => setMode('login')}>
                  {t('auth.backToLogin')}
                </Link>
              </Form>
            )}

            {/* Step 3: Set new password */}
            {mode === 2 && (
              <Form
                layout="vertical"
                onFinish={reset3Form.handleSubmit(onResetStep3)}
              >
                <Form.Item
                  label={t('auth.newPassword')}
                  validateStatus={reset3Form.formState.errors.newPassword ? 'error' : ''}
                  help={reset3Form.formState.errors.newPassword?.message}
                >
                  <Controller
                    name="newPassword"
                    control={reset3Form.control}
                    render={({ field }) => (
                      <Input.Password
                        {...field}
                        prefix={<LockOutlined />}
                        placeholder={t('auth.newPasswordPlaceholder')}
                        size="large"
                      />
                    )}
                  />
                </Form.Item>
                <Form.Item
                  label={t('auth.passwordConfirm')}
                  validateStatus={reset3Form.formState.errors.confirmPassword ? 'error' : ''}
                  help={reset3Form.formState.errors.confirmPassword?.message}
                >
                  <Controller
                    name="confirmPassword"
                    control={reset3Form.control}
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
                <Form.Item className="!mb-2">
                  <Button
                    type="primary"
                    htmlType="submit"
                    block
                    size="large"
                    loading={resetLoading}
                  >
                    {t('auth.resetPassword')}
                  </Button>
                </Form.Item>
                <Link onClick={() => setMode('login')}>
                  {t('auth.backToLogin')}
                </Link>
              </Form>
            )}
          </>
        )}
      </Card>
    </div>
  )
}
