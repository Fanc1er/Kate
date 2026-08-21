import type { ComponentPublicInstance } from 'vue'

export type ToastExpose = {
  success: (message: string, duration?: number) => void
  error: (message: string, duration?: number) => void
  warning: (message: string, duration?: number) => void
  info: (message: string, duration?: number) => void
}

let inst: ToastExpose | null = null

export function registerToast(cmp: ComponentPublicInstance<ToastExpose> | ToastExpose | null): void {
  inst = (cmp as unknown as ToastExpose | null)
}

function call(method: keyof ToastExpose, message: string, duration?: number): void {
  inst?.[method]?.(message, duration)
}

export const toast = {
  success: (m: string, d?: number) => call('success', m, d),
  error: (m: string, d?: number) => call('error', m, d),
  warning: (m: string, d?: number) => call('warning', m, d),
  info: (m: string, d?: number) => call('info', m, d),
}

export function confirmDialog(message: string): boolean {
  return window.confirm(message)
}
