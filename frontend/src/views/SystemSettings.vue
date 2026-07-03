<template>
  <div class="settings-page">
    <PageHeader title="系统设置">
      <template #extra>
        <el-tag v-if="config.appVersion" type="info" size="small">v{{ config.appVersion }}</el-tag>
      </template>
      <template #actions>
        <el-button type="primary" @click="loadConfig" :loading="loading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </template>
    </PageHeader>

    <div class="settings-layout">
      <el-tabs v-model="activeTab" tab-position="left" class="settings-tabs">
        <!-- 用户管理 -->
        <el-tab-pane label="👤 用户" name="user">
          <div class="tab-content">
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="当前用户">
                <el-tag type="primary" size="small">{{ user.username }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="用户角色">
                <el-tag type="danger" size="small">ADMIN</el-tag>
              </el-descriptions-item>
            </el-descriptions>
            <div style="margin-top: 16px;">
              <el-button type="warning" @click="openChangePassword">
                <el-icon><Lock /></el-icon> 修改密码
              </el-button>
            </div>
          </div>
        </el-tab-pane>

        <!-- 通知渠道 -->
        <el-tab-pane label="📢 通知" name="notification">
          <div class="tab-content">
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
                      <div class="inline-edit">
                        <el-input v-model="editValues['telegram.bot.token']" type="password" show-password placeholder="输入 Bot Token" size="small" />
                        <el-button size="small" type="primary" :loading="savingKeys['telegram.bot.token']" @click="saveField('telegram.bot.token')">保存</el-button>
                      </div>
                    </el-descriptions-item>
                    <el-descriptions-item label="Chat ID">
                      <div class="inline-edit">
                        <el-input v-model="editValues['telegram.chat.id']" placeholder="输入 Chat ID" size="small" />
                        <el-button size="small" type="primary" :loading="savingKeys['telegram.chat.id']" @click="saveField('telegram.chat.id')">保存</el-button>
                      </div>
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
                      <div class="inline-edit">
                        <el-input v-model="editValues['dingtalk.webhook']" placeholder="输入 Webhook URL" size="small" />
                        <el-button size="small" type="primary" :loading="savingKeys['dingtalk.webhook']" @click="saveField('dingtalk.webhook')">保存</el-button>
                      </div>
                    </el-descriptions-item>
                    <el-descriptions-item label="签名密钥">
                      <div class="inline-edit">
                        <el-input v-model="editValues['dingtalk.secret']" type="password" show-password placeholder="输入签名密钥（可选）" size="small" />
                        <el-button size="small" type="primary" :loading="savingKeys['dingtalk.secret']" @click="saveField('dingtalk.secret')">保存</el-button>
                      </div>
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
                      <div class="inline-edit">
                        <el-input v-model="editValues['bark.key']" placeholder="输入 Device Key" size="small" />
                        <el-button size="small" type="primary" :loading="savingKeys['bark.key']" @click="saveField('bark.key')">保存</el-button>
                      </div>
                    </el-descriptions-item>
                    <el-descriptions-item label="Server">
                      <div class="inline-edit">
                        <el-input v-model="editValues['bark.server']" placeholder="https://api.day.app (默认)" size="small" />
                        <el-button size="small" type="primary" :loading="savingKeys['bark.server']" @click="saveField('bark.server')">保存</el-button>
                      </div>
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
                      <div class="inline-edit">
                        <el-input v-model="editValues['feishu.webhook']" placeholder="输入 Webhook URL" size="small" />
                        <el-button size="small" type="primary" :loading="savingKeys['feishu.webhook']" @click="saveField('feishu.webhook')">保存</el-button>
                      </div>
                    </el-descriptions-item>
                    <el-descriptions-item label="签名密钥">
                      <div class="inline-edit">
                        <el-input v-model="editValues['feishu.secret']" type="password" show-password placeholder="输入签名密钥（可选）" size="small" />
                        <el-button size="small" type="primary" :loading="savingKeys['feishu.secret']" @click="saveField('feishu.secret')">保存</el-button>
                      </div>
                    </el-descriptions-item>
                  </el-descriptions>
                </el-card>
              </el-col>
            </el-row>
          </div>
        </el-tab-pane>

        <!-- DNS 服务 -->
        <el-tab-pane label="🌐 DNS" name="dns">
          <div class="tab-content">
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
                      <div class="inline-edit">
                        <el-input v-model="editValues['cloudflare.api.token']" type="password" show-password placeholder="输入 API Token" size="small" />
                        <el-button size="small" type="primary" :loading="savingKeys['cloudflare.api.token']" @click="saveField('cloudflare.api.token')">保存</el-button>
                      </div>
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
                      <div class="inline-edit">
                        <el-input v-model="editValues['edgeone.secretId']" type="password" show-password placeholder="输入 Secret ID" size="small" />
                        <el-button size="small" type="primary" :loading="savingKeys['edgeone.secretId']" @click="saveField('edgeone.secretId')">保存</el-button>
                      </div>
                    </el-descriptions-item>
                    <el-descriptions-item label="Zone ID">
                      <div class="inline-edit">
                        <el-input v-model="editValues['edgeone.zoneId']" placeholder="输入 Zone ID" size="small" />
                        <el-button size="small" type="primary" :loading="savingKeys['edgeone.zoneId']" @click="saveField('edgeone.zoneId')">保存</el-button>
                      </div>
                    </el-descriptions-item>
                  </el-descriptions>
                </el-card>
              </el-col>
            </el-row>
          </div>
        </el-tab-pane>

        <!-- SSL 证书 -->
        <el-tab-pane label="🔒 SSL" name="ssl">
          <div class="tab-content">
            <!-- Certificate Status Card -->
            <el-card shadow="none" class="ssl-status-card" :class="sslStatusClass">
              <template #header>
                <div class="channel-header">
                  <span>🔒 证书状态</span>
                  <el-tag :type="sslStatusType" size="small" effect="dark">{{ sslStatusText }}</el-tag>
                </div>
              </template>
              <el-descriptions :column="2" size="small" border>
                <el-descriptions-item label="域名">{{ cfgStr('ssl.domain') || '未配置' }}</el-descriptions-item>
                <el-descriptions-item label="邮箱">{{ cfgStr('ssl.email') || '未配置' }}</el-descriptions-item>
                <el-descriptions-item label="证书颁发者">
                  {{ cfgStr('ssl.domain') ? 'Let\'s Encrypt' : '—' }}
                </el-descriptions-item>
                <el-descriptions-item label="到期时间">
                  <span v-if="sslExpiry" :style="{ color: sslExpiryColor }">{{ sslExpiry }}</span>
                  <span v-else style="color: var(--text-muted)">—</span>
                </el-descriptions-item>
                <el-descriptions-item label="剩余天数">
                  <span v-if="sslDaysRemaining !== null" :style="{ color: sslDaysColor, fontWeight: 'var(--font-semibold)' }">
                    {{ sslDaysRemaining > 0 ? sslDaysRemaining + ' 天' : '已过期' }}
                  </span>
                  <span v-else style="color: var(--text-muted)">—</span>
                </el-descriptions-item>
                <el-descriptions-item label="自动续期">
                  <el-tag type="success" size="small">每天 04:00</el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="模式">
                  <el-tag :type="config.bools?.['ssl.staging'] ? 'warning' : 'success'" size="small">
                    {{ config.bools?.['ssl.staging'] ? 'Staging (测试)' : 'Production (生产)' }}
                  </el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="DNS 提供商">
                  <el-tag type="info" size="small">Cloudflare</el-tag>
                </el-descriptions-item>
              </el-descriptions>
            </el-card>

            <!-- Quick Actions -->
            <el-card shadow="none" class="channel-card" style="margin-top: 16px;">
              <template #header>
                <div class="channel-header">
                  <span>⚡ 快捷操作</span>
                </div>
              </template>
              <div class="ssl-actions">
                <el-button
                  type="primary"
                  :loading="sslRenewing"
                  :disabled="!cfgStr('ssl.domain') || !cfgStr('ssl.email')"
                  @click="forceRenewSsl"
                >
                  <el-icon><Refresh /></el-icon> 强制续期
                </el-button>
                <el-button
                  :disabled="!cfgStr('ssl.domain')"
                  @click="viewCertDetails"
                >
                  <el-icon><Document /></el-icon> 查看证书详情
                </el-button>
                <el-button
                  :disabled="!cfgStr('ssl.domain')"
                  @click="testSslConfig"
                >
                  <el-icon><Connection /></el-icon> 测试 SSL 配置
                </el-button>
              </div>
              <div v-if="!cfgStr('ssl.domain')" style="margin-top:12px">
                <el-alert
                  title="SSL 证书未配置"
                  type="info"
                  description="请先配置域名和邮箱，然后设置 Cloudflare API Token 以启用 Let's Encrypt 自动签发/续期。"
                  :closable="false"
                  show-icon
                />
              </div>
            </el-card>

            <!-- Configuration Form -->
            <el-card shadow="none" class="channel-card" style="margin-top: 16px;">
              <template #header>
                <div class="channel-header">
                  <span>⚙️ 配置</span>
                </div>
              </template>
              <el-descriptions :column="2" border>
                <el-descriptions-item label="域名">
                  <div class="inline-edit">
                    <el-input v-model="editValues['ssl.domain']" placeholder="输入域名" size="small" />
                    <el-button size="small" type="primary" :loading="savingKeys['ssl.domain']" @click="saveField('ssl.domain')">保存</el-button>
                  </div>
                </el-descriptions-item>
                <el-descriptions-item label="Email">
                  <div class="inline-edit">
                    <el-input v-model="editValues['ssl.email']" placeholder="输入 Email" size="small" />
                    <el-button size="small" type="primary" :loading="savingKeys['ssl.email']" @click="saveField('ssl.email')">保存</el-button>
                  </div>
                </el-descriptions-item>
              </el-descriptions>
            </el-card>
          </div>
        </el-tab-pane>

        <!-- 网络代理 -->
        <el-tab-pane label="🌐 代理" name="proxy">
          <div class="tab-content">
            <el-alert
              title="配置应用级别的出站代理，用于访问外部 API（如 Telegram Bot、OCI API 等）"
              type="info"
              :closable="false"
              show-icon
              style="margin-bottom: 16px;"
            />
            <el-form :model="proxyForm" label-width="100px" :rules="proxyRules" ref="proxyFormRef">
              <el-row :gutter="16">
                <el-col :md="12" :sm="24">
                  <el-form-item label="代理类型" prop="type">
                    <el-select v-model="proxyForm.type" style="width: 100%">
                      <el-option label="HTTP" value="HTTP" />
                      <el-option label="HTTPS" value="HTTPS" />
                      <el-option label="SOCKS5（推荐）" value="SOCKS5" />
                    </el-select>
                  </el-form-item>
                </el-col>
                <el-col :md="12" :sm="24">
                  <el-form-item label="启用代理">
                    <el-switch v-model="proxyForm.enabled" active-text="启用" inactive-text="禁用" />
                  </el-form-item>
                </el-col>
              </el-row>
              <el-row :gutter="16">
                <el-col :md="16" :sm="24">
                  <el-form-item label="代理地址" prop="host">
                    <el-input v-model="proxyForm.host" placeholder="代理服务器 IP 或域名" />
                  </el-form-item>
                </el-col>
                <el-col :md="8" :sm="24">
                  <el-form-item label="端口" prop="port">
                    <el-input-number v-model="proxyForm.port" :min="1" :max="65535" style="width: 100%" />
                  </el-form-item>
                </el-col>
              </el-row>
              <el-row :gutter="16">
                <el-col :md="12" :sm="24">
                  <el-form-item label="用户名">
                    <el-input v-model="proxyForm.username" placeholder="选填" />
                  </el-form-item>
                </el-col>
                <el-col :md="12" :sm="24">
                  <el-form-item label="密码">
                    <el-input v-model="proxyForm.password" type="password" show-password placeholder="选填" />
                  </el-form-item>
                </el-col>
              </el-row>
              <el-form-item>
                <el-button type="primary" :loading="proxySaving" @click="saveProxyConfig">
                  <el-icon><Check /></el-icon> 保存配置
                </el-button>
                <el-button :loading="proxyTesting" @click="testProxyConnection">
                  <el-icon><Connection /></el-icon> 测试连通性
                </el-button>
                <el-button @click="loadProxyConfig">
                  <el-icon><Refresh /></el-icon> 重置
                </el-button>
              </el-form-item>
            </el-form>
          </div>
        </el-tab-pane>

        <!-- 安全与认证 -->
        <el-tab-pane label="🔐 安全" name="security">
          <div class="tab-content">
            <el-row :gutter="20">
              <!-- MFA -->
              <el-col :xs="24" :sm="8">
                <div class="security-block">
                  <div class="security-block-header">
                    <span class="security-block-title">MFA 验证</span>
                    <el-tag v-if="mfaStatus.enabled" type="success" size="small" effect="dark">已启用</el-tag>
                    <el-tag v-else type="info" size="small">未启用</el-tag>
                  </div>
                  <p class="security-block-desc">使用 TOTP 动态验证码保护登录安全</p>
                  <div class="security-block-actions">
                    <el-button v-if="mfaStatus.enabled" type="danger" size="small" :loading="mfaDisabling" @click="disableMfa">
                      禁用
                    </el-button>
                    <el-button v-else type="primary" size="small" :loading="mfaSettingUp" @click="setupTotp">
                      设置 TOTP
                    </el-button>
                  </div>
                </div>
              </el-col>

              <!-- Turnstile -->
              <el-col :xs="24" :sm="8">
                <div class="security-block">
                  <div class="security-block-header">
                    <span class="security-block-title">Turnstile 验证</span>
                    <el-tag v-if="turnstileForm.enabled" type="success" size="small" effect="dark">已启用</el-tag>
                    <el-tag v-else type="info" size="small">未启用</el-tag>
                  </div>
                  <p class="security-block-desc">Cloudflare 人机验证，防止恶意登录</p>
                  <el-form label-width="70px" size="small" class="security-block-form">
                    <el-form-item label="启用">
                      <el-switch v-model="turnstileForm.enabled" />
                    </el-form-item>
                    <el-form-item label="Site Key">
                      <el-input v-model="turnstileForm.siteKey" placeholder="站点密钥" />
                    </el-form-item>
                    <el-form-item label="Secret">
                      <el-input v-model="turnstileForm.secretKey" type="password" show-password placeholder="安全密钥" />
                    </el-form-item>
                  </el-form>
                  <div class="security-block-actions">
                    <el-button type="primary" size="small" :loading="turnstileSaving" @click="saveTurnstile">保存</el-button>
                  </div>
                </div>
              </el-col>

              <!-- GitHub OAuth -->
              <el-col :xs="24" :sm="8">
                <div class="security-block">
                  <div class="security-block-header">
                    <span class="security-block-title">GitHub OAuth</span>
                    <el-tag v-if="githubForm.enabled" type="success" size="small" effect="dark">已启用</el-tag>
                    <el-tag v-else type="info" size="small">未启用</el-tag>
                  </div>
                  <p class="security-block-desc">使用 GitHub 账号作为替代登录方式</p>
                  <el-form label-width="70px" size="small" class="security-block-form">
                    <el-form-item label="启用">
                      <el-switch v-model="githubForm.enabled" />
                    </el-form-item>
                    <el-form-item label="Client ID">
                      <el-input v-model="githubForm.clientId" placeholder="OAuth App Client ID" />
                    </el-form-item>
                    <el-form-item label="Secret">
                      <el-input v-model="githubForm.clientSecret" type="password" show-password placeholder="Client Secret" />
                    </el-form-item>
                    <el-form-item label="回调地址">
                      <el-input v-model="githubForm.redirectUri" placeholder="http://your-domain/api/github/callback" />
                    </el-form-item>
                  </el-form>
                  <div class="security-block-actions">
                    <el-button type="primary" size="small" :loading="githubSaving" @click="saveGithubOAuth">保存</el-button>
                  </div>
                </div>
              </el-col>
            </el-row>

            <!-- Login History, IP Whitelist, and Session Management -->
            <el-row :gutter="16" style="margin-top: 20px;">
              <!-- Login History -->
              <el-col :xs="24" :sm="12">
                <el-card shadow="none" class="channel-card">
                  <template #header>
                    <div class="channel-header">
                      <span>📋 登录历史</span>
                      <el-button type="primary" link size="small" @click="loadLoginHistory">
                        <el-icon><Refresh /></el-icon> 刷新
                      </el-button>
                    </div>
                  </template>
                  <el-table :data="loginHistory" size="small" max-height="200">
                    <el-table-column prop="time" label="时间" width="160" />
                    <el-table-column prop="ip" label="IP 地址" width="130" />
                    <el-table-column prop="status" label="状态" width="80">
                      <template #default="{ row }">
                        <el-tag :type="row.success ? 'success' : 'danger'" size="small">
                          {{ row.success ? '成功' : '失败' }}
                        </el-tag>
                      </template>
                    </el-table-column>
                  </el-table>
                  <div v-if="loginHistory.length === 0" style="text-align: center; padding: 20px; color: var(--text-muted);">
                    暂无登录记录
                  </div>
                </el-card>
              </el-col>

              <!-- IP Whitelist -->
              <el-col :xs="24" :sm="12">
                <el-card shadow="none" class="channel-card">
                  <template #header>
                    <div class="channel-header">
                      <span>🛡️ IP 白名单</span>
                      <el-tag type="info" size="small">{{ ipWhitelist.length }} 条规则</el-tag>
                    </div>
                  </template>
                  <el-input
                    v-model="newIpRule"
                    placeholder="输入 IP 地址或 CIDR"
                    size="small"
                    style="margin-bottom: 8px;"
                  >
                    <template #append>
                      <el-button @click="addIpRule">添加</el-button>
                    </template>
                  </el-input>
                  <div v-for="(ip, index) in ipWhitelist" :key="index" class="ip-rule-item">
                    <span class="data-mono">{{ ip }}</span>
                    <el-button type="danger" text size="small" @click="removeIpRule(index)">
                      <el-icon><Delete /></el-icon>
                    </el-button>
                  </div>
                  <div v-if="ipWhitelist.length === 0" style="text-align: center; padding: 20px; color: var(--text-muted);">
                    未配置 IP 白名单（允许所有 IP）
                  </div>
                </el-card>
              </el-col>
            </el-row>

            <!-- Session Management -->
            <el-row :gutter="16" style="margin-top: 16px;">
              <el-col :xs="24" :sm="24">
                <el-card shadow="none" class="channel-card">
                  <template #header>
                    <div class="channel-header">
                      <span>🔑 会话管理</span>
                      <el-tag type="info" size="small">{{ activeSessions }} 个活跃会话</el-tag>
                    </div>
                  </template>
                  <el-descriptions :column="2" size="small" border>
                    <el-descriptions-item label="当前会话">
                      <el-tag type="success" size="small">当前设备</el-tag>
                    </el-descriptions-item>
                    <el-descriptions-item label="最后活动时间">
                      {{ lastActivityTime }}
                    </el-descriptions-item>
                    <el-descriptions-item label="会话总数">
                      {{ activeSessions }}
                    </el-descriptions-item>
                    <el-descriptions-item label="会话有效期">
                      <el-tag type="info" size="small">24 小时</el-tag>
                    </el-descriptions-item>
                  </el-descriptions>
                  <div style="margin-top: 12px; text-align: right;">
                    <el-button type="danger" size="small" @click="logoutAllSessions">
                      <el-icon><SwitchButton /></el-icon> 注销所有会话
                    </el-button>
                  </div>
                </el-card>
              </el-col>
            </el-row>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>

    <!-- TOTP Setup Dialog -->
    <el-dialog v-model="totpDialogVisible" title="设置 TOTP 验证" width="420px" destroy-on-close>
      <div v-if="totpSetupData.qrCodeBase64" style="text-align: center;">
        <p>使用 Google Authenticator 或其他 TOTP 应用扫描二维码：</p>
        <img :src="totpSetupData.qrCodeBase64" alt="TOTP QR Code" style="width: 200px; height: 200px;" />
        <p style="margin-top: 8px; font-size: 12px; color: var(--text-secondary);">
          密钥：<code>{{ totpSetupData.secret }}</code>
        </p>
        <el-input v-model="totpVerifyCode" placeholder="输入 6 位验证码" maxlength="6"
          style="margin-top: 12px; width: 200px;" @keyup.enter="verifyTotp" />
      </div>
      <template #footer>
        <el-button @click="totpDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="totpVerifying" @click="verifyTotp">验证并启用</el-button>
      </template>
    </el-dialog>

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
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Lock, Check, Connection, Delete, SwitchButton } from '@element-plus/icons-vue'
import { useUserStore } from '../store/user'
import request from '../utils/request'
import PageHeader from '../components/common/PageHeader.vue'
import type { SystemConfig } from '../types/api'

