<template>
  <div class="ssh-terminal-page">
    <!-- Tab bar -->
    <div class="tab-bar">
      <div
        v-for="s in sessions"
        :key="s.id"
        class="tab-item"
        :class="{ active: s.id === activeId }"
        @click="activate(s.id)"
      >
        <span class="dot" :class="{ on: s.connected, connecting: s.connecting }" />
        <span class="tab-title">{{ s.title || '新连接' }}</span>
        <span class="tab-close" @click.stop="closeSession(s.id)">×</span>
      </div>
      <button class="tab-new" @click="newSession" title="新建连接">+</button>
    </div>

    <!-- Connect form (per active session) -->
    <el-card v-if="active && !active.connected" class="connect-card" shadow="hover">
      <el-form :model="active.form" label-width="80px" inline>
        <el-form-item label="实例">
          <el-select
            v-model="active.form.instanceId"
            placeholder="选择实例自动填充（可选）"
            clearable filterable
            :loading="loadingInstances"
            style="width:340px"
            @focus="loadInstances"
            @change="onPickInstance(active)"
          >
            <el-option
              v-for="inst in instances"
              :key="inst.instanceId"
              :label="`${inst.displayName} (${inst.instanceId})`"
              :value="inst.instanceId"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="主机">
          <el-input v-model="active.form.host" placeholder="IP 地址" style="width:180px" />
        </el-form-item>
        <el-form-item label="端口">
          <el-input-number v-model="active.form.port" :min="1" :max="65535" style="width:120px" />
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model="active.form.username" placeholder="root" style="width:140px" />
        </el-form-item>
        <el-form-item label="认证">
          <el-radio-group v-model="active.form.authType">
            <el-radio value="password">密码</el-radio>
            <el-radio value="key">密钥</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="active.form.authType === 'password'" label="密码">
          <el-input v-model="active.form.password" type="password" placeholder="密码" show-password style="width:160px" />
        </el-form-item>
        <template v-else>
          <el-form-item label="密钥">
            <el-select v-model="active.form.savedKeyId" placeholder="选择已存密钥" clearable style="width:200px" @change="onSavedKeyChange(active)">
              <el-option v-for="k in savedKeys" :key="k.id" :label="k.label" :value="k.id" />
            </el-select>
            <el-button size="small" @click="keyDialogVisible = true" style="margin-left:8px">管理密钥</el-button>
          </el-form-item>
          <el-form-item label="私钥">
            <el-input v-model="active.form.privateKey" type="textarea" :rows="3" placeholder="粘贴 PEM 私钥（或选择已存密钥）" style="width:360px" />
          </el-form-item>
          <el-form-item label="口令">
            <el-input v-model="active.form.passphrase" type="password" placeholder="私钥口令（可选）" show-password style="width:160px" />
          </el-form-item>
        </template>
        <el-form-item>
          <el-button type="primary" :loading="active.connecting" @click="connect(active)">连接</el-button>
        </el-form-item>
      </el-form>

      <div v-if="recents.length" class="recents">
        <span class="recents-label">最近连接：</span>
        <el-tag
          v-for="r in recents"
          :key="r.key"
          class="recent-tag"
          size="small"
          @click="applyRecent(active, r)"
        >{{ r.username }}@{{ r.host }}:{{ r.port }}</el-tag>
      </div>
    </el-card>

    <!-- Saved keys management dialog -->
    <el-dialog v-model="keyDialogVisible" title="管理 SSH 私钥（DB 加密存储）" width="600px" @open="loadSavedKeys">
      <el-form :model="keyForm" label-width="80px">
        <el-form-item label="标签">
          <el-input v-model="keyForm.label" placeholder="如：实例 A root 密钥" />
        </el-form-item>
        <el-form-item label="私钥">
          <el-input v-model="keyForm.content" type="textarea" :rows="5" placeholder="粘贴 PEM 私钥（仅在保存时发送到服务端，加密存储）" />
        </el-form-item>
        <el-form-item label="口令">
          <el-input v-model="keyForm.passphrase" type="password" placeholder="私钥口令（可选）" show-password />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="keySaving" @click="addSavedKey">保存</el-button>
        </el-form-item>
      </el-form>
      <el-table :data="savedKeys" size="small" empty-text="无已存密钥">
        <el-table-column label="标签" prop="label" min-width="140" />
        <el-table-column label="指纹" prop="fingerprint" min-width="200" />
        <el-table-column label="操作" width="90">
          <template #default="{ row }">
            <el-button size="small" type="danger" @click="removeSavedKey(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top:8px;color:#909399;font-size:12px">私钥以 AES-256-GCM 加密存于数据库（master key），连接时服务端按 id 解密使用，内容不回前端。</div>
    </el-dialog>

    <!-- Terminal area -->
    <div class="terminals">
      <div
        v-for="s in sessions"
        :key="s.id"
        v-show="s.id === activeId"
        class="terminal-wrap"
      >
        <div class="term-toolbar">
          <span class="term-status">
            <span class="dot" :class="{ on: s.connected, connecting: s.connecting }" />
            {{ s.connected ? '已连接' : (s.connecting ? '连接中…' : '未连接') }}
          </span>
          <el-button size="small" text :disabled="!s.connected" @click="reconnect(s)">重连</el-button>
          <el-button size="small" text :disabled="!s.term" @click="clearTerm(s)">清屏</el-button>
          <el-button size="small" text :disabled="!s.term" @click="copySel(s)">复制</el-button>
          <el-button size="small" text @click="toggleFullscreen(s)">全屏</el-button>
          <el-button size="small" text type="danger" :disabled="!s.connected" @click="disconnect(s)">断开</el-button>
        </div>
        <div :ref="el => setTermEl(s.id, el as HTMLElement)" class="terminal-container" />
      </div>
    </div>

    <el-alert v-if="active && active.error" type="error" :title="active.error" show-icon closable
      @close="active.error = ''" style="margin-top:12px" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onActivated, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../utils/request'

