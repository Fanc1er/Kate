<script setup lang="ts">
import { ref, reactive, onMounted, toRef } from 'vue'
import {
  listAssets,
  createAsset,
  updateAsset,
  deleteAsset,
  batchDelete,
  batchScan,
  batchGroup,
  getAssetProfile,
  getAssetHistory,
  listGroups,
  listWechat,
  createWechat,
  updateWechat,
  deleteWechat,
  type Asset,
  type AssetProfile,
  type AssetHistoryItem,
  type WechatAsset,
} from '../../api/asset'
import { formatTime } from '../../utils/format'
import { toast, confirmDialog } from '../../utils/toast'
import Skeleton from '../../components/Skeleton.vue'
import { useQuerySync } from '../../composables/useQuerySync'

const list = ref<Asset[]>([])
const total = ref(0)
const loading = ref(false)
const keyword = ref('')
const groupName = ref('')
const selected = ref<number[]>([])
const groups = ref<string[]>([])

const page = reactive({ page: 1, page_size: 20 })

useQuerySync(
  [
    ['keyword', keyword],
    ['group_name', groupName],
    ['page', toRef(page, 'page')],
    ['page_size', toRef(page, 'page_size')],
  ],
  { numberKeys: ['page', 'page_size'], defaults: { page: 1, page_size: 20 } },
)

const showCreate = ref(false)
const showGroupDialog = ref(false)
const groupInput = ref('')
const editing = ref<Asset | null>(null)
const form = reactive({ name: '', url: '', group_name: '' })
const saving = ref(false)
const error = ref('')

const showProfile = ref(false)
const profile = ref<AssetProfile | null>(null)
const profileLoading = ref(false)
const history = ref<AssetHistoryItem[]>([])
const historyLoading = ref(false)
const historyTab = ref<'profile' | 'history'>('profile')

// 微信公众号资产。
const wechatTab = ref(false)
const wechatList = ref<WechatAsset[]>([])
const wechatTotal = ref(0)
const wechatPage = ref(1)
const wechatForm = reactive({ name: '', wechat_id: '', avatar_url: '', intro: '', verify_status: '', fans_count: 0, article_count: 0 })
const showWechatCreate = ref(false)
const wechatEditing = ref<WechatAsset | null>(null)
const wechatSaving = ref(false)
const wechatError = ref('')

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listAssets({ page: page.page, page_size: page.page_size, keyword: keyword.value, group_name: groupName.value })
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}


async function loadGroups(): Promise<void> {
  try {
    const res = await listGroups()
    groups.value = res.map((g) => g.group_name)
  } catch {
    // 忽略
  }
}

function openCreate(): void {
  editing.value = null
  Object.assign(form, { name: '', url: '', group_name: '' })
  error.value = ''
  showCreate.value = true
}

function openEdit(a: Asset): void {
  editing.value = a
  Object.assign(form, { name: a.name, url: a.url, group_name: a.group_name })
  error.value = ''
  showCreate.value = true
}

