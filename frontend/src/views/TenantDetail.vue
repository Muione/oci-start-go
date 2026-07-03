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
        <el-button size="small" @click="syncOci" :loading="syncing"><el-icon><Connection /></el-icon> 同步 OCI</el-button>
        <el-button size="small" @click="checkTenant" :loading="checking"><el-icon><Monitor /></el-icon> 测试存活</el-button>
      </div>
    </div>
    <el-alert v-if="checkResult" :type="checkResult.alive ? 'success' : 'error'" show-icon style="margin-bottom:12px"
      :title="checkResult.alive ? 'OCI 认证成功' : '异常: ' + (checkResult.error || '未知错误')" />

    <!-- Fold panels -->
    <el-collapse v-model="activePanels" accordion @change="onPanelChange">

      <!-- ======================== 基本信息 ======================== -->
      <el-collapse-item name="overview">
        <template #title>
          <span class="panel-title">基本信息</span>
        </template>
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
            <el-descriptions-item label="订阅天数"><span class="days-chip">{{ tenant.activeDays || '—' }}</span></el-descriptions-item>
            <el-descriptions-item label="主区域">{{ tenant.regionName || tenant.region || '—' }}</el-descriptions-item>
            <el-descriptions-item label="API 同步"><el-tag :type="tenant.apiSynced ? 'success' : 'info'" size="small">{{ tenant.apiSynced ? '已同步' : '未同步' }}</el-tag></el-descriptions-item>
            <el-descriptions-item label="父租户 ID">{{ tenant.parenId || '无 (主租户)' }}</el-descriptions-item>
          </el-descriptions>
          <!-- Domain tenants -->
          <h4 style="margin:16px 0 8px">域内其他租户</h4>
          <el-table v-loading="domainsLoading" :data="domains" border stripe size="small" max-height="240">
            <template #empty><el-empty description="该域下无其他租户" :image-size="40"/></template>
            <el-table-column prop="userName" label="用户名" min-width="120"/>
            <el-table-column prop="tenancyDes" label="自定义名称" min-width="120"/>
            <el-table-column prop="region" label="区域" width="120"/>
            <el-table-column label="状态" width="80" align="center"><template #default="{ row }"><span class="status-dot" :class="row.isActive ? 'status-dot--up' : 'status-dot--down'"/>{{ row.isActive ? '正常' : '停用' }}</template></el-table-column>
          </el-table>
          <!-- Update / Export / Delete -->
          <el-divider/>
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
        </template>
      </el-collapse-item>

      <!-- ======================== 实例列表 ======================== -->
      <el-collapse-item name="instances">
        <template #title>
          <span class="panel-title">实例列表 ({{ instances.length }})</span>
        </template>
        <el-skeleton v-if="instLoading" :rows="5" animated/>
        <el-table v-else :data="instances" border stripe size="small">
          <template #empty><el-empty description="该租户下暂无实例" :image-size="60"/></template>
          <el-table-column prop="displayName" label="名称" min-width="160">
            <template #default="{ row }">
              <el-link type="primary" @click="router.push({ path: '/instances', query: { instanceId: row.instanceId } })">{{ row.displayName }}</el-link>
            </template>
          </el-table-column>
          <el-table-column prop="instanceId" label="实例 ID" min-width="200" show-overflow-tooltip/>
          <el-table-column prop="shape" label="Shape" min-width="140"/>
          <el-table-column label="状态" width="100"><template #default="{ row }"><div class="state-cell"><span class="status-dot" :class="instStateDot(row.state)"/>{{ row.state || '—' }}</div></template></el-table-column>
          <el-table-column prop="publicIps" label="公网 IP" width="140"/>
          <el-table-column prop="architecture" label="架构" width="80"/>
          <el-table-column label="规格" width="120"><template #default="{ row }">{{ row.ocpus || 0 }}C / {{ row.memoryInGbs || 0 }}G</template></el-table-column>
          <el-table-column prop="availabilityDomain" label="AD" min-width="120" show-overflow-tooltip/>
          <el-table-column prop="createTime" label="创建时间" width="160"/>
        </el-table>
      </el-collapse-item>

      <!-- ======================== 费用账单 ======================== -->
      <el-collapse-item name="costs">
        <template #title>
          <span class="panel-title">费用账单</span>
        </template>
        <!-- Subscription days -->
        <el-descriptions :column="3" border size="small" style="margin-bottom:16px">
          <el-descriptions-item label="订阅天数"><span class="days-chip">{{ subscriptionDays || '—' }}</span></el-descriptions-item>
        </el-descriptions>
        <!-- Subscription info -->
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
        <!-- Cost query -->
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
        <!-- Quota -->
        <el-divider/>
        <el-alert v-if="quotaError" type="warning" :closable="false" show-icon style="margin-bottom:8px">
          <template #title>配额加载失败: {{ quotaError }}</template>
          <template #default><el-button size="small" text type="primary" @click="loadQuota">重试</el-button></template>
        </el-alert>
        <h4 style="margin:0 0 8px">配额</h4>
        <div style="margin-bottom:8px;display:flex;gap:8px;align-items:center">
          <el-select v-model="quotaServiceName" size="small" style="width:200px" @change="loadQuota">
            <el-option v-for="s in quotaServices" :key="s.name" :label="s.description || s.name" :value="s.name"/>
          </el-select>
        </div>
        <el-table v-loading="quotaLoading" :data="quotaItems" border stripe size="small" max-height="300">
          <template #empty><el-empty description="暂无配额数据" :image-size="40"/></template>
          <el-table-column prop="name" label="配额名称" min-width="220" show-overflow-tooltip/>
          <el-table-column label="使用率" width="160">
            <template #default="{ row }">
              <el-progress
                v-if="row.total > 0"
                :percentage="Math.min(100, Math.round(row.used / row.total * 100))"
                :color="quotaUsageColor(row.used, row.total)"
                :stroke-width="12"
                :text-inside="true"
              />
              <span v-else style="color: var(--text-secondary); font-size: 12px">无限制/未配置</span>
            </template>
          </el-table-column>
          <el-table-column label="已用" width="90" align="right">
            <template #default="{ row }"><span class="data-mono">{{ row.used }}</span></template>
          </el-table-column>
          <el-table-column label="可用" width="90" align="right">
            <template #default="{ row }"><span class="data-mono" :style="row.available > 0 ? '' : 'color:var(--status-down)'">{{ row.available }}</span></template>
          </el-table-column>
          <el-table-column label="总量" width="90" align="right">
            <template #default="{ row }"><span class="data-mono">{{ row.total || '—' }}</span></template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!quotaServices.length && !quotaLoading" description="该租户无任何配额数据" :image-size="40"/>
      </el-collapse-item>

      <!-- ======================== IAM 用户 ======================== -->
      <el-collapse-item name="users">
        <template #title>
          <span class="panel-title">IAM 用户 ({{ userList.length }})</span>
        </template>
        <div style="margin-bottom:12px;display:flex;gap:8px">
          <el-button type="primary" size="small" @click="addUserFormVisible=true"><el-icon><Plus /></el-icon> 创建用户</el-button>
        </div>
        <el-skeleton v-if="userLoading" :rows="4" animated/>
        <el-table v-else :data="userList" border stripe size="small">
          <template #empty><el-empty description="暂无 IAM 用户" :image-size="60"/></template>
          <el-table-column prop="name" label="用户名" min-width="140"/>
          <el-table-column prop="description" label="描述" min-width="160" show-overflow-tooltip/>
          <el-table-column label="状态" width="90" align="center"><template #default="{ row }"><el-tag :type="row.lifecycleState==='ACTIVE'?'success':'warning'" size="small">{{ row.lifecycleState || '—' }}</el-tag></template></el-table-column>
          <el-table-column prop="email" label="邮箱" min-width="180" show-overflow-tooltip/>
          <el-table-column label="操作" width="180" align="center">
            <template #default="{ row }">
              <el-button size="small" @click="resetUserPassword(row)">重置密码</el-button>
              <el-button size="small" type="danger" @click="deleteUser(row)"><el-icon><Delete /></el-icon></el-button>
            </template>
          </el-table-column>
        </el-table>
        <!-- User groups -->
        <h4 style="margin:16px 0 8px">用户组</h4>
        <el-table v-loading="groupsLoading" :data="groups" border stripe size="small" max-height="200">
          <template #empty><el-empty description="暂无用户组" :image-size="40"/></template>
          <el-table-column prop="name" label="组名" min-width="200"/>
          <el-table-column prop="ocid" label="OCID" min-width="300" show-overflow-tooltip/>
        </el-table>
        <!-- Password policy -->
        <h4 style="margin:16px 0 8px">密码策略</h4>
        <el-form :inline="true" size="small" style="margin-bottom:12px">
          <el-form-item label="密码过期">
            <el-switch v-model="pwPolicy.isPasswordExpiryEnabled" @change="savePasswordPolicy"/>
          </el-form-item>
          <el-form-item label="过期天数" v-if="pwPolicy.isPasswordExpiryEnabled">
            <el-input-number v-model="pwPolicy.passwordExpiryDays" :min="1" :max="365" @change="savePasswordPolicy"/>
          </el-form-item>
        </el-form>
        <!-- Add user dialog -->
        <el-dialog v-model="addUserFormVisible" title="创建 IAM 用户" width="460px" append-to-body destroy-on-close>
          <el-form :model="addUserForm" label-width="80px">
            <el-form-item label="用户名" required><el-input v-model="addUserForm.username" placeholder="IAM 用户名"/></el-form-item>
            <el-form-item label="邮箱" required><el-input v-model="addUserForm.email" placeholder="用户邮箱"/></el-form-item>
            <el-form-item label="用户组">
              <el-select v-model="addUserForm.groupName" placeholder="选择用户组（可选）" clearable>
                <el-option v-for="g in groups" :key="g.ocid" :label="g.name" :value="g.name"/>
              </el-select>
            </el-form-item>
          </el-form>
          <el-alert v-if="createdUserPwd" title="用户创建成功！请复制以下一次性密码" type="success" :closable="true" show-icon @close="createdUserPwd=''" style="margin-top:12px">
            <template #default><code style="user-select:all">{{ createdUserPwd }}</code></template>
          </el-alert>
          <template #footer>
            <el-button @click="addUserFormVisible=false">关闭</el-button>
            <el-button type="primary" :loading="addUserSaving" @click="createUser">创建</el-button>
          </template>
        </el-dialog>
        <!-- API Key add dialog -->
        <el-dialog v-model="apiKeyAddVisible" title="添加 API 密钥" width="560px" append-to-body destroy-on-close>
          <el-form label-width="100px">
            <el-form-item label="公钥 PEM" required><el-input v-model="apiKeyPem" type="textarea" :rows="6" placeholder="-----BEGIN PUBLIC KEY-----&#10;...&#10;-----END PUBLIC KEY-----"/></el-form-item>
          </el-form>
          <template #footer><el-button @click="apiKeyAddVisible=false">取消</el-button><el-button type="primary" :loading="credSaving" @click="createApiKey">添加</el-button></template>
        </el-dialog>
        <!-- Auth Token add dialog -->
        <el-dialog v-model="authTokenAddVisible" title="创建 Auth 令牌" width="460px" append-to-body destroy-on-close>
          <el-form label-width="80px"><el-form-item label="描述" required><el-input v-model="authTokenDesc" placeholder="令牌用途描述"/></el-form-item></el-form>
          <el-alert v-if="createdToken" title="令牌已创建！请立即复制（仅显示一次）" type="success" :closable="true" show-icon @close="createdToken=''" style="margin-top:12px"><template #default><code style="user-select:all">{{ createdToken }}</code></template></el-alert>
          <template #footer><el-button @click="authTokenAddVisible=false">关闭</el-button><el-button type="primary" :loading="credSaving" @click="createAuthToken" :disabled="!!createdToken">创建</el-button></template>
        </el-dialog>
        <!-- SMTP add dialog -->
        <el-dialog v-model="smtpAddVisible" title="创建 SMTP 凭证" width="460px" append-to-body destroy-on-close>
          <el-form label-width="80px"><el-form-item label="描述" required><el-input v-model="smtpDesc" placeholder="凭证用途描述"/></el-form-item></el-form>
          <el-alert v-if="createdSmtpPassword" title="SMTP 密码已创建！请立即复制（仅显示一次）" type="success" :closable="true" show-icon @close="createdSmtpPassword=''" style="margin-top:12px"><template #default><code style="user-select:all">{{ createdSmtpPassword }}</code></template></el-alert>
          <template #footer><el-button @click="smtpAddVisible=false">关闭</el-button><el-button type="primary" :loading="credSaving" @click="createSmtpCred" :disabled="!!createdSmtpPassword">创建</el-button></template>
        </el-dialog>
        <!-- Secret Key add dialog -->
        <el-dialog v-model="secretKeyAddVisible" title="创建 Customer Secret Key" width="460px" append-to-body destroy-on-close>
          <el-form label-width="100px"><el-form-item label="显示名称" required><el-input v-model="secretKeyDisplay" placeholder="密钥显示名称"/></el-form-item></el-form>
          <el-alert v-if="createdSecretKey" title="Secret Key 已创建！请立即复制（仅显示一次）" type="success" :closable="true" show-icon @close="createdSecretKey=''" style="margin-top:12px"><template #default><code style="user-select:all">{{ createdSecretKey }}</code></template></el-alert>
          <template #footer><el-button @click="secretKeyAddVisible=false">关闭</el-button><el-button type="primary" :loading="credSaving" @click="createSecretKey" :disabled="!!createdSecretKey">创建</el-button></template>
        </el-dialog>
      </el-collapse-item>

      <!-- ======================== 安全与合规 ======================== -->
      <el-collapse-item name="security">
        <template #title>
          <span class="panel-title">安全与合规</span>
        </template>

        <!-- 凭证管理 -->
        <div class="section-block">
          <h4 class="section-title">凭证管理</h4>
          <div class="credential-user-select">
            <span class="select-label">选择 IAM 用户:</span>
            <el-select v-model="credUserOcid" filterable placeholder="选择用户" @change="loadCredentials" style="width: 300px" size="small">
              <el-option v-for="u in userList" :key="u.ocid" :label="u.name" :value="u.ocid"/>
            </el-select>
          </div>

          <template v-if="credUserOcid">
            <!-- API Keys -->
            <h5 class="subsection-title">API 密钥</h5>
            <el-table :data="apiKeys" v-loading="credLoading" border stripe size="small" max-height="200">
              <template #empty><el-empty description="暂无 API 密钥" :image-size="40"/></template>
              <el-table-column prop="fingerprint" label="指纹" min-width="200" show-overflow-tooltip/>
              <el-table-column label="操作" width="80" align="center">
                <template #default="{ row }">
                  <el-button size="small" type="danger" text @click="deleteApiKey(row)">
                    <el-icon><Delete/></el-icon>
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
            <el-button size="small" type="primary" @click="apiKeyAddVisible=true" style="margin-top:8px">
              <el-icon><Plus/></el-icon> 添加 API 密钥
            </el-button>

            <!-- Auth Tokens -->
            <h5 class="subsection-title">Auth 令牌</h5>
            <el-table :data="authTokens" v-loading="credLoading" border stripe size="small" max-height="200">
              <template #empty><el-empty description="暂无 Auth 令牌" :image-size="40"/></template>
              <el-table-column prop="description" label="描述" min-width="200"/>
              <el-table-column label="操作" width="80" align="center">
                <template #default="{ row }">
                  <el-button size="small" type="danger" text @click="deleteAuthToken(row)">
                    <el-icon><Delete/></el-icon>
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
            <el-button size="small" type="primary" @click="authTokenAddVisible=true" style="margin-top:8px">
              <el-icon><Plus/></el-icon> 创建 Auth 令牌
            </el-button>

            <!-- SMTP Credentials -->
            <h5 class="subsection-title">SMTP 凭证</h5>
            <el-table :data="smtpCreds" v-loading="credLoading" border stripe size="small" max-height="200">
              <template #empty><el-empty description="暂无 SMTP 凭证" :image-size="40"/></template>
              <el-table-column prop="description" label="描述" min-width="200"/>
              <el-table-column prop="username" label="用户名" min-width="160" show-overflow-tooltip/>
              <el-table-column label="操作" width="80" align="center">
                <template #default="{ row }">
                  <el-button size="small" type="danger" text @click="deleteSmtpCred(row)">
                    <el-icon><Delete/></el-icon>
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
            <el-button size="small" type="primary" @click="smtpAddVisible=true" style="margin-top:8px">
              <el-icon><Plus/></el-icon> 创建 SMTP 凭证
            </el-button>

            <!-- Customer Secret Keys -->
            <h5 class="subsection-title">Customer Secret Keys</h5>
            <el-table :data="customerSecretKeys" v-loading="credLoading" border stripe size="small" max-height="200">
              <template #empty><el-empty description="暂无 Customer Secret Key" :image-size="40"/></template>
              <el-table-column prop="displayName" label="显示名称" min-width="200"/>
              <el-table-column label="操作" width="80" align="center">
                <template #default="{ row }">
                  <el-button size="small" type="danger" text @click="deleteSecretKey(row)">
                    <el-icon><Delete/></el-icon>
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
            <el-button size="small" type="primary" @click="secretKeyAddVisible=true" style="margin-top:8px">
              <el-icon><Plus/></el-icon> 创建 Customer Secret Key
            </el-button>
          </template>
        </div>

        <!-- Security Rules -->
        <h4 class="section-title">安全规则</h4>
        <el-tabs v-model="secRulesTab" @tab-change="loadSecRules">
          <el-tab-pane label="入站规则" name="ingress"/>
          <el-tab-pane label="出站规则" name="egress"/>
        </el-tabs>
        <div style="margin-bottom:12px;display:flex;gap:8px">
          <el-button type="primary" size="small" @click="secRuleAddVisible=true"><el-icon><Plus /></el-icon> 添加规则</el-button>
          <el-button size="small" @click="batchEnableSecRules"><el-icon><Connection /></el-icon> 批量启用全部</el-button>
        </div>
        <el-skeleton v-if="secRulesLoading" :rows="4" animated/>
        <el-table v-else :data="secRulesList" border stripe size="small">
          <template #empty><el-empty description="暂无安全规则" :image-size="60"/></template>
          <el-table-column prop="protocol" label="协议" width="80"/>
          <el-table-column label="端口范围" width="120"><template #default="{ row }">{{ row.tcpOptions?.destinationPortRange?.min || row.udpOptions?.destinationPortRange?.min || '全部' }}{{ row.tcpOptions?.destinationPortRange?.min !== row.tcpOptions?.destinationPortRange?.max ? '-' + (row.tcpOptions?.destinationPortRange?.max || '') : '' }}</template></el-table-column>
          <el-table-column prop="source" label="源地址" min-width="160" show-overflow-tooltip/>
          <el-table-column prop="description" label="描述" min-width="160" show-overflow-tooltip/>
          <el-table-column label="操作" width="80" align="center">
            <template #default="{ row }">
              <el-button size="small" type="danger" @click="deleteSecRule(row)"><el-icon><Delete /></el-icon></el-button>
            </template>
          </el-table-column>
        </el-table>
        <!-- Social Login -->
        <el-divider/>
        <h4 class="section-title">社交登录</h4>
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
        <!-- Email Config -->
        <el-divider/>
        <h4 class="section-title">邮件服务</h4>
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
        <div style="display:flex;gap:8px">
          <el-button size="small" type="success" :loading="emailEnabling" @click="enableEmail">启用邮件服务（OCI + DNS）</el-button>
          <el-button size="small" type="warning" :loading="emailDisabling" @click="disableEmail">禁用邮件服务</el-button>
        </div>
        <p style="color:var(--text-secondary);font-size:12px;margin-top:8px">启用：配置 OCI Email Delivery 域名 + Cloudflare DNS 记录。禁用：拆除 OCI 邮件资源 + DNS 记录。</p>
        <!-- MFA -->
        <el-divider/>
        <el-alert v-if="mfaError" type="warning" :closable="false" show-icon style="margin-bottom:8px">
          <template #title>MFA 状态加载失败: {{ mfaError }}</template>
          <template #default><el-button size="small" text type="primary" @click="loadMfaStatus">重试</el-button></template>
        </el-alert>
        <h4 class="section-title">MFA 多因素认证</h4>
        <el-descriptions :column="2" border size="small" style="margin-bottom:12px">
          <el-descriptions-item label="TOTP"><el-tag :type="mfaStatus.totpEnabled?'success':'info'" size="small">{{ mfaStatus.totpEnabled ? '已启用' : '未启用' }}</el-tag></el-descriptions-item>
          <el-descriptions-item label="邮箱"><el-tag :type="mfaStatus.emailEnabled?'success':'info'" size="small">{{ mfaStatus.emailEnabled ? '已启用' : '未启用' }}</el-tag></el-descriptions-item>
          <el-descriptions-item label="短信"><el-tag :type="mfaStatus.smsEnabled?'success':'info'" size="small">{{ mfaStatus.smsEnabled ? '已启用' : '未启用' }}</el-tag></el-descriptions-item>
          <el-descriptions-item label="安全问题"><el-tag :type="mfaStatus.securityQuestionsEnabled?'success':'info'" size="small">{{ mfaStatus.securityQuestionsEnabled ? '已启用' : '未启用' }}</el-tag></el-descriptions-item>
        </el-descriptions>
        <div style="display:flex;gap:8px;margin-bottom:16px">
          <el-button size="small" type="primary" @click="toggleMfa(true)" :loading="mfaToggling">启用邮箱 MFA</el-button>
          <el-button size="small" @click="toggleMfa(false)" :loading="mfaToggling">禁用邮箱 MFA</el-button>
          <el-button size="small" type="warning" @click="resetMfa" :loading="mfaResetting">重置 MFA 设备</el-button>
        </div>
        <!-- Sign-on Policies -->
        <el-divider/>
        <h4 class="section-title">登录策略</h4>
        <el-alert v-if="signonError" type="warning" :closable="false" show-icon style="margin-bottom:8px"><template #title>加载失败: {{ signonError }}</template><template #default><el-button size="small" text type="primary" @click="loadSignonPolicies">重试</el-button></template></el-alert>
        <el-table v-loading="signonLoading" :data="signonPolicies" border stripe size="small" max-height="200">
          <template #empty><el-empty description="暂无登录策略" :image-size="40"/></template>
          <el-table-column prop="name" label="名称" min-width="160"/>
          <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip/>
          <el-table-column label="状态" width="80" align="center"><template #default="{ row }"><el-tag :type="row.active?'success':'info'" size="small">{{ row.active ? '启用' : '禁用' }}</el-tag></template></el-table-column>
        </el-table>
        <!-- Account Recovery -->
        <el-divider/>
        <h4 class="section-title">账号恢复设置</h4>
        <el-alert v-if="recoveryError" type="warning" :closable="false" show-icon style="margin-bottom:8px"><template #title>加载失败: {{ recoveryError }}</template><template #default><el-button size="small" text type="primary" @click="loadAccountRecovery">重试</el-button></template></el-alert>
        <el-form v-loading="recoveryLoading" :inline="true" size="small" style="margin-bottom:8px">
          <el-form-item label="恢复方式">
            <el-checkbox-group v-model="recoveryFactors">
              <el-checkbox label="EMAIL">邮箱</el-checkbox>
              <el-checkbox label="SMS">短信</el-checkbox>
              <el-checkbox label="SECURITY_QUESTIONS">安全问题</el-checkbox>
              <el-checkbox label="PUSH">推送</el-checkbox>
              <el-checkbox label="TOTP">TOTP</el-checkbox>
            </el-checkbox-group>
          </el-form-item>
          <el-form-item><el-button type="primary" size="small" :loading="recoverySaving" @click="updateAccountRecovery">保存</el-button></el-form-item>
        </el-form>
        <!-- Notification Recipients -->
        <el-divider/>
        <el-alert v-if="notifError" type="warning" :closable="false" show-icon style="margin-bottom:8px">
          <template #title>通知接收人加载失败: {{ notifError }}</template>
          <template #default><el-button size="small" text type="primary" @click="loadNotifRecipients">重试</el-button></template>
        </el-alert>
        <h4 class="section-title">通知接收人</h4>
        <el-table :data="notifRecipients" border size="small" style="margin-bottom:8px" max-height="200">
          <template #empty><el-empty description="暂无通知接收人" :image-size="40"/></template>
          <el-table-column prop="email" label="邮箱" min-width="200"/>
          <el-table-column prop="state" label="状态" width="100"><template #default="{ row }"><el-tag :type="row.state==='VERIFIED'?'success':'warning'" size="small">{{ row.state || '—' }}</el-tag></template></el-table-column>
          <el-table-column label="操作" width="80" align="center">
            <template #default="{ row }">
              <el-button size="small" type="danger" text @click="deleteRecipient(row.email)"><el-icon><Delete /></el-icon></el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-input v-model="notifEmailInput" placeholder="输入邮箱地址，多个用逗号分隔" size="small" style="max-width:400px;margin-right:8px"/>
        <el-button size="small" type="primary" @click="updateNotifRecipients" :loading="notifSaving">更新接收人</el-button>
        <!-- Audit Log -->
        <el-divider/>
        <el-alert v-if="auditError" type="warning" :closable="false" show-icon style="margin-bottom:8px">
          <template #title>审计日志加载失败: {{ auditError }}</template>
          <template #default><el-button size="small" text type="primary" @click="loadAudit(auditDays || 1)">重试</el-button></template>
        </el-alert>
        <h4 class="section-title">审计日志</h4>
        <div style="display:flex;gap:8px;align-items:center;margin-bottom:12px;flex-wrap:wrap">
          <el-button size="small" :type="auditDays===1?'primary':''" @click="loadAudit(1)">今日</el-button>
          <el-button size="small" :type="auditDays===7?'primary':''" @click="loadAudit(7)">7 天</el-button>
          <el-button size="small" :type="auditDays===30?'primary':''" @click="loadAudit(30)">30 天</el-button>
          <el-date-picker v-model="auditDateRange" type="daterange" range-separator="至" start-placeholder="开始" end-placeholder="结束" format="YYYY-MM-DD" value-format="YYYY-MM-DD" size="small" style="width:280px"/>
          <el-button size="small" type="primary" @click="loadAuditCustom" :loading="auditLoading"><el-icon><Search /></el-icon> 查询</el-button>
        </div>
        <el-table v-loading="auditLoading" :data="auditLogs" border stripe size="small" max-height="400" show-overflow-tooltip>
          <template #empty><el-empty description="暂无审计日志" :image-size="40"/></template>
          <el-table-column prop="eventTime" label="时间" width="170"/>
          <el-table-column prop="eventType" label="事件" min-width="200"/>
          <el-table-column prop="userName" label="用户" width="140"/>
          <el-table-column prop="userType" label="类型" width="80"/>
          <el-table-column prop="ipAddress" label="IP" width="130"/>
          <el-table-column prop="responseStatus" label="状态" width="80"/>
        </el-table>
        <!-- Security Rules Dialog -->
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
        <!-- Social Login Dialog -->
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
      </el-collapse-item>

      <!-- ======================== 区域管理 ======================== -->
      <el-collapse-item name="regions">
        <template #title>
          <span class="panel-title">区域管理</span>
        </template>
        <!-- Summary -->
        <el-descriptions v-if="regionSummary" :column="3" border size="small" style="margin-bottom:16px">
          <el-descriptions-item label="总区域">{{ regionSummary.totalRegions }}</el-descriptions-item>
          <el-descriptions-item label="已订阅">{{ regionSummary.subscribedRegions }}</el-descriptions-item>
          <el-descriptions-item label="未订阅">{{ regionSummary.unsubscribedRegions }}</el-descriptions-item>
        </el-descriptions>
        <el-tabs v-model="regionSubTab">
          <el-tab-pane label="已订阅" name="subscribed">
            <el-skeleton v-if="regionsLoading" :rows="4" animated/>
            <el-table v-else :data="subscribedRegions" border stripe size="small">
              <template #empty><el-empty description="无已订阅区域" :image-size="40"/></template>
              <el-table-column prop="regionKey" label="区域代码" min-width="160"/>
              <el-table-column prop="regionName" label="区域名称" min-width="160"/>
              <el-table-column prop="status" label="状态" width="100"><template #default="{ row }"><el-tag :type="row.status==='READY'?'success':'warning'" size="small">{{ row.status || '—' }}</el-tag></template></el-table-column>
              <el-table-column label="主区域" width="80" align="center"><template #default="{ row }">{{ row.isHomeRegion ? '是' : '否' }}</template></el-table-column>
            </el-table>
          </el-tab-pane>
          <el-tab-pane label="可订阅" name="unsubscribed">
            <div style="margin-bottom:12px;display:flex;gap:8px;align-items:center">
              <el-button type="primary" size="small" :disabled="!selectedRegions.length" :loading="subscribing" @click="subscribeRegions">
                <el-icon><Plus /></el-icon> 订阅选中 ({{ selectedRegions.length }})
              </el-button>
            </div>
            <el-skeleton v-if="regionsLoading" :rows="4" animated/>
            <el-table v-else :data="unsubscribedRegions" border stripe size="small" @selection-change="(rows: any[]) => { selectedRegions = rows.map(r => r.key) }">
              <template #empty><el-empty description="无可订阅区域" :image-size="40"/></template>
              <el-table-column type="selection" width="50"/>
              <el-table-column prop="key" label="区域代码" min-width="160"/>
              <el-table-column prop="name" label="区域名称" min-width="160"/>
              <el-table-column prop="cnName" label="中文名" min-width="120"/>
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </el-collapse-item>

    </el-collapse>

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
import { ref, computed, onMounted } from 'vue'
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
const activePanels = ref('overview')
const loading = ref(false)
const tenant = ref<any>({})
const syncing = ref(false)
const checking = ref(false)
const checkResult = ref<any>(null)