defineOptions({ name: 'terminal' }) // keep-alive in Default.vue

interface SessionForm {
  instanceId: string; host: string; port: number; username: string; password: string
  authType: 'password' | 'key'; privateKey: string; passphrase: string; savedKeyId: string
}
interface Session {
  id: string
  title: string
  form: SessionForm
  term: any
  ws: WebSocket | null
  fitAddon: any
  resizeObserver: ResizeObserver | null
  connected: boolean
  connecting: boolean
  error: string
  el: HTMLElement | null
}
interface Recent { key: string; host: string; port: number; username: string }
// Saved keys live in the DB (encrypted at rest); the frontend only sees
// id/label/fingerprint — never the key content.
interface SavedKey { id: number; label: string; fingerprint: string }

const sessions = ref<Session[]>([])
const activeId = ref('')
const instances = ref<any[]>([])
const loadingInstances = ref(false)
const recents = ref<Recent[]>(loadRecents())
const savedKeys = ref<SavedKey[]>([])
const keyDialogVisible = ref(false)
const keyForm = ref({ label: '', content: '', passphrase: '' })
const keySaving = ref(false)

const active = computed(() => sessions.value.find(s => s.id === activeId.value) || null)

function genId() { return 's' + Math.random().toString(36).slice(2, 9) }

function newSession() {
  const s: Session = {
    id: genId(),
    title: '',
    form: {
      instanceId: '', host: '', port: 22, username: 'root', password: '',
      authType: 'password', privateKey: '', passphrase: '', savedKeyId: '',
    },
    term: null, ws: null, fitAddon: null, resizeObserver: null,
    connected: false, connecting: false, error: '', el: null,
  }
  sessions.value.push(s)
  activeId.value = s.id
}

function closeSession(id: string) {
  const i = sessions.value.findIndex(s => s.id === id)
  if (i < 0) return
  const s = sessions.value[i]
  disposeSession(s)
  sessions.value.splice(i, 1)
  if (activeId.value === id) {
    activeId.value = sessions.value[sessions.value.length - 1]?.id || ''
    if (!activeId.value && sessions.value.length === 0) newSession()
  }
}

function activate(id: string) {
  activeId.value = id
  nextTick(() => { const s = sessions.value.find(x => x.id === id); s?.fitAddon?.fit() })
}

function setTermEl(id: string, el: HTMLElement | null) {
  const s = sessions.value.find(x => x.id === id)
  if (s) s.el = el
}

async function loadInstances() {
  if (loadingInstances.value) return
  loadingInstances.value = true
  try {
    const res = await request.get('/instances/list', { params: { limit: 200, offset: 0 } }) as any
    instances.value = res.items || []
  } catch (e: any) { ElMessage.error('加载实例失败: ' + e.message) }
  finally { loadingInstances.value = false }
}

function onPickInstance(s: Session) {
  const inst = instances.value.find(i => i.instanceId === s.form.instanceId)
  if (!inst) return
  // instance_detail has public_ips (JSON array or csv) + username
  s.form.host = firstIP(inst.publicIps) || firstIP(inst.privateIps) || s.form.host
  if (inst.username) s.form.username = inst.username
  if (inst.port) s.form.port = Number(inst.port) || 22
  s.title = inst.displayName || inst.instanceId
}

