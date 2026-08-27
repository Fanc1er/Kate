<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { listIntel, importIntel, deleteIntel, type IntelItem, type IntelInput } from '../../api/intel'
import { formatTime, severityLabel } from '../../utils/format'
import Skeleton from '../../components/Skeleton.vue'
import { toast } from '../../utils/toast'

const list = ref<IntelItem[]>([])
const total = ref(0)
const loading = ref(false)
const showForm = ref(false)
const saving = ref(false)
const page = reactive({ page: 1, page_size: 20 })

const form = reactive({
  intel_id: '',
  title: '',
  description: '',
  severity: 'high',
  component: '',
  max_version: '',
})

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listIntel({ page: page.page, page_size: page.page_size })
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  Object.assign(form, {
    intel_id: '', title: '', description: '', severity: 'high', component: '', max_version: '',
  })
  showForm.value = true
}

async function submit(): Promise<void> {
  if (!form.intel_id || !form.title) {
    toast.error('编号与标题必填')
    return
  }
  saving.value = true
  try {
    const item: IntelInput = {
      intel_id: form.intel_id,
      title: form.title,
      description: form.description || undefined,
      severity: form.severity,
      component: form.component || undefined,
      max_version: form.max_version || undefined,
    }
    const res = await importIntel([item])
    toast.success(`已导入 ${res.imported} 条`)
    showForm.value = false
    await load()
  } catch {
    // 拦截器已提示
  } finally {
    saving.value = false
  }
}

async function remove(it: IntelItem): Promise<void> {
  try {
    await deleteIntel(it.id)
    toast.success('已删除')
    await load()
  } catch {
    // 拦截器已提示
  }
}

onMounted(load)
</script>

<template>
  <div class="intel-page list-main">
    <div class="toolbar">
      <span class="hint">带组件名与版本上限的条目将随扫描任务下发，情报引擎按目标 Server 头自动匹配 CVE</span>
      <span class="spacer" />
      <button class="btn primary" @click="openCreate">导入情报</button>
    </div>

    <div v-if="showForm" class="form-card">
      <h4>导入组件漏洞情报</h4>
      <div class="grid">
        <label>漏洞编号 *<input v-model="form.intel_id" placeholder="CVE-2023-44487" /></label>
        <label>标题 *<input v-model="form.title" placeholder="HTTP/2 Rapid Reset DoS" /></label>
        <label>等级
          <select v-model="form.severity">
            <option value="critical">严重</option>
            <option value="high">高危</option>
            <option value="medium">中危</option>
            <option value="low">低危</option>
          </select>
        </label>
        <label>组件名<input v-model="form.component" placeholder="nginx / tomcat / apache（留空仅入库）" /></label>
        <label>受影响最高版本<input v-model="form.max_version" placeholder="1.24.0" /></label>
      </div>
      <label class="desc">描述<textarea v-model="form.description" rows="2" /></label>
      <div class="actions">
        <button class="btn" @click="showForm = false">取消</button>
        <button class="btn primary" :disabled="saving" @click="submit">{{ saving ? '导入中...' : '确认导入' }}</button>
      </div>
    </div>

    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>编号</th>
            <th>标题</th>
            <th>等级</th>
            <th>组件</th>
            <th>受影响版本</th>
            <th>来源</th>
            <th>更新时间</th>
            <th style="width: 80px">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="it in list" :key="it.id">
            <td class="mono">{{ it.intel_id }}</td>
            <td>{{ it.title }}</td>
            <td><span class="sev" :class="it.severity">{{ severityLabel(it.severity) }}</span></td>
            <td>{{ it.tech_stack || '–' }}</td>
            <td class="mono">{{ it.scope || '–' }}</td>
            <td>{{ it.source === 'manual' ? '手动' : it.source }}</td>
            <td>{{ formatTime(it.updated_at) }}</td>
            <td><button class="link danger" @click="remove(it)">删除</button></td>
          </tr>
        </tbody>
      </table>
      <div v-if="!loading && list.length === 0" class="empty">暂无情报条目，点击右上角「导入情报」开始建设情报库</div>
      <Skeleton v-if="loading" :rows="6" :cols="6" />
    </div>

    <div class="pager">
      <span>共 {{ total }} 条</span>
      <button class="btn" :disabled="page.page <= 1" @click="page.page--; load()">上一页</button>
      <span>{{ page.page }}</span>
      <button class="btn" :disabled="page.page * page.page_size >= total" @click="page.page++; load()">下一页</button>
    </div>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
}
.hint {
  font-size: 12px;
  color: var(--color-text-tertiary);
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
.link.danger {
  color: var(--color-danger);
}
.form-card {
  background: #fff;
  border-radius: var(--radius-md);
  padding: 16px 20px;
  box-shadow: var(--shadow-card);
}
.form-card h4 {
  margin: 0 0 12px;
}
.grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}
.grid label,
.desc {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--color-text-secondary);
}
input,
select,
textarea {
  height: 32px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 0 10px;
  font-size: 13px;
  outline: none;
}
textarea {
  padding-top: 6px;
}
.desc {
  margin-top: 12px;
}
.actions {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
  gap: 8px;
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
