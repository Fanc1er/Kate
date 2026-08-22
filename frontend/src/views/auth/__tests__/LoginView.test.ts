import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('vue-router', () => ({
  useRouter: () => ({ replace: vi.fn() }),
}))

vi.mock('../../../api/auth', () => ({
  login: vi.fn(),
}))

import LoginView from '../LoginView.vue'
import * as authApi from '../../../api/auth'

function input(wrapper: ReturnType<typeof mount>, selector: string, value: string): void {
  const el = wrapper.find(selector)
  ;(el.element as HTMLInputElement).value = value
  el.trigger('input')
}

describe('LoginView 表单校验', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('空表单提交提示错误且不调用接口', async () => {
    const wrapper = mount(LoginView, { global: { stubs: { 'router-link': true } } })
    await wrapper.find('form').trigger('submit')
    expect(wrapper.find('.error').text()).toContain('请输入用户名和密码')
    expect(authApi.login).not.toHaveBeenCalled()
  })

  it('填写完整后提交调用 login 接口', async () => {
    vi.mocked(authApi.login).mockResolvedValue({
      access_token: 'at',
      refresh_token: 'rt',
      expires_in: 7200,
      user: {
        id: 1,
        username: 'u',
        email: 'u@example.com',
        status: 'active',
        role: 'admin',
        permissions: [],
      },
    })
    const wrapper = mount(LoginView, { global: { stubs: { 'router-link': true } } })
    input(wrapper, 'input[autocomplete="username"]', 'admin')
    input(wrapper, 'input[autocomplete="current-password"]', 'secret')
    await wrapper.find('form').trigger('submit')
    expect(authApi.login).toHaveBeenCalledWith({ username: 'admin', password: 'secret' })
  })
})
