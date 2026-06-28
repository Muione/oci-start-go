<template>
  <div class="migration-page">
    <div class="page-header">
      <h2>数据迁移</h2>
      <p class="subtitle">从 Java 版 oci-start (H2) 迁移数据到 Go 版 (SQLite)</p>
    </div>

    <!-- Status Cards -->
    <el-row :gutter="20" class="stats-row">
      <el-col :span="6">
        <el-card shadow="hover">
          <el-statistic title="总行数" :value="stats.totalLines || 0" />
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <el-statistic title="插入成功" :value="stats.inserted || 0">
            <template #suffix>
              <el-icon style="color: var(--status-up)"><SuccessFilled /></el-icon>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <el-statistic title="跳过" :value="(stats.skipped || 0) + (stats.skippedDups || 0) + (stats.skippedUser || 0)">
            <template #suffix>
              <el-icon style="color: var(--status-warn)"><WarningFilled /></el-icon>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <el-statistic title="错误" :value="stats.errors || 0">
            <template #suffix>
              <el-icon v-if="stats.errors > 0" style="color: var(--status-down)"><CircleCloseFilled /></el-icon>
              <el-icon v-else style="color: var(--status-up)"><SuccessFilled /></el-icon>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
    </el-row>

    <!-- Import Section -->
    <el-card class="import-card">
      <template #header>
        <span class="card-title">
          <el-icon><Upload /></el-icon> 导入数据库
        </span>
      </template>

      <el-tabs v-model="importMode" type="border-card">
        <!-- Plain SQL Import -->
        <el-tab-pane label="明文 SQL" name="plain">
          <div class="import-form">
            <p class="help-text">
              上传 Java 版导出的 <code>.sql</code> 文件。可通过 Migration 页面
              <code>/migration/export</code> 接口导出，或使用 CLI 工具。
            </p>

            <el-alert
              title="注意"
              type="warning"
              description="导入前请确认目标数据库中不存在重复的 Tenant 数据，否则导入会失败。"
              show-icon
              :closable="false"
              style="margin-bottom: 16px"
            />

            <el-upload
              ref="plainUpload"
              :auto-upload="false"
              :on-change="handlePlainFileChange"
              :limit="1"
              accept=".sql"
              drag
            >
              <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
              <div class="el-upload__text">拖拽 SQL 文件到此处，或点击上传</div>
              <template #tip>
                <div class="el-upload__tip">仅支持 .sql 格式的数据库备份文件</div>
              </template>
            </el-upload>

            <el-button
              type="primary"
              :loading="importing"
              :disabled="!plainFile"
              @click="importPlain"
              style="margin-top: 16px"
            >
              <el-icon><Upload /></el-icon> 开始导入
            </el-button>
          </div>
        </el-tab-pane>

        <!-- Encrypted .enc Import -->
        <el-tab-pane label="加密备份 (.enc)" name="encrypted">
          <div class="import-form">
            <p class="help-text">
              上传 Java 版导出的加密 <code>.enc</code> 文件。需要提供导出时显示的 <strong>Master Key</strong>。
            </p>

            <el-form label-width="100px">
              <el-form-item label="Master Key">
                <el-input
                  v-model="masterKey"
                  type="textarea"
                  :rows="2"
                  placeholder="请输入导出时显示的 X-MASTER-KEY"
                />
              </el-form-item>

              <el-form-item label="加密文件">
                <el-upload
                  ref="encUpload"
                  :auto-upload="false"
                  :on-change="handleEncFileChange"
                  :limit="1"
                  accept=".enc"
                  drag
                >
                  <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
                  <div class="el-upload__text">拖拽 .enc 文件到此处，或点击上传</div>
                  <template #tip>
                    <div class="el-upload__tip">仅支持 .enc 格式的加密备份文件</div>
                  </template>
                </el-upload>
              </el-form-item>

              <el-form-item>
                <el-button
                  type="primary"
                  :loading="importing"
                  :disabled="!encFile || !masterKey"
                  @click="importEncrypted"
                >
                  <el-icon><Upload /></el-icon> 解密并导入
                </el-button>
              </el-form-item>
            </el-form>
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <!-- CLI Migration Guide -->
    <el-card class="guide-card">
      <template #header>
        <span class="card-title">
          <el-icon><Terminal /></el-icon> CLI 迁移工具
        </span>
      </template>

      <div class="guide-content">
        <p>也可以使用命令行工具进行离线迁移：</p>

        <el-divider content-position="left">明文 SQL 导入</el-divider>
        <pre class="code-block"><code># 从 Java 版导出明文 SQL
curl -o backup.sql http://old-server:9856/migration/export

# 使用 CLI 导入到 Go 版数据库
./migrate -db /path/to/oci-start.db -file backup.sql</code></pre>

        <el-divider content-position="left">加密备份导入</el-divider>
        <pre class="code-block"><code># 从 Java 版导出加密备份
