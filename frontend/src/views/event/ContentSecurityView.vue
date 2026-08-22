<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { listFindings, type Finding } from '../../api/event'
import { listAssets, type Asset } from '../../api/asset'
import { formatTime, severityLabel, statusLabel } from '../../utils/format'
import { parseExtra } from '../../api/finding'
import EvidenceDrawer from '../../components/EvidenceDrawer.vue'
import Skeleton from '../../components/Skeleton.vue'

// Tab 定义：每个 tab 对应一类内容安全 finding。
// engine_name 为空时按 type 过滤（跨引擎同名 type）。
interface TabDef {
  key: string
  label: string
  engineName: string
  types: string[]
  columns: string[]
}

const tabs: TabDef[] = [
  {
    key: 'sensitive_content',
    label: '敏感内容',
    engineName: 'content_security',
    types: ['sensitive_word', 'content_violation', 'image_ocr'],
    columns: ['敏感词 / 分类', '命中文本', '来源'],
  },
  {
    key: 'sensitive_info',
    label: '信息泄漏',
    engineName: 'content_security',
    types: ['sensitive_info'],
    columns: ['规则', '泄漏内容', '范围'],
  },
  {
    key: 'tamper',
    label: '篡改对比',
    engineName: '',
    types: ['content_integrity', 'external_link', 'link_tampered'],
    columns: ['变更维度', '基线 → 现状', '类型'],
  },
  {
    key: 'multi_ua',
    label: '多端UA',
    engineName: 'multi_ua',
    types: ['multi_ua_availability', 'multi_ua_evaluation'],
    columns: ['端差异', '评分', '端级异常'],
  },
  {
    key: 'keyword',
    label: '关键词命中',
    engineName: 'content_security',
    types: ['keyword_hit'],
    columns: ['命中词', '规则', '敏感级别'],
  },
  {
    key: 'dead_link',
    label: '死链',
    engineName: 'content_security',
    types: ['dead_link'],
    columns: ['状态码', '链接', '失败原因'],
  },
  {
    key: 'asset_discovery',
    label: '资产发现',
    engineName: '__assets__',
    types: [],
    columns: ['来源', 'URL', '类型'],
  },
]

const activeTab = ref(tabs[0].key)
const list = ref<Finding[]>([])
const assetList = ref<Asset[]>([])
const total = ref(0)
const loading = ref(false)
const severity = ref('')
const status = ref('')
const keyword = ref('')
const page = reactive({ page: 1, page_size: 20 })

const drawerVisible = ref(false)
const drawerIds = ref<number[]>([])
const detailVisible = ref(false)
const detailFinding = ref<Finding | null>(null)

const currentTab = tabs.find((t) => t.key === activeTab.value)

function scoreClass(score: number | undefined): string {
  const s = score ?? 0
  if (s >= 85) return 'score-critical'
  if (s >= 60) return 'score-high'
  if (s >= 30) return 'score-medium'
  return 'score-low'
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const tab = tabs.find((t) => t.key === activeTab.value)
    if (!tab) return
    // 资产发现 tab 走 assets 接口（source_type 过滤）。
    if (tab.key === 'asset_discovery') {
      const res = await listAssets({
        page: page.page,
        page_size: page.page_size,
        keyword: keyword.value || undefined,
      })
      assetList.value = res.list
      total.value = res.total
      return
    }
    // 单 engine 过滤优先走 engine_name，多类型跨引擎时走 type 循环。
    if (tab.engineName) {
      const res = await listFindings({
        page: page.page,
        page_size: page.page_size,
        engine_name: tab.engineName,
        severity: severity.value || undefined,
        status: status.value || undefined,
        keyword: keyword.value || undefined,
      })
      const typeSet = new Set(tab.types)
      list.value = res.list.filter((f) => typeSet.has(f.type))
      total.value = res.total
    } else {
      // 跨引擎：按 type 分别拉取后合并（分页近似处理）。
      let merged: Finding[] = []
      let mergedTotal = 0
      for (const ty of tab.types) {
        const res = await listFindings({
          page: 1,
          page_size: 100,
          type: ty,
          severity: severity.value || undefined,
          status: status.value || undefined,
        })
        merged = merged.concat(res.list)
        mergedTotal += res.total
      }
      list.value = merged
      total.value = mergedTotal
    }
  } finally {
    loading.value = false
  }
}

