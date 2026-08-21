export const ROLES = {
  super_admin: 'super_admin',
  org_admin: 'org_admin',
  engineer: 'engineer',
  viewer: 'viewer',
} as const

export type Role = (typeof ROLES)[keyof typeof ROLES]

export const ROLE_LABELS: Record<string, string> = {
  super_admin: '平台管理员',
  org_admin: '组织管理员',
  engineer: '安全工程师',
  viewer: '只读访客',
}

export const ALL_PERMISSIONS = [
  'asset:read',
  'asset:export',
  'asset:write',
  'asset:batch-delete',
  'wechat:write',
  'policy:write',
  'plan:write',
  'task:write',
  'task:delete',
  'event:write',
  'noise:write',
  'alert:write',
  'vuln:write',
  'ticket:write',
  'evidence:upload',
  'report:delete',
  'report-template:write',
  'member:write',
  'worker:write',
  'channel:write',
  'route:write',
  'rules:write',
  'intel-sub:write',
  'whitelist:write',
  'token:write',
  'webhook:write',
  'scenario:write',
  'escalation:write',
  'watch:write',
] as const

export type Permission = (typeof ALL_PERMISSIONS)[number]

const ENGINEER_PERMISSIONS = new Set<Permission>([
  'asset:read',
  'asset:export',
  'asset:write',
  'wechat:write',
  'task:write',
  'event:write',
  'alert:write',
  'vuln:write',
  'ticket:write',
  'evidence:upload',
])

const VIEWER_PERMISSIONS = new Set<Permission>(['asset:read', 'asset:export'])

export function permissionsOf(role?: string): Set<Permission> {
  switch (role) {
    case ROLES.super_admin:
    case ROLES.org_admin:
      return new Set<Permission>(ALL_PERMISSIONS)
    case ROLES.engineer:
      return new Set(ENGINEER_PERMISSIONS)
    case ROLES.viewer:
      return new Set(VIEWER_PERMISSIONS)
    default:
      return new Set<Permission>()
  }
}

export function hasPermission(role: string | undefined, perm: string): boolean {
  return permissionsOf(role).has(perm as Permission)
}
