<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import {
  listTickets, getTicket, createTicket, updateTicketStatus, assignTicket,
  deleteTicket, type Ticket, type TicketDetail,
} from '../../api/ticket'
import { listEvents, type EventItem } from '../../api/event'
import { formatTime } from '../../utils/format'
import { toast } from '../../utils/toast'
import Skeleton from '../../components/Skeleton.vue'

const list = ref<Ticket[]>([])
const total = ref(0)
const loading = ref(false)
const status = ref('')
const source = ref('')
const page = reactive({ page: 1, page_size: 20 })

const STATUS_OPTIONS: Record<string, string> = {
  open: '待派发',
  dispatched: '已派发',
  fixing: '修复中',
  retest: '复测中',
  archived: '已归档',
}
function statusLabel(s: string): string {
  return STATUS_OPTIONS[s] ?? s
}

const detailVisible = ref(false)
const detail = ref<TicketDetail | null>(null)

const createVisible = ref(false)
const createForm = reactive({ event_id: 0, vuln_id: 0, assignee: '', notes: '' })
const events = ref<EventItem[]>([])
const eventsLoading = ref(false)

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listTickets({
      page: page.page,
      page_size: page.page_size,
      status: status.value || undefined,
      source: source.value || undefined,
    })
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

async function openDetail(t: Ticket): Promise<void> {
  try {
    detail.value = await getTicket(t.id)
    detailVisible.value = true
  } catch {
    // 拦截器已提示
  }
}

async function doStatus(t: Ticket, s: string): Promise<void> {
  try {
    await updateTicketStatus(t.id, s, t.version)
    toast.success('状态已更新')
    await load()
    if (detailVisible.value) await openDetail(t)
  } catch {
    // 拦截器已提示
  }
}

async function doAssign(t: Ticket): Promise<void> {
  const name = window.prompt('派发给（用户名）：', t.assignee || '')
  if (name === null) return
  if (!name.trim()) {
    toast.error('处理人不能为空')
    return
  }
  try {
    await assignTicket(t.id, name.trim(), t.version)
    toast.success('已派发')
    await load()
  } catch {
    // 拦截器已提示
  }
}

async function doDelete(t: Ticket): Promise<void> {
  if (!window.confirm(`确认删除工单 #${t.id}？`)) return
  try {
    await deleteTicket(t.id)
    toast.success('已删除')
    await load()
  } catch {
    // 拦截器已提示
  }
}

async function openCreate(): Promise<void> {
  createForm.event_id = 0
  createForm.vuln_id = 0
  createForm.assignee = ''
  createForm.notes = ''
  createVisible.value = true
  eventsLoading.value = true
  try {
    const res = await listEvents({ page: 1, page_size: 100, status: 'pending' })
    events.value = res.list
  } catch {
    // 拦截器已提示
  } finally {
    eventsLoading.value = false
  }
}

async function submitCreate(): Promise<void> {
  if (!createForm.event_id && !createForm.vuln_id) {
    toast.error('请选择事件来源')
    return
  }
  try {
    await createTicket({
      event_id: createForm.event_id || undefined,
      vuln_id: createForm.vuln_id || undefined,
      assignee: createForm.assignee || undefined,
      notes: createForm.notes || undefined,
    })
    toast.success('工单已创建')
    createVisible.value = false
    await load()
  } catch {
    // 拦截器已提示
  }
}

onMounted(load)
</script>

