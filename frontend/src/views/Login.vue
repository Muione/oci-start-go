<template>
  <div class="login-wrap">
    <el-card class="login-card">
      <template #header>
        <h2>oci-start 登录</h2>
      </template>
      <el-form :model="form" label-width="80px" @submit.prevent="onSubmit">
        <el-form-item label="用户名">
          <el-input v-model="form.username" autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" show-password autocomplete="current-password" />
        </el-form-item>
        <el-form-item v-if="init.turnstile.enabled" label="验证">
          <div ref="turnstileEl"></div>
        </el-form-item>
        <el-form-item v-if="init.mfaEnabled" label="MFA">
          <el-input v-model="form.mfaCode" placeholder="6 位动态码" />
        </el-form-item>
        <el-form-item label="">
          <el-checkbox v-model="form.rememberMe">记住我</el-checkbox>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="loading" @click="onSubmit">登录</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import request from '../utils/request'
import { encryptPassword } from '../utils/crypto'

const router = useRouter()
const route = useRoute()
const loading = ref(false)
const turnstileEl = ref<HTMLElement | null>(null)

const init = reactive({
  preLoginToken: '',
  publicKey: '',
  turnstile: { enabled: false, siteKey: '' },
  mfaEnabled: false,
  firstUserRegistered: true,
})
const form = reactive({
  username: '',
  password: '',
  mfaCode: '',
  rememberMe: false,
  turnstileToken: '',
})

onMounted(async () => {
  await loadInit()
  if (!init.firstUserRegistered) {
    router.replace('/first-user')
    return
  }
  if (init.turnstile.enabled) loadTurnstile()
})

async function loadInit() {
  const data = (await request.get('/api/login/init')) as any
  Object.assign(init, data)
}

function loadTurnstile() {
  const s = document.createElement('script')
  s.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?onload=onTurnstileLoad&render=explicit'
  s.async = true
  window.onTurnstileLoad = () => {
    if (turnstileEl.value && window.turnstile) {
      window.turnstile.render(turnstileEl.value, {
        sitekey: init.turnstile.siteKey,
        callback: (token: string) => { form.turnstileToken = token },
      })
    }
  }
  document.head.appendChild(s)
}

async function onSubmit() {
  loading.value = true
  try {
    const password = encryptPassword(form.password, init.publicKey)
    await request.post('/api/login', {
      preLoginToken: init.preLoginToken,
      username: form.username,
      password,
      rememberMe: form.rememberMe,
      turnstileToken: form.turnstileToken,
      mfaCode: form.mfaCode,
    })
    const redirect = (route.query.redirect as string) || '/'
    router.push(redirect)
  } catch (e: any) {
    ElMessage.error(e.message || '登录失败')
    await loadInit()
    form.turnstileToken = ''
    if (init.turnstile.enabled) {
      // re-render turnstile widget with fresh token
      if (turnstileEl.value) turnstileEl.value.innerHTML = ''
      loadTurnstile()
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100vh;
  background: linear-gradient(135deg, #0f1115 0%, #1a1f2e 30%, #141820 60%, #0f1115 100%);
  position: relative;
  overflow: hidden;
}

.login-wrap::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background:
    radial-gradient(circle at 20% 50%, rgba(129, 153, 240, 0.06) 0%, transparent 50%),
    radial-gradient(circle at 80% 80%, rgba(129, 153, 240, 0.04) 0%, transparent 50%);
  pointer-events: none;
}

.login-card {
  width: 420px;
  z-index: 10;
  border: 1px solid var(--border-default);
  box-shadow: var(--shadow-overlay);
  border-radius: var(--radius-lg);
  animation: slide-up 0.5s ease-out;
  background: var(--bg-surface);
}

.login-card :deep(.el-card__header) {
  background: transparent;
  border-bottom: 1px solid var(--border-subtle);
  padding: var(--space-6);
}

.login-card :deep(.el-card__header h2) {
  color: var(--text-primary);
  font-size: var(--text-xl);
  font-weight: var(--font-bold);
  margin: 0;
  letter-spacing: var(--tracking-tight);
}

.login-card :deep(.el-card__body) {
  background: var(--bg-surface);
  padding: var(--space-6);
}

.login-card :deep(.el-form-item__label) {
  color: var(--text-secondary);
  font-weight: var(--font-medium);
}

.login-card :deep(.el-button) {
  width: 100%;
  font-size: var(--text-md);
  font-weight: var(--font-semibold);
  height: 40px;
  border-radius: var(--radius-md);
}

.login-card :deep(.el-checkbox__label) {
  color: var(--text-secondary);
  font-weight: var(--font-medium);
}

@keyframes slide-up {
  from {
    opacity: 0;
    transform: translateY(30px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 480px) {
  .login-card {
    width: 100%;
    margin: 0 16px;
  }
}
</style>
