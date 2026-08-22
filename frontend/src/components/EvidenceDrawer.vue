<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { getEvidence, getEvidenceFile } from '../api/finding'
import type { EvidenceDetail, EvidenceFile } from '../api/finding'
import { formatBytes, formatTime } from '../utils/format'
import { sanitizeHtml } from '../utils/sanitize'
import { toast } from '../utils/toast'

const props = defineProps<{
  evidenceIds: number[] | null
  visible: boolean
}>()

const emit = defineEmits<{ (e: 'update:visible', v: boolean): void }>()

const loading = ref(false)
const items = ref<EvidenceDetail[]>([])
const activeIndex = ref(0)
const errorMsg = ref('')
const activeFile = ref<EvidenceFile | null>(null)
const fileContent = ref('')
const fileLoading = ref(false)
const hashMismatch = ref(false)

function parseIds(raw: number[] | null): number[] {
  if (!raw) return []
  return raw.filter((n) => typeof n === 'number' && n > 0)
}

async function load(): Promise<void> {
  const ids = parseIds(props.evidenceIds)
  if (ids.length === 0) {
    items.value = []
    return
  }
  loading.value = true
  errorMsg.value = ''
  items.value = []
  activeIndex.value = 0
  activeFile.value = null
  fileContent.value = ''
  hashMismatch.value = false
  try {
    const results = await Promise.all(ids.map((id) => getEvidence(id)))
    items.value = results
    await selectEvidenceContent(results[0])
  } catch (e) {
    errorMsg.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

const current = computed<EvidenceDetail | undefined>(() => items.value[activeIndex.value])

function isScreenshotFile(f: EvidenceFile): boolean {
  return f.kind === 'screenshot' || (f.mime_type || '').startsWith('image/')
}

const viewType = computed<'reqresp' | 'html' | 'screenshot' | 'text'>(() => {
  const f = activeFile.value
  if (!f) return 'text'
  if (isScreenshotFile(f)) return 'screenshot'
  if (f.kind === 'html' || f.mime_type === 'text/html') return 'html'
  if (f.kind === 'req' || f.kind === 'resp' || f.kind === 'har' || f.kind === 'text') return 'reqresp'
  return 'text'
})

const reqFiles = computed<EvidenceFile[]>(() => (current.value?.files || []).filter((f) => f.kind === 'req'))
const respFiles = computed<EvidenceFile[]>(() => (current.value?.files || []).filter((f) => f.kind === 'resp'))

async function selectEvidenceContent(ev: EvidenceDetail): Promise<void> {
  activeFile.value = ev.files[0] || null
  fileContent.value = ''
  hashMismatch.value = false
  const first = ev.files[0]
  if (first && !isScreenshotFile(first)) {
    fileLoading.value = true
    try {
      fileContent.value = await getEvidenceFile(ev.evidence.id).then((b) => b.text())
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e)
      if (/tamper|篡改/i.test(msg)) {
        hashMismatch.value = true
      }
      fileContent.value = ''
    } finally {
      fileLoading.value = false
    }
  }
}

function pickFile(f: EvidenceFile): void {
  const ev = current.value
  if (!ev) return
  activeFile.value = f
  if (!isScreenshotFile(f)) {
    fileLoading.value = true
    hashMismatch.value = false
    getEvidenceFile(ev.evidence.id)
      .then(async (b) => {
        fileContent.value = await b.text()
      })
      .catch(() => {
        fileContent.value = ''
      })
      .finally(() => {
        fileLoading.value = false
      })
  }
}

function selectEvidence(i: number): void {
  activeIndex.value = i
  const it = items.value[i]
  if (it) {
    void selectEvidenceContent(it)
  }
}

function doDownload(): void {
  const it = current.value
  if (!it) return
  void getEvidenceFile(it.evidence.id).then((blob) => {
    const url = window.URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `evidence-${it.evidence.id}.${it.evidence.mime_type.split('/')[1] ?? 'bin'}`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    window.URL.revokeObjectURL(url)
  })
}

function screenshotUrl(): string {
  const ev = current.value
  return ev ? `/api/v1/evidence/${ev.evidence.id}/file` : ''
}

function copyContent(): void {
  if (!fileContent.value) return
  void navigator.clipboard.writeText(fileContent.value).then(() => {
    toast.success('内容已复制')
  })
}

watch(
  () => props.visible,
  (v) => {
    if (v) void load()
  },
)

function close(): void {
  emit('update:visible', false)
}
</script>

<template>
  <div v-if="visible" class="drawer-mask" @click.self="close">
    <div class="drawer">
      <div class="drawer-header">
        <span>证据抽屉</span>
        <button class="close" @click="close">×</button>
      </div>
      <div class="drawer-body">
        <p v-if="loading">加载中…</p>
        <p v-else-if="errorMsg" class="error">{{ errorMsg }}</p>
        <template v-else-if="items.length > 0">
          <div class="tabs">
            <button
              v-for="(it, i) in items"
              :key="it.evidence.id"
              class="tab"
              :class="{ active: i === activeIndex }"
              @click="selectEvidence(i)"
            >
              证据 #{{ i + 1 }}
            </button>
          </div>

          <div v-if="current" class="detail">
            <div class="meta">
              <span>SHA-256: <code>{{ current.evidence.sha256.slice(0, 16) }}…</code></span>
              <span>{{ formatBytes(current.evidence.size) }}</span>
              <span>{{ formatTime(current.evidence.created_at) }}</span>
              <button class="btn" @click="doDownload">下载</button>
            </div>

            <div v-if="current.files.length > 0" class="file-list">
              <button
                v-for="f in current.files"
                :key="f.id"
                class="file-chip"
                :class="{ active: activeFile?.id === f.id }"
                @click="pickFile(f)"
              >
                {{ f.kind }}
                <span v-if="f.kind === 'req' || f.kind === 'resp'" class="chip-mono">{{ formatBytes(f.size) }}</span>
              </button>
            </div>

            <div v-if="hashMismatch" class="tamper-banner">证据 Hash 校验失败，内容可能被篡改</div>

            <div v-if="viewType === 'screenshot' && activeFile" class="screenshot-wrap">
              <img :src="screenshotUrl()" class="screenshot" alt="截图" />
            </div>

            <div v-else-if="viewType === 'reqresp' && activeFile">
              <div class="split">
                <div class="split-pane">
                  <h4>请求 (Req)</h4>
                  <pre v-if="reqFiles.length" class="code">{{ fileContent }}</pre>
                  <p v-else class="hint">无 Req 内容</p>
                </div>
                <div class="split-pane">
                  <h4>响应 (Resp)</h4>
                  <pre v-if="respFiles.length" class="code">{{ fileContent }}</pre>
                  <p v-else class="hint">无 Resp 内容</p>
                </div>
              </div>
            </div>

            <div v-else-if="viewType === 'html' && activeFile" class="html-wrap">
              <div class="html-toolbar">
                <span class="hint">HTML 已通过 DOMPurify 白名单净化</span>
                <button class="btn" @click="copyContent">复制源码</button>
              </div>
              <div v-if="fileContent" class="html-lines">
                <template v-for="(line, idx) in fileContent.split('\n')" :key="idx">
                  <div class="code-line"><span class="line-no">{{ idx + 1 }}</span><span class="line-text" v-html="sanitizeHtml(line)"></span></div>
                </template>
              </div>
              <p v-else class="hint">加载中…</p>
            </div>

            <div v-else-if="viewType === 'text' && activeFile" class="text-wrap">
              <pre class="code">{{ fileContent }}</pre>
            </div>

            <p v-else class="hint">无文件内容（仅元数据）</p>
          </div>
        </template>
        <p v-else>无证据</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.drawer-mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  z-index: 2000;
}
.drawer {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 70%;
  max-width: 900px;
  background: #fff;
  display: flex;
  flex-direction: column;
}
.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  border-bottom: 1px solid var(--color-border);
  font-weight: var(--font-weight-semibold);
}
.close {
  border: none;
  background: transparent;
  font-size: 20px;
  cursor: pointer;
}
.drawer-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 20px;
}
.tabs {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
.tab {
  border: 1px solid var(--color-border);
  background: #fff;
  border-radius: var(--radius-sm);
  padding: 4px 12px;
  cursor: pointer;
  font-size: 13px;
}
.tab.active {
  border-color: var(--color-brand);
  color: var(--color-brand);
}
.meta {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: var(--color-text-tertiary);
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.btn {
  border: 1px solid var(--color-border);
  background: #fff;
  border-radius: var(--radius-sm);
  padding: 2px 10px;
  cursor: pointer;
  font-size: 12px;
}
.file-list {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
.file-chip {
  border: 1px solid var(--color-border);
  background: #fff;
  border-radius: var(--radius-lg);
  padding: 2px 12px;
  cursor: pointer;
  font-size: 12px;
}
.file-chip.active {
  border-color: var(--color-brand);
  color: var(--color-brand);
  background: var(--color-brand-light);
}
.chip-mono {
  margin-left: 6px;
  color: var(--color-text-tertiary);
}
.tamper-banner {
  background: #ffece8;
  color: var(--color-danger);
  border-radius: var(--radius-md);
  padding: 10px 12px;
  margin-bottom: 12px;
  font-size: 13px;
}
.split {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.split-pane h4 {
  margin: 0 0 8px;
  font-size: 13px;
}
.code {
  background: #f7f8fa;
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-md);
  padding: 12px;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 50vh;
  overflow: auto;
  font-family: var(--font-family-mono);
}
.screenshot-wrap {
  background: #f7f8fa;
  border-radius: var(--radius-md);
  padding: 12px;
}
.screenshot {
  max-width: 100%;
  border-radius: var(--radius-md);
}
.html-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}
.html-lines {
  background: #f7f8fa;
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border-light);
  max-height: 50vh;
  overflow: auto;
  font-family: var(--font-family-mono);
}
.code-line {
  display: flex;
  font-size: 12px;
  line-height: 1.6;
}
.line-no {
  width: 48px;
  flex: none;
  text-align: right;
  padding-right: 12px;
  color: #c0c4cc;
  user-select: none;
  border-right: 1px solid var(--color-border-light);
  margin-right: 12px;
}
.line-text {
  padding-right: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}
.hint {
  color: var(--color-text-tertiary);
  font-size: 13px;
}
.error {
  color: var(--color-danger);
  font-size: 13px;
}
.text-wrap {
  max-height: 55vh;
  overflow: auto;
}
</style>
