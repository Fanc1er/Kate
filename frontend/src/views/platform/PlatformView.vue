<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { listOrganizations, createOrganization, listWorkerNodes, getPlatformStats } from '../../api/platform'
import { formatTime, statusLabel } from '../../utils/format'

const orgs = ref<Array<{ id: number; name: string; plan: string; status: string; created_at: string }>>([])
const orgTotal = ref(0)
const workers = ref<Array<{ id: number; name: string; client_id: string; status: string; last_heartbeat_at?: string }>>([])
const stats = ref<Record<string, unknown> | null>(null)
const orgPage = reactive({ page: 1, page_size: 20 })

const showCreate = ref(false)
const form = reactive({ name: '', plan: 'free' })
const saving = ref(false)
const error = ref('')

async function loadOrgs(): Promise<void> {
  const res = await listOrganizations({ page: orgPage.page, page_size: orgPage.page_size })
  orgs.value = res.list as never
  orgTotal.value = res.total
}

async function loadWorkers(): Promise<void> {
  try {
    const res = await listWorkerNodes({ page: 1, page_size: 100 })
    workers.value = res.list as never
  } catch {
    // 忽略
  }
}

async function loadStats(): Promise<void> {
  try {
    stats.value = await getPlatformStats()
  } catch {
    // 忽略
  }
}

async function create(): Promise<void> {
  if (!form.name) {
    error.value = '请输入组织名称'
    return
  }
  saving.value = true
  error.value = ''
  try {
    await createOrganization({ name: form.name, plan: form.plan })
    showCreate.value = false
    await loadOrgs()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void loadOrgs()
  void loadWorkers()
  void loadStats()
})
</script>

<template>
  <div class="platform-page">
    <div class="toolbar">
      <h2>平台管理</h2>
      <span class="spacer" />
      <button class="btn primary" @click="showCreate = true">创建组织</button>
    </div>

    <div v-if="stats" class="stats-cards">
      <div class="stat-card"><div class="label">组织数</div><div class="value">{{ (stats as Record<string, number>).organizations ?? orgs.length }}</div></div>
      <div class="stat-card"><div class="label">Worker 节点</div><div class="value">{{ workers.length }}</div></div>
    </div>

    <h3>组织列表</h3>
    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>ID</th>
            <th>名称</th>
            <th>套餐</th>
            <th>状态</th>
            <th>创建时间</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="o in orgs" :key="o.id">
            <td>{{ o.id }}</td>
            <td>{{ o.name }}</td>
            <td>{{ o.plan }}</td>
            <td>{{ statusLabel(o.status) }}</td>
            <td>{{ formatTime(o.created_at) }}</td>
          </tr>
        </tbody>
      </table>
      <div v-if="orgs.length === 0" class="empty">暂无组织</div>
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

    <div v-if="showCreate" class="modal-mask" @click.self="showCreate = false">
      <div class="modal">
        <h3>创建组织</h3>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="field">
          <label>组织名称</label>
          <input v-model="form.name" class="input" />
        </div>
        <div class="field">
          <label>套餐</label>
          <select v-model="form.plan" class="input">
            <option value="free">免费</option>
            <option value="pro">专业版</option>
            <option value="enterprise">企业版</option>
          </select>
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showCreate = false">取消</button>
          <button class="btn primary" :disabled="saving" @click="create">{{ saving ? '创建中…' : '创建' }}</button>
        </div>
      </div>
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
.stats-cards {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
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
  width: 420px;
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
  height: 34px;
  border: 1px solid #e5e6eb;
  border-radius: 6px;
  padding: 0 10px;
  outline: none;
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
