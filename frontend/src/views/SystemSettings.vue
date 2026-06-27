<template>
  <div class="settings-page">
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>系统设置</h2>
        <el-tag type="info" size="small">{{ config.appVersion ? 'v' + config.appVersion : '' }}</el-tag>
      </div>
      <el-button type="primary" @click="loadConfig" :loading="loading">
        <el-icon><Refresh /></el-icon> 刷新
      </el-button>
    </div>

    <!-- User Management -->
    <el-card shadow="hover" class="section-card">
      <template #header>
        <div class="card-header">
          <span>👤 用户管理</span>
        </div>
      </template>
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="当前用户">
          <el-tag type="primary" size="small">{{ user.username }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="用户角色">
          <el-tag :type="user.role === 'ADMIN' ? 'danger' : 'info'" size="small">
            {{ user.role || 'USER' }}
          </el-tag>
        </el-descriptions-item>
      </el-descriptions>
      <div style="margin-top: 16px; display: flex; gap: 12px; flex-wrap: wrap;">
        <el-button type="warning" @click="openChangePassword">
          <el-icon><Lock /></el-icon> 修改密码
        </el-button>
        <el-button @click="openEdit('app.version', config.appVersion || '')">
          <el-icon><Edit /></el-icon> 编辑版本号
        </el-button>
      </div>
    </el-card>

    <!-- Notification Channels -->
    <el-card shadow="hover" class="section-card">
      <template #header>
        <div class="card-header">
          <span>📢 通知渠道</span>
        </div>
      </template>
      <el-row :gutter="16">
        <!-- Telegram -->
        <el-col :md="12" :sm="24" style="margin-bottom:16px">
          <el-card shadow="none" class="channel-card" :class="{ configured: cfgStr('telegram.bot.token') }">
            <template #header>
              <div class="channel-header">
                <span>📨 Telegram</span>
                <el-tag :type="cfgStr('telegram.bot.token') ? 'success' : 'info'" size="small">
                  {{ cfgStr('telegram.bot.token') ? '已配置' : '未配置' }}
                </el-tag>
              </div>
            </template>
            <el-descriptions :column="1" size="small" border>
              <el-descriptions-item label="Bot Token">
                <span v-if="cfgStr('telegram.bot.token')" style="color:#67c23a">••••••••</span>
                <span v-else style="color:#909399">未配置</span>
                <el-button size="small" type="primary" link style="margin-left:8px" @click="openEdit('telegram.bot.token', cfgStr('telegram.bot.token'))">编辑</el-button>
              </el-descriptions-item>
              <el-descriptions-item label="Chat ID">
                {{ cfgStr('telegram.chat.id') || '未配置' }}
                <el-button size="small" type="primary" link style="margin-left:8px" @click="openEdit('telegram.chat.id', cfgStr('telegram.chat.id'))">编辑</el-button>
              </el-descriptions-item>
            </el-descriptions>
          </el-card>
        </el-col>

        <!-- DingTalk -->
        <el-col :md="12" :sm="24" style="margin-bottom:16px">
          <el-card shadow="none" class="channel-card" :class="{ configured: cfgStr('dingtalk.webhook') }">
            <template #header>
              <div class="channel-header">
                <span>🔔 钉钉</span>
                <el-tag :type="cfgStr('dingtalk.webhook') ? 'success' : 'info'" size="small">
                  {{ cfgStr('dingtalk.webhook') ? '已配置' : '未配置' }}
                </el-tag>
              </div>
            </template>
            <el-descriptions :column="1" size="small" border>
              <el-descriptions-item label="Webhook URL">
                <span v-if="cfgStr('dingtalk.webhook')" style="color:#67c23a">••••••••</span>
                <span v-else style="color:#909399">未配置</span>
                <el-button size="small" type="primary" link style="margin-left:8px" @click="openEdit('dingtalk.webhook', cfgStr('dingtalk.webhook'))">编辑</el-button>
              </el-descriptions-item>
              <el-descriptions-item label="签名密钥">
                {{ cfgStr('dingtalk.secret') ? '*** (已设置)' : '未配置（可选）' }}
                <el-button size="small" type="primary" link style="margin-left:8px" @click="openEdit('dingtalk.secret', cfgStr('dingtalk.secret'))">编辑</el-button>
              </el-descriptions-item>
            </el-descriptions>
          </el-card>
        </el-col>

        <!-- Bark -->
        <el-col :md="12" :sm="24" style="margin-bottom:16px">
          <el-card shadow="none" class="channel-card" :class="{ configured: cfgStr('bark.key') }">
            <template #header>
              <div class="channel-header">
                <span>📱 Bark (iOS)</span>
                <el-tag :type="cfgStr('bark.key') ? 'success' : 'info'" size="small">
                  {{ cfgStr('bark.key') ? '已配置' : '未配置' }}
                </el-tag>
              </div>
            </template>
            <el-descriptions :column="1" size="small" border>
              <el-descriptions-item label="Device Key">
                {{ cfgStr('bark.key') || '未配置' }}
                <el-button size="small" type="primary" link style="margin-left:8px" @click="openEdit('bark.key', cfgStr('bark.key'))">编辑</el-button>
              </el-descriptions-item>
              <el-descriptions-item label="Server">
                {{ cfgStr('bark.server') || 'https://api.day.app (默认)' }}
                <el-button size="small" type="primary" link style="margin-left:8px" @click="openEdit('bark.server', cfgStr('bark.server'))">编辑</el-button>
              </el-descriptions-item>
            </el-descriptions>
          </el-card>
        </el-col>

        <!-- Feishu -->
        <el-col :md="12" :sm="24" style="margin-bottom:16px">
          <el-card shadow="none" class="channel-card" :class="{ configured: cfgStr('feishu.webhook') }">
            <template #header>
              <div class="channel-header">
                <span>🧧 飞书</span>
                <el-tag :type="cfgStr('feishu.webhook') ? 'success' : 'info'" size="small">
                  {{ cfgStr('feishu.webhook') ? '已配置' : '未配置' }}
                </el-tag>
              </div>
            </template>
            <el-descriptions :column="1" size="small" border>
              <el-descriptions-item label="Webhook URL">
                <span v-if="cfgStr('feishu.webhook')" style="color:#67c23a">••••••••</span>
                <span v-else style="color:#909399">未配置</span>
                <el-button size="small" type="primary" link style="margin-left:8px" @click="openEdit('feishu.webhook', cfgStr('feishu.webhook'))">编辑</el-button>
              </el-descriptions-item>
              <el-descriptions-item label="签名密钥">
                {{ cfgStr('feishu.secret') ? '*** (已设置)' : '未配置（可选）' }}
                <el-button size="small" type="primary" link style="margin-left:8px" @click="openEdit('feishu.secret', cfgStr('feishu.secret'))">编辑</el-button>
              </el-descriptions-item>
            </el-descriptions>
          </el-card>
        </el-col>
      </el-row>
    </el-card>

    <!-- DNS Providers -->
    <el-card shadow="hover" class="section-card">
      <template #header>
        <div class="card-header">
          <span>🌐 DNS 服务</span>
        </div>
      </template>
      <el-row :gutter="16">
        <el-col :md="12" :sm="24" style="margin-bottom:16px">
          <el-card shadow="none" class="channel-card" :class="{ configured: cfgStr('cloudflare.email') }">
            <template #header>
              <div class="channel-header">
                <span>☁️ Cloudflare</span>
                <el-tag :type="cfgStr('cloudflare.email') ? 'success' : 'info'" size="small">
                  {{ cfgStr('cloudflare.email') ? '已配置' : '未配置' }}
                </el-tag>
              </div>
            </template>
            <el-descriptions :column="1" size="small" border>
              <el-descriptions-item label="Email">
                {{ cfgStr('cloudflare.email') || '未配置' }}
                <el-button size="small" type="primary" link style="margin-left:8px" @click="openEdit('cloudflare.email', cfgStr('cloudflare.email'))">编辑</el-button>
              </el-descriptions-item>
              <el-descriptions-item label="API Key">
                {{ cfgStr('cloudflare.api.key') ? '*** (已设置)' : '未配置' }}
                <el-button size="small" type="primary" link style="margin-left:8px" @click="openEdit('cloudflare.api.key', cfgStr('cloudflare.api.key'))">编辑</el-button>
              </el-descriptions-item>
            </el-descriptions>
          </el-card>
        </el-col>

        <el-col :md="12" :sm="24" style="margin-bottom:16px">
          <el-card shadow="none" class="channel-card" :class="{ configured: cfgStr('edgeone.secretId') }">
            <template #header>
              <div class="channel-header">
                <span>🔷 EdgeOne (腾讯云)</span>
                <el-tag :type="cfgStr('edgeone.secretId') ? 'success' : 'info'" size="small">
                  {{ cfgStr('edgeone.secretId') ? '已配置' : '未配置' }}
                </el-tag>
              </div>
            </template>
            <el-descriptions :column="1" size="small" border>
              <el-descriptions-item label="Secret ID">
                {{ cfgStr('edgeone.secretId') ? '*** (已设置)' : '未配置' }}
                <el-button size="small" type="primary" link style="margin-left:8px" @click="openEdit('edgeone.secretId', cfgStr('edgeone.secretId'))">编辑</el-button>
              </el-descriptions-item>
              <el-descriptions-item label="Zone ID">
                {{ cfgStr('edgeone.zoneId') || '未配置' }}
                <el-button size="small" type="primary" link style="margin-left:8px" @click="openEdit('edgeone.zoneId', cfgStr('edgeone.zoneId'))">编辑</el-button>
              </el-descriptions-item>
            </el-descriptions>
          </el-card>
        </el-col>
      </el-row>
    </el-card>

    <!-- SSL Certificate -->
    <el-card shadow="hover" class="section-card">
      <template #header>
        <div class="card-header">
          <span>🔒 SSL 证书</span>
        </div>
      </template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="域名">
          {{ cfgStr('ssl.domain') || '未配置' }}
          <el-button size="small" type="primary" link style="margin-left:8px" @click="openEdit('ssl.domain', cfgStr('ssl.domain'))">编辑</el-button>
        </el-descriptions-item>
        <el-descriptions-item label="Email">
          {{ cfgStr('ssl.email') || '未配置' }}
          <el-button size="small" type="primary" link style="margin-left:8px" @click="openEdit('ssl.email', cfgStr('ssl.email'))">编辑</el-button>
        </el-descriptions-item>
        <el-descriptions-item label="模式">
          <el-tag :type="config.bools?.['ssl.staging'] ? 'warning' : 'success'" size="small">
            {{ config.bools?.['ssl.staging'] ? 'Staging (测试)' : 'Production (生产)' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="自动续期">
          <el-tag type="info" size="small">每天 04:00 (SslCertJob)</el-tag>
        </el-descriptions-item>
      </el-descriptions>
      <div v-if="!cfgStr('ssl.domain')" style="margin-top:12px">
        <el-alert
          title="SSL 证书未配置"
          type="info"
          description="在系统配置中设置 ssl.domain、ssl.email、cloudflare.email、cloudflare.api.key 以启用 Let's Encrypt 自动签发/续期。"
          :closable="false"
          show-icon
        />
      </div>
    </el-card>

    <!-- Security & Auth -->
    <el-card shadow="hover" class="section-card">
      <template #header>
        <div class="card-header">
          <span>🔐 安全与认证</span>
        </div>
      </template>
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="MFA">
          <el-tag :type="config.bools?.['mfa.enabled'] ? 'success' : 'info'" size="small">
            {{ config.bools?.['mfa.enabled'] ? '已启用' : '已禁用' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="Turnstile">
          <el-tag :type="config.bools?.['turnstile.enabled'] ? 'success' : 'info'" size="small">
            {{ config.bools?.['turnstile.enabled'] ? '已启用' : '已禁用' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="GitHub OAuth">
          {{ cfgStr('github.clientId') ? '已配置' : '未配置' }}
        </el-descriptions-item>
        <el-descriptions-item label="Google OAuth">
          {{ cfgStr('google.clientId') ? '已配置' : '未配置' }}
        </el-descriptions-item>
        <el-descriptions-item label="GCP Service Account">
          {{ cfgStr('gcp.serviceAccountJson') ? '已配置' : '未配置' }}
        </el-descriptions-item>
        <el-descriptions-item label="GCP Project ID">
          {{ cfgStr('gcp.projectId') || '未配置' }}
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- Change Password Dialog -->
    <el-dialog v-model="pwdVisible" title="修改密码" width="460px" destroy-on-close>
      <el-form :model="pwdForm" label-width="100px">
        <el-form-item label="当前密码" required>
          <el-input v-model="pwdForm.currentPassword" type="password" show-password placeholder="输入当前密码" />
        </el-form-item>
        <el-form-item label="新密码" required>
          <el-input v-model="pwdForm.newPassword" type="password" show-password placeholder="至少 6 位字符" />
        </el-form-item>
        <el-form-item label="确认密码" required>
          <el-input v-model="pwdForm.confirmPassword" type="password" show-password placeholder="再次输入新密码" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="pwdVisible = false">取消</el-button>
        <el-button type="primary" :loading="pwdSaving" @click="doChangePassword">确认修改</el-button>
      </template>
    </el-dialog>

    <!-- Edit Config Dialog -->
    <el-dialog v-model="editVisible" title="编辑配置" width="520px" destroy-on-close>
      <el-form :model="{ key: editKey, value: editValue }" label-width="120px">
        <el-form-item label="配置键">
          <el-input :model-value="editKey" disabled />
        </el-form-item>
        <el-form-item label="配置值">
          <el-input v-model="editValue" type="textarea" :rows="3" placeholder="输入新的配置值" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="editSaving" @click="saveEdit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Edit, Lock } from '@element-plus/icons-vue'
import { useUserStore } from '../store/user'
import request from '../utils/request'
import type { SystemConfig } from '../types/api'

const user = useUserStore()
const loading = ref(false)
const config = ref<SystemConfig>({
  strings: {},
  bools: {},
  appVersion: '',
})

// Edit dialog state
const editVisible = ref(false)
const editKey = ref('')
const editValue = ref('')
const editSaving = ref(false)

// Password change state
const pwdVisible = ref(false)
const pwdSaving = ref(false)
const pwdForm = ref({ currentPassword: '', newPassword: '', confirmPassword: '' })

function cfgStr(key: string): string {
  return config.value.strings?.[key] || ''
}

async function loadConfig() {
  loading.value = true
  try {
    const data: SystemConfig = await request.get('/system/config')
    if (data) {
      config.value = data
    }
  } catch { /* silently ignore */ }
  loading.value = false
}

function openEdit(key: string, currentValue: string) {
  editKey.value = key
  editValue.value = currentValue
  editVisible.value = true
}

async function saveEdit() {
  editSaving.value = true
  try {
    await request.post('/system/config/save', { key: editKey.value, value: editValue.value })
    ElMessage.success('配置已保存')
    editVisible.value = false
    await loadConfig()
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    editSaving.value = false
  }
}

function openChangePassword() {
  pwdForm.value = { currentPassword: '', newPassword: '', confirmPassword: '' }
  pwdVisible.value = true
}

async function doChangePassword() {
  if (!pwdForm.value.currentPassword) {
    ElMessage.warning('请输入当前密码')
    return
  }
  if (!pwdForm.value.newPassword || pwdForm.value.newPassword.length < 6) {
    ElMessage.warning('新密码至少 6 位字符')
    return
  }
  if (pwdForm.value.newPassword !== pwdForm.value.confirmPassword) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  pwdSaving.value = true
  try {
    await request.post('/api/change-password', {
      currentPassword: pwdForm.value.currentPassword,
      newPassword: pwdForm.value.newPassword,
    })
    ElMessage.success('密码修改成功')
    pwdVisible.value = false
  } catch (e: any) {
    ElMessage.error(e.message || '密码修改失败')
  } finally {
    pwdSaving.value = false
  }
}

onMounted(() => {
  loadConfig()
})
</script>

<style scoped>
.settings-page {
  padding: 0;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 28px;
  flex-wrap: wrap;
  gap: 16px;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.toolbar-left h2 {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  color: #1e293b;
  background: linear-gradient(135deg, #0066ff, #00bcd4);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.toolbar-left :deep(.el-tag) {
  border-radius: 6px;
  font-weight: 600;
  background: rgba(0, 102, 255, 0.1);
  color: #0066ff;
  border: none;
}

.section-card {
  margin-bottom: 24px;
  border-radius: 16px;
  border: 1px solid rgba(0, 102, 255, 0.1);
  background: linear-gradient(135deg, #ffffff, #f8fafc);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.section-card:hover {
  box-shadow: 0 20px 40px rgba(0, 102, 255, 0.15);
  transform: translateY(-4px);
  border-color: rgba(0, 102, 255, 0.2);
}

.section-card :deep(.el-card__header) {
  padding: 20px 24px;
  background: linear-gradient(90deg, rgba(0, 102, 255, 0.03), rgba(0, 188, 212, 0.03));
  border-bottom: 1px solid rgba(0, 102, 255, 0.1);
}

.section-card :deep(.el-card__body) {
  padding: 24px;
  background: #ffffff;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 16px;
  font-weight: 700;
  color: #1e293b;
}

.channel-card {
  border: 1px solid rgba(0, 102, 255, 0.1);
  border-radius: 12px;
  transition: all 0.3s ease;
  background: linear-gradient(135deg, #f8fafc, #ffffff);
}

.channel-card:hover {
  border-color: rgba(0, 102, 255, 0.2);
  box-shadow: 0 8px 16px rgba(0, 102, 255, 0.1);
  transform: translateY(-2px);
}

.channel-card.configured {
  border-color: rgba(16, 185, 129, 0.3);
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.05), #ffffff);
}

.channel-card.configured::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: linear-gradient(90deg, #10b981, #34d399);
  border-radius: 12px 12px 0 0;
}

.channel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(0, 102, 255, 0.1);
}

:deep(.el-descriptions) {
  background: transparent;
}

:deep(.el-descriptions__body) {
  background: transparent;
}

:deep(.el-descriptions__item) {
  background: transparent;
}

:deep(.el-descriptions__cell) {
  padding: 12px 0;
  color: #64748b;
}

:deep(.el-descriptions__label) {
  font-weight: 600;
  color: #1e293b;
}

:deep(.el-button) {
  border-radius: 8px;
  transition: all 0.3s ease;
}

:deep(.el-button--primary:hover) {
  box-shadow: 0 8px 16px rgba(0, 102, 255, 0.3);
  transform: translateY(-1px);
}

:deep(.el-button--warning:hover) {
  box-shadow: 0 8px 16px rgba(245, 158, 11, 0.3);
  transform: translateY(-1px);
}

:deep(.el-button--info:hover) {
  box-shadow: 0 8px 16px rgba(0, 102, 255, 0.3);
  transform: translateY(-1px);
}

:deep(.el-tag) {
  border-radius: 8px;
  border: none;
  font-weight: 600;
  padding: 6px 12px;
}

:deep(.el-dialog) {
  border-radius: 16px;
}

:deep(.el-dialog__header) {
  border-bottom: 1px solid rgba(0, 102, 255, 0.1);
  padding: 20px 24px;
}

:deep(.el-dialog__title) {
  font-size: 18px;
  font-weight: 700;
  color: #1e293b;
}

@media (max-width: 768px) {
  .toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .toolbar-left h2 {
    font-size: 20px;
  }

  .section-card {
    margin-bottom: 16px;
  }

  .card-header {
    font-size: 14px;
  }
}
</style>
