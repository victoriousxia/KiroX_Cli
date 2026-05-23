<template>
  <div class="app-container">
    <el-container v-if="authStore.token" class="main-layout">
      <el-aside width="200px" class="sidebar">
        <div class="logo">
          <h2>KiroX</h2>
        </div>
        <el-menu
          :default-active="route.path"
        router
          background-color="#1a1a2e"
          text-color="#a0a0b0"
          active-text-color="#a78bfa"
        >
          <el-menu-item index="/dashboard">
            <el-icon><DataAnalysis /></el-icon>
            <span>仪表盘</span>
          </el-menu-item>
          <el-menu-item index="/tasks">
            <el-icon><List /></el-icon>
            <span>任务管理</span>
          </el-menu-item>
          <el-menu-item index="/accounts">
            <el-icon><User /></el-icon>
            <span>账户管理</span>
          </el-menu-item>
          <el-menu-item index="/settings">
            <el-icon><Setting /></el-icon>
            <span>系统设置</span>
          </el-menu-item>
        </el-menu>
        <div class="logout-btn">
          <el-button text @click="handleLogout">
            <el-icon><SwitchButton /></el-icon>
            退出登录
          </el-button>
        </div>
      </el-aside>
      <el-main class="main-content">
        <router-view />
      </el-main>
    </el-container>
    <router-view v-else />
  </div>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from './stores/auth'
import {
  DataAnalysis,
  List,
  User,
  Setting,
  SwitchButton,
} from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

function handleLogout() {
  authStore.logout()
  router.push('/login')
}
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

html, body, #app {
  height: 100%;
  background-color: #0f0f1a;
  color: #e0e0e0;
}

.app-container {
  height: 100%;
}

.main-layout {
  height: 100%;
}

.sidebar {
  background-color: #1a1a2e;
  border-right: 1px solid #2a2a3e;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.logo {
  padding: 20px;
  text-align: center;
  border-bottom: 1px solid #2a2a3e;
}

.logo h2 {
  color: #a78bfa;
  font-size: 22px;
  letter-spacing: 2px;
}

.sidebar .el-menu {
  border-right: none;
  flex: 1;
}

.logout-btn {
  padding: 16px;
  border-top: 1px solid #2a2a3e;
  text-align: center;
}

.logout-btn .el-button {
  color: #a0a0b0;
}

.main-content {
  background-color: #0f0f1a;
  padding: 24px;
  overflow-y: auto;
}

.el-menu-item.is-active {
  background-color: #2a2a4e !important;
}
</style>
