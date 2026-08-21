<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'

const router = useRouter()
const auth = useAuthStore()

const loading = ref(false)
const error = ref('')
const activeOrgId = ref<number | null>(null)

const cards = computed(() => {
  const list: Array<{ org_id: number; name: string; role: string }> = []
  if (auth.isSuperAdmin) {
    list.push({ org_id: 0, name: '平台管理', role: 'super_admin' })
  }
  for (const o of auth.organizations) {
    list.push({ org_id: o.org_id, name: o.name, role: o.role })
  }
  return list
})

async function pick(orgId: number): Promise<void> {
  if (orgId === 0 && auth.isSuperAdmin) {
    router.replace('/platform')
    return
  }
  loading.value = true
  error.value = ''
  try {
    await auth.selectOrg(orgId)
    router.replace('/')
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (!auth.isLoggedIn) {
    router.replace('/login')
  }
  if (!auth.isSuperAdmin && auth.organizations.length === 1) {
    void pick(auth.organizations[0].org_id)
  }
})
</script>

<template>
  <div class="select-page">
    <div class="select-card">
      <h2>选择组织</h2>
      <p class="hint">当前账号关联以下组织，请选择进入。</p>
      <p v-if="error" class="error">{{ error }}</p>
      <div class="grid">
        <div
          v-for="c in cards"
          :key="c.org_id"
          class="org-card"
          @click="activeOrgId = c.org_id"
        >
          <div class="org-name">{{ c.name }}</div>
          <div class="org-role">{{ c.role }}</div>
        </div>
      </div>
      <button
        class="submit-btn"
        :disabled="activeOrgId == null || loading"
        @click="pick(activeOrgId ?? 0)"
      >
        {{ loading ? '进入中…' : '进入' }}
      </button>
      <button class="link-btn" @click="auth.logout().then(() => router.replace('/login'))">退出登录</button>
    </div>
  </div>
</template>

<style scoped>
.select-page {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
}
.select-card {
  width: 520px;
  background: #fff;
  border-radius: 12px;
  padding: 32px;
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.08);
}
.hint {
  color: #86909c;
  font-size: 13px;
}
.grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  margin: 16px 0;
}
.org-card {
  border: 1px solid #e5e6eb;
  border-radius: 8px;
  padding: 16px;
  cursor: pointer;
  transition: all 0.2s;
}
.org-card:hover {
  border-color: #3370ff;
}
.org-name {
  font-weight: 600;
}
.org-role {
  color: #86909c;
  font-size: 12px;
  margin-top: 4px;
}
.submit-btn {
  width: 100%;
  height: 38px;
  background: #3370ff;
  color: #fff;
  border: none;
  border-radius: 6px;
  cursor: pointer;
}
.link-btn {
  width: 100%;
  margin-top: 8px;
  border: none;
  background: transparent;
  color: #86909c;
  cursor: pointer;
}
.error {
  color: #d03050;
  font-size: 13px;
}
</style>