function firstIP(raw: any): string {
  if (!raw) return ''
  if (Array.isArray(raw)) return raw[0] || ''
  if (typeof raw === 'string') {
    try { const a = JSON.parse(raw); if (Array.isArray(a)) return a[0] || '' } catch { /* not json */ }
    return raw.split(',')[0].trim()
  }
  return ''
}

async function initTerm(s: Session) {
  if (!s.el || s.term) return
  const { Terminal } = await import('@xterm/xterm')
  const { FitAddon } = await import('@xterm/addon-fit')
  s.term = new Terminal({
    cursorBlink: true, fontSize: 14, convertEol: true, scrollback: 10000, tabStopWidth: 4,
    fontFamily: '"Cascadia Code", Menlo, Monaco, "Courier New", monospace',
    theme: { background: '#1e1e1e', foreground: '#d4d4d4', cursor: '#d4d4d4', selectionBackground: '#264f78' },
  })
  s.fitAddon = new FitAddon()
  s.term.loadAddon(s.fitAddon)
  s.term.open(s.el)
  s.fitAddon.fit()
  s.term.onData((data: string) => {
    if (s.ws && s.ws.readyState === WebSocket.OPEN) s.ws.send(JSON.stringify({ type: 'input', data }))
  })
  s.term.onResize(({ cols, rows }: { cols: number; rows: number }) => {
    if (s.ws && s.ws.readyState === WebSocket.OPEN) s.ws.send(JSON.stringify({ type: 'resize', data: { cols, rows } }))
  })
  s.resizeObserver = new ResizeObserver(() => s.fitAddon?.fit())
  s.resizeObserver.observe(s.el)
}

function connect(s: Session) {
  if (!s.form.host) { ElMessage.warning('请输入主机地址'); return }
  s.connecting = true; s.error = ''; s.connected = false
  initTerm(s).then(() => {
    if (!s.term) { s.connecting = false; return }
    s.term.clear()
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    s.ws = new WebSocket(`${proto}//${location.host}/ws/ssh`)
    s.ws.binaryType = 'arraybuffer'
    s.ws.onopen = () => {
      s.connecting = false; s.connected = true
      s.title = s.title || `${s.form.username}@${s.form.host}`
      pushRecent({ key: `${s.form.username}@${s.form.host}:${s.form.port}`, host: s.form.host, port: s.form.port, username: s.form.username })
      const payload: any = { host: s.form.host, port: s.form.port, username: s.form.username }
      if (s.form.authType === 'key') {
        if (s.form.savedKeyId) {
          payload.keyId = Number(s.form.savedKeyId) // backend resolves + decrypts; content stays server-side
        } else if (s.form.privateKey) {
          payload.privateKey = s.form.privateKey
          if (s.form.passphrase) payload.passphrase = s.form.passphrase
        } else {
          s.error = '请选择已存密钥或粘贴私钥'; s.connected = false; s.ws?.close(); return
        }
      } else {
        payload.password = s.form.password
      }
      s.ws!.send(JSON.stringify({ type: 'connect', data: payload }))
    }
    s.ws.onmessage = (e) => {
      if (!s.term) return
      if (typeof e.data === 'string') s.term.write(e.data)
      else if (e.data instanceof ArrayBuffer) s.term.write(new Uint8Array(e.data))
    }
    s.ws.onerror = () => { s.connecting = false; s.error = 'WebSocket 连接失败' }
    s.ws.onclose = () => {
      s.connected = false
      if (s.term) s.term.write('\r\n\x1b[33m[连接已断开]\x1b[0m\r\n')
    }
  })
}

function disconnect(s: Session) {
  if (s.ws) { try { s.ws.close() } catch { /* closing */ }; s.ws = null }
  s.connected = false
}

function reconnect(s: Session) { disconnect(s); connect(s) }
function clearTerm(s: Session) { s.term?.clear() }
function copySel(s: Session) {
  const sel = s.term?.getSelection?.()
  if (sel) navigator.clipboard.writeText(sel).then(() => ElMessage.success('已复制')).catch(() => ElMessage.error('复制失败'))
  else ElMessage.info('无选中文本')
}
function toggleFullscreen(s: Session) {
  const el = s.el?.parentElement
  if (!el) return
  if (document.fullscreenElement) document.exitFullscreen()
  else el.requestFullscreen()
}

