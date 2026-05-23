<template>
  <div class="log-viewer">
    <div ref="logContainer" class="log-container">
      <div v-for="(log, index) in logs" :key="index" class="log-line">
        <span class="log-time">{{ formatTime(log.timestamp) }}</span>
        <span class="log-msg">{{ log.message }}</span>
      </div>
      <div v-if="logs.length === 0" class="log-empty">等待日志...</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick, watch } from 'vue'

interface LogEntry {
  taskId: string
  message: string
  timestamp: string
}

const props = defineProps<{
  taskId: string
}>()

const logs = ref<LogEntry[]>([])
const logContainer = ref<HTMLElement | null>(null)
let ws: WebSocket | null = null

function formatTime(ts: string) {
  if (!ts) return ''
  const d = new Date(ts)
  return d.toLocaleTimeString('zh-CN')
}

function scrollToBottom() {
  nextTick(() => {
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  })
}

function connect() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  ws = new WebSocket(`${protocol}//${host}/ws/logs/${props.taskId}`)

  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data) as LogEntry
      logs.value.push(data)
      scrollToBottom()
    } catch {
      logs.value.push({
        taskId: props.taskId,
        message: event.data,
        timestamp: new Date().toISOString(),
      })
      scrollToBottom()
    }
  }

  ws.onerror = () => {
    logs.value.push({
      taskId: props.taskId,
      message: '[连接错误]',
      timestamp: new Date().toISOString(),
    })
  }

  ws.onclose = () => {
    logs.value.push({
      taskId: props.taskId,
      message: '[连接已关闭]',
      timestamp: new Date().toISOString(),
    })
  }
}

function disconnect() {
  if (ws) {
    ws.close()
    ws = null
  }
}

watch(
  () => props.taskId,
  (newId) => {
    disconnect()
    logs.value = []
    if (newId) connect()
  }
)

onMounted(() => {
  if (props.taskId) connect()
})

onUnmounted(disconnect)
</script>

<style scoped>
.log-viewer {
  width: 100%;
}

.log-container {
  background-color: #0a0a14;
  border: 1px solid #2a2a3e;
  border-radius: 6px;
  padding: 12px;
  height: 360px;
  overflow-y: auto;
  font-family: 'Courier New', Courier, monospace;
  font-size: 12px;
  line-height: 1.6;
}

.log-line {
  display: flex;
  gap: 12px;
}

.log-time {
  color: #6b7280;
  white-space: nowrap;
  flex-shrink: 0;
}

.log-msg {
  color: #d1d5db;
  word-break: break-all;
}

.log-empty {
  color: #6b7280;
  text-align: center;
  padding: 40px 0;
}

.log-container::-webkit-scrollbar {
  width: 6px;
}

.log-container::-webkit-scrollbar-track {
  background: #1a1a2e;
}

.log-container::-webkit-scrollbar-thumb {
  background: #4a4a6e;
  border-radius: 3px;
}
</style>
