import { describe, it, expect } from 'vitest'
import { sanitizeHtml } from '../sanitize'
import { canAccess } from '../../config/routes'

describe('sanitizeHtml XSS 净化', () => {
  it('剥离 script 标签', () => {
    const out = sanitizeHtml('<p>hello</p><script>alert(1)</script>')
    expect(out).not.toContain('<script')
    expect(out).toContain('hello')
  })

  it('剥离事件属性 onerror', () => {
    const out = sanitizeHtml('<img src="x" onerror="alert(1)">')
    expect(out).not.toContain('onerror')
    expect(out).not.toContain('alert')
  })

  it('剥离 iframe', () => {
    const out = sanitizeHtml('<div>ok</div><iframe src="http://evil"></iframe>')
    expect(out).not.toContain('iframe')
  })

  it('剥离 javascript: 协议 href', () => {
    const out = sanitizeHtml('<a href="javascript:alert(1)">x</a>')
    expect(out).not.toContain('javascript:')
  })

  it('保留安全文本与合法 http 链接', () => {
    const out = sanitizeHtml('<p>正文</p><a href="https://example.com">链接</a>')
    expect(out).toContain('正文')
    expect(out).toContain('https://example.com')
  })
})

describe('canAccess 权限判定', () => {
  it('无 roles 限制时放行', () => {
    expect(canAccess(undefined, 'viewer')).toBe(true)
    expect(canAccess([], 'viewer')).toBe(true)
  })

  it('命中角色放行', () => {
    expect(canAccess(['org_admin', 'engineer'], 'engineer')).toBe(true)
    expect(canAccess(['super_admin'], 'super_admin')).toBe(true)
  })

  it('未命中角色拒绝', () => {
    expect(canAccess(['org_admin'], 'viewer')).toBe(false)
    expect(canAccess(['super_admin'], 'org_admin')).toBe(false)
  })

  it('平台管理仅 super_admin 可访问', () => {
    expect(canAccess(['super_admin'], 'super_admin')).toBe(true)
    expect(canAccess(['super_admin'], 'org_admin')).toBe(false)
  })
})
