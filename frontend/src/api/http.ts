import axios, { type AxiosRequestConfig } from 'axios'
import type { ApiResponse } from '../types'
import { toast } from '../utils/toast'

const http = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
})

export function getToken(): string | null {
  return localStorage.getItem('cinsight_access_token')
}

export function setTokens(access: string, refresh: string): void {
  localStorage.setItem('cinsight_access_token', access)
  localStorage.setItem('cinsight_refresh_token', refresh)
}

export function clearTokens(): void {
  localStorage.removeItem('cinsight_access_token')
  localStorage.removeItem('cinsight_refresh_token')
}

export function getOrgId(): string | null {
  return localStorage.getItem('cinsight_org_id')
}

export function setOrgId(id: string | number): void {
  localStorage.setItem('cinsight_org_id', String(id))
}

export function clearOrgId(): void {
  localStorage.removeItem('cinsight_org_id')
}

http.interceptors.request.use((config) => {
  const token = getToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  const orgId = getOrgId()
  if (orgId) {
    config.headers['X-Org-Id'] = orgId
  }
  return config
})

let redirecting = false

const CONFLICT_CODE = 409

function isConflictCode(code: number): boolean {
  return (
    code === CONFLICT_CODE ||
    code === 3001 || // TASK_STATE_CONFLICT
    code === 4002 // RULE_VERSION_MISMATCH
  )
}

http.interceptors.response.use(
  (res) => {
    const body = res.data as ApiResponse
    if (body && typeof body.code === 'number') {
      if (body.code === 0) {
        return body.data
      }
      if (body.code === 401) {
        handleUnauthorized()
      }
      if (isConflictCode(body.code)) {
        toast.warning('数据已被他人修改，请刷新后重试')
      } else {
        toast.error(body.message || '操作失败')
      }
      return Promise.reject(new ApiError(body.code, body.message))
    }
    return res.data
  },
  (err) => {
    const status = err?.response?.status
    const body = err?.response?.data as ApiResponse | undefined
    if (status === 401) {
      handleUnauthorized()
      return Promise.reject(new ApiError(401, '登录已过期'))
    }
    if (status === 409 || isConflictCode(body?.code ?? -1)) {
      toast.warning('数据已被他人修改，请刷新后重试')
    } else {
      toast.error(body?.message || err?.message || '网络错误')
    }
    const msg = body?.message || err?.message || '网络错误'
    return Promise.reject(new ApiError(body?.code ?? status ?? -1, msg))
  },
)

function handleUnauthorized(): void {
  if (redirecting) return
  redirecting = true
  clearTokens()
  clearOrgId()
  const path = window.location.pathname
  if (!path.startsWith('/login')) {
    window.location.href = '/login'
  }
  setTimeout(() => {
    redirecting = false
  }, 1000)
}

export class ApiError extends Error {
  code: number
  constructor(code: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
  }
}

export async function request<T>(config: AxiosRequestConfig): Promise<T> {
  return http.request(config) as Promise<T>
}

export function get<T>(url: string, params?: Record<string, unknown>): Promise<T> {
  return http.get(url, { params }) as Promise<T>
}

export function post<T>(url: string, data?: unknown): Promise<T> {
  return http.post(url, data) as Promise<T>
}

export function put<T>(url: string, data?: unknown): Promise<T> {
  return http.put(url, data) as Promise<T>
}

export function patch<T>(url: string, data?: unknown): Promise<T> {
  return http.patch(url, data) as Promise<T>
}

export function del<T>(url: string, params?: Record<string, unknown>): Promise<T> {
  return http.delete(url, { params }) as Promise<T>
}