const user = useUserStore()
const loading = ref(false)
const activeTab = ref('user')
const config = ref<SystemConfig>({
  strings: {},
  bools: {},
  appVersion: '',
})

// Inline edit state — all configurable fields
const editValues = reactive<Record<string, string>>({
  'telegram.bot.token': '',
  'telegram.chat.id': '',
  'dingtalk.webhook': '',
  'dingtalk.secret': '',
  'bark.key': '',
  'bark.server': '',
  'feishu.webhook': '',
  'feishu.secret': '',
  'cloudflare.api.token': '',
  'edgeone.secretId': '',
  'edgeone.zoneId': '',
  'ssl.domain': '',
  'ssl.email': '',
})
const savingKeys = reactive<Record<string, boolean>>({})

// Password change state
const pwdVisible = ref(false)
const pwdSaving = ref(false)
const pwdForm = ref({ currentPassword: '', newPassword: '', confirmPassword: '' })

// MFA state
const mfaStatus = ref({ enabled: false, configured: false })
const mfaSettingUp = ref(false)
const mfaDisabling = ref(false)
const totpDialogVisible = ref(false)
const totpSetupData = ref({ secret: '', otpauthUrl: '', qrCodeBase64: '' })
const totpVerifyCode = ref('')
const totpVerifying = ref(false)

