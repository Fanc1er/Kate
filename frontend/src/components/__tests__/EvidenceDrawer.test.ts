import { describe, it, expect, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

vi.mock('../../api/finding', () => ({
  getEvidence: vi.fn(),
  getEvidenceFile: vi.fn(),
}))

import EvidenceDrawer from '../EvidenceDrawer.vue'
import * as findingApi from '../../api/finding'

function baseEvidence(id: number) {
  return {
    evidence: {
      id,
      sha256: 'a'.repeat(64),
      mime_type: 'text/html',
      size: 12,
      created_at: '2026-08-20T00:00:00Z',
    },
    files: [{ id: id * 10, evidence_id: id, kind: 'resp', sha256: 'a'.repeat(64), size: 12, mime_type: 'text/html', created_at: '2026-08-20T00:00:00Z' }],
  }
}

describe('EvidenceDrawer 证据抽屉', () => {
  async function mountVisible(ids: number[], visible = false) {
    const wrapper = mount(EvidenceDrawer, {
      props: { evidenceIds: ids, visible },
    })
    if (!visible) {
      await wrapper.setProps({ visible: true })
    }
    await flushPromises()
    await flushPromises()
    return wrapper
  }

  it('加载证据并展示文件', async () => {
    vi.mocked(findingApi.getEvidence).mockResolvedValue(baseEvidence(1) as never)
    vi.mocked(findingApi.getEvidenceFile).mockResolvedValue(new Blob(['<p>ok</p>']) as never)
    const wrapper = await mountVisible([1])
    expect(wrapper.text()).toContain('SHA-256')
    expect(findingApi.getEvidence).toHaveBeenCalledWith(1)
    expect(findingApi.getEvidenceFile).toHaveBeenCalledWith(1)
  })

  it('证据读取失败（tampered）标红', async () => {
    vi.mocked(findingApi.getEvidence).mockResolvedValue(baseEvidence(2) as never)
    vi.mocked(findingApi.getEvidenceFile).mockRejectedValue(new Error('EVIDENCE_TAMPERED'))
    const wrapper = await mountVisible([2])
    expect(wrapper.find('.tamper-banner').exists()).toBe(true)
    expect(wrapper.text()).toContain('篡改')
  })

  it('无证据时不渲染内容', async () => {
    const wrapper = await mountVisible([], true)
    expect(wrapper.text()).toContain('无证据')
  })
})