// overview: edit name / cost / domains
const editNameVisible = ref(false)
const editNameValue = ref('')
const editCostVisible = ref(false)
const editCostValue = ref('')
const editSaving = ref(false)
const domains = ref<any[]>([])
const domainsLoading = ref(false)
const subscriptionDays = ref('')

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
const costTotal = computed(() => costData.value.reduce((s, i) => s + (Number(i.computedAmount) || 0), 0))

// users
const userLoading = ref(false)
const userList = ref<any[]>([])
const addUserFormVisible = ref(false)
const addUserSaving = ref(false)
const addUserForm = ref({ username: '', email: '', groupName: '' })
const createdUserPwd = ref('')
const groups = ref<any[]>([])
const groupsLoading = ref(false)
const pwPolicy = ref({ isPasswordExpiryEnabled: false, passwordExpiryDays: 90 })

// email
const emailForm = ref({ domainName: '', smtpHost: '', smtpPort: '587', smtpUsername: '', smtpPassword: '', senderEmail: '', active: false })
const emailSaving = ref(false)
const emailConfigId = ref<number | null>(null)
const emailEnabling = ref(false)
const emailDisabling = ref(false)

// social
const socialList = ref<any[]>([])
const socialLoading = ref(false)
const socialTypes = ['GITHUB', 'GOOGLE']
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