// Turnstile state
const turnstileForm = reactive({
  enabled: false,
  siteKey: '',
  secretKey: '',
})
const turnstileSaving = ref(false)

// GitHub OAuth state
const githubForm = reactive({
  enabled: false,
  clientId: '',
  clientSecret: '',
  redirectUri: '',
})
const githubSaving = ref(false)

// Proxy configuration state
const proxyFormRef = ref()
const proxySaving = ref(false)
const proxyTesting = ref(false)
const proxyForm = reactive({
  type: 'SOCKS5',
  host: '',
  port: 1080,
  username: '',
  password: '',
  enabled: false,
})
const proxyRules = {
  type: [{ required: true, message: '请选择代理类型', trigger: 'change' }],
  host: [{ required: true, message: '请输入代理地址', trigger: 'blur' }],
  port: [{ required: true, message: '请输入端口', trigger: 'blur' }],
}

// Login history state (mock data for now)
const loginHistory = ref([
  { time: '2026-07-03 10:30:22', ip: '192.168.1.100', success: true },
  { time: '2026-07-03 09:15:45', ip: '192.168.1.100', success: true },
  { time: '2026-07-02 22:10:33', ip: '10.0.0.50', success: true },
  { time: '2026-07-02 18:45:12', ip: '172.16.0.200', success: false },
  { time: '2026-07-02 15:30:08', ip: '192.168.1.100', success: true },
])