async function save(): Promise<void> {
  if (!form.name || !form.url) {
    error.value = '名称与 URL 必填'
    return
  }
  saving.value = true
  error.value = ''
  try {
    if (editing.value) {
      await updateAsset(editing.value.id, { ...form })
      toast.success('资产已更新')
    } else {
      await createAsset({ ...form })
      toast.success('资产已创建')
    }
    showCreate.value = false
    await load()
    await loadGroups()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

async function remove(a: Asset): Promise<void> {
  if (!confirmDialog(`确认删除资产「${a.name}」？`)) return
  try {
    await deleteAsset(a.id)
    toast.success('资产已删除')
    await load()
  } catch {
    // 拦截器已提示
  }
}

async function doBatchDelete(): Promise<void> {
  if (selected.value.length === 0) return
  if (!confirmDialog(`确认删除选中的 ${selected.value.length} 个资产？`)) return
  try {
    await batchDelete(selected.value)
    selected.value = []
    toast.success('批量删除成功')
    await load()
  } catch {
    // 拦截器已提示
  }
}

async function doBatchScan(): Promise<void> {
  if (selected.value.length === 0) return
  try {
    const res = await batchScan(selected.value)
    toast.success(`已加入扫描 ${res.started} 个任务`)
    selected.value = []
  } catch {
    // 拦截器已提示
  }
}

async function scanOne(a: Asset): Promise<void> {
  try {
    const res = await batchScan([a.id])
    toast.success(`已加入扫描：${res.started} 个任务`)
  } catch {
    // 拦截器已提示
  }
}

async function copyUrl(a: Asset): Promise<void> {
  try {
    await navigator.clipboard.writeText(a.url)
    toast.success('URL 已复制')
  } catch {
    toast.warning('复制失败，请手动复制')
  }
}

function openGroupDialog(): void {
  groupInput.value = ''
  showGroupDialog.value = true
}

async function doBatchGroup(): Promise<void> {
  if (selected.value.length === 0 || !groupInput.value) return
  try {
    await batchGroup(selected.value, groupInput.value)
    showGroupDialog.value = false
    selected.value = []
    toast.success('批量分组成功')
    await load()
    await loadGroups()
  } catch {
    // 拦截器已提示
  }
}

async function openProfile(a: Asset): Promise<void> {
  showProfile.value = true
  historyTab.value = 'profile'
  profileLoading.value = true
  profile.value = null
  try {
    profile.value = await getAssetProfile(a.id)
  } catch {
    // 拦截器已提示
  } finally {
    profileLoading.value = false
  }
}

async function openHistory(a: Asset): Promise<void> {
  showProfile.value = true
  historyTab.value = 'history'
  historyLoading.value = true
  history.value = []
  try {
    history.value = await getAssetHistory(a.id)
  } catch {
    // 拦截器已提示
  } finally {
    historyLoading.value = false
  }
}

function toggleSelect(id: number): void {
  const idx = selected.value.indexOf(id)
  if (idx >= 0) selected.value.splice(idx, 1)
  else selected.value.push(id)
}

// ---- 微信公众号 ----
async function loadWechat(): Promise<void> {
  try {
    const res = await listWechat(wechatPage.value, 10)
    wechatList.value = res.list
    wechatTotal.value = res.total
  } catch {
    // 拦截器已提示
  }
}

function openWechatCreate(): void {
  wechatEditing.value = null
  Object.assign(wechatForm, { name: '', wechat_id: '', avatar_url: '', intro: '', verify_status: '', fans_count: 0, article_count: 0 })
  wechatError.value = ''
  showWechatCreate.value = true
}

function openWechatEdit(w: WechatAsset): void {
  wechatEditing.value = w
  Object.assign(wechatForm, {
    name: w.name, wechat_id: w.wechat_id, avatar_url: w.avatar_url, intro: w.intro,
    verify_status: w.verify_status, fans_count: w.fans_count, article_count: w.article_count,
  })
  wechatError.value = ''
  showWechatCreate.value = true
}

async function saveWechat(): Promise<void> {
  if (!wechatForm.name || !wechatForm.wechat_id) {
    wechatError.value = '公众号名与微信号必填'
    return
  }
  wechatSaving.value = true
  wechatError.value = ''
  try {
    if (wechatEditing.value) {
      await updateWechat(wechatEditing.value.id, { ...wechatForm })
      toast.success('公众号已更新')
    } else {
      await createWechat({ ...wechatForm })
      toast.success('公众号已添加')
    }
    showWechatCreate.value = false
    await loadWechat()
  } catch (e) {
    wechatError.value = e instanceof Error ? e.message : String(e)
  } finally {
    wechatSaving.value = false
  }
}

async function removeWechat(w: WechatAsset): Promise<void> {
  if (!confirmDialog(`确认删除公众号「${w.name}」？`)) return
  try {
    await deleteWechat(w.id)
    toast.success('公众号已删除')
    await loadWechat()
  } catch {
    // 拦截器已提示
  }
}

onMounted(() => {
  void load()
  void loadGroups()
})
</script>

<template>
  <div class="asset-page list-main">

      <div class="toolbar">
        <select v-model="groupName" class="filter-input" @change="page.page = 1; load()">
          <option value="">全部分组</option>
          <option v-for="g in groups" :key="g" :value="g">{{ g }}</option>
        </select>
        <span class="spacer" />
        <button class="btn" :class="{ 'tab-active': !wechatTab }" @click="wechatTab = false">Web 资产</button>
        <button class="btn" :class="{ 'tab-active': wechatTab }" @click="wechatTab = true; loadWechat()">微信公众号</button>
        <span class="divider" />
        <template v-if="!wechatTab">
          <input v-model="keyword" class="input search" placeholder="搜索名称 / URL" @keyup.enter="page.page = 1; load()" />
          <button class="btn" @click="load">查询</button>
          <span class="spacer" />
        <button class="btn primary" @click="openCreate">新增资产</button>
        <button class="btn" :disabled="selected.length === 0" @click="doBatchScan">批量扫描</button>
        <button class="btn" :disabled="selected.length === 0" @click="openGroupDialog">批量分组</button>
        <button class="btn danger" :disabled="selected.length === 0" @click="doBatchDelete">批量删除</button>
      </template>
      <template v-else>
        <span class="spacer" />
        <button class="btn primary" @click="openWechatCreate">新增公众号</button>
      </template>
    </div>

    <div v-if="!wechatTab" class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th style="width: 40px"></th>
            <th>名称</th>
            <th>URL</th>
            <th>分组</th>
            <th>状态</th>
            <th>风险等级</th>
            <th>更新时间</th>
            <th style="width: 200px">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="a in list" :key="a.id">
            <td><input type="checkbox" :checked="selected.includes(a.id)" @change="toggleSelect(a.id)" /></td>
            <td>{{ a.name }}</td>
            <td class="mono">{{ a.url }}</td>
            <td>{{ a.group_name || '-' }}</td>
            <td>{{ a.status }}</td>
            <td>{{ a.risk_level || '-' }}</td>
            <td>{{ formatTime(a.updated_at) }}</td>
            <td>
              <button class="link" @click="scanOne(a)">立即扫描</button>
              <button class="link" @click="openProfile(a)">画像</button>
              <button class="link" @click="openHistory(a)">变更</button>
              <button class="link" @click="copyUrl(a)">复制 URL</button>
              <button class="link" @click="openEdit(a)">编辑</button>
              <button class="link danger" @click="remove(a)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="!loading && list.length === 0" class="empty">暂无资产，点击「新增资产」开始监测</div>
      <Skeleton v-if="loading" :rows="6" :cols="6" />
    </div>

    <div v-else class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>公众号名</th>
            <th>微信号</th>
            <th>头像</th>
            <th>粉丝数</th>
            <th>认证状态</th>
            <th>文章数</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="w in wechatList" :key="w.id">
            <td>{{ w.name }}</td>
            <td class="mono">{{ w.wechat_id }}</td>
            <td>
              <img v-if="w.avatar_url" :src="w.avatar_url" class="avatar" alt="头像" />
              <span v-else>-</span>
            </td>
            <td>{{ w.fans_count }}</td>
            <td>{{ w.verify_status || '-' }}</td>
            <td>{{ w.article_count }}</td>
            <td>
              <button class="link" @click="openWechatEdit(w)">编辑</button>
              <button class="link danger" @click="removeWechat(w)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="wechatList.length === 0" class="empty">暂无公众号资产</div>
      <div class="pager">
        <span>共 {{ wechatTotal }} 条</span>
        <button class="btn" :disabled="wechatPage <= 1" @click="wechatPage--; loadWechat()">上一页</button>
        <span>{{ wechatPage }}</span>
        <button class="btn" :disabled="wechatPage * 10 >= wechatTotal" @click="wechatPage++; loadWechat()">下一页</button>
      </div>
    </div>

    <div v-if="!wechatTab" class="pager">
      <span>共 {{ total }} 条</span>
      <button class="btn" :disabled="page.page <= 1" @click="page.page--; load()">上一页</button>
      <span>{{ page.page }}</span>
      <button class="btn" :disabled="page.page * page.page_size >= total" @click="page.page++; load()">下一页</button>
    </div>


    <div v-if="showCreate" class="modal-mask" @click.self="showCreate = false">
      <div class="modal">
        <h3>{{ editing ? '编辑资产' : '新增资产' }}</h3>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="field">
          <label>名称</label>
          <input v-model="form.name" class="input" />
        </div>
        <div class="field">
          <label>URL</label>
          <input v-model="form.url" class="input" placeholder="https://example.com" />
        </div>
        <div class="field">
          <label>分组</label>
          <input v-model="form.group_name" class="input" placeholder="可选" />
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showCreate = false">取消</button>
          <button class="btn primary" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存' }}</button>
        </div>
      </div>
    </div>

    <div v-if="showGroupDialog" class="modal-mask" @click.self="showGroupDialog = false">
      <div class="modal">
        <h3>批量分组（{{ selected.length }} 项）</h3>
        <div class="field">
          <label>分组名</label>
          <input v-model="groupInput" class="input" placeholder="输入分组名" />
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showGroupDialog = false">取消</button>
          <button class="btn primary" @click="doBatchGroup">确认</button>
        </div>
      </div>
    </div>

    <div v-if="showProfile" class="modal-mask" @click.self="showProfile = false">
      <div class="modal wide">
        <h3>资产详情</h3>
        <div class="tabs">
          <button class="link" :class="{ 'tab-active': historyTab === 'profile' }" @click="historyTab = 'profile'">画像</button>
          <button class="link" :class="{ 'tab-active': historyTab === 'history' }" @click="historyTab = 'history'">变更历史</button>
        </div>

        <template v-if="historyTab === 'profile'">
          <p v-if="profileLoading">加载中…</p>
          <div v-else class="profile-grid">
            <div class="p-item"><span class="p-label">技术栈</span><span>{{ profile?.tech_stack || '-' }}</span></div>
            <div class="p-item"><span class="p-label">ICP 备案</span><span>{{ profile?.icp || '-' }}</span></div>
            <div class="p-item"><span class="p-label">SSL 到期</span><span>{{ profile?.ssl_expire_at ? formatTime(profile.ssl_expire_at) : '-' }}</span></div>
            <div class="p-item">
              <span class="p-label">子域名</span>
              <span class="mono chips">{{ (profile?.subdomains && profile.subdomains.length ? profile.subdomains.join(', ') : '-') }}</span>
            </div>
            <div class="p-item">
              <span class="p-label">开放端口</span>
              <span class="mono chips">{{ (profile?.ports && profile.ports.length ? profile.ports.join(', ') : '-') }}</span>
            </div>
          </div>
        </template>

        <template v-else>
          <p v-if="historyLoading">加载中…</p>
          <table v-else class="table history-table">
            <thead>
              <tr>
                <th>时间</th>
                <th>字段</th>
                <th>变更前</th>
                <th>变更后</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="h in history" :key="h.id">
                <td>{{ formatTime(h.changed_at) }}</td>
                <td>{{ h.field }}</td>
                <td class="mono">{{ h.before || '-' }}</td>
                <td class="mono">{{ h.after || '-' }}</td>
              </tr>
            </tbody>
          </table>
          <div v-if="!historyLoading && history.length === 0" class="empty">暂无变更记录</div>
        </template>

        <div class="modal-actions">
          <button class="btn" @click="showProfile = false">关闭</button>
        </div>
      </div>
    </div>

    <div v-if="showWechatCreate" class="modal-mask" @click.self="showWechatCreate = false">
      <div class="modal">
        <h3>{{ wechatEditing ? '编辑公众号' : '新增公众号' }}</h3>
        <p v-if="wechatError" class="error">{{ wechatError }}</p>
        <div class="field">
          <label>公众号名</label>
          <input v-model="wechatForm.name" class="input" />
        </div>
        <div class="field">
          <label>微信号</label>
          <input v-model="wechatForm.wechat_id" class="input" />
        </div>
        <div class="field">
          <label>头像 URL</label>
          <input v-model="wechatForm.avatar_url" class="input" />
        </div>
        <div class="field">
          <label>简介</label>
          <input v-model="wechatForm.intro" class="input" />
        </div>
        <div class="field">
          <label>认证状态</label>
          <select v-model="wechatForm.verify_status" class="input">
            <option value="">未选择</option>
            <option value="verified">已认证</option>
            <option value="unverified">未认证</option>
          </select>
        </div>
        <div class="field-row">
          <div class="field">
            <label>粉丝数</label>
            <input v-model.number="wechatForm.fans_count" type="number" class="input" />
          </div>
          <div class="field">
            <label>文章数</label>
            <input v-model.number="wechatForm.article_count" type="number" class="input" />
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showWechatCreate = false">取消</button>
          <button class="btn primary" :disabled="wechatSaving" @click="saveWechat">{{ wechatSaving ? '保存中…' : '保存' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.input {
  height: 34px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 0 10px;
  outline: none;
}
.search {
  width: 220px;
}
.select {
  width: 140px;
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
.btn.danger {
  color: var(--color-danger);
  border-color: var(--color-danger);
}
.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.link {
  border: none;
  background: transparent;
  color: var(--color-brand);
  cursor: pointer;
  font-size: 13px;
  margin-right: 8px;
}
.link.danger {
  color: var(--color-danger);
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
  max-height: 80vh;
  overflow-y: auto;
}
.modal.wide {
  width: 640px;
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
.profile-json {
  background: #f7f8fa;
  padding: 12px;
  border-radius: var(--radius-md);
  font-size: 12px;
  max-height: 50vh;
  overflow: auto;
  white-space: pre-wrap;
}
.divider {
  width: 1px;
  height: 20px;
  background: var(--color-border);
  margin: 0 4px;
}
.tab-active {
  background: var(--color-brand);
  color: #fff;
  border-color: var(--color-brand);
}
.avatar {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-md);
  object-fit: cover;
}
.field-row {
  display: flex;
  gap: 12px;
}
.field-row .field {
  flex: 1;
}
.tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 12px;
  border-bottom: 1px solid var(--color-border-light);
  padding-bottom: 8px;
}
.profile-grid {
  display: flex;
  flex-direction: column;
  gap: 10px;
  font-size: 13px;
}
.p-item {
  display: flex;
  gap: 12px;
}
.p-label {
  width: 80px;
  color: var(--color-text-tertiary);
  flex: none;
}
.chips {
  word-break: break-all;
}
.history-table {
  max-height: 50vh;
  overflow: auto;
  display: block;
}
</style>
