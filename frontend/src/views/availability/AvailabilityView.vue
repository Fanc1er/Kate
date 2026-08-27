<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import {
  getAvailabilityList,
  getAvailabilityTimeseries,
  type AvailabilityItem,
  type AvailabilityPoint,
  type AvailabilitySparkPoint,
} from '../../api/availability'
import Skeleton from '../../components/Skeleton.vue'
import EChart from '../../components/EChart.vue'
import { formatTime } from '../../utils/format'
import { useQuerySync } from '../../composables/useQuerySync'
import { reactive, toRef } from 'vue'

const tab = ref<'list' | 'timing'>('list')
const list = ref<AvailabilityItem[]>([])
const total = ref(0)
const loading = ref(false)
const keyword = ref('')
const statusFilter = ref('')
const page = reactive({ page: 1, page_size: 20 })

useQuerySync(
  [
    ['keyword', keyword],
    ['status', statusFilter],
    ['page', toRef(page, 'page')],
    ['page_size', toRef(page, 'page_size')],
  ],
  { numberKeys: ['page', 'page_size'], defaults: { page: 1, page_size: 20 } },
)

const detail = ref<AvailabilityItem | null>(null)
const timingData = ref<AvailabilityPoint[]>([])
const timingLoading = ref(false)

const statusColor: Record<string, string> = {
  normal: '#22c55e',
  abnormal: '#ef4444',
  unknown: '#9ca3af',
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await getAvailabilityList({
      keyword: keyword.value,
      status: statusFilter.value || undefined,
      page: page.page,
      page_size: page.page_size,
    })
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

async function openDetail(item: AvailabilityItem): Promise<void> {
  detail.value = item
  tab.value = 'timing'
  timingLoading.value = true
  try {
    timingData.value = await getAvailabilityTimeseries(item.asset_id, 24)
  } finally {
    timingLoading.value = false
  }
}

// 迷你趋势：高度按 24h 内最大耗时相对缩放，颜色与右侧图例阈值一致。
const MS_GREEN = 500
const MS_YELLOW = 1000

function sparkBars(points: AvailabilitySparkPoint[]): { height: number; color: string }[] {
  const win = points.slice(-12)
  if (!win.length) return []
  let maxV = 1
  for (const p of win) maxV = Math.max(maxV, p.response_ms)
  return win.map((p) => {
    const color = !p.ok || p.response_ms > MS_YELLOW ? '#ef4444' : p.response_ms > MS_GREEN ? '#eab308' : '#22c55e'
    return { height: Math.max(4, Math.round((p.response_ms / maxV) * 20)), color }
  })
}

const chartData = computed(() => {
  if (!timingData.value.length) return null
  const labels = timingData.value.map((p) => formatTime(p.sampled_at))
  const responseMs = timingData.value.map((p) => p.response_ms)
  const statusColors = timingData.value.map((p) =>
    p.status_code === 0 || p.status_code >= 500
      ? '#ef4444'
      : p.status_code >= 400
        ? '#f97316'
        : '#22c55e',
  )
  return { labels, responseMs, statusColors }
})

onMounted(() => {
  void load()
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center gap-3">
      <h2 class="text-lg font-semibold text-gray-800">可用性网络</h2>
      <div class="flex gap-1 bg-gray-100 rounded-lg p-0.5">
        <button
          :class="[
            'px-3 py-1 text-xs rounded-md transition-colors',
            tab === 'list' ? 'bg-white shadow text-gray-800' : 'text-gray-500 hover:text-gray-700',
          ]"
          @click="tab = 'list'"
        >
          列表
        </button>
        <button
          :class="[
            'px-3 py-1 text-xs rounded-md transition-colors',
            tab === 'timing' ? 'bg-white shadow text-gray-800' : 'text-gray-500 hover:text-gray-700',
          ]"
          @click="tab = 'timing'"
        >
          时序网络
        </button>
      </div>
    </div>

    <div v-if="tab === 'list'">
      <div class="flex flex-wrap items-center gap-2 mb-3">
        <input
          v-model="keyword"
          placeholder="搜索名称 / URL"
          class="h-8 px-3 text-sm border border-gray-300 rounded-md w-56 focus:outline-none focus:border-blue-400"
          @keydown.enter="page.page = 1; load()"
        />
        <select
          v-model="statusFilter"
          class="h-8 px-2 text-sm border border-gray-300 rounded-md bg-white focus:outline-none focus:border-blue-400"
          @change="page.page = 1; load()"
        >
          <option value="">全部状态</option>
          <option value="normal">正常</option>
          <option value="abnormal">异常</option>
          <option value="unknown">未知</option>
        </select>
      </div>
      <div v-if="loading" class="space-y-2">
        <Skeleton class="h-10 w-full" />
        <Skeleton class="h-10 w-full" />
        <Skeleton class="h-10 w-full" />
      </div>
      <div v-else-if="list.length === 0" class="text-center py-12 text-gray-500 text-sm">暂无数据</div>
      <div v-else class="space-y-2">
        <div
          v-for="item in list"
          :key="item.asset_id"
          @click="openDetail(item)"
          class="flex items-center gap-4 p-3 border border-gray-200 rounded-lg hover:bg-gray-50 cursor-pointer"
        >
          <div
            class="w-2.5 h-2.5 rounded-full flex-shrink-0"
            :style="{ background: statusColor[item.availability_status] ?? '#9ca3af' }"
          />
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium text-gray-800 truncate">{{ item.name }}</p>
            <p class="text-xs text-gray-500 truncate">{{ item.url }}</p>
          </div>
          <div class="text-right flex-shrink-0">
            <p class="text-sm text-gray-700">{{ item.status_code }}</p>
            <p class="text-xs text-gray-400">{{ item.response_ms }}ms</p>
          </div>
          <div class="flex-shrink-0">
            <div v-if="sparkBars(item.sparkline).length" class="flex items-end gap-0.5 h-6">
              <div
                v-for="(b, i) in sparkBars(item.sparkline)"
                :key="i"
                class="w-1 rounded-sm bg-gray-200"
                :style="{ height: `${b.height}px`, background: b.color }"
              />
            </div>
          </div>
          <span class="text-xs text-gray-400 flex-shrink-0">{{ formatTime(item.sampled_at ?? '') }}</span>
        </div>
        <div class="flex items-center justify-between pt-2">
          <span class="text-xs text-gray-500">共 {{ total }} 条</span>
          <div class="flex gap-1">
            <button
              :disabled="page.page <= 1"
              class="px-2 py-1 text-xs border border-gray-300 rounded disabled:opacity-40"
              @click="page.page--"
            >
              上一页
            </button>
            <button
              :disabled="page.page * page.page_size >= total"
              class="px-2 py-1 text-xs border border-gray-300 rounded disabled:opacity-40"
              @click="page.page++"
            >
              下一页
            </button>
          </div>
        </div>
      </div>
    </div>

    <div v-else-if="tab === 'timing'">
      <div v-if="!detail" class="text-center py-12 text-gray-500 text-sm">
        请从列表选择一个资产查看时序网络
      </div>
      <div v-else class="space-y-4">
        <div class="flex items-center gap-3">
          <button
            class="px-3 py-1.5 text-sm border border-gray-300 rounded-lg hover:bg-gray-50"
            @click="tab = 'list'"
          >
            返回列表
          </button>
          <span class="text-sm font-medium text-gray-800">{{ detail.name }}</span>
          <span class="text-xs text-gray-400">{{ detail.url }}</span>
        </div>

        <div v-if="timingLoading" class="py-8">
          <Skeleton class="h-64 w-full" />
        </div>
        <div v-else-if="!timingData.length" class="text-center py-8 text-gray-500 text-sm">
          暂无时序数据
        </div>
        <div v-if="chartData">
          <EChart
            :option="{
              tooltip: { trigger: 'axis' },
              legend: { data: ['响应时间 (ms)'] },
              grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
              xAxis: { type: 'category', data: chartData.labels, axisLabel: { fontSize: 10 } },
              yAxis: { type: 'value', name: 'ms' },
              visualMap: {
                show: false,
                pieces: [
                  { gt: 0, lt: 500, color: '#22c55e' },
                  { gt: 500, lt: 1000, color: '#eab308' },
                  { gte: 1000, color: '#ef4444' },
                ],
                outOfRange: { color: '#9ca3af' },
              },
              series: [
                {
                  name: '响应时间 (ms)',
                  type: 'line',
                  smooth: true,
                  symbol: 'circle',
                  symbolSize: 6,
                  lineStyle: { width: 2 },
                  itemStyle: { color: (params: any) => (chartData as NonNullable<typeof chartData>).statusColors[params.dataIndex] },
                  areaStyle: {
                    color: {
                      type: 'linear',
                      x: 0, y: 0, x2: 0, y2: 1,
                      colorStops: [
                        { offset: 0, color: 'rgba(59,130,246,0.2)' },
                        { offset: 1, color: 'rgba(59,130,246,0)' },
                      ],
                    },
                  },
                  data: chartData.responseMs,
                },
              ],
            }"
            style="height: 300px"
          />
          <div class="mt-2 flex gap-4 text-xs text-gray-500">
            <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full bg-green-500" /> &lt; 500ms 正常</span>
            <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full bg-yellow-500" /> 500-1000ms 警告</span>
            <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full bg-red-500" /> &gt; 1000ms 异常</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
