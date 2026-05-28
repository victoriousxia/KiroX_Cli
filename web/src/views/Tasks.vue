<template>
  <div class="tasks-page">
    <div class="page-header">
      <h2 class="page-title">任务管理</h2>
      <el-button type="primary" @click="showCreateDialog = true">
        <el-icon><Plus /></el-icon>
        新建任务
      </el-button>
    </div>

    <el-card shadow="never" class="task-table-card">
      <el-table :data="tasks" style="width: 100%">
        <el-table-column prop="id" label="任务ID" width="220" show-overflow-tooltip />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">
              {{ statusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="成功/失败/总数" width="150">
          <template #default="{ row }">
            <span class="text-success">{{ row.success }}</span> /
            <span class="text-danger">{{ row.failed }}</span> /
            {{ row.total }}
          </template>
        </el-table-column>
        <el-table-column prop="createdAt" label="创建时间" width="180" />
        <el-table-column label="操作" width="160">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="viewTask(row)">
              详情
            </el-button>
            <el-button
              v-if="row.status === 'running'"
              size="small"
              text
              type="danger"
              @click="handleStop(row.id)"
            >
              停止
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create Task Dialog -->
    <el-dialog
      v-model="showCreateDialog"
      title="新建任务"
      width="520px"
      :close-on-click-modal="false"
    >
      <TaskForm ref="taskFormRef" @submit="handleCreate" @cancel="showCreateDialog = false" />
    </el-dialog>

    <!-- Task Detail Drawer -->
    <el-drawer
      v-model="showDetail"
      title="任务详情"
      size="700px"
      direction="rtl"
    >
      <div v-if="currentTask" class="task-detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="任务ID">{{ currentTask.id }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="statusType(currentTask.status)" size="small">
              {{ statusText(currentTask.status) }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="成功">{{ currentTask.success }}</el-descriptions-item>
          <el-descriptions-item label="失败">{{ currentTask.failed }}</el-descriptions-item>
          <el-descriptions-item label="总数">{{ currentTask.total }}</el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ currentTask.createdAt }}</el-descriptions-item>
        </el-descriptions>

        <!-- Results Table -->
        <div v-if="currentTask.results && currentTask.results.length > 0" class="results-section">
          <h4>注册结果</h4>
          <el-table :data="currentTask.results" size="small" max-height="300">
            <el-table-column prop="email" label="邮箱" min-width="200" show-overflow-tooltip />
            <el-table-column prop="status" label="状态" width="80">
              <template #default="{ row }">
                <el-tag :type="row.status === 'success' ? 'success' : 'danger'" size="small">
                  {{ row.status === 'success' ? '成功' : '失败' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="error" label="错误信息" min-width="200" show-overflow-tooltip>
              <template #default="{ row }">
                <span class="error-text">{{ row.error || '-' }}</span>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div class="log-section">
          <h4>任务日志</h4>
          <LogViewer :task-id="currentTask.id" :history-logs="currentTask.logs" />
        </div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { getTasks, getTaskDetail, stopTask, createTask, type Task, type TaskDetail, type TaskForm as TaskFormType } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'
import TaskForm from '../components/TaskForm.vue'
import LogViewer from '../components/LogViewer.vue'

const tasks = ref<Task[]>([])
const showCreateDialog = ref(false)
const showDetail = ref(false)
const currentTask = ref<TaskDetail | null>(null)

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

async function loadTasks() {
  try {
    tasks.value = await getTasks()
  } catch {
    tasks.value = []
  }
}

async function handleCreate(form: TaskFormType) {
  try {
    await createTask(form)
    ElMessage.success('任务创建成功')
    showCreateDialog.value = false
    await loadTasks()
    startAutoRefresh()
  } catch (e: any) {
    ElMessage.error(e.response?.data?.error || '创建失败')
  }
}

async function viewTask(task: Task) {
  try {
    currentTask.value = await getTaskDetail(task.id)
    showDetail.value = true
  } catch {
    ElMessage.error('获取任务详情失败')
  }
}

// Auto-refresh task list every 3 seconds when there are running tasks
let refreshTimer: ReturnType<typeof setInterval> | null = null

function startAutoRefresh() {
  if (refreshTimer) return
  refreshTimer = setInterval(async () => {
    await loadTasks()
    // Also refresh current task detail if open
    if (showDetail.value && currentTask.value) {
      try {
        currentTask.value = await getTaskDetail(currentTask.value.id)
      } catch { /* ignore */ }
    }
    // Stop refreshing if no running tasks
    const hasRunning = tasks.value.some(t => t.status === 'running')
    if (!hasRunning && refreshTimer) {
      clearInterval(refreshTimer)
      refreshTimer = null
    }
  }, 3000)
}

onMounted(() => {
  loadTasks().then(() => {
    const hasRunning = tasks.value.some(t => t.status === 'running')
    if (hasRunning) startAutoRefresh()
  })
})

async function handleStop(id: string) {
  try {
    await ElMessageBox.confirm('确定要停止该任务吗？', '确认', {
      type: 'warning',
    })
    await stopTask(id)
    ElMessage.success('任务已停止')
    await loadTasks()
  } catch {
    // cancelled or error
  }
}

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
})
</script>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-title {
  color: #e0e0e0;
  font-size: 20px;
}

.task-table-card {
  background-color: #1a1a2e;
  border: 1px solid #2a2a3e;
  border-radius: 8px;
}

.text-success {
  color: #34d399;
}

.text-danger {
  color: #f87171;
}

.task-detail {
  padding: 0 8px;
}

.results-section {
  margin-top: 24px;
}

.results-section h4 {
  color: #e0e0e0;
  margin-bottom: 12px;
}

.error-text {
  color: #f87171;
  font-size: 12px;
}

.log-section {
  margin-top: 24px;
}

.log-section h4 {
  color: #e0e0e0;
  margin-bottom: 12px;
}
</style>