function switchTab(key: string): void {
  activeTab.value = key
  page.page = 1
  load()
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
  if (ids.length === 0) return
  drawerIds.value = ids
  drawerVisible.value = true
}

function openDetail(f: Finding): void {
  detailFinding.value = f
  detailVisible.value = true
}

function closeDetail(): void {
  detailVisible.value = false
  detailFinding.value = null
}

function prettyJSON(obj: unknown): string {
  try {
    return JSON.stringify(obj, null, 2)
  } catch {
    return String(obj)
  }
}

// 详情字段辅助。
function extraOf(f: Finding): Record<string, unknown> {
  return parseExtra(f.extra || '')
}

function wordOf(f: Finding): string {
  const e = extraOf(f)
  return (e.word as string) || (e.category as string) || f.type
}

function sourceOf(f: Finding): string {
  const e = extraOf(f)
  return (e.source as string) || 'regex'
}

function ruleOf(f: Finding): string {
  const e = extraOf(f)
  return (e.rule as string) || (e.rule_name as string) || '-'
}

function severityHitOf(f: Finding): string {
  const e = extraOf(f)
  return (e.sensitive_level as string) || (e.level as string) || '-'
}

function ocrTextOf(f: Finding): string {
  const e = extraOf(f)
  return (e.ocr_text as string) || '-'
}

// MultiUA 展示。
interface MultiUAProbe {
  name: string
  status_code?: number
  latency_ms?: number
  failed?: boolean
  error?: string
  title?: string
  text_len?: number
  dom_similarity?: number
  sensitive_hits?: string[]
}
interface MultiUAResult {
  score?: number
  level?: string
  suggestion?: string
  end_down?: string[]
  end_diff?: string[]
  spa_suspected?: boolean
  base_score?: number
  feature_score?: number
  scenario_score?: number
  dom_similarity?: number
  probes?: MultiUAProbe[]
}
function multiUAOf(f: Finding): MultiUAResult {
  const e = extraOf(f)
  return (e.multi_ua as MultiUAResult) || {}
}

onMounted(load)
</script>

