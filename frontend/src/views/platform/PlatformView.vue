<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listWorkerNodes, getPlatformStats } from '../../api/platform'
import { formatTime, statusLabel } from '../../utils/format'

const workers = ref<Array<{ id: number; name: string; client_id: string; status: string; last_heartbeat_at?: string }>>([])
const stats = ref<Record<string, number> | null>(null)

async function loadWorkers(): Promise<void> {
  try {
    workers.value = await listWorkerNodes({ page: 1, page_size: 100 })
  } catch {
    // 忽略
  }
}

async function loadStats(): Promise<void> {
  try {
    stats.value = (await getPlatformStats()) as Record<string, number>
  } catch {
    // 忽略
  }
}

onMounted(() => {
  void loadWorkers()
  void loadStats()
})
</script>

<template>
  <div class="platform-page">
    <div class="toolbar">
      <h2>平台管理</h2>
    </div>

    <div v-if="stats" class="stats-cards">
      <div class="stat-card"><div class="label">用户数</div><div class="value">{{ stats.users }}</div></div>
      <div class="stat-card"><div class="label">资产数</div><div class="value">{{ stats.assets }}</div></div>
      <div class="stat-card"><div class="label">任务数</div><div class="value">{{ stats.tasks }}</div></div>
      <div class="stat-card"><div class="label">告警数</div><div class="value">{{ stats.alerts }}</div></div>
    </div>

    <h3>Worker 节点</h3>
    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>ID</th>
            <th>名称</th>
            <th>Client ID</th>
            <th>状态</th>
            <th>最近心跳</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="w in workers" :key="w.id">
            <td>{{ w.id }}</td>
            <td>{{ w.name }}</td>
            <td class="mono">{{ w.client_id }}</td>
            <td>{{ statusLabel(w.status) }}</td>
            <td>{{ formatTime(w.last_heartbeat_at) }}</td>
          </tr>
        </tbody>
      </table>
      <div v-if="workers.length === 0" class="empty">暂无 Worker 节点</div>
    </div>
  </div>
</template>

<style scoped>
.platform-page {
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
.stats-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}
.stat-card {
  background: #fff;
  border-radius: 8px;
  padding: 16px 20px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}
.label {
  color: #86909c;
  font-size: 13px;
}
.value {
  font-size: 28px;
  font-weight: 700;
  margin-top: 6px;
}
h3 {
  margin: 0;
}
.table-wrap {
  background: #fff;
  border-radius: 8px;
  padding: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
  min-height: 100px;
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
  border-bottom: 1px solid #f2f3f5;
}
.mono {
  font-family: ui-monospace, monospace;
  font-size: 12px;
}
.empty {
  text-align: center;
  color: #86909c;
  padding: 30px 0;
}
</style>
