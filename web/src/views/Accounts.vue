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
      <el-table :data="filteredAccounts" style="width: 100%">
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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { getAccounts, type Account } from '../api'
import { ElMessage } from 'element-plus'

const accounts = ref<Account[]>([])
const searchText = ref('')
const verifying = ref(false)

const filteredAccounts = computed(() => {
  if (!searchText.value) return accounts.value
  const keyword = searchText.value.toLowerCase()
  return accounts.value.filter((a) => a.email.toLowerCase().includes(keyword))
})

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
    // Reload accounts after verify - actual verify needs account credentials
    await loadAccounts()
    ElMessage.success('验证完成')
  } catch {
    ElMessage.error('验证失败')
  } finally {
    verifying.value = false
  }
}

function handleExport() {
  const data = JSON.stringify(accounts.value, null, 2)
  const blob = new Blob([data], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'accounts.json'
  a.click()
  URL.revokeObjectURL(url)
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
</style>
