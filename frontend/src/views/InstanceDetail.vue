<template>
  <div class="instance-detail-page">
    <!-- Header -->
    <div class="detail-header">
      <div class="header-left">
        <el-button text @click="router.push('/instances')"><el-icon><ArrowLeft /></el-icon> 返回实例列表</el-button>
        <h2>{{ instance.displayName || `实例 #${instance.id}` }}</h2>
        <StatusBadge
          :status="getStateStatus(instance.state)"
          :label="instance.state || '-'"
          :pulse="instance.state === 'Running'"
        />
        <el-tag v-if="instance.tenantName" size="small" type="info">{{ instance.tenantName }}</el-tag>
        <el-tag v-if="instance.availabilityDomain" size="small">{{ instance.availabilityDomain }}</el-tag>
      </div>
      <div class="header-right">
        <el-button size="small" type="success" :disabled="instance.state === 'Running'" @click="confirmStart" :loading="actionLoading">
          <el-icon><VideoPlay /></el-icon> 启动
        </el-button>
        <el-button size="small" type="danger" :disabled="instance.state !== 'Running'" @click="confirmStop" :loading="actionLoading">
          <el-icon><VideoPause /></el-icon> 停止
        </el-button>
      </div>
    </div>

    <!-- Tabs -->
    <el-tabs v-model="activeTab" type="border-card">
      <!-- ======================== 概览 Tab ======================== -->
      <el-tab-pane label="概览" name="overview">
        <el-collapse v-model="activePanels" @change="onPanelChange">

          <!-- ======================== 基本信息 ======================== -->
          <el-collapse-item name="info">
            <template #title>
              <span class="panel-title">基本信息</span>
            </template>
            <el-skeleton v-if="loading" :rows="8" animated />
            <template v-else>
              <el-descriptions :column="2" border size="small">
                <el-descriptions-item label="实例 ID" :span="2">
                  <MonoText>{{ instance.instanceId || '-' }}</MonoText>
                </el-descriptions-item>
                <el-descriptions-item label="Shape">{{ instance.shape || '-' }}</el-descriptions-item>
                <el-descriptions-item label="架构">{{ instance.architecture || '-' }}</el-descriptions-item>
                <el-descriptions-item label="OCPU">{{ instance.ocpus ?? '-' }}</el-descriptions-item>
                <el-descriptions-item label="内存(GB)">{{ instance.memoryInGbs ?? '-' }}</el-descriptions-item>
                <el-descriptions-item label="公网 IP">
                  <MonoText>{{ instance.publicIps || '-' }}</MonoText>
                </el-descriptions-item>
                <el-descriptions-item label="私网 IP">
                  <MonoText>{{ instance.privateIps || '-' }}</MonoText>
                </el-descriptions-item>
                <el-descriptions-item label="IPv6">
                  <MonoText>{{ instance.ipv6Addresses || '-' }}</MonoText>
                </el-descriptions-item>
                <el-descriptions-item label="可用域">{{ instance.availabilityDomain || '-' }}</el-descriptions-item>
                <el-descriptions-item label="启动卷 ID" :span="2">
                  <MonoText>{{ instance.bootVolumeId || '-' }}</MonoText>
                </el-descriptions-item>
                <el-descriptions-item label="VNIC IDs" :span="2">
                  <MonoText>{{ instance.vnicIds || '-' }}</MonoText>
                </el-descriptions-item>
                <el-descriptions-item label="最后心跳">{{ instance.lastHeartbeat || '-' }}</el-descriptions-item>
                <el-descriptions-item label="创建时间">{{ instance.createTime || '-' }}</el-descriptions-item>
              </el-descriptions>
            </template>
          </el-collapse-item>

          <!-- ======================== 操作 ======================== -->
          <el-collapse-item name="actions">
            <template #title>
              <span class="panel-title">操作</span>
            </template>
            <div class="action-buttons">
              <el-button size="small" @click="handleAction('rescue')">
                <el-icon><FirstAidKit /></el-icon> 救援模式
              </el-button>
              <el-button size="small" @click="handleAction('console')">
                <el-icon><Monitor /></el-icon> VNC 控制台
              </el-button>
              <el-button size="small" @click="handleAction('terminal')">
                <el-icon><Connection /></el-icon> SSH 终端
              </el-button>
              <el-button size="small" type="warning" @click="handleAction('modify')">
                <el-icon><Edit /></el-icon> 修改配置
              </el-button>
            </div>
          </el-collapse-item>

          <!-- ======================== 磁盘 ======================== -->
          <el-collapse-item name="disk">
            <template #title>
              <span class="panel-title">磁盘</span>
            </template>
            <el-skeleton v-if="diskLoading" :rows="4" animated />
            <template v-else>
              <el-descriptions :column="2" border size="small" style="margin-bottom:16px">
                <el-descriptions-item label="启动卷大小">{{ instance.bootVolumeSizeInGbs ?? '-' }} GB</el-descriptions-item>
                <el-descriptions-item label="当前 VPU/GB">{{ instance.vpusPerGb || '-' }}</el-descriptions-item>
                <el-descriptions-item label="启动卷 ID" :span="2">
                  <MonoText style="font-size:var(--text-xs)">{{ instance.bootVolumeId || '-' }}</MonoText>
                </el-descriptions-item>
              </el-descriptions>
              <el-alert
                title="调整磁盘 VPU 性能需要先停止实例。修改后重新启动实例生效。"
                type="info"
                :closable="false"
                show-icon
                style="margin-bottom:16px"
              />
              <el-form :model="diskForm" label-width="120px" size="small">
                <el-form-item label="VPU 性能等级">
                  <el-select v-model="diskForm.vpusPerGb" style="width:100%">
                    <el-option :value="10" label="10 - 均衡 (Balanced)">
                      <div class="vpu-option">
                        <span class="vpu-label">10 - 均衡</span>
                        <span class="vpu-desc">适用于大多数工作负载</span>
                      </div>
                    </el-option>
                    <el-option :value="20" label="20 - 更高性能 (Higher Performance)">
                      <div class="vpu-option">
                        <span class="vpu-label">20 - 更高性能</span>
                        <span class="vpu-desc">适用于需要更高 IOPS 的场景</span>
                      </div>
                    </el-option>
                    <el-option :value="30" label="30 - 超高性能 (Ultra High)">
                      <div class="vpu-option">
                        <span class="vpu-label">30 - 超高性能</span>
                        <span class="vpu-desc">适用于数据库和高 IO 负载</span>
                      </div>
                    </el-option>
                    <el-option :value="40" label="40 - 超高性能">
                      <div class="vpu-option"><span class="vpu-label">40 - 超高性能</span></div>
                    </el-option>
                    <el-option :value="50" label="50 - 超高性能">
                      <div class="vpu-option"><span class="vpu-label">50 - 超高性能</span></div>
                    </el-option>
                    <el-option :value="60" label="60 - 超高性能">
                      <div class="vpu-option"><span class="vpu-label">60 - 超高性能</span></div>
                    </el-option>
                    <el-option :value="80" label="80 - 超高性能">
                      <div class="vpu-option"><span class="vpu-label">80 - 超高性能</span></div>
                    </el-option>
                    <el-option :value="100" label="100 - 超高性能">
                      <div class="vpu-option"><span class="vpu-label">100 - 超高性能</span></div>
                    </el-option>
                    <el-option :value="120" label="120 - 超高性能">
                      <div class="vpu-option"><span class="vpu-label">120 - 超高性能</span></div>
                    </el-option>
                  </el-select>
                </el-form-item>
                <el-form-item>
                  <el-button type="primary" :loading="vpuSaving" @click="saveVpu">更新 VPU 设置</el-button>
                </el-form-item>
              </el-form>
            </template>
          </el-collapse-item>

          <!-- ======================== 网络 ======================== -->
          <el-collapse-item name="network">
            <template #title>
              <span class="panel-title">网络</span>
            </template>
            <el-descriptions :column="2" border size="small" style="margin-bottom:16px">
              <el-descriptions-item label="VNIC IDs" :span="2">
                <MonoText style="font-size:var(--text-xs)">{{ instance.vnicIds || '-' }}</MonoText>
              </el-descriptions-item>
              <el-descriptions-item label="公网 IP">
                <MonoText>{{ instance.publicIps || '-' }}</MonoText>
              </el-descriptions-item>
              <el-descriptions-item label="私网 IP">
                <MonoText>{{ instance.privateIps || '-' }}</MonoText>
              </el-descriptions-item>
              <el-descriptions-item label="IPv6" :span="2">
                <MonoText>{{ instance.ipv6Addresses || '未启用' }}</MonoText>
              </el-descriptions-item>
            </el-descriptions>
            <div style="display:flex;gap:var(--space-3);flex-wrap:wrap">
              <el-button type="primary" size="small" @click="router.push({ path: '/vnic', query: { instanceId: instance.instanceId } })">
                <el-icon><Connection /></el-icon> 打开 VNIC 管理
              </el-button>
              <el-button size="small" @click="handleAction('change-ip')">
                <el-icon><Connection /></el-icon> 更换公网 IP
              </el-button>
              <el-button size="small" :disabled="!!instance.ipv6Addresses" @click="handleAction('enable-ipv6')">
                <el-icon><Link /></el-icon> 启用 IPv6
              </el-button>
            </div>
          </el-collapse-item>

          <!-- ======================== SSH 配置 ======================== -->
          <el-collapse-item name="ssh">
            <template #title>
              <span class="panel-title">SSH 配置</span>
            </template>
            <el-skeleton v-if="sshLoading" :rows="4" animated />
            <template v-else>
              <el-form :model="sshForm" label-width="100px" size="small" style="max-width:500px">
                <el-form-item label="用户名">
                  <el-input v-model="sshForm.username" placeholder="root" />
                </el-form-item>
                <el-form-item label="端口">
                  <el-input-number v-model="sshForm.port" :min="1" :max="65535" style="width:100%" />
                </el-form-item>
                <el-form-item label="密码">
                  <el-input v-model="sshForm.password" type="password" show-password placeholder="SSH 密码" />
                </el-form-item>
                <el-form-item>
                  <el-button type="primary" :loading="sshSaving" @click="saveSshConfig">保存 SSH 配置</el-button>
                </el-form-item>
              </el-form>
            </template>
          </el-collapse-item>

          <!-- ======================== 备注 ======================== -->
          <el-collapse-item name="remark">
            <template #title>
              <span class="panel-title">备注</span>
            </template>
            <el-skeleton v-if="remarkLoading" :rows="3" animated />
            <template v-else>
              <el-input
                v-model="remarkText"
                type="textarea"
                :rows="4"
                placeholder="添加备注信息..."
              />
              <el-button type="primary" size="small" style="margin-top:12px" :loading="remarkSaving" @click="saveRemark">
                保存备注
              </el-button>
            </template>
          </el-collapse-item>

        </el-collapse>
      </el-tab-pane>

      <!-- ======================== 流量监控 Tab ======================== -->
      <el-tab-pane label="流量监控" name="traffic">
        <InstanceTrafficPanel v-if="instance.instanceId" :instance-id="instance.instanceId" />
      </el-tab-pane>

      <!-- ======================== 磁盘管理 Tab ======================== -->
      <el-tab-pane label="磁盘管理" name="disk-mgmt">
        <!-- DiskManagementPanel will be created in Task 5 -->
        <el-empty description="磁盘管理面板 (即将上线)" />
      </el-tab-pane>

      <!-- ======================== 控制台连接 Tab ======================== -->
      <el-tab-pane label="控制台连接" name="console">
        <!-- Console connection panel placeholder -->
        <el-empty description="控制台连接面板 (即将上线)" />
      </el-tab-pane>
    </el-tabs>

    <!-- ================================================================ -->
    <!-- Change IP Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="changeIpVisible" title="更换公网 IP" width="460px" destroy-on-close>
      <el-alert
        title="更换 IP 会释放当前公网 IP 并分配新的临时公网 IP"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom:16px"
      />
      <el-descriptions :column="1" border>
        <el-descriptions-item label="实例">{{ instance.displayName }}</el-descriptions-item>
        <el-descriptions-item label="当前 IP">{{ instance.publicIps || '-' }}</el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button @click="changeIpVisible = false">取消</el-button>
        <el-button type="primary" :loading="changeIpLoading" @click="doChangeIp">确认更换</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Enable IPv6 Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="ipv6Visible" title="启用 IPv6" width="460px" destroy-on-close>
      <el-descriptions :column="1" border>
        <el-descriptions-item label="实例">{{ instance.displayName }}</el-descriptions-item>
        <el-descriptions-item label="当前 IPv6">{{ instance.ipv6Addresses || '未启用' }}</el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button @click="ipv6Visible = false">取消</el-button>
        <el-button type="primary" :loading="ipv6Loading" @click="doEnableIpv6">确认启用</el-button>
      </template>
    </el-dialog>

    <!-- ================================================================ -->
    <!-- Modify Config Dialog -->
    <!-- ================================================================ -->
    <el-dialog v-model="modifyVisible" title="修改实例配置" width="520px" destroy-on-close>
      <el-alert
        title="提示：修改 Shape 或资源规格可能需要先停止实例"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom:16px"
      />
      <el-form :model="modifyForm" label-width="100px">
        <el-form-item label="当前 Shape">
          <el-input :model-value="instance.shape" disabled />
        </el-form-item>
        <el-form-item label="显示名称">
          <el-input v-model="modifyForm.displayName" placeholder="修改实例显示名称" />
        </el-form-item>
        <el-form-item label="新 Shape">
          <el-input v-model="modifyForm.shape" placeholder="例如: VM.Standard.E4.Flex" />
        </el-form-item>
        <el-form-item label="OCPU">
          <el-input-number v-model="modifyForm.ocpus" :min="1" :max="128" :step="1" controls-position="right" style="width:100%" />
        </el-form-item>
        <el-form-item label="内存(GB)">
          <el-input-number v-model="modifyForm.memoryInGbs" :min="1" :max="1024" :step="1" controls-position="right" style="width:100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="modifyVisible = false">取消</el-button>
        <el-button type="primary" :loading="modifySaving" @click="doModify">确认修改</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  ArrowLeft, VideoPlay, VideoPause, FirstAidKit, Monitor,
  Connection, Edit, Link,
} from '@element-plus/icons-vue'
import request from '../utils/request'
import StatusBadge from '../components/common/StatusBadge.vue'
import MonoText from '../components/common/MonoText.vue'
import InstanceTrafficPanel from '../components/instance/InstanceTrafficPanel.vue'

