import { get, post, del } from './http'

export type AvailabilityStatus = 'normal' | 'abnormal' | 'unknown'

export interface AvailabilityItem {
  asset_id: number
  name: string
  url: string
  group_name: string
  importance: string
  status_code: number
  response_ms: number
  sampled_at: string | null
  availability_status: AvailabilityStatus
  sparkline: number[]
}

export interface AvailabilityListResult {
  list: AvailabilityItem[]
  total: number
}

export interface AvailabilityPoint {
  id: number
  asset_id: number
  engine: string
  status_code: number
  response_ms: number
  sampled_at: string
}

export interface WorkerNode {
  id: number
  name: string
  ip: string
  version: string
  status: string
  heartbeat_at: string | null
  load: number
}

export interface WorkerTopology {
  master: { name: string; role: string; status: string }
  workers: WorkerNode[]
}

export interface AvailabilityListParams {
  keyword?: string
  status?: string
  status_code_group?: string
  sort?: string
  sort_order?: string
  page?: number
  page_size?: number
}

export function getAvailabilityList(params: AvailabilityListParams): Promise<AvailabilityListResult> {
  return get('/availability/list', {
    keyword: params.keyword,
    status: params.status,
    status_code_group: params.status_code_group,
    sort: params.sort,
    sort_order: params.sort_order,
    page: params.page,
    page_size: params.page_size,
  })
}

export function getAvailabilityTimeseries(assetId: number, hours = 24): Promise<AvailabilityPoint[]> {
  return get(`/availability/${assetId}/timeseries`, { hours })
}

export function getWorkerTopology(): Promise<WorkerTopology> {
  return get('/availability/worker-topology')
}

export function reprobe(assetIds: number[]): Promise<{ queued: number }> {
  return post('/availability/reprobe', { asset_ids: assetIds })
}

export interface WhitelistRule {
  id: number
  kind: string
  value: string
  remark: string
  enabled: string
}

export function getWhitelist(): Promise<WhitelistRule[]> {
  return get('/availability/whitelist')
}

export function addWhitelist(kind: string, value: string, remark: string): Promise<WhitelistRule> {
  return post('/availability/whitelist', { kind, value, remark })
}

export function removeWhitelist(id: number): Promise<unknown> {
  return del(`/availability/whitelist/${id}`)
}
