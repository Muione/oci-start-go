<template>
  <div class="console-page">
    <el-card class="connect-card" shadow="hover">
      <el-form :model="form" label-width="100px" inline>
        <el-form-item label="实例">
          <el-select v-model="form.instanceId" placeholder="选择实例" style="width:420px"
            filterable clearable :loading="loadingInstances" @focus="loadInstances">
            <el-option v-for="inst in instances" :key="inst.instanceId"
              :label="`${inst.displayName} (${inst.instanceId})`" :value="inst.instanceId" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="connecting" :disabled="!form.instanceId" @click="connectConsole">
            {{ connecting ? '连接中...' : '连接' }}
          </el-button>
          <el-button :disabled="!connected" @click="disconnect">断开</el-button>
        </el-form-item>
      </el-form>
      <!-- Connection status log -->
      <div v-if="statusLog" class="status-log">
        <pre>{{ statusLog }}</pre>
      </div>
    </el-card>

    <!-- noVNC canvas container -->
    <el-card v-show="connected" class="vnc-card" shadow="hover">
      <template #header>
        <div style="display:flex;align-items:center;justify-content:space-between">
          <span>VNC Console - {{ vncInfo.instanceId }}</span>
          <div style="display:flex;align-items:center;gap:8px">
            <el-tag type="success" size="small">已连接</el-tag>
            <el-button size="small" @click="toggleFullscreen" title="全屏">
              <el-icon><FullScreen /></el-icon>
            </el-button>
            <el-button size="small" type="danger" @click="disconnect" title="断开">
              断开
            </el-button>
          </div>
        </div>
      </template>
      <div ref="vncContainer" class="vnc-container" />
    </el-card>

    <!-- Usage instructions when not connected -->
    <el-card v-if="!connected" shadow="hover">
      <template #header>使用说明</template>
      <el-steps direction="vertical" :active="1" process-status="finish" finish-status="success">
        <el-step title="选择目标实例" description="从下拉列表中选择要远程控制的 OCI 实例" />
        <el-step title="建立连接" description="点击「连接」通过 SSH 隧道建立 VNC 连接" />
        <el-step title="远程操作" description="连接成功后即可在浏览器中直接操作实例桌面" />
        <el-step title="断开会话" description="操作完成后点击「断开」释放资源" />
      </el-steps>
    </el-card>

    <el-alert v-if="error" type="error" :title="error" show-icon closable @close="error = ''" style="margin-top:16px" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { FullScreen } from '@element-plus/icons-vue'
import request from '../utils/request'

// noVNC RFB class - will be imported dynamically
let RFB: any = null

const form = ref({ instanceId: '' })
const connecting = ref(false)
const connected = ref(false)
const error = ref('')
const statusLog = ref('')
const loadingInstances = ref(false)
const instances = ref<any[]>([])
const vncContainer = ref<HTMLElement>()
const vncInfo = ref({ instanceId: '' })

let controlWs: WebSocket | null = null
let rfb: any = null

function getWsUrl(path: string): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${location.host}${path}`
}

async function loadInstances() {
  if (loadingInstances.value) return
  loadingInstances.value = true
  try {
    const res = await request.get('/instances/list', { params: { limit: 200, offset: 0 } }) as any
    instances.value = res.items || []
  } catch (e: any) {
    ElMessage.error('加载实例失败: ' + e.message)
  } finally {
    loadingInstances.value = false
  }
}

async function connectConsole() {
  if (!form.value.instanceId) return
  connecting.value = true
  error.value = ''
  statusLog.value = '正在连接...\n'

  // Dynamically import noVNC
  if (!RFB) {
    try {
      const novnc = await import('@novnc/novnc')
      RFB = novnc.default
    } catch (e: any) {
      error.value = 'Failed to load noVNC: ' + e.message
      connecting.value = false
      return
    }
  }

  controlWs = new WebSocket(getWsUrl('/ws/console'))

  controlWs.onopen = () => {
    controlWs!.send(JSON.stringify({
      type: 'create_connection',
      data: { instanceId: form.value.instanceId, tenantId: 0 },
    }))
  }

  controlWs.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      switch (msg.type) {
        case 'vnc_ready':
          connecting.value = false
          connected.value = true
          vncInfo.value.instanceId = msg.instanceId
          statusLog.value += 'VNC 就绪，正在加载显示器...\n'
          nextTick(() => initNoVNC(msg.vncWsUrl))
          break
        case 'output':
          statusLog.value += msg.data || ''
          break
        case 'error':
          connecting.value = false
          error.value = msg.message || msg.data || 'Unknown error'
          statusLog.value += '错误: ' + error.value + '\n'
          break
        case 'pong':
          break
      }
    } catch (e) {
      // ignore parse errors
    }
  }

  controlWs.onclose = () => {
    connecting.value = false
    if (connected.value) {
      connected.value = false
      cleanupNoVNC()
      statusLog.value += '连接已断开\n'
    }
  }

  controlWs.onerror = () => {
    connecting.value = false
    error.value = 'WebSocket 连接失败'
  }
}

function initNoVNC(wsPath: string) {
  if (!vncContainer.value) return
  cleanupNoVNC()

  const url = getWsUrl(wsPath)
  try {
    rfb = new RFB(vncContainer.value, url, {
      credentials: { password: '' },
    })

    rfb.resizeSession = true
    rfb.scaleViewport = true
    rfb.background = '#000000'

    rfb.addEventListener('connect', () => {
      ElMessage.success('VNC 连接成功')
      statusLog.value += 'VNC 显示器已加载\n'
    })

    rfb.addEventListener('disconnect', (_e: any) => {
      if (connected.value) {
        ElMessage.info('VNC 已断开')
        connected.value = false
      }
    })

    rfb.addEventListener('credentialsrequired', () => {
      // OCI console doesn't need VNC password
    })
  } catch (e: any) {
    error.value = 'Failed to initialize noVNC: ' + e.message
  }
}

function cleanupNoVNC() {
  if (rfb) {
    try { rfb.disconnect() } catch (_e) {}
    rfb = null
  }
}

function disconnect() {
  cleanupNoVNC()
  if (controlWs && controlWs.readyState === WebSocket.OPEN) {
    controlWs.send(JSON.stringify({
      type: 'disconnect',
      data: { instanceId: form.value.instanceId },
    }))
    controlWs.close()
  }
  controlWs = null
  connected.value = false
  statusLog.value += '已断开连接\n'
}

function toggleFullscreen() {
  if (!vncContainer.value) return
  if (document.fullscreenElement) {
    document.exitFullscreen()
  } else {
    vncContainer.value.requestFullscreen()
  }
}

onMounted(() => loadInstances())
onBeforeUnmount(() => disconnect())
</script>

<style scoped>
.console-page {
  padding: 20px;
}
.connect-card {
  margin-bottom: 16px;
}
.status-log {
  margin-top: 12px;
  background: #1e1e1e;
  color: #d4d4d4;
  padding: 12px;
  border-radius: 4px;
  font-family: 'Courier New', monospace;
  font-size: 12px;
  max-height: 150px;
  overflow-y: auto;
}
.status-log pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
}
.vnc-container {
  width: 100%;
  height: 600px;
  background: #000;
  border-radius: 4px;
  overflow: hidden;
}
.vnc-card :deep(.el-card__body) {
  padding: 0;
}
</style>
