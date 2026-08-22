import { get, request } from './http'

export interface LicenseStatus {
  status: string
  days_remaining: number
  not_before: string
  not_after: string
  max_assets: number
  max_workers: number
}

export function machineCode(): Promise<{ machine_code: string; source: string }> {
  return get('/license/machine-code')
}

export function status(): Promise<LicenseStatus> {
  return get('/license/status')
}

export function importLicense(content: string): Promise<{ status: string }> {
  return request({
    url: '/license/import',
    method: 'POST',
    data: content,
    headers: { 'Content-Type': 'text/plain' },
  })
}
