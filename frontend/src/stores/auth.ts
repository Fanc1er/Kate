import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as authApi from '../api/auth'
import { setTokens, clearTokens, setOrgId, clearOrgId, getOrgId } from '../api/http'
import type { UserInfo } from '../types'
import type { OrgEntry } from '../api/auth'
import { permissionsOf } from '../config/permissions'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(localStorage.getItem('cinsight_access_token'))
  const refreshToken = ref<string | null>(localStorage.getItem('cinsight_refresh_token'))
  const user = ref<UserInfo | null>(null)
  const orgId = ref<string | null>(getOrgId())
  const orgName = ref('')
  const organizations = ref<OrgEntry[]>([])
  const needSelectOrg = ref(false)
  const isSuperAdmin = ref(false)

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
    organizations.value = res.organizations ?? []
    isSuperAdmin.value = res.is_super_admin
    needSelectOrg.value = res.need_select_org
    if (!res.need_select_org && res.user?.org_id) {
      setOrgId(res.user.org_id)
      orgId.value = String(res.user.org_id)
      orgName.value = res.user.org_name ?? ''
    }
  }

  async function fetchMe(): Promise<void> {
    const me = await authApi.me()
    user.value = me
    isSuperAdmin.value = me.is_super_admin
    if (me.org_id) {
      setOrgId(me.org_id)
      orgId.value = String(me.org_id)
      orgName.value = me.org_name ?? ''
    }
  }

  async function selectOrg(id: number): Promise<void> {
    const res = await authApi.selectOrg(id)
    applyLogin(res)
    setOrgId(res.org_id)
    orgId.value = String(res.org_id)
    orgName.value = res.org_name
    const userInfo = user.value
    if (userInfo) {
      userInfo.role = res.role
      userInfo.org_id = res.org_id
      userInfo.org_name = res.org_name
    }
    needSelectOrg.value = false
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
    clearOrgId()
    token.value = null
    refreshToken.value = null
    user.value = null
    orgId.value = null
    orgName.value = ''
    organizations.value = []
  }

  return {
    token,
    refreshToken,
    user,
    orgId,
    orgName,
    organizations,
    needSelectOrg,
    isSuperAdmin,
    role,
    permissions,
    isLoggedIn,
    login,
    fetchMe,
    selectOrg,
    logout,
  }
})
