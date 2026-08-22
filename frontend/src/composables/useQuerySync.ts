import { type Ref, watch } from 'vue'
import { useRoute, useRouter, type LocationQueryRaw } from 'vue-router'

type SyncEntry = [key: string, ref: Ref<any>]

export interface UseQuerySyncOptions {
  numberKeys?: string[]
  defaults?: Record<string, unknown>
}

/**
 * 将一组响应式状态与 URL query 双向同步：
 * 1. 挂载时从 route.query 恢复初始值（支持刷新/深链）。
 * 2. 状态变化时写回 URL（保留现有 query 参数，默认值会被移除以保持 URL 干净）。
 */
export function useQuerySync(entries: SyncEntry[], options: UseQuerySyncOptions = {}): void {
  const route = useRoute()
  const router = useRouter()
  const numberKeys = new Set(options.numberKeys ?? [])
  const defaults = options.defaults ?? {}

  for (const [key, ref] of entries) {
    const raw = route.query[key]
    if (raw === undefined || raw === null) continue
    const v = Array.isArray(raw) ? raw[0] : raw
    if (numberKeys.has(key)) {
      const n = Number(v)
      if (Number.isFinite(n)) {
        ref.value = n
      }
    } else if (typeof v === 'string') {
      ref.value = v
    }
  }

  let syncing = false
  watch(
    () => entries.map(([, r]) => r.value),
    () => {
      if (syncing) return
      const query: LocationQueryRaw = { ...route.query }
      for (const [key, ref] of entries) {
        const v = ref.value
        if (v === '' || v === null || v === undefined || v === defaults[key]) {
          delete query[key]
        } else {
          query[key] = v
        }
      }
      syncing = true
      router.replace({ query }).catch(() => {})
      syncing = false
    },
  )
}