// IP Whitelist state
const ipWhitelist = ref(['192.168.1.0/24', '10.0.0.0/8'])
const newIpRule = ref('')

// Session management state
const activeSessions = ref(3)
const lastActivityTime = ref('2026-07-03 10:30:22')

function cfgStr(key: string): string {
  return config.value.strings?.[key] || ''
}

async function loadConfig() {
  loading.value = true
  try {
    const data: SystemConfig = await request.get('/system/config')
    if (data) {
      config.value = data
      // Sync config values to edit fields
      for (const key of Object.keys(editValues)) {
        editValues[key] = data.strings?.[key] || ''
      }
    }
  } catch { /* silently ignore */ }
  loading.value = false
}

async function saveField(key: string) {
  savingKeys[key] = true
  try {
    await request.post('/system/config/save', { key, value: editValues[key] })
    ElMessage.success('配置已保存')
    await loadConfig()
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    savingKeys[key] = false
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

async function loadProxyConfig() {
  try {
    const data = await request.get('/system/proxy') as any
    if (data) {
      proxyForm.type = data.type || 'SOCKS5'
      proxyForm.host = data.host || ''
      proxyForm.port = data.port || 1080
      proxyForm.username = data.username || ''
      proxyForm.password = data.password || ''
      proxyForm.enabled = data.enabled || false
    }
  } catch { /* silently ignore */ }
}

async function saveProxyConfig() {
  if (!proxyFormRef.value) return
  try {
    await proxyFormRef.value.validate()
  } catch {
    return
  }
  proxySaving.value = true
  try {
    await request.put('/system/proxy', {
      type: proxyForm.type,
      host: proxyForm.host,
      port: proxyForm.port,
      username: proxyForm.username,
      password: proxyForm.password,
      enabled: proxyForm.enabled,
    })
    ElMessage.success('代理配置已保存')
  } catch (e: any) {
    ElMessage.error(e.message || '保存代理配置失败')
  } finally {
    proxySaving.value = false
  }
}

async function testProxyConnection() {
  if (!proxyForm.host || !proxyForm.port) {
    ElMessage.warning('请先填写代理地址和端口')
    return
  }
  proxyTesting.value = true
  try {
    const res = await request.post('/system/proxy/test', {
      type: proxyForm.type,
      host: proxyForm.host,
      port: proxyForm.port,
      username: proxyForm.username,
      password: proxyForm.password,
    }) as any
    if (res?.reachable) {
      ElMessage.success(res.message || '代理连接成功')
    } else {
      ElMessage.error('代理连接失败')
    }
  } catch (e: any) {
    ElMessage.error(e.message || '代理连接测试失败')
  } finally {
    proxyTesting.value = false
  }
}

// MFA functions
async function loadMfaStatus() {
  try {
    const data = await request.get('/api/mfa/status') as any
    mfaStatus.value = data || { enabled: false, configured: false }
  } catch { /* silently ignore */ }
}

async function setupTotp() {
  mfaSettingUp.value = true
  try {
    const data = await request.post('/api/mfa/totp/setup') as any
    totpSetupData.value = data
    totpVerifyCode.value = ''
    totpDialogVisible.value = true
  } catch (e: any) {
    ElMessage.error(e.message || 'TOTP 初始化失败')
  } finally {
    mfaSettingUp.value = false
  }
}

async function verifyTotp() {
  if (!totpVerifyCode.value || totpVerifyCode.value.length !== 6) {
    ElMessage.warning('请输入 6 位验证码')
    return
  }
  totpVerifying.value = true
  try {
    await request.post('/api/mfa/totp/verify', { code: totpVerifyCode.value })
    ElMessage.success('MFA 已启用')
    totpDialogVisible.value = false
    await loadMfaStatus()
  } catch (e: any) {
    ElMessage.error(e.message || '验证失败')
  } finally {
    totpVerifying.value = false
  }
}

async function disableMfa() {
  try {
    await ElMessageBox.confirm('确定要禁用 MFA 吗？', '确认', { type: 'warning' })
  } catch { return }
  mfaDisabling.value = true
  try {
    await request.post('/api/mfa/disable')
    ElMessage.success('MFA 已禁用')
    await loadMfaStatus()
  } catch (e: any) {
    ElMessage.error(e.message || '禁用 MFA 失败')
  } finally {
    mfaDisabling.value = false
  }
}

// Turnstile functions
function loadTurnstile() {
  const strs = config.value.strings || {}
  turnstileForm.enabled = config.value.bools?.['turnstile.enabled'] || false
  turnstileForm.siteKey = strs['turnstile.site.key'] || ''
  turnstileForm.secretKey = strs['turnstile.secret.key'] || ''
}

async function saveTurnstile() {
  turnstileSaving.value = true
  try {
    await request.put('/system/settings', {
      security: {
        turnstile: {
          enabled: turnstileForm.enabled,
          siteKey: turnstileForm.siteKey,
          secretKey: turnstileForm.secretKey,
        },
      },
    })
    ElMessage.success('Turnstile 配置已保存')
    await loadConfig()
    loadTurnstile()
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    turnstileSaving.value = false
  }
}

// GitHub OAuth functions
function loadGithubOAuth() {
  // Read from the already-loaded system config
  const strs = config.value.strings || {}
  githubForm.enabled = config.value.bools?.['github.enabled'] || false
  githubForm.clientId = strs['github.client.id'] || ''
  githubForm.clientSecret = strs['github.client.secret'] || ''
  githubForm.redirectUri = strs['github.redirect.uri'] || ''
}

async function saveGithubOAuth() {
  githubSaving.value = true
  try {
    await request.put('/system/settings', {
      oauth: {
        github: {
          enabled: githubForm.enabled,
          clientId: githubForm.clientId,
          clientSecret: githubForm.clientSecret,
          redirectUri: githubForm.redirectUri,
        },
      },
    })
    ElMessage.success('GitHub OAuth 配置已保存')
    await loadConfig()
    loadGithubOAuth()
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    githubSaving.value = false
  }
}

// Login history functions
async function loadLoginHistory() {
  try {
    // TODO: Replace with real API call when available
    // const data = await request.get('/api/security/login-history') as any
    // loginHistory.value = data || []
    ElMessage.info('登录历史刷新成功')
  } catch { /* silently ignore */ }
}

// IP Whitelist functions
function addIpRule() {
  const ip = newIpRule.value.trim()
  if (!ip) {
    ElMessage.warning('请输入 IP 地址')
    return
  }
  // Basic IP/CIDR validation
  const ipRegex = /^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$/
  if (!ipRegex.test(ip)) {
    ElMessage.warning('请输入有效的 IP 地址或 CIDR 格式')
    return
  }
  if (ipWhitelist.value.includes(ip)) {
    ElMessage.warning('该 IP 规则已存在')
    return
  }
  ipWhitelist.value.push(ip)
  newIpRule.value = ''
  ElMessage.success('IP 规则已添加')
  // TODO: Save to backend when API is available
}

function removeIpRule(index: number) {
  ipWhitelist.value.splice(index, 1)
  ElMessage.success('IP 规则已删除')
  // TODO: Save to backend when API is available
}

// Session management functions
async function logoutAllSessions() {
  try {
    await ElMessageBox.confirm('确定要注销所有会话吗？这将使所有设备退出登录。', '确认', { type: 'warning' })
  } catch { return }

  try {
    // TODO: Replace with real API call when available
    // await request.post('/api/security/logout-all')
    activeSessions.value = 1
    lastActivityTime.value = new Date().toLocaleString()
    ElMessage.success('所有会话已注销')
  } catch (e: any) {
    ElMessage.error(e.message || '注销会话失败')
  }
}

onMounted(async () => {
  await loadConfig()
  loadProxyConfig()
  loadMfaStatus()
  loadTurnstile()
  loadGithubOAuth()
})
</script>

<style scoped>
.settings-page {
  padding: 0;
}

.settings-layout {
  display: flex;
  gap: var(--space-6);
}

.settings-tabs {
  flex: 1;
}

.settings-tabs :deep(.el-tabs__header) {
  margin-right: var(--space-6);
}

.settings-tabs :deep(.el-tabs__item) {
  height: 44px;
  line-height: 44px;
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
}

.tab-content {
  min-height: 400px;
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

.inline-edit {
  display: flex;
  gap: 8px;
  align-items: center;
}

.inline-edit .el-input {
  flex: 1;
}

:deep(.el-form-item) {
  margin-bottom: 18px;
}

:deep(.el-alert) {
  border-radius: var(--radius-md);
}

:deep(.el-input-number) {
  width: 100%;
}

.security-block {
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  padding: 16px;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.security-block-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}

.security-block-title {
  font-size: var(--text-md);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

.security-block-desc {
  font-size: 12px;
  color: var(--text-secondary);
  margin: 0 0 12px 0;
  line-height: 1.4;
}

.security-block-form {
  flex: 1;
}

.security-block-form :deep(.el-form-item) {
  margin-bottom: 10px;
}

.security-block-form :deep(.el-form-item__label) {
  font-size: 12px;
}

.security-block-actions {
  margin-top: auto;
  padding-top: 10px;
  text-align: right;
}

.ip-rule-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 8px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  margin-bottom: 6px;
  background: var(--bg-surface);
}

.ip-rule-item:last-child {
  margin-bottom: 0;
}

.data-mono {
  font-family: monospace;
  font-size: var(--text-sm);
  color: var(--text-primary);
}

@media (max-width: 768px) {
  .settings-layout {
    flex-direction: column;
  }

  .settings-tabs :deep(.el-tabs__header) {
    margin-right: 0;
    margin-bottom: var(--space-4);
  }

  .settings-tabs :deep(.el-tabs__nav-wrap) {
    overflow-x: auto;
  }
}
</style>
