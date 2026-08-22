export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
}

export interface PageResult<T> {
  list: T[]
  total: number
}

export interface PageQuery {
  page?: number
  page_size?: number
  sort?: string
  [key: string]: unknown
}

export interface UserInfo {
  id: number
  username: string
  email: string
  phone?: string
  avatar_url?: string
  status: string
  role: string
  permissions: string[]
}
