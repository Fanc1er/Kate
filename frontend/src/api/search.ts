import { get } from './http'

export interface SearchDocument {
  id: number
  type: 'asset' | 'finding' | 'event'
  title: string
  url: string
  content: string
  severity?: string
  engine?: string
}

export interface GlobalSearchResult {
  keyword: string
  total: number
  page: number
  items: SearchDocument[]
}

export async function globalSearch(
  keyword: string,
  page = 1,
): Promise<GlobalSearchResult> {
  const res = await get<GlobalSearchResult>('/search/global', {
    params: { q: keyword, page },
  })
  return res
}
