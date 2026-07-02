<template>
  <div class="tenant-detail-page">
    <!-- Header -->
    <div class="detail-header">
      <div class="header-left">
        <el-button text @click="router.push('/tenants')"><el-icon><ArrowLeft /></el-icon> 返回</el-button>
        <h2>{{ tenant.userName || tenant.tenancyName || `租户 #${tenant.id}` }}</h2>
        <el-tag v-if="tenant.accountType" :type="accountTypeTag(tenant.accountType)" size="small">{{ accountTypeLabel(tenant.accountType) }}</el-tag>
        <el-tag v-if="tenant.isActive !== undefined" :type="tenant.isActive ? 'success' : 'danger'" size="small">{{ tenant.isActive ? '正常' : '停用' }}</el-tag>
      </div>
      <div class="header-right">
        <el-button size="small" @click="syncOci" :loading="syncing"><el-icon><Connection /></el-icon> 同步</el-button>
        <el-button size="small" @click="doUpdateDetail" :loading="updateSaving"><el-icon><Refresh /></el-icon> 更新信息</el-button>
        <el-button size="small" @click="checkTenant" :loading="checking"><el-icon><Monitor /></el-icon> 测试存活</el-button>
      </div>
    </div>
    <el-alert v-if="checkResult" :type="checkResult.alive ? 'success' : 'error'" show-icon style="margin-bottom:12px"
      :title="checkResult.alive ? 'OCI 认证成功' : '异常: ' + (checkResult.error || '未知错误')" />

    <!-- Tabs -->
    <el-tabs v-model="activeTab" @tab-change="onTabChange">
      <!-- ======================== 概况 ======================== -->
      <el-tab-pane label="概况" name="overview">
        <el-skeleton v-if="loading" :rows="8" animated/>
        <template v-else>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="租户 ID">{{ tenant.id }}</el-descriptions-item>
            <el-descriptions-item label="用户名">{{ tenant.userName }}</el-descriptions-item>
            <el-descriptions-item label="Tenancy OCID" :span="2"><span class="data-mono">{{ tenant.tenancy }}</span></el-descriptions-item>
            <el-descriptions-item label="User OCID" :span="2"><span class="data-mono">{{ tenant.tenantId }}</span></el-descriptions-item>
            <el-descriptions-item label="指纹" :span="2"><span class="data-mono">{{ tenant.fingerprint }}</span></el-descriptions-item>
            <el-descriptions-item label="区域">{{ tenant.regionName || tenant.region }}</el-descriptions-item>
            <el-descriptions-item label="区域代码">{{ tenant.region }}</el-descriptions-item>
            <el-descriptions-item label="云厂商"><el-tag size="small">{{ cloudTypeLabel(tenant.cloudType) }}</el-tag></el-descriptions-item>
            <el-descriptions-item label="自定义名称">
              {{ tenant.tenancyDes || '—' }}
              <el-button size="small" text type="primary" @click="openEditName" style="margin-left:8px">编辑</el-button>
            </el-descriptions-item>
            <el-descriptions-item label="账号成本">
              <span class="data-mono">{{ tenant.accountCost || '—' }}</span>
              <el-button size="small" text type="primary" @click="openEditCost" style="margin-left:8px">编辑</el-button>
            </el-descriptions-item>
            <el-descriptions-item label="邮箱">{{ tenant.emailAddress || '—' }}</el-descriptions-item>
            <el-descriptions-item label="创建时间">{{ tenant.createdAt || '—' }}</el-descriptions-item>
            <el-descriptions-item label="存活天数"><span class="days-chip">{{ tenant.activeDays || '0' }} 天</span></el-descriptions-item>
            <el-descriptions-item label="主区域">{{ tenant.regionName || tenant.region || '—' }}</el-descriptions-item>
            <el-descriptions-item label="API 同步"><el-tag :type="tenant.apiSynced ? 'success' : 'info'" size="small">{{ tenant.apiSynced ? '已同步' : '未同步' }}</el-tag></el-descriptions-item>
            <el-descriptions-item label="父租户 ID">{{ tenant.parenId || '无 (主租户)' }}</el-descriptions-item>
          </el-descriptions>
        </template>
      </el-tab-pane>

      <!-- ======================== 实例 ======================== -->
      <el-tab-pane label="实例" name="instances">
        <el-skeleton v-if="instLoading" :rows="5" animated/>
        <el-table v-else :data="instances" border stripe size="small">
          <template #empty><el-empty description="该租户下暂无实例" :image-size="60"/></template>
          <el-table-column prop="displayName" label="名称" min-width="160"/>
          <el-table-column prop="instanceId" label="实例 ID" min-width="200" show-overflow-tooltip/>
          <el-table-column prop="shape" label="Shape" min-width="140"/>
          <el-table-column label="状态" width="100">
            <template #default="{ row }"><div class="state-cell"><span class="status-dot" :class="instStateDot(row.state)"/>{{ row.state || '—' }}</div></template>
          </el-table-column>
          <el-table-column prop="publicIps" label="公网 IP" width="140"/>
          <el-table-column prop="architecture" label="架构" width="80"/>
          <el-table-column label="规格" width="120"><template #default="{ row }">{{ row.ocpus || 0 }}C / {{ row.memoryInGbs || 0 }}G</template></el-table-column>
          <el-table-column prop="availabilityDomain" label="AD" min-width="120" show-overflow-tooltip/>
          <el-table-column prop="createTime" label="创建时间" width="160"/>
        </el-table>
      </el-tab-pane>

      <!-- ======================== 费用 ======================== -->
      <el-tab-pane label="费用" name="costs">
        <div v-if="subscriptionData" style="margin-bottom:16px">
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="计划类型">
              <el-tag :type="subscriptionData.planType === 'FREE_TIER' ? 'warning' : 'success'" size="small">{{ subscriptionData.planType === 'FREE_TIER' ? '免费层' : subscriptionData.planType === 'PAYG' ? '按量付费' : subscriptionData.planType || '—' }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="账号类型">
              <el-tag size="small">{{ subscriptionData.accountType === 'PERSONAL' ? '个人' : subscriptionData.accountType === 'CORPORATE' ? '企业' : subscriptionData.accountType || '—' }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="订阅开始">{{ subscriptionData.timeStart || '—' }}</el-descriptions-item>
            <el-descriptions-item label="货币">{{ subscriptionData.currencyCode || '—' }}</el-descriptions-item>
            <el-descriptions-item label="付费意向">{{ subscriptionData.isIntentToPay ? '是' : '否' }}</el-descriptions-item>
            <el-descriptions-item label="邮箱">{{ subscriptionData.emailAddress || '—' }}</el-descriptions-item>
          </el-descriptions>
        </div>
        <div style="margin-bottom:12px;display:flex;gap:8px;align-items:center;flex-wrap:wrap">
          <el-button size="small" :type="costQueryType==='today'?'primary':''" @click="loadCost('today')">今日</el-button>
          <el-button size="small" :type="costQueryType==='yesterday'?'primary':''" @click="loadCost('yesterday')">昨日</el-button>
          <el-button size="small" :type="costQueryType==='current_month'?'primary':''" @click="loadCost('current_month')">本月</el-button>
          <el-button size="small" :type="costQueryType==='last_month'?'primary':''" @click="loadCost('last_month')">上月</el-button>
          <el-date-picker v-model="costDateRange" type="daterange" range-separator="至" start-placeholder="开始" end-placeholder="结束" format="YYYY-MM-DD" value-format="YYYY-MM-DD" size="small" style="width:280px" @change="costQueryType='custom'"/>
          <el-button size="small" type="primary" @click="loadCostCustomRange" :loading="costLoading"><el-icon><Search /></el-icon> 查询</el-button>
        </div>
        <div v-if="costData.length" style="margin-bottom:12px;padding:12px 16px;background:var(--bg-raised);border-radius:var(--radius-md);display:flex;gap:24px;align-items:center">
          <el-statistic title="总费用"><template #default><span style="color:var(--accent);font-size:var(--text-xl)">{{ costTotal.toFixed(2) }}</span></template></el-statistic>
          <el-statistic title="记录数" :value="costData.length"/>
        </div>
        <el-table v-loading="costLoading" :data="costData" border stripe size="small" max-height="360" show-overflow-tooltip>
          <template #empty><el-empty description="暂无费用数据" :image-size="60"/></template>
          <el-table-column prop="service" label="服务" min-width="180" sortable/>
          <el-table-column label="金额" width="130" align="right" sortable sort-by="computedAmount"><template #default="{ row }"><span class="data-mono" style="font-weight:600">{{ Number(row.computedAmount || 0).toFixed(2) }}</span></template></el-table-column>
          <el-table-column prop="currency" label="货币" width="80" align="center"/>
          <el-table-column label="用量" width="100" align="right"><template #default="{ row }">{{ Number(row.computedQuantity || 0).toFixed(2) }}</template></el-table-column>
          <el-table-column prop="skuName" label="SKU" min-width="160"/>
          <el-table-column prop="region" label="区域" width="120"/>
          <el-table-column label="开始" width="160"><template #default="{ row }">{{ row.timeUsageStarted || '—' }}</template></el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- ======================== 用户 ======================== -->
      <el-tab-pane label="用户" name="users">
        <div style="margin-bottom:12px"><el-button type="primary" size="small" @click="addUserFormVisible=true"><el-icon><Plus /></el-icon> 创建用户</el-button></div>
        <el-skeleton v-if="userLoading" :rows="4" animated/>
        <el-table v-else :data="userList" border stripe size="small">
          <template #empty><el-empty description="暂无 IAM 用户" :image-size="60"/></template>
          <el-table-column prop="name" label="用户名" min-width="140"/>
          <el-table-column prop="description" label="描述" min-width="160" show-overflow-tooltip/>
          <el-table-column label="状态" width="90" align="center"><template #default="{ row }"><el-tag :type="row.lifecycleState==='ACTIVE'?'success':'warning'" size="small">{{ row.lifecycleState || '—' }}</el-tag></template></el-table-column>
          <el-table-column prop="email" label="邮箱" min-width="180" show-overflow-tooltip/>
          <el-table-column label="操作" width="180" align="center">
            <template #default="{ row }">
              <el-button size="small" @click="resetUserPassword(row)" :loading="row._resetting">重置密码</el-button>
              <el-button size="small" type="danger" @click="deleteUser(row)"><el-icon><Delete /></el-icon></el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-dialog v-model="addUserFormVisible" title="创建 IAM 用户" width="460px" append-to-body destroy-on-close>
          <el-form :model="addUserForm" label-width="80px">
            <el-form-item label="用户名" required><el-input v-model="addUserForm.name" placeholder="用户名"/></el-form-item>
            <el-form-item label="描述"><el-input v-model="addUserForm.description" placeholder="可选描述"/></el-form-item>
          </el-form>
          <el-alert v-if="createdUserPwd" title="用户创建成功！请复制以下一次性密码" type="success" :closable="true" show-icon @close="createdUserPwd=''" style="margin-top:12px">
            <template #default><code style="user-select:all">{{ createdUserPwd }}</code></template>
          </el-alert>
          <template #footer>
            <el-button @click="addUserFormVisible=false">关闭</el-button>
            <el-button type="primary" :loading="addUserSaving" @click="createUser">创建</el-button>
          </template>
        </el-dialog>
      </el-tab-pane>

      <!-- ======================== 邮件 ======================== -->
      <el-tab-pane label="邮件" name="email">
        <el-form :model="emailForm" label-width="100px" style="max-width:500px">
          <el-form-item label="域名"><el-input v-model="emailForm.domainName" placeholder="example.com"/></el-form-item>
          <el-form-item label="SMTP 主机"><el-input v-model="emailForm.smtpHost" placeholder="smtp.example.com"/></el-form-item>
          <el-form-item label="SMTP 端口"><el-input v-model="emailForm.smtpPort" placeholder="587"/></el-form-item>
          <el-form-item label="用户名"><el-input v-model="emailForm.smtpUsername" placeholder="邮箱用户名"/></el-form-item>
          <el-form-item label="密码"><el-input v-model="emailForm.smtpPassword" type="password" show-password placeholder="邮箱密码"/></el-form-item>
          <el-form-item label="发件人"><el-input v-model="emailForm.senderEmail" placeholder="noreply@example.com"/></el-form-item>
          <el-form-item label="启用">
            <el-switch v-model="emailForm.active"/>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="emailSaving" @click="saveEmail">保存</el-button>
            <el-button type="danger" @click="deleteEmail">删除配置</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- ======================== 社交登录 ======================== -->
      <el-tab-pane label="社交登录" name="social">
        <div style="margin-bottom:12px"><el-button type="primary" size="small" @click="openAddSocial"><el-icon><Plus /></el-icon> 添加配置</el-button></div>
        <el-table :data="socialList" border size="small" v-loading="socialLoading">
          <template #empty><el-empty description="暂无社媒配置" :image-size="40"/></template>
          <el-table-column prop="socialTypeStr" label="平台" width="100"/>
          <el-table-column prop="clientId" label="Client ID" min-width="160" show-overflow-tooltip/>
          <el-table-column label="状态" width="90" align="center"><template #default="{ row }"><el-tag :type="row.socialStatus==='enabled'?'success':'info'" size="small">{{ row.socialStatus==='enabled'?'启用':'禁用' }}</el-tag></template></el-table-column>
          <el-table-column label="操作" width="140" align="center">
            <template #default="{ row }">
              <el-button size="small" @click="editSocial(row)"><el-icon><Edit /></el-icon></el-button>
              <el-button size="small" type="warning" @click="toggleSocial(row)">{{ row.socialStatus==='enabled'?'禁用':'启用' }}</el-button>
              <el-button size="small" type="danger" @click="deleteSocial(row)"><el-icon><Delete /></el-icon></el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-dialog v-model="socialEditVisible" :title="socialEditId ? '编辑社媒配置' : '添加社媒配置'" width="460px" append-to-body destroy-on-close>
          <el-form :model="socialForm" label-width="80px">
            <el-form-item label="平台类型">
              <el-select v-model="socialForm.socialType" placeholder="选择平台">
                <el-option v-for="t in socialTypes" :key="t" :label="t" :value="t"/>
              </el-select>
            </el-form-item>
            <el-form-item label="Client ID" required><el-input v-model="socialForm.clientId" placeholder="OAuth Client ID"/></el-form-item>
            <el-form-item label="Secret" required><el-input v-model="socialForm.clientSecret" type="password" show-password placeholder="OAuth Client Secret"/></el-form-item>
            <el-form-item label="回调地址"><el-input v-model="socialForm.redirectUrl" placeholder="https://your-domain.com/oauth/callback"/></el-form-item>
            <el-form-item label="登录地址"><el-input v-model="socialForm.loginUrl" placeholder="https://platform.com/login"/></el-form-item>
          </el-form>
          <template #footer>
            <el-button @click="socialEditVisible=false">取消</el-button>
            <el-button type="primary" :loading="socialEditSaving" @click="saveSocial">保存</el-button>
          </template>
        </el-dialog>
      </el-tab-pane>

      <!-- ======================== 安全规则 ======================== -->
      <el-tab-pane label="安全规则" name="secRules">
        <el-tabs v-model="secRulesTab" @tab-change="loadSecRules">
          <el-tab-pane label="入站规则" name="ingress"/>
          <el-tab-pane label="出站规则" name="egress"/>
        </el-tabs>
        <div style="margin-bottom:12px"><el-button type="primary" size="small" @click="secRuleAddVisible=true"><el-icon><Plus /></el-icon> 添加规则</el-button></div>
        <el-skeleton v-if="secRulesLoading" :rows="4" animated/>
        <el-table v-else :data="secRulesList" border stripe size="small">
          <template #empty><el-empty description="暂无安全规则" :image-size="60"/></template>
          <el-table-column prop="protocol" label="协议" width="80"/>
          <el-table-column label="端口范围" width="120"><template #default="{ row }">{{ row.tcpOptions?.destinationPortRange?.min || row.udpOptions?.destinationPortRange?.min || '全部' }}{{ row.tcpOptions?.destinationPortRange?.min !== row.tcpOptions?.destinationPortRange?.max ? '-' + (row.tcpOptions?.destinationPortRange?.max || '') : '' }}</template></el-table-column>
          <el-table-column prop="source" label="源地址" min-width="160" show-overflow-tooltip/>
          <el-table-column prop="description" label="描述" min-width="160" show-overflow-tooltip/>
        </el-table>
        <el-dialog v-model="secRuleAddVisible" title="添加安全规则" width="460px" append-to-body destroy-on-close>
          <el-form :model="secRuleForm" label-width="80px">
            <el-form-item label="协议"><el-select v-model="secRuleForm.protocol"><el-option label="TCP" value="6"/><el-option label="UDP" value="17"/><el-option label="全部" value="all"/></el-select></el-form-item>
            <el-form-item label="源地址"><el-input v-model="secRuleForm.source" placeholder="0.0.0.0/0"/></el-form-item>
            <el-form-item label="端口最小"><el-input-number v-model="secRuleForm.min" :min="1" :max="65535"/></el-form-item>
            <el-form-item label="端口最大"><el-input-number v-model="secRuleForm.max" :min="1" :max="65535"/></el-form-item>
            <el-form-item label="描述"><el-input v-model="secRuleForm.description" placeholder="可选"/></el-form-item>
          </el-form>
          <template #footer>
            <el-button @click="secRuleAddVisible=false">取消</el-button>
            <el-button type="primary" :loading="secRuleSaving" @click="addSecRule">添加</el-button>
          </template>
        </el-dialog>
      </el-tab-pane>

      <!-- ======================== 设置 ======================== -->
      <el-tab-pane label="设置" name="settings">
        <el-descriptions :column="1" border size="small" style="margin-bottom:16px">
          <el-descriptions-item label="从 OCI 获取信息">
            <el-button size="small" @click="doUpdateDetail" :loading="updateSaving"><el-icon><Refresh /></el-icon> 从 OCI 获取</el-button>
            <span v-if="updateResult" style="margin-left:8px;color:var(--status-up)">✓ {{ updateResult.tenancyName || '已获取' }}</span>
            <span v-if="updateError" style="margin-left:8px;color:var(--status-down)">✗ {{ updateError }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="导出租户数据">
            <el-button size="small" @click="doExport"><el-icon><Download /></el-icon> 导出</el-button>
          </el-descriptions-item>
          <el-descriptions-item label="删除租户">
            <el-button size="small" type="danger" @click="remove"><el-icon><Delete /></el-icon> 删除此租户</el-button>
            <span style="color:var(--text-secondary);font-size:12px;margin-left:8px">不可恢复，将删除所有实例记录</span>
          </el-descriptions-item>
        </el-descriptions>
      </el-tab-pane>
    </el-tabs>

    <!-- Edit name dialog -->
    <el-dialog v-model="editNameVisible" title="设置自定义名称" width="460px" destroy-on-close>
      <el-form label-width="100px">
        <el-form-item label="租户名"><el-input :model-value="tenant.userName || tenant.tenancyName" disabled/></el-form-item>
        <el-form-item label="自定义名称"><el-input v-model="editNameValue" placeholder="输入自定义名称"/></el-form-item>
      </el-form>
      <template #footer><el-button @click="editNameVisible=false">取消</el-button><el-button type="primary" :loading="editSaving" @click="saveCustomName">保存</el-button></template>
    </el-dialog>
    <!-- Edit cost dialog -->
    <el-dialog v-model="editCostVisible" title="设置账号成本" width="460px" destroy-on-close>
      <el-form label-width="100px">
        <el-form-item label="租户名"><el-input :model-value="tenant.userName || tenant.tenancyName" disabled/></el-form-item>
        <el-form-item label="账号成本"><el-input v-model="editCostValue" placeholder="例如: $29.99/月"/></el-form-item>
      </el-form>
      <template #footer><el-button @click="editCostVisible=false">取消</el-button><el-button type="primary" :loading="editSaving" @click="saveAccountCost">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  ArrowLeft, Connection, Refresh, Monitor, Plus, Edit, Download, Delete, Search
} from '@element-plus/icons-vue'
import request from '../utils/request'
import { accountTypeTag, accountTypeLabel, cloudTypeLabel, instStateDot } from '../utils/tenant-utils'

defineOptions({ name: 'tenant-detail' })

const route = useRoute()
const router = useRouter()
const tenantId = Number(route.params.id)

// --- common ---
const activeTab = ref('overview')
const loading = ref(false)
const tenant = ref<any>({})
const syncing = ref(false)
const checking = ref(false)
const checkResult = ref<any>(null)

// overview: edit name / cost
const editNameVisible = ref(false)
const editNameValue = ref('')
const editCostVisible = ref(false)
const editCostValue = ref('')
const editSaving = ref(false)

// update from OCI
const updateSaving = ref(false)
const updateResult = ref<any>(null)
const updateError = ref('')

// instances
const instLoading = ref(false)
const instances = ref<any[]>([])

// costs
const subscriptionData = ref<any>(null)
const costQueryType = ref('current_month')
const costDateRange = ref<string[] | null>(null)
const costLoading = ref(false)
const costData = ref<any[]>([])
const costTotal = ref(0)

// users
const userLoading = ref(false)
const userList = ref<any[]>([])
const addUserFormVisible = ref(false)
const addUserSaving = ref(false)
const addUserForm = ref({ name: '', description: '' })
const createdUserPwd = ref('')

// email
const emailForm = ref({ domainName: '', smtpHost: '', smtpPort: '587', smtpUsername: '', smtpPassword: '', senderEmail: '', active: false })
const emailSaving = ref(false)

// social
const socialList = ref<any[]>([])
const socialLoading = ref(false)
const socialTypes = ['GITHUB', 'GOOGLE', 'WEIXIN']
const socialEditVisible = ref(false)
const socialEditSaving = ref(false)
const socialEditId = ref('')
const socialForm = ref({ socialType: 'GITHUB', clientId: '', clientSecret: '', redirectUrl: '', loginUrl: '' })

// security rules
const secRulesTab = ref('ingress')
const secRulesList = ref<any[]>([])
const secRulesLoading = ref(false)
const secRuleAddVisible = ref(false)
const secRuleSaving = ref(false)
const secRuleForm = ref({ protocol: '6', source: '0.0.0.0/0', min: 22, max: 22, description: '' })

// --- lifecycle ---
onMounted(async () => {
  loading.value = true
  try {
    tenant.value = await request.get(`/tenants/${tenantId}`)
  } catch (e: any) { ElMessage.error('加载租户失败: ' + e.message) }
  finally { loading.value = false }
})

function onTabChange(tab: string | number) {
  const t = String(tab)
  if (t === 'instances' && !instances.value.length) loadInstances()
  if (t === 'costs' && !subscriptionData.value && !costData.value.length) { loadSubscription(); loadCost() }
  if (t === 'users' && !userList.value.length) loadUsers()
  if (t === 'email') loadEmail()
  if (t === 'social' && !socialList.value.length) loadSocial()
  if (t === 'secRules' && !secRulesList.value.length) loadSecRules()
}

// --- overview actions ---
async function syncOci() {
  try {
    await ElMessageBox.confirm(`从 OCI 同步租户 ${tenant.value.userName} 的实例？`, '确认同步', { type: 'info' })
    syncing.value = true
    await request.get('/tenants/syncOci', { params: { tenantId } })
    ElMessage.success('同步完成')
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error('同步失败: ' + e.message) }
  finally { syncing.value = false }
}
async function checkTenant() {
  checking.value = true; checkResult.value = null
  try { checkResult.value = await request.get(`/tenants/${tenantId}/check`) }
  catch (e: any) { ElMessage.error(e.message) }
  finally { checking.value = false }
}
async function doUpdateDetail() {
  updateSaving.value = true; updateError.value = ''; updateResult.value = null
  try {
    updateResult.value = await request.post(`/tenants/${tenantId}/update-detail`)
    ElMessage.success('已从 OCI 获取租户信息')
    tenant.value = await request.get(`/tenants/${tenantId}`)
  } catch (e: any) { updateError.value = '获取失败: ' + (e?.message || e) }
  finally { updateSaving.value = false }
}
function openEditName() { editNameValue.value = tenant.value.tenancyDes || ''; editNameVisible.value = true }
async function saveCustomName() {
  editSaving.value = true
  try {
    await request.put(`/tenants/${tenantId}`, { ...tenant.value, tenancyDes: editNameValue.value })
    tenant.value.tenancyDes = editNameValue.value; editNameVisible.value = false; ElMessage.success('已更新')
  } catch (e: any) { ElMessage.error(e.message) }
  finally { editSaving.value = false }
}
function openEditCost() { editCostValue.value = tenant.value.accountCost || ''; editCostVisible.value = true }
async function saveAccountCost() {
  editSaving.value = true
  try {
    await request.put(`/tenants/${tenantId}`, { ...tenant.value })
    tenant.value.accountCost = editCostValue.value; editCostVisible.value = false; ElMessage.success('已更新')
  } catch (e: any) { ElMessage.error(e.message) }
  finally { editSaving.value = false }
}

// --- instances ---
async function loadInstances() {
  instLoading.value = true
  try { instances.value = await request.get(`/tenants/${tenantId}/instances`) as any[] }
  catch (e: any) { ElMessage.error(e.message) }
  finally { instLoading.value = false }
}

// --- costs ---
async function loadSubscription() {
  try { subscriptionData.value = await request.get(`/tenants/${tenantId}/subscription`) }
  catch { subscriptionData.value = null }
}
async function loadCost(type?: string, start?: string, end?: string) {
  const qType = type || costQueryType.value; costQueryType.value = qType; costLoading.value = true; costData.value = []; costTotal.value = 0
  try {
    const params: any = { type: qType }
    if (qType === 'custom' && start && end) { params.start = start; params.end = end }
    const resp = await request.get(`/tenants/${tenantId}/cost`, { params }) as any[]
    costData.value = resp || []; costTotal.value = (resp || []).reduce((s, i) => s + (Number(i.computedAmount) || 0), 0)
  } catch (e: any) { ElMessage.error('费用查询失败: ' + (e?.message || e)) }
  finally { costLoading.value = false }
}
function loadCostCustomRange() {
  if (!costDateRange.value || costDateRange.value.length !== 2) { ElMessage.warning('请选择日期范围'); return }
  loadCost('custom', costDateRange.value[0], costDateRange.value[1])
}

// --- users ---
async function loadUsers() {
  userLoading.value = true
  try { userList.value = await request.get(`/tenants/${tenantId}/users`) as any[] }
  catch { userList.value = [] }
  finally { userLoading.value = false }
}
async function createUser() {
  if (!addUserForm.value.name) { ElMessage.warning('请填写用户名'); return }
  addUserSaving.value = true; createdUserPwd.value = ''
  try {
    const r: any = await request.post(`/tenants/${tenantId}/users`, addUserForm.value)
    createdUserPwd.value = r?.password || ''
    ElMessage.success('用户已创建'); await loadUsers(); addUserForm.value = { name: '', description: '' }
  } catch (e: any) { ElMessage.error(e.message) }
  finally { addUserSaving.value = false }
}
async function resetUserPassword(row: any) {
  try {
    const r: any = await request.post(`/tenants/${tenantId}/users/${encodeURIComponent(row.ocid)}/reset-password`)
    ElMessage.success('密码已重置: ' + (r?.password || ''))
  } catch (e: any) { ElMessage.error(e.message) }
}
async function deleteUser(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除用户「${row.name}」？`, '确认', { type: 'warning' })
    await request.delete(`/tenants/${tenantId}/users/${encodeURIComponent(row.ocid)}`)
    ElMessage.success('已删除'); await loadUsers()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
}

// --- email ---
async function loadEmail() {
  try {
    const cfg: any = await request.get(`/tenants/${tenantId}/email`)
    emailForm.value = { domainName: cfg?.domainName || '', smtpHost: cfg?.smtpHost || '', smtpPort: cfg?.smtpPort || '587', smtpUsername: cfg?.smtpUsername || '', smtpPassword: cfg?.smtpPassword || '', senderEmail: cfg?.senderEmail || '', active: cfg?.active === true || cfg?.active === 1 }
  } catch { /* no config yet */ }
}
async function saveEmail() {
  emailSaving.value = true
  try { await request.post(`/tenants/${tenantId}/email`, emailForm.value); ElMessage.success('邮件配置已保存') }
  catch (e: any) { ElMessage.error(e.message) }
  finally { emailSaving.value = false }
}
async function deleteEmail() {
  try { await request.delete(`/tenants/${tenantId}/email`); ElMessage.success('已删除'); emailForm.value = { domainName: '', smtpHost: '', smtpPort: '587', smtpUsername: '', smtpPassword: '', senderEmail: '', active: false } }
  catch (e: any) { ElMessage.error(e.message) }
}

// --- social ---
async function loadSocial() {
  socialLoading.value = true
  try { socialList.value = await request.get(`/tenants/${tenantId}/social`) as any[] }
  catch { socialList.value = [] }
  finally { socialLoading.value = false }
}
function openAddSocial() { socialEditId.value = ''; socialForm.value = { socialType: 'GITHUB', clientId: '', clientSecret: '', redirectUrl: '', loginUrl: '' }; socialEditVisible.value = true }
function editSocial(row: any) { socialEditId.value = row.id; socialForm.value = { socialType: row.socialTypeStr || 'GITHUB', clientId: row.clientId || '', clientSecret: '', redirectUrl: row.redirectUrl || '', loginUrl: row.loginUrl || '' }; socialEditVisible.value = true }
async function saveSocial() {
  socialEditSaving.value = true
  try {
    const url = socialEditId.value ? `/tenants/${tenantId}/social/${socialEditId.value}` : `/tenants/${tenantId}/social`
    const method = socialEditId.value ? 'put' : 'post'
    await request[method](url, socialForm.value)
    ElMessage.success('已保存'); socialEditVisible.value = false; await loadSocial()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { socialEditSaving.value = false }
}
async function toggleSocial(row: any) {
  try {
    await request.put(`/tenants/${tenantId}/social/${row.id}/toggle`)
    ElMessage.success('已切换'); await loadSocial()
  } catch (e: any) { ElMessage.error(e.message) }
}
async function deleteSocial(row: any) {
  try {
    await ElMessageBox.confirm(`删除「${row.socialTypeStr}」配置？`, '确认', { type: 'warning' })
    await request.delete(`/tenants/${tenantId}/social/${row.id}`); ElMessage.success('已删除'); await loadSocial()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
}

// --- security rules ---
async function loadSecRules() {
  secRulesLoading.value = true
  try { secRulesList.value = await request.get('/tenants/security-rules', { params: { tenantId, type: secRulesTab.value } }) as any[] }
  catch { secRulesList.value = [] }
  finally { secRulesLoading.value = false }
}
async function addSecRule() {
  secRuleSaving.value = true
  try {
    await request.post('/tenants/security-rules', { tenantId, type: secRulesTab.value, ...secRuleForm.value })
    ElMessage.success('已添加'); secRuleAddVisible.value = false; await loadSecRules()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { secRuleSaving.value = false }
}

// --- export / delete ---
async function doExport() {
  try {
    const data = await request.get(`/tenants/${tenantId}/export`, { responseType: 'blob' }) as any
    const url = URL.createObjectURL(data); const a = document.createElement('a')
    a.href = url; a.download = `tenant_${tenantId}_export.json`
    document.body.appendChild(a); a.click(); document.body.removeChild(a); URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
  } catch (e: any) { ElMessage.error(e.message) }
}
async function remove() {
  try {
    await ElMessageBox.confirm(`确定删除租户「${tenant.value.userName || tenant.value.tenancyName}」？不可恢复。`, '确认删除', { confirmButtonText: '确定删除', cancelButtonText: '取消', type: 'warning' })
    await request.get('/tenants/deleteApi', { params: { tenantId } })
    ElMessage.success('已删除'); router.push('/tenants')
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
}
</script>

<style scoped>
.tenant-detail-page { padding: 20px; }
.detail-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; flex-wrap: wrap; gap: 12px; }
.header-left { display: flex; align-items: center; gap: 12px; }
.header-left h2 { margin: 0; font-size: var(--text-xl); font-weight: var(--font-bold); }
.header-right { display: flex; align-items: center; gap: 8px; }
.data-mono { font-family: 'JetBrains Mono', monospace; font-size: var(--text-sm); }
.days-chip { display: inline-block; padding: 2px 8px; border-radius: var(--radius-sm); background: var(--bg-raised); font-size: var(--text-sm); font-weight: var(--font-semibold); color: var(--text-primary); }
.state-cell { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); }
:deep(.el-descriptions) { margin-bottom: 16px; }
:deep(.el-table) { border-radius: var(--radius-md); overflow: hidden; }
:deep(.el-table th) { background: var(--bg-raised); font-weight: var(--font-semibold); color: var(--text-primary); }
</style>