defineOptions({ name: 'instance-detail' })

interface Instance {
  id: number; tenantId: number; tenantName: string
  instanceId: string; displayName: string; shape: string; state: string
  ocpus: number; memoryInGbs: number; bootVolumeSizeInGbs: number
  publicIps: string; privateIps: string; availabilityDomain: string
  compartmentId: string; bootVolumeId: string; bootVolumeName: string
  vpusPerGb: string; ipv6Addresses: string; vnicIds: string
  architecture: string; onLineEnable: number; lastHeartbeat: string; createTime: string
}

const route = useRoute()
const router = useRouter()
const instanceId = Number(route.params.id)

// --- state ---
const loading = ref(false)
const instance = ref<Instance>({} as Instance)
const activeTab = ref('overview')
const activePanels = ref('info')
const actionLoading = ref(false)

// disk
const diskLoading = ref(false)
const diskForm = ref({ vpusPerGb: 10 })
const vpuSaving = ref(false)

// network
const changeIpVisible = ref(false)
const changeIpLoading = ref(false)
const ipv6Visible = ref(false)
const ipv6Loading = ref(false)

// ssh
const sshLoading = ref(false)
const sshForm = ref({ username: 'root', port: 22, password: '' })
const sshSaving = ref(false)

// remark
const remarkLoading = ref(false)
const remarkText = ref('')
const remarkSaving = ref(false)

