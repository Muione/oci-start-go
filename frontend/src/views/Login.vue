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
  background: linear-gradient(135deg, #667eea 0%, #764ba2 25%, #f093fb 50%, #4facfe 75%, #00f2fe 100%);
  background-size: 400% 400%;
  animation: gradient-animation 15s ease infinite;
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
  background: radial-gradient(circle at 20% 50%, rgba(255, 255, 255, 0.1) 0%, transparent 50%),
              radial-gradient(circle at 80% 80%, rgba(255, 255, 255, 0.1) 0%, transparent 50%);
  pointer-events: none;
}

.login-card {
  width: 420px;
  z-index: 10;
  border: 1px solid rgba(255, 255, 255, 0.2);
  backdrop-filter: blur(10px);
  box-shadow: 0 8px 32px 0 rgba(31, 38, 135, 0.37);
  border-radius: 16px;
  animation: slide-up 0.5s ease-out;
}

.login-card :deep(.el-card__header) {
  background: transparent;
  border-bottom: 2px solid rgba(255, 255, 255, 0.1);
  padding: 24px;
}

.login-card :deep(.el-card__header h2) {
  color: #fff;
  font-size: 24px;
  font-weight: 700;
  text-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
  margin: 0;
  background: linear-gradient(135deg, #667eea, #764ba2);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.login-card :deep(.el-card__body) {
  background: rgba(255, 255, 255, 0.95);
  padding: 24px;
}

.login-card :deep(.el-form-item__label) {
  color: #1e293b;
  font-weight: 600;
}

.login-card :deep(.el-button) {
  width: 100%;
  font-size: 16px;
  font-weight: 600;
  height: 40px;
  border-radius: 8px;
  transition: all 0.3s ease;
}

.login-card :deep(.el-button--primary) {
  background: linear-gradient(135deg, #667eea, #764ba2);
  border: none;
  box-shadow: 0 4px 15px rgba(102, 126, 234, 0.4);
}

.login-card :deep(.el-button--primary:hover) {
  background: linear-gradient(135deg, #764ba2, #667eea);
  box-shadow: 0 8px 25px rgba(102, 126, 234, 0.6);
  transform: translateY(-2px);
}

.login-card :deep(.el-checkbox__label) {
  color: #64748b;
  font-weight: 500;
}

@keyframes gradient-animation {
  0% {
    background-position: 0% 50%;
  }
  50% {
    background-position: 100% 50%;
  }
  100% {
    background-position: 0% 50%;
  }
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
