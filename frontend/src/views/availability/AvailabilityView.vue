<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import {
  getAvailabilityList,
  getAvailabilityTimeseries,
  reprobe,
  getWhitelist,
  addWhitelist,
  removeWhitelist,
  type AvailabilityItem,
  type AvailabilityStatus,
  type AvailabilityPoint,
  type WhitelistRule,
} from '../../api/availability'
import { formatTime } from '../../utils/format'
import Skeleton from '../../components/Skeleton.vue'
import EChart from '../../components/EChart.vue'
import WorkerTopology from './WorkerTopology.vue'

const tab = ref<'list' | 'topology'>('list')

const list = ref<AvailabilityItem[]>([])
const total = ref(0)
const loading = ref(false)
const keyword = ref('')
const statusFilter = ref('')
const codeGroupFilter = ref('')
const page = reactive({ page: 1, page_size: 20 })

const selected = ref<number[]>([])

const toast = ref('')
let toastTimer: number | undefined

const statusOptions: { value: AvailabilityStatus; label: string }[] = [
  { value: 'normal', label: '正常' },
  { value: 'abnormal', label: '异常' },
  { value: 'unknown', label: '未知' },
]
const codeGroupOptions = ['2xx', '3xx', '4xx', '5xx']

const detailOpen = ref(false)
const detailItem = ref<AvailabilityItem | null>(null)
const detailPoints = ref<AvailabilityPoint[]>([])
const detailLoading = ref(false)

const whitelistOpen = ref(false)
const whitelistRules = ref<WhitelistRule[]>([])
const whitelistLoading = ref(false)
const wlForm = reactive({ kind: 'domain', value: '', remark: '' })

let debounceTimer: number | undefined

function showToast(msg: string): void {
  toast.value = msg
  window.clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => {
    toast.value = ''
  }, 2500)
}

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

function toggleSelect(id: number): void {
  const i = selected.value.indexOf(id)
  if (i >= 0) selected.value.splice(i, 1)
  else selected.value.push(id)
}

const allSelected = computed(
  () => list.value.length > 0 && list.value.every((i) => selected.value.includes(i.asset_id)),
)

function toggleSelectAll(): void {
  if (allSelected.value) {
    selected.value = selected.value.filter((id) => !list.value.some((i) => i.asset_id === id))
  } else {
    for (const i of list.value) {
      if (!selected.value.includes(i.asset_id)) selected.value.push(i.asset_id)
    }
  }
}

function clearSelection(): void {
  selected.value = []
}

async function batchReprobe(): Promise<void> {
  if (selected.value.length === 0) return
  await reprobe(selected.value)
  showToast(`已下发 ${selected.value.length} 个重新探测任务`)
  clearSelection()
}

async function batchWhitelist(): Promise<void> {
  if (selected.value.length === 0) return
  const targets = list.value.filter((i) => selected.value.includes(i.asset_id))
  let n = 0
  for (const t of targets) {
    const host = hostOf(t.url)
    if (!host) continue
    await addWhitelist('domain', host, '')
    n += 1
  }
  showToast(`已加入 ${n} 条白名单规则`)
  clearSelection()
}

function hostOf(url: string): string {
  try {
    return new URL(url).hostname
  } catch {
    return url
  }
}

function openDetail(item: AvailabilityItem): void {
  detailItem.value = item
  detailOpen.value = true
  detailPoints.value = []
  void loadTimeseries(item.asset_id)
}

function closeDetail(): void {
  detailOpen.value = false
  detailItem.value = null
}

async function loadTimeseries(assetId: number): Promise<void> {
  detailLoading.value = true
  try {
    detailPoints.value = await getAvailabilityTimeseries(assetId, 24)
  } finally {
    detailLoading.value = false
  }
}

const detailChartOption = computed<Record<string, unknown>>(() => {
  const pts = detailPoints.value
  if (!pts.length) return {}
  return {
    tooltip: { trigger: 'axis' },
    grid: { left: 48, right: 20, top: 20, bottom: 40 },
    xAxis: {
      type: 'category',
      data: pts.map((p) => formatTime(p.sampled_at)),
      axisLabel: { fontSize: 11 },
    },
    yAxis: { type: 'value', name: 'ms' },
    series: [
      {
        type: 'line',
        name: '响应耗时',
        data: pts.map((p) => p.response_ms),
        smooth: true,
        showSymbol: false,
        areaStyle: { opacity: 0.08 },
      },
    ],
  }
})

async function reprobeRow(item: AvailabilityItem): Promise<void> {
  await reprobe([item.asset_id])
  showToast(`已下发 ${item.name || item.url} 重新探测任务`)
}

function openWhitelistForm(item?: AvailabilityItem): void {
  wlForm.kind = 'domain'
  wlForm.value = item ? hostOf(item.url) : ''
  wlForm.remark = ''
  whitelistOpen.value = true
  void loadWhitelist()
}

