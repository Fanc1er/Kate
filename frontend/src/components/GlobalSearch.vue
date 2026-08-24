<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { listAssets, type Asset } from '../api/asset'
import { listFindings, type Finding } from '../api/event'
import { globalSearch, type SearchDocument } from '../api/search'

const router = useRouter()
const open = ref(false)
const keyword = ref('')
const assets = ref<Asset[]>([])
const findings = ref<Finding[]>([])
const loading = ref(false)
const inputEl = ref<HTMLInputElement | null>(null)
let debounceTimer: number | undefined

watch(open, (v) => {
  if (v) {
    void nextTick(() => inputEl.value?.focus())
  }
})

function onKeydown(e: KeyboardEvent): void {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    open.value = !open.value
    if (open.value) {
      keyword.value = ''
      assets.value = []
      findings.value = []
    }
  } else if (e.key === 'Escape') {
    open.value = false
  }
}

function onInput(): void {
  window.clearTimeout(debounceTimer)
  const kw = keyword.value.trim()
  if (!kw) {
    assets.value = []
    findings.value = []
    return
  }
  debounceTimer = window.setTimeout(async () => {
    loading.value = true
    try {
      const [apiRes, localRes] = await Promise.all([
        globalSearch(kw, 1).catch(() => ({ keyword: kw, total: 0, page: 1, items: [] as SearchDocument[] })),
        Promise.all([
          listAssets({ page: 1, page_size: 5, keyword: kw }),
          listFindings({ page: 1, page_size: 5, keyword: kw }),
        ]),
      ])

      const [la, lf] = localRes as [ { list: Asset[] }, { list: Finding[] } ]
      assets.value = la.list
      findings.value = lf.list

      const seen = new Set<string>()
      for (const d of apiRes.items) {
        const key = `${d.type}:${d.id}`
        if (seen.has(key)) continue
        seen.add(key)
        if (d.type === 'asset') {
          if (!assets.value.find((a) => a.id === d.id)) {
            assets.value.push(d as unknown as Asset)
          }
        } else if (d.type === 'event' && findings.value.length < 5) {
          findings.value.push(d as unknown as Finding)
        }
      }
    } finally {
      loading.value = false
    }
  }, 300)
}

function go(path: string): void {
  open.value = false
  router.push(path)
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div v-if="open" class="search-mask" @click.self="open = false">
    <div class="search-panel">
      <input
        ref="inputEl"
        v-model="keyword"
        class="search-input"
        placeholder="搜索站点 / URL / 发现 / 事件…"
        @input="onInput"
      />
      <div v-if="loading" class="hint">搜索中…</div>
      <template v-else>
        <div v-if="assets.length === 0 && findings.length === 0" class="hint">
          {{ keyword ? '无匹配结果' : '输入关键词开始搜索（Cmd/Ctrl+K 唤起）' }}
        </div>
        <div v-if="assets.length" class="group">
          <div class="group-title">资产</div>
          <button v-for="a in assets" :key="a.id" class="result" @click="go('/assets')">
            <span class="result-name">{{ a.name || a.url }}</span>
            <span class="result-url">{{ a.url }}</span>
          </button>
        </div>
        <div v-if="findings.length" class="group">
          <div class="group-title">发现 & 事件</div>
          <button v-for="f in findings" :key="f.id" class="result" @click="go('/risk/findings')">
            <span class="result-name">{{ f.title }}</span>
            <span class="result-url">{{ f.url }}</span>
          </button>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.search-mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 3000;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 40px 16px;
}

.search-panel {
  background: var(--color-bg-card);
  border-radius: var(--radius-lg);
  width: 100%;
  max-width: 640px;
  max-height: calc(100vh - 80px);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  box-shadow: var(--shadow-hover);
}

.search-input {
  width: 100%;
  height: 48px;
  padding: 0 16px;
  border: none;
  border-bottom: 1px solid var(--color-border);
  font-size: 16px;
  background: var(--color-bg-card);
  color: var(--color-text-primary);
  outline: none;
}

.search-input::placeholder {
  color: var(--color-text-tertiary);
}

.hint {
  padding: 16px;
  text-align: center;
  color: var(--color-text-tertiary);
  font-size: 14px;
}

.group {
  padding: 8px 0;
  border-top: 1px solid var(--color-border-light);
}

.group-title {
  padding: 8px 16px;
  font-size: 12px;
  color: var(--color-text-tertiary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.result {
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: 100%;
  padding: 12px 16px;
  text-align: left;
  background: none;
  border: none;
  cursor: pointer;
  transition: background 0.15s;
}

.result:hover {
  background: var(--color-bg-hover);
}

.result-name {
  font-size: 14px;
  color: var(--color-text-primary);
}

.result-url {
  font-size: 12px;
  color: var(--color-text-tertiary);
  font-family: var(--font-family-mono);
}

@media (max-width: 768px) {
  .search-mask {
    padding: 0;
    align-items: stretch;
  }

  .search-panel {
    max-height: 100vh;
    border-radius: 0;
    width: 100%;
  }

  .search-input {
    font-size: 16px;
    height: 52px;
  }
}
.search-panel {
  width: 560px;
  max-width: 92vw;
  background: var(--color-bg-card);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-card);
  overflow: hidden;
}
.search-input {
  width: 100%;
  height: 48px;
  border: none;
  border-bottom: 1px solid var(--color-border);
  padding: 0 16px;
  font-size: 15px;
  outline: none;
}
.hint {
  padding: 16px;
  color: var(--color-text-tertiary);
  font-size: 13px;
}
.group {
  padding: 8px 0;
  max-height: 260px;
  overflow-y: auto;
}
.group-title {
  padding: 4px 16px;
  font-size: 12px;
  color: var(--color-text-tertiary);
}
.result {
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: 100%;
  text-align: left;
  border: none;
  background: transparent;
  padding: 8px 16px;
  cursor: pointer;
}
.result:hover {
  background: var(--color-bg-hover);
}
.result-name {
  font-size: 13px;
  color: var(--color-text-primary);
}
.result-url {
  font-size: 12px;
  color: var(--color-text-tertiary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