// MFA
const mfaStatus = ref<any>({ totpEnabled: false, emailEnabled: false, smsEnabled: false, securityQuestionsEnabled: false })
const mfaToggling = ref(false)
const mfaResetting = ref(false)

// notification recipients
const notifRecipients = ref<any[]>([])
const notifEmailInput = ref('')
const notifSaving = ref(false)

// quota
const quotaItems = ref<any[]>([])
const quotaLoading = ref(false)
const quotaServices = ref<any[]>([])
const quotaServiceName = ref('compute')

// audit log
const auditLogs = ref<any[]>([])
const auditLoading = ref(false)
const auditDays = ref(1)
const auditDateRange = ref<string[] | null>(null)

// regions
const regionSummary = ref<any>(null)
const subscribedRegions = ref<any[]>([])
const unsubscribedRegions = ref<any[]>([])
const regionsLoading = ref(false)
const subscribing = ref(false)
const selectedRegions = ref<string[]>([])
const regionSubTab = ref('subscribed')

const instLoaded = ref(false)
const userLoaded = ref(false)
const socialLoaded = ref(false)
const costLoaded = ref(false)
const domainsLoaded = ref(false)
const settingsLoaded = ref(false)

const mfaError = ref('')
const notifError = ref('')
const auditError = ref('')
const quotaError = ref('')

