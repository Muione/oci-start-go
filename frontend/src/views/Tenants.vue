<template>
  <div class="tenants-page">
    <!-- ================================================================ -->
    <!-- Toolbar -->
    <!-- ================================================================ -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>租户管理</h2>
        <el-tag type="info" size="small">{{ rows.length }} 个租户</el-tag>
        <el-input
          v-model="searchText"
          placeholder="搜索租户名称..."
          size="small"
          clearable
          style="width: 200px"
          :prefix-icon="Search"
        />
      </div>
      <div class="toolbar-right">
        <el-button type="primary" @click="openAdd">
          <el-icon><Plus /></el-icon> 新增租户
        </el-button>
        <el-button @click="startBatchCheck" :disabled="rows.length === 0">
          <el-icon><Connection /></el-icon> 批量检测
        </el-button>
        <el-button @click="load" :loading="loading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </div>
    </div>

    <!-- ================================================================ -->
    <!-- Table -->
    <!-- ================================================================ -->
    <el-card shadow="none" class="table-card">
      <el-table :data="filteredRows" v-loading="loading" border stripe style="width: 100%">
        <template #empty>
          <el-empty description="暂无租户，请新增" :image-size="80">
            <el-button type="primary" @click="openAdd">新增租户</el-button>
          </el-empty>
        </template>
        <el-table-column type="index" label="#" width="50" align="center"/>
        <el-table-column label="租户名" min-width="110">
          <template #default="{ row }">
            <span class="spoiler-link" @click="showName = showName === row.id ? 0 : row.id">
              <template v-if="showName === row.id">{{ row.tenancyName || row.userName }}</template>
              <template v-else>
                {{ maskedName(row.tenancyName || row.userName) }}
              </template>
            </span>
          </template>
        </el-table-column>
        <el-table-column label="自定义名称" min-width="120">
          <template #default="{ row }">
            <span
              class="cell-edit-link"
              @click="openEditCustomName(row)"
              :title="row.tenancyDes || '点击设置'"
            >{{ row.tenancyDes || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="账号成本" width="100" align="center">
          <template #default="{ row }">
            <span class="cell-edit-link data-mono" @click="openEditCost(row)">
              {{ row.accountCost || '—' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="存活天数" width="90" align="center">
          <template #default="{ row }">
            <span class="days-chip">{{ row.activeDays || '0' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="开机任务" width="100" align="center">
          <template #default="{ row }">
            <span class="status-badge" :class="row.hasBootTask ? 'status-running' : 'status-idle'">
              <el-icon v-if="row.hasBootTask" class="is-loading" :size="10"><Operation /></el-icon>
              {{ row.hasBootTask ? '有任务' : '无任务' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="主区域" width="100">
          <template #default="{ row }">
            <span class="data-mono">{{ row.regionName || row.region || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="多区" width="60" align="center">
          <template #default="{ row }">
            <span :class="row.hasChildren ? 'home-region-badge is-home' : 'home-region-badge not-home'">
              {{ row.hasChildren ? '是' : '否' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="账号类型" width="110">
          <template #default="{ row }">
            <el-tag
              :type="accountTypeTag(row.accountType)"
              size="small"
              effect="dark"
            >{{ accountTypeLabel(row.accountType) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="150">
          <template #default="{ row }">
            <span class="data-mono" style="font-size:12px">{{ row.createdAt || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <span class="status-dot" :class="row.isActive ? 'status-dot--up status-dot--pulse' : 'status-dot--down'"></span>
            {{ row.isActive ? '正常' : '停用' }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="60" fixed="right" align="center">
          <template #default="{ row }">
            <el-dropdown trigger="click" @command="(cmd: string) => handleAction(cmd, row)">
              <el-button size="small" circle>
                <el-icon><MoreFilled /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="detail">
                    <el-icon><InfoFilled /></el-icon> 租户详情
                  </el-dropdown-item>
                  <el-dropdown-item command="update">
                    <el-icon><Edit /></el-icon> 账号更新
                  </el-dropdown-item>
                  <el-dropdown-item command="check" divided>
                    <el-icon><Connection /></el-icon> 测试存活
                  </el-dropdown-item>
                  <el-dropdown-item command="boot">
                    <el-icon><VideoPlay /></el-icon> 开机任务
                  </el-dropdown-item>
                  <el-dropdown-item command="instances">
                    <el-icon><Monitor /></el-icon> 实例列表
                  </el-dropdown-item>
                  <el-dropdown-item command="securityRules" divided>
                    <el-icon><Warning /></el-icon> 安全规则
                  </el-dropdown-item>
                  <el-dropdown-item command="quota">
                    <el-icon><DataLine /></el-icon> 配额查看
                  </el-dropdown-item>
                  <el-dropdown-item command="regionSub">
                    <el-icon><Location /></el-icon> 区域订阅
                  </el-dropdown-item>
                  <el-dropdown-item command="auditLog">
                    <el-icon><Document /></el-icon> 审计日志
                  </el-dropdown-item>
                  <el-dropdown-item command="trafficAlert" divided>
                    <el-icon><Warning /></el-icon> 流量预警
                  </el-dropdown-item>
                  <el-dropdown-item command="trafficQuery">
                    <el-icon><DataAnalysis /></el-icon> 流量查询
                  </el-dropdown-item>
                  <el-dropdown-item command="users" divided>
                    <el-icon><User /></el-icon> 用户管理
                  </el-dropdown-item>
                  <el-dropdown-item command="email">
                    <el-icon><Message /></el-icon> 邮箱服务
                  </el-dropdown-item>
                  <el-dropdown-item command="social">
                    <el-icon><Share /></el-icon> 社媒配置
                  </el-dropdown-item>
                  <el-dropdown-item command="export" divided>
                    <el-icon><Download /></el-icon> 导出租户
                  </el-dropdown-item>
                  <el-dropdown-item command="delete" divided style="color:var(--status-down)">
                    <el-icon><Delete /></el-icon> 删除租户
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- ================================================================ -->
    <!-- Add Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="addVisible" title="新增租户" width="660px" destroy-on-close>
      <el-collapse v-model="configCollapse" style="margin-bottom:16px">
        <el-collapse-item title="从 OCI Config 文件导入" name="config">
          <el-alert title="粘贴 OCI CLI 配置文件内容，自动填写下方表单" type="info" :closable="false" show-icon style="margin-bottom:12px"/>
          <el-input v-model="ociConfigText" type="textarea" :rows="6" placeholder="[DEFAULT]&#10;user=ocid1.user.oc1..xxx&#10;fingerprint=xx:xx:xx&#10;tenancy=ocid1.tenancy.oc1..xxx&#10;region=ap-singapore-2"/>
          <div style="margin-top:12px;display:flex;gap:8px">
            <el-button type="primary" size="small" @click="parseOciConfig" :disabled="!ociConfigText.trim()">解析并填写</el-button>
            <el-button size="small" @click="ociConfigText='';configCollapse=[]">清空</el-button>
          </div>
        </el-collapse-item>
      </el-collapse>
      <el-form :model="form" label-width="120px">
        <el-form-item label="Tenancy OCID" required>
          <el-input v-model="form.tenancy" placeholder="ocid1.tenancy.oc1..xxxxx"/>
        </el-form-item>
        <el-form-item label="User OCID" required>
          <el-input v-model="form.tenantId" placeholder="ocid1.user.oc1..xxxxx"/>
        </el-form-item>
        <el-form-item label="指纹" required>
          <el-input v-model="form.fingerprint" placeholder="3a:37:17:38:xx:xx:xx"/>
        </el-form-item>
        <el-form-item label="区域" required>
          <el-select v-model="form.region" filterable allow-create placeholder="选择或输入区域">
            <el-option v-for="r in regions" :key="r.code" :label="`${r.name} (${r.code})`" :value="r.name"/>
          </el-select>
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model="form.userName" placeholder="留空自动生成"/>
        </el-form-item>
        <el-form-item label="API 私钥" required>
          <input ref="keyFile" type="file" @change="onFile" style="display:block"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addVisible=false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Detail Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="detailVisible" :title="`租户详情 — ${detailData.userName || '#'+detailData.id}`" width="720px" destroy-on-close>
      <template v-if="detailLoading">
        <el-skeleton :rows="8" animated/>
      </template>
      <template v-else>
        <el-tabs v-model="detailTab">
          <el-tab-pane label="基本信息" name="basic">
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="租户 ID">{{ detailData.id }}</el-descriptions-item>
              <el-descriptions-item label="用户名">{{ detailData.userName }}</el-descriptions-item>
              <el-descriptions-item label="Tenancy OCID" :span="2">
                <span class="data-mono">{{ detailData.tenancy }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="User OCID" :span="2">
                <span class="data-mono">{{ detailData.tenantId }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="指纹" :span="2">
                <span class="data-mono">{{ detailData.fingerprint }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="区域">{{ detailData.regionName || detailData.region }}</el-descriptions-item>
              <el-descriptions-item label="区域代码">{{ detailData.region }}</el-descriptions-item>
              <el-descriptions-item label="账号类型">
                <el-tag :type="accountTypeTag(detailData.accountType)" size="small">{{ detailData.accountType || '—' }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="云厂商">
                <el-tag :type="detailData.cloudType===1?'':'warning'" size="small">{{ cloudTypeLabel(detailData.cloudType) }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="自定义名称">{{ detailData.tenancyDes || '—' }}</el-descriptions-item>
              <el-descriptions-item label="邮箱地址">{{ detailData.emailAddress || '—' }}</el-descriptions-item>
              <el-descriptions-item label="创建时间">{{ detailData.createdAt }}</el-descriptions-item>
            </el-descriptions>
          </el-tab-pane>
          <el-tab-pane label="状态信息" name="status">
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="活跃状态">
                <el-tag :type="detailData.isActive?'success':'danger'" size="small">{{ detailData.isActive ? '正常' : '停用' }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="API 已同步">
                <el-tag :type="detailData.apiSynced?'success':'info'" size="small">{{ detailData.apiSynced ? '已同步' : '未同步' }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="邮箱已启用">
                <el-tag :type="detailData.emailEnable?'success':'info'" size="small">{{ detailData.emailEnable ? '是' : '否' }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="主区域">
                {{ detailData.regionName || detailData.region || '—' }}
              </el-descriptions-item>
              <el-descriptions-item label="开启 ICMP">{{ detailData.enableIcmp ? '是' : '否' }}</el-descriptions-item>
              <el-descriptions-item label="所有协议">{{ detailData.enableAllProtocol ? '是' : '否' }}</el-descriptions-item>
              <el-descriptions-item label="转移状态">{{ detailData.transferStatus || '—' }}</el-descriptions-item>
              <el-descriptions-item label="转移金额">{{ detailData.transferAmount || '—' }}</el-descriptions-item>
            </el-descriptions>
          </el-tab-pane>
          <el-tab-pane label="备注与同步" name="sync">
            <el-descriptions :column="1" border size="small">
              <el-descriptions-item label="租户备注">{{ detailData.tenancyDes || '无' }}</el-descriptions-item>
              <el-descriptions-item label="Region EN">{{ detailData.regionEn || '—' }}</el-descriptions-item>
              <el-descriptions-item label="ID String">{{ detailData.idStr || '—' }}</el-descriptions-item>
              <el-descriptions-item label="父租户 ID">{{ detailData.parenId || '无 (主租户)' }}</el-descriptions-item>
            </el-descriptions>
            <div style="margin-top:12px;display:flex;gap:8px">
              <el-button size="small" @click="syncOci(detailData)" :loading="syncing">
                <el-icon><Refresh /></el-icon> 同步 OCI 实例
              </el-button>
              <el-button size="small" @click="checkTenant(detailData.id)" :loading="checking">
                <el-icon><Connection /></el-icon> 测试存活
              </el-button>
            </div>
            <div v-if="checkResult !== null" style="margin-top:8px">
              <el-alert
                :title="checkResult.alive ? '账号存活 — OCI 认证成功' : '账号异常 — ' + checkResult.error"
                :type="checkResult.alive ? 'success' : 'error'"
                :closable="false"
                show-icon
              />
            </div>
          </el-tab-pane>
        </el-tabs>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Edit Custom Name Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="editNameVisible" title="设置自定义名称" width="460px" destroy-on-close>
      <el-form label-width="100px">
        <el-form-item label="租户名">
          <el-input :model-value="editTarget?.userName || editTarget?.tenancyName" disabled/>
        </el-form-item>
        <el-form-item label="自定义名称">
          <el-input v-model="editNameValue" placeholder="输入自定义名称"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editNameVisible=false">取消</el-button>
        <el-button type="primary" :loading="editSaving" @click="saveCustomName">保存</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Edit Account Cost Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="editCostVisible" title="设置账号成本" width="460px" destroy-on-close>
      <el-form label-width="100px">
        <el-form-item label="租户名">
          <el-input :model-value="editTarget?.userName || editTarget?.tenancyName" disabled/>
        </el-form-item>
        <el-form-item label="账号成本">
          <el-input v-model="editCostValue" placeholder="例如: $29.99/月"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editCostVisible=false">取消</el-button>
        <el-button type="primary" :loading="editSaving" @click="saveAccountCost">保存</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Account Update Dialog (auto-fetch from OCI) -->
    <!-- ================================================================ -->
    <el-dialog v-model="updateVisible" title="从 OCI 获取租户信息" width="520px" destroy-on-close>
      <el-alert title="将从 Oracle Cloud 自动获取租户的 Tenancy Name、账号类型、邮箱地址等信息" type="info" :closable="false" show-icon style="margin-bottom:16px"/>
      <p style="color:var(--text-secondary);font-size:var(--text-sm)">租户: <strong>{{ updateTarget?.userName || updateTarget?.tenancyName || `#${updateTarget?.id}` }}</strong></p>
      <div v-if="updateResult" style="margin-top:12px">
        <el-descriptions :column="1" border size="small">
          <el-descriptions-item label="Tenancy Name">{{ updateResult.tenancyName || '—' }}</el-descriptions-item>
          <el-descriptions-item label="账号类型">
            <el-tag :type="accountTypeTag(updateResult.accountType)" size="small">{{ accountTypeLabel(updateResult.accountType) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="邮箱地址">{{ updateResult.emailAddress || '—' }}</el-descriptions-item>
          <el-descriptions-item label="描述">{{ updateResult.description || '—' }}</el-descriptions-item>
        </el-descriptions>
      </div>
      <div v-if="updateError" style="margin-top:8px">
        <el-alert :title="updateError" type="error" :closable="false" show-icon/>
      </div>
      <template #footer>
        <el-button @click="updateVisible=false">关闭</el-button>
        <el-button type="primary" :loading="updateSaving" @click="doUpdateDetail">从 OCI 获取</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- User Management Dialog (3 tabs) -->
    <!-- ================================================================ -->
    <el-dialog v-model="userMgmtVisible" :title="`用户管理 — ${userMgmtTenantName}`" width="900px" destroy-on-close>
      <el-tabs v-model="userMgmtTab">
        <!-- Tab 1: User List -->
        <el-tab-pane label="用户列表" name="users">
          <div style="margin-bottom:12px;display:flex;gap:8px">
            <el-button type="success" size="small" @click="showAddUserForm">
              <el-icon><Plus /></el-icon> 添加用户
            </el-button>
            <el-button type="primary" size="small" @click="refreshUserList">
              <el-icon><Refresh /></el-icon> 刷新列表
            </el-button>
            <el-button size="small" @click="showPasswordPolicyDialog">
              <el-icon><Key /></el-icon> 密码策略
            </el-button>
          </div>

          <!-- Add User Form -->
          <el-card v-if="addUserFormVisible" shadow="none" style="margin-bottom:12px;border:1px solid var(--border-default)">
            <template #header>新建 IAM 用户</template>
            <el-form :model="addUserForm" label-width="100px" size="small">
              <el-form-item label="用户名" required>
                <el-input v-model="addUserForm.username" placeholder="IAM 用户名"/>
              </el-form-item>
              <el-form-item label="邮箱" required>
                <el-input v-model="addUserForm.email" placeholder="user@example.com"/>
              </el-form-item>
              <el-form-item label="用户组">
                <el-select v-model="addUserForm.groupName" filterable allow-create placeholder="选择或输入用户组" style="width:100%">
                  <el-option v-for="g in userGroups" :key="g.ocid" :label="g.name" :value="g.name"/>
                </el-select>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" :loading="addUserSaving" @click="doAddUser">创建用户</el-button>
                <el-button @click="addUserFormVisible=false">取消</el-button>
              </el-form-item>
            </el-form>
          </el-card>

          <!-- Created user password display -->
          <el-alert v-if="createdUserPwd" title="用户创建成功！请复制以下一次性密码" type="success" :closable="true" show-icon @close="createdUserPwd=''">
            <template #default>
              <div style="margin-top:4px">
                <span class="data-mono" style="font-size:14px">{{ createdUserPwd }}</span>
                <el-button size="small" style="margin-left:8px" @click="copyText(createdUserPwd)">复制</el-button>
              </div>
              <p style="color:var(--text-secondary);font-size:12px;margin-top:4px">该密码仅在首次登录时有效，请妥善保存</p>
            </template>
          </el-alert>

          <el-table :data="userList" border stripe size="small" v-loading="userListLoading" max-height="360">
            <template #empty><el-empty description="暂无 IAM 用户" :image-size="60"/></template>
            <el-table-column prop="domain" label="所属域" width="80"/>
            <el-table-column prop="name" label="用户名" min-width="120" show-overflow-tooltip/>
            <el-table-column prop="email" label="邮箱地址" min-width="160" show-overflow-tooltip/>
            <el-table-column label="账号状态" width="90" align="center">
              <template #default="{ row }">
                <el-tag :type="row.lifecycleState==='ACTIVE'?'success':'info'" size="small">
                  {{ row.lifecycleState === 'ACTIVE' ? 'Active' : row.lifecycleState || '—' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="创建时间" width="150">
              <template #default="{ row }">{{ formatTime(row.timeCreated) }}</template>
            </el-table-column>
            <el-table-column label="最后登录" width="150">
              <template #default="{ row }">{{ formatTime(row.lastSuccessfulLoginTime) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="140" align="center" fixed="right">
              <template #default="{ row }">
                <el-button size="small" @click="resetUserPassword(row)">重置密码</el-button>
                <el-button size="small" type="danger" @click="deleteUser(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <!-- Tab 2: Notification Email -->
        <el-tab-pane label="通知邮箱" name="notifications">
          <div style="margin-bottom:12px;display:flex;gap:8px">
            <el-button type="success" size="small" @click="showAddNotifEmailForm = true">
              <el-icon><Plus /></el-icon> 添加邮箱
            </el-button>
            <el-button type="primary" size="small" @click="refreshNotifRecipients">
              <el-icon><Refresh /></el-icon> 刷新
            </el-button>
          </div>
          <el-card v-if="showAddNotifEmailForm" shadow="none" style="margin-bottom:12px;border:1px solid var(--border-default)">
            <el-form :model="addNotifEmailForm" label-width="80px" size="small" inline>
              <el-form-item label="邮箱地址" required>
                <el-input v-model="addNotifEmailForm.email" placeholder="admin@example.com"/>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" :loading="notifSaving" @click="doAddNotifEmail">添加</el-button>
                <el-button @click="showAddNotifEmailForm=false">取消</el-button>
              </el-form-item>
            </el-form>
          </el-card>
          <el-table :data="notifRecipients" border stripe size="small" v-loading="notifLoading">
            <template #empty><el-empty description="暂无通知邮箱" :image-size="60"/></template>
            <el-table-column type="index" label="#" width="50" align="center"/>
            <el-table-column prop="email" label="邮箱地址" min-width="200"/>
            <el-table-column prop="state" label="状态" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="row.state==='ACTIVE'?'success':'info'" size="small">{{ row.state || '—' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="80" align="center">
              <template #default="{ row }">
                <el-button size="small" type="danger" @click="deleteNotifEmail(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div style="margin-top:12px;padding:8px;background:var(--bg-raised);border-radius:var(--radius-sm);font-size:12px;color:var(--text-secondary)">
            共 <strong>{{ notifRecipients.length }}</strong> 个收件人
          </div>
        </el-tab-pane>

        <!-- Tab 3: MFA Management -->
        <el-tab-pane label="MFA管理" name="mfa">
          <div style="margin-bottom:12px;display:flex;gap:8px">
            <el-button type="warning" size="small" @click="resetMfa">
              <el-icon><Key /></el-icon> 重置MFA
            </el-button>
            <el-button type="success" size="small" @click="toggleEmailMfa(true)" :disabled="mfaStatus?.emailEnabled">
              <el-icon><Message /></el-icon> 启用邮箱MFA
            </el-button>
            <el-button type="danger" size="small" @click="toggleEmailMfa(false)" :disabled="!mfaStatus?.emailEnabled">
              <el-icon><Message /></el-icon> 禁用邮箱MFA
            </el-button>
            <el-button type="primary" size="small" @click="refreshMfaStatus">
              <el-icon><Refresh /></el-icon> 刷新
            </el-button>
          </div>
          <div style="padding:16px;background:var(--bg-raised);border-radius:var(--radius-md)">
            <template v-if="mfaLoading">
              <el-skeleton :rows="3" animated/>
            </template>
            <template v-else>
              <el-descriptions :column="2" border size="small">
                <el-descriptions-item label="TOTP 认证">
                  <el-tag :type="mfaStatus?.totpEnabled?'success':'info'" size="small">{{ mfaStatus?.totpEnabled ? '已启用' : '未启用' }}</el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="邮箱 MFA">
                  <el-tag :type="mfaStatus?.emailEnabled?'success':'info'" size="small">{{ mfaStatus?.emailEnabled ? '已启用' : '未启用' }}</el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="短信 MFA">
                  <el-tag :type="mfaStatus?.smsEnabled?'success':'info'" size="small">{{ mfaStatus?.smsEnabled ? '已启用' : '未启用' }}</el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="安全提问">
                  <el-tag :type="mfaStatus?.securityQuestionsEnabled?'success':'info'" size="small">{{ mfaStatus?.securityQuestionsEnabled ? '已启用' : '未启用' }}</el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="推送通知">
                  <el-tag :type="mfaStatus?.pushEnabled?'success':'info'" size="small">{{ mfaStatus?.pushEnabled ? '已启用' : '未启用' }}</el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="FIDO 认证">
                  <el-tag :type="mfaStatus?.fidoAuthenticatorEnabled?'success':'info'" size="small">{{ mfaStatus?.fidoAuthenticatorEnabled ? '已启用' : '未启用' }}</el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="电话呼叫">
                  <el-tag :type="mfaStatus?.phoneCallEnabled?'success':'info'" size="small">{{ mfaStatus?.phoneCallEnabled ? '已启用' : '未启用' }}</el-tag>
                </el-descriptions-item>
              </el-descriptions>
            </template>
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Password Policy Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="passwordPolicyVisible" title="密码策略配置" width="500px" destroy-on-close>
      <el-form :model="passwordPolicyForm" label-width="120px">
        <el-form-item label="启用密码过期">
          <el-switch v-model="passwordPolicyForm.enableExpiry" @change="onPwdExpiryToggle"/>
        </el-form-item>
        <el-form-item v-if="passwordPolicyForm.enableExpiry" label="过期天数">
          <el-input-number v-model="passwordPolicyForm.expiryDays" :min="0" :max="365" style="width:100%"/>
        </el-form-item>
      </el-form>
      <el-alert title="说明" type="info" :closable="false" show-icon style="margin-top:12px">
        <ul style="margin:0;padding-left:16px;font-size:12px;color:var(--text-secondary)">
          <li>启用后，用户密码将在指定天数后过期</li>
          <li>设置为 0 表示密码永不过期</li>
          <li>默认过期天数为 120 天</li>
          <li>密码过期后用户需要在下次登录时修改密码</li>
        </ul>
      </el-alert>
      <template #footer>
        <el-button @click="passwordPolicyVisible=false">取消</el-button>
        <el-button type="primary" :loading="passwordPolicySaving" @click="savePasswordPolicy">保存</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Traffic Alert Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="trafficVisible" title="流量预警设置" width="480px" destroy-on-close>
      <el-alert title="当流量超过阈值时，自动触发告警通知" type="warning" :closable="false" show-icon style="margin-bottom:16px"/>
      <el-form :model="trafficForm" label-width="120px">
        <el-form-item label="启用流量统计">
          <el-switch v-model="trafficForm.statisticsEnabled" @change="onTrafficToggle"/>
        </el-form-item>
        <template v-if="trafficForm.statisticsEnabled">
          <el-form-item label="流量阈值 (GB)">
            <el-input-number v-model="trafficForm.threshold" :min="0" :step="10" style="width:100%"/>
          </el-form-item>
          <el-form-item label="自动关机">
            <el-switch v-model="trafficForm.autoShutdown" active-text="超过阈值自动关机"/>
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="trafficVisible=false">取消</el-button>
        <el-button type="primary" :loading="trafficSaving" @click="saveTrafficAlert">保存</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Email Service Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="emailVisible" title="邮箱服务配置" width="520px" destroy-on-close>
      <el-alert title="配置 SMTP 邮箱服务，用于发送邮件通知" type="info" :closable="false" show-icon style="margin-bottom:16px"/>
      <el-form :model="emailForm" label-width="120px">
        <el-form-item label="域名">
          <el-input v-model="emailForm.domainName" placeholder="example.com"/>
        </el-form-item>
        <el-form-item label="SMTP 服务器">
          <el-input v-model="emailForm.smtpHost" placeholder="smtp.example.com"/>
        </el-form-item>
        <el-form-item label="SMTP 端口">
          <el-input v-model="emailForm.smtpPort" placeholder="587"/>
        </el-form-item>
        <el-form-item label="SMTP 用户名">
          <el-input v-model="emailForm.smtpUsername" placeholder="username"/>
        </el-form-item>
        <el-form-item label="SMTP 密码">
          <el-input v-model="emailForm.smtpPassword" type="password" show-password placeholder="password"/>
        </el-form-item>
        <el-form-item label="发件邮箱">
          <el-input v-model="emailForm.senderEmail" placeholder="noreply@example.com"/>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="emailForm.active"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="emailVisible=false">取消</el-button>
        <el-button type="primary" :loading="emailSaving" @click="saveEmail">保存</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Social Config Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="socialVisible" title="社媒登录配置" width="600px" destroy-on-close>
      <el-alert title="配置第三方 OAuth 登录 (Google, GitHub, Microsoft 等)" type="info" :closable="false" show-icon style="margin-bottom:16px"/>
      <div style="margin-bottom:12px;display:flex;gap:8px">
        <el-select v-model="socialType" placeholder="选择社交平台" size="small" style="width:160px">
          <el-option v-for="t in socialTypes" :key="t" :label="t" :value="t"/>
        </el-select>
        <el-button type="primary" size="small" @click="openAddSocial">添加配置</el-button>
      </div>
      <el-table :data="socialList" border size="small" v-loading="socialLoading">
        <template #empty><el-empty description="暂无社媒配置" :image-size="40"/></template>
        <el-table-column prop="socialTypeStr" label="平台" width="100"/>
        <el-table-column prop="clientId" label="Client ID" min-width="160" show-overflow-tooltip/>
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.socialStatus==='enabled'?'success':row.socialStatus==='disabled'?'info':'danger'" size="small">
              {{ row.socialStatus || '—' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140" align="center">
          <template #default="{ row }">
            <el-button size="small" @click="editSocial(row)"><el-icon><Edit /></el-icon></el-button>
            <el-button size="small" type="warning" @click="toggleSocial(row)">{{ row.socialStatus==='enabled'?'禁用':'启用' }}</el-button>
            <el-button size="small" type="danger" @click="deleteSocial(row)"><el-icon><Delete /></el-icon></el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- Social Add/Edit sub-dialog -->
      <el-dialog v-model="socialEditVisible" :title="socialEditId ? '编辑社媒配置' : '添加社媒配置'" width="460px" append-to-body destroy-on-close>
        <el-form :model="socialForm" label-width="120px">
          <el-form-item label="平台类型">
            <el-input :model-value="socialForm.socialTypeStr" disabled/>
          </el-form-item>
          <el-form-item label="Client ID" required>
            <el-input v-model="socialForm.clientId" placeholder="OAuth Client ID"/>
          </el-form-item>
          <el-form-item label="Client Secret" required>
            <el-input v-model="socialForm.clientSecret" type="password" show-password placeholder="OAuth Client Secret"/>
          </el-form-item>
          <el-form-item label="回调地址">
            <el-input v-model="socialForm.redirectUrl" placeholder="https://your-domain.com/oauth/callback"/>
          </el-form-item>
          <el-form-item label="登录地址">
            <el-input v-model="socialForm.thirdLoginAddress" placeholder="第三方登录 URL"/>
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="socialEditVisible=false">取消</el-button>
          <el-button type="primary" :loading="socialSaving" @click="saveSocial">保存</el-button>
        </template>
      </el-dialog>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Export Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="exportVisible" title="导出租户数据" width="460px" destroy-on-close>
      <el-alert title="导出该租户及其所有实例数据为 JSON 文件" type="info" :closable="false" show-icon style="margin-bottom:16px"/>
      <p style="color:var(--text-secondary);font-size:var(--text-sm)">租户: <strong>{{ exportTarget?.userName || exportTarget?.tenancyName }}</strong></p>
      <template #footer>
        <el-button @click="exportVisible=false">取消</el-button>
        <el-button type="primary" :loading="exporting" @click="doExport">确认导出</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Batch Check Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="batchCheckVisible" title="批量检测账号存活" width="700px" destroy-on-close>
      <div v-if="batchChecking" style="text-align:center;padding:24px">
        <el-progress :percentage="batchProgress" :stroke-width="20" :text-inside="true"/>
        <p style="margin-top:12px;color:var(--text-secondary)">正在检测账号连通性…</p>
      </div>
      <template v-else-if="batchResults.length > 0">
        <div style="margin-bottom:12px;display:flex;gap:16px">
          <el-statistic title="总计" :value="batchResults.length"/>
          <el-statistic title="存活">
            <template #default>
              <span style="color:var(--status-up)">{{ batchResults.filter(r=>r.alive).length }}</span>
            </template>
          </el-statistic>
          <el-statistic title="异常">
            <template #default>
              <span style="color:var(--status-down)">{{ batchResults.filter(r=>!r.alive).length }}</span>
            </template>
          </el-statistic>
        </div>
        <el-table :data="batchResults" border size="small" max-height="360">
          <el-table-column prop="userName" label="租户" min-width="120"/>
          <el-table-column label="状态" width="100" align="center">
            <template #default="{ row }">
              <el-tag :type="row.alive?'success':'danger'" size="small">{{ row.alive ? '存活' : '异常' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="error" label="错误信息" min-width="200" show-overflow-tooltip/>
        </el-table>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Instances Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="instVisible" :title="`实例列表 — 租户 ${instTenantName}`" width="85%" destroy-on-close>
      <template v-if="instLoading">
        <el-skeleton :rows="5" animated/>
      </template>
      <el-table v-else :data="instances" border stripe size="small">
        <template #empty>
          <el-empty description="该租户下暂无实例" :image-size="60"/>
        </template>
        <el-table-column prop="displayName" label="名称" min-width="160"/>
        <el-table-column prop="instanceId" label="实例ID" min-width="200" show-overflow-tooltip/>
        <el-table-column prop="shape" label="Shape" min-width="140"/>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <div class="state-cell">
              <span class="status-dot" :class="instStateDot(row.state)"></span>
              {{ row.state || '—' }}
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="publicIps" label="公网IP" width="140"/>
        <el-table-column prop="architecture" label="架构" width="80"/>
        <el-table-column label="规格" width="120">
          <template #default="{ row }">{{ row.ocpus || 0 }}C / {{ row.memoryInGbs || 0 }}G</template>
        </el-table-column>
        <el-table-column prop="availabilityDomain" label="AD" min-width="120" show-overflow-tooltip/>
        <el-table-column prop="createTime" label="创建时间" width="160"/>
      </el-table>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Traffic Query Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="trafficQueryVisible" :title="`流量查询 — ${trafficQueryName}`" width="80%" destroy-on-close>
      <div style="margin-bottom:12px;display:flex;gap:12px;align-items:center;flex-wrap:wrap">
        <span style="font-size:13px;color:var(--text-secondary)">日期范围：</span>
        <el-date-picker
          v-model="trafficDateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          format="YYYY-MM-DD"
          value-format="YYYY-MM-DD"
          size="small"
          style="width:280px"
        />
        <el-button size="small" type="primary" @click="queryTrafficWithDate" :loading="trafficQueryLoading">
          <el-icon><Search /></el-icon> 查询
        </el-button>
        <span v-if="trafficDateRange" style="font-size:12px;color:var(--text-secondary)">
          {{ trafficDateRange[0] }} ~ {{ trafficDateRange[1] }}
        </span>
      </div>
      <template v-if="trafficQueryLoading">
        <el-skeleton :rows="5" animated/>
      </template>
      <el-table v-else :data="trafficQueryData" border stripe size="small">
        <template #empty><el-empty description="暂无流量数据" :image-size="60"/></template>
        <el-table-column prop="instanceId" label="实例 ID" min-width="180" show-overflow-tooltip/>
        <el-table-column label="入站流量" width="140" align="right">
          <template #default="{ row }">{{ formatBytes(row.ingressBytes) }}</template>
        </el-table-column>
        <el-table-column label="出站流量" width="140" align="right">
          <template #default="{ row }">{{ formatBytes(row.egressBytes) }}</template>
        </el-table-column>
        <el-table-column label="统计日期" width="180">
          <template #default="{ row }">{{ row.statsDate || '—' }}</template>
        </el-table-column>
        <el-table-column label="区域" width="100">
          <template #default="{ row }">{{ row.region || '—' }}</template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Security Rules Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="secRulesVisible" :title="`安全规则 — ${secRulesTenantName}`" width="85%" destroy-on-close>
      <el-tabs v-model="secRulesTab" @tab-change="loadSecRules">
        <el-tab-pane label="入站规则" name="ingress"/>
        <el-tab-pane label="出站规则" name="egress"/>
      </el-tabs>
      <div style="margin-bottom:12px;display:flex;gap:8px">
        <el-button type="success" size="small" @click="openSecRuleAdd">
          <el-icon><Plus /></el-icon> 添加规则
        </el-button>
        <el-button type="primary" size="small" @click="loadSecRules" :loading="secRulesLoading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
        <el-button type="warning" size="small" @click="enableAllProtocols" :loading="secRulesLoading">
          <el-icon><Connection /></el-icon> 启用所有协议
        </el-button>
      </div>
      <template v-if="secRulesLoading">
        <el-skeleton :rows="5" animated/>
      </template>
      <el-table v-else :data="secRulesList" border stripe size="small">
        <template #empty><el-empty description="暂无安全规则" :image-size="60"/></template>
        <el-table-column type="index" label="#" width="50" align="center"/>
        <el-table-column label="类型" width="80">
          <template #default="{ row }">
            <el-tag :type="row.type==='入站'?'success':'warning'" size="small">{{ row.type || '—' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="protocol" label="协议" width="100"/>
        <el-table-column prop="source" label="源/目标 CIDR" min-width="180" show-overflow-tooltip/>
        <el-table-column prop="ports" label="端口" width="120">
          <template #default="{ row }">{{ row.ports || 'ALL' }}</template>
        </el-table-column>
        <el-table-column prop="icmpType" label="ICMP Type" width="110">
          <template #default="{ row }">{{ row.icmpType || '—' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="80" align="center">
          <template #default="{ row, $index }">
            <el-button size="small" type="danger" @click="deleteSecRule(row, $index)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- Add Rule sub-dialog -->
      <el-dialog v-model="secRuleAddVisible" title="添加安全规则" width="460px" append-to-body destroy-on-close>
        <el-form :model="secRuleForm" label-width="100px">
          <el-form-item label="规则类型">
            <el-tag size="small">{{ secRulesTab === 'ingress' ? '入站' : '出站' }}</el-tag>
          </el-form-item>
          <el-form-item label="协议" required>
            <el-select v-model="secRuleForm.protocol" style="width:100%">
              <el-option label="ALL (所有协议)" value="all"/>
              <el-option label="TCP" value="6"/>
              <el-option label="UDP" value="17"/>
              <el-option label="ICMP" value="1"/>
            </el-select>
          </el-form-item>
          <el-form-item :label="secRulesTab === 'ingress' ? '源 CIDR' : '目标 CIDR'" required>
            <el-input v-model="secRuleForm.source" placeholder="0.0.0.0/0"/>
          </el-form-item>
          <el-form-item label="端口范围">
            <el-input v-model="secRuleForm.ports" placeholder="80 或 8080-9090，留空表示所有端口"/>
          </el-form-item>
          <el-form-item v-if="secRuleForm.protocol === '1'" label="ICMP Type">
            <el-input v-model="secRuleForm.icmpType" placeholder="8,0 (默认: Echo Request)"/>
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="secRuleAddVisible=false">取消</el-button>
          <el-button type="primary" :loading="secRuleSaving" @click="addSecRule">添加</el-button>
        </template>
      </el-dialog>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Quota Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="quotaVisible" :title="`配额查看 — ${quotaTenantName}`" width="80%" destroy-on-close>
      <div style="margin-bottom:12px;display:flex;gap:12px;align-items:center">
        <span style="font-size:13px;color:var(--text-secondary)">服务：</span>
        <el-select v-model="quotaService" size="small" style="width:180px" @change="loadQuota(0)">
          <el-option label="Compute" value="compute"/>
          <el-option label="Block Storage" value="block-storage"/>
          <el-option label="Object Storage" value="object-storage"/>
        </el-select>
        <el-button size="small" type="primary" @click="loadQuota(0)" :loading="quotaLoading">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
        <span v-if="quotaRegion" style="font-size:12px;color:var(--text-secondary)">
          区域: {{ quotaRegion }}
        </span>
      </div>
      <template v-if="quotaLoading">
        <el-skeleton :rows="5" animated/>
      </template>
      <el-table v-else :data="quotaItems" border stripe size="small">
        <template #empty><el-empty description="暂无配额数据" :image-size="60"/></template>
        <el-table-column prop="name" label="资源名称" min-width="240" show-overflow-tooltip/>
        <el-table-column label="可用" width="100" align="right">
          <template #default="{ row }">
            <span style="color:var(--status-up);font-weight:var(--font-semibold)">{{ row.available }}</span>
          </template>
        </el-table-column>
        <el-table-column label="已用" width="100" align="right">
          <template #default="{ row }">{{ row.used }}</template>
        </el-table-column>
        <el-table-column label="总计" width="100" align="right">
          <template #default="{ row }">{{ row.total }}</template>
        </el-table-column>
        <el-table-column label="使用率" width="120" align="center">
          <template #default="{ row }">
            <el-progress
              :percentage="row.total > 0 ? Math.round(row.used / row.total * 100) : 0"
              :stroke-width="12"
              :text-inside="true"
              :status="row.total > 0 && row.used / row.total > 0.8 ? 'exception' : ''"
            />
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top:12px;display:flex;justify-content:center;gap:8px">
        <el-button size="small" :disabled="quotaPage <= 0" @click="loadQuota(quotaPage - 1)">上一页</el-button>
        <span style="line-height:28px;font-size:13px;color:var(--text-secondary)">第 {{ quotaPage + 1 }} 页</span>
        <el-button size="small" :disabled="!quotaHasNext" @click="loadQuota(quotaPage + 1)">下一页</el-button>
      </div>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Region Subscription Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="regionSubVisible" :title="`区域订阅 — ${regionSubTenantName}`" width="85%" destroy-on-close>
      <!-- Summary Card -->
      <div style="display:flex;gap:24px;margin-bottom:16px;padding:16px;background:var(--bg-raised);border-radius:var(--radius-md)">
        <el-statistic title="总区域数" :value="regionSummary?.totalRegions ?? 0"/>
        <el-statistic title="已订阅">
          <template #default>
            <span style="color:var(--status-up)">{{ regionSummary?.subscribedRegions ?? 0 }}</span>
          </template>
        </el-statistic>
        <el-statistic title="未订阅">
          <template #default>
            <span style="color:var(--text-secondary)">{{ regionSummary?.unsubscribedRegions ?? 0 }}</span>
          </template>
        </el-statistic>
        <el-button size="small" type="primary" @click="loadRegionSubData" :loading="regionSubLoading" style="align-self:center">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </div>

      <el-tabs v-model="regionSubTab">
        <el-tab-pane label="已订阅" name="subscribed">
          <template v-if="regionSubLoading">
            <el-skeleton :rows="4" animated/>
          </template>
          <el-table v-else :data="regionSubscribedList" border stripe size="small">
            <template #empty><el-empty description="暂无已订阅区域" :image-size="60"/></template>
            <el-table-column prop="regionKey" label="区域 Key" min-width="160"/>
            <el-table-column prop="regionName" label="区域名称" min-width="160"/>
            <el-table-column label="状态" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status==='READY'?'success':'warning'" size="small">{{ row.status || '—' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="主区域" width="80" align="center">
              <template #default="{ row }">
                <span :class="row.isHomeRegion ? 'home-region-badge is-home' : 'home-region-badge not-home'">
                  {{ row.isHomeRegion ? '是' : '否' }}
                </span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="100" align="center">
              <template #default="{ row }">
                <el-button size="small" @click="checkRegionStatus(row.regionKey)" :loading="regionChecking">
                  检查状态
                </el-button>
              </template>
            </el-table-column>
          </el-table>
          <div v-if="regionCheckResult" style="margin-top:8px">
            <el-alert
              :title="`${regionCheckResult.regionKey}: ${regionCheckResult.status} (${regionCheckResult.subscribed ? '已订阅' : '未订阅'})`"
              :type="regionCheckResult.subscribed ? 'success' : 'info'"
              :closable="true"
              show-icon
              @close="regionCheckResult=null"
            />
          </div>
        </el-tab-pane>
        <el-tab-pane label="未订阅" name="unsubscribed">
          <div style="margin-bottom:12px;display:flex;gap:8px">
            <el-button type="success" size="small" @click="subscribeSelectedRegions" :disabled="regionSelectedKeys.length === 0" :loading="regionSubscribing">
              <el-icon><Plus /></el-icon> 订阅选中 ({{ regionSelectedKeys.length }})
            </el-button>
          </div>
          <template v-if="regionSubLoading">
            <el-skeleton :rows="4" animated/>
          </template>
          <el-table v-else :data="regionUnsubscribedList" border stripe size="small" @selection-change="onRegionSelectionChange">
            <template #empty><el-empty description="所有区域均已订阅" :image-size="60"/></template>
            <el-table-column type="selection" width="50"/>
            <el-table-column prop="key" label="区域 Key" min-width="160"/>
            <el-table-column prop="name" label="区域名称" min-width="160"/>
            <el-table-column prop="cnName" label="中文名" min-width="140"/>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Audit Log Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="auditVisible" :title="`审计日志 — ${auditTenantName}`" width="90%" destroy-on-close>
      <div style="margin-bottom:12px;display:flex;gap:12px;align-items:center;flex-wrap:wrap">
        <span style="font-size:13px;color:var(--text-secondary)">快速选择：</span>
        <el-radio-group v-model="auditDays" size="small" @change="onAuditDaysChange">
          <el-radio-button :value="1">1天</el-radio-button>
          <el-radio-button :value="3">3天</el-radio-button>
          <el-radio-button :value="7">7天</el-radio-button>
          <el-radio-button :value="30">30天</el-radio-button>
          <el-radio-button :value="90">90天</el-radio-button>
        </el-radio-group>
        <span style="font-size:13px;color:var(--text-secondary);margin-left:8px">或日期范围：</span>
        <el-date-picker
          v-model="auditDateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          format="YYYY-MM-DD"
          value-format="YYYY-MM-DD"
          size="small"
          style="width:280px"
          @change="auditDays = 0"
        />
        <el-button size="small" type="primary" @click="queryAuditLog" :loading="auditLoading">
          <el-icon><Search /></el-icon> 查询
        </el-button>
      </div>
      <template v-if="auditLoading">
        <el-skeleton :rows="6" animated/>
      </template>
      <el-table v-else :data="auditEvents" border stripe size="small" max-height="480">
        <template #empty><el-empty description="暂无审计日志" :image-size="60"/></template>
        <el-table-column prop="eventTime" label="时间" width="170"/>
        <el-table-column prop="eventType" label="事件类型" min-width="280" show-overflow-tooltip/>
        <el-table-column prop="userName" label="用户" min-width="200" show-overflow-tooltip/>
        <el-table-column prop="ipAddress" label="IP 地址" min-width="180" show-overflow-tooltip/>
        <el-table-column prop="clientEnv" label="客户端" min-width="180" show-overflow-tooltip/>
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.responseStatus==='200'?'success':'danger'" size="small">{{ row.responseStatus || '—' }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top:12px;display:flex;justify-content:center;gap:8px">
        <el-button size="small" :disabled="!auditHasNext" @click="queryAuditLogNext">
          加载更多
        </el-button>
        <span v-if="auditEvents.length > 0" style="line-height:28px;font-size:12px;color:var(--text-secondary)">
          已加载 {{ auditEvents.length }} 条
        </span>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus, Refresh, Monitor, Connection, InfoFilled, Edit, VideoPlay,
  Warning, DataAnalysis, Message, Share, Download, Delete, Search,
  Operation, MoreFilled, Key, User, DataLine, Location, Document
} from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import request from '../utils/request'

const router = useRouter()

interface Tenant {
  id: number; tenantId?: string; userName: string; tenancy: string; region: string
  regionName: string; fingerprint: string; apiSynced: boolean
  tenancyName?: string; tenancyDes?: string; accountType?: string
  cloudType?: number; emailAddress?: string; emailEnable?: boolean
  isActive?: boolean; isHomeRegion?: boolean; createdAt?: string
  enableIcmp?: boolean; enableAllProtocol?: boolean
  parenId?: number; regionEn?: string; idStr?: string
  transferStatus?: number; transferAmount?: string
  instanceCount?: number; accountCost?: string
  hasBootTask?: boolean; hasChildren?: boolean; activeDays?: string
}

interface RegionItem { code: string; name: string }

// --- state ---
const rows = ref<Tenant[]>([])
const loading = ref(false)
const searchText = ref('')
const showName = ref(0)

// add dialog
const addVisible = ref(false)
const saving = ref(false)
const keyFile = ref<HTMLInputElement | null>(null)
const fileBytes = ref<File | null>(null)
const form = ref({ tenancy:'', tenantId:'', fingerprint:'', region:'', userName:'' })
const configCollapse = ref<string[]>([])
const ociConfigText = ref('')
const syncing = ref(false)
const checking = ref(false)
const checkResult = ref<{alive:boolean;error?:string}|null>(null)

// detail dialog
const detailVisible = ref(false)
const detailLoading = ref(false)
const detailTab = ref('basic')
const detailData = ref<Tenant>({} as Tenant)

// edit name dialog
const editNameVisible = ref(false)
const editNameValue = ref('')
const editSaving = ref(false)

// edit cost dialog
const editCostVisible = ref(false)
const editCostValue = ref('')
const editTarget = ref<Tenant | null>(null)

// update dialog (auto-fetch from OCI)
const updateVisible = ref(false)
const updateSaving = ref(false)
const updateTarget = ref<Tenant | null>(null)
const updateResult = ref<any>(null)
const updateError = ref('')

// user management dialog
const userMgmtVisible = ref(false)
const userMgmtTab = ref('users')
const userMgmtTenantId = ref(0)
const userMgmtTenantName = ref('')
const userList = ref<any[]>([])
const userListLoading = ref(false)
const userGroups = ref<any[]>([])
const addUserFormVisible = ref(false)
const addUserSaving = ref(false)
const addUserForm = ref({ username: '', email: '', groupName: '' })
const createdUserPwd = ref('')
const resetPasswordResult = ref('')

// notification recipients
const showAddNotifEmailForm = ref(false)
const notifLoading = ref(false)
const notifSaving = ref(false)
const notifRecipients = ref<any[]>([])
const addNotifEmailForm = ref({ email: '' })

// MFA
const mfaLoading = ref(false)
const mfaStatus = ref<any>(null)

// password policy
const passwordPolicyVisible = ref(false)
const passwordPolicySaving = ref(false)
const passwordPolicyForm = ref({ enableExpiry: true, expiryDays: 120 })

// traffic alert dialog
const trafficVisible = ref(false)
const trafficSaving = ref(false)
const trafficTenantId = ref(0)
const trafficForm = ref({ statisticsEnabled:false, threshold:100, autoShutdown:false })

// email dialog
const emailVisible = ref(false)
const emailSaving = ref(false)
const emailTenantId = ref(0)
const emailForm = ref({ domainName:'', smtpHost:'', smtpPort:'', smtpUsername:'', smtpPassword:'', senderEmail:'', active:false })

// social dialog
const socialVisible = ref(false)
const socialLoading = ref(false)
const socialTenantId = ref(0)
const socialTenantName = ref('')
const socialList = ref<any[]>([])
const socialType = ref('Google')
const socialTypes = ['Google', 'GitHub', 'Microsoft']
const socialEditVisible = ref(false)
const socialEditId = ref(0)
const socialSaving = ref(false)
const socialForm = ref({ socialTypeStr:'', clientId:'', clientSecret:'', redirectUrl:'', thirdLoginAddress:'' })

// export dialog
const exportVisible = ref(false)
const exportTarget = ref<Tenant | null>(null)
const exporting = ref(false)

// batch check dialog
const batchCheckVisible = ref(false)
const batchChecking = ref(false)
const batchProgress = ref(0)
const batchResults = ref<{tenantId:number;userName:string;alive:boolean;error?:string}[]>([])

// instances dialog
const instVisible = ref(false)
const instLoading = ref(false)
const instTenantId = ref(0)
const instTenantName = ref('')
const instances = ref<any[]>([])

// traffic query dialog
const trafficQueryVisible = ref(false)
const trafficQueryLoading = ref(false)
const trafficQueryName = ref('')
const trafficQueryTenantId = ref(0)
const trafficDateRange = ref<string[] | null>(null)
const trafficQueryData = ref<any[]>([])

// security rules dialog
const secRulesVisible = ref(false)
const secRulesLoading = ref(false)
const secRulesTenantId = ref(0)
const secRulesTenantName = ref('')
const secRulesTab = ref('ingress')
const secRulesList = ref<any[]>([])
const secRuleAddVisible = ref(false)
const secRuleSaving = ref(false)
const secRuleForm = ref({ protocol: 'all', source: '0.0.0.0/0', ports: '', icmpType: '' })

// quota dialog
const quotaVisible = ref(false)
const quotaLoading = ref(false)
const quotaTenantId = ref(0)
const quotaTenantName = ref('')
const quotaService = ref('compute')
const quotaItems = ref<any[]>([])
const quotaPage = ref(0)
const quotaPageSize = ref(20)
const quotaHasNext = ref(false)
const quotaRegion = ref('')

// region subscription dialog
const regionSubVisible = ref(false)
const regionSubLoading = ref(false)
const regionSubTenantId = ref(0)
const regionSubTenantName = ref('')
const regionSummary = ref<{totalRegions:number;subscribedRegions:number;unsubscribedRegions:number}|null>(null)
const regionSubscribedList = ref<any[]>([])
const regionUnsubscribedList = ref<any[]>([])
const regionSubTab = ref('subscribed')
const regionSubscribing = ref(false)
const regionSelectedKeys = ref<string[]>([])
const regionChecking = ref(false)
const regionCheckResult = ref<any>(null)

// audit log dialog
const auditVisible = ref(false)
const auditLoading = ref(false)
const auditTenantId = ref(0)
const auditTenantName = ref('')
const auditDays = ref(7)
const auditDateRange = ref<string[] | null>(null)
const auditEvents = ref<any[]>([])
const auditNextPageToken = ref('')
const auditHasNext = ref(false)

// --- computed ---
const filteredRows = computed(() => {
  if (!searchText.value) return rows.value
  const q = searchText.value.toLowerCase()
  return rows.value.filter(r =>
    (r.userName || '').toLowerCase().includes(q) ||
    (r.tenancyName || '').toLowerCase().includes(q) ||
    (r.tenancyDes || '').toLowerCase().includes(q) ||
    (r.region || '').toLowerCase().includes(q)
  )
})

// --- regions ---
const regions: RegionItem[] = [
  { code:'ap-tokyo-1',name:'东京'},{code:'ap-osaka-1',name:'大阪'},
  { code:'ap-seoul-1',name:'首尔'},{code:'ap-singapore-1',name:'新加坡'},
  { code:'ap-singapore-2',name:'新加坡(西)'},{code:'ap-mumbai-1',name:'孟买'},
  { code:'ap-hyderabad-1',name:'海得拉巴'},{code:'ap-sydney-1',name:'悉尼'},
  { code:'ap-melbourne-1',name:'墨尔本'},{code:'ap-chuncheon-1',name:'春川'},
  { code:'ap-osaka-2',name:'大阪(第2)'},
  { code:'us-ashburn-1',name:'阿什本'},{code:'us-phoenix-1',name:'凤凰城'},
  { code:'us-sanjose-1',name:'圣何塞'},{code:'us-sanjose-2',name:'圣何塞(第2)'},
  { code:'us-chicago-1',name:'芝加哥'},{code:'us-phoenix-2',name:'凤凰城(第2)'},
  { code:'us-ashburn-2',name:'阿什本(第2)'},
  { code:'sa-saopaulo-1',name:'圣保罗'},{code:'sa-vinhedo-1',name:'维涅杜'},
  { code:'mx-queretaro-1',name:'克雷塔罗'},{code:'mx-queretaro-2',name:'克雷塔罗(第2)'},
  { code:'eu-frankfurt-1',name:'法兰克福'},{code:'eu-frankfurt-2',name:'法兰克福(第2)'},
  { code:'uk-london-1',name:'伦敦'},{code:'uk-cardiff-1',name:'加的夫'},
  { code:'eu-zurich-1',name:'苏黎世'},{code:'eu-amsterdam-1',name:'阿姆斯特丹'},
  { code:'eu-madrid-1',name:'马德里'},{code:'eu-milan-1',name:'米兰'},
  { code:'me-jeddah-1',name:'吉达'},{code:'me-dubai-1',name:'迪拜'},
  { code:'me-abudhabi-1',name:'阿布扎比'},{code:'af-johannesburg-1',name:'约翰内斯堡'},
]
const regionCodeToName: Record<string, string> = {}
regions.forEach(r => { regionCodeToName[r.code] = r.name })

// --- helpers ---
function maskedName(n: string): string {
  if (!n || n.length <= 2) return n || '***'
  return n[0] + '***' + n[n.length - 1]
}

function accountTypeTag(t: string | undefined): 'success'|'warning'|'info'|'' {
  if (!t) return 'info'
  if (t.includes('trial') || t.includes('试用')) return 'warning'
  if (t.includes('paid') || t.includes('付费')) return 'success'
  if (t.includes('enterprise') || t.includes('企业')) return ''
  return 'info'
}

function accountTypeLabel(t: string | undefined): string {
  if (!t) return '—'
  const m: Record<string,string> = { trial:'免费试用',paid:'付费账户',enterprise:'企业账户',free:'免费账户' }
  return m[t] || t
}

function cloudTypeLabel(ct: number | undefined): string {
  return ct === 1 ? 'OCI' : ct === 2 ? 'AWS' : ct === 3 ? 'GCP' : ct === 4 ? 'Azure' : String(ct || '—')
}

function instStateDot(state: string): string {
  if (!state) return 'status-dot--idle'
  const s = state.toLowerCase()
  if (s === 'running') return 'status-dot--up status-dot--pulse'
  if (s === 'stopped' || s === 'terminated') return 'status-dot--down'
  if (s === 'starting' || s === 'stopping') return 'status-dot--warn'
  return 'status-dot--idle'
}

function formatBytes(bytes: number | string | undefined): string {
  const n = Number(bytes)
  if (!n || isNaN(n)) return '0 B'
  if (n < 1024) return n + ' B'
  if (n < 1048576) return (n / 1024).toFixed(1) + ' KB'
  if (n < 1073741824) return (n / 1048576).toFixed(1) + ' MB'
  return (n / 1073741824).toFixed(2) + ' GB'
}

// --- action router ---
function handleAction(cmd: string, row: Tenant) {
  switch (cmd) {
    case 'detail': showDetail(row); break
    case 'update': openUpdate(row); break
    case 'check': checkTenant(row.id); break
    case 'boot': router.push('/boot'); break
    case 'instances': showInstances(row); break
    case 'securityRules': openSecRules(row); break
    case 'quota': openQuota(row); break
    case 'regionSub': openRegionSub(row); break
    case 'auditLog': openAuditLog(row); break
    case 'trafficAlert': openTrafficAlert(row); break
    case 'trafficQuery': openTrafficQuery(row); break
    case 'email': openEmail(row); break
    case 'social': openSocial(row); break
    case 'users': openUserManagement(row); break
    case 'export': exportTarget.value = row; exportVisible.value = true; break
    case 'delete': remove(row); break
  }
}

// --- load ---
async function load() {
  loading.value = true
  try {
    const tenants = await request.get('/tenants/listAll') as Tenant[]
    const countResults = await Promise.allSettled(
      tenants.map(t => request.get(`/tenants/${t.id}/instances`) as Promise<any[]>)
    )
    countResults.forEach((r, i) => {
      tenants[i].instanceCount = r.status === 'fulfilled' ? (r.value?.length ?? 0) : 0
    })
    rows.value = tenants
  } catch (e: any) { ElMessage.error(e.message) }
  finally { loading.value = false }
}

// --- add ---
function openAdd() {
  form.value = { tenancy:'', tenantId:'', fingerprint:'', region:'', userName:'' }
  fileBytes.value = null; ociConfigText.value = ''; configCollapse.value = []
  if (keyFile.value) keyFile.value.value = ''
  addVisible.value = true
}

function onFile(e: Event) {
  fileBytes.value = (e.target as HTMLInputElement).files?.[0] || null
}

async function save() {
  if (!form.value.tenancy || !form.value.tenantId || !form.value.fingerprint || !form.value.region) {
    ElMessage.warning('请填写所有必填字段'); return
  }
  if (!fileBytes.value) { ElMessage.warning('请选择 API 私钥文件'); return }
  saving.value = true
  try {
    const fd = new FormData()
    fd.append('tenancy', form.value.tenancy)
    fd.append('tenantId', form.value.tenantId)
    fd.append('fingerprint', form.value.fingerprint)
    fd.append('region', form.value.region)
    fd.append('userName', form.value.userName)
    fd.append('cloudType', '1')
    fd.append('isHomeRegion', 'true')
    fd.append('keyFileStr', fileBytes.value)
    await request.post('/tenants/save', fd, { headers:{'Content-Type':'multipart/form-data'} })
    ElMessage.success('保存成功')
    addVisible.value = false
    await load()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { saving.value = false }
}

function parseOciConfig() {
  const text = ociConfigText.value.trim()
  if (!text) { ElMessage.warning('请先粘贴 OCI 配置文件内容'); return }
  const lines = text.split('\n')
  let inDefault = false
  const kv: Record<string,string> = {}
  for (const raw of lines) {
    const line = raw.trim()
    if (!line || line.startsWith('#') || line.startsWith(';')) continue
    if (line.startsWith('[') && line.endsWith(']')) {
      inDefault = line.toLowerCase().includes('default'); continue
    }
    if (!inDefault && text.includes('[')) continue
    const eq = line.indexOf('=')
    if (eq === -1) continue
    const key = line.substring(0, eq).trim().toLowerCase()
    const value = line.substring(eq + 1).trim()
    if (key && value) kv[key] = value
  }
  let filled = 0
  if (kv['user'] && !form.value.tenantId) { form.value.tenantId = kv['user']; filled++ }
  if (kv['fingerprint'] && !form.value.fingerprint) { form.value.fingerprint = kv['fingerprint']; filled++ }
  if (kv['tenancy'] && !form.value.tenancy) { form.value.tenancy = kv['tenancy']; filled++ }
  if (kv['region'] && !form.value.region) {
    form.value.region = regionCodeToName[kv['region']] || kv['region']; filled++
  }
  if (kv['region'] && !form.value.userName) {
    form.value.userName = `${kv['region'].replace(/-/g,'_')}_${Math.random().toString(36).substring(2,6)}`
  }
  if (filled > 0) { ElMessage.success(`已自动填写 ${filled} 个字段`); configCollapse.value = [] }
  else { ElMessage.info('未找到可识别的配置项') }
}

// --- detail ---
async function showDetail(row: Tenant) {
  detailVisible.value = true
  detailTab.value = 'basic'
  detailLoading.value = true
  checkResult.value = null
  try {
    detailData.value = await request.get(`/tenants/${row.id}`) as Tenant
  } catch (e: any) { ElMessage.error(e.message) }
  finally { detailLoading.value = false }
}

// --- sync ---
async function syncOci(row: Tenant) {
  try {
    await ElMessageBox.confirm(`从 OCI 同步租户 ${row.userName} 的实例？`, '确认同步', { type:'info' })
    syncing.value = true
    await request.get('/tenants/syncOci', { params:{tenantId:row.id} })
    ElMessage.success('同步完成')
    await load()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error('同步失败: ' + e.message)
  }
  finally { syncing.value = false }
}

// --- check ---
async function checkTenant(id: number) {
  checking.value = true
  checkResult.value = null
  try {
    checkResult.value = await request.get(`/tenants/${id}/check`) as any
  } catch (e: any) { ElMessage.error(e.message) }
  finally { checking.value = false }
}

// --- batch check ---
async function startBatchCheck() {
  batchCheckVisible.value = true
  batchChecking.value = true
  batchProgress.value = 0
  batchResults.value = []
  try {
    const ids = rows.value.map(r => r.id)
    const step = Math.max(1, Math.floor(100 / ids.length))
    // Process in chunks to update progress
    const results: any[] = []
    for (let i = 0; i < ids.length; i++) {
      try {
        const r = await request.get(`/tenants/${ids[i]}/check`)
        results.push(r)
      } catch {
        results.push({ tenantId: ids[i], userName: rows.value[i]?.userName || '', alive: false, error: '请求失败' })
      }
      batchProgress.value = Math.min(100, Math.round((i + 1) / ids.length * 100))
    }
    batchResults.value = results
  } catch (e: any) { ElMessage.error(e.message) }
  finally { batchChecking.value = false }
}

// --- edit custom name ---
function openEditCustomName(row: Tenant) {
  editTarget.value = row
  editNameValue.value = row.tenancyDes || ''
  editNameVisible.value = true
}

async function saveCustomName() {
  if (!editTarget.value) return
  editSaving.value = true
  try {
    await request.put(`/tenants/${editTarget.value.id}`, {
      tenancyName: editTarget.value.tenancyName || editTarget.value.userName,
      tenancyDes: editNameValue.value,
      accountType: editTarget.value.accountType || '',
      emailAddress: editTarget.value.emailAddress || '',
      isActive: editTarget.value.isActive ?? true,
    })
    ElMessage.success('已更新')
    editTarget.value.tenancyDes = editNameValue.value
    editNameVisible.value = false
  } catch (e: any) { ElMessage.error(e.message) }
  finally { editSaving.value = false }
}

// --- edit cost ---
function openEditCost(row: Tenant) {
  editTarget.value = row
  editCostValue.value = row.accountCost || ''
  editCostVisible.value = true
}

async function saveAccountCost() {
  if (!editTarget.value) return
  editSaving.value = true
  try {
    const tenancyName = editTarget.value.tenancyName || editTarget.value.userName
    // Use cloud_tenancy cost update
    await request.put(`/tenants/${editTarget.value.id}`, {
      tenancyName: tenancyName,
      tenancyDes: editTarget.value.tenancyDes || '',
      accountType: editTarget.value.accountType || '',
      emailAddress: editTarget.value.emailAddress || '',
      isActive: editTarget.value.isActive ?? true,
    })
    ElMessage.success('已更新')
    editTarget.value.accountCost = editCostValue.value
    editCostVisible.value = false
  } catch (e: any) { ElMessage.error(e.message) }
  finally { editSaving.value = false }
}

// --- update (auto-fetch from OCI) ---
function openUpdate(row: Tenant) {
  updateTarget.value = row
  updateResult.value = null
  updateError.value = ''
  updateVisible.value = true
}

async function doUpdateDetail() {
  if (!updateTarget.value) return
  updateSaving.value = true
  updateError.value = ''
  updateResult.value = null
  try {
    updateResult.value = await request.post(`/tenants/${updateTarget.value.id}/update-detail`)
    ElMessage.success('已从 OCI 获取租户信息')
    await load()
  } catch (e: any) {
    updateError.value = '获取失败: ' + (e?.message || e)
    ElMessage.error(updateError.value)
  }
  finally { updateSaving.value = false }
}

// --- traffic alert ---
async function openTrafficAlert(row: Tenant) {
  trafficTenantId.value = row.id
  try {
    const data = await request.get('/traffic/alert/get', { params:{tenantId:row.id} }) as any
    trafficForm.value = {
      statisticsEnabled: data?.statisticsEnabled === 1 || data?.statisticsEnabled === true,
      threshold: data?.threshold || 100,
      autoShutdown: data?.autoShutdown === 1 || data?.autoShutdown === true,
    }
  } catch {
    trafficForm.value = { statisticsEnabled: false, threshold: 100, autoShutdown: false }
  }
  trafficVisible.value = true
}

function onTrafficToggle(val: boolean) {
  if (!val) trafficForm.value.autoShutdown = false
}

async function saveTrafficAlert() {
  trafficSaving.value = true
  try {
    await request.post('/traffic/alert/save', {
      tenantId: trafficTenantId.value,
      statisticsEnabled: trafficForm.value.statisticsEnabled,
      threshold: trafficForm.value.threshold,
      autoShutdown: trafficForm.value.autoShutdown,
    })
    ElMessage.success('流量预警配置已保存')
    trafficVisible.value = false
  } catch (e: any) { ElMessage.error(e.message) }
  finally { trafficSaving.value = false }
}

// --- traffic query ---
async function openTrafficQuery(row: Tenant) {
  trafficQueryName.value = row.userName || row.tenancyName || `#${row.id}`
  trafficQueryTenantId.value = row.id
  trafficDateRange.value = null
  trafficQueryData.value = []
  trafficQueryVisible.value = true
}

async function queryTrafficWithDate() {
  await doTrafficQuery()
}

async function doTrafficQuery() {
  trafficQueryLoading.value = true
  try {
    const params: any = { tenantId: trafficQueryTenantId.value }
    if (trafficDateRange.value && trafficDateRange.value.length === 2) {
      params.startDate = trafficDateRange.value[0]
      params.endDate = trafficDateRange.value[1]
    }
    trafficQueryData.value = await request.get('/instances/traffic', { params }) as any[]
  } catch (e: any) { ElMessage.error('流量查询失败: ' + (e?.message || e)) }
  finally { trafficQueryLoading.value = false }
}

// --- user management ---
function openUserManagement(row: Tenant) {
  userMgmtTenantId.value = row.id
  userMgmtTenantName.value = row.userName || row.tenancyName || `#${row.id}`
  userMgmtTab.value = 'users'
  userMgmtVisible.value = true
  refreshUserList()
}

async function refreshUserList() {
  userListLoading.value = true
  try {
    userList.value = await request.get(`/tenants/${userMgmtTenantId.value}/users`) as any[]
    userGroups.value = await request.get(`/tenants/${userMgmtTenantId.value}/groups`) as any[]
  } catch { userList.value = []; userGroups.value = [] }
  finally { userListLoading.value = false }
}

function showAddUserForm() {
  addUserForm.value = { username: '', email: '', groupName: '' }
  addUserFormVisible.value = true
  createdUserPwd.value = ''
}

async function doAddUser() {
  if (!addUserForm.value.username || !addUserForm.value.email) {
    ElMessage.warning('请填写用户名和邮箱'); return
  }
  addUserSaving.value = true
  createdUserPwd.value = ''
  try {
    const result = await request.post(`/tenants/${userMgmtTenantId.value}/users`, addUserForm.value) as any
    createdUserPwd.value = result?.password || ''
    addUserFormVisible.value = false
    if (createdUserPwd.value) {
      ElMessage.success('用户创建成功')
    }
    await refreshUserList()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { addUserSaving.value = false }
}

async function resetUserPassword(row: any) {
  try {
    await ElMessageBox.confirm(`确定重置用户 ${row.name} 的控制台登录密码？`, '确认重置密码', { type: 'warning' })
    const result = await request.post(`/tenants/${userMgmtTenantId.value}/users/${encodeURIComponent(row.ocid)}/reset-password`) as any
    if (result?.password) {
      ElMessageBox.alert(`新密码：${result.password}`, '密码已重置', {
        confirmButtonText: '已复制',
        type: 'success',
        callback: () => { navigator.clipboard?.writeText(result.password) }
      })
    } else {
      ElMessage.success('密码已重置')
    }
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

async function deleteUser(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除 IAM 用户「${row.name}」？此操作不可恢复。`, '确认删除', { type: 'warning' })
    await request.delete(`/tenants/${userMgmtTenantId.value}/users/${encodeURIComponent(row.ocid)}`)
    ElMessage.success('用户已删除')
    await refreshUserList()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// --- notification recipients ---
async function refreshNotifRecipients() {
  notifLoading.value = true
  try {
    notifRecipients.value = await request.get(`/tenants/${userMgmtTenantId.value}/notification-recipients`) as any[]
  } catch { notifRecipients.value = [] }
  finally { notifLoading.value = false }
}

async function doAddNotifEmail() {
  if (!addNotifEmailForm.value.email) { ElMessage.warning('请输入邮箱地址'); return }
  notifSaving.value = true
  try {
    // Build full list: current recipients + new email
    const currentEmails = notifRecipients.value.map((r: any) => r.email)
    if (currentEmails.includes(addNotifEmailForm.value.email)) {
      ElMessage.warning('该邮箱已存在')
      notifSaving.value = false
      return
    }
    currentEmails.push(addNotifEmailForm.value.email)
    await request.post(`/tenants/${userMgmtTenantId.value}/notification-recipients/update`, {
      emails: currentEmails
    })
    ElMessage.success('已添加')
    showAddNotifEmailForm.value = false
    addNotifEmailForm.value.email = ''
    await refreshNotifRecipients()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { notifSaving.value = false }
}

async function deleteNotifEmail(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除通知邮箱 ${row.email}？`, '确认删除', { type: 'warning' })
    // Build list without the deleted email
    const updatedEmails = notifRecipients.value
      .filter((r: any) => r.email !== row.email)
      .map((r: any) => r.email)
    await request.post(`/tenants/${userMgmtTenantId.value}/notification-recipients/update`, {
      emails: updatedEmails
    })
    ElMessage.success('已删除')
    await refreshNotifRecipients()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// --- MFA ---
async function refreshMfaStatus() {
  mfaLoading.value = true
  try {
    mfaStatus.value = await request.get(`/tenants/${userMgmtTenantId.value}/mfa/status`) as any
  } catch { mfaStatus.value = null }
  finally { mfaLoading.value = false }
}

async function toggleEmailMfa(enable: boolean) {
  const action = enable ? '启用' : '禁用'
  try {
    await ElMessageBox.confirm(`确定${action}邮箱 MFA？`, `确认${action}`, { type: 'warning' })
    await request.post(`/tenants/${userMgmtTenantId.value}/mfa/toggle`, { enable })
    ElMessage.success(`邮箱 MFA 已${action}`)
    await refreshMfaStatus()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

async function resetMfa() {
  try {
    await ElMessageBox.confirm('确定重置租户的 MFA 配置？此操作将删除该租户下所有用户的 MFA TOTP 设备。', '确认重置 MFA', { type: 'warning' })
    const res = await request.post(`/tenants/${userMgmtTenantId.value}/mfa/reset`) as any
    ElMessage.success(`MFA 已重置，已删除 ${res?.deletedDevices ?? 0} 个设备`)
    await refreshMfaStatus()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// --- password policy ---
function showPasswordPolicyDialog() {
  passwordPolicyVisible.value = true
  // Load current policy
  request.get(`/tenants/${userMgmtTenantId.value}/password-policy`).then((data: any) => {
    if (data) {
      passwordPolicyForm.value.enableExpiry = data.isPasswordExpiryEnabled !== false
      passwordPolicyForm.value.expiryDays = data.passwordExpiryDays || 120
    }
  }).catch(() => {})
}

function onPwdExpiryToggle(val: boolean) {
  if (!val) passwordPolicyForm.value.expiryDays = 120
}

async function savePasswordPolicy() {
  passwordPolicySaving.value = true
  try {
    await request.post(`/tenants/${userMgmtTenantId.value}/password-policy`, {
      enableExpiry: passwordPolicyForm.value.enableExpiry,
      expiryDays: passwordPolicyForm.value.expiryDays,
    })
    ElMessage.success('密码策略已保存')
    passwordPolicyVisible.value = false
  } catch (e: any) { ElMessage.error(e.message) }
  finally { passwordPolicySaving.value = false }
}

function formatTime(t: string | undefined): string {
  if (!t) return '—'
  try { return new Date(t).toLocaleString('zh-CN') } catch { return t }
}

function copyText(text: string) {
  navigator.clipboard?.writeText(text).then(() => ElMessage.success('已复制到剪贴板')).catch(() => {})
}

// --- email ---
async function openEmail(row: Tenant) {
  emailTenantId.value = row.id
  try {
    const cfg = await request.get(`/tenants/${row.id}/email`) as any
    emailForm.value = {
      domainName: cfg?.domainName || '',
      smtpHost: cfg?.smtpHost || '',
      smtpPort: cfg?.smtpPort || '587',
      smtpUsername: cfg?.smtpUsername || '',
      smtpPassword: cfg?.smtpPassword || '',
      senderEmail: cfg?.senderEmail || '',
      active: cfg?.active === true || cfg?.active === 1,
    }
  } catch {
    emailForm.value = { domainName:'', smtpHost:'', smtpPort:'587', smtpUsername:'', smtpPassword:'', senderEmail:'', active:false }
  }
  emailVisible.value = true
}

async function saveEmail() {
  emailSaving.value = true
  try {
    await request.post(`/tenants/${emailTenantId.value}/email`, {
      ...emailForm.value,
      tenantId: emailTenantId.value,
    })
    ElMessage.success('邮箱配置已保存')
    emailVisible.value = false
  } catch (e: any) { ElMessage.error(e.message) }
  finally { emailSaving.value = false }
}

// --- social ---
async function openSocial(row: Tenant) {
  socialTenantId.value = row.id
  socialTenantName.value = row.userName || row.tenancyName || ''
  socialVisible.value = true
  await loadSocialList()
}

async function loadSocialList() {
  socialLoading.value = true
  try {
    socialList.value = await request.get(`/tenants/${socialTenantId.value}/social`) as any[]
  } catch { socialList.value = [] }
  finally { socialLoading.value = false }
}

function openAddSocial() {
  socialEditId.value = 0
  socialForm.value = { socialTypeStr: socialType.value, clientId:'', clientSecret:'', redirectUrl:'', thirdLoginAddress:'' }
  socialEditVisible.value = true
}

function editSocial(item: any) {
  socialEditId.value = item.id
  socialForm.value = {
    socialTypeStr: item.socialTypeStr,
    clientId: item.clientId,
    clientSecret: item.clientSecret,
    redirectUrl: item.redirectUrl || '',
    thirdLoginAddress: item.thirdLoginAddress || '',
  }
  socialEditVisible.value = true
}

async function saveSocial() {
  if (!socialForm.value.clientId || !socialForm.value.clientSecret) {
    ElMessage.warning('请填写 Client ID 和 Client Secret'); return
  }
  socialSaving.value = true
  try {
    const payload: any = {
      id: socialEditId.value || 0,
      tenantId: socialTenantId.value,
      tenancy: socialTenantName.value,
      cloudType: 1,
      ...socialForm.value,
    }
    await request.post(`/tenants/${socialTenantId.value}/social`, payload)
    ElMessage.success(socialEditId.value ? '已更新' : '已添加')
    socialEditVisible.value = false
    await loadSocialList()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { socialSaving.value = false }
}

async function toggleSocial(item: any) {
  const newStatus = item.socialStatus === 'enabled' ? 'disabled' : 'enabled'
  try {
    await request.put(`/tenants/${socialTenantId.value}/social/${item.id}/toggle`, { socialStatus: newStatus })
    ElMessage.success(`已${newStatus === 'enabled' ? '启用' : '禁用'}`)
    await loadSocialList()
  } catch (e: any) { ElMessage.error(e.message) }
}

async function deleteSocial(item: any) {
  try {
    await ElMessageBox.confirm(`确定删除 ${item.socialTypeStr} 社媒配置？`, '确认删除', { type:'warning' })
    await request.delete(`/tenants/${socialTenantId.value}/social/${item.id}`)
    ElMessage.success('已删除')
    await loadSocialList()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// --- export ---
async function doExport() {
  if (!exportTarget.value) return
  exporting.value = true
  try {
    const data = await request.get(`/tenants/${exportTarget.value.id}/export`, { responseType:'blob' }) as any
    const blob = data instanceof Blob ? data : new Blob([JSON.stringify(data, null, 2)], { type:'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `tenant_${exportTarget.value.id}.json`
    a.click()
    URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
    exportVisible.value = false
  } catch (e: any) { ElMessage.error(e.message) }
  finally { exporting.value = false }
}

// --- delete ---
async function remove(row: Tenant) {
  try {
    await ElMessageBox.confirm(
      `确定删除租户「${row.userName || row.tenancyName}」？此操作将同时删除该租户下的所有实例记录，不可恢复。`,
      '确认删除',
      { confirmButtonText:'确定删除', cancelButtonText:'取消', type:'warning' }
    )
    await request.get('/tenants/deleteApi', { params:{tenantId:row.id} })
    ElMessage.success('已删除')
    await load()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// --- instances ---
async function showInstances(row: Tenant) {
  instTenantId.value = row.id
  instTenantName.value = row.userName || `#${row.id}`
  instVisible.value = true
  instLoading.value = true
  try {
    instances.value = await request.get(`/tenants/${row.id}/instances`) as any[]
  } catch (e: any) { ElMessage.error(e.message) }
  finally { instLoading.value = false }
}

// --- security rules ---
async function openSecRules(row: Tenant) {
  secRulesTenantId.value = row.id
  secRulesTenantName.value = row.userName || row.tenancyName || `#${row.id}`
  secRulesTab.value = 'ingress'
  secRulesVisible.value = true
  await loadSecRules()
}

async function loadSecRules() {
  secRulesLoading.value = true
  try {
    secRulesList.value = await request.get('/tenants/security-rules', {
      params: { tenantId: secRulesTenantId.value, type: secRulesTab.value }
    }) as any[]
  } catch (e: any) {
    ElMessage.error('加载安全规则失败: ' + (e?.message || e))
    secRulesList.value = []
  } finally { secRulesLoading.value = false }
}

function openSecRuleAdd() {
  secRuleForm.value = { protocol: 'all', source: '0.0.0.0/0', ports: '', icmpType: '' }
  secRuleAddVisible.value = true
}

async function addSecRule() {
  if (!secRuleForm.value.source) {
    ElMessage.warning('请填写源/目标 CIDR'); return
  }
  secRuleSaving.value = true
  try {
    await request.post('/tenants/security-rules', {
      tenantId: secRulesTenantId.value,
      type: secRulesTab.value,
      protocol: secRuleForm.value.protocol,
      source: secRuleForm.value.source,
      ports: secRuleForm.value.ports || null,
      icmpType: secRuleForm.value.protocol === '1' ? (secRuleForm.value.icmpType || '8,0') : null,
    })
    ElMessage.success('规则已添加')
    secRuleAddVisible.value = false
    await loadSecRules()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { secRuleSaving.value = false }
}

async function deleteSecRule(row: any, index: number) {
  try {
    await ElMessageBox.confirm('确定删除该安全规则？', '确认删除', { type: 'warning' })
    const compositeId = `${secRulesTenantId.value}_${index}_${secRulesTab.value}`
    await request.delete(`/tenants/security-rules/${compositeId}`)
    ElMessage.success('规则已删除')
    await loadSecRules()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

async function enableAllProtocols() {
  try {
    await ElMessageBox.confirm('将为所有租户启用所有协议规则，是否继续？', '启用所有协议', { type: 'warning' })
    secRulesLoading.value = true
    await request.post('/tenants/enableAll')
    ElMessage.success('所有协议已启用')
    await loadSecRules()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  } finally { secRulesLoading.value = false }
}

// --- quota ---
async function openQuota(row: Tenant) {
  quotaTenantId.value = row.id
  quotaTenantName.value = row.userName || row.tenancyName || `#${row.id}`
  quotaService.value = 'compute'
  quotaPage.value = 0
  quotaVisible.value = true
  await loadQuota(0)
}

async function loadQuota(page: number) {
  quotaLoading.value = true
  try {
    const resp: any = await request.get(`/tenants/${quotaTenantId.value}/quota`, {
      params: { serviceName: quotaService.value, page, pageSize: quotaPageSize.value }
    })
    quotaItems.value = resp?.items || []
    quotaPage.value = resp?.page ?? page
    quotaHasNext.value = resp?.hasNextPage ?? false
    quotaRegion.value = resp?.region || resp?.regionEn || ''
  } catch (e: any) {
    ElMessage.error('加载配额失败: ' + (e?.message || e))
    quotaItems.value = []
  } finally { quotaLoading.value = false }
}

// --- region subscription ---
async function openRegionSub(row: Tenant) {
  regionSubTenantId.value = row.id
  regionSubTenantName.value = row.userName || row.tenancyName || `#${row.id}`
  regionSubTab.value = 'subscribed'
  regionSelectedKeys.value = []
  regionCheckResult.value = null
  regionSubVisible.value = true
  await loadRegionSubData()
}

async function loadRegionSubData() {
  regionSubLoading.value = true
  try {
    const [summary, subscribed, unsubscribed] = await Promise.all([
      request.get(`/tenants/${regionSubTenantId.value}/regions/summary`),
      request.get(`/tenants/${regionSubTenantId.value}/regions/subscribed`),
      request.get(`/tenants/${regionSubTenantId.value}/regions/unsubscribed`),
    ])
    regionSummary.value = summary as any
    regionSubscribedList.value = subscribed as any[]
    regionUnsubscribedList.value = unsubscribed as any[]
  } catch (e: any) {
    ElMessage.error('加载区域订阅数据失败: ' + (e?.message || e))
  } finally { regionSubLoading.value = false }
}

function onRegionSelectionChange(rows: any[]) {
  regionSelectedKeys.value = rows.map((r: any) => r.key)
}

async function subscribeSelectedRegions() {
  if (regionSelectedKeys.value.length === 0) return
  try {
    await ElMessageBox.confirm(
      `确定订阅选中的 ${regionSelectedKeys.value.length} 个区域？此操作可能需要几分钟完成。`,
      '确认订阅', { type: 'info' }
    )
    regionSubscribing.value = true
    const resp: any = await request.post(`/tenants/${regionSubTenantId.value}/regions/subscribe`, {
      regionKeys: regionSelectedKeys.value
    })
    if (resp?.success) {
      ElMessage.success(resp?.message || '订阅请求已提交')
    } else {
      ElMessage.warning(resp?.message || '部分区域订阅失败')
    }
    regionSelectedKeys.value = []
    await loadRegionSubData()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  } finally { regionSubscribing.value = false }
}

async function checkRegionStatus(regionKey: string) {
  regionChecking.value = true
  regionCheckResult.value = null
  try {
    regionCheckResult.value = await request.get(`/tenants/${regionSubTenantId.value}/regions/subscription-status`, {
      params: { regionKey }
    })
  } catch (e: any) {
    ElMessage.error('检查状态失败: ' + (e?.message || e))
  } finally { regionChecking.value = false }
}

// --- audit log ---
function openAuditLog(row: Tenant) {
  auditTenantId.value = row.id
  auditTenantName.value = row.userName || row.tenancyName || `#${row.id}`
  auditDays.value = 7
  auditDateRange.value = null
  auditEvents.value = []
  auditNextPageToken.value = ''
  auditHasNext.value = false
  auditVisible.value = true
  queryAuditLog()
}

function onAuditDaysChange(val: number) {
  if (val > 0) auditDateRange.value = null
}

async function queryAuditLog() {
  auditLoading.value = true
  auditEvents.value = []
  auditNextPageToken.value = ''
  auditHasNext.value = false
  try {
    const body: any = {}
    if (auditDateRange.value && auditDateRange.value.length === 2) {
      body.startDate = auditDateRange.value[0]
      body.endDate = auditDateRange.value[1]
    } else {
      body.days = auditDays.value || 7
    }
    const resp: any = await request.post(`/tenants/${auditTenantId.value}/audit-log`, body)
    const page = resp?.data || resp
    auditEvents.value = page?.data || []
    auditNextPageToken.value = page?.nextPageToken || ''
    auditHasNext.value = !!auditNextPageToken.value
  } catch (e: any) {
    ElMessage.error('查询审计日志失败: ' + (e?.message || e))
  } finally { auditLoading.value = false }
}

async function queryAuditLogNext() {
  if (!auditNextPageToken.value) return
  auditLoading.value = true
  try {
    const body: any = { pageToken: auditNextPageToken.value }
    if (auditDateRange.value && auditDateRange.value.length === 2) {
      body.startDate = auditDateRange.value[0]
      body.endDate = auditDateRange.value[1]
    } else {
      body.days = auditDays.value || 7
    }
    const resp: any = await request.post(`/tenants/${auditTenantId.value}/audit-log`, body)
    const page = resp?.data || resp
    const newEvents = page?.data || []
    auditEvents.value = [...auditEvents.value, ...newEvents]
    auditNextPageToken.value = page?.nextPageToken || ''
    auditHasNext.value = !!auditNextPageToken.value
  } catch (e: any) {
    ElMessage.error('加载更多失败: ' + (e?.message || e))
  } finally { auditLoading.value = false }
}

onMounted(load)

// Watch tab switch in user management dialog to load relevant data
watch(userMgmtTab, (tab) => {
  if (tab === 'notifications') refreshNotifRecipients()
  else if (tab === 'mfa') refreshMfaStatus()
})
</script>

<style scoped>
.tenants-page { padding: 0; }

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

.table-card { border-radius: var(--radius-md); overflow: hidden; }
.table-card :deep(.el-card__body) { padding: 0; }

/* clickable links */
.spoiler-link { cursor: pointer; color: var(--accent); font-weight: var(--font-medium); }
.spoiler-link:hover { color: var(--accent-hover); text-decoration: underline; }

.cell-edit-link { cursor: pointer; color: var(--text-primary); }
.cell-edit-link:hover { color: var(--accent); text-decoration: underline; }

/* status badges */
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  font-size: var(--text-xs);
  font-weight: var(--font-medium);
}
.status-badge.status-running { background: color-mix(in srgb, var(--status-up) 15%, transparent); color: var(--status-up); }
.status-badge.status-idle { background: var(--bg-raised); color: var(--text-secondary); }

.days-chip {
  display: inline-block;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  background: var(--bg-raised);
  font-size: var(--text-sm);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

.home-region-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  font-size: var(--text-xs);
  font-weight: var(--font-medium);
}
.home-region-badge.is-home { background: color-mix(in srgb, var(--status-up) 15%, transparent); color: var(--status-up); }
.home-region-badge.not-home { background: var(--bg-raised); color: var(--text-secondary); }

.state-cell {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-sm);
}

/* skeleton in dialog */
:deep(.el-dialog__body) .el-skeleton { padding: var(--space-4) 0; }

/* element overrides */
:deep(.el-collapse-item__header) {
  font-size: var(--text-base);
  font-weight: var(--font-semibold);
  color: var(--accent);
}
:deep(.el-collapse-item__header:hover) { color: var(--accent-hover); }

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

:deep(.el-collapse) {
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  background: var(--bg-surface);
}
:deep(.el-collapse-item) { border-bottom: 1px solid var(--border-subtle); }
:deep(.el-collapse-item:last-child) { border-bottom: none; }

:deep(.el-statistic__head) { font-size: var(--text-xs); color: var(--text-secondary); }
:deep(.el-statistic__content) { font-size: var(--text-xl); }

@media (max-width: 768px) {
  .toolbar { flex-direction: column; align-items: flex-start; }
  .toolbar-left h2 { font-size: var(--text-lg); }
  .toolbar-right { width: 100%; justify-content: flex-start; }
}
</style>
