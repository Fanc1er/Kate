<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { listTasks, type ScanTask } from '../../api/task'
import { formatTime } from '../../utils/format'
import Skeleton from '../../components/Skeleton.vue'

const list = ref<ScanTask[]>([])
const total = ref(0)
const loading = ref(false)
const refreshing = ref(false)

async function load(): Promise<void> {
  if (loading.value) return
  loading.value = true
  refreshing.value = true
  try {
    const res = await listTasks({ page: 1, page_size: 50 })
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

let timer: ReturnType<typeof setInterval> | null = null
function startPolling(): void {
  timer = setInterval(() => {
    if (list.value.some((t) => t.status === 'running')) {
      load()
    }
  }, 5000)
}
function stopPolling(): void {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

onMounted(() => {
  void load()
  startPolling()
})
onBeforeUnmount(stopPolling)

const statusColor: Record<string, string> = {
  pending: '#6b7280',
  running: '#3b82f6',
  completed: '#22c55e',
  failed: '#ef4444',
  stopped: '#f97316',
}

const statusLabel: Record<string, string> = {
  pending: '等待中',
  running: '运行中',
  completed: '已完成',
  failed: '失败',
  stopped: '已停止',
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between">
      <h2 class="text-lg font-semibold text-gray-800">任务队列监控</h2>
      <button
        @click="void load()"
        :disabled="refreshing"
        class="px-3 py-1.5 text-sm border border-gray-300 rounded-lg hover:bg-gray-50 disabled:opacity-50"
      >
        {{ refreshing ? '刷新中...' : '刷新' }}
      </button>
    </div>

    <div v-if="loading" class="space-y-2">
      <Skeleton class="h-10 w-full" />
      <Skeleton class="h-10 w-full" />
      <Skeleton class="h-10 w-full" />
    </div>

    <div v-else-if="list.length === 0" class="text-center py-12 text-gray-500 text-sm">
      暂无任务
    </div>

    <div v-else class="space-y-2">
      <p class="text-xs text-gray-500">共 {{ total }} 条任务</p>
      <div
        v-for="task in list"
        :key="task.id"
        class="flex items-center gap-4 p-3 border border-gray-200 rounded-lg"
      >
        <div
          class="w-2 h-2 rounded-full flex-shrink-0"
          :style="{ background: statusColor[task.status] ?? '#6b7280' }"
        />
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-gray-800">#{{ task.id }}</span>
            <span
              class="text-xs px-1.5 py-0.5 rounded"
              :style="{ color: statusColor[task.status] ?? '#6b7280', background: `${statusColor[task.status] ?? '#6b7280'}15` }"
            >
              {{ statusLabel[task.status] ?? task.status }}
            </span>
            <span v-if="task.worker_id" class="text-xs text-gray-400">{{ task.worker_id }}</span>
          </div>
          <p class="text-xs text-gray-500 truncate">{{ task.asset_name || `资产 #${task.asset_id}` }}</p>
        </div>
        <div class="flex flex-col items-end gap-1">
          <div class="flex items-center gap-2">
            <span v-if="task.status === 'running'" class="text-xs text-blue-600">
              {{ task.progress ?? 0 }}%
            </span>
            <span class="text-xs text-gray-400">{{ formatTime(task.created_at) }}</span>
          </div>
          <div
            v-if="task.status === 'running' || task.status === 'pending'"
            class="w-24 h-1.5 bg-gray-200 rounded-full overflow-hidden"
          >
            <div
              class="h-full bg-blue-500 rounded-full transition-all"
              :style="{ width: `${task.progress ?? 0}%` }"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
