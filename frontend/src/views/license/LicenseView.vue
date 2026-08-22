<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useLicenseStore } from '../../stores/license'
import { formatTime } from '../../utils/format'

const router = useRouter()
const license = useLicenseStore()

const importing = ref(false)
const error = ref('')
const copied = ref(false)

const statusLabel: Record<string, string> = {
  missing: '未导入授权',
  invalid: '授权文件无效',
  not_yet_active: '授权尚未生效',
  expired: '授权已过期',
  machine_mismatch: '机器不匹配',
  valid: '授权有效',
}

const currentLabel = computed(() => statusLabel[license.status] ?? license.status)
const nearExpiry = computed(() => license.daysRemaining > 0 && license.daysRemaining <= 30)

async function copyCode(): Promise<void> {
  if (!license.machineCode) return
  try {
    await navigator.clipboard.writeText(license.machineCode)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 1500)
  } catch {
    // 忽略复制失败
  }
}

async function onFileChange(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  error.value = ''
  importing.value = true
  try {
    const content = await readFileText(file)
    await license.importLicense(content)
    if (license.status === 'valid') {
      router.replace('/login')
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    importing.value = false
    input.value = ''
  }
}

function readFileText(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result))
    reader.onerror = () => reject(new Error('读取文件失败'))
    reader.readAsText(file)
  })
}

onMounted(() => {
  void license.fetchMachineCode()
})
</script>

<template>
  <div class="license-page">
    <div class="license-card">
      <h1 class="title">CInsight</h1>
      <p class="subtitle">离线授权</p>

      <div class="status-row">
        <span class="status-label">当前状态</span>
        <span class="status-value" :class="license.status">{{ currentLabel }}</span>
      </div>

      <div class="section">
        <div class="section-title">机器码</div>
        <div class="machine-code">{{ license.machineCode || '加载中…' }}</div>
        <div v-if="license.source" class="source">特征来源：{{ license.source }}</div>
        <button class="btn" :disabled="!license.machineCode" @click="copyCode">
          {{ copied ? '已复制' : '复制机器码' }}
        </button>
      </div>

      <div class="section">
        <div class="section-title">导入授权文件</div>
        <label class="upload">
          <input type="file" accept=".lic,.json,.txt" :disabled="importing" @change="onFileChange" />
          <span class="btn primary">{{ importing ? '导入中…' : '选择 .lic 文件导入' }}</span>
        </label>
        <p v-if="error" class="error">{{ error }}</p>
      </div>

      <div v-if="license.status === 'not_yet_active' && license.notBefore" class="notice">
        授权将在 {{ formatTime(license.notBefore) }} 生效。
      </div>
      <div v-else-if="nearExpiry" class="notice">
        授权将于 {{ formatTime(license.notAfter) }} 到期，剩余 {{ license.daysRemaining }} 天，请及时续期。
      </div>
      <div v-else-if="license.status === 'expired'" class="notice">
        授权已于 {{ formatTime(license.notAfter) }} 到期，请重新导入有效授权。
      </div>
      <div v-else-if="license.status === 'valid'" class="notice success">
        授权有效，有效期至 {{ formatTime(license.notAfter) }}。
      </div>
    </div>
  </div>
</template>

<style scoped>
.license-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1d2129 0%, #3370ff 100%);
  padding: 24px;
}
.license-card {
  width: 520px;
  background: #fff;
  border-radius: 12px;
  padding: 40px 32px;
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.2);
}
.title {
  margin: 0 0 4px;
  font-size: 24px;
  color: #1d2129;
  text-align: center;
}
.subtitle {
  margin: 0 0 24px;
  color: #86909c;
  text-align: center;
  font-size: 13px;
}
.status-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  background: #f7f8fa;
  border-radius: 8px;
  margin-bottom: 20px;
}
.status-label {
  color: #4e5969;
  font-size: 13px;
}
.status-value {
  font-size: 14px;
  font-weight: 600;
}
.status-value.valid {
  color: #00b42a;
}
.status-value.missing,
.status-value.invalid,
.status-value.expired,
.status-value.machine_mismatch {
  color: #d03050;
}
.status-value.not_yet_active {
  color: #ff7d00;
}
.section {
  margin-bottom: 20px;
}
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: #1d2129;
  margin-bottom: 8px;
}
.machine-code {
  font-family: ui-monospace, monospace;
  font-size: 12px;
  word-break: break-all;
  background: #f7f8fa;
  border: 1px solid #e5e6eb;
  border-radius: 6px;
  padding: 10px;
  margin-bottom: 8px;
}
.source {
  color: #86909c;
  font-size: 12px;
  margin-bottom: 8px;
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
  display: inline-block;
  line-height: 34px;
}
.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
.upload input {
  display: none;
}
.error {
  color: #d03050;
  font-size: 13px;
  margin: 8px 0 0;
}
.notice {
  background: #fff7e6;
  color: #d46b08;
  border-radius: 8px;
  padding: 10px 14px;
  font-size: 13px;
}
.notice.success {
  background: #e8ffea;
  color: #00b42a;
}
</style>