<template>
  <div class="cs-page">
    <div class="tabs">
      <button
        v-for="t in tabs"
        :key="t.key"
        class="tab"
        :class="{ active: t.key === activeTab }"
        @click="switchTab(t.key)"
      >
        {{ t.label }}
      </button>
    </div>

    <div class="toolbar">
      <select v-model="severity" class="input" @change="load">
        <option value="">全部等级</option>
        <option value="critical">严重</option>
        <option value="high">高危</option>
        <option value="medium">中危</option>
        <option value="low">低危</option>
      </select>
      <select v-model="status" class="input" @change="load">
        <option value="">全部状态</option>
        <option value="open">待处理</option>
        <option value="confirmed">已确认</option>
        <option value="closed">已关闭</option>
        <option value="ignored">已忽略</option>
      </select>
      <input v-model="keyword" class="input search" placeholder="搜索标题 / URL" @keyup.enter="load" />
      <button class="btn" @click="load">查询</button>
    </div>

    <div class="table-wrap">
      <!-- 资产发现表格 -->
      <table v-if="activeTab === 'asset_discovery'" class="table">
        <thead>
          <tr>
            <th>来源</th>
            <th>URL</th>
            <th>状态</th>
            <th>时间</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="a in assetList" :key="a.id">
            <td><span class="tag">{{ a.source_type || 'manual' }}</span></td>
            <td class="mono">{{ a.url || '-' }}</td>
            <td>{{ statusLabel(a.status) }}</td>
            <td>{{ formatTime(a.created_at) }}</td>
          </tr>
        </tbody>
      </table>
      <!-- findings 表格 -->
      <table v-else class="table">
        <thead>
          <tr>
            <th>等级</th>
            <th>标题</th>
            <th>URL</th>
            <th v-if="currentTab?.key === 'multi_ua'">评分 / 分级</th>
            <th>状态</th>
            <th>时间</th>
            <th style="width: 120px">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="f in list" :key="f.id">
            <td><span class="sev" :class="f.severity">{{ severityLabel(f.severity) }}</span></td>
            <td>
              <div class="title">{{ f.title }}</div>
              <div v-if="activeTab === 'sensitive_content'" class="sub">{{ wordOf(f) }} · 来源: {{ sourceOf(f) }}</div>
              <div v-if="activeTab === 'sensitive_info'" class="sub">{{ ruleOf(f) }} · {{ severityHitOf(f) }}</div>
              <div v-if="activeTab === 'tamper'" class="sub">
                <template v-if="f.type === 'content_integrity'">
                  变更: {{ (extraOf(f).changed_dims as string[])?.join('、') || '-' }} · 第 {{ extraOf(f).changed_count ?? 0 }} 次
                </template>
                <template v-else-if="f.type === 'external_link'">
                  {{ extraOf(f).action ?? '' }} · {{ extraOf(f).link ?? '' }}
                </template>
                <template v-else>{{ f.type }}</template>
              </div>
              <div v-if="activeTab === 'keyword'" class="sub">{{ ruleOf(f) }} · {{ severityHitOf(f) }}</div>
              <div v-if="activeTab === 'dead_link'" class="sub">状态 {{ extraOf(f).status_code ?? '-' }} · {{ extraOf(f).reason ?? '' }}</div>
              <div v-if="activeTab === 'multi_ua' && multiUAOf(f).probes?.length" class="sub">
                {{ multiUAOf(f).end_down?.length ? '宕机端: ' + (multiUAOf(f).end_down || []).join(',') : '' }}
                {{ multiUAOf(f).end_diff?.join(' / ') }}
              </div>
              <div v-if="activeTab === 'sensitive_content' && f.type === 'image_ocr'" class="sub ocr">
                OCR: {{ ocrTextOf(f).slice(0, 60) }}
              </div>
            </td>
            <td class="mono">{{ f.url || '-' }}</td>
            <td v-if="currentTab?.key === 'multi_ua'">
              <template v-if="multiUAOf(f).score !== undefined">
                <span class="score" :class="scoreClass(multiUAOf(f).score)">{{ multiUAOf(f).score }}</span>
                <span class="lv">{{ multiUAOf(f).level || '' }}</span>
              </template>
            </td>
            <td>{{ statusLabel(f.status) }}</td>
            <td>{{ formatTime(f.created_at) }}</td>
            <td>
              <button v-if="activeTab === 'tamper' || activeTab === 'multi_ua' || activeTab === 'sensitive_info'" class="link" @click="openDetail(f)">详情</button>
              <button v-if="parseEvidenceIds(f.evidence_ids).length" class="link" @click="openEvidence(f)">证据</button>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="!loading && (activeTab === 'asset_discovery' ? assetList.length === 0 : list.length === 0)" class="empty">暂无发现</div>
      <Skeleton v-if="loading" :rows="6" :cols="5" />
    </div>

    <div class="pager">
      <span>共 {{ total }} 条</span>
      <button class="btn" :disabled="page.page <= 1" @click="page.page--; load()">上一页</button>
      <span>{{ page.page }}</span>
      <button class="btn" :disabled="page.page * page.page_size >= total" @click="page.page++; load()">下一页</button>
    </div>

    <!-- 详情抽屉 -->
    <div v-if="detailVisible && detailFinding" class="modal-mask" @click.self="closeDetail">
      <div class="modal">
        <div class="modal-head">
          <span>{{ detailFinding.title }}</span>
          <button class="link" @click="closeDetail">关闭</button>
        </div>
        <div class="modal-body">
          <div class="kv"><label>类型</label><span>{{ detailFinding.type }}</span></div>
          <div class="kv"><label>URL</label><span class="mono">{{ detailFinding.url || '-' }}</span></div>
          <div class="kv"><label>描述</label><span>{{ detailFinding.description || '-' }}</span></div>
          <template v-if="activeTab === 'multi_ua'">
            <div class="kv"><label>评分</label><span>{{ multiUAOf(detailFinding).score ?? '-' }} / {{ multiUAOf(detailFinding).level || '-' }}</span></div>
            <div class="kv"><label>建议</label><span>{{ multiUAOf(detailFinding).suggestion || '-' }}</span></div>
            <div class="kv"><label>SPA</label><span>{{ multiUAOf(detailFinding).spa_suspected ? '疑似' : '否' }}</span></div>
            <div v-if="(multiUAOf(detailFinding).probes || []).length" class="probes">
              <div v-for="p in multiUAOf(detailFinding).probes || []" :key="p.name" class="probe">
                <span class="tag">{{ p.name }}</span>
                <span>状态 {{ p.status_code ?? '-' }}</span>
                <span>延迟 {{ p.latency_ms ?? '-' }}ms</span>
                <span v-if="p.failed" class="err">{{ p.error || '失败' }}</span>
                <span v-if="p.sensitive_hits?.length" class="err">敏感词: {{ p.sensitive_hits.join(',') }}</span>
              </div>
            </div>
          </template>
          <template v-else>
            <pre class="json">{{ prettyJSON(extraOf(detailFinding)) }}</pre>
          </template>
        </div>
      </div>
    </div>

    <EvidenceDrawer v-model:visible="drawerVisible" :evidence-ids="drawerIds" />
  </div>
