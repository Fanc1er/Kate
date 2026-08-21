<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getStats, getTopRisks, type TopRisk } from '../../api/dashboard'
import { formatTime } from '../../utils/format'
import { downloadBlob } from '../../utils/format'
import { listFindings } from '../../api/event'
import { toast } from '../../utils/toast'

const stats = ref<Record<string, number> | null>(null)
const topRisks = ref<TopRisk[]>([])
const generatedAt = ref('')
const generating = ref(false)

async function generate(): Promise<void> {
  generating.value = true
  try {
    const [s, r] = await Promise.all([getStats(), getTopRisks(10)])
    stats.value = s as unknown as Record<string, number>
    topRisks.value = r
    generatedAt.value = formatTime(new Date())
  } finally {
    generating.value = false
  }
}

async function exportCsv(): Promise<void> {
  try {
    const res = await listFindings({ page: 1, page_size: 200 })
    const rows = res.list.map((f) => [f.id, f.severity, f.title, f.url, f.status, f.risk_score].join(','))
    const csv = ['id,severity,title,url,status,risk_score', ...rows].join('\n')
    downloadBlob(new Blob([csv], { type: 'text/csv' }), `findings-${Date.now()}.csv`)
    toast.success('CSV 导出成功')
  } catch {
    // 拦截器已提示
  }
}

onMounted(() => void generate())
</script>

<template>
  <div class="report-page">
    <div class="toolbar">
      <h2>安全监测报告</h2>
      <span class="spacer" />
      <button class="btn" @click="generate">重新生成</button>
      <button class="btn primary" @click="exportCsv">导出漏洞清单 CSV</button>
    </div>

    <div v-if="stats" class="report">
      <p class="meta">生成时间：{{ generatedAt }}</p>
      <h3>资产与发现概览</h3>
      <table class="table">
        <tbody>
          <tr><td>资产总数</td><td>{{ stats.assets }}</td></tr>
          <tr><td>发现总数</td><td>{{ stats.findings }}</td></tr>
          <tr><td>今日事件</td><td>{{ stats.events_today }}</td></tr>
          <tr><td>未处理告警</td><td>{{ stats.alerts_open }}</td></tr>
          <tr><td>严重</td><td>{{ stats.critical }}</td></tr>
          <tr><td>高危</td><td>{{ stats.high }}</td></tr>
          <tr><td>引擎覆盖率</td><td>{{ stats.coverage }}%</td></tr>
        </tbody>
      </table>

      <h3>风险 Top10</h3>
      <table class="table">
        <thead>
          <tr>
            <th>#</th>
            <th>标题</th>
            <th>等级</th>
            <th>风险分</th>
            <th>资产</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(r, i) in topRisks" :key="i">
            <td>{{ i + 1 }}</td>
            <td>{{ r.title }}</td>
            <td>{{ r.severity }}</td>
            <td>{{ r.risk_score }}</td>
            <td class="mono">{{ r.url }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p v-else-if="generating">生成中…</p>
  </div>
</template>

<style scoped>
.report-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.toolbar {
  display: flex;
  align-items: center;
}
.toolbar h2 {
  margin: 0;
}
.spacer {
  flex: 1;
}
.btn {
  height: 34px;
  border: 1px solid #e5e6eb;
  background: #fff;
  border-radius: 6px;
  padding: 0 14px;
  cursor: pointer;
  font-size: 13px;
  margin-left: 8px;
}
.btn.primary {
  background: #3370ff;
  color: #fff;
  border-color: #3370ff;
}
.report {
  background: #fff;
  border-radius: 8px;
  padding: 24px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}
.meta {
  color: #86909c;
  font-size: 12px;
}
.table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  margin-bottom: 24px;
}
.table th,
.table td {
  text-align: left;
  padding: 8px 10px;
  border-bottom: 1px solid #f2f3f5;
}
.mono {
  font-family: ui-monospace, monospace;
  font-size: 12px;
}
</style>
