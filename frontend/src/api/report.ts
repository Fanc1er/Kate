import { get, post, put, del, getToken } from './http'

export interface ReportTemplate {
  id: number
  name: string
  period: string
  cron_expr: string
  timezone: string
  enabled: boolean
  last_run_at?: string
  created_at: string
}

export interface ReportArchive {
  id: number
  template_id: number
  name: string
  title: string
  format: string
  status: string
  file_path: string
  snapshot: string
  created_at: string
}

export interface TemplateInput {
  name: string
  period: string
  cron_expr: string
  timezone: string
  enabled: boolean
}

export function listTemplates(): Promise<ReportTemplate[]> {
  return get('/report-templates')
}

export function createTemplate(input: TemplateInput): Promise<ReportTemplate> {
  return post('/report-templates', input)
}

export function updateTemplate(id: number, patch: Partial<TemplateInput>): Promise<ReportTemplate> {
  return put(`/report-templates/${id}`, patch)
}

export function deleteTemplate(id: number): Promise<void> {
  return del(`/report-templates/${id}`)
}

export function runTemplate(id: number): Promise<ReportArchive> {
  return post(`/report-templates/${id}/run`)
}

export function listArchives(): Promise<ReportArchive[]> {
  return get('/reports')
}

export function deleteArchive(id: number): Promise<void> {
  return del(`/reports/${id}`)
}

export async function downloadArchive(id: number, filename: string): Promise<void> {
  const res = await fetch(`/api/reports/${id}/download`, {
    headers: { Authorization: `Bearer ${getToken()}` },
  })
  if (!res.ok) throw new Error('下载失败')
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}
