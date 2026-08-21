import { get, request } from './http'
import type { PageResult } from '../types'

export interface EvidenceMeta {
  id: number
  md5?: string
  sha256: string
  mime_type: string
  size: number
  created_at: string
}

export interface EvidenceFile {
  id: number
  evidence_id: number
  kind: string
  sha256: string
  size: number
  mime_type: string
  created_at: string
}

export interface EvidenceDetail {
  evidence: EvidenceMeta
  files: EvidenceFile[]
}

export function listEvidence(findingId: number): Promise<PageResult<EvidenceMeta>> {
  return get('/evidence', { finding_id: findingId })
}

export function getEvidence(id: number): Promise<EvidenceDetail> {
  return get(`/evidence/${id}`)
}

export function downloadEvidence(id: number): Promise<Blob> {
  return request<Blob>({ url: `/evidence/${id}/download`, method: 'get', responseType: 'blob' })
}

export function getEvidenceFile(id: number): Promise<Blob> {
  return request<Blob>({ url: `/evidence/${id}/file`, method: 'get', responseType: 'blob' })
}

export function getVulnEvidence(vulnId: number): Promise<EvidenceMeta[]> {
  return get(`/vulnerabilities/${vulnId}/evidence`)
}
