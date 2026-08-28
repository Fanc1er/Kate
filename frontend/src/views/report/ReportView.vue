<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { getStats, getTopRisks, type TopRisk } from '../../api/dashboard'
import { formatTime } from '../../utils/format'
import { downloadBlob } from '../../utils/format'
import { listFindings } from '../../api/event'
import {
  listTemplates, createTemplate, updateTemplate, deleteTemplate, runTemplate,
  listArchives, deleteArchive, downloadArchive,
  type ReportTemplate, type ReportArchive, type TemplateInput,
} from '../../api/report'
import { toast } from '../../utils/toast'

const tab = ref<'overview' | 'templates' | 'archives'>('overview')

// 即时概览
const stats = ref<Record<string, number> | null>(null)
const topRisks = ref<TopRisk[]>([])
const generatedAt = ref('')
const generating = ref(false)

async function generate(): Promise<void> {
  generating.value = true
  try {
    const [s, r] = await Promise.all([getStats(), getTopRisks(10)])
    stats.value = s as unknown as Record<string, number>
    topRisks.value = r
    generatedAt.value = formatTime(new Date())
  } finally {
    generating.value = false
  }
}

async function exportCsv(): Promise<void> {
  try {
    const res = await listFindings({ page: 1, page_size: 200 })
    const rows = res.list.map((f) => [f.id, f.severity, f.title, f.url, f.status, f.risk_score].join(','))
    const csv = ['id,severity,title,url,status,risk_score', ...rows].join('\n')
    downloadBlob(new Blob([csv], { type: 'text/csv' }), `findings-${Date.now()}.csv`)
    toast.success('CSV 导出成功')
  } catch {
    // 拦截器已提示
  }
}

// 定时模板
const templates = ref<ReportTemplate[]>([])
const tplLoading = ref(false)
const showTplForm = ref(false)
const editingId = ref<number | null>(null)
const tplSaving = ref(false)
const tplForm = reactive({ name: '', period: 'daily', cron_expr: '0 8 * * *', timezone: 'Asia/Shanghai', enabled: true })

async function loadTemplates(): Promise<void> {
  tplLoading.value = true
  try {
    templates.value = await listTemplates()
  } finally {
    tplLoading.value = false
  }
}

function openTplCreate(): void {
  editingId.value = null
  Object.assign(tplForm, { name: '', period: 'daily', cron_expr: '0 8 * * *', timezone: 'Asia/Shanghai', enabled: true })
  showTplForm.value = true
}

function openTplEdit(tpl: ReportTemplate): void {
  editingId.value = tpl.id
  Object.assign(tplForm, {
    name: tpl.name, period: tpl.period || 'daily', cron_expr: tpl.cron_expr,
    timezone: tpl.timezone || 'Asia/Shanghai', enabled: tpl.enabled,
  })
  showTplForm.value = true
}

async function submitTpl(): Promise<void> {
  if (!tplForm.name || !tplForm.cron_expr) {
    toast.error('名称与 cron 表达式必填')
    return
  }
  tplSaving.value = true
  try {
    const input: TemplateInput = { ...tplForm }
    if (editingId.value) {
      await updateTemplate(editingId.value, input)
      toast.success('已保存')
    } else {
      await createTemplate(input)
      toast.success('已创建')
    }
    showTplForm.value = false
    await loadTemplates()
  } catch {
    // 拦截器已提示
  } finally {
    tplSaving.value = false
  }
}

async function toggleTpl(tpl: ReportTemplate): Promise<void> {
  try {
    await updateTemplate(tpl.id, { enabled: !tpl.enabled })
    await loadTemplates()
  } catch {
    // 拦截器已提示
  }
}

async function removeTpl(tpl: ReportTemplate): Promise<void> {
  try {
    await deleteTemplate(tpl.id)
    toast.success('已删除')
    await loadTemplates()
  } catch {
    // 拦截器已提示
  }
}

async function runTpl(tpl: ReportTemplate): Promise<void> {
  try {
    await runTemplate(tpl.id)
    toast.success('报告已生成，可在「报告存档」中下载')
  } catch {
    // 拦截器已提示
  }
}

// 报告存档
const archives = ref<ReportArchive[]>([])
const archLoading = ref(false)

async function loadArchives(): Promise<void> {
  archLoading.value = true
  try {
    archives.value = await listArchives()
  } finally {
    archLoading.value = false
  }
}

