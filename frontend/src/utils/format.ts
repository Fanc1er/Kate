import dayjs from 'dayjs'

export function formatTime(input?: string | number | Date): string {
  if (!input) return '-'
  return dayjs(input).format('YYYY-MM-DD HH:mm:ss')
}

export function formatDate(input?: string | number | Date): string {
  if (!input) return '-'
  return dayjs(input).format('YYYY-MM-DD')
}

export function fromNow(input?: string | number | Date): string {
  if (!input) return '-'
  return dayjs(input).format('YYYY-MM-DD HH:mm')
}

export function formatBytes(bytes?: number): string {
  if (bytes == null || Number.isNaN(bytes)) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`
}

export function maskEmail(email?: string): string {
  if (!email) return ''
  const [name, domain] = email.split('@')
  if (!domain) return email
  const first = name.charAt(0)
  const last = name.slice(-1)
  return `${first}***${last}@${domain}`
}

export function maskSecret(secret?: string): string {
  if (!secret) return ''
  if (secret.length <= 4) return '****'
  return `${secret.slice(0, 2)}****${secret.slice(-2)}`
}

export function maskPhone(phone?: string): string {
  if (!phone || phone.length < 7) return phone || ''
  return `${phone.slice(0, 3)}****${phone.slice(-4)}`
}

export function severityColor(severity: string): string {
  switch (severity) {
    case 'critical':
      return '#d03050'
    case 'high':
      return '#ff8000'
    case 'medium':
      return '#ffc800'
    case 'low':
      return '#52c41a'
    default:
      return '#909399'
  }
}

export function severityLabel(severity: string): string {
  const map: Record<string, string> = {
    critical: '严重',
    high: '高危',
    medium: '中危',
    low: '低危',
    info: '提示',
  }
  return map[severity] ?? severity
}

export function statusLabel(status: string): string {
  const map: Record<string, string> = {
    active: '正常',
    disabled: '禁用',
    pending: '待执行',
    processing: '执行中',
    completed: '已完成',
    failed: '失败',
    cancelled: '已取消',
    open: '待处理',
    closed: '已关闭',
    confirmed: '已确认',
    ignored: '已忽略',
    online: '在线',
    offline: '离线',
    running: '运行中',
  }
  return map[status] ?? status
}

export function downloadBlob(blob: Blob, filename: string): void {
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  window.URL.revokeObjectURL(url)
}

export function formatPercent(value?: number): string {
  if (value == null || Number.isNaN(value)) return '-'
  return `${value.toFixed(1)}%`
}
