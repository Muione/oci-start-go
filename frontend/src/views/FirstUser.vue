<template>
  <div class="login-wrap">
    <el-card class="login-card">
      <template #header>
        <h2>初始化管理员</h2>
      </template>
      <el-alert type="info" :closable="false" show-icon style="margin-bottom: 16px">
        系统尚未初始化，请创建首个管理员账户。
      </el-alert>
      <el-form :model="form" label-width="100px" @submit.prevent="onSubmit">
        <el-form-item label="用户名">
          <el-input v-model="form.username" autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="loading" @click="onSubmit">注册并初始化</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import request from '../utils/request'
import { encryptPassword } from '../utils/crypto'

const router = useRouter()
const loading = ref(false)
const pub = reactive({ preLoginToken: '', publicKey: '' })
const form = reactive({ username: '', password: '' })

onMounted(async () => {
  try {
    const check = await request.get('/api/config/initialized') as any
    if (check?.initialized) {
      ElMessage.warning('系统已初始化，请直接登录')
      router.push('/login')
      return
    }
  } catch { /* fall through — allow registration attempt */ }
  const data = (await request.get('/api/login/init')) as any
  pub.preLoginToken = data.preLoginToken
  pub.publicKey = data.publicKey
})

async function onSubmit() {
  loading.value = true
  try {
    const password = encryptPassword(form.password, pub.publicKey)
    await request.post('/api/register-first-user', {
      preLoginToken: pub.preLoginToken,
      username: form.username,
      password,
    })
    ElMessage.success('注册成功，请登录')
    router.push('/login')
  } catch (e: any) {
    ElMessage.error(e.message || '注册失败')
    const data = (await request.get('/api/login/init')) as any
    pub.preLoginToken = data.preLoginToken
    pub.publicKey = data.publicKey
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-wrap { display: flex; align-items: center; justify-content: center; height: 100vh; background: #f0f2f5; }
.login-card { width: 420px; }
</style>
