<template>
  <div class="settings-page">
    <h2 class="page-title">系统设置</h2>

    <el-card shadow="never" class="settings-card">
      <template #header>
        <span>基本配置</span>
      </template>
      <el-form :model="config" label-width="140px" label-position="left">
        <el-form-item label="代理地址">
          <el-input v-model="config.proxy" placeholder="例如: socks5://127.0.0.1:1080" />
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
        <span>Outlook CSV 上传</span>
      </template>
      <el-upload
        :auto-upload="false"
        :on-change="handleFileChange"
        accept=".csv"
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
          <div class="upload-tip">请上传 Outlook 账户 CSV 文件</div>
        </template>
      </el-upload>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getConfig, updateConfig, uploadOutlookCsv, type AppConfig } from '../api'
import { ElMessage } from 'element-plus'
import type { UploadFile } from 'element-plus'

const config = ref<AppConfig>({
  proxy: '',
  moEmailUrl: '',
  moEmailKey: '',
})
const saving = ref(false)
const uploading = ref(false)
const fileList = ref<UploadFile[]>([])
const selectedFile = ref<File | null>(null)

async function loadConfig() {
  try {
    config.value = await getConfig()
  } catch {
    // use defaults
  }
}

async function handleSave() {
  saving.value = true
  try {
    await updateConfig(config.value)
    ElMessage.success('配置已保存')
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

function handleFileChange(file: UploadFile) {
  selectedFile.value = file.raw || null
}

async function handleUpload() {
  if (!selectedFile.value) {
    ElMessage.warning('请先选择文件')
    return
  }
  uploading.value = true
  try {
    const res = await uploadOutlookCsv(selectedFile.value)
    ElMessage.success(`上传成功，导入 ${res.data.count} 个账户`)
    fileList.value = []
    selectedFile.value = null
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '上传失败')
  } finally {
    uploading.value = false
  }
}

onMounted(loadConfig)
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

.upload-tip {
  color: #a0a0b0;
  font-size: 12px;
  margin-top: 8px;
}
</style>
