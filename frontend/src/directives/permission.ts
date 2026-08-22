import type { Directive, DirectiveBinding } from 'vue'
import { useAuthStore } from '../stores/auth'
import { hasPermission } from '../config/permissions'

function check(value: string | string[], role: string): boolean {
  if (Array.isArray(value)) {
    const roleValues = ['admin', 'user']
    if (value.length > 0 && value.some((v) => roleValues.includes(v))) {
      return value.includes(role)
    }
    return value.every((v) => hasPermission(role, v))
  }
  return hasPermission(role, value)
}

export const permission: Directive = {
  mounted(el: HTMLElement, binding: DirectiveBinding<string | string[]>) {
    const auth = useAuthStore()
    if (!check(binding.value, auth.role)) {
      el.parentNode?.removeChild(el)
    }
  },
}
