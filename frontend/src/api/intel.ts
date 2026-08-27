import { get, post, del } from './http'
import type { PageResult, PageQuery } from '../types'

export interface IntelItem {
  id: number
  source: string
  intel_id: string
  title: string
  severity: string
  scope: string
  tech_stack: string
  published_at?: string
  updated_at: string
}

export interface IntelInput {
  intel_id: string
  title: string
  description?: string
  severity?: string
  component?: string
  max_version?: string
}

export function listIntel(params: PageQuery): Promise<PageResult<IntelItem>> {
  return get('/intel', params)
}

export function importIntel(items: IntelInput[]): Promise<{ imported: number }> {
  return post('/intel/import', { items })
}

export function deleteIntel(id: number): Promise<void> {
  return del(`/intel/${id}`)
}
