<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { listPolicies, createPolicy, updatePolicy, type Policy } from '../../api/task'
import { formatTime } from '../../utils/format'

const list = ref<Policy[]>([])
const loading = ref(false)
const showForm = ref(false)
const editing = ref<Policy | null>(null)
const saving = ref(false)
const error = ref('')

const ENGINES = [
  'availability',
  'vuln_scan',
  'dns',
  'sensitive',
  'webshell',
  'content',
  'intel',
  'subdomain',
  'port',
  'tech_stack',
]

const form = reactive({
  name: '',
  scenario: 'daily',
  concurrency: 4,
  timeout: 60,
  rate_limit: 10,
  scan_depth: 2,
  crawl_subpages: true,
  allow_static: false,
  same_origin: true,
  engine_switches: {} as Record<string, boolean>,
})

async function load(): Promise<void> {
  loading.value = true
  try {
    list.value = await listPolicies()
  } finally {
    loading.value = false
  }
}

function parseSwitches(raw?: string): Record<string, boolean> {
  if (!raw) return {}
  try {
    return JSON.parse(raw)
  } catch {
    return {}
  }
}

function openCreate(): void {
  editing.value = null
  Object.assign(form, {
    name: '',
    scenario: 'daily',
    concurrency: 4,
    timeout: 60,
    rate_limit: 10,
    scan_depth: 2,
    crawl_subpages: true,
    allow_static: false,
    same_origin: true,
    engine_switches: Object.fromEntries(ENGINES.map((e) => [e, true])),
  })
  error.value = ''
  showForm.value = true
}

function openEdit(p: Policy): void {
  editing.value = p
  Object.assign(form, {
    name: p.name,
    scenario: p.scenario,
    concurrency: p.concurrency,
    timeout: p.timeout,
    rate_limit: p.rate_limit,
    scan_depth: p.scan_depth,
    crawl_subpages: p.crawl_subpages,
    allow_static: p.allow_static,
    same_origin: p.same_origin,
    engine_switches: parseSwitches(p.engine_switches),
  })
  error.value = ''
  showForm.value = true
}

async function save(): Promise<void> {
  if (!form.name) {
    error.value = '请输入策略名称'
    return
  }
  saving.value = true
  error.value = ''
  try {
    const payload = {
      name: form.name,
      scenario: form.scenario,
      concurrency: form.concurrency,
      timeout: form.timeout,
      rate_limit: form.rate_limit,
      scan_depth: form.scan_depth,
      crawl_subpages: form.crawl_subpages,
      allow_static: form.allow_static,
      same_origin: form.same_origin,
      engine_switches: JSON.stringify(form.engine_switches),
    }
    if (editing.value) {
      await updatePolicy(editing.value.id, payload)
    } else {
      await createPolicy(payload)
    }
    showForm.value = false
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="policy-page">
    <div class="toolbar">
      <h2>策略模板</h2>
      <span class="spacer" />
      <button class="btn primary" @click="openCreate">新建策略</button>
    </div>

    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>名称</th>
            <th>场景</th>
            <th>并发</th>
            <th>超时(min)</th>
            <th>速率</th>
            <th>深度</th>
            <th>子页面</th>
            <th>版本</th>
            <th>更新时间</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in list" :key="p.id">
            <td>{{ p.name }}</td>
            <td>{{ p.scenario }}</td>
            <td>{{ p.concurrency }}</td>
            <td>{{ p.timeout }}</td>
            <td>{{ p.rate_limit }}</td>
            <td>{{ p.scan_depth }}</td>
            <td>{{ p.crawl_subpages ? '是' : '否' }}</td>
            <td>v{{ p.version }}</td>
            <td>{{ formatTime(p.updated_at) }}</td>
            <td><button class="link" @click="openEdit(p)">编辑</button></td>
          </tr>
        </tbody>
      </table>
      <div v-if="!loading && list.length === 0" class="empty">暂无策略模板</div>
    </div>

    <div v-if="showForm" class="modal-mask" @click.self="showForm = false">
      <div class="modal">
        <h3>{{ editing ? '编辑策略' : '新建策略' }}</h3>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="field">
          <label>名称</label>
          <input v-model="form.name" class="input" />
        </div>
        <div class="field">
          <label>场景</label>
          <select v-model="form.scenario" class="input">
            <option value="daily">日常巡检</option>
            <option value="important">重要保障</option>
            <option value="hw">护网</option>
          </select>
        </div>
        <div class="field-row">
          <div class="field">
            <label>并发</label>
            <input v-model.number="form.concurrency" type="number" class="input" />
          </div>
          <div class="field">
            <label>超时(min)</label>
            <input v-model.number="form.timeout" type="number" class="input" />
          </div>
          <div class="field">
            <label>速率(req/s)</label>
            <input v-model.number="form.rate_limit" type="number" class="input" />
          </div>
          <div class="field">
            <label>扫描深度</label>
            <input v-model.number="form.scan_depth" type="number" class="input" />
          </div>
        </div>
        <div class="field">
          <label>引擎开关</label>
          <div class="engine-grid">
            <label v-for="e in ENGINES" :key="e" class="check-item">
              <input v-model="form.engine_switches[e]" type="checkbox" />
              {{ e }}
            </label>
          </div>
        </div>
        <div class="field">
          <label class="check-item">
            <input v-model="form.crawl_subpages" type="checkbox" />
            爬取子页面
          </label>
          <label class="check-item">
            <input v-model="form.allow_static" type="checkbox" />
            允许静态资源
          </label>
          <label class="check-item">
            <input v-model="form.same_origin" type="checkbox" />
            仅同源
          </label>
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showForm = false">取消</button>
          <button class="btn primary" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.policy-page {
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
  border: 1px solid var(--color-border);
  background: #fff;
  border-radius: var(--radius-md);
  padding: 0 14px;
  cursor: pointer;
  font-size: 13px;
}
.btn.primary {
  background: var(--color-brand);
  color: #fff;
  border-color: var(--color-brand);
}
.table-wrap {
  background: #fff;
  border-radius: var(--radius-md);
  padding: 8px;
  box-shadow: var(--shadow-card);
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
  border-bottom: 1px solid var(--color-border-light);
}
.empty {
  text-align: center;
  color: var(--color-text-tertiary);
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
  width: 560px;
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
.field-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}
.input {
  width: 100%;
  height: 34px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 0 10px;
  outline: none;
}
.engine-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 6px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 8px;
}
.check-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  padding: 2px 0;
}
.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}
.error {
  color: var(--color-danger);
  font-size: 13px;
}
</style>
