<script setup lang="ts">
import { ref, reactive, onMounted, toRef } from 'vue'
import { listAlerts, resolveAlert, type AlertItem } from '../../api/event'
import { formatTime, severityLabel, statusLabel } from '../../utils/format'
import { toast } from '../../utils/toast'
import FilterPanel from '../../components/FilterPanel.vue'
import { useQuerySync } from '../../composables/useQuerySync'

const list = ref<AlertItem[]>([])
const total = ref(0)
const loading = ref(false)
const severity = ref('')
const status = ref('')
const page = reactive({ page: 1, page_size: 20 })

useQuerySync(
  [
    ['severity', severity],
    ['status', status],
    ['page', toRef(page, 'page')],
    ['page_size', toRef(page, 'page_size')],
  ],
  { numberKeys: ['page', 'page_size'], defaults: { page: 1, page_size: 20 } },
)

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listAlerts({
      page: page.page,
      page_size: page.page_size,
      severity: severity.value || undefined,
      status: status.value || undefined,
    })
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

async function doResolve(a: AlertItem): Promise<void> {
  try {
    await resolveAlert(a.id)
    toast.success('告警已处置')
    await load()
  } catch {
    // 拦截器已提示
  }
}

function clearFilters(): void {
  severity.value = ''
  status.value = ''
  page.page = 1
  void load()
}

onMounted(load)
</script>

<template>
  <div class="list-page alert-page">
    <FilterPanel clearable @clear="clearFilters">
      <div class="filter-group">
        <div class="filter-label">等级</div>
        <select v-model="severity" class="filter-select" @change="page.page = 1; load()">
          <option value="">全部等级</option>
          <option value="critical">严重</option>
          <option value="high">高危</option>
          <option value="medium">中危</option>
          <option value="low">低危</option>
        </select>
      </div>
      <div class="filter-group">
        <div class="filter-label">状态</div>
        <select v-model="status" class="filter-select" @change="page.page = 1; load()">
          <option value="">全部状态</option>
          <option value="open">待处理</option>
          <option value="resolved">已解决</option>
          <option value="silenced">已静默</option>
        </select>
      </div>
    </FilterPanel>

    <section class="list-main">
      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>等级</th>
              <th>类型</th>
              <th>标题</th>
              <th>内容</th>
              <th>状态</th>
              <th>时间</th>
              <th style="width: 100px">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="a in list" :key="a.id">
              <td><span class="sev" :class="a.severity">{{ severityLabel(a.severity) }}</span></td>
              <td>{{ a.alert_type }}</td>
              <td>{{ a.title }}</td>
              <td class="truncate">{{ a.content || '-' }}</td>
              <td>{{ statusLabel(a.status) }}</td>
              <td>{{ formatTime(a.created_at) }}</td>
              <td>
                <button v-permission="'alert:write'" v-if="a.status === 'open'" class="link" @click="doResolve(a)">解决</button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="!loading && list.length === 0" class="empty">暂无告警</div>
      </div>

      <div class="pager">
        <span>共 {{ total }} 条</span>
        <button class="btn" :disabled="page.page <= 1" @click="page.page--; load()">上一页</button>
        <span>{{ page.page }}</span>
        <button class="btn" :disabled="page.page * page.page_size >= total" @click="page.page++; load()">下一页</button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.btn {
  height: 34px;
  border: 1px solid var(--color-border);
  background: #fff;
  border-radius: var(--radius-md);
  padding: 0 14px;
  cursor: pointer;
  font-size: 13px;
}
.link {
  border: none;
  background: transparent;
  color: var(--color-brand);
  cursor: pointer;
  font-size: 13px;
}
.table-wrap {
  background: #fff;
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
.truncate {
  max-width: 320px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sev {
  display: inline-block;
  border-radius: var(--radius-sm);
  padding: 2px 8px;
  font-size: 12px;
}
.sev.critical {
  background: #ffece8;
  color: var(--color-danger);
}
.sev.high {
  background: #fff3e8;
  color: var(--color-warning);
}
.sev.medium {
  background: #fff7e8;
  color: #ffc800;
}
.sev.low {
  background: #e8ffea;
  color: var(--color-success);
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
