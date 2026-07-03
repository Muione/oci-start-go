<script setup lang="ts">
defineOptions({ name: 'NetworkConfigPanel' })

import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../../utils/request'
import type { NatGatewayInfo, RouteTableInfo } from '../../types/api'
import CreateNatModal from './CreateNatModal.vue'
import CreateRouteTableModal from './CreateRouteTableModal.vue'

const props = defineProps<{ compartmentId: string; vcnId: string; instanceId: string; tenantId: number }>()

const loading = ref(false)
const natGateways = ref<NatGatewayInfo[]>([])
const routeTables = ref<RouteTableInfo[]>([])

// Modals
const createNatVisible = ref(false)
const createRouteTableVisible = ref(false)

async function loadNatGateways() {
  try {
    const res = await request.get('/oci/nat/list', { params: { tenantId: props.tenantId, compartmentId: props.compartmentId, vcnId: props.vcnId } }) as NatGatewayInfo[]
    natGateways.value = res ?? []
  } catch (e: any) {
    ElMessage.error(e.message || '加载 NAT 网关失败')
  }
}

async function loadRouteTables() {
  try {
    const res = await request.get('/oci/route-table/list', { params: { tenantId: props.tenantId, compartmentId: props.compartmentId, vcnId: props.vcnId } }) as RouteTableInfo[]
    routeTables.value = res ?? []
  } catch (e: any) {
    ElMessage.error(e.message || '加载路由表失败')
  }
}

async function deleteNat(nat: NatGatewayInfo) {
  try {
    await ElMessageBox.confirm(`确定删除 NAT 网关 "${nat.displayName}"？`, '删除 NAT 网关', { type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger' })
    await request.get('/oci/nat/delete', { params: { tenantId: props.tenantId, natGatewayId: nat.id } })
    ElMessage.success('NAT 网关已删除')
    loadNatGateways()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '删除失败')
  }
}

async function deleteRouteTable(rt: RouteTableInfo) {
  try {
    await ElMessageBox.confirm(`确定删除路由表 "${rt.displayName}"？`, '删除路由表', { type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger' })
    await request.get('/oci/route-table/delete', { params: { tenantId: props.tenantId, routeTableId: rt.id } })
    ElMessage.success('路由表已删除')
    loadRouteTables()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '删除失败')
  }
}

async function resetToDefault() {
  try {
    await ElMessageBox.confirm('将重置实例主 VNIC 的路由表为 VCN 默认路由表，是否继续？', '重置路由表', { type: 'info', confirmButtonText: '确认重置' })
    await request.post('/oci/route-table/reset', { tenantId: props.tenantId, instanceId: props.instanceId, compartmentId: props.compartmentId })
    ElMessage.success('路由表已重置为默认')
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '重置失败')
  }
}

function loadData() {
  loadNatGateways()
  loadRouteTables()
}

onMounted(loadData)
</script>

<template>
  <div class="network-config-panel" v-loading="loading">
    <!-- NAT Gateways -->
    <el-card shadow="hover" style="margin-bottom: 16px;">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <span>NAT 网关</span>
          <el-button size="small" type="primary" @click="createNatVisible = true">创建 NAT 网关</el-button>
        </div>
      </template>
      <el-table :data="natGateways" stripe>
        <el-table-column prop="displayName" label="名称" />
        <el-table-column prop="id" label="OCID">
          <template #default="{ row }"><el-text truncated>{{ row.id }}</el-text></template>
        </el-table-column>
        <el-table-column prop="lifecycleState" label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.lifecycleState === 'AVAILABLE' ? 'success' : 'info'" size="small">{{ row.lifecycleState }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="danger" @click="deleteNat(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="natGateways.length === 0" description="暂无 NAT 网关" />
    </el-card>

    <!-- Route Tables -->
    <el-card shadow="hover">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <span>路由表</span>
          <el-space>
            <el-button size="small" type="primary" @click="createRouteTableVisible = true">创建路由表</el-button>
            <el-button size="small" @click="resetToDefault">重置为默认路由表</el-button>
          </el-space>
        </div>
      </template>
      <el-table :data="routeTables" stripe>
        <el-table-column prop="displayName" label="名称" />
        <el-table-column prop="id" label="OCID">
          <template #default="{ row }"><el-text truncated>{{ row.id }}</el-text></template>
        </el-table-column>
        <el-table-column label="规则数" width="100">
          <template #default="{ row }">{{ row.routeRules?.length ?? 0 }}</template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="danger" @click="deleteRouteTable(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="routeTables.length === 0" description="暂无路由表" />
    </el-card>

    <!-- Modals -->
    <CreateNatModal v-model="createNatVisible" :compartment-id="compartmentId" :vcn-id="vcnId" :tenant-id="tenantId" @saved="loadNatGateways" />
    <CreateRouteTableModal v-model="createRouteTableVisible" :compartment-id="compartmentId" :vcn-id="vcnId" :tenant-id="tenantId" @saved="loadRouteTables" />
  </div>
</template>

<style scoped>
.network-config-panel {
  padding: var(--space-4);
}
</style>
