import { get, post } from './http'
import type { PageResult, PageQuery } from '../types'

export interface WorkerNode {
  id: number
  org_id: number
  name: string
  client_id: string
  version: string
  status: string
  last_heartbeat_at?: string
  load?: number
  created_at: string
}

export interface Organization {
  id: number
  name: string
  plan: string
  status: string
  created_at: string
}

export function listWorkerNodes(params: PageQuery): Promise<WorkerNode[]> {
  return get('/worker/nodes', params)
}

export function registerWorkerNode(name: string): Promise<{ bootstrap_token: string }> {
  return post('/worker/nodes', { name })
}

export function revokeWorkerNode(id: number): Promise<null> {
  return post(`/worker/nodes/${id}/revoke`)
}

export function listOrganizations(params: PageQuery): Promise<PageResult<Organization>> {
  return get('/admin/organizations', params)
}

export function createOrganization(data: { name: string; plan?: string }): Promise<Organization> {
  return post('/admin/organizations', data)
}

export function getPlatformStats(): Promise<Record<string, unknown>> {
  return get('/admin/stats')
}
