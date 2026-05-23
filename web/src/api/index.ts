import axios from 'axios'

export const api = axios.create({
  baseURL: '',
  timeout: 30000,
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

// Auth
export function loginApi(password: string) {
  return api.post<{ token: string }>('/api/login', { password })
}

// Status
export function getStatus() {
  return api.get<{ status: string; version: string; time: string }>('/api/status')
}

// Tasks
export interface TaskForm {
  count: number
  concurrency: number
  delay: number
  proxy: string
  useOutlook: boolean
  outlookCsv: string
  moEmailUrl: string
  moEmailKey: string
}

export interface Task {
  id: string
  status: string
  total: number
  success: number
  failed: number
  createdAt: string
  startedAt?: string
  endedAt?: string
}

export interface TaskResult {
  email: string
  status: string
  error?: string
  subscription?: string
  creditUsed?: number
  creditLimit?: number
}

export interface TaskDetail extends Task {
  config: TaskForm
  results: TaskResult[]
}

export async function createTask(form: TaskForm): Promise<Task> {
  const res = await api.post('/api/tasks', form)
  return res.data.task
}

export async function getTasks(): Promise<Task[]> {
  const res = await api.get('/api/tasks')
  return res.data.tasks || []
}

export async function getTaskDetail(id: string): Promise<TaskDetail> {
  const res = await api.get(`/api/tasks/${id}`)
  return res.data.task
}

export function stopTask(id: string) {
  return api.post(`/api/tasks/${id}/stop`)
}

// Accounts
export interface Account {
  email: string
  subscription: string
  creditUsed: number
  creditLimit: number
  provider: string
  region: string
}

export interface VerifyAccountInput {
  clientId: string
  clientSecret: string
  refreshToken: string
  proxy?: string
}

export interface VerifyResult {
  alive: boolean
  email: string
  subscription: string
  creditUsed: number
  creditLimit: number
  suspended?: boolean
  error?: string
}

export async function getAccounts(): Promise<Account[]> {
  const res = await api.get('/api/accounts')
  return res.data.accounts || []
}

export async function verifyAccounts(accounts: VerifyAccountInput[]): Promise<VerifyResult[]> {
  const res = await api.post('/api/accounts/verify', accounts)
  return res.data.results || []
}

// Config
export interface AppConfig {
  proxy: string
  moEmailUrl: string
  moEmailKey: string
}

export async function getConfig(): Promise<AppConfig> {
  const res = await api.get('/api/config')
  return res.data.config
}

export function updateConfig(config: AppConfig) {
  return api.post('/api/config', config)
}

export function uploadOutlookCsv(file: File) {
  const formData = new FormData()
  formData.append('file', file)
  return api.post('/api/config/outlook', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}