// credentials tab
const credUserOcid = ref('')
const credLoading = ref(false)
const credSaving = ref(false)
const apiKeys = ref<any[]>([])
const authTokens = ref<any[]>([])
const smtpCreds = ref<any[]>([])
const customerSecretKeys = ref<any[]>([])
const apiKeyAddVisible = ref(false)
const apiKeyPem = ref('')
const authTokenAddVisible = ref(false)
const authTokenDesc = ref('')
const createdToken = ref('')
const smtpAddVisible = ref(false)
const smtpDesc = ref('')
const createdSmtpPassword = ref('')
const secretKeyAddVisible = ref(false)
const secretKeyDisplay = ref('')
const createdSecretKey = ref('')
const credLoaded = ref(false)

// signon + recovery (settings tab)
const signonPolicies = ref<any[]>([])
const signonLoading = ref(false)
const signonError = ref('')
const recoveryFactors = ref<string[]>([])
const recoveryLoading = ref(false)
const recoverySaving = ref(false)
const recoveryError = ref('')

// --- lifecycle ---
onMounted(async () => {
  loading.value = true
  try {
    tenant.value = await request.get(`/tenants/${tenantId}`)
  } catch (e: any) { ElMessage.error('加载租户失败: ' + e.message) }
  finally { loading.value = false }
})

