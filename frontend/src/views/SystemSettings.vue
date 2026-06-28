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
                <span v-if="cfgStr('telegram.bot.token')" style="color:var(--status-up)">••••••••</span>
                <span v-else style="color:var(--text-muted)">未配置</span>
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
                <span v-if="cfgStr('dingtalk.webhook')" style="color:var(--status-up)">••••••••</span>
                <span v-else style="color:var(--text-muted)">未配置</span>
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
                <span v-if="cfgStr('feishu.webhook')" style="color:var(--status-up)">••••••••</span>
                <span v-else style="color:var(--text-muted)">未配置</span>
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
          <el-card shadow="none" class="channel-card" :class="{ configured: cfgStr('cloudflare.api.token') }">
            <template #header>
              <div class="channel-header">
                <span>☁️ Cloudflare</span>
                <el-tag :type="cfgStr('cloudflare.api.token') ? 'success' : 'info'" size="small">
                  {{ cfgStr('cloudflare.api.token') ? '已配置' : '未配置' }}
                </el-tag>
              </div>
            </template>
            <el-descriptions :column="1" size="small" border>
              <el-descriptions-item label="API Token">
                {{ cfgStr('cloudflare.api.token') ? '*** (已设置)' : '未配置' }}
                <el-button size="small" type="primary" link style="margin-left:8px" @click="openEdit('cloudflare.api.token', cfgStr('cloudflare.api.token'))">编辑</el-button>
              </el-descriptions-item>
            </el-descriptions>
            <div style="font-size:12px;color:var(--text-muted);margin-top:8px">
              <i class="el-icon-info"></i> 请在 Cloudflare 控制台创建 API Token（推荐使用「编辑 DNS」模板）
            </div>
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
          description="在系统配置中设置 ssl.domain、ssl.email、cloudflare.api.token 以启用 Let's Encrypt 自动签发/续期。"
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
  margin-bottom: var(--space-6);
  flex-wrap: wrap;
  gap: var(--space-4);
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.toolbar-left h2 {
  margin: 0;
  font-size: var(--text-xl);
  font-weight: var(--font-bold);
  color: var(--text-primary);
  letter-spacing: var(--tracking-tight);
}

.toolbar-left :deep(.el-tag) {
  border-radius: var(--radius-sm);
  font-weight: var(--font-semibold);
  background: var(--accent-subtle);
  color: var(--accent);
  border: none;
}

.section-card {
  margin-bottom: var(--space-6);
  border-radius: var(--radius-md);
}

.section-card :deep(.el-card__body) {
  padding: var(--space-5);
}

.card-header {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-md);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

.channel-card {
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  transition: border-color var(--transition-fast);
  background: var(--bg-surface);
  position: relative;
  overflow: hidden;
}

.channel-card:hover {
  border-color: var(--accent);
}

.channel-card.configured {
  border-color: rgba(52, 168, 83, 0.3);
}

.channel-card.configured::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: var(--status-up);
  border-radius: var(--radius-md) var(--radius-md) 0 0;
}

.channel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: var(--space-3);
  border-bottom: 1px solid var(--border-subtle);
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
  color: var(--text-secondary);
}

:deep(.el-descriptions__label) {
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

:deep(.el-dialog) {
  border-radius: var(--radius-lg);
}

:deep(.el-dialog__title) {
  font-size: var(--text-lg);
  font-weight: var(--font-semibold);
}

@media (max-width: 768px) {
  .toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .toolbar-left h2 {
    font-size: var(--text-lg);
  }

  .section-card {
    margin-bottom: var(--space-4);
  }

  .card-header {
    font-size: var(--text-base);
  }
}
</style>