// modify
const modifyVisible = ref(false)
const modifySaving = ref(false)
const modifyForm = ref({ shape: '', ocpus: 4, memoryInGbs: 24, displayName: '' })

// --- helpers ---
function getStateStatus(state: string): 'up' | 'down' | 'warn' | 'idle' {
  const s = (state || '').toLowerCase()
  if (s === 'running') return 'up'
  if (s === 'stopped' || s === 'terminated') return 'down'
  if (s === 'starting' || s === 'stopping') return 'warn'
  return 'idle'
}

// --- data loading ---
async function loadInstance() {
  loading.value = true
  try {
    const res = await request.get('/instances/list', { params: { limit: 9999, offset: 0 } }) as any
    const items: Instance[] = res.items || []
    const found = items.find(i => i.id === instanceId)
    if (found) {
      instance.value = { ...found }
    } else {
      ElMessage.error('未找到实例')
      router.push('/instances')
    }
  } catch (e: any) {
    ElMessage.error('加载实例失败: ' + e.message)
  } finally {
    loading.value = false
  }
}

async function loadDisk() {
  diskLoading.value = true
  try {
    diskForm.value = {
      vpusPerGb: parseInt(instance.value.vpusPerGb) || 10,
    }
  } finally {
    diskLoading.value = false
  }
}

