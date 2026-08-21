import { get, post } from './http'
import type { SelectOrgResult, UserInfo } from '../types'

export interface OrgEntry {
  org_id: number
  name: string
  role: string
  plan: string
  status: string
}

export interface LoginPayload {
  username: string
  password: string
}

export function login(data: LoginPayload): Promise<{
  access_token: string
  refresh_token: string
  user: UserInfo
  organizations: OrgEntry[]
  is_super_admin: boolean
  need_select_org: boolean
}> {
  return post('/auth/login', data)
}

export function logout(refreshToken: string): Promise<null> {
  return post('/auth/logout', { refresh_token: refreshToken })
}

export function me(): Promise<UserInfo> {
  return get('/auth/me')
}

export function selectOrg(orgId: number): Promise<SelectOrgResult> {
  return post('/auth/select-org', { org_id: orgId })
}

export function refresh(refreshToken: string): Promise<{
  access_token: string
  refresh_token: string
}> {
  return post('/auth/refresh', { refresh_token: refreshToken })
}

export function changePassword(oldPassword: string, newPassword: string): Promise<null> {
  return post('/auth/change-password', { old_password: oldPassword, new_password: newPassword })
}

export function forgotPassword(email: string): Promise<null> {
  return post('/auth/forgot-password', { email })
}

export function resetPassword(email: string, code: string, newPassword: string): Promise<null> {
  return post('/auth/reset-password', { email, code, new_password: newPassword })
}
