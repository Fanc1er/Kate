import type { Component } from 'vue'

export interface MenuItem {
  title: string
  icon?: Component
  path: string
  roles: string[]
}

export interface AppRoute {
  path: string
  name: string
  component: () => Promise<unknown>
  meta: {
    title: string
    roles?: string[]
    requiresOrg?: boolean
    hideInMenu?: boolean
  }
}

export const STATIC_ROUTES: AppRoute[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('../views/auth/LoginView.vue'),
    meta: { title: '登录' },
  },
  {
    path: '/select-org',
    name: 'select-org',
    component: () => import('../views/auth/SelectOrgView.vue'),
    meta: { title: '选择组织' },
  },
  {
    path: '/forgot-password',
    name: 'forgot-password',
    component: () => import('../views/auth/ForgotPasswordView.vue'),
    meta: { title: '忘记密码' },
  },
  {
    path: '/reset-password',
    name: 'reset-password',
    component: () => import('../views/auth/ResetPasswordView.vue'),
    meta: { title: '重置密码' },
  },
]

export const ORG_ROUTES: AppRoute[] = [
  {
    path: '/',
    name: 'dashboard',
    component: () => import('../views/dashboard/DashboardView.vue'),
    meta: { title: '仪表盘', requiresOrg: true },
  },
  {
    path: '/assets',
    name: 'assets',
    component: () => import('../views/asset/AssetView.vue'),
    meta: { title: '资产', requiresOrg: true },
  },
  {
    path: '/tasks',
    name: 'tasks',
    component: () => import('../views/task/TaskView.vue'),
    meta: { title: '任务', requiresOrg: true },
  },
  {
    path: '/events',
    name: 'events',
    component: () => import('../views/event/EventView.vue'),
    meta: { title: '安全事件', requiresOrg: true },
  },
  {
    path: '/alerts',
    name: 'alerts',
    component: () => import('../views/alert/AlertView.vue'),
    meta: { title: '告警', requiresOrg: true },
  },
  {
    path: '/vulnerabilities',
    name: 'vulnerabilities',
    component: () => import('../views/vulnerability/VulnerabilityView.vue'),
    meta: { title: '漏洞', requiresOrg: true },
  },
  {
    path: '/findings',
    name: 'findings',
    component: () => import('../views/event/FindingView.vue'),
    meta: { title: '发现', requiresOrg: true },
  },
  {
    path: '/content-security',
    name: 'content-security',
    component: () => import('../views/event/ContentSecurityView.vue'),
    meta: { title: '内容安全', requiresOrg: true },
  },
  {
    path: '/reports',
    name: 'reports',
    component: () => import('../views/report/ReportView.vue'),
    meta: { title: '报告', requiresOrg: true },
  },
  {
    path: '/team',
    name: 'team',
    component: () => import('../views/team/TeamView.vue'),
    meta: { title: '团队', requiresOrg: true, roles: ['org_admin'] },
  },
  {
    path: '/platform',
    name: 'platform',
    component: () => import('../views/platform/PlatformView.vue'),
    meta: { title: '平台管理', requiresOrg: false, roles: ['super_admin'] },
  },
  {
    path: '/policy',
    name: 'policy',
    component: () => import('../views/policy/PolicyView.vue'),
    meta: { title: '策略模板', requiresOrg: true, roles: ['org_admin', 'engineer'] },
  },
]

export const MENU: MenuItem[] = [
  { title: '仪表盘', path: '/', roles: ['super_admin', 'org_admin', 'engineer', 'viewer'] },
  { title: '资产', path: '/assets', roles: ['super_admin', 'org_admin', 'engineer', 'viewer'] },
  { title: '任务', path: '/tasks', roles: ['super_admin', 'org_admin', 'engineer'] },
  { title: '安全事件', path: '/events', roles: ['super_admin', 'org_admin', 'engineer', 'viewer'] },
  { title: '告警', path: '/alerts', roles: ['super_admin', 'org_admin', 'engineer', 'viewer'] },
  { title: '漏洞', path: '/vulnerabilities', roles: ['super_admin', 'org_admin', 'engineer', 'viewer'] },
  { title: '发现', path: '/findings', roles: ['super_admin', 'org_admin', 'engineer', 'viewer'] },
  { title: '内容安全', path: '/content-security', roles: ['super_admin', 'org_admin', 'engineer', 'viewer'] },
  { title: '报告', path: '/reports', roles: ['super_admin', 'org_admin', 'engineer'] },
  { title: '策略模板', path: '/policy', roles: ['org_admin', 'engineer'] },
  { title: '团队', path: '/team', roles: ['org_admin'] },
  { title: '平台管理', path: '/platform', roles: ['super_admin'] },
]

export function canAccess(roles: string[] | undefined, role: string): boolean {
  if (!roles || roles.length === 0) return true
  return roles.includes(role)
}