async function loadSsh() {
  sshLoading.value = true
  try {
    const ssh = await request.get(`/instances/${instance.value.id}/ssh-config`) as any
    sshForm.value = {
      username: ssh.username || 'root',
      port: ssh.port || 22,
      password: '',
    }
  } catch {
    sshForm.value = { username: 'root', port: 22, password: '' }
  } finally {
    sshLoading.value = false
  }
}

async function loadRemark() {
  remarkLoading.value = true
  try {
    // Remark is part of instance data; we just show a textarea for editing
    remarkText.value = ''
  } finally {
    remarkLoading.value = false
  }
}

function onPanelChange(name: string | number | (string | number)[]) {
  const panel = Array.isArray(name) ? String(name[0] || '') : String(name || '')
  if (!panel) return
  if (panel === 'disk') loadDisk()
  if (panel === 'ssh') loadSsh()
  if (panel === 'remark') loadRemark()
}

// --- actions ---
function handleAction(cmd: string) {
  switch (cmd) {
    case 'rescue':
      router.push({ path: '/rescue', query: { instanceId: instance.value.instanceId } })
      return
    case 'console':
      router.push({ path: '/console', query: { instanceId: instance.value.instanceId } })
      return
    case 'terminal':
      router.push({ path: '/terminal', query: { host: instance.value.publicIps || '' } })
      return
    case 'modify':
      openModify()
      return
    case 'change-ip':
      changeIpVisible.value = true
      return
    case 'enable-ipv6':
      ipv6Visible.value = true
      return
  }
}

