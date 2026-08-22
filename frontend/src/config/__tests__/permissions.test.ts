import { describe, it, expect } from 'vitest'
import { hasPermission, permissionsOf, ALL_PERMISSIONS } from '../permissions'

describe('v-permission 底层权限判定 hasPermission', () => {
  it('user 可读写业务，不可管理成员', () => {
    expect(hasPermission('user', 'asset:read')).toBe(true)
    expect(hasPermission('user', 'asset:write')).toBe(true)
    expect(hasPermission('user', 'task:write')).toBe(true)
    expect(hasPermission('user', 'event:write')).toBe(true)
    expect(hasPermission('user', 'alert:write')).toBe(true)
    expect(hasPermission('user', 'vuln:write')).toBe(true)
    expect(hasPermission('user', 'member:write')).toBe(false)
  })

  it('admin 拥有全部权限', () => {
    expect(hasPermission('admin', 'asset:batch-delete')).toBe(true)
    expect(hasPermission('admin', 'member:write')).toBe(true)
    expect(hasPermission('admin', 'report-template:write')).toBe(true)
  })

  it('未知角色无权限', () => {
    expect(hasPermission('guest', 'asset:read')).toBe(false)
    expect(hasPermission(undefined, 'asset:read')).toBe(false)
  })

  it('permissionsOf 返回集合且 admin 含全部权限', () => {
    expect(permissionsOf('admin').size).toBe(ALL_PERMISSIONS.length)
    for (const p of ALL_PERMISSIONS) {
      expect(permissionsOf('admin').has(p)).toBe(true)
    }
  })
})
