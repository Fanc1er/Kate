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
  redirect?: string
  children?: AppRoute[]
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
    path: '/risk',
    name: 'risk',
    component: () => import('../views/risk/RiskCenterView.vue'),
    redirect: '/risk/findings',
    meta: { title: '风险中心' },
    children: [
      {
        path: 'findings',
        name: 'risk-findings',
        component: () => import('../views/event/FindingView.vue'),
        meta: { title: '发现' },
      },
      {
        path: 'events',
        name: 'risk-events',
        component: () => import('../views/event/EventView.vue'),
        meta: { title: '安全事件' },
      },
      {
        path: 'alerts',
        name: 'risk-alerts',
        component: () => import('../views/alert/AlertView.vue'),
        meta: { title: '告警' },
      },
      {
        path: 'vulnerabilities',
        name: 'risk-vulnerabilities',
        component: () => import('../views/vulnerability/VulnerabilityView.vue'),
        meta: { title: '漏洞' },
      },
    ],
  },
  {
    path: '/content-security',
    name: 'content-security',
    component: () => import('../views/event/ContentSecurityView.vue'),
    meta: { title: '内容安全' },
  },
  {
    path: '/availability',
    name: 'availability',
    component: () => import('../views/availability/AvailabilityView.vue'),
    meta: { title: '可用性监测' },
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
  {
    path: '/intel',
    name: 'intel',
    component: () => import('../views/intel/IntelView.vue'),
    meta: { title: '情报库' },
  },
  {
    path: '/notify',
    name: 'notify',
    component: () => import('../views/notify/NotifyView.vue'),
    meta: { title: '通知渠道', roles: ['admin'] },
  },
  {
    path: '/search',
    name: 'search',
    component: () => import('../views/search/SearchView.vue'),
    meta: { title: '全局搜索' },
  },
  {
    path: '/tasks/queue',
    name: 'task-queue',
    component: () => import('../views/task/TaskQueueView.vue'),
    meta: { title: '任务队列' },
  },
  {
    path: '/engines',
    name: 'engines',
    component: () => import('../views/engine/EngineOverviewView.vue'),
    meta: { title: '引擎总览' },
    redirect: '/engines/vuln_scan',
    children: [
      { path: 'vuln_scan', name: 'engine-vuln-scan', component: () => import('../views/engine/VulnScanView.vue'), meta: { title: '漏洞扫描' } },
      { path: 'hidden_link', name: 'engine-hidden-link', component: () => import('../views/engine/HiddenLinkView.vue'), meta: { title: '暗链检测' } },
      { path: 'webshell', name: 'engine-webshell', component: () => import('../views/engine/WebshellView.vue'), meta: { title: 'Webshell' } },
      { path: 'phishing', name: 'engine-phishing', component: () => import('../views/engine/PhishingView.vue'), meta: { title: '钓鱼检测' } },
      { path: 'port_service', name: 'engine-port-service', component: () => import('../views/engine/PortServiceView.vue'), meta: { title: '端口服务' } },
      { path: 'dns_security', name: 'engine-dns-security', component: () => import('../views/engine/DNSSecurityView.vue'), meta: { title: 'DNS 安全' } },
      { path: 'threat_intelligence', name: 'engine-threat-intelligence', component: () => import('../views/engine/ThreatIntelligenceView.vue'), meta: { title: '威胁情报' } },
      { path: 'intelligence', name: 'engine-intelligence', component: () => import('../views/engine/IntelligenceView.vue'), meta: { title: '情报关联' } },
    ],
  },
]

export const MENU: MenuItem[] = [
  { title: '仪表盘', path: '/', roles: ['admin', 'user'] },
  { title: '资产', path: '/assets', roles: ['admin', 'user'] },
  { title: '任务', path: '/tasks', roles: ['admin', 'user'] },
  { title: '风险中心', path: '/risk', roles: ['admin', 'user'] },
  { title: '内容安全', path: '/content-security', roles: ['admin', 'user'] },
  { title: '可用性监测', path: '/availability', roles: ['admin', 'user'] },
  { title: '引擎总览', path: '/engines', roles: ['admin', 'user'] },
  { title: '工单', path: '/tickets', roles: ['admin', 'user'] },
  { title: '报告', path: '/reports', roles: ['admin', 'user'] },
  { title: '策略模板', path: '/policy', roles: ['admin', 'user'] },
  { title: '情报库', path: '/intel', roles: ['admin', 'user'] },
  { title: '通知渠道', path: '/notify', roles: ['admin'] },
  { title: '用户管理', path: '/members', roles: ['admin'] },
  { title: '平台管理', path: '/platform', roles: ['admin'] },
]

export function canAccess(roles: string[] | undefined, role: string): boolean {
  if (!roles || roles.length === 0) return true
  return roles.includes(role)
}
