<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import {
  listTasks,
  createTask,
  stopTask,
  rerunTask,
  deleteTask,
  batchStop,
  listPolicies,
  type ScanTask,
  type Policy,
} from '../../api/task'
import { listAssets, type Asset } from '../../api/asset'
import { formatTime, statusLabel } from '../../utils/format'
import { toast, confirmDialog } from '../../utils/toast'
import Skeleton from '../../components/Skeleton.vue'

const list = ref<ScanTask[]>([])
const total = ref(0)
const loading = ref(false)
const statusFilter = ref('')
const selected = ref<number[]>([])
const page = reactive({ page: 1, page_size: 20 })

const showCreate = ref(false)
const assets = ref<Asset[]>([])
const policies = ref<Policy[]>([])
const form = reactive({ asset_ids: [] as number[], policy_id: 0 })
const saving = ref(false)
const error = ref('')

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listTasks({ page: page.page, page_size: page.page_size, status: statusFilter.value || undefined })
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

async function openCreate(): Promise<void> {
  showCreate.value = true
  error.value = ''
  Object.assign(form, { asset_ids: [], policy_id: 0 })
  try {
    const [a, p] = await Promise.all([listAssets({ page: 1, page_size: 200 }), listPolicies()])
    assets.value = a.list
    policies.value = p
  } catch {
    // 忽略
  }
}

async function save(): Promise<void> {
  if (form.asset_ids.length === 0 || !form.policy_id) {
    error.value = '请选择资产与策略模板'
    return
  }
  saving.value = true
  error.value = ''
  try {
    await createTask({ asset_ids: form.asset_ids, policy_id: form.policy_id })
    showCreate.value = false
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

async function doStop(t: ScanTask): Promise<void> {
  try {
    await stopTask(t.id)
    toast.success('任务已停止')
    await load()
  } catch {
    // 拦截器已提示
  }
}

async function doRerun(t: ScanTask): Promise<void> {
  try {
    await rerunTask(t.id)
    toast.success('任务已重新入队')
    await load()
  } catch {
    // 拦截器已提示
  }
}

async function doDelete(t: ScanTask): Promise<void> {
  if (!confirmDialog('确认删除该任务？')) return
  try {
    await deleteTask(t.id)
    toast.success('任务已删除')
    await load()
  } catch {
    // 拦截器已提示
  }
}

async function doBatchStop(): Promise<void> {
  if (selected.value.length === 0) return
  try {
    await batchStop(selected.value)
    selected.value = []
    toast.success('批量停止已提交')
    await load()
  } catch {
    // 拦截器已提示
  }
}

function toggleSelect(id: number): void {
  const idx = selected.value.indexOf(id)
  if (idx >= 0) selected.value.splice(idx, 1)
  else selected.value.push(id)
}

const canStop = (t: ScanTask): boolean => t.status === 'pending' || t.status === 'processing'

onMounted(load)
</script>

<template>
  <div class="task-page">
    <div class="toolbar">
      <select v-model="statusFilter" class="input select" @change="load">
        <option value="">全部状态</option>
        <option value="pending">待执行</option>
        <option value="processing">执行中</option>
        <option value="completed">已完成</option>
        <option value="failed">失败</option>
        <option value="cancelled">已取消</option>
      </select>
      <button class="btn" @click="load">查询</button>
      <span class="spacer" />
      <button class="btn primary" @click="openCreate">新建任务</button>
      <button class="btn" :disabled="selected.length === 0" @click="doBatchStop">批量停止</button>
    </div>

    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th style="width: 40px"></th>
            <th>ID</th>
            <th>资产</th>
            <th>策略</th>
            <th>状态</th>
            <th>进度</th>
            <th>发现数</th>
            <th>创建时间</th>
            <th style="width: 180px">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="t in list" :key="t.id">
            <td><input type="checkbox" :checked="selected.includes(t.id)" @change="toggleSelect(t.id)" /></td>
            <td>#{{ t.id }}</td>
            <td>{{ t.asset_name || t.asset_id }}</td>
            <td>#{{ t.policy_id }}</td>
            <td>
              {{ statusLabel(t.status) }}
              <span v-if="t.task_timeout" class="tag">超时</span>
              <span v-if="t.stopped_by_user" class="tag">手动停止</span>
            </td>
            <td>{{ t.progress }}%</td>
            <td>{{ t.findings_count ?? 0 }}</td>
            <td>{{ formatTime(t.created_at) }}</td>
            <td>
              <button v-if="canStop(t)" class="link" @click="doStop(t)">停止</button>
              <button class="link" @click="doRerun(t)">重跑</button>
              <button class="link danger" @click="doDelete(t)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="!loading && list.length === 0" class="empty">暂无任务</div>
      <Skeleton v-if="loading" :rows="6" :cols="5" />
    </div>

    <div class="pager">
      <span>共 {{ total }} 条</span>
      <button class="btn" :disabled="page.page <= 1" @click="page.page--; load()">上一页</button>
      <span>{{ page.page }}</span>
      <button class="btn" :disabled="page.page * page.page_size >= total" @click="page.page++; load()">下一页</button>
    </div>

    <div v-if="showCreate" class="modal-mask" @click.self="showCreate = false">
      <div class="modal">
        <h3>新建任务</h3>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="field">
          <label>选择资产</label>
          <div class="checkbox-list">
            <label v-for="a in assets" :key="a.id" class="check-item">
              <input v-model="form.asset_ids" type="checkbox" :value="a.id" />
              {{ a.name }}（{{ a.url }}）
            </label>
          </div>
        </div>
        <div class="field">
          <label>策略模板</label>
          <select v-model="form.policy_id" class="input">
            <option :value="0">请选择</option>
            <option v-for="p in policies" :key="p.id" :value="p.id">{{ p.name }}</option>
          </select>
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showCreate = false">取消</button>
          <button class="btn primary" :disabled="saving" @click="save">{{ saving ? '创建中…' : '创建' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.task-page {
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
  border: 1px solid #e5e6eb;
  border-radius: 6px;
  padding: 0 10px;
  outline: none;
}
.select {
  width: 140px;
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
}
.btn.primary {
  background: #3370ff;
  color: #fff;
  border-color: #3370ff;
}
.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.link {
  border: none;
  background: transparent;
  color: #3370ff;
  cursor: pointer;
  font-size: 13px;
  margin-right: 8px;
}
.link.danger {
  color: #d03050;
}
.table-wrap {
  background: #fff;
  border-radius: 8px;
  padding: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
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
  border-bottom: 1px solid #f2f3f5;
}
.tag {
  display: inline-block;
  background: #ffece8;
  color: #d03050;
  font-size: 12px;
  border-radius: 4px;
  padding: 0 6px;
  margin-left: 6px;
}
.empty {
  text-align: center;
  color: #86909c;
  padding: 40px 0;
}
.pager {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
}
.modal-mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}
.modal {
  background: #fff;
  border-radius: 10px;
  padding: 24px;
  width: 520px;
  max-height: 80vh;
  overflow-y: auto;
}
.field {
  margin-bottom: 14px;
}
.field label {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
}
.input {
  width: 100%;
}
.checkbox-list {
  max-height: 200px;
  overflow-y: auto;
  border: 1px solid #e5e6eb;
  border-radius: 6px;
  padding: 8px;
}
.check-item {
  display: block;
  padding: 4px 0;
  font-size: 13px;
}
.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}
.error {
  color: #d03050;
  font-size: 13px;
}
</style>