async function removeArchive(rep: ReportArchive): Promise<void> {
  try {
    await deleteArchive(rep.id)
    toast.success('已删除')
    await loadArchives()
  } catch {
    // 拦截器已提示
  }
}

async function download(rep: ReportArchive): Promise<void> {
  try {
    await downloadArchive(rep.id, `${rep.name}-${rep.created_at.slice(0, 10)}.pdf`)
    toast.success('开始下载')
  } catch {
    toast.error('下载失败')
  }
}

function switchTab(name: 'overview' | 'templates' | 'archives'): void {
  tab.value = name
  if (name === 'templates' && templates.value.length === 0) void loadTemplates()
  if (name === 'archives' && archives.value.length === 0) void loadArchives()
}

function snapshotOf(rep: ReportArchive): { assets: number; findings: number; alerts_open: number; critical: number; high: number } | null {
  try {
    return JSON.parse(rep.snapshot)
  } catch {
    return null
  }
}

onMounted(() => void generate())
</script>

<template>
  <div class="report-page">
    <div class="tabs">
      <button class="tab" :class="{ active: tab === 'overview' }" @click="switchTab('overview')">即时概览</button>
      <button class="tab" :class="{ active: tab === 'templates' }" @click="switchTab('templates')">定时模板</button>
      <button class="tab" :class="{ active: tab === 'archives' }" @click="switchTab('archives')">报告存档</button>
    </div>

    <template v-if="tab === 'overview'">
      <div class="toolbar">
        <span class="hint">基于当前数据快照的即时报告</span>
        <span class="spacer" />
        <button class="btn" @click="generate">重新生成</button>
        <button class="btn primary" @click="exportCsv">导出漏洞清单 CSV</button>
      </div>

      <div v-if="stats" class="report">
        <p class="meta">生成时间：{{ generatedAt }}</p>
        <h3>资产与发现概览</h3>
        <table class="table">
          <tbody>
            <tr><td>资产总数</td><td>{{ stats.assets }}</td></tr>
            <tr><td>发现总数</td><td>{{ stats.findings }}</td></tr>
            <tr><td>今日事件</td><td>{{ stats.events_today }}</td></tr>
            <tr><td>未处理告警</td><td>{{ stats.alerts_open }}</td></tr>
            <tr><td>严重</td><td>{{ stats.critical }}</td></tr>
            <tr><td>高危</td><td>{{ stats.high }}</td></tr>
            <tr><td>引擎覆盖率</td><td>{{ stats.coverage }}%</td></tr>
          </tbody>
        </table>

        <h3>风险 Top10</h3>
        <table class="table">
          <thead>
            <tr>
              <th>#</th>
              <th>标题</th>
              <th>等级</th>
              <th>风险分</th>
              <th>资产</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(r, i) in topRisks" :key="i">
              <td>{{ i + 1 }}</td>
              <td>{{ r.title }}</td>
              <td>{{ r.severity }}</td>
              <td>{{ r.risk_score }}</td>
              <td class="mono">{{ r.url }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else-if="generating">生成中…</p>
    </template>

    <template v-else-if="tab === 'templates'">
      <div class="toolbar">
        <span class="hint">按 cron 表达式定时生成报告存档（分 时 日 月 周，支持 */N）</span>
        <span class="spacer" />
        <button class="btn primary" @click="openTplCreate">新建模板</button>
      </div>

      <div v-if="showTplForm" class="form-card">
        <h4>{{ editingId ? '编辑模板' : '新建定时报告模板' }}</h4>
        <div class="grid">
          <label>名称 *<input v-model="tplForm.name" placeholder="每日安全日报" /></label>
          <label>周期
            <select v-model="tplForm.period">
              <option value="daily">每日</option>
              <option value="weekly">每周</option>
              <option value="monthly">每月</option>
            </select>
          </label>
          <label>cron 表达式 *<input v-model="tplForm.cron_expr" placeholder="0 8 * * *" /></label>
          <label>时区<input v-model="tplForm.timezone" placeholder="Asia/Shanghai" /></label>
          <label class="checkbox"><input v-model="tplForm.enabled" type="checkbox" /> 启用调度</label>
        </div>
        <div class="actions">
          <button class="btn" @click="showTplForm = false">取消</button>
          <button class="btn primary" :disabled="tplSaving" @click="submitTpl">{{ tplSaving ? '保存中...' : '保存' }}</button>
        </div>
      </div>

      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>名称</th>
              <th>周期</th>
              <th>cron</th>
              <th>时区</th>
              <th>状态</th>
              <th>上次生成</th>
              <th style="width: 220px">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="tpl in templates" :key="tpl.id">
              <td>{{ tpl.name }}</td>
              <td>{{ tpl.period === 'weekly' ? '每周' : tpl.period === 'monthly' ? '每月' : '每日' }}</td>
              <td class="mono">{{ tpl.cron_expr }}</td>
              <td>{{ tpl.timezone || '–' }}</td>
              <td>
                <span class="state" :class="tpl.enabled ? 'on' : 'off'">{{ tpl.enabled ? '启用中' : '已停用' }}</span>
              </td>
              <td>{{ tpl.last_run_at ? formatTime(tpl.last_run_at) : '–' }}</td>
              <td class="ops">
                <button class="link" @click="runTpl(tpl)">立即生成</button>
                <button class="link" @click="openTplEdit(tpl)">编辑</button>
                <button class="link" @click="toggleTpl(tpl)">{{ tpl.enabled ? '停用' : '启用' }}</button>
                <button class="link danger" @click="removeTpl(tpl)">删除</button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="!tplLoading && templates.length === 0" class="empty">暂无模板，点击右上角「新建模板」配置定时报告</div>
      </div>
    </template>

    <template v-else>
      <div class="toolbar">
        <span class="hint">定时生成与手动生成的报告 PDF 存档，可下载留存</span>
        <span class="spacer" />
        <button class="btn" @click="loadArchives">刷新</button>
      </div>

      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>标题</th>
              <th>资产</th>
              <th>发现</th>
              <th>未处理告警</th>
              <th>严重/高危</th>
              <th>生成时间</th>
              <th style="width: 120px">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="rep in archives" :key="rep.id">
              <td>{{ rep.title }}</td>
              <td>{{ snapshotOf(rep)?.assets ?? '–' }}</td>
              <td>{{ snapshotOf(rep)?.findings ?? '–' }}</td>
              <td>{{ snapshotOf(rep)?.alerts_open ?? '–' }}</td>
              <td>{{ snapshotOf(rep)?.critical ?? '–' }} / {{ snapshotOf(rep)?.high ?? '–' }}</td>
              <td>{{ formatTime(rep.created_at) }}</td>
              <td class="ops">
                <button class="link" @click="download(rep)">下载</button>
                <button class="link danger" @click="removeArchive(rep)">删除</button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="!archLoading && archives.length === 0" class="empty">暂无报告存档，可在「定时模板」中立即生成</div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.report-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.tabs {
  display: flex;
  gap: 4px;
  border-bottom: 1px solid var(--color-border-light);
}
.tab {
  height: 38px;
  padding: 0 18px;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  font-size: 14px;
  color: var(--color-text-secondary);
}
.tab.active {
  color: var(--color-brand);
  border-bottom-color: var(--color-brand);
  font-weight: 600;
}
.toolbar {
  display: flex;
  align-items: center;
}
.hint {
  font-size: 12px;
  color: var(--color-text-tertiary);
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
  margin-left: 8px;
}
.btn.primary {
  background: var(--color-brand);
  color: #fff;
  border-color: var(--color-brand);
}
.report {
  background: #fff;
  border-radius: var(--radius-md);
  padding: 24px;
  box-shadow: var(--shadow-card);
}
.meta {
  color: var(--color-text-tertiary);
  font-size: 12px;
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
.grid label {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--color-text-secondary);
}
.grid label.checkbox {
  flex-direction: row;
  align-items: center;
  gap: 6px;
}
input,
select {
  height: 32px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 0 10px;
  font-size: 13px;
  outline: none;
}
input[type='checkbox'] {
  height: auto;
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
  margin-bottom: 0;
}
.table th,
.table td {
  text-align: left;
  padding: 10px;
  border-bottom: 1px solid var(--color-border-light);
}
.report .table th,
.report .table td {
  padding: 8px 10px;
}
.report .table {
  margin-bottom: 24px;
}
.mono {
  font-family: var(--font-family-mono);
  font-size: 12px;
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
