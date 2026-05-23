<template>
  <div class="login-container">
    <div class="login-card">
      <h1 class="login-title">KiroX</h1>
      <p class="login-subtitle">请输入密码登录</p>
      <el-form @submit.prevent="handleLogin">
        <el-form-item>
          <el-input
            v-model="password"
            type="password"
            placeholder="请输入密码"
            size="large"
            show-password
            @keyup.enter="handleLogin"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            class="login-btn"
            @click="handleLogin"
          >
            登录
          </el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { ElMessage } from 'element-plus'

const password = ref('')
const loading = ref(false)
const router = useRouter()
const authStore = useAuthStore()

async function handleLogin() {
  if (!password.value) {
    ElMessage.warning('请输入密码')
    return
  }
  loading.value = true
  try {
    await authStore.login(password.value)
    router.push('/dashboard')
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #0f0f1a 0%, #1a1a3e 100%);
}

.login-card {
  width: 380px;
  padding: 40px;
  background-color: #1a1a2e;
  border-radius: 12px;
  border: 1px solid #2a2a4e;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
}

.login-title {
  text-align: center;
  color: #a78bfa;
  font-size: 32px;
  margin-bottom: 8px;
  letter-spacing: 3px;
}

.login-subtitle {
  text-align: center;
  color: #a0a0b0;
  margin-bottom: 32px;
  font-size: 14px;
}

.login-btn {
  width: 100%;
  background: linear-gradient(135deg, #7c3aed, #6366f1);
  border: none;
}

.login-btn:hover {
  background: linear-gradient(135deg, #6d28d9, #4f46e5);
}
</style>
