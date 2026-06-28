<template>
  <div>
    <div class="toolbar">
      <h2>实例救援</h2>
      <el-button @click="loadInstances" :loading="loadingInstances">刷新实例列表</el-button>
    </div>

    <el-card class="connect-card" shadow="hover">
      <el-form :model="form" label-width="100px" @submit.prevent>
        <el-form-item label="选择实例">
          <el-select
            v-model="form.instanceId"
            placeholder="请选择要救援的实例"
            style="width: 420px"
            filterable
            clearable
            :loading="loadingInstances"
            :filter-method="filterInstances"
            @focus="loadInstances"
            @change="onInstanceChange"
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

        <el-form-item label="救援类型">
          <el-radio-group v-model="form.rescueType">
            <el-radio :value="0">DD 重建</el-radio>
            <el-radio :value="1">NetBoot</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item v-if="form.rescueType === 1" label="救援卷 ID">
          <el-input v-model="form.rescueImageId" placeholder="预创建的急救引导卷 OCID" style="width:360px" clearable />
          <div style="font-size:12px;color:var(--text-muted);margin-top:2px">
            请输入已通过 OCI 控制台或 API 预创建的救援引导卷 OCID（非镜像 OCID）
          </div>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="starting" :disabled="!form.instanceId" @click="startRescue">
            {{ starting ? '启动中...' : '开始救援' }}
          </el-button>
          <el-button :disabled="!active" @click="cancelRescue">取消</el-button>
          <el-button :disabled="!active" @click="completeRescue" type="success">完成救援</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Selected instance info -->
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

    <el-card v-if="error" class="error-card" shadow="hover">
      <el-alert type="error" :title="error" show-icon :closable="true" @close="error = ''" />
    </el-card>

    <el-card v-if="active" class="progress-card" shadow="hover">
      <template #header>
        <span>救援进度 — {{ status.step }}</span>
        <el-tag v-if="active" type="warning" style="margin-left:8px">进行中</el-tag>
      </template>
      <div style="max-width:600px">
        <el-progress :percentage="status.progress" :color="status.progress === 100 ? 'var(--status-up)' : 'var(--accent)'" />
        <p style="margin-top:12px; font-size:14px">{{ status.message }}</p>
        <el-descriptions v-if="status.instanceId" :column="1" border style="margin-top:12px">
          <el-descriptions-item label="实例">{{ status.instanceId }}</el-descriptions-item>
          <el-descriptions-item label="当前步骤">{{ status.step }}</el-descriptions-item>
        </el-descriptions>
      </div>
    </el-card>

    <el-card v-if="history.length > 0" class="history-card" shadow="hover">
      <template #header>操作历史</template>
      <el-timeline>
        <el-timeline-item
          v-for="(item, idx) in history"
          :key="idx"
          :timestamp="item.time"
          :color="item.ok ? 'var(--status-up)' : 'var(--status-down)'"
        >
          {{ item.msg }}
        </el-timeline-item>
      </el-timeline>
    </el-card>

    <el-card v-if="!active && history.length === 0" shadow="hover">
      <template #header>救援流程说明</template>
      <el-steps direction="vertical" :active="1" process-status="finish" finish-status="success">
        <el-step title="选择实例" description="从下拉列表中选择要救援的目标实例" />
        <el-step title="停止实例" description="安全停止目标实例" />
        <el-step title="卸载引导卷" description="分离原始引导卷" />
        <el-step title="挂载急救卷" description="挂载急救/恢复引导卷" />
        <el-step title="启动急救系统" description="启动实例进入急救模式" />
        <el-step title="SSH 救援操作" description="通过 SSH 连接实例执行 DD 重建或其他修复操作" />
        <el-step title="完成救援" description="点击「完成救援」后自动还原引导卷并重启实例" />
      </el-steps>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
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

const form = ref({ instanceId: '', rescueType: 0, rescueImageId: '' })
const starting = ref(false)
const active = ref(false)
const error = ref('')
const loadingInstances = ref(false)
const instances = ref<InstanceItem[]>([])
const searchText = ref('')
const status = reactive({ step: '', message: '', progress: 0, instanceId: '' })
const history = reactive<Array<{ time: string; msg: string; ok: boolean }>>([])

let ws: WebSocket | null = null

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

function onInstanceChange() {
  // Reset state when switching instances
  error.value = ''
  history.length = 0
}

const instDetail = computed(() => {
  return instances.value.find(i => i.instanceId === form.value.instanceId) || null
})

function getWsUrl(): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${location.host}/ws/rescue`
}

function addHistory(msg: string, ok: boolean) {
  history.unshift({
    time: new Date().toLocaleTimeString(),
    msg,
    ok,
  })
}

function startRescue() {
  if (!form.value.instanceId.trim()) {
    ElMessage.warning('请先选择一个实例')
    return
  }
  if (form.value.rescueType === 1 && !form.value.rescueImageId.trim()) {
    ElMessage.warning('NetBoot 模式需要填写救援卷 ID（预创建的引导卷 OCID）')
    return
  }

  starting.value = true
  error.value = ''
  history.length = 0

  try {
    ws = new WebSocket(getWsUrl())

    ws.onopen = () => {
      ws!.send(JSON.stringify({
        type: 'init',
        data: {
          instanceId: form.value.instanceId,
          rescueType: form.value.rescueType,
          rescueImageId: form.value.rescueImageId,
          tenantId: 0,
        },
      }))
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        switch (msg.type) {
          case 'info':
            starting.value = false
            ElMessage.info(msg.message)
            break
          case 'error':
            starting.value = false
            active.value = false
            error.value = msg.message
            addHistory(`错误: ${msg.message}`, false)
            ElMessage.error(msg.message)
            break
          case 'cancelled':
            active.value = false
            addHistory('操作已取消', false)
            ElMessage.warning('救援已取消')
            break
          default:
            // Progress update
            starting.value = false
            active.value = true
            Object.assign(status, msg)
            if (msg.progress > 0) {
              addHistory(`[${msg.step}] ${msg.message}`, true)
            }
            if (msg.step === 'complete') {
              active.value = false
              ElMessage.success('救援流程完成！')
            }
            break
        }
      } catch { /* ignore */ }
    }

    ws.onclose = () => {
      starting.value = false
      if (active.value) {
        active.value = false
        ElMessage.warning('WebSocket 连接已断开')
      }
    }

    ws.onerror = () => {
      starting.value = false
      error.value = 'WebSocket 连接失败'
      ElMessage.error('WebSocket 连接失败')
    }
  } catch (err: any) {
    starting.value = false
    error.value = err?.message || '连接失败'
  }
}

function cancelRescue() {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({
      type: 'cancel',
      data: { instanceId: form.value.instanceId },
    }))
    addHistory('已发送取消请求', false)
  }
}

function completeRescue() {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({
      type: 'complete',
      data: { instanceId: form.value.instanceId },
    }))
    ElMessage.success('已发送完成指令，开始还原引导卷')
    addHistory('用户确认救援完成，开始还原', true)
  }
}

onBeforeUnmount(() => {
  if (ws) {
    cancelRescue()
    ws.close()
  }
})
</script>

<style scoped>
.toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
.toolbar h2 { margin: 0; margin-right: auto; }
.connect-card { margin-bottom: 16px; }
.info-card { margin-bottom: 16px; }
.progress-card { margin-bottom: 16px; }
.error-card { margin-bottom: 16px; }
.history-card { margin-bottom: 16px; }
</style>
