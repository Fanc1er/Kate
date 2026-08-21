import { get, post, put } from './http'
import type { PageResult, PageQuery } from '../types'

export interface Finding {
  id: number
  org_id: number
  asset_id: number
  task_id: number
  engine_name: string
  type: string
  severity: string
  risk_level?: string
  risk_score: number
  suggestion?: string
  title: string
  description?: string
  url: string
  line_no?: number
  confidence: number
  evidence_ids?: string
  status: string
  created_at: string
}

export interface EventItem {
  id: number
  org_id: number
  asset_id: number
  finding_ids?: string
  engine_name: string
  event_type: string
  title: string
  severity: string
  url?: string
  content?: string
  evidence_ids?: string
  status: string
  sop_attached?: string
  created_at: string
}

export interface AlertItem {
  id: number
  org_id: number
  asset_id: number
  finding_id: number
  alert_type: string
  severity: string
  title: string
  content?: string
  status: string
  created_at: string
  resolved_at?: string
}

export interface Vulnerability {
  id: number
  org_id: number
  asset_id: number
  finding_id: number
  cve_id?: string
  engine_name: string
  severity: string
  status: string
  title: string
  description?: string
  evidence_ids?: string
  first_seen_at: string
  last_seen_at: string
  closed_at?: string
}

export function listFindings(params: PageQuery & { status?: string; severity?: string; engine_name?: string; asset_id?: number; keyword?: string }): Promise<PageResult<Finding>> {
  return get('/findings', params)
}

export function updateFindingStatus(id: number, status: string): Promise<Finding> {
  return put(`/findings/${id}/status`, { status })
}

export function listEvents(params: PageQuery & { severity?: string; status?: string; event_type?: string }): Promise<PageResult<EventItem>> {
  return get('/events', params)
}

export function updateEventStatus(id: number, status: string): Promise<null> {
  return put(`/events/${id}/status`, { status })
}

export function resolveEvent(id: number): Promise<null> {
  return post(`/events/${id}/resolve`)
}

export function listAlerts(params: PageQuery & { severity?: string; status?: string; alert_type?: string }): Promise<PageResult<AlertItem>> {
  return get('/alerts', params)
}

export function resolveAlert(id: number): Promise<null> {
  return post(`/alerts/${id}/resolve`)
}

export function listVulnerabilities(params: PageQuery & { severity?: string; status?: string; asset_id?: number }): Promise<PageResult<Vulnerability>> {
  return get('/vulnerabilities', params)
}
