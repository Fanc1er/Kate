import { get, post, put, del } from './http'
import type { PageResult, PageQuery } from '../types'

export interface Asset {
  id: number
  name: string
  url: string
  group_name: string
  status: string
  risk_level: string
  tech_stack?: string
  icp?: string
  source_type?: string
  created_at: string
  updated_at: string
}

export interface AssetProfile {
  tech_stack?: string
  icp?: string
  subdomains?: string[]
  ssl_expire_at?: string
  ports?: string[]
  [key: string]: unknown
}

export interface AssetHistoryItem {
  id: number
  changed_at: string
  field: string
  before: string
  after: string
}

export interface AssetGroup {
  group_name: string
  count: number
}

export function listAssets(params: PageQuery & { keyword?: string; group_name?: string; status?: string; source_type?: string }): Promise<PageResult<Asset>> {
  return get('/assets', params)
}

export function getAsset(id: number): Promise<Asset> {
  return get(`/assets/${id}`)
}

export function createAsset(data: { name: string; url: string; group_name?: string }): Promise<Asset> {
  return post('/assets', data)
}

export function updateAsset(id: number, data: Partial<Asset>): Promise<Asset> {
  return put(`/assets/${id}`, data)
}

export function deleteAsset(id: number): Promise<null> {
  return del(`/assets/${id}`)
}

export function getAssetProfile(id: number): Promise<AssetProfile> {
  return get(`/assets/${id}/profile`)
}

export function getAssetHistory(id: number): Promise<AssetHistoryItem[]> {
  return get(`/assets/${id}/history`)
}

export function listGroups(): Promise<AssetGroup[]> {
  return get('/assets/groups')
}

export function batchDelete(ids: number[]): Promise<{ deleted: number }> {
  return post('/assets/batch-delete', { ids })
}

export function batchScan(ids: number[]): Promise<{ started: number }> {
  return post('/assets/batch-scan', { ids })
}

export function batchGroup(ids: number[], group: string): Promise<{ updated: number }> {
  return post('/assets/batch-group', { ids, group })
}

export function importTemplate(): Promise<Blob> {
  return get('/assets/import-template')
}

export function exportAssets(params: Record<string, unknown>): Promise<Blob> {
  return get('/assets/export', params)
}

export interface WechatAsset {
  id: number
  name: string
  wechat_id: string
  avatar_url: string
  intro: string
  verify_status: string
  fans_count: number
  article_count: number
  created_at: string
}

export function listWechat(page: number, pageSize: number): Promise<PageResult<WechatAsset>> {
  return get('/wechat-assets', { page, page_size: pageSize })
}

export function createWechat(data: Partial<WechatAsset>): Promise<WechatAsset> {
  return post('/wechat-assets', data)
}

export function updateWechat(id: number, data: Partial<WechatAsset>): Promise<WechatAsset> {
  return put(`/wechat-assets/${id}`, data)
}

export function deleteWechat(id: number): Promise<null> {
  return del(`/wechat-assets/${id}`)
}
