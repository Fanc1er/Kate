<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { listAssets, type Asset } from '../api/asset'
import { listFindings, type Finding } from '../api/event'

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
      const [a, f] = await Promise.all([
        listAssets({ page: 1, page_size: 5, keyword: kw }),
        listFindings({ page: 1, page_size: 5, keyword: kw }),
      ])
      assets.value = a.list
      findings.value = f.list
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
        placeholder="搜索站点 / URL / 发现…"
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
          <div class="group-title">发现</div>
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
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 12vh;
  z-index: 2000;
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