// --- start/stop ---
async function confirmStart() {
  try {
    await ElMessageBox.confirm(`确定启动实例 ${instance.value.displayName}？`, '确认启动', { type: 'info' })
    actionLoading.value = true
    await request.post(`/instances/${instance.value.id}/start`)
    ElMessage.success('启动请求已发送')
    await loadInstance()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  } finally {
    actionLoading.value = false
  }
}

async function confirmStop() {
  try {
    await ElMessageBox.confirm(`确定停止实例 ${instance.value.displayName}？`, '确认停止', { type: 'warning' })
    actionLoading.value = true
    await request.post(`/instances/${instance.value.id}/stop`)
    ElMessage.success('停止请求已发送')
    await loadInstance()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  } finally {
    actionLoading.value = false
  }
}

// --- disk ---
async function saveVpu() {
  if (!instance.value.id) return
  vpuSaving.value = true
  try {
    await request.post(`/instances/${instance.value.id}/vpu`, {
      vpusPerGb: diskForm.value.vpusPerGb,
    })
    ElMessage.success('VPU 设置已更新')
    await loadInstance()
  } catch (e: any) {
    ElMessage.error(e.message || '更新 VPU 失败')
  } finally {
    vpuSaving.value = false
  }
}

// --- network ---
async function doChangeIp() {
  if (!instance.value.id) return
  changeIpLoading.value = true
  try {
    const res = await request.post(`/instances/${instance.value.id}/change-ip`) as any
    ElMessage.success(`IP 已更换: ${res.oldIp} → ${res.newIp}`)
    changeIpVisible.value = false
    await loadInstance()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    changeIpLoading.value = false
  }
}

