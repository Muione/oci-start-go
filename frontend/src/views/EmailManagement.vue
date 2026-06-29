<template>
  <div class="email-page">
    <!-- ================================================================ -->
    <!-- Page Header -->
    <!-- ================================================================ -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>邮件管理</h2>
      </div>
      <div class="toolbar-right">
        <el-button @click="refreshCurrentTab" :loading="tabLoading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </div>
    </div>

    <!-- ================================================================ -->
    <!-- Tabs -->
    <!-- ================================================================ -->
    <el-tabs v-model="activeTab" @tab-change="onTabChange">
      <!-- ============================================================== -->
      <!-- Tab 1: Recipient Management -->
      <!-- ============================================================== -->
      <el-tab-pane label="收件人管理" name="recipients">
        <div class="tab-toolbar">
          <el-input
            v-model="recipientSearch"
            placeholder="搜索邮箱或姓名..."
            size="small"
            clearable
            style="width: 240px"
            :prefix-icon="Search"
          />
          <el-button type="primary" size="small" @click="openAddRecipient">
            <el-icon><Plus /></el-icon> 添加收件人
          </el-button>
        </div>

        <el-card shadow="none" class="table-card">
          <el-table :data="filteredRecipients" v-loading="recipientLoading" border stripe style="width: 100%">
            <template #empty>
              <el-empty description="暂无收件人" :image-size="80">
                <el-button type="primary" @click="openAddRecipient">添加收件人</el-button>
              </el-empty>
            </template>
            <el-table-column type="index" label="#" width="50" align="center" />
            <el-table-column prop="name" label="姓名" min-width="120" show-overflow-tooltip />
            <el-table-column prop="email" label="邮箱地址" min-width="200" show-overflow-tooltip />
            <el-table-column label="创建时间" width="170">
              <template #default="{ row }">
                <span class="data-mono" style="font-size:12px">{{ row.createTime || '—' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="80" align="center" fixed="right">
              <template #default="{ row }">
                <el-button size="small" type="danger" @click="deleteRecipient(row)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- ============================================================== -->
      <!-- Tab 2: Email Sending (Compose) -->
      <!-- ============================================================== -->
      <el-tab-pane label="发送邮件" name="compose">
        <el-card shadow="none" class="compose-card">
          <el-form :model="composeForm" label-width="100px" :rules="composeRules" ref="composeFormRef">
            <el-form-item label="邮件配置" prop="tenantEmailConfigId" required>
              <el-select
                v-model="composeForm.tenantEmailConfigId"
                placeholder="选择发件配置"
                style="width: 100%"
                filterable
              >
                <el-option
                  v-for="cfg in tenantConfigs"
                  :key="cfg.id"
                  :label="`${cfg.tenantName || '租户#' + cfg.tenantId} — ${cfg.senderEmail} (${cfg.domainName})`"
                  :value="cfg.id"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="邮件主题" prop="title" required>
              <el-input v-model="composeForm.title" placeholder="输入邮件主题" maxlength="200" show-word-limit />
            </el-form-item>
            <el-form-item label="收件人" prop="emailReceiveIds" required>
              <el-transfer
                v-model="composeForm.emailReceiveIds"
                :data="recipientTransferData"
                :titles="['可选收件人', '已选收件人']"
                filterable
                filter-placeholder="搜索邮箱"
                style="width: 100%"
              />
            </el-form-item>
            <el-form-item label="邮件正文" prop="content" required>
              <el-input
                v-model="composeForm.content"
                type="textarea"
                :rows="12"
                placeholder="输入邮件正文内容..."
                maxlength="10000"
                show-word-limit
              />
            </el-form-item>
            <el-form-item>
              <el-button
                type="primary"
                :loading="sending"
                @click="sendEmail"
                :disabled="composeForm.emailReceiveIds.length === 0"
              >
                <el-icon><Promotion /></el-icon>
                发送邮件 ({{ composeForm.emailReceiveIds.length }} 位收件人)
              </el-button>
              <el-button @click="resetComposeForm">重置</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <!-- Send Result -->
        <el-card v-if="sendResults.length > 0" shadow="none" class="result-card" style="margin-top: 16px">
          <template #header>
            <div style="display:flex;align-items:center;justify-content:space-between">
              <span>发送结果</span>
              <div style="display:flex;gap:16px">
                <el-tag type="success" size="small">成功: {{ sendResults.filter(r => r.success).length }}</el-tag>
                <el-tag type="danger" size="small">失败: {{ sendResults.filter(r => !r.success).length }}</el-tag>
              </div>
            </div>
          </template>
          <el-table :data="sendResults" border size="small" max-height="300">
            <el-table-column prop="email" label="收件人" min-width="200" />
            <el-table-column label="状态" width="80" align="center">
              <template #default="{ row }">
                <el-tag :type="row.success ? 'success' : 'danger'" size="small">
                  {{ row.success ? '成功' : '失败' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="message" label="信息" min-width="200" show-overflow-tooltip />
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- ============================================================== -->
      <!-- Tab 3: Email History -->
      <!-- ============================================================== -->
      <el-tab-pane label="发送记录" name="history">
        <div class="tab-toolbar">
          <el-select v-model="historyConfigFilter" placeholder="按配置筛选" size="small" clearable style="width: 200px">
            <el-option
              v-for="cfg in tenantConfigs"
              :key="cfg.id"
              :label="`${cfg.tenantName || '租户#' + cfg.tenantId} — ${cfg.senderEmail}`"
              :value="cfg.id"
            />
          </el-select>
          <el-button type="danger" size="small" @click="batchDeleteBodies" :disabled="selectedBodies.length === 0">
            <el-icon><Delete /></el-icon> 批量删除 ({{ selectedBodies.length }})
          </el-button>
        </div>

        <el-card shadow="none" class="table-card">
          <el-table
            :data="emailBodies"
            v-loading="bodyLoading"
            border
            stripe
            style="width: 100%"
            @selection-change="onBodySelectionChange"
          >
            <template #empty>
              <el-empty description="暂无发送记录" :image-size="80" />
            </template>
            <el-table-column type="selection" width="45" />
            <el-table-column prop="title" label="邮件主题" min-width="200" show-overflow-tooltip />
            <el-table-column prop="senderEmail" label="发件邮箱" width="180" show-overflow-tooltip />
            <el-table-column label="收件人数" width="90" align="center">
              <template #default="{ row }">
                <span class="data-mono">{{ row.receiveTotal || 0 }}</span>
              </template>
            </el-table-column>
            <el-table-column label="成功" width="70" align="center">
              <template #default="{ row }">
                <span style="color:var(--status-up);font-weight:var(--font-semibold)">{{ row.receiveSuccessTotal || 0 }}</span>
              </template>
            </el-table-column>
            <el-table-column label="失败" width="70" align="center">
              <template #default="{ row }">
                <span :style="{ color: row.receiveFailTotal > 0 ? 'var(--status-down)' : 'var(--text-secondary)', fontWeight: 'var(--font-semibold)' }">
                  {{ row.receiveFailTotal || 0 }}
                </span>
              </template>
            </el-table-column>
            <el-table-column label="发送时间" width="170">
              <template #default="{ row }">
                <span class="data-mono" style="font-size:12px">{{ row.createTime || '—' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="140" align="center" fixed="right">
              <template #default="{ row }">
                <el-button size="small" @click="viewSendRecords(row)">详情</el-button>
                <el-button size="small" type="danger" @click="deleteBody(row)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <div v-if="bodyTotal > bodyPageSize" style="margin-top:12px;display:flex;justify-content:center">
          <el-pagination
            v-model:current-page="bodyPage"
            :page-size="bodyPageSize"
            :total="bodyTotal"
            layout="prev, pager, next"
            @current-change="loadEmailBodies"
          />
        </div>
      </el-tab-pane>

      <!-- ============================================================== -->
      <!-- Tab 4: Tenant Email Config -->
      <!-- ============================================================== -->
      <el-tab-pane label="租户邮件配置" name="config">
        <el-card shadow="none" class="table-card">
          <el-table :data="tenantConfigs" v-loading="configLoading" border stripe style="width: 100%">
            <template #empty>
              <el-empty description="暂无邮件配置" :image-size="80" />
            </template>
            <el-table-column type="index" label="#" width="50" align="center" />
            <el-table-column prop="tenantName" label="租户名称" min-width="120" show-overflow-tooltip />
            <el-table-column prop="domainName" label="域名" min-width="140" show-overflow-tooltip />
            <el-table-column prop="senderEmail" label="发件邮箱" min-width="180" show-overflow-tooltip />
            <el-table-column prop="smtpHost" label="SMTP 服务器" min-width="220" show-overflow-tooltip />
            <el-table-column label="状态" width="90" align="center">
              <template #default="{ row }">
                <span class="status-dot" :class="row.active === 1 ? 'status-dot--up status-dot--pulse' : 'status-dot--down'"></span>
                {{ row.active === 1 ? '启用' : '停用' }}
              </template>
            </el-table-column>
            <el-table-column label="今日已发" width="90" align="center">
              <template #default="{ row }">
                <span class="data-mono">{{ row.todaySentCount || 0 }} / {{ row.dailyEmailLimit || 200 }}</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="200" align="center" fixed="right">
              <template #default="{ row }">
                <el-button size="small" @click="viewConfigDetail(row)">详情</el-button>
                <el-button
                  v-if="row.active !== 1"
                  size="small"
                  type="success"
                  @click="enableEmail(row)"
                  :loading="enableLoading"
                >
                  启用
                </el-button>
                <el-button
                  v-if="row.active === 1"
                  size="small"
                  type="warning"
                  @click="disableEmail(row)"
                  :loading="disableLoading"
                >
                  禁用
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- ================================================================ -->
    <!-- Add Recipient Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="addRecipientVisible" title="添加收件人" width="460px" destroy-on-close>
      <el-form :model="recipientForm" label-width="80px">
        <el-form-item label="姓名" required>
          <el-input v-model="recipientForm.name" placeholder="收件人姓名" />
        </el-form-item>
        <el-form-item label="邮箱" required>
          <el-input v-model="recipientForm.email" placeholder="recipient@example.com" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addRecipientVisible = false">取消</el-button>
        <el-button type="primary" :loading="addRecipientSaving" @click="saveRecipient">添加</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Send Records Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="sendRecordsVisible" :title="`发送详情 — ${sendRecordsBodyTitle}`" width="800px" destroy-on-close>
      <el-table :data="sendRecords" v-loading="sendRecordsLoading" border size="small" max-height="420">
        <template #empty><el-empty description="暂无发送记录" :image-size="60" /></template>
        <el-table-column type="index" label="#" width="50" align="center" />
        <el-table-column prop="receiveEmailAddress" label="收件人邮箱" min-width="200" show-overflow-tooltip />
        <el-table-column prop="emailSendAddress" label="发件人邮箱" min-width="180" show-overflow-tooltip />
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.sendState === 1 ? 'success' : 'danger'" size="small">
              {{ row.sendState === 1 ? '成功' : '失败' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="发送时间" width="170">
          <template #default="{ row }">
            <span class="data-mono" style="font-size:12px">{{ row.createTime || '—' }}</span>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Config Detail Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="configDetailVisible" :title="`邮件配置详情 — ${configDetailData.tenantName || ''}`" width="600px" destroy-on-close>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item label="配置 ID">{{ configDetailData.id }}</el-descriptions-item>
        <el-descriptions-item label="租户 ID">{{ configDetailData.tenantId }}</el-descriptions-item>
        <el-descriptions-item label="租户名称">{{ configDetailData.tenantName || '—' }}</el-descriptions-item>
        <el-descriptions-item label="域名">{{ configDetailData.domainName || '—' }}</el-descriptions-item>
        <el-descriptions-item label="发件邮箱">{{ configDetailData.senderEmail || '—' }}</el-descriptions-item>
        <el-descriptions-item label="SMTP 服务器">{{ configDetailData.smtpHost || '—' }}</el-descriptions-item>
        <el-descriptions-item label="SMTP 端口">{{ configDetailData.smtpPort || '—' }}</el-descriptions-item>
        <el-descriptions-item label="SMTP 用户名">
          <span class="data-mono">{{ configDetailData.smtpUsername || '—' }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="configDetailData.active === 1 ? 'success' : 'info'" size="small">
            {{ configDetailData.active === 1 ? '启用' : '停用' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="每日限额">{{ configDetailData.dailyEmailLimit || 200 }}</el-descriptions-item>
        <el-descriptions-item label="今日已发">{{ configDetailData.todaySentCount || 0 }}</el-descriptions-item>
        <el-descriptions-item label="域名 OCID">
          <span class="data-mono" style="font-size:11px">{{ configDetailData.domainId || '—' }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="发件人 OCID">
          <span class="data-mono" style="font-size:11px">{{ configDetailData.senderId || '—' }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="DKIM OCID">
          <span class="data-mono" style="font-size:11px">{{ configDetailData.dkimId || '—' }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ configDetailData.createdTime || '—' }}</el-descriptions-item>
      </el-descriptions>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, Delete, Search, Promotion } from '@element-plus/icons-vue'
import request from '../utils/request'

// ============================================================
// Types
// ============================================================

interface EmailRecipient {
  id: number
  name: string
  email: string
  createTime?: string
  updateTime?: string
}

interface TenantEmailConfig {
  id: number
  tenantId: number
  tenantName?: string
  domainId?: string
  domainName?: string
  senderId?: string
  credentialId?: string
  smtpUsername?: string
  smtpPassword?: string
  smtpHost?: string
  smtpPort?: string
  senderEmail?: string
  dkimId?: string
  active: number
  createdTime?: string
  dailyEmailLimit?: number
  todaySentCount?: number
}

interface EmailBody {
  id: number
  emailBodyId: string
  title: string
  content?: string
  senderEmail?: string
  tenantEmailConfigId?: number
  receiveTotal: number
  receiveSuccessTotal: number
  receiveFailTotal: number
  createTime?: string
}

interface EmailSendRecord {
  id: number
  emailSendRecordId: string
  emailBodyId: string
  emailSendAddress: string
  receiveEmailAddress: string
  sendState: number
  createTime?: string
}

interface SendResult {
  email: string
  success: boolean
  message: string
}

// ============================================================
// State — Tabs
// ============================================================

const activeTab = ref('recipients')
const tabLoading = ref(false)

// ============================================================
// State — Recipients
// ============================================================

const recipients = ref<EmailRecipient[]>([])
const recipientLoading = ref(false)
const recipientSearch = ref('')
const addRecipientVisible = ref(false)
const addRecipientSaving = ref(false)
const recipientForm = ref({ name: '', email: '' })

// ============================================================
// State — Compose
// ============================================================

const tenantConfigs = ref<TenantEmailConfig[]>([])
const composeFormRef = ref()
const composeForm = ref({
  tenantEmailConfigId: null as number | null,
  title: '',
  content: '',
  emailReceiveIds: [] as number[],
})
const composeRules = {
  tenantEmailConfigId: [{ required: true, message: '请选择邮件配置', trigger: 'change' }],
  title: [{ required: true, message: '请输入邮件主题', trigger: 'blur' }],
  emailReceiveIds: [{ required: true, type: 'array', min: 1, message: '请选择至少一位收件人', trigger: 'change' }],
  content: [{ required: true, message: '请输入邮件正文', trigger: 'blur' }],
}
const sending = ref(false)
const sendResults = ref<SendResult[]>([])

// ============================================================
// State — History
// ============================================================

const emailBodies = ref<EmailBody[]>([])
const bodyLoading = ref(false)
const bodyPage = ref(1)
const bodyPageSize = ref(20)
const bodyTotal = ref(0)
const selectedBodies = ref<EmailBody[]>([])
const historyConfigFilter = ref<number | null>(null)

// ============================================================
// State — Send Records Dialog
// ============================================================

const sendRecordsVisible = ref(false)
const sendRecordsLoading = ref(false)
const sendRecordsBodyTitle = ref('')
const sendRecords = ref<EmailSendRecord[]>([])

// ============================================================
// State — Tenant Config
// ============================================================

const configLoading = ref(false)
const enableLoading = ref(false)
const disableLoading = ref(false)

// ============================================================
// State — Config Detail Dialog
// ============================================================

const configDetailVisible = ref(false)
const configDetailData = ref<TenantEmailConfig>({} as TenantEmailConfig)

// ============================================================
// Computed
// ============================================================

const filteredRecipients = computed(() => {
  if (!recipientSearch.value) return recipients.value
  const q = recipientSearch.value.toLowerCase()
  return recipients.value.filter(r =>
    (r.name || '').toLowerCase().includes(q) ||
    (r.email || '').toLowerCase().includes(q)
  )
})

const recipientTransferData = computed(() => {
  return recipients.value.map(r => ({
    key: r.id,
    label: `${r.name} <${r.email}>`,
    disabled: false,
  }))
})

// ============================================================
// Tab Change
// ============================================================

function onTabChange(tab: string | number) {
  switch (tab) {
    case 'recipients': loadRecipients(); break
    case 'compose': loadTenantConfigs(); loadRecipients(); break
    case 'history': loadTenantConfigs(); loadEmailBodies(); break
    case 'config': loadTenantConfigs(); break
  }
}

function refreshCurrentTab() {
  onTabChange(activeTab.value)
}

// ============================================================
// Recipient Management
// ============================================================

async function loadRecipients() {
  recipientLoading.value = true
  try {
    const resp: any = await request.post('/email/receive/list', { page: 1, pageSize: 500 })
    recipients.value = resp?.list || resp || []
  } catch (e: any) {
    ElMessage.error('加载收件人失败: ' + (e?.message || e))
    recipients.value = []
  } finally {
    recipientLoading.value = false
  }
}

function openAddRecipient() {
  recipientForm.value = { name: '', email: '' }
  addRecipientVisible.value = true
}

async function saveRecipient() {
  if (!recipientForm.value.name || !recipientForm.value.email) {
    ElMessage.warning('请填写姓名和邮箱')
    return
  }
  addRecipientSaving.value = true
  try {
    await request.post('/email/receive/add', {
      name: recipientForm.value.name,
      email: recipientForm.value.email,
    })
    ElMessage.success('收件人已添加')
    addRecipientVisible.value = false
    await loadRecipients()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    addRecipientSaving.value = false
  }
}

async function deleteRecipient(row: EmailRecipient) {
  try {
    await ElMessageBox.confirm(
      `确定删除收件人「${row.name} <${row.email}>」？`,
      '确认删除',
      { type: 'warning' }
    )
    await request.post('/email/receive/delete', { id: row.id })
    ElMessage.success('已删除')
    await loadRecipients()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// ============================================================
// Compose & Send
// ============================================================

async function loadTenantConfigs() {
  try {
    const resp: any = await request.post('/email/tenant/list', { page: 1, pageSize: 100 })
    tenantConfigs.value = resp?.list || resp || []
  } catch (e: any) {
    tenantConfigs.value = []
  }
}

function resetComposeForm() {
  composeForm.value = {
    tenantEmailConfigId: null,
    title: '',
    content: '',
    emailReceiveIds: [],
  }
  sendResults.value = []
}

async function sendEmail() {
  if (!composeFormRef.value) return
  try {
    await composeFormRef.value.validate()
  } catch {
    return
  }

  sending.value = true
  sendResults.value = []
  try {
    const resp: any = await request.post('/email/send', {
      tenantEmailConfigId: composeForm.value.tenantEmailConfigId,
      title: composeForm.value.title,
      content: composeForm.value.content,
      emailReceiveIds: composeForm.value.emailReceiveIds,
    })
    sendResults.value = resp?.results || resp || []
    const successCount = sendResults.value.filter(r => r.success).length
    const failCount = sendResults.value.filter(r => !r.success).length
    if (failCount === 0) {
      ElMessage.success(`全部 ${successCount} 封邮件发送成功`)
    } else {
      ElMessage.warning(`发送完成: ${successCount} 成功, ${failCount} 失败`)
    }
  } catch (e: any) {
    ElMessage.error('发送失败: ' + (e?.message || e))
  } finally {
    sending.value = false
  }
}

// ============================================================
// Email History
// ============================================================

async function loadEmailBodies() {
  bodyLoading.value = true
  try {
    const payload: any = { page: bodyPage.value, pageSize: bodyPageSize.value }
    if (historyConfigFilter.value) {
      payload.tenantEmailConfigId = historyConfigFilter.value
    }
    const resp: any = await request.post('/email/body/list', payload)
    emailBodies.value = resp?.list || resp || []
    bodyTotal.value = resp?.total || emailBodies.value.length
  } catch (e: any) {
    ElMessage.error('加载发送记录失败: ' + (e?.message || e))
    emailBodies.value = []
  } finally {
    bodyLoading.value = false
  }
}

function onBodySelectionChange(rows: EmailBody[]) {
  selectedBodies.value = rows
}

async function viewSendRecords(body: EmailBody) {
  sendRecordsBodyTitle.value = body.title || body.emailBodyId
  sendRecordsVisible.value = true
  sendRecordsLoading.value = true
  try {
    const resp: any = await request.post('/email/send/list', {
      emailBodyId: body.emailBodyId,
      page: 1,
      pageSize: 500,
    })
    sendRecords.value = resp?.list || resp || []
  } catch (e: any) {
    ElMessage.error('加载发送详情失败: ' + (e?.message || e))
    sendRecords.value = []
  } finally {
    sendRecordsLoading.value = false
  }
}

async function deleteBody(body: EmailBody) {
  try {
    await ElMessageBox.confirm(
      `确定删除邮件「${body.title}」及其所有发送记录？`,
      '确认删除',
      { type: 'warning' }
    )
    await request.post('/email/body/delete', { id: body.id, emailBodyId: body.emailBodyId })
    ElMessage.success('已删除')
    await loadEmailBodies()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

async function batchDeleteBodies() {
  if (selectedBodies.value.length === 0) return
  try {
    await ElMessageBox.confirm(
      `确定批量删除 ${selectedBodies.value.length} 条邮件记录及其发送记录？此操作不可恢复。`,
      '确认批量删除',
      { type: 'warning' }
    )
    const ids = selectedBodies.value.map(b => b.id)
    const emailBodyIds = selectedBodies.value.map(b => b.emailBodyId)
    await request.post('/email/body/batchDelete', { ids, emailBodyIds })
    ElMessage.success(`已删除 ${ids.length} 条记录`)
    selectedBodies.value = []
    await loadEmailBodies()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// ============================================================
// Tenant Email Config
// ============================================================

function viewConfigDetail(cfg: TenantEmailConfig) {
  configDetailData.value = cfg
  configDetailVisible.value = true
}

async function enableEmail(cfg: TenantEmailConfig) {
  try {
    await ElMessageBox.confirm(
      `确定为租户「${cfg.tenantName || '租户#' + cfg.tenantId}」启用邮件服务？\n\n此操作将在 OCI 中创建邮件域、发件人、DKIM 并配置 DNS 记录。`,
      '确认启用邮件服务',
      { type: 'info' }
    )
    enableLoading.value = true
    await request.post('/email/enable', { tenantId: cfg.tenantId })
    ElMessage.success('邮件服务已启用')
    await loadTenantConfigs()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error('启用失败: ' + e.message)
  } finally {
    enableLoading.value = false
  }
}

async function disableEmail(cfg: TenantEmailConfig) {
  try {
    await ElMessageBox.confirm(
      `确定为租户「${cfg.tenantName || '租户#' + cfg.tenantId}」禁用邮件服务？\n\n此操作将删除 OCI 中的邮件域、发件人、DKIM 和 DNS 记录，且不可恢复。`,
      '确认禁用邮件服务',
      { type: 'warning', confirmButtonText: '确定禁用' }
    )
    disableLoading.value = true
    await request.post('/email/disable', { id: cfg.id, tenantId: cfg.tenantId })
    ElMessage.success('邮件服务已禁用')
    await loadTenantConfigs()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error('禁用失败: ' + e.message)
  } finally {
    disableLoading.value = false
  }
}

// ============================================================
// Watch config filter for history tab
// ============================================================

// Reload bodies when config filter changes
watch(historyConfigFilter, () => {
  if (activeTab.value === 'history') {
    bodyPage.value = 1
    loadEmailBodies()
  }
})

// ============================================================
// Init
// ============================================================

onMounted(() => {
  loadRecipients()
})
</script>

<style scoped>
.email-page { padding: 0; }

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-5);
  flex-wrap: wrap;
  gap: var(--space-4);
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex-wrap: wrap;
}

.toolbar-left h2 {
  margin: 0;
  font-size: var(--text-xl);
  font-weight: var(--font-bold);
  color: var(--text-primary);
  letter-spacing: var(--tracking-tight);
}

.toolbar-right { display: flex; gap: var(--space-2); }

.tab-toolbar {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
  flex-wrap: wrap;
}

.table-card { border-radius: var(--radius-md); overflow: hidden; }
.table-card :deep(.el-card__body) { padding: 0; }

.compose-card { border-radius: var(--radius-md); }
.result-card { border-radius: var(--radius-md); }

.data-mono {
  font-family: var(--font-mono, 'SF Mono', 'Consolas', monospace);
}

/* Element overrides */
:deep(.el-table) { border-radius: var(--radius-md); overflow: hidden; }
:deep(.el-table th) {
  background: var(--bg-raised);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

:deep(.el-dialog) { border-radius: var(--radius-lg); }
:deep(.el-dialog__title) {
  font-size: var(--text-lg);
  font-weight: var(--font-semibold);
}

:deep(.el-tabs__header) {
  margin-bottom: var(--space-4);
}

:deep(.el-transfer) {
  display: flex;
  align-items: flex-start;
}
:deep(.el-transfer-panel) {
  width: 280px;
}

@media (max-width: 768px) {
  .toolbar { flex-direction: column; align-items: flex-start; }
  .toolbar-left h2 { font-size: var(--text-lg); }
  .toolbar-right { width: 100%; justify-content: flex-start; }
  .tab-toolbar { flex-direction: column; align-items: flex-start; }
  :deep(.el-transfer) { flex-direction: column; }
}
</style>