<template>
  <div class="ticket-page">
    <div class="toolbar">
      <select v-model="status" class="input" @change="load">
        <option value="">全部状态</option>
        <option value="open">待派发</option>
        <option value="dispatched">已派发</option>
        <option value="fixing">修复中</option>
        <option value="retest">复测中</option>
        <option value="archived">已归档</option>
      </select>
      <select v-model="source" class="input" @change="load">
        <option value="">全部来源</option>
        <option value="event">事件</option>
        <option value="vuln">漏洞</option>
      </select>
      <button class="btn" @click="load">查询</button>
      <button class="btn primary" @click="openCreate">新建工单</button>
    </div>

    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>ID</th>
            <th>来源</th>
            <th>处理人</th>
            <th>状态</th>
            <th>备注</th>
            <th>创建时间</th>
            <th style="width: 180px">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="t in list" :key="t.id">
            <td>#{{ t.id }}</td>
            <td>
              <span v-if="t.event_id" class="tag">事件 #{{ t.event_id }}</span>
              <span v-if="t.vuln_id" class="tag vuln">漏洞 #{{ t.vuln_id }}</span>
              <span v-if="!t.event_id && !t.vuln_id" class="tag">-</span>
            </td>
            <td>{{ t.assignee || '-' }}</td>
            <td><span class="st" :class="t.status">{{ statusLabel(t.status) }}</span></td>
            <td class="note">{{ t.notes || '-' }}</td>
            <td>{{ formatTime(t.created_at) }}</td>
            <td>
              <button class="link" @click="openDetail(t)">详情</button>
              <button v-if="t.status === 'open'" class="link" @click="doAssign(t)">派发</button>
              <button v-if="t.status === 'open'" class="link" @click="doStatus(t, 'dispatched')">确认</button>
              <button v-if="t.status === 'dispatched'" class="link" @click="doStatus(t, 'fixing')">修复中</button>
              <button v-if="t.status === 'fixing'" class="link" @click="doStatus(t, 'retest')">复测</button>
              <button v-if="t.status === 'retest'" class="link" @click="doStatus(t, 'archived')">归档</button>
              <button class="link danger" @click="doDelete(t)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="!loading && list.length === 0" class="empty">暂无工单</div>
      <Skeleton v-if="loading" :rows="6" :cols="5" />
    </div>

    <div class="pager">
      <span>共 {{ total }} 条</span>
      <button class="btn" :disabled="page.page <= 1" @click="page.page--; load()">上一页</button>
      <span>{{ page.page }}</span>
      <button class="btn" :disabled="page.page * page.page_size >= total" @click="page.page++; load()">下一页</button>
    </div>

    <!-- 详情抽屉 -->
    <div v-if="detailVisible && detail" class="modal-mask" @click.self="detailVisible = false">
      <div class="modal">
        <div class="modal-head">
          <span>工单 #{{ detail.ticket.id }}</span>
          <button class="link" @click="detailVisible = false">关闭</button>
        </div>
        <div class="modal-body">
          <div class="kv"><label>状态</label><span>{{ statusLabel(detail.ticket.status) }}</span></div>
          <div class="kv"><label>处理人</label><span>{{ detail.ticket.assignee || '-' }}</span></div>
          <div class="kv"><label>来源</label>
            <span>
              <span v-if="detail.event">事件 #{{ detail.event.id }} · {{ detail.event.title }}</span>
              <span v-if="detail.vulnerability">漏洞 #{{ detail.vulnerability.id }} · {{ detail.vulnerability.title }}</span>
            </span>
          </div>
          <div class="kv"><label>备注</label><span>{{ detail.ticket.notes || '-' }}</span></div>
          <div class="kv"><label>版本</label><span>v{{ detail.ticket.version }}</span></div>
        </div>
      </div>
    </div>

    <!-- 新建工单抽屉 -->
    <div v-if="createVisible" class="modal-mask" @click.self="createVisible = false">
      <div class="modal">
        <div class="modal-head">
          <span>新建工单</span>
          <button class="link" @click="createVisible = false">关闭</button>
        </div>
        <div class="modal-body">
          <div class="form-row">
            <label>事件来源</label>
            <select v-model="createForm.event_id" class="input wide">
              <option :value="0">不关联事件</option>
              <option v-for="e in events" :key="e.id" :value="e.id">{{ e.id }} · {{ e.title }}</option>
            </select>
          </div>
          <div class="form-row">
            <label>处理人</label>
            <input v-model="createForm.assignee" class="input wide" placeholder="指派给谁" />
          </div>
          <div class="form-row">
            <label>备注</label>
            <textarea v-model="createForm.notes" class="input wide area" placeholder="处理说明"></textarea>
          </div>
          <div class="form-actions">
            <button class="btn" @click="createVisible = false">取消</button>
            <button class="btn primary" @click="submitCreate">创建</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ticket-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.toolbar {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.input {
  height: 34px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 0 10px;
  outline: none;
  width: 130px;
}
.input.wide {
  width: 100%;
}
.input.area {
  height: 80px;
  padding: 8px 10px;
  resize: vertical;
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
.btn.primary {
  background: var(--color-brand);
  border-color: var(--color-brand);
  color: #fff;
}
.table-wrap {
  min-height: 200px;
}
.table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.table th {
  text-align: left;
  padding: 10px 12px;
  background: #f7f8fa;
  color: var(--color-text-secondary);
  border-bottom: 1px solid var(--color-border);
  font-weight: var(--font-weight-semibold);
  white-space: nowrap;
}
.table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--color-border-light);
  vertical-align: middle;
}
.note {
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-text-secondary);
}
.link {
  border: none;
  background: none;
  color: var(--color-brand);
  cursor: pointer;
  font-size: 13px;
  padding: 0;
  margin-right: 8px;
}
.link.danger {
  color: var(--color-danger);
}
.tag {
  display: inline-block;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  background: #f0f5ff;
  color: var(--color-info);
  font-size: 12px;
  margin-right: 4px;
}
.tag.vuln {
  background: #fff7e8;
  color: var(--color-warning);
}
.st {
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  font-size: 12px;
}
.st.open { background: #fff1f0; color: var(--color-danger); }
.st.dispatched { background: #fff7e8; color: var(--color-warning); }
.st.fixing { background: #f0f5ff; color: var(--color-info); }
.st.retest { background: #f0f5ff; color: #722ed1; }
.st.archived { background: var(--color-border-light); color: var(--color-text-tertiary); }
.empty {
  text-align: center;
  color: var(--color-text-tertiary);
  padding: 40px 0;
}
.pager {
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--color-text-secondary);
  font-size: 13px;
}
.modal-mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}
.modal {
  background: #fff;
  border-radius: var(--radius-md);
  width: 560px;
  max-width: 92vw;
  max-height: 82vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.modal-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 16px;
  border-bottom: 1px solid var(--color-border);
  font-weight: var(--font-weight-semibold);
}
.modal-body {
  padding: 16px;
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.kv {
  display: flex;
  gap: 10px;
  font-size: 13px;
}
.kv label {
  color: var(--color-text-tertiary);
  width: 70px;
  flex-shrink: 0;
}
.form-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
}
.form-row label {
  color: var(--color-text-secondary);
}
.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}
</style>