async function doEnableIpv6() {
  if (!instance.value.id) return
  ipv6Loading.value = true
  try {
    const res = await request.post(`/instances/${instance.value.id}/enable-ipv6`) as any
    ElMessage.success('IPv6 已启用: ' + (res?.ipv6Address || ''))
    ipv6Visible.value = false
    await loadInstance()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    ipv6Loading.value = false
  }
}

// --- ssh ---
async function saveSshConfig() {
  if (!instance.value.id) return
  if (!sshForm.value.username) { ElMessage.warning('用户名不能为空'); return }
  sshSaving.value = true
  try {
    await request.post(`/instances/${instance.value.id}/ssh-config`, {
      username: sshForm.value.username,
      port: sshForm.value.port,
      password: sshForm.value.password,
    })
    ElMessage.success('SSH 配置已保存')
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    sshSaving.value = false
  }
}

// --- remark ---
async function saveRemark() {
  if (!instance.value.id) return
  remarkSaving.value = true
  try {
    await request.post(`/instances/${instance.value.id}/remark`, { remark: remarkText.value })
    ElMessage.success('备注已更新')
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    remarkSaving.value = false
  }
}

// --- modify ---
function openModify() {
  modifyForm.value = {
    shape: instance.value.shape || '',
    ocpus: instance.value.ocpus || 4,
    memoryInGbs: instance.value.memoryInGbs || 24,
    displayName: instance.value.displayName || '',
  }
  modifyVisible.value = true
}

async function doModify() {
  if (!instance.value.id) return
  modifySaving.value = true
  try {
    const body: any = {}
    if (modifyForm.value.shape && modifyForm.value.shape !== instance.value.shape) body.shape = modifyForm.value.shape
    if (modifyForm.value.ocpus !== instance.value.ocpus) body.ocpus = modifyForm.value.ocpus
    if (modifyForm.value.memoryInGbs !== instance.value.memoryInGbs) body.memoryInGbs = modifyForm.value.memoryInGbs
    if (modifyForm.value.displayName && modifyForm.value.displayName !== instance.value.displayName) {
      body.displayName = modifyForm.value.displayName
    }
    if (Object.keys(body).length === 0) {
      ElMessage.warning('没有需要修改的配置')
      modifySaving.value = false
      return
    }
    await request.post(`/instances/${instance.value.id}/modify`, body)
    ElMessage.success('修改请求已提交，实例正在更新中')
    modifyVisible.value = false
    await loadInstance()
  } catch (e: any) {
    ElMessage.error(e.message || '修改失败')
  } finally {
    modifySaving.value = false
  }
}

// --- lifecycle ---
onMounted(async () => {
  await loadInstance()
  // Wire up panel change for lazy loading
})
</script>

<style scoped>
.instance-detail-page {
  padding: 20px;
}

.detail-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 12px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-left h2 {
  margin: 0;
  font-size: var(--text-xl);
  font-weight: var(--font-bold);
}

.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.action-buttons {
  display: flex;
  gap: var(--space-3);
  flex-wrap: wrap;
}

.panel-title {
  font-size: var(--text-md);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

:deep(.el-collapse-item__header) {
  font-size: var(--text-md);
  font-weight: var(--font-semibold);
}

:deep(.el-collapse-item__content) {
  padding-bottom: var(--space-4);
}

:deep(.el-descriptions) {
  margin-bottom: 16px;
}

:deep(.el-table) {
  border-radius: var(--radius-md);
  overflow: hidden;
}

:deep(.el-table th) {
  background: var(--bg-raised);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

.vpu-option {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 2px 0;
}

.vpu-label {
  font-weight: var(--font-medium);
  font-size: var(--text-sm);
}

.vpu-desc {
  font-size: var(--text-xs);
  color: var(--text-muted);
}
</style>
