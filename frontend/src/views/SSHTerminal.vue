<template>
  <div>
    <div class="toolbar">
      <h2>SSH 终端</h2>
      <el-button @click="showConnect = !showConnect">
        {{ showConnect ? '隐藏' : '新建连接' }}
      </el-button>
      <el-button @click="disconnect" :disabled="!connected" type="danger">断开</el-button>
    </div>

    <el-card v-if="showConnect" class="connect-card" shadow="hover">
      <el-form :model="form" label-width="80px" inline>
        <el-form-item label="主机">
          <el-input v-model="form.host" placeholder="IP 地址" style="width:180px" />
        </el-form-item>
        <el-form-item label="端口">
          <el-input-number v-model="form.port" :min="1" :max="65535" style="width:120px" />
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model="form.username" placeholder="root" style="width:140px" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" placeholder="密码" style="width:160px" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="connecting" @click="connect">连接</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <div ref="terminal" class="terminal-container" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage } from 'element-plus'

const terminal = ref<HTMLElement | null>(null)
const showConnect = ref(true)
const connecting = ref(false)
const connected = ref(false)
const form = ref({ host: '', port: 22, username: 'root', password: '' })

let term: any = null
let ws: WebSocket | null = null
let fitAddon: any = null

function getTermSize() {
  const el = terminal.value
  if (!el) return { cols: 80, rows: 24 }
  // Estimate cols/rows based on container size and font metrics
  const fontW = 8.4  // approximate width of monospace 14px char
  const fontH = 17   // approximate height including line spacing
  const w = el.clientWidth
  const h = el.clientHeight
  return {
    cols: Math.max(20, Math.floor((w - 20) / fontW)),
    rows: Math.max(10, Math.floor((h - 20) / fontH)),
  }
}

async function initTerminal() {
  await nextTick()
  if (!terminal.value) return

  try {
    const { Terminal } = await import('@xterm/xterm')
    const { FitAddon } = await import('@xterm/addon-fit')

    const size = getTermSize()
    term = new Terminal({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: '"Cascadia Code", Menlo, Monaco, "Courier New", monospace',
      theme: {
        background: '#1e1e1e',
        foreground: '#d4d4d4',
        cursor: '#d4d4d4',
        selectionBackground: '#264f78',
        black: '#000000', red: '#cd3131', green: '#0dbc79', yellow: '#e5e510',
        blue: '#2472c8', magenta: '#bc3fbc', cyan: '#11a8cd', white: '#e5e5e5',
        brightBlack: '#666666', brightRed: '#f14c4c', brightGreen: '#23d18b',
        brightYellow: '#f5f543', brightBlue: '#3b8eea', brightMagenta: '#d670d6',
        brightCyan: '#29b8db', brightWhite: '#ffffff',
      },
      cols: size.cols,
      rows: size.rows,
      convertEol: true,
      allowProposedApi: true,
      scrollback: 5000,
      tabStopWidth: 4,
    })

    fitAddon = new FitAddon()
    term.loadAddon(fitAddon)

    term.open(terminal.value)
    fitAddon.fit()

    // Handle container resize
    const resizeObserver = new ResizeObserver(() => {
      fitAddon?.fit()
    })
    resizeObserver.observe(terminal.value)
  } catch {
    if (terminal.value) {
      terminal.value.innerHTML = '<p style="padding:20px;color:#999">请安装 xterm.js: npm install @xterm/xterm @xterm/addon-fit</p>'
    }
    return
  }
}

function connect() {
  if (!term) return
  if (!form.value.host) { ElMessage.warning('请输入主机地址'); return }
  connecting.value = true

  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = `${proto}//${location.host}/ws/ssh`
  ws = new WebSocket(url)
  // Accept both text and binary frames
  ws.binaryType = 'arraybuffer'

  // Set up terminal → WS before connecting, so we don't miss initial output
  term.onData((data: string) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'input', data }))
    }
  })

  term.onResize(({ cols, rows }: { cols: number; rows: number }) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', data: { cols, rows } }))
    }
  })

  ws.onmessage = (e) => {
    if (typeof e.data === 'string') {
      term.write(e.data)
    } else if (e.data instanceof ArrayBuffer) {
      term.write(new Uint8Array(e.data))
    } else if (e.data instanceof Blob) {
      // Rare case: Blob (when binaryType is 'blob')
      const reader = new FileReader()
      reader.onload = () => {
        term.write(new Uint8Array(reader.result as ArrayBuffer))
      }
      reader.readAsArrayBuffer(e.data)
    }
  }

  ws.onopen = () => {
    connecting.value = false
    connected.value = true
    showConnect.value = false
    term.clear()
    ws!.send(JSON.stringify({
      type: 'connect',
      data: {
        host: form.value.host,
        port: form.value.port,
        username: form.value.username,
        password: form.value.password,
      },
    }))
  }

  ws.onerror = () => {
    connecting.value = false
    ElMessage.error('WebSocket 连接失败')
  }

  ws.onclose = () => {
    connected.value = false
    showConnect.value = true
    if (term) {
      term.write('\r\n\x1b[33m[连接已断开]\x1b[0m\r\n')
    }
  }
}

function disconnect() {
  if (ws) {
    ws.close()
    ws = null
    connected.value = false
  }
}

onMounted(initTerminal)
onBeforeUnmount(disconnect)
</script>

<style scoped>
.toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
.toolbar h2 { margin: 0; margin-right: auto; }
.connect-card { margin-bottom: 16px; }
.terminal-container { height: calc(100vh - 200px); min-height: 400px; }
.terminal-container :deep(.xterm) { height: 100%; padding: 8px; }
.terminal-container :deep(.xterm-viewport) { scrollbar-width: thin; }
</style>
