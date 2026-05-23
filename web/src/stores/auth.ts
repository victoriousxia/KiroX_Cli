import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '../api'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string>(localStorage.getItem('token') || '')

  function setToken(t: string) {
    token.value = t
    localStorage.setItem('token', t)
  }

  async function login(password: string) {
    const res = await api.post('/api/login', { password })
    setToken(res.data.token)
  }

  function logout() {
    token.value = ''
    localStorage.removeItem('token')
  }

  return { token, login, logout }
})
