import { get } from './http'

export interface DashboardStats {
  assets: number
  findings: number
  events_today: number
  alerts_open: number
  critical: number
  high: number
  tasks: number
  tasks_today: number
  coverage: number
  availability: number
}

export interface TrendPoint {
  date: string
  [key: string]: unknown
}

export interface TopRisk {
  title: string
  risk_score: number
  severity: string
  url: string
  created_at: string
}

export interface EngineCoverage {
  [engine: string]: number
}

export function getStats(): Promise<DashboardStats> {
  return get('/dashboard/stats')
}

export function getTrends(days = 7): Promise<{ dates: string[]; findings: number[]; alerts: number[]; availability: number[] }> {
  return get('/dashboard/trends', { days })
}

export function getTopRisks(limit = 10): Promise<TopRisk[]> {
  return get('/dashboard/top-risks', { limit })
}

export function getEngineCoverage(): Promise<EngineCoverage> {
  return get('/dashboard/engine-coverage')
}
