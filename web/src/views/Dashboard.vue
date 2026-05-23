<template>
  <div class="dashboard">
    <h2 class="page-title">仪表盘</h2>
    <el-row :gutter="20" class="stats-row">
      <el-col :span="6">
        <el-card class="stat-card" shadow="never">
          <div class="stat-value">{{ stats.total }}</div>
          <div class="stat-label">总任务数</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card running" shadow="never">
          <div class="stat-value">{{ stats.running }}</div>
          <div class="stat-label">运行中</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card success" shadow="never">
          <div class="stat-value">{{ stats.success }}</div>
          <div class="stat-label">成功数</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card fail" shadow="never">
          <div class="stat-value">{{ stats.failed }}</div>
          <div class="stat-label">失败数</div>
        </el-card>
      </el-col>
    </el-row>

    <el-card class="recent-tasks" shadow="never">
      <template #header>
        <span>最近任务</span>
      </template>
      <el-table :data="recentTasks" style="width: 100%">
        <el-table-column prop="id" label="ID" width="220" />
        <el-table-column prop="status" label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">
              {{ statusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="进度" width="180">
          <template #default="{ row }">
            {{ row.success }}/{{ row.failed }}/{{ row.total }}
          </template>
        </el-table-column>
        <el-table-column prop="createdAt" label="创建时间" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { getTasks, type Task } from '../api'

const tasks = ref<Task[]>([])

const stats = computed(() => {
  const total = tasks.value.length
  const running = tasks.value.filter((t) => t.status === 'running').length
  const success = tasks.value.reduce((sum, t) => sum + t.success, 0)
  const failed = tasks.value.reduce((sum, t) => sum + t.failed, 0)
  return { total, running, success, failed }
})

const recentTasks = computed(() => {
  return [...tasks.value].slice(0, 10)
})

function statusType(status: string) {
  const map: Record<string, string> = {
    running: 'primary',
    completed: 'success',
    failed: 'danger',
    stopped: 'warning',
  }
  return (map[status] || 'info') as any
}

function statusText(status: string) {
  const map: Record<string, string> = {
    running: '运行中',
    completed: '已完成',
    failed: '失败',
    stopped: '已停止',
  }
  return map[status] || status
}

onMounted(async () => {
  try {
    tasks.value = await getTasks()
  } catch {
    tasks.value = []
  }
})
</script>

<style scoped>
.page-title {
  margin-bottom: 24px;
  color: #e0e0e0;
  font-size: 20px;
}

.stats-row {
  margin-bottom: 24px;
}

.stat-card {
  background-color: #1a1a2e;
  border: 1px solid #2a2a3e;
  border-radius: 8px;
  text-align: center;
  padding: 12px 0;
}

.stat-value {
  font-size: 32px;
  font-weight: bold;
  color: #a78bfa;
}

.stat-card.running .stat-value {
  color: #60a5fa;
}

.stat-card.success .stat-value {
  color: #34d399;
}

.stat-card.fail .stat-value {
  color: #f87171;
}

.stat-label {
  color: #a0a0b0;
  font-size: 13px;
  margin-top: 4px;
}

.recent-tasks {
  background-color: #1a1a2e;
  border: 1px solid #2a2a3e;
  border-radius: 8px;
}

.recent-tasks :deep(.el-card__header) {
  color: #e0e0e0;
  border-bottom: 1px solid #2a2a3e;
}
</style>
