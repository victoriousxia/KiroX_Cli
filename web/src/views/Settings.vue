<template>
  <div class="settings-page">
    <h2 class="page-title">系统设置</h2>

    <el-card shadow="never" class="settings-card">
      <template #header>
        <span>基本配置</span>
      </template>
      <el-form :model="config" label-width="140px" label-position="left">
        <el-form-item label="代理地址">
          <div style="display: flex; gap: 8px; width: 100%">
            <el-input v-model="config.proxy" placeholder="例如: socks5://127.0.0.1:20170" />
            <el-button :loading="testing" @click="handleTestProxy">检测代理</el-button>
          </div>
        </el-form-item>
        <el-form-item label="MoeMail API URL">
          <el-input v-model="config.moEmailUrl" placeholder="MoeMail 服务地址" />
        </el-form-item>
        <el-form-item label="MoeMail API Key">
          <el-input v-model="config.moEmailKey" placeholder="MoeMail API 密钥" show-password />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="handleSave">
            保存配置
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never" class="settings-card" style="margin-top: 20px">
      <template #header>
        <div class="card-header-row">
          <span>Outlook 邮箱池</span>
          <el-tag v-if="outlookCount > 0" type="success" size="small">{{ outlookCount }} 个账号</el-tag>
        </div>
      </template>
      <el-upload
        :auto-upload="false"
        :on-change="handleFileChange"
        accept=".csv,.txt"
        :limit="1"
        :file-list="fileList"
      >
        <template #trigger>
          <el-button>选择文件</el-button>
        </template>
        <el-button
          type="primary"
          style="margin-left: 12px"
          :loading="uploading"
          @click="handleUpload"
        >
          上传
        </el-button>
        <template #tip>
          <div class="upload-tip">支持 .csv 或 .txt 文件，格式：邮箱----密码----客户端ID----RefreshToken</div>
        </template>
      </el-upload>

      <!-- 已上传的邮箱列表 -->
      <el-table
        v-if="outlookAccounts.length > 0"
        :data="outlookAccounts"
        style="width: 100%; margin-top: 16px"
        max-height="300"
        size="small"
      >
        <el-table-column type="index" label="#" width="50" />
        <el-table-column prop="email" label="邮箱" min-width="240" show-overflow-tooltip />
        <el-table-column prop="clientId" label="客户端ID" width="300" show-overflow-tooltip />
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getConfig, updateConfig, uploadOutlookCsv, type AppConfig, api } from '../api'
import { ElMessage } from 'element-plus'
import type { UploadFile } from 'element-plus'

const config = ref<AppConfig>({
  proxy: '',
  moEmailUrl: '',
  moEmailKey: '',
})
const saving = ref(false)
const uploading = ref(false)
const testing = ref(false)
const fileList = ref<UploadFile[]>([])
const selectedFile = ref<File | null>(null)
const outlookAccounts = ref<Array<{ email: string; clientId: string }>>([])
const outlookCount = ref(0)

async function loadConfig() {
  try {
    config.value = await getConfig()
  } catch {
    // use defaults
  }
}

async function loadOutlookAccounts() {
  try {
    const res = await api.get('/api/config/outlook')
    outlookAccounts.value = res.data.accounts || []
    outlookCount.value = res.data.count || 0
  } catch {
    outlookAccounts.value = []
    outlookCount.value = 0
  }
}

async function handleSave() {
  saving.value = true
  try {
    await updateConfig(config.value)
    ElMessage.success('配置已保存')
  } catch (e: any) {
    ElMessage.error(e.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

function handleFileChange(file: UploadFile) {
  selectedFile.value = file.raw || null
}

async function handleTestProxy() {
  testing.value = true
  try {
    const res = await api.post('/api/config/test-proxy', { proxy: config.value.proxy })
    const data = res.data
    if (data.success) {
      ElMessage.success(`代理可用! IP: ${data.ip} [${data.country} ${data.region} ${data.city}] ISP: ${data.isp} 延迟: ${data.latency}ms`)
    } else {
      ElMessage.error(data.error || '代理连接失败')
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.error || '检测失败')
  } finally {
    testing.value = false
  }
}

async function handleUpload() {
  if (!selectedFile.value) {
    ElMessage.warning('请先选择文件')
    return
  }
  uploading.value = true
  try {
    const res = await uploadOutlookCsv(selectedFile.value)
    ElMessage.success(`上传成功，导入 ${res.data.count} 个账号`)
    fileList.value = []
    selectedFile.value = null
    await loadOutlookAccounts()
  } catch (e: any) {
    ElMessage.error(e.response?.data?.error || '上传失败')
  } finally {
    uploading.value = false
  }
}

onMounted(() => {
  loadConfig()
  loadOutlookAccounts()
})
</script>

<style scoped>
.page-title {
  color: #e0e0e0;
  font-size: 20px;
  margin-bottom: 24px;
}

.settings-card {
  background-color: #1a1a2e;
  border: 1px solid #2a2a3e;
  border-radius: 8px;
}

.settings-card :deep(.el-card__header) {
  color: #e0e0e0;
  border-bottom: 1px solid #2a2a3e;
}

.card-header-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.upload-tip {
  color: #a0a0b0;
  font-size: 12px;
  margin-top: 8px;
}
</style>