function onPanelChange(name: string | number | (string | number)[]) {
  const panel = Array.isArray(name) ? String(name[0] || '') : String(name || '')
  if (!panel) return
  if (panel === 'overview' && !domainsLoaded.value) loadDomains()
  if (panel === 'instances' && !instLoaded.value) loadInstances()
  if (panel === 'costs') {
    if (!costLoaded.value) loadSubscription().then(() => { loadCost(); loadSubscriptionDays() })
    if (!quotaServices.value.length) loadQuotaServices().then(() => loadQuota())
  }
  if (panel === 'users') {
    if (!userLoaded.value) {
      loadUsers()
      loadGroups()
      loadPasswordPolicy()
    }
  }
  if (panel === 'security') {
    loadEmail()
    if (!socialLoaded.value) loadSocial()
    if (!secRulesList.value.length) loadSecRules()
    if (!settingsLoaded.value) {
      loadMfaStatus()
      loadNotifRecipients()
      loadSignonPolicies()
      loadAccountRecovery()
      settingsLoaded.value = true
    }
    // Load credentials when security panel opens (user selector lives here now)
    if (!userLoaded.value) {
      loadUsers().then(() => {
        if (userList.value.length) {
          credUserOcid.value = userList.value[0].ocid
          loadCredentials()
        }
      })
    } else {
      if (userList.value.length && !credLoaded.value) {
        if (!credUserOcid.value) credUserOcid.value = userList.value[0].ocid
        loadCredentials()
      }
    }
  }
  if (panel === 'regions') loadRegions()
}

