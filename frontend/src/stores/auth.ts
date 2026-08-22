import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as authApi from '../api/auth'
import { setTokens, clearTokens } from '../api/http'
import type { UserInfo } from '../types'
import { permissionsOf } from '../config/permissions'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(localStorage.getItem('cinsight_access_token'))
  const refreshToken = ref<string | null>(localStorage.getItem('cinsight_refresh_token'))
  const user = ref<UserInfo | null>(null)

  const role = computed(() => user.value?.role ?? '')
  const permissions = computed(() => permissionsOf(role.value))

  const isLoggedIn = computed(() => !!token.value)

  function applyLogin(data: { access_token: string; refresh_token: string }): void {
    setTokens(data.access_token, data.refresh_token)
    token.value = data.access_token
    refreshToken.value = data.refresh_token
  }

  async function login(username: string, password: string): Promise<void> {
    const res = await authApi.login({ username, password })
    applyLogin(res)
    user.value = res.user
  }

  async function fetchMe(): Promise<void> {
    user.value = await authApi.me()
  }

  async function logout(): Promise<void> {
    if (refreshToken.value) {
      try {
        await authApi.logout(refreshToken.value)
      } catch {
        // 忽略登出失败
      }
    }
    clearTokens()
    token.value = null
    refreshToken.value = null
    user.value = null
  }

  return {
    token,
    refreshToken,
    user,
    role,
    permissions,
    isLoggedIn,
    login,
    fetchMe,
    logout,
  }
})
