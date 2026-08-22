<script setup lang="ts">
import { ref, reactive, onMounted, toRef } from 'vue'
import { listFindings, updateFindingStatus, type Finding } from '../../api/event'
import { formatTime, severityLabel, statusLabel } from '../../utils/format'
import EvidenceDrawer from '../../components/EvidenceDrawer.vue'
import Skeleton from '../../components/Skeleton.vue'
import { toast } from '../../utils/toast'
import { useQuerySync } from '../../composables/useQuerySync'

const list = ref<Finding[]>([])
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

const drawerVisible = ref(false)
const drawerIds = ref<number[]>([])

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listFindings({
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

function parseEvidenceIds(raw?: string): number[] {
  if (!raw) return []
  try {
    const arr = JSON.parse(raw)
    return Array.isArray(arr) ? arr.filter((n) => typeof n === 'number') : []
  } catch {
    return []
  }
}

function openEvidence(f: Finding): void {
  const ids = parseEvidenceIds(f.evidence_ids)
  if (ids.length === 0) {
    toast.info('该发现无关联证据')
    return
  }
  drawerIds.value = ids
  drawerVisible.value = true
}

async function setStatus(f: Finding, s: string): Promise<void> {
  try {
    await updateFindingStatus(f.id, s)
    toast.success('状态已更新')
    await load()
  } catch {
    // 拦截器已提示
  }
}


onMounted(load)
</script>

<template>
  <div class="finding-page list-main">

      <div class="toolbar">
      <select v-model="severity" class="filter-input" @change="page.page = 1; load()">
        <option value="">全部等级</option>
        <option value="critical">严重</option>
        <option value="high">高危</option>
        <option value="medium">中危</option>
        <option value="low">低危</option>
      </select>
      <select v-model="status" class="filter-input" @change="page.page = 1; load()">
        <option value="">全部状态</option>
        <option value="open">待处理</option>
        <option value="confirmed">已确认</option>
        <option value="closed">已关闭</option>
        <option value="ignored">已忽略</option>
      </select>
      <span class="spacer" />
      <button class="btn" @click="load">查询</button>
    </div>
    <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>等级</th>
              <th>标题</th>
              <th>引擎</th>
              <th>URL</th>
              <th>风险分</th>
              <th>置信度</th>
              <th>状态</th>
              <th>时间</th>
              <th style="width: 140px">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="f in list" :key="f.id">
              <td><span class="sev" :class="f.severity">{{ severityLabel(f.severity) }}</span></td>
              <td>{{ f.title }}</td>
              <td>{{ f.engine_name }}</td>
              <td class="mono">{{ f.url }}</td>
              <td>{{ f.risk_score }}</td>
              <td>{{ Math.round(f.confidence * 100) }}%</td>
              <td>{{ statusLabel(f.status) }}</td>
              <td>{{ formatTime(f.created_at) }}</td>
              <td>
                <button class="link" @click="openEvidence(f)">证据</button>
                <button v-permission="'event:write'" v-if="f.status !== 'closed'" class="link" @click="setStatus(f, 'closed')">关闭</button>
                <button v-permission="'event:write'" v-if="f.status !== 'ignored'" class="link" @click="setStatus(f, 'ignored')">忽略</button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="!loading && list.length === 0" class="empty">暂无发现</div>
        <Skeleton v-if="loading" :rows="6" :cols="6" />
      </div>

      <div class="pager">
        <span>共 {{ total }} 条</span>
        <button class="btn" :disabled="page.page <= 1" @click="page.page--; load()">上一页</button>
        <span>{{ page.page }}</span>
        <button class="btn" :disabled="page.page * page.page_size >= total" @click="page.page++; load()">下一页</button>
      </div>

    <EvidenceDrawer v-model:visible="drawerVisible" :evidence-ids="drawerIds" />
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  gap: 8px;
}
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
  margin-right: 8px;
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
.mono {
  font-family: var(--font-family-mono);
  font-size: 12px;
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