async function loadWhitelist(): Promise<void> {
  whitelistLoading.value = true
  try {
    whitelistRules.value = await getWhitelist()
  } finally {
    whitelistLoading.value = false
  }
}

async function submitWhitelist(): Promise<void> {
  if (!wlForm.value.trim()) return
  await addWhitelist(wlForm.kind, wlForm.value.trim(), wlForm.remark.trim())
  showToast('已加入白名单')
  wlForm.value = ''
  wlForm.remark = ''
  void loadWhitelist()
}

async function deleteWhitelist(rule: WhitelistRule): Promise<void> {
  await removeWhitelist(rule.id)
  showToast('已删除白名单规则')
  void loadWhitelist()
}

onMounted(() => void load())
</script>

<template>
  <div class="availability-page">
    <div class="tabs">
      <button class="tab" :class="{ active: tab === 'list' }" @click="tab = 'list'">站点可用性</button>
      <button class="tab" :class="{ active: tab === 'topology' }" @click="tab = 'topology'">工作节点拓扑</button>
    </div>

    <WorkerTopology v-if="tab === 'topology'" />

    <template v-else>
      <div class="toolbar">
        <input v-model="keyword" class="input search" placeholder="搜索站点名 / URL" @input="onKeywordInput" />
        <span class="spacer" />
        <button class="btn" @click="openWhitelistForm()">白名单管理</button>
        <button class="btn" @click="load">刷新</button>
      </div>

      <div v-if="selected.length" class="batch-bar">
        <span>已选 {{ selected.length }} 项</span>
        <button class="btn primary" @click="batchReprobe">批量重新探测</button>
        <button class="btn" @click="batchWhitelist">批量加入白名单</button>
        <button class="btn link" @click="clearSelection">取消选择</button>
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
                <th class="check-col">
                  <input type="checkbox" :checked="allSelected" @change="toggleSelectAll" />
                </th>
                <th>站点名</th>
                <th>URL</th>
                <th>状态</th>
                <th>状态码</th>
                <th>响应耗时</th>
                <th>最后探测时间</th>
                <th>24h 趋势</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in list" :key="item.asset_id">
                <td class="check-col">
                  <input
                    type="checkbox"
                    :checked="selected.includes(item.asset_id)"
                    @change="toggleSelect(item.asset_id)"
                  />
                </td>
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
                <td>
                  <div class="row-actions">
                    <button class="btn-mini" @click="openDetail(item)">详情</button>
                    <button class="btn-mini" @click="reprobeRow(item)">重新探测</button>
                    <button class="btn-mini" @click="openWhitelistForm(item)">加入白名单</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
          <div v-if="!loading && list.length === 0" class="empty">暂无可用性监测数据</div>
          <Skeleton v-if="loading" :rows="6" :cols="8" />
        </div>
      </div>

      <div class="pager">
        <span>共 {{ total }} 条</span>
        <button class="btn" :disabled="page.page <= 1" @click="page.page--; load()">上一页</button>
        <span>{{ page.page }}</span>
        <button class="btn" :disabled="page.page * page.page_size >= total" @click="page.page++; load()">下一页</button>
      </div>
    </template>

    <div v-if="detailOpen && detailItem" class="drawer-mask" @click.self="closeDetail">
      <div class="drawer">
        <div class="drawer-head">
          <div>
            <div class="drawer-title">{{ detailItem.name || '-' }}</div>
            <div class="drawer-url mono">{{ detailItem.url }}</div>
          </div>
          <button class="btn-mini" @click="closeDetail">关闭</button>
        </div>
        <div class="drawer-meta">
          <span class="badge" :class="statusClass(detailItem.availability_status)">
            {{ statusLabel(detailItem.availability_status) }}
          </span>
          <span>状态码 {{ detailItem.availability_status === 'unknown' ? '-' : detailItem.status_code }}</span>
          <span>{{ detailItem.availability_status === 'unknown' ? '-' : `${detailItem.response_ms} ms` }}</span>
          <span>{{ detailItem.sampled_at ? formatTime(detailItem.sampled_at) : '-' }}</span>
        </div>
        <div class="drawer-actions">
          <button class="btn primary" @click="reprobeRow(detailItem)">重新探测</button>
          <button class="btn" @click="openWhitelistForm(detailItem)">加入白名单</button>
        </div>
        <div class="drawer-chart">
          <div v-if="detailLoading" class="muted">加载时序中…</div>
          <template v-else>
            <div v-if="detailPoints.length === 0" class="muted">暂无 24h 时序数据</div>
            <EChart v-else :option="detailChartOption" height="220px" />
          </template>
        </div>
      </div>
    </div>

    <div v-if="whitelistOpen" class="modal-mask" @click.self="whitelistOpen = false">
      <div class="modal">
        <div class="modal-head">
          <span class="modal-title">白名单管理</span>
          <button class="btn-mini" @click="whitelistOpen = false">关闭</button>
        </div>
        <div class="wl-form">
          <select v-model="wlForm.kind" class="input">
            <option value="domain">domain</option>
            <option value="ip">ip</option>
            <option value="cidr">cidr</option>
          </select>
          <input v-model="wlForm.value" class="input" placeholder="规则值（域名 / IP / CIDR）" />
          <input v-model="wlForm.remark" class="input" placeholder="备注（可选）" />
          <button class="btn primary" :disabled="!wlForm.value.trim()" @click="submitWhitelist">添加</button>
        </div>
        <div class="wl-list">
          <div v-if="whitelistLoading" class="muted">加载中…</div>
          <div v-else-if="whitelistRules.length === 0" class="muted">暂无白名单规则</div>
          <div v-for="r in whitelistRules" :key="r.id" class="wl-row">
            <span class="wl-kind">{{ r.kind }}</span>
            <span class="wl-value mono">{{ r.value }}</span>
            <span class="wl-remark">{{ r.remark }}</span>
            <button class="btn-mini danger" @click="deleteWhitelist(r)">删除</button>
          </div>
        </div>
      </div>
    </div>

    <div v-if="toast" class="toast">{{ toast }}</div>
  </div>