// --- overview actions ---
async function syncOci() {
  try {
    await ElMessageBox.confirm(`从 OCI 同步租户 ${tenant.value.userName} 的全部信息（实例、订阅、账号）？`, '确认同步', { type: 'info' })
    syncing.value = true
    await request.get('/tenants/syncOci', { params: { tenantId } })
    ElMessage.success('同步完成')
    // Refresh overview (email, subscription days) + already-loaded tabs so
    // synced data shows without a manual page reload. The backend syncOci now
    // syncs instances + tenancy detail + subscription in one call.
    tenant.value = await request.get(`/tenants/${tenantId}`)
    subscriptionDays.value = tenant.value?.activeDays || '—'
    if (costLoaded.value) await loadSubscription()
    if (instLoaded.value) await loadInstances()
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
    await request.post(`/tenants/${tenantId}/account-cost`, { cost: editCostValue.value })
    tenant.value.accountCost = editCostValue.value
    editCostVisible.value = false
    ElMessage.success('已更新')
  } catch (e: any) { ElMessage.error(e.message) }
  finally { editSaving.value = false }
}
async function loadDomains() {
  domainsLoading.value = true
  try { domains.value = await request.get(`/tenants/${tenantId}/domains`) as any[] }
  catch { domains.value = [] }
  finally { domainsLoading.value = false; domainsLoaded.value = true }
}

// --- instances ---
async function loadInstances() {
  instLoading.value = true
  try { instances.value = await request.get(`/tenants/${tenantId}/instances`) as any[] }
  catch (e: any) { ElMessage.error(e.message) }
  finally { instLoading.value = false; instLoaded.value = true }
}

