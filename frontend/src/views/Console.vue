<template>
  <div class="console-page">
    <el-card class="connect-card" shadow="hover">
      <el-form :model="form" label-width="100px" inline>
        <el-form-item label="实例">
          <el-select v-model="form.instanceId" placeholder="选择实例" style="width:420px"
            filterable clearable :loading="loadingInstances" @focus="loadInstances"
            @change="onInstanceChange">
            <el-option v-for="inst in instances" :key="inst.instanceId"
              :label="`${inst.displayName} (${inst.instanceId})`" :value="inst.instanceId" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="connecting" :disabled="!form.instanceId" @click="connectSerial">
            {{ connecting ? '连接中...' : '连接串口' }}
          </el-button>
          <el-button :disabled="!connected" type="danger" @click="disconnect">断开</el-button>
        </el-form-item>
      </el-form>

      <!-- Existing console connections (management: list + delete) -->
      <div v-if="form.instanceId" class="connections-section">
        <div class="connections-header">
          <span>已有控制台连接</span>
          <el-button size="small" text @click="loadConnections">刷新</el-button>
        </div>
        <el-table v-loading="loadingConns" :data="connections" size="small" style="margin-top:8px"
          empty-text="无控制台连接（连接串口时会自动创建）">
          <el-table-column label="连接 ID" min-width="240">
            <template #default="{ row }"><span :title="row.connId">{{ shortConn(row.connId) }}</span></template>
          </el-table-column>
          <el-table-column label="状态" prop="lifecycleState" width="110" />
          <el-table-column label="来源" width="100">
            <template #default="{ row }">
              <el-tag v-if="row.isOurs" type="success" size="small">本应用</el-tag>
              <el-tag v-else type="info" size="small">其他</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120">
            <template #default="{ row }">
              <el-button size="small" type="danger" :loading="deletingConn === row.connId"
                @click="deleteConnection(row.connId)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div v-if="statusLog" ref="statusLogEl" class="status-log"><pre>{{ statusLog }}</pre></div>
    </el-card>

    <el-card v-show="connected || connecting" class="term-card" shadow="hover">
      <template #header><span>串口控制台 - {{ form.instanceId }}</span></template>
      <div ref="terminal" class="terminal-container" />
    </el-card>

    <el-alert v-if="error" type="error" :title="error" show-icon closable @close="error = ''" style="margin-top:16px" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onActivated, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../utils/request'

// name matches <keep-alive :include="['console']"> in Default.vue so the
// serial WS + xterm survive tab switches instead of tearing down.
defineOptions({ name: 'console' })

const form = ref({ instanceId: '' })
const connecting = ref(false)
const connected = ref(false)
const error = ref('')
const statusLog = ref('')
const loadingInstances = ref(false)
const loadingConns = ref(false)
const deletingConn = ref('')
const instances = ref<any[]>([])
const connections = ref<any[]>([])
const terminal = ref<HTMLElement>()
const statusLogEl = ref<HTMLElement>()

let term: any = null
let ws: WebSocket | null = null
let fitAddon: any = null
let resizeObserver: ResizeObserver | null = null