curl -o backup.enc http://old-server:9856/migration/exportEncrypted
# 响应头 X-MASTER-KEY 包含解密密钥（仅显示一次！）

# 使用 CLI 导入
./migrate -db /path/to/oci-start.db -file backup.enc -key &lt;master-key&gt;</code></pre>

        <el-divider content-position="left">迁移步骤</el-divider>
        <el-steps :active="0" direction="vertical">
          <el-step title="导出" description="从旧版 Java 服务器导出数据（SQL 或加密文件）" />
          <el-step title="备份" description="备份 Go 版 SQLite 数据库文件" />
          <el-step title="导入" description="使用 CLI 工具或 Web 页面上传导入" />
          <el-step title="验证" description="检查 Tenant 列表、实例数据、代理配置等" />
          <el-step title="切换" description="确认数据完整后，切换到新版服务" />
        </el-steps>
      </div>
    </el-card>

    <!-- Tables Found -->
    <el-card v-if="stats.tablesFound && Object.keys(stats.tablesFound).length > 0" class="tables-card">
      <template #header>
        <span class="card-title">
          <el-icon><Grid /></el-icon> 导入表统计
        </span>
      </template>
      <el-table :data="tableStats" stripe size="small">
        <el-table-column prop="table" label="表名" />
        <el-table-column prop="count" label="导入行数" width="120" align="right" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, type Ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

interface ImportStats {
  totalLines?: number
  insertLines?: number
  inserted?: number
  skipped?: number
  skippedDups?: number
  skippedUser?: number
  errors?: number
  tablesFound?: Record<string, number>
  message?: string
}

const importMode = ref('plain')
const importing = ref(false)
const masterKey = ref('')
const plainFile: Ref<File | null> = ref(null)
const encFile: Ref<File | null> = ref(null)
const stats = ref<ImportStats>({})

const tableStats = computed(() => {
  if (!stats.value.tablesFound) return []
  return Object.entries(stats.value.tablesFound).map(([table, count]) => ({
    table,
    count,
  }))
})

function handlePlainFileChange(file: any) {
  plainFile.value = file.raw
}

function handleEncFileChange(file: any) {
  encFile.value = file.raw
}

async function importPlain() {
  if (!plainFile.value) return

  try {
    await ElMessageBox.confirm(
      '导入前请确认数据库中没有重复的 Tenant 数据。是否继续？',
      '确认导入',
      { confirmButtonText: '确认导入', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }

  importing.value = true
  try {
    const formData = new FormData()
    formData.append('file', plainFile.value)

    const res = await fetch('/migration/import', {
      method: 'POST',
      body: formData,
      credentials: 'include',
    })
    const data = await res.json()

    if (data.code === 200) {
      stats.value = data.data
      ElMessage.success(data.data?.message || '导入成功')
    } else {
      ElMessage.error(data.message || '导入失败')
    }
  } catch (e: any) {
    ElMessage.error('导入失败: ' + e.message)
  } finally {
    importing.value = false
  }
}

async function importEncrypted() {
  if (!encFile.value || !masterKey.value) return

  try {
    await ElMessageBox.confirm(
      '确认使用提供的 Master Key 解密并导入数据？',
      '确认导入',
      { confirmButtonText: '确认导入', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }

  importing.value = true
  try {
    const formData = new FormData()
    formData.append('file', encFile.value)
    formData.append('masterKey', masterKey.value)

    const res = await fetch('/migration/import-encrypted', {
      method: 'POST',
      body: formData,
      credentials: 'include',
    })
    const data = await res.json()

    if (data.code === 200) {
      stats.value = data.data
      ElMessage.success(data.data?.message || '导入成功')
    } else {
      ElMessage.error(data.message || '导入失败')
    }
  } catch (e: any) {
    ElMessage.error('导入失败: ' + e.message)
  } finally {
    importing.value = false
  }
}
</script>

<style scoped>
.migration-page {
  padding: 20px;
}

.page-header {
  margin-bottom: 24px;
}

.page-header h2 {
  margin: 0 0 8px 0;
  font-size: 24px;
  color: var(--el-text-color-primary);
}

.subtitle {
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 14px;
}

.stats-row {
  margin-bottom: 20px;
}

.import-card,
.guide-card,
.tables-card {
  margin-bottom: 20px;
}

.card-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 16px;
  font-weight: 600;
}

.help-text {
  color: var(--el-text-color-secondary);
  margin-bottom: 16px;
  line-height: 1.6;
}

.help-text code {
  background: var(--el-fill-color-light);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
}

.code-block {
  background: var(--bg-root);
  color: var(--text-primary);
  padding: 16px;
  border-radius: 8px;
  overflow-x: auto;
  font-size: 13px;
  line-height: 1.5;
}

.code-block code {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
}

.guide-content p {
  color: var(--el-text-color-secondary);
  margin-bottom: 12px;
}

.import-form {
  padding: 8px 0;
}
</style>
