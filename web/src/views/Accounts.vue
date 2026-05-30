<template>
  <div class="accounts-page">
    <div class="page-header">
      <h2 class="page-title">账户管理</h2>
      <div class="header-actions">
        <el-input
          v-model="searchText"
          placeholder="搜索邮箱"
          clearable
          style="width: 240px; margin-right: 12px"
        />
        <el-button type="primary" @click="handleVerify" :loading="verifying">
          批量验证
        </el-button>
        <el-button @click="handleExport">导出 JSON</el-button>
      </div>
    </div>

    <el-card shadow="never" class="accounts-card">
      <el-table
        :data="filteredAccounts"
        style="width: 100%"
        @row-click="handleRowClick"
        row-class-name="clickable-row"
      >
        <el-table-column prop="email" label="邮箱" min-width="220" show-overflow-tooltip />
        <el-table-column prop="subscription" label="订阅类型" width="140" />
        <el-table-column label="额度使用" width="160">
          <template #default="{ row }">
            {{ row.creditUsed }} / {{ row.creditLimit }}
          </template>
        </el-table-column>
        <el-table-column prop="provider" label="提供商" width="120" />
        <el-table-column prop="region" label="区域" width="100" />
      </el-table>
    </el-card>

    <el-dialog
      v-model="detailVisible"
      title="账户详情"
      width="520px"
      :close-on-click-modal="true"
    >
      <div v-if="selectedAccount" class="account-detail">
        <div class="detail-item">
          <span class="detail-label">邮箱</span>
          <div class="detail-value-row">
            <span class="detail-value">{{ selectedAccount.email }}</span>
            <el-button size="small" text @click="copyText(selectedAccount.email)">复制</el-button>
          </div>
        </div>
        <div class="detail-item">
          <span class="detail-label">Kiro 密码</span>
          <div class="detail-value-row">
            <span class="detail-value mono">{{ selectedAccount.password || '-' }}</span>
            <el-button v-if="selectedAccount.password" size="small" text @click="copyText(selectedAccount.password)">复制</el-button>
          </div>
        </div>
        <div class="detail-item" v-if="selectedAccount.emailPassword">
          <span class="detail-label">邮箱密码</span>
          <div class="detail-value-row">
            <span class="detail-value mono">{{ selectedAccount.emailPassword }}</span>
            <el-button size="small" text @click="copyText(selectedAccount.emailPassword!)">复制</el-button>
          </div>
        </div>
        <el-divider />
        <div class="detail-item">
          <span class="detail-label">订阅类型</span>
          <span class="detail-value">{{ selectedAccount.subscription || '-' }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">额度</span>
          <span class="detail-value">{{ selectedAccount.creditUsed }} / {{ selectedAccount.creditLimit }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">提供商</span>
          <span class="detail-value">{{ selectedAccount.provider }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">区域</span>
          <span class="detail-value">{{ selectedAccount.region }}</span>
        </div>
        <el-divider />
        <div class="copy-all-section">
          <el-button type="primary" @click="copyAll">一键复制全部信息</el-button>
          <el-button
            type="warning"
            @click="handleSubscribe"
            :loading="subscribing"
            :disabled="!selectedAccount?.clientId || !selectedAccount?.clientSecret || !selectedAccount?.refreshToken"
          >
            订阅 Pro+
          </el-button>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { getAccounts, subscribeAccount, type Account } from '../api'
import { ElMessage } from 'element-plus'

const accounts = ref<Account[]>([])
const searchText = ref('')
const verifying = ref(false)
const subscribing = ref(false)
const detailVisible = ref(false)
const selectedAccount = ref<Account | null>(null)

const filteredAccounts = computed(() => {
  if (!searchText.value) return accounts.value
  const keyword = searchText.value.toLowerCase()
  return accounts.value.filter((a) => a.email.toLowerCase().includes(keyword))
})

function handleRowClick(row: Account) {
  selectedAccount.value = row
  detailVisible.value = true
}

function copyText(text: string) {
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(text).then(() => {
      ElMessage.success('已复制')
    }).catch(() => {
      fallbackCopy(text)
    })
  } else {
    fallbackCopy(text)
  }
}

function fallbackCopy(text: string) {
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  const success = document.execCommand('copy')
  document.body.removeChild(textarea)
  if (success) {
    ElMessage.success('已复制')
  } else {
    ElMessage.error('复制失败')
  }
}

function copyAll() {
  if (!selectedAccount.value) return
  const acc = selectedAccount.value
  const lines = [
    `邮箱: ${acc.email}`,
    `Kiro密码: ${acc.password || '-'}`,
  ]
  if (acc.emailPassword) {
    lines.push(`邮箱密码: ${acc.emailPassword}`)
  }
  lines.push(`订阅: ${acc.subscription || '-'}`)
  lines.push(`额度: ${acc.creditUsed} / ${acc.creditLimit}`)
  copyText(lines.join('\n'))
}

async function loadAccounts() {
  try {
    accounts.value = await getAccounts()
  } catch {
    accounts.value = []
  }
}

async function handleVerify() {
  verifying.value = true
  try {
    await loadAccounts()
    ElMessage.success('验证完成')
  } catch {
    ElMessage.error('验证失败')
  } finally {
    verifying.value = false
  }
}

function handleExport() {
  const exportData = accounts.value.map(({ password, emailPassword, clientId, clientSecret, refreshToken, ...rest }) => rest)
  const data = JSON.stringify(exportData, null, 2)
  const blob = new Blob([data], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'accounts.json'
  a.click()
  URL.revokeObjectURL(url)
}

async function handleSubscribe() {
  if (!selectedAccount.value?.clientId || !selectedAccount.value?.clientSecret || !selectedAccount.value?.refreshToken) {
    ElMessage.warning('该账号缺少凭证信息，无法订阅')
    return
  }
  subscribing.value = true
  try {
    const checkoutUrl = await subscribeAccount({
      clientId: selectedAccount.value.clientId,
      clientSecret: selectedAccount.value.clientSecret,
      refreshToken: selectedAccount.value.refreshToken,
      email: selectedAccount.value.email,
    })
    window.open(checkoutUrl, '_blank')
    ElMessage.success('已打开支付页面')
  } catch (e: any) {
    const resp = e?.response?.data
    if (resp?.removed) {
      ElMessage.warning(`账号 ${resp.email} 无订阅权限，已从列表移除`)
      detailVisible.value = false
      await loadAccounts()
    } else {
      const msg = resp?.error || e?.message || '获取支付链接失败'
      ElMessage.error(msg)
    }
  } finally {
    subscribing.value = false
  }
}

onMounted(loadAccounts)
</script>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  flex-wrap: wrap;
  gap: 12px;
}

.page-title {
  color: #e0e0e0;
  font-size: 20px;
}

.header-actions {
  display: flex;
  align-items: center;
}

.accounts-card {
  background-color: #1a1a2e;
  border: 1px solid #2a2a3e;
  border-radius: 8px;
}

:deep(.clickable-row) {
  cursor: pointer;
}

:deep(.clickable-row:hover > td) {
  background-color: #252540 !important;
}

.account-detail {
  padding: 4px 0;
}

.detail-item {
  display: flex;
  align-items: center;
  padding: 10px 0;
}

.detail-label {
  width: 90px;
  flex-shrink: 0;
  color: #909399;
  font-size: 14px;
}

.detail-value-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.detail-value {
  color: #e0e0e0;
  font-size: 14px;
  word-break: break-all;
}

.detail-value.mono {
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  background: #1a1a2e;
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid #2a2a3e;
}

.copy-all-section {
  display: flex;
  justify-content: center;
  padding-top: 8px;
}
</style>