function getWsUrl(path: string): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${location.host}${path}`
}
function shortConn(id: string): string { return id && id.length > 28 ? id.slice(0, 28) + '…' : (id || '') }

async function loadInstances() {
  if (loadingInstances.value) return
  loadingInstances.value = true
  try {
    const res = await request.get('/instances/list', { params: { limit: 200, offset: 0 } }) as any
    instances.value = res.items || []
  } catch (e: any) { ElMessage.error('加载实例失败: ' + e.message) }
  finally { loadingInstances.value = false }
}

async function loadConnections() {
  if (!form.value.instanceId) { connections.value = []; return }
  loadingConns.value = true
  try {
    // request interceptor already unwraps ApiResponse.data, so the response
    // IS the views array (not {data: [...]}).
    const res = await request.get(`/instances/${form.value.instanceId}/console-connections`) as any
    connections.value = Array.isArray(res) ? res : (Array.isArray(res?.data) ? res.data : [])
  } catch (e: any) { ElMessage.error('加载控制台连接失败: ' + e.message); connections.value = [] }
  finally { loadingConns.value = false }
}

function onInstanceChange() {
  if (connected.value || connecting.value) disconnect()
  loadConnections()
}

async function deleteConnection(connId: string) {
  deletingConn.value = connId
  try {
    await request.delete(`/instances/${form.value.instanceId}/console-connections/${connId}`)
    // Optimistic removal for instant feedback; the OCI delete is async (DELETING→DELETED)
    // so a refresh right away would still show it. Refresh after a delay, by which
    // time it's DELETED and filtered out of the list.
    connections.value = connections.value.filter(c => c.connId !== connId)
    ElMessage.success('已删除')
    setTimeout(loadConnections, 3000)
  } catch (e: any) { ElMessage.error('删除失败: ' + e.message) }
  finally { deletingConn.value = '' }
}

async function initTerminal() {
  await nextTick()
  if (!terminal.value || term) return
  try {
    const { Terminal } = await import('@xterm/xterm')
    const { FitAddon } = await import('@xterm/addon-fit')
    term = new Terminal({
      cursorBlink: true, fontSize: 14, convertEol: true, scrollback: 5000, tabStopWidth: 4,
      fontFamily: '"Cascadia Code", Menlo, Monaco, "Courier New", monospace',
      theme: { background: '#1e1e1e', foreground: '#d4d4d4', cursor: '#d4d4d4' },
    })
    fitAddon = new FitAddon()
    term.loadAddon(fitAddon)
    term.open(terminal.value)
    fitAddon.fit()
    term.onData((data: string) => {
      if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'input', data }))
    })
    term.onResize(({ cols, rows }: { cols: number; rows: number }) => {
      if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'resize', data: { cols, rows } }))
    })
    resizeObserver = new ResizeObserver(() => fitAddon?.fit())
    resizeObserver.observe(terminal.value)
  } catch {
    if (terminal.value) terminal.value.innerHTML = '<p style="padding:20px;color:#999">请安装 xterm.js: npm install @xterm/xterm @xterm/addon-fit</p>'
  }
}

function appendStatus(msg: string) {
  statusLog.value += msg + '\n'
  nextTick(() => { if (statusLogEl.value) statusLogEl.value.scrollTop = statusLogEl.value.scrollHeight })
}

async function connectSerial() {
  if (!form.value.instanceId) return
  await initTerminal()
  if (!term) return
  connecting.value = true
  connected.value = false
  error.value = ''
  statusLog.value = '正在连接串口控制台...\n'
  if (term) term.clear?.()

  ws = new WebSocket(getWsUrl(`/ws/console/serial?instanceId=${encodeURIComponent(form.value.instanceId)}`))
  ws.binaryType = 'arraybuffer'

  ws.onmessage = (e) => {
    if (typeof e.data === 'string') {
      // Control JSON: {"type":"output"|"error", ...} vs raw terminal bytes.
      // The serial backend sends stdout as binary frames + control as text JSON.
      // xterm keystroke echo is binary; text frames are control messages.
      try {
        const m = JSON.parse(e.data)
        if (m.type === 'output') { appendStatus(m.data || ''); return }
        if (m.type === 'error') { connecting.value = false; error.value = m.message || 'Unknown error'; appendStatus('错误: ' + error.value); return }
      } catch { /* not JSON → fall through to term.write */ }
      term.write(e.data)
    } else if (e.data instanceof ArrayBuffer) {
      term.write(new Uint8Array(e.data))
    }
  }
  ws.onopen = () => { connecting.value = false; connected.value = true; appendStatus('串口控制台已连接') }
  ws.onerror = () => { connecting.value = false; error.value = 'WebSocket 连接失败' }
  ws.onclose = () => {
    connecting.value = false
    if (connected.value) { connected.value = false; appendStatus('串口连接已断开') }
    loadConnections()
  }
  // Refresh the connections list shortly after connect so the newly-created
  // (or reused) OCI connection shows up while the session is active.
  setTimeout(loadConnections, 5000)
}

function disconnect() {
  if (ws) { try { ws.close() } catch { /* closing */ }; ws = null }
  connected.value = false
  statusLog.value += '已断开\n'
}

watch(() => form.value.instanceId, () => loadConnections())
watch(statusLog, () => nextTick(() => { if (statusLogEl.value) statusLogEl.value.scrollTop = statusLogEl.value.scrollHeight }))

onMounted(() => loadInstances())
onActivated(() => loadConnections())
onBeforeUnmount(() => disconnect())
</script>

<style scoped>
.console-page { padding: 20px; }
.connect-card { margin-bottom: 16px; }
.connections-section { margin-top: 12px; }
.connections-header { display: flex; align-items: center; justify-content: space-between; font-weight: 600; color: #606266; }
.status-log {
  margin-top: 12px; background: #1e1e1e; color: #d4d4d4; padding: 12px;
  border-radius: 4px; font-family: 'Courier New', monospace; font-size: 12px;
  max-height: 140px; overflow-y: auto;
}
.status-log pre { margin: 0; white-space: pre-wrap; word-break: break-all; }
.term-card { margin-bottom: 16px; }
.terminal-container {
  width: 100%; height: 560px; background: #1e1e1e; padding: 4px; border-radius: 4px;
}
.term-card :deep(.el-card__body) { padding: 0; }
</style>
