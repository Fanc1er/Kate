<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { listChannels, createChannel, updateChannel, deleteChannel, testChannel, type NotifyChannel, type WebhookConfig } from '../../api/notify'
import { toast } from '../../utils/toast'
import Skeleton from '../../components/Skeleton.vue'

const list = ref<NotifyChannel[]>([])
const loading = ref(false)
const showForm = ref(false)
const editing = ref<number | null>(null)
const saving = ref(false)

const form = reactive({
  url: '',
  secret: '',
})

function parseConfig(ch: NotifyChannel): WebhookConfig {
  try {
    return JSON.parse(ch.config) as WebhookConfig
  } catch {
    return { url: '' }
  }
}

async function load(): Promise<void> {
  loading.value = true
  try {
    list.value = await listChannels()
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  editing.value = null
  form.url = ''
  form.secret = ''
  showForm.value = true
}

function openEdit(ch: NotifyChannel): void {
  editing.value = ch.id
  const cfg = parseConfig(ch)
  form.url = cfg.url
  form.secret = cfg.secret || ''
  showForm.value = true
}

async function submit(): Promise<void> {
  if (!form.url) {
    toast.error('Webhook URL 必填')
    return
  }
  saving.value = true
  try {
    const cfg: WebhookConfig = { url: form.url, secret: form.secret || undefined }
    if (editing.value) {
      await updateChannel(editing.value, cfg, null)
      toast.success('已保存')
    } else {
      await createChannel('webhook', cfg)
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

async function toggle(ch: NotifyChannel): Promise<void> {
  try {
    await updateChannel(ch.id, null, ch.enabled === 'true' ? 'false' : 'true')
    await load()
  } catch {
    // 拦截器已提示
  }
}

async function remove(ch: NotifyChannel): Promise<void> {
  try {
    await deleteChannel(ch.id)
    toast.success('已删除')
    await load()
  } catch {
    // 拦截器已提示
  }
}

async function test(ch: NotifyChannel): Promise<void> {
  try {
    await testChannel(ch.id)
    toast.success('测试推送已发送，请查收')
  } catch {
    // 拦截器已提示
  }
}

onMounted(load)
</script>

<template>
  <div class="notify-page list-main">
    <div class="toolbar">
      <span class="hint">命中告警时向已启用的 Webhook 推送 JSON 事件（event=alert.new），含 X-CInsight-Secret 头</span>
      <span class="spacer" />
      <button class="btn primary" @click="openCreate">新建渠道</button>
    </div>

    <div v-if="showForm" class="form-card">
      <h4>{{ editing ? '编辑渠道' : '新建 Webhook 渠道' }}</h4>
      <div class="grid">
        <label>Webhook URL *<input v-model="form.url" placeholder="https://example.com/hook" /></label>
        <label>密钥（可选）<input v-model="form.secret" placeholder="X-CInsight-Secret 头的值" /></label>
      </div>
      <div class="actions">
        <button class="btn" @click="showForm = false">取消</button>
        <button class="btn primary" :disabled="saving" @click="submit">{{ saving ? '保存中...' : '保存' }}</button>
      </div>
    </div>

    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>ID</th>
            <th>类型</th>
            <th>URL</th>
            <th>状态</th>
            <th style="width: 220px">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="ch in list" :key="ch.id">
            <td>{{ ch.id }}</td>
            <td>Webhook</td>
            <td class="mono">{{ parseConfig(ch).url }}</td>
            <td>
              <span class="state" :class="ch.enabled === 'true' ? 'on' : 'off'">{{ ch.enabled === 'true' ? '已启用' : '已停用' }}</span>
            </td>
            <td class="ops">
              <button class="link" @click="test(ch)">测试</button>
              <button class="link" @click="openEdit(ch)">编辑</button>
              <button class="link" @click="toggle(ch)">{{ ch.enabled === 'true' ? '停用' : '启用' }}</button>
              <button class="link danger" @click="remove(ch)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="!loading && list.length === 0" class="empty">暂无通知渠道，点击右上角「新建渠道」接入 Webhook</div>
      <Skeleton v-if="loading" :rows="4" :cols="5" />
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
.grid label {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--color-text-secondary);
}
input {
  height: 32px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 0 10px;
  font-size: 13px;
  outline: none;
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
  word-break: break-all;
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
