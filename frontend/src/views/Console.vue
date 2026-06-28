<template>
  <div>
    <div class="toolbar">
      <h2>VNC 控制台</h2>
      <el-button @click="loadInstances" :loading="loadingInstances">刷新实例列表</el-button>
    </div>

    <el-card class="connect-card" shadow="hover">
      <el-form :model="form" label-width="100px" inline>
        <el-form-item label="选择实例">
          <el-select
            v-model="form.instanceId"
            placeholder="请选择要远程控制的实例"
            style="width: 420px"
            filterable
            clearable
            :loading="loadingInstances"
            :filter-method="filterInstances"
            @focus="loadInstances"
          >
            <el-option
              v-for="inst in filteredInstances"
              :key="inst.instanceId"
              :label="`${inst.displayName} (${inst.instanceId})`"
              :value="inst.instanceId"
            >
              <div style="display:flex;justify-content:space-between;align-items:center">
                <span style="font-weight:500">{{ inst.displayName }}</span>
                <el-tag :type="inst.state === 'Running' ? 'success' : 'info'" size="small" style="margin-left:8px">
                  {{ inst.state || 'Unknown' }}
                </el-tag>
              </div>
              <div style="font-size:12px;color:var(--text-muted);margin-top:2px">
                {{ inst.instanceId }} · {{ inst.shape }} · {{ inst.publicIps || '无公网IP' }}
              </div>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="connecting" :disabled="!form.instanceId" @click="connectConsole">
            {{ connecting ? '连接中...' : '建立连接' }}
          </el-button>
          <el-button :disabled="!connected" @click="disconnect">断开</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card v-if="instDetail && form.instanceId" class="info-card" shadow="hover">
      <template #header>
        <span>实例信息</span>
      </template>
      <el-descriptions :column="3" border size="small">
        <el-descriptions-item label="名称">{{ instDetail.displayName || '-' }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="instDetail.state === 'Running' ? 'success' : 'info'" size="small">
            {{ instDetail.state || '-' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="规格">{{ instDetail.shape || '-' }}</el-descriptions-item>
        <el-descriptions-item label="公网 IP">{{ instDetail.publicIps || '-' }}</el-descriptions-item>
        <el-descriptions-item label="私网 IP">{{ instDetail.privateIps || '-' }}</el-descriptions-item>
        <el-descriptions-item label="可用域">{{ instDetail.availabilityDomain || '-' }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-card v-if="connected" class="status-card" shadow="hover">
      <template #header>
        <span>VNC 连接信息</span>
        <el-tag type="success" style="margin-left:8px">已连接</el-tag>
      </template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="实例">{{ vncInfo.instanceId }}</el-descriptions-item>
        <el-descriptions-item label="显示名称">{{ vncInfo.displayName || '-' }}</el-descriptions-item>
        <el-descriptions-item label="VNC 主机">{{ vncInfo.vncHost }}</el-descriptions-item>
        <el-descriptions-item label="VNC 端口">{{ vncInfo.vncPort }}</el-descriptions-item>
        <el-descriptions-item label="规格">{{ vncInfo.shape || '-' }}</el-descriptions-item>
      </el-descriptions>
      <div style="margin-top:16px; padding:12px; background:var(--bg-raised); border-radius:4px">
        <p>请使用 VNC 客户端连接 <code>{{ vncInfo.vncHost }}:{{ vncInfo.vncPort }}</code></p>
        <p style="color:var(--text-muted); font-size:13px">推荐使用 RealVNC、TigerVNC 或 TightVNC 客户端</p>
        <el-button size="small" style="margin-top:8px" @click="copyVNCAddress">
          <el-icon><CopyDocument /></el-icon> 复制地址
        </el-button>
      </div>
    </el-card>

    <el-card v-if="error" class="error-card" shadow="hover">
      <el-alert type="error" :title="error" show-icon :closable="true" @close="error = ''" />
    </el-card>

    <el-card v-if="!connected" shadow="hover">
      <template #header>使用说明</template>
      <el-steps direction="vertical" :active="1" process-status="finish" finish-status="success">
        <el-step title="选择目标实例" description="从下拉列表中选择要远程控制的 OCI 实例" />
        <el-step title="建立连接" description="点击「建立连接」通过 SSH 隧道代理 VNC 流量" />
        <el-step title="VNC 客户端连接" description="使用返回的 VNC 地址和端口连接，即可远程操作实例桌面" />
        <el-step title="断开会话" description="操作完成后点击「断开」释放资源" />
      </el-steps>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { CopyDocument } from '@element-plus/icons-vue'
import request from '../utils/request'

interface InstanceItem {
  id: number
  instanceId: string
  displayName: string
  shape: string
  state: string
  publicIps: string
  privateIps: string
  availabilityDomain: string
  ocpus: number
  memoryInGbs: number
}

const route = useRoute()

const form = ref({ instanceId: '' })
const connecting = ref(false)
const connected = ref(false)
const error = ref('')
const loadingInstances = ref(false)
const instances = ref<InstanceItem[]>([])
const searchText = ref('')
const vncInfo = ref({
  instanceId: '',
  displayName: '',
  vncHost: '127.0.0.1',
  vncPort: 0,
  shape: '',
})

// Pre-fill instanceId from route query (from Instances.vue detail dialog)
onMounted(() => {
  const qId = route.query.instanceId as string
  if (qId) {
    form.value.instanceId = qId
  }
  loadInstances()
})

async function loadInstances() {
  if (loadingInstances.value) return
  loadingInstances.value = true
  try {
    const res = await request.get('/instances/list', { params: { limit: 200, offset: 0 } }) as any
    instances.value = res.items || []
  } catch (e: any) {
    ElMessage.error('加载实例列表失败: ' + (e.message || e))
  } finally {
    loadingInstances.value = false
  }
}

const filteredInstances = computed(() => {
  if (!searchText.value) return instances.value
  const q = searchText.value.toLowerCase()
  return instances.value.filter(inst =>
    inst.displayName.toLowerCase().includes(q) ||
    inst.instanceId.toLowerCase().includes(q) ||
    (inst.publicIps && inst.publicIps.toLowerCase().includes(q)) ||
    (inst.shape && inst.shape.toLowerCase().includes(q))
  )
})

function filterInstances(query: string) {
  searchText.value = query
}

// Compute detail for the selected instance
const instDetail = computed(() => {
  return instances.value.find(i => i.instanceId === form.value.instanceId) || null
})

let ws: WebSocket | null = null

function getWsUrl(): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${location.host}/ws/console`
}

async function connectConsole() {
  if (!form.value.instanceId.trim()) {
    ElMessage.warning('请先选择一个实例')
    return
  }
  connecting.value = true
  error.value = ''

  try {
    ws = new WebSocket(getWsUrl())

    ws.onopen = () => {
      ws!.send(JSON.stringify({
        type: 'create_connection',
        data: { instanceId: form.value.instanceId, tenantId: 0 },
      }))
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        switch (msg.type) {
          case 'connection_created':
            connecting.value = false
            connected.value = true
            vncInfo.value = {
              instanceId: msg.instanceId || form.value.instanceId,
              displayName: msg.message || '',
              vncHost: msg.vncHost || '127.0.0.1',
              vncPort: msg.vncPort || 0,
              shape: instDetail.value?.shape || '',
            }
            ElMessage.success('VNC 连接已建立')
            break
          case 'error':
            connecting.value = false
            error.value = msg.message
            ElMessage.error(msg.message)
            break
          case 'pong':
            break
          default:
            break
        }
      } catch { /* ignore parse errors */ }
    }

    ws.onclose = () => {
      connecting.value = false
      if (connected.value) {
        connected.value = false
        ElMessage.info('VNC 连接已断开')
      }
    }

    ws.onerror = () => {
      connecting.value = false
      error.value = 'WebSocket 连接失败'
      ElMessage.error('WebSocket 连接失败')
    }
  } catch (err: any) {
    connecting.value = false
    error.value = err?.message || '连接失败'
  }
}

function disconnect() {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({
      type: 'disconnect',
      data: { instanceId: form.value.instanceId },
    }))
    ws.close()
  }
  ws = null
  connected.value = false
}

function copyVNCAddress() {
  const addr = `${vncInfo.value.vncHost}:${vncInfo.value.vncPort}`
  navigator.clipboard.writeText(addr).then(() => {
    ElMessage.success('已复制: ' + addr)
  }).catch(() => {
    ElMessage.info('VNC 地址: ' + addr)
  })
}

onBeforeUnmount(() => {
  disconnect()
})
</script>

<style scoped>
.toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
.toolbar h2 { margin: 0; margin-right: auto; }
.connect-card { margin-bottom: 16px; }
.info-card { margin-bottom: 16px; }
.status-card { margin-bottom: 16px; }
.error-card { margin-bottom: 16px; }
</style>
