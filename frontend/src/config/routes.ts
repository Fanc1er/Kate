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
    path: '/license',
    name: 'license',
    component: () => import('../views/license/LicenseView.vue'),
    meta: { title: '授权' },
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

export const APP_ROUTES: AppRoute[] = [
  {
    path: '/',
    name: 'dashboard',
    component: () => import('../views/dashboard/DashboardView.vue'),
    meta: { title: '仪表盘' },
  },
  {
    path: '/assets',
    name: 'assets',
    component: () => import('../views/asset/AssetView.vue'),
    meta: { title: '资产' },
  },
  {
    path: '/tasks',
    name: 'tasks',
    component: () => import('../views/task/TaskView.vue'),
    meta: { title: '任务' },
  },
  {
    path: '/events',
    name: 'events',
    component: () => import('../views/event/EventView.vue'),
    meta: { title: '安全事件' },
  },
  {
    path: '/alerts',
    name: 'alerts',
    component: () => import('../views/alert/AlertView.vue'),
    meta: { title: '告警' },
  },
  {
    path: '/vulnerabilities',
    name: 'vulnerabilities',
    component: () => import('../views/vulnerability/VulnerabilityView.vue'),
    meta: { title: '漏洞' },
  },
  {
    path: '/findings',
    name: 'findings',
    component: () => import('../views/event/FindingView.vue'),
    meta: { title: '发现' },
  },
  {
    path: '/content-security',
    name: 'content-security',
    component: () => import('../views/event/ContentSecurityView.vue'),
    meta: { title: '内容安全' },
  },
  {
    path: '/tickets',
    name: 'tickets',
    component: () => import('../views/task/TicketView.vue'),
    meta: { title: '工单' },
  },
  {
    path: '/reports',
    name: 'reports',
    component: () => import('../views/report/ReportView.vue'),
    meta: { title: '报告' },
  },
  {
    path: '/members',
    name: 'members',
    component: () => import('../views/members/MembersView.vue'),
    meta: { title: '用户管理', roles: ['admin'] },
  },
  {
    path: '/platform',
    name: 'platform',
    component: () => import('../views/platform/PlatformView.vue'),
    meta: { title: '平台管理', roles: ['admin'] },
  },
  {
    path: '/policy',
    name: 'policy',
    component: () => import('../views/policy/PolicyView.vue'),
    meta: { title: '策略模板' },
  },
]

export const MENU: MenuItem[] = [
  { title: '仪表盘', path: '/', roles: ['admin', 'user'] },
  { title: '资产', path: '/assets', roles: ['admin', 'user'] },
  { title: '任务', path: '/tasks', roles: ['admin', 'user'] },
  { title: '安全事件', path: '/events', roles: ['admin', 'user'] },
  { title: '告警', path: '/alerts', roles: ['admin', 'user'] },
  { title: '漏洞', path: '/vulnerabilities', roles: ['admin', 'user'] },
  { title: '发现', path: '/findings', roles: ['admin', 'user'] },
  { title: '内容安全', path: '/content-security', roles: ['admin', 'user'] },
  { title: '工单', path: '/tickets', roles: ['admin', 'user'] },
  { title: '报告', path: '/reports', roles: ['admin', 'user'] },
  { title: '策略模板', path: '/policy', roles: ['admin', 'user'] },
  { title: '用户管理', path: '/members', roles: ['admin'] },
  { title: '平台管理', path: '/platform', roles: ['admin'] },
]

export function canAccess(roles: string[] | undefined, role: string): boolean {
  if (!roles || roles.length === 0) return true
  return roles.includes(role)
}
