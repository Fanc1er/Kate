import { get, post, put, del } from './http'
import type { PageResult, PageQuery } from '../types'

export interface Policy {
  id: number
  name: string
  scenario: string
  engine_switches: string
  concurrency: number
  timeout: number
  rate_limit: number
  scan_depth: number
  allow_static: boolean
  same_origin: boolean
  crawl_subpages: boolean
  version: number
  created_at: string
  updated_at: string
}

export interface ScanTask {
  id: number
  asset_id: number
  asset_name?: string
  policy_id: number
  status: string
  progress: number
  worker_id?: string
  started_at?: string
  finished_at?: string
  created_at: string
  findings_count?: number
  task_timeout?: boolean
  stopped_by_user?: boolean
}

export function listTasks(params: PageQuery & { status?: string; asset_id?: number }): Promise<PageResult<ScanTask>> {
  return get('/tasks', params)
}

export function getTask(id: number): Promise<ScanTask> {
  return get(`/tasks/${id}`)
}

export function getTaskProgress(id: number): Promise<{ progress: number; status: string }> {
  return get(`/tasks/${id}/progress`)
}

export function createTask(data: { asset_ids: number[]; policy_id: number; priority?: string }): Promise<ScanTask> {
  return post('/tasks', data)
}

export function stopTask(id: number): Promise<null> {
  return post(`/tasks/${id}/stop`)
}

export function rerunTask(id: number): Promise<null> {
  return post(`/tasks/${id}/rerun`)
}

export function deleteTask(id: number): Promise<null> {
  return del(`/tasks/${id}`)
}

export function batchStop(ids: number[]): Promise<{ stopped: number }> {
  return post('/tasks/batch-stop', { ids })
}

export function batchRerun(ids: number[]): Promise<{ started: number }> {
  return post('/tasks/batch-rerun', { ids })
}

export function listQueue(): Promise<unknown[]> {
  return get('/tasks/queue')
}

export function listPolicies(): Promise<Policy[]> {
  return get('/policies')
}

export function createPolicy(data: Partial<Policy>): Promise<Policy> {
  return post('/policies', data)
}

export function updatePolicy(id: number, data: Partial<Policy>): Promise<Policy> {
  return put(`/policies/${id}`, data)
}