// --- costs ---
async function loadSubscription() {
  try { subscriptionData.value = await request.get(`/tenants/${tenantId}/subscription`) }
  catch { subscriptionData.value = null }
}
async function loadSubscriptionDays() {
  // activeDays is computed server-side from register_detail.register_time
  // (same source as the list page) — subscription days === active days.
  subscriptionDays.value = tenant.value?.activeDays || '—'
}
async function loadCost(type?: string, start?: string, end?: string) {
  const qType = type || costQueryType.value; costQueryType.value = qType; costLoading.value = true; costData.value = []
  try {
    const params: any = { type: qType }
    if (qType === 'custom' && start && end) { params.start = start; params.end = end }
    const resp = await request.get(`/tenants/${tenantId}/cost`, { params }) as any[]
    costData.value = resp || []
  } catch (e: any) { ElMessage.error('费用查询失败: ' + (e?.message || e)) }
  finally { costLoading.value = false; costLoaded.value = true }
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
  finally { userLoading.value = false; userLoaded.value = true }
}
async function createUser() {
  if (!addUserForm.value.username || !addUserForm.value.email) {
    ElMessage.warning('请填写用户名和邮箱')
    return
  }
  addUserSaving.value = true
  createdUserPwd.value = ''
  try {
    const r: any = await request.post(`/tenants/${tenantId}/users`, addUserForm.value)
    createdUserPwd.value = r?.password || ''
    ElMessage.success('用户已创建')
    await loadUsers()
    addUserForm.value = { username: '', email: '', groupName: '' }
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
async function loadGroups() {
  groupsLoading.value = true
  try { groups.value = await request.get(`/tenants/${tenantId}/groups`) as any[] }
  catch { groups.value = [] }
  finally { groupsLoading.value = false }
}
async function loadPasswordPolicy() {
  try {
    const r: any = await request.get(`/tenants/${tenantId}/password-policy`)
    pwPolicy.value = { isPasswordExpiryEnabled: r?.isPasswordExpiryEnabled ?? false, passwordExpiryDays: r?.passwordExpiryDays ?? 90 }
  } catch { /* no policy */ }
}
async function savePasswordPolicy() {
  try {
    await request.post(`/tenants/${tenantId}/password-policy`, { enableExpiry: pwPolicy.value.isPasswordExpiryEnabled, expiryDays: pwPolicy.value.passwordExpiryDays })
    ElMessage.success('密码策略已更新')
  } catch (e: any) { ElMessage.error(e.message) }
}

// --- email ---
async function loadEmail() {
  try {
    const cfg: any = await request.get(`/tenants/${tenantId}/email`)
    emailConfigId.value = cfg?.id ?? null
    emailForm.value = { domainName: cfg?.domainName || '', smtpHost: cfg?.smtpHost || '', smtpPort: cfg?.smtpPort || '587', smtpUsername: cfg?.smtpUsername || '', smtpPassword: cfg?.smtpPassword || '', senderEmail: cfg?.senderEmail || '', active: cfg?.active === true || cfg?.active === 1 }
  } catch { emailConfigId.value = null }
}
async function saveEmail() {
  emailSaving.value = true
  try { await request.post(`/tenants/${tenantId}/email`, emailForm.value); ElMessage.success('邮件配置已保存') }
  catch (e: any) { ElMessage.error(e.message) }
  finally { emailSaving.value = false }
}
async function deleteEmail() {
  try { await request.delete(`/tenants/${tenantId}/email`); ElMessage.success('已删除'); emailForm.value = { domainName: '', smtpHost: '', smtpPort: '587', smtpUsername: '', smtpPassword: '', senderEmail: '', active: false }; emailConfigId.value = null }
  catch (e: any) { ElMessage.error(e.message) }
}
async function enableEmail() {
  if (!emailForm.value.domainName) { ElMessage.warning('请先填写域名'); return }
  emailEnabling.value = true
  try { await request.post('/api/email/enable', { tenantId, domainName: emailForm.value.domainName }); ElMessage.success('邮件服务已启用') }
  catch (e: any) { ElMessage.error(e.message) }
  finally { emailEnabling.value = false }
}
async function disableEmail() {
  if (!emailConfigId.value) { ElMessage.warning('未找到邮件配置 ID'); return }
  emailDisabling.value = true
  try { await request.post('/api/email/disable', { tenantEmailConfigId: emailConfigId.value }); ElMessage.success('邮件服务已禁用') }
  catch (e: any) { ElMessage.error(e.message) }
  finally { emailDisabling.value = false }
}

// --- social ---
async function loadSocial() {
  socialLoading.value = true
  try { socialList.value = await request.get(`/tenants/${tenantId}/social`) as any[] }
  catch { socialList.value = [] }
  finally { socialLoading.value = false; socialLoaded.value = true }
}
function openAddSocial() { socialEditId.value = ''; socialForm.value = { socialType: 'GITHUB', clientId: '', clientSecret: '', redirectUrl: '', loginUrl: '' }; socialEditVisible.value = true }
function editSocial(row: any) { socialEditId.value = row.id; socialForm.value = { socialType: row.socialTypeStr || 'GITHUB', clientId: row.clientId || '', clientSecret: '', redirectUrl: row.redirectUrl || '', loginUrl: row.loginUrl || '' }; socialEditVisible.value = true }
async function saveSocial() {
  socialEditSaving.value = true
  try {
    const payload = socialEditId.value
      ? { ...socialForm.value, id: socialEditId.value }
      : socialForm.value
    await request.post(`/tenants/${tenantId}/social`, payload)
    ElMessage.success('已保存')
    socialEditVisible.value = false
    await loadSocial()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { socialEditSaving.value = false }
}
async function toggleSocial(row: any) {
  try { await request.put(`/tenants/${tenantId}/social/${row.id}/toggle`); ElMessage.success('已切换'); await loadSocial() }
  catch (e: any) { ElMessage.error(e.message) }
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
async function deleteSecRule(row: any) {
  try {
    await ElMessageBox.confirm('确定删除此安全规则？', '确认', { type: 'warning' })
    await request.delete(`/tenants/security-rules/${row.id}`)
    ElMessage.success('已删除'); await loadSecRules()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
}
async function batchEnableSecRules() {
  try {
    await ElMessageBox.confirm('将启用所有安全规则，确定？', '确认', { type: 'info' })
    await request.post('/tenants/enableAll', { tenantId })
    ElMessage.success('已启用全部安全规则')
  } catch (e: any) { ElMessage.error(e.message) }
}

// --- MFA ---
async function loadMfaStatus() {
  mfaError.value = ''
  try { mfaStatus.value = await request.get(`/tenants/${tenantId}/mfa/status`) }
  catch (e: any) { mfaError.value = e.message || '加载失败' }
}
async function toggleMfa(enable: boolean) {
  mfaToggling.value = true
  try {
    await request.post(`/tenants/${tenantId}/mfa/toggle`, { enable })
    ElMessage.success(enable ? '已启用邮箱 MFA' : '已禁用邮箱 MFA')
    await loadMfaStatus()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { mfaToggling.value = false }
}
async function resetMfa() {
  try {
    await ElMessageBox.confirm('将重置所有 MFA 设备，确定？', '确认', { type: 'warning' })
    mfaResetting.value = true
    const r: any = await request.post(`/tenants/${tenantId}/mfa/reset`)
    ElMessage.success(`已重置 ${r?.deletedDevices || 0} 个 MFA 设备`)
    await loadMfaStatus()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
  finally { mfaResetting.value = false }
}

// --- notification recipients ---
async function loadNotifRecipients() {
  notifError.value = ''
  try { notifRecipients.value = await request.get(`/tenants/${tenantId}/notification-recipients`) as any[] }
  catch (e: any) { notifError.value = e.message || '加载失败'; notifRecipients.value = [] }
}
async function updateNotifRecipients() {
  const emails = notifEmailInput.value.split(',').map(e => e.trim()).filter(Boolean)
  if (!emails.length) { ElMessage.warning('请输入至少一个邮箱'); return }
  notifSaving.value = true
  try {
    await request.post(`/tenants/${tenantId}/notification-recipients/update`, { emails })
    ElMessage.success('已更新'); await loadNotifRecipients(); notifEmailInput.value = ''
  } catch (e: any) { ElMessage.error(e.message) }
  finally { notifSaving.value = false }
}
async function deleteRecipient(email: string) {
  try {
    await ElMessageBox.confirm(`确定删除接收人「${email}」？`, '确认', { type: 'warning' })
    const remaining = notifRecipients.value
      .map(r => r.email)
      .filter(e => e !== email)
    notifSaving.value = true
    await request.post(`/tenants/${tenantId}/notification-recipients/update`, { emails: remaining })
    ElMessage.success('已删除')
    await loadNotifRecipients()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
  finally { notifSaving.value = false }
}

// --- quota ---
async function loadQuota() {
  if (!quotaServiceName.value || !quotaServices.value.find(s => s.name === quotaServiceName.value)) {
    quotaItems.value = []
    return
  }
  quotaLoading.value = true; quotaError.value = ''
  try {
    const r: any = await request.get(`/tenants/${tenantId}/quota`, {
      params: { serviceName: quotaServiceName.value, pageSize: 100 }
    })
    quotaItems.value = r?.items || []
  } catch (e: any) { quotaError.value = e.message || '加载失败'; quotaItems.value = [] }
  finally { quotaLoading.value = false }
}

function quotaUsageColor(used: number, total: number): string {
  if (total <= 0) return '#909399'
  const pct = used / total * 100
  if (pct >= 90) return '#f56c6c'   // red
  if (pct >= 70) return '#e6a23c'   // orange
  return '#67c23a'                   // green
}

async function loadQuotaServices() {
  quotaLoading.value = true
  try {
    quotaServices.value = await request.get(`/tenants/${tenantId}/quota/services`) as any[]
    // Auto-select first available service if current selection not in list
    if (quotaServices.value.length && !quotaServices.value.find(s => s.name === quotaServiceName.value)) {
      quotaServiceName.value = quotaServices.value[0].name
    }
  } catch { quotaServices.value = [] }
  finally { quotaLoading.value = false }
}

// --- audit log ---
async function loadAudit(days: number) {
  auditDays.value = days; auditDateRange.value = null; auditLoading.value = true; auditError.value = ''
  try {
    const r: any = await request.post(`/tenants/${tenantId}/audit-log`, { days })
    auditLogs.value = r?.data || []
  } catch (e: any) { auditError.value = e.message || '加载失败'; auditLogs.value = [] }
  finally { auditLoading.value = false }
}
async function loadAuditCustom() {
  if (!auditDateRange.value || auditDateRange.value.length !== 2) { ElMessage.warning('请选择日期范围'); return }
  auditDays.value = 0; auditLoading.value = true; auditError.value = ''
  try {
    const r: any = await request.post(`/tenants/${tenantId}/audit-log`, { startDate: auditDateRange.value[0], endDate: auditDateRange.value[1] })
    auditLogs.value = r?.data || []
  } catch (e: any) { auditError.value = e.message || '加载失败'; auditLogs.value = [] }
  finally { auditLoading.value = false }
}

// --- regions ---
async function loadRegions() {
  regionsLoading.value = true
  try {
    const [summary, sub, unsub] = await Promise.all([
      request.get(`/tenants/${tenantId}/regions/summary`),
      request.get(`/tenants/${tenantId}/regions/subscribed`) as Promise<any[]>,
      request.get(`/tenants/${tenantId}/regions/unsubscribed`) as Promise<any[]>,
    ])
    regionSummary.value = summary
    subscribedRegions.value = sub || []
    unsubscribedRegions.value = unsub || []
  } catch { regionSummary.value = null; subscribedRegions.value = []; unsubscribedRegions.value = [] }
  finally { regionsLoading.value = false }
}
async function subscribeRegions() {
  if (!selectedRegions.value.length) return
  subscribing.value = true
  try {
    const r: any = await request.post(`/tenants/${tenantId}/regions/subscribe`, { regionKeys: selectedRegions.value })
    const ok = r?.details?.filter((d: any) => d.success).length || 0
    const fail = r?.details?.filter((d: any) => !d.success).length || 0
    if (fail) ElMessage.warning(`订阅完成: ${ok} 成功, ${fail} 失败`)
    else ElMessage.success(`已订阅 ${ok} 个区域`)
    selectedRegions.value = []
    await loadRegions()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { subscribing.value = false }
}

// --- credentials ---
async function loadCredentials() {
  if (!credUserOcid.value) return
  credLoading.value = true
  const uo = encodeURIComponent(credUserOcid.value)
  try {
    const [keys, tokens, smtp, csks] = await Promise.all([
      request.get(`/tenants/${tenantId}/users/${uo}/api-keys`),
      request.get(`/tenants/${tenantId}/users/${uo}/auth-tokens`),
      request.get(`/tenants/${tenantId}/users/${uo}/smtp-credentials`),
      request.get(`/tenants/${tenantId}/users/${uo}/customer-secret-keys`),
    ])
    apiKeys.value = (keys as unknown as any[]) || []
    authTokens.value = (tokens as unknown as any[]) || []
    smtpCreds.value = (smtp as unknown as any[]) || []
    customerSecretKeys.value = (csks as unknown as any[]) || []
    credLoaded.value = true
  } catch (e: any) { ElMessage.error('加载凭证失败: ' + e.message) }
  finally { credLoading.value = false }
}

async function createApiKey() {
  if (!apiKeyPem.value.trim()) { ElMessage.warning('请粘贴公钥 PEM'); return }
  credSaving.value = true
  const uo = encodeURIComponent(credUserOcid.value)
  try {
    await request.post(`/tenants/${tenantId}/users/${uo}/api-keys`, { key: apiKeyPem.value })
    ElMessage.success('已添加'); apiKeyAddVisible.value = false; apiKeyPem.value = ''; await loadCredentials()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { credSaving.value = false }
}
async function deleteApiKey(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除此 API 密钥（${row.fingerprint?.slice(0,16)}...）？`, '确认', { type: 'warning' })
    const uo = encodeURIComponent(credUserOcid.value)
    await request.delete(`/tenants/${tenantId}/users/${uo}/api-keys/${encodeURIComponent(row.id)}`)
    ElMessage.success('已删除'); await loadCredentials()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
}

async function createAuthToken() {
  if (!authTokenDesc.value.trim()) { ElMessage.warning('请输入描述'); return }
  credSaving.value = true; createdToken.value = ''
  const uo = encodeURIComponent(credUserOcid.value)
  try {
    const r: any = await request.post(`/tenants/${tenantId}/users/${uo}/auth-tokens`, { description: authTokenDesc.value })
    createdToken.value = r?.token || ''
    ElMessage.success('已创建'); await loadCredentials()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { credSaving.value = false }
}
async function deleteAuthToken(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除令牌「${row.description}」？`, '确认', { type: 'warning' })
    const uo = encodeURIComponent(credUserOcid.value)
    await request.delete(`/tenants/${tenantId}/users/${uo}/auth-tokens/${encodeURIComponent(row.id)}`)
    ElMessage.success('已删除'); await loadCredentials()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
}

async function createSmtpCred() {
  if (!smtpDesc.value.trim()) { ElMessage.warning('请输入描述'); return }
  credSaving.value = true; createdSmtpPassword.value = ''
  const uo = encodeURIComponent(credUserOcid.value)
  try {
    const r: any = await request.post(`/tenants/${tenantId}/users/${uo}/smtp-credentials`, { description: smtpDesc.value })
    createdSmtpPassword.value = r?.password || ''
    ElMessage.success('已创建'); await loadCredentials()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { credSaving.value = false }
}
async function deleteSmtpCred(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除 SMTP 凭证「${row.description || row.username}」？`, '确认', { type: 'warning' })
    const uo = encodeURIComponent(credUserOcid.value)
    await request.delete(`/tenants/${tenantId}/users/${uo}/smtp-credentials/${encodeURIComponent(row.id)}`)
    ElMessage.success('已删除'); await loadCredentials()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
}

async function createSecretKey() {
  if (!secretKeyDisplay.value.trim()) { ElMessage.warning('请输入显示名称'); return }
  credSaving.value = true; createdSecretKey.value = ''
  const uo = encodeURIComponent(credUserOcid.value)
  try {
    const r: any = await request.post(`/tenants/${tenantId}/users/${uo}/customer-secret-keys`, { displayName: secretKeyDisplay.value })
    createdSecretKey.value = r?.secretKey || ''
    ElMessage.success('已创建'); await loadCredentials()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { credSaving.value = false }
}
async function deleteSecretKey(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除密钥「${row.displayName}」？`, '确认', { type: 'warning' })
    const uo = encodeURIComponent(credUserOcid.value)
    await request.delete(`/tenants/${tenantId}/users/${uo}/customer-secret-keys/${encodeURIComponent(row.id)}`)
    ElMessage.success('已删除'); await loadCredentials()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
}

// --- signon + recovery ---
async function loadSignonPolicies() {
  signonError.value = ''; signonLoading.value = true
  try { signonPolicies.value = await request.get(`/tenants/${tenantId}/signon-policies`) as any[] }
  catch (e: any) { signonError.value = e.message || '加载失败'; signonPolicies.value = [] }
  finally { signonLoading.value = false }
}

async function loadAccountRecovery() {
  recoveryError.value = ''; recoveryLoading.value = true
  try {
    const r: any = await request.get(`/tenants/${tenantId}/account-recovery`)
    recoveryFactors.value = r?.factors || []
  } catch (e: any) { recoveryError.value = e.message || '加载失败'; recoveryFactors.value = [] }
  finally { recoveryLoading.value = false }
}
async function updateAccountRecovery() {
  recoverySaving.value = true
  try {
    await request.put(`/tenants/${tenantId}/account-recovery`, { factors: recoveryFactors.value })
    ElMessage.success('已更新'); await loadAccountRecovery()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { recoverySaving.value = false }
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
.panel-title { font-size: var(--text-md); font-weight: var(--font-semibold); color: var(--text-primary); }
:deep(.el-collapse-item__header) { font-size: var(--text-md); font-weight: var(--font-semibold); }
:deep(.el-collapse-item__content) { padding-bottom: var(--space-4); }
.section-block { margin-bottom: var(--space-6); }
.section-title { font-size: var(--text-md); font-weight: var(--font-semibold); color: var(--text-primary); margin: 0 0 var(--space-4) 0; padding-bottom: var(--space-2); border-bottom: 1px solid var(--border-subtle); }
.subsection-title { font-size: var(--text-sm); font-weight: var(--font-semibold); color: var(--text-secondary); margin: var(--space-4) 0 var(--space-2) 0; }
.credential-user-select { display: flex; align-items: center; gap: var(--space-3); margin-bottom: var(--space-4); }
.select-label { font-size: var(--text-sm); color: var(--text-secondary); font-weight: var(--font-medium); }
</style>
