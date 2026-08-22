export const ROLES = {
  admin: 'admin',
  user: 'user',
} as const

export type Role = (typeof ROLES)[keyof typeof ROLES]

export const ROLE_LABELS: Record<string, string> = {
  admin: '管理员',
  user: '普通用户',
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

const USER_PERMISSIONS = new Set<Permission>([
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
])

export function permissionsOf(role?: string): Set<Permission> {
  switch (role) {
    case ROLES.admin:
      return new Set<Permission>(ALL_PERMISSIONS)
    case ROLES.user:
      return new Set(USER_PERMISSIONS)
    default:
      return new Set<Permission>()
  }
}

export function hasPermission(role: string | undefined, perm: string): boolean {
  return permissionsOf(role).has(perm as Permission)
}
