<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import EChart from '../../components/EChart.vue'
import { getStats, getTrends, getTopRisks, getEngineCoverage } from '../../api/dashboard'
import type { DashboardStats, TopRisk } from '../../api/dashboard'
import { eventStream } from '../../api/ws'
import { formatTime } from '../../utils/format'

const stats = ref<DashboardStats | null>(null)
const topRisks = ref<TopRisk[]>([])
const trendOption = ref<Record<string, unknown>>({})
const radarOption = ref<Record<string, unknown>>({})
const loading = ref(true)

const cards = ref([
  { key: 'assets', label: '资产', color: '#3370ff' },
  { key: 'findings', label: '发现', color: '#ff7d00' },
  { key: 'events_today', label: '今日事件', color: '#00b42a' },
  { key: 'alerts_open', label: '未处理告警', color: '#d03050' },
])

async function load(): Promise<void> {
  loading.value = true
  try {
    const [s, t, risks, cov] = await Promise.all([
      getStats(),
      getTrends(7),
      getTopRisks(10),
      getEngineCoverage(),
    ])
    stats.value = s
    topRisks.value = risks
    trendOption.value = {
      tooltip: { trigger: 'axis' },
      legend: { data: ['发现', '告警'] },
      grid: { left: 40, right: 20, top: 40, bottom: 30 },
      xAxis: { type: 'category', data: t.dates },
      yAxis: { type: 'value', minInterval: 1 },
      series: [
        { name: '发现', type: 'line', smooth: true, data: t.findings, itemStyle: { color: '#3370ff' } },
        { name: '告警', type: 'line', smooth: true, data: t.alerts, itemStyle: { color: '#d03050' } },
      ],
    }
    const engineNames = Object.keys(cov)
    radarOption.value = {
      tooltip: {},
      radar: {
        indicator: engineNames.map((n) => ({ name: n, max: 100 })),
        radius: '60%',
      },
      series: [
        {
          type: 'radar',
          data: [{ value: engineNames.map((n) => cov[n]), name: '引擎覆盖率' }],
          areaStyle: { opacity: 0.2 },
        },
      ],
    }
  } finally {
    loading.value = false
  }
}

let unsub: (() => void) | null = null

onMounted(async () => {
  await load()
  unsub = eventStream.subscribe(() => {
    void load()
  })
})

onBeforeUnmount(() => {
  unsub?.()
})
</script>

<template>
  <div class="dashboard">
    <div class="cards">
      <div v-for="c in cards" :key="c.key" class="card">
        <div class="card-label">{{ c.label }}</div>
        <div class="card-value" :style="{ color: c.color }">
          {{ stats ? String((stats as Record<string, number>)[c.key] ?? 0) : '–' }}
        </div>
      </div>
    </div>

    <div class="row">
      <div class="panel">
        <h3>7 天趋势</h3>
        <EChart v-if="trendOption.xAxis" :option="trendOption" height="300px" />
      </div>
      <div class="panel">
        <h3>引擎覆盖率</h3>
        <EChart v-if="radarOption.radar" :option="radarOption" height="300px" />
      </div>
    </div>

    <div class="panel">
      <h3>风险 Top10</h3>
      <table class="risk-table">
        <thead>
          <tr>
            <th>#</th>
            <th>标题</th>
            <th>等级</th>
            <th>风险分</th>
            <th>资产</th>
            <th>时间</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(r, i) in topRisks" :key="i">
            <td>{{ i + 1 }}</td>
            <td>{{ r.title }}</td>
            <td>{{ r.severity }}</td>
            <td>{{ r.risk_score }}</td>
            <td class="mono">{{ r.url }}</td>
            <td>{{ formatTime(r.created_at) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}
.card {
  background: #fff;
  border-radius: 8px;
  padding: 20px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}
.card-label {
  color: #86909c;
  font-size: 13px;
}
.card-value {
  font-size: 32px;
  font-weight: 700;
  margin-top: 8px;
}
.row {
  display: grid;
  grid-template-columns: 3fr 2fr;
  gap: 16px;
}
.panel {
  background: #fff;
  border-radius: 8px;
  padding: 16px 20px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}
.panel h3 {
  margin: 0 0 12px;
  font-size: 15px;
}
.risk-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.risk-table th,
.risk-table td {
  text-align: left;
  padding: 8px;
  border-bottom: 1px solid #f2f3f5;
}
.mono {
  font-family: ui-monospace, monospace;
  font-size: 12px;
  color: #4e5969;
}
</style>
