<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { listMembers, inviteMember, setMemberRole, setMemberStatus, removeMember, type Member } from '../../api/admin'
import { formatTime, statusLabel } from '../../utils/format'
import { ROLE_LABELS } from '../../config/permissions'
import { toast, confirmDialog } from '../../utils/toast'

const list = ref<Member[]>([])
const total = ref(0)
const loading = ref(false)
const page = reactive({ page: 1, page_size: 20 })

const showInvite = ref(false)
const form = reactive({ email: '', role: 'user' })
const saving = ref(false)
const error = ref('')

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listMembers({ page: page.page, page_size: page.page_size })
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

async function invite(): Promise<void> {
  if (!form.email) {
    error.value = '请输入邮箱'
    return
  }
  saving.value = true
  error.value = ''
  try {
    await inviteMember({ email: form.email, role: form.role })
    showInvite.value = false
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

async function changeRole(m: Member, role: string): Promise<void> {
  try {
    await setMemberRole(m.id, role)
    toast.success('角色已更新')
    await load()
  } catch {
    // 拦截器已提示
  }
}

async function toggleStatus(m: Member): Promise<void> {
  const next = m.status === 'active' ? 'disabled' : 'active'
  try {
    await setMemberStatus(m.id, next)
    toast.success(next === 'active' ? '成员已启用' : '成员已禁用')
    await load()
  } catch {
    // 拦截器已提示
  }
}

async function doRemove(m: Member): Promise<void> {
  if (!confirmDialog(`确认移除成员「${m.username}」？`)) return
  try {
    await removeMember(m.id)
    toast.success('成员已移除')
    await load()
  } catch {
    // 拦截器已提示
  }
}

onMounted(load)
</script>

<template>
  <div class="members-page">
    <div class="toolbar">
      <h2>用户管理</h2>
      <span class="spacer" />
      <button class="btn primary" @click="showInvite = true">邀请成员</button>
    </div>

    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>用户名</th>
            <th>邮箱</th>
            <th>角色</th>
            <th>状态</th>
            <th>加入时间</th>
            <th style="width: 200px">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="m in list" :key="m.id">
            <td>{{ m.username }}</td>
            <td>{{ m.email }}</td>
            <td>
              <select :value="m.role" class="role-select" @change="changeRole(m, ($event.target as HTMLSelectElement).value)">
                <option v-for="(label, val) in ROLE_LABELS" :key="val" :value="val">{{ label }}</option>
              </select>
            </td>
            <td>{{ statusLabel(m.status) }}</td>
            <td>{{ formatTime(m.created_at) }}</td>
            <td>
              <button class="link" @click="toggleStatus(m)">{{ m.status === 'active' ? '禁用' : '启用' }}</button>
              <button class="link danger" @click="doRemove(m)">移除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="!loading && list.length === 0" class="empty">暂无成员</div>
    </div>

    <div v-if="showInvite" class="modal-mask" @click.self="showInvite = false">
      <div class="modal">
        <h3>邀请成员</h3>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="field">
          <label>邮箱</label>
          <input v-model="form.email" type="email" class="input" placeholder="name@example.com" />
        </div>
        <div class="field">
          <label>角色</label>
          <select v-model="form.role" class="input">
            <option v-for="(label, val) in ROLE_LABELS" :key="val" :value="val">{{ label }}</option>
          </select>
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showInvite = false">取消</button>
          <button class="btn primary" :disabled="saving" @click="invite">{{ saving ? '邀请中…' : '邀请' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.members-page {
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
.link {
  border: none;
  background: transparent;
  color: #3370ff;
  cursor: pointer;
  font-size: 13px;
  margin-right: 8px;
}
.link.danger {
  color: #d03050;
}
.role-select {
  border: 1px solid #e5e6eb;
  border-radius: 4px;
  padding: 4px 6px;
  font-size: 13px;
}
.table-wrap {
  background: #fff;
  border-radius: 8px;
  padding: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
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
  border-bottom: 1px solid #f2f3f5;
}
.empty {
  text-align: center;
  color: #86909c;
  padding: 40px 0;
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