</template>

<style scoped>
.cs-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.tabs {
  display: flex;
  gap: 6px;
  border-bottom: 1px solid var(--color-border);
  padding-bottom: 8px;
  flex-wrap: wrap;
}
.tab {
  height: 32px;
  border: 1px solid var(--color-border);
  background: #fff;
  border-radius: var(--radius-md);
  padding: 0 14px;
  cursor: pointer;
  font-size: 13px;
  color: var(--color-text-secondary);
}
.tab.active {
  border-color: var(--color-brand);
  color: var(--color-brand);
  background: var(--color-brand-light);
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
.input.search {
  width: 220px;
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
  vertical-align: top;
}
.title {
  font-weight: var(--font-weight-semibold);
  max-width: 420px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sub {
  color: var(--color-text-tertiary);
  font-size: 12px;
  margin-top: 2px;
  max-width: 420px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ocr {
  color: var(--color-text-secondary);
}
.sev {
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  font-size: 12px;
  color: #fff;
}
.sev.critical { background: var(--color-danger); }
.sev.high { background: #ff8000; }
.sev.medium { background: #ff9f0a; }
.sev.low { background: #52c41a; }
.sev.info { background: #909399; }
.mono {
  font-family: var(--font-family-mono);
  font-size: 12px;
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.link {
  border: none;
  background: none;
  color: var(--color-brand);
  cursor: pointer;
  font-size: 13px;
  padding: 0;
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
  color: var(--color-text-secondary);
  font-size: 13px;
}
.score {
  display: inline-block;
  width: 40px;
  text-align: center;
  border-radius: var(--radius-sm);
  color: #fff;
  font-weight: var(--font-weight-semibold);
  padding: 2px 0;
  margin-right: 6px;
}
.score-critical { background: var(--color-danger); }
.score-high { background: #ff8000; }
.score-medium { background: #ff9f0a; }
.score-low { background: #52c41a; }
.lv {
  font-size: 12px;
  color: var(--color-text-secondary);
}
.tag {
  display: inline-block;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  background: #f0f5ff;
  color: var(--color-info);
  font-size: 12px;
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
  width: 640px;
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
  gap: 10px;
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
.probes {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.probe {
  display: flex;
  gap: 10px;
  font-size: 12px;
  align-items: center;
  border: 1px solid var(--color-border-light);
  padding: 8px;
  border-radius: var(--radius-md);
}
.err {
  color: var(--color-danger);
}
.json {
  background: #f7f8fa;
  border-radius: var(--radius-md);
  padding: 12px;
  font-size: 12px;
  max-height: 50vh;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
