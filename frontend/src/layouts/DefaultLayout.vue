<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { MENU, canAccess } from '../config/routes'
import { ROLE_LABELS } from '../config/permissions'
import { eventStream } from '../api/ws'
import AppErrorBoundary from '../components/AppErrorBoundary.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const collapsed = ref(false)
const wsConnected = ref(false)
const wsBannerVisible = ref(false)
let wsBannerTimer = 0

const menus = computed(() => MENU.filter((m) => canAccess(m.roles, auth.role)))

const roleLabel = computed(() => ROLE_LABELS[auth.role] ?? auth.role)

async function doLogout(): Promise<void> {
  await auth.logout()
  eventStream.disconnect()
  router.replace('/login')
}

function onWsStatusChange(connected: boolean): void {
  const prev = wsConnected.value
  wsConnected.value = connected
  if (!prev && !connected) {
    wsBannerVisible.value = true
    window.clearTimeout(wsBannerTimer)
  } else if (connected && wsBannerVisible.value) {
    wsBannerVisible.value = false
    wsBannerTimer = window.setTimeout(() => {}, 0)
  }
}

let unsub: (() => void) | null = null

onMounted(() => {
  eventStream.connect()
  eventStream.onStatusChange(document.body, onWsStatusChange)
  unsub = eventStream.subscribe((e) => {
    if (e.kind === 'alert.new') {
      router.push('/alerts')
    }
  })
})

onBeforeUnmount(() => {
  unsub?.()
  eventStream.disconnect()
})
</script>

<template>
  <div class="layout">
    <aside class="sider" :class="{ collapsed }">
      <div class="logo">
        <span class="logo-dot">C</span>
        <span v-if="!collapsed" class="logo-text">CInsight</span>
      </div>
      <nav class="menu">
        <router-link
          v-for="m in menus"
          :key="m.path"
          :to="m.path"
          class="menu-item"
          :class="{ active: route.path === m.path }"
        >
          <span v-if="!collapsed">{{ m.title }}</span>
          <span v-else class="collapsed-tip">{{ m.title }}</span>
        </router-link>
      </nav>
    </aside>

    <div class="main">
      <header class="topbar">
        <button class="collapse-btn" @click="collapsed = !collapsed">
          {{ collapsed ? '»' : '«' }}
        </button>
        <div class="topbar-title">{{ String(route.meta.title ?? '') }}</div>
        <div class="topbar-right">
          <span class="role-tag">{{ roleLabel }}</span>
          <button class="btn-link" @click="doLogout">退出</button>
        </div>
      </header>

      <div v-if="wsBannerVisible" class="ws-banner">
        WebSocket 已断开，正在自动重连…
      </div>

      <main class="content">
        <AppErrorBoundary>
          <router-view />
        </AppErrorBoundary>
      </main>
    </div>
  </div>
</template>

<style scoped>
.layout {
  display: flex;
  height: 100vh;
}
.sider {
  width: 200px;
  background: #1d2129;
  color: #c9cdd4;
  display: flex;
  flex-direction: column;
  transition: width 0.2s;
  flex-shrink: 0;
}
.sider.collapsed {
  width: 64px;
}
.logo {
  height: 56px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 16px;
  border-bottom: 1px solid #2e3340;
}
.logo-dot {
  background: #3370ff;
  color: #fff;
  border-radius: 6px;
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
}
.logo-text {
  color: #fff;
  font-size: 16px;
  font-weight: 600;
}
.menu {
  flex: 1;
  padding: 8px;
  overflow-y: auto;
}
.menu-item {
  display: block;
  padding: 10px 12px;
  margin-bottom: 4px;
  border-radius: 6px;
  font-size: 14px;
  cursor: pointer;
  color: #c9cdd4;
}
.menu-item:hover {
  background: #2b313c;
  color: #fff;
}
.menu-item.active {
  background: #3370ff;
  color: #fff;
}
.collapsed-tip {
  display: block;
  text-align: center;
  font-size: 12px;
}
.main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.topbar {
  height: 56px;
  background: #fff;
  border-bottom: 1px solid #e5e6eb;
  display: flex;
  align-items: center;
  padding: 0 16px;
  gap: 12px;
}
.collapse-btn {
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 16px;
  color: #666;
}
.topbar-title {
  font-size: 15px;
  font-weight: 600;
}
.topbar-right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
}
.org-name {
  color: #333;
}
.role-tag {
  background: #e8f3ff;
  color: #3370ff;
  border-radius: 4px;
  padding: 2px 8px;
}
.btn-link {
  border: none;
  background: transparent;
  color: #3370ff;
  cursor: pointer;
  font-size: 13px;
}
.btn-link:hover {
  text-decoration: underline;
}
.ws-banner {
  background: #fff7e6;
  color: #d46b08;
  text-align: center;
  padding: 6px;
  font-size: 13px;
}
.content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
}
</style>
