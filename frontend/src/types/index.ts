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

export interface LoginResult {
  access_token: string
  refresh_token: string
  expires_in: number
  token_type: string
}

export interface OrgBrief {
  id: number
  name: string
  role: string
  status: string
}

export interface UserInfo {
  id: number
  username: string
  email: string
  phone?: string
  avatar_url?: string
  status: string
  is_super_admin: boolean
  org_id?: number
  org_name?: string
  role: string
  permissions: string[]
}

export interface SelectOrgResult {
  access_token: string
  refresh_token: string
  org_id: number
  org_name: string
  role: string
}
