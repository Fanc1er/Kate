<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { listScenarios, createScenario, updateScenario, deleteScenario, toggleScenario, type Scenario, type ScenarioInput } from '../../api/scenario'
import { formatTime } from '../../utils/format'
import { toast } from '../../utils/toast'

const list = ref<Scenario[]>([])
const loading = ref(false)
const showForm = ref(false)
const editing = ref<number | null>(null)
const saving = ref(false)

const form = reactive<Omit<ScenarioInput, 'policy_id'> & { policy_id: number; activated?: boolean }>({
  name: '',
  scenario_type: 'full_scan',
  description: '',
  policy_id: 0,
  asset_group_name: '',
  activated: false,
})

async function load(): Promise<void> {
  loading.value = true
  try {
    list.value = await listScenarios()
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  editing.value = null
  Object.assign(form, {
    name: '', scenario_type: 'full_scan', description: '',
    policy_id: 0, asset_group_name: '', activated: false,
  })
  showForm.value = true
}

function openEdit(sc: Scenario): void {
  editing.value = sc.id
  Object.assign(form, {
    name: sc.name, scenario_type: sc.scenario_type, description: sc.description,
    policy_id: sc.policy_id, asset_group_name: sc.asset_group_name, activated: false,
  })
  showForm.value = true
}

async function submit(): Promise<void> {
  if (!form.name || !form.policy_id) {
    toast.error('场景名称与策略模板必填')
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await updateScenario(editing.value, form)
      toast.success('已保存')
    } else {
      await createScenario(form)
      toast.success('已创建')
    }
    showForm.value = false
    await load()
  } catch {
    // 拦截器已提示
  } finally {
    saving.value = false
  }
}

async function toggle(sc: Scenario): Promise<void> {
  try {
    const next = !sc.activated
    await toggleScenario(sc.id, next)
    toast.success(next ? '已激活，任务触发中' : '已停用')
    await load()
  } catch {
    // 拦截器已提示
  }
}

async function remove(sc: Scenario): Promise<void> {
  try {
    await deleteScenario(sc.id)
    toast.success('已删除')
    await load()
  } catch {
    // 拦截器已提示
  }
}

onMounted(load)
</script>

<template>
  <div class="scenario-page list-main">
    <div class="toolbar">
      <span class="hint">激活场景后自动为资产组内全部活跃资产创建全量扫描任务</span>
      <span class="spacer" />
      <button class="btn primary" @click="openCreate">新建场景</button>
    </div>

    <div v-if="showForm" class="form-card">
      <h4>{{ editing ? '编辑场景' : '新建扫描场景' }}</h4>
      <div class="grid">
        <label>场景名称 *<input v-model="form.name" placeholder="每日合规巡检" /></label>
        <label>场景类型
          <select v-model="form.scenario_type">
            <option value="full_scan">全量扫描</option>
            <option value="compliance">合规检查</option>
            <option value="vuln_scan">漏洞专项</option>
            <option value="content_audit">内容审核</option>
          </select>
        </label>
        <label>策略模板 *<input v-model.number="form.policy_id" type="number" placeholder="1" /></label>
        <label>资产组名<input v-model="form.asset_group_name" placeholder="留空 = 全部资产" /></label>
      </div>
      <label class="desc">描述<textarea v-model="form.description" rows="2" /></label>
      <div class="actions">
        <button class="btn" @click="showForm = false">取消</button>
        <button class="btn primary" :disabled="saving" @click="submit">{{ saving ? '保存中...' : '保存' }}</button>
      </div>
    </div>

    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>名称</th>
            <th>类型</th>
            <th>策略 ID</th>
            <th>资产组</th>
            <th>状态</th>
            <th>上次激活</th>
            <th style="width: 200px">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="sc in list" :key="sc.id">
            <td>{{ sc.name }}</td>
            <td>{{ sc.scenario_type }}</td>
            <td class="mono">{{ sc.policy_id }}</td>
            <td>{{ sc.asset_group_name || '全部' }}</td>
            <td>
              <span class="state" :class="sc.activated ? 'on' : 'off'">{{ sc.activated ? '已激活' : '已停用' }}</span>
            </td>
            <td>{{ sc.activated_at ? formatTime(sc.activated_at) : '–' }}</td>
            <td class="ops">
              <button class="link" @click="toggle(sc)">{{ sc.activated ? '停用' : '激活' }}</button>
              <button class="link" @click="openEdit(sc)">编辑</button>
              <button class="link danger" @click="remove(sc)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="!loading && list.length === 0" class="empty">暂无扫描场景，点击右上角「新建场景」配置预置任务规则</div>
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
  border-color: var(--color-brand);
  color: #fff;
}
.link {
  color: var(--color-brand);
  cursor: pointer;
}
.link.danger {
  color: var(--color-danger);
}
.ops {
  display: flex;
  gap: 10px;
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
  grid-template-columns: repeat(2, 1fr);
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
.state {
  display: inline-block;
  border-radius: var(--radius-sm);
  padding: 2px 8px;
  font-size: 12px;
}
.state.on {
  background: #e8ffea;
  color: var(--color-success);
}
.state.off {
  background: #f0f1f3;
  color: var(--color-text-tertiary);
}
.empty {
  text-align: center;
  color: var(--color-text-tertiary);
  padding: 40px 0;
}
</style>