</template>

<style scoped>
.availability-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.tabs {
  display: flex;
  gap: 4px;
  border-bottom: 1px solid var(--color-border);
}
.tab {
  border: none;
  background: transparent;
  padding: 8px 16px;
  cursor: pointer;
  font-size: 14px;
  color: var(--color-text-secondary);
  border-bottom: 2px solid transparent;
}
.tab.active {
  color: var(--color-brand);
  border-bottom-color: var(--color-brand);
  font-weight: var(--font-weight-semibold);
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
  font-size: 13px;
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
.btn.primary {
  background: var(--color-brand);
  border-color: var(--color-brand);
  color: #fff;
}
.btn.link {
  border: none;
  color: var(--color-brand);
}
.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.batch-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--color-brand-light);
  border: 1px solid var(--color-brand-border);
  border-radius: var(--radius-md);
  padding: 8px 12px;
  font-size: 13px;
  color: var(--color-brand);
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
.check-col {
  width: 32px;
}
.name {
  font-weight: var(--font-weight-semibold);
  color: var(--color-text-primary);
}
.url {
  max-width: 180px;
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
.row-actions {
  display: flex;
  gap: 6px;
}
.btn-mini {
  border: 1px solid var(--color-border);
  background: var(--color-bg-card);
  border-radius: var(--radius-sm);
  padding: 3px 8px;
  cursor: pointer;
  font-size: 12px;
  color: var(--color-text-secondary);
}
.btn-mini:hover {
  color: var(--color-brand);
  border-color: var(--color-brand-border);
}
.btn-mini.danger {
  color: var(--color-danger);
}
.drawer-mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  z-index: 1000;
}
.drawer {
  position: absolute;
  top: 0;
  right: 0;
  width: 480px;
  max-width: 92vw;
  height: 100%;
  background: var(--color-bg-card);
  box-shadow: -2px 0 8px rgba(0, 0, 0, 0.08);
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
}
.drawer-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}
.drawer-title {
  font-size: 16px;
  font-weight: var(--font-weight-semibold);
}
.drawer-url {
  color: var(--color-text-secondary);
  margin-top: 4px;
  word-break: break-all;
}
.drawer-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  font-size: 13px;
  color: var(--color-text-secondary);
}
.drawer-actions {
  display: flex;
  gap: 8px;
}
.drawer-chart {
  flex: 1;
  min-height: 220px;
}
.modal-mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1100;
}
.modal {
  width: 560px;
  max-width: 92vw;
  max-height: 80vh;
  background: var(--color-bg-card);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-card);
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.modal-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.modal-title {
  font-size: 16px;
  font-weight: var(--font-weight-semibold);
}
.wl-form {
  display: flex;
  gap: 8px;
}
.wl-form .input {
  flex: 1;
  min-width: 0;
}
.wl-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 320px;
  overflow-y: auto;
}
.wl-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-sm);
  font-size: 13px;
}
.wl-kind {
  flex: none;
  background: var(--color-bg-hover);
  border-radius: var(--radius-sm);
  padding: 2px 6px;
  font-size: 12px;
  color: var(--color-text-secondary);
}
.wl-value {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.wl-remark {
  flex: none;
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-text-tertiary);
}
.toast {
  position: fixed;
  top: 72px;
  left: 50%;
  transform: translateX(-50%);
  background: var(--color-text-primary);
  color: #fff;
  border-radius: var(--radius-md);
  padding: 10px 18px;
  font-size: 13px;
  z-index: 1200;
}
</style>
