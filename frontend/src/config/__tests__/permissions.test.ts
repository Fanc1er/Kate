import { describe, it, expect } from 'vitest'
import { hasPermission, permissionsOf } from '../permissions'

describe('v-permission 底层权限判定 hasPermission', () => {
  it('viewer 仅可读资产', () => {
    expect(hasPermission('viewer', 'asset:read')).toBe(true)
    expect(hasPermission('viewer', 'asset:export')).toBe(true)
    expect(hasPermission('viewer', 'task:write')).toBe(false)
    expect(hasPermission('viewer', 'alert:write')).toBe(false)
  })

  it('engineer 可写资产/任务/事件/告警/漏洞，不可写成员/策略', () => {
    expect(hasPermission('engineer', 'asset:write')).toBe(true)
    expect(hasPermission('engineer', 'task:write')).toBe(true)
    expect(hasPermission('engineer', 'event:write')).toBe(true)
    expect(hasPermission('engineer', 'alert:write')).toBe(true)
    expect(hasPermission('engineer', 'vuln:write')).toBe(true)
    expect(hasPermission('engineer', 'member:write')).toBe(false)
    expect(hasPermission('engineer', 'policy:write')).toBe(false)
  })

  it('org_admin 拥有全部权限', () => {
    for (const p of ['asset:write', 'member:write', 'policy:write', 'webhook:write', 'worker:write']) {
      expect(hasPermission('org_admin', p)).toBe(true)
    }
  })

  it('super_admin 拥有全部权限', () => {
    expect(hasPermission('super_admin', 'asset:batch-delete')).toBe(true)
    expect(hasPermission('super_admin', 'report-template:write')).toBe(true)
  })

  it('未知角色无权限', () => {
    expect(hasPermission('guest', 'asset:read')).toBe(false)
    expect(hasPermission(undefined, 'asset:read')).toBe(false)
  })

  it('permissionsOf 返回集合且含全部权限', () => {
    expect(permissionsOf('super_admin').size).toBeGreaterThan(10)
    expect(permissionsOf('org_admin')).toEqual(permissionsOf('super_admin'))
  })
})