function disposeSession(s: Session) {
  disconnect(s)
  s.resizeObserver?.disconnect(); s.resizeObserver = null
  try { s.term?.dispose() } catch { /* disposed */ }
  s.term = null; s.fitAddon = null; s.el = null
}

// --- recents (localStorage) ---
function loadRecents(): Recent[] {
  try { return JSON.parse(localStorage.getItem('ssh-recents') || '[]') } catch { return [] }
}
function saveRecents() { localStorage.setItem('ssh-recents', JSON.stringify(recents.value.slice(0, 8))) }
function pushRecent(r: Recent) {
  recents.value = [r, ...recents.value.filter(x => x.key !== r.key)].slice(0, 8)
  saveRecents()
}
function applyRecent(s: Session, r: Recent) {
  s.form.host = r.host; s.form.port = r.port; s.form.username = r.username; s.form.instanceId = ''
}

// --- saved SSH private keys (DB-encrypted; content never sent back) ---
async function loadSavedKeys() {
  try {
    const res = await request.get('/ssh-keys') as any
    savedKeys.value = Array.isArray(res) ? res : (Array.isArray(res?.data) ? res.data : [])
  } catch (e: any) { ElMessage.error('加载密钥失败: ' + e.message) }
}
async function addSavedKey() {
  if (!keyForm.value.label || !keyForm.value.content) { ElMessage.warning('请填写标签并粘贴私钥'); return }
  keySaving.value = true
  try {
    await request.post('/ssh-keys', { label: keyForm.value.label, content: keyForm.value.content, passphrase: keyForm.value.passphrase })
    keyForm.value = { label: '', content: '', passphrase: '' }
    await loadSavedKeys()
    ElMessage.success('已保存')
  } catch (e: any) { ElMessage.error('保存失败: ' + e.message) }
  finally { keySaving.value = false }
}
async function removeSavedKey(id: number) {
  try {
    await request.delete(`/ssh-keys/${id}`)
    await loadSavedKeys()
    ElMessage.success('已删除')
  } catch (e: any) { ElMessage.error('删除失败: ' + e.message) }
}
// Selecting a saved key just records its id; the backend resolves + decrypts
// it at connect time. The ad-hoc privateKey/passphrase fields are ignored when
// a saved key is selected.
function onSavedKeyChange(_s: Session) { /* savedKeyId already bound via v-model */ }

onMounted(() => { loadSavedKeys() })
onActivated(() => { nextTick(() => active.value?.fitAddon?.fit()) })
onBeforeUnmount(() => sessions.value.forEach(disposeSession))

// start with one session
if (!sessions.value.length) newSession()
</script>

<style scoped>
.ssh-terminal-page { padding: 20px; }
.tab-bar { display: flex; align-items: center; gap: 4px; margin-bottom: 12px; flex-wrap: wrap; }
.tab-item {
  display: flex; align-items: center; gap: 6px; padding: 6px 12px; border-radius: 6px 6px 0 0;
  background: #f0f0f0; cursor: pointer; font-size: 13px; color: #606266; max-width: 220px;
}
.tab-item.active { background: #1e1e1e; color: #d4d4d4; }
.tab-item.active .tab-close { color: #d4d4d4; }
.tab-title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tab-close { margin-left: 4px; opacity: .6; }
.tab-close:hover { opacity: 1; }
.tab-new { border: 1px dashed #c0c4cc; background: transparent; border-radius: 6px; padding: 4px 10px; cursor: pointer; color: #606266; }
.tab-new:hover { border-color: #409eff; color: #409eff; }
.connect-card { margin-bottom: 12px; }
.recents { margin-top: 8px; display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.recents-label { color: #909399; font-size: 12px; }
.recent-tag { cursor: pointer; }
.terminals { position: relative; }
.terminal-wrap { display: flex; flex-direction: column; }
.term-toolbar { display: flex; align-items: center; gap: 4px; padding: 4px 8px; background: #2a2a2a; border-radius: 6px 6px 0 0; }
.term-status { color: #d4d4d4; font-size: 12px; margin-right: auto; display: flex; align-items: center; gap: 6px; }
.dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: #909399; }
.dot.on { background: #0dbc79; }
.dot.connecting { background: #e5e510; animation: pulse 1s infinite; }
@keyframes pulse { 50% { opacity: .4; } }
.terminal-container { height: calc(100vh - 260px); min-height: 380px; background: #1e1e1e; border-radius: 0 0 6px 6px; overflow: hidden; }
.terminal-container :deep(.xterm) { height: 100%; padding: 8px; }
.terminal-container :deep(.xterm-viewport) { scrollbar-width: thin; background: #1e1e1e !important; }
</style>
