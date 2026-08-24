<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { globalSearch, type SearchDocument } from '../../api/search'
import Skeleton from '../../components/Skeleton.vue'
import { useRouter } from 'vue-router'

const router = useRouter()

const keyword = ref('')
const loading = ref(false)
const results = ref<SearchDocument[]>([])
const total = ref(0)
const page = ref(1)

async function doSearch(): Promise<void> {
  if (!keyword.value.trim()) return
  loading.value = true
  try {
    const res = await globalSearch(keyword.value.trim(), page.value)
    results.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

function handleClick(doc: SearchDocument): void {
  if (doc.type === 'asset') {
    router.push({ path: '/assets', query: { keyword: doc.title } })
  } else if (doc.type === 'finding') {
    router.push({ path: '/risk/findings', query: { keyword: doc.title } })
  } else {
    router.push({ path: '/risk/events', query: { keyword: doc.title } })
  }
}

const severityColor: Record<string, string> = {
  critical: '#ef4444',
  high: '#f97316',
  medium: '#eab308',
  low: '#22c55e',
  info: '#3b82f6',
}

onMounted(() => {
  // 从 URL 参数读取搜索词
  const q = new URLSearchParams(window.location.search).get('q')
  if (q) keyword.value = q
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center gap-3">
      <h2 class="text-lg font-semibold text-gray-800">全局搜索</h2>
    </div>

    <div class="flex gap-2">
      <input
        v-model="keyword"
        type="text"
        placeholder="搜索资产、发现、事件..."
        class="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        @keyup.enter="doSearch"
      />
      <button
        @click="doSearch"
        :disabled="loading"
        class="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700 disabled:opacity-50"
      >
        {{ loading ? '搜索中...' : '搜索' }}
      </button>
    </div>

    <div v-if="loading" class="space-y-2">
      <Skeleton class="h-12 w-full" />
      <Skeleton class="h-12 w-full" />
      <Skeleton class="h-12 w-full" />
    </div>

    <div v-else-if="results.length === 0 && keyword" class="text-center py-12 text-gray-500 text-sm">
      未找到与「{{ keyword }}」相关的结果
    </div>

    <div v-else-if="results.length > 0" class="space-y-2">
      <p class="text-xs text-gray-500 mb-2">共 {{ total }} 条结果</p>
      <div
        v-for="doc in results"
        :key="`${doc.type}:${doc.id}`"
        @click="handleClick(doc)"
        class="flex items-start gap-3 p-3 border border-gray-200 rounded-lg hover:bg-gray-50 cursor-pointer"
      >
        <div
          class="w-8 h-8 rounded-lg flex items-center justify-center text-xs font-bold text-white flex-shrink-0"
          :style="{ background: doc.type === 'asset' ? '#3b82f6' : doc.type === 'finding' ? '#f97316' : '#8b5cf6' }"
        >
          {{ doc.type === 'asset' ? '资' : doc.type === 'finding' ? '发' : '事' }}
        </div>
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-gray-800 truncate">{{ doc.title }}</span>
            <span
              v-if="doc.severity"
              class="text-xs px-1.5 py-0.5 rounded"
              :style="{ color: severityColor[doc.severity] ?? '#6b7280', background: (severityColor[doc.severity] ?? '#6b7280') + '15' }"
            >
              {{ doc.severity }}
            </span>
            <span
              v-if="doc.engine"
              class="text-xs text-gray-400"
            >
              {{ doc.engine }}
            </span>
          </div>
          <p class="text-xs text-gray-500 truncate mt-0.5">{{ doc.url }}</p>
          <p class="text-xs text-gray-600 mt-1 line-clamp-2">{{ doc.content }}</p>
        </div>
      </div>
    </div>
  </div>
</template>
