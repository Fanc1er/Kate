import { get, post } from './http'
import type { PageQuery } from '../types'

export interface WorkerNode {
  id: number
  name: string
  client_id: string
  version: string
  status: string
  last_heartbeat_at?: string
  load?: number
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

export function getPlatformStats(): Promise<Record<string, unknown>> {
  return get('/admin/stats')
}
