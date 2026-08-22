<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { getAvailabilityList, type AvailabilityItem, type AvailabilityStatus } from '../../api/availability'
import { formatTime } from '../../utils/format'
import Skeleton from '../../components/Skeleton.vue'

const list = ref<AvailabilityItem[]>([])
const total = ref(0)
const loading = ref(false)
const keyword = ref('')
const statusFilter = ref('')
const codeGroupFilter = ref('')
const page = reactive({ page: 1, page_size: 20 })

const statusOptions: { value: AvailabilityStatus; label: string }[] = [
  { value: 'normal', label: '正常' },
  { value: 'abnormal', label: '异常' },
  { value: 'unknown', label: '未知' },
]
const codeGroupOptions = ['2xx', '3xx', '4xx', '5xx']

let debounceTimer: number | undefined

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await getAvailabilityList({
      page: page.page,
      page_size: page.page_size,
      keyword: keyword.value || undefined,
      status: statusFilter.value || undefined,
      status_code_group: codeGroupFilter.value || undefined,
    })
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

function toggleStatus(v: string): void {
  statusFilter.value = statusFilter.value === v ? '' : v
  page.page = 1
  void load()
}

function toggleCodeGroup(v: string): void {
  codeGroupFilter.value = codeGroupFilter.value === v ? '' : v
  page.page = 1
  void load()
}

function onKeywordInput(): void {
  window.clearTimeout(debounceTimer)
  debounceTimer = window.setTimeout(() => {
    page.page = 1
    void load()
  }, 300)
}

function statusLabel(s: AvailabilityStatus): string {
  return statusOptions.find((o) => o.value === s)?.label ?? s
}

function statusClass(s: AvailabilityStatus): string {
  return `status-${s}`
}

// 内联 SVG sparkline 折线点串。
function sparklinePoints(data: number[]): string {
  if (!data || data.length < 2) return ''
  const w = 120
  const h = 28
  const pad = 2
  const max = Math.max(...data, 1)
  const min = Math.min(...data, 0)
  const range = max - min || 1
  return data
    .map((v, i) => {
      const x = pad + (i * (w - pad * 2)) / (data.length - 1)
      const y = h - pad - ((v - min) / range) * (h - pad * 2)
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
}

onMounted(() => void load())
</script>

<template>
  <div class="availability-page">
    <div class="toolbar">
      <input v-model="keyword" class="input search" placeholder="搜索站点名 / URL" @input="onKeywordInput" />
      <span class="spacer" />
      <button class="btn" @click="load">刷新</button>
    </div>

    <div class="body">
      <aside class="filters">
        <div class="filter-group">
          <div class="filter-title">可用性状态</div>
          <button
            v-for="o in statusOptions"
            :key="o.value"
            class="chip"
            :class="{ active: statusFilter === o.value }"
            @click="toggleStatus(o.value)"
          >
            {{ o.label }}
          </button>
        </div>
        <div class="filter-group">
          <div class="filter-title">状态码</div>
          <button
            v-for="g in codeGroupOptions"
            :key="g"
            class="chip"
            :class="{ active: codeGroupFilter === g }"
            @click="toggleCodeGroup(g)"
          >
            {{ g }}
          </button>
        </div>
      </aside>

      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>站点名</th>
              <th>URL</th>
              <th>状态</th>
              <th>状态码</th>
              <th>响应耗时</th>
              <th>最后探测时间</th>
              <th>24h 趋势</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in list" :key="item.asset_id">
              <td class="name">{{ item.name || '-' }}</td>
              <td class="mono url">{{ item.url }}</td>
              <td>
                <span class="badge" :class="statusClass(item.availability_status)">
                  {{ statusLabel(item.availability_status) }}
                </span>
              </td>
              <td>{{ item.availability_status === 'unknown' ? '-' : item.status_code }}</td>
              <td>{{ item.availability_status === 'unknown' ? '-' : `${item.response_ms} ms` }}</td>
              <td class="mono">{{ item.sampled_at ? formatTime(item.sampled_at) : '-' }}</td>
              <td>
                <svg v-if="item.sparkline.length >= 2" class="sparkline" viewBox="0 0 120 28" preserveAspectRatio="none">
                  <polyline :points="sparklinePoints(item.sparkline)" fill="none" stroke="var(--color-brand)" stroke-width="1.5" />
                </svg>
                <span v-else class="muted">-</span>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="!loading && list.length === 0" class="empty">暂无可用性监测数据</div>
        <Skeleton v-if="loading" :rows="6" :cols="7" />
      </div>
    </div>

    <div class="pager">
      <span>共 {{ total }} 条</span>
      <button class="btn" :disabled="page.page <= 1" @click="page.page--; load()">上一页</button>
      <span>{{ page.page }}</span>
      <button class="btn" :disabled="page.page * page.page_size >= total" @click="page.page++; load()">下一页</button>
    </div>
  </div>
</template>

<style scoped>
.availability-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.toolbar {
  display: flex;
  gap: 8px;
  align-items: center;
}
.input {
  height: 34px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 0 10px;
  outline: none;
}
.search {
  width: 260px;
}
.spacer {
  flex: 1;
}
.btn {
  height: 34px;
  border: 1px solid var(--color-border);
  background: var(--color-bg-card);
  border-radius: var(--radius-md);
  padding: 0 14px;
  cursor: pointer;
  font-size: 13px;
}
.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.body {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}
.filters {
  flex: none;
  width: 180px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 16px;
  box-shadow: var(--shadow-card);
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.filter-title {
  font-size: 13px;
  font-weight: var(--font-weight-semibold);
  margin-bottom: 10px;
  color: var(--color-text-secondary);
}
.filter-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.chip {
  border: 1px solid var(--color-border);
  background: var(--color-bg-card);
  border-radius: var(--radius-sm);
  padding: 6px 10px;
  font-size: 13px;
  cursor: pointer;
  text-align: left;
  color: var(--color-text-secondary);
}
.chip.active {
  background: var(--color-bg-selected);
  border-color: var(--color-brand-border);
  color: var(--color-brand);
}
.table-wrap {
  flex: 1;
  min-width: 0;
  background: var(--color-bg-card);
  border-radius: var(--radius-md);
  padding: 8px;
  box-shadow: var(--shadow-card);
  min-height: 200px;
}
.table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.table th,
.table td {
  text-align: left;
  padding: 10px;
  border-bottom: 1px solid var(--color-border-light);
}
.name {
  font-weight: var(--font-weight-semibold);
  color: var(--color-text-primary);
}
.url {
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-text-secondary);
}
.mono {
  font-family: var(--font-family-mono);
  font-size: 12px;
}
.badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  font-size: 12px;
}
.status-normal {
  background: var(--color-brand-light);
  color: var(--color-brand);
}
.status-abnormal {
  background: #fff1f0;
  color: var(--color-danger);
}
.status-unknown {
  background: var(--color-bg-hover);
  color: var(--color-text-tertiary);
}
.sparkline {
  width: 120px;
  height: 28px;
  display: block;
}
.muted {
  color: var(--color-text-tertiary);
}
.empty {
  text-align: center;
  color: var(--color-text-tertiary);
  padding: 40px 0;
}
.pager {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
}
</style>
