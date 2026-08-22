<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { getWorkerTopology, type WorkerTopology } from '../../api/availability'
import EChart from '../../components/EChart.vue'
import { formatTime } from '../../utils/format'

const topology = ref<WorkerTopology | null>(null)
const loading = ref(false)

const option = computed<Record<string, unknown>>(() => {
  const t = topology.value
  if (!t) return {}
  const nodes = [
    {
      id: 'master',
      name: t.master.name,
      category: 0,
      symbolSize: 56,
      label: { show: true },
    },
    ...t.workers.map((w) => ({
      id: `w${w.id}`,
      name: w.name,
      category: 1,
      symbolSize: 36,
      label: { show: true },
      itemStyle: w.status === 'online' ? {} : { color: '#d9d9d9' },
    })),
  ]
  const links = t.workers.map((w) => ({ source: 'master', target: `w${w.id}` }))
  return {
    tooltip: { trigger: 'item' },
    legend: [{ data: ['Master', 'Worker'], bottom: 0 }],
    series: [
      {
        type: 'graph',
        layout: 'force',
        roam: true,
        draggable: true,
        categories: [{ name: 'Master' }, { name: 'Worker' }],
        data: nodes,
        links,
        force: { repulsion: 240, edgeLength: 130 },
        label: { show: true, position: 'bottom', fontSize: 12 },
        emphasis: { focus: 'adjacency' },
        lineStyle: { color: 'source', curveness: 0.1 },
      },
    ],
  }
})

async function load(): Promise<void> {
  loading.value = true
  try {
    topology.value = await getWorkerTopology()
  } finally {
    loading.value = false
  }
}

onMounted(() => void load())
</script>

<template>
  <div class="topology-page">
    <div class="toolbar">
      <span class="title">工作节点拓扑</span>
      <span class="spacer" />
      <button class="btn" @click="load">刷新</button>
    </div>

    <div v-if="loading" class="card loading">加载中…</div>
    <div v-else-if="!topology || topology.workers.length === 0" class="card empty">
      暂无工作节点在线
    </div>
    <template v-else>
      <div class="card graph-card">
        <EChart :option="option" height="360px" />
      </div>

      <div class="card">
        <table class="table">
          <thead>
            <tr>
              <th>节点</th>
              <th>IP</th>
              <th>版本</th>
              <th>状态</th>
              <th>负载</th>
              <th>最近心跳</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="w in topology.workers" :key="w.id">
              <td class="name">{{ w.name }}</td>
              <td class="mono">{{ w.ip }}</td>
              <td class="mono">{{ w.version }}</td>
              <td>
                <span class="badge" :class="w.status === 'online' ? 'status-online' : 'status-offline'">
                  {{ w.status === 'online' ? '在线' : '离线' }}
                </span>
              </td>
              <td>{{ w.load }}</td>
              <td class="mono">{{ w.heartbeat_at ? formatTime(w.heartbeat_at) : '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </div>
</template>

<style scoped>
.topology-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
}
.title {
  font-size: 15px;
  font-weight: var(--font-weight-semibold);
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
.card {
  background: var(--color-bg-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-card);
  padding: 16px;
}
.loading,
.empty {
  color: var(--color-text-tertiary);
  text-align: center;
  padding: 40px 0;
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
.status-online {
  background: var(--color-brand-light);
  color: var(--color-brand);
}
.status-offline {
  background: var(--color-bg-hover);
  color: var(--color-text-tertiary);
}
</style>
