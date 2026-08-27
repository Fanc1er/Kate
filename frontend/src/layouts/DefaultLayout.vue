<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { MENU, canAccess } from '../config/routes'
import { ROLE_LABELS } from '../config/permissions'
import { eventStream } from '../api/ws'
import AppErrorBoundary from '../components/AppErrorBoundary.vue'
import GlobalSearch from '../components/GlobalSearch.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const collapsed = ref(false)
const mobileMenuOpen = ref(false)
const isMobile = ref(window.innerWidth <= 768)
// 折叠态仅在桌面视口生效，避免移动端抽屉残留压缩样式。
const siderCollapsed = computed(() => collapsed.value && !isMobile.value)

function onViewportChange(): void {
  isMobile.value = window.innerWidth <= 768
  if (!isMobile.value) {
    mobileMenuOpen.value = false
  }
}

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
  window.addEventListener('resize', onViewportChange)
  eventStream.connect()
  eventStream.onStatusChange(document.body, onWsStatusChange)
  unsub = eventStream.subscribe((e) => {
    if (e.kind === 'alert.new') {
      router.push('/risk/alerts')
    }
  })
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onViewportChange)
  unsub?.()
  eventStream.disconnect()
})
</script>

<template>
  <div class="layout">
    <aside class="sider" :class="{ collapsed: siderCollapsed, open: mobileMenuOpen }">
      <div class="logo">
        <span class="logo-dot">C</span>
        <span v-if="!siderCollapsed" class="logo-text">CInsight</span>
      </div>
      <nav class="menu">
        <router-link
          v-for="m in menus"
          :key="m.path"
          :to="m.path"
          class="menu-item"
          :class="{ active: route.path === m.path || route.path.startsWith(m.path + '/') }"
          @click="mobileMenuOpen = false"
        >
          <span v-if="!siderCollapsed">{{ m.title }}</span>
          <span v-else class="collapsed-tip">{{ m.title }}</span>
        </router-link>
      </nav>
    </aside>

    <div class="main">
      <header class="topbar">
        <button class="mobile-menu-btn" @click="mobileMenuOpen = !mobileMenuOpen">
          <span />
          <span />
          <span />
        </button>
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
    <div v-if="mobileMenuOpen" class="mobile-overlay" @click="mobileMenuOpen = false" />
    <GlobalSearch />
  </div>
</template>

<style scoped>
.layout {
  display: flex;
  height: 100vh;
}
.sider {
  width: 200px;
  background: var(--color-bg-card);
  border-right: 1px solid var(--color-border);
  color: var(--color-text-secondary);
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
  border-bottom: 1px solid var(--color-border);
}
.logo-dot {
  background: var(--color-brand);
  color: #fff;
  border-radius: var(--radius-md);
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-weight: var(--font-weight-semibold);
}
.logo-text {
  color: var(--color-text-primary);
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
}
.menu {
  flex: 1;
  padding: var(--spacing-2);
  overflow-y: auto;
}
.menu-item {
  display: block;
  padding: 10px 12px;
  margin-bottom: var(--spacing-1);
  border-radius: var(--radius-md);
  font-size: var(--font-size-md);
  cursor: pointer;
  color: var(--color-text-secondary);
  transition:
    background 0.15s,
    color 0.15s;
}
.menu-item:hover {
  background: var(--color-bg-hover);
  color: var(--color-text-primary);
}
.menu-item.active {
  background: var(--color-bg-selected);
  color: var(--color-brand);
  font-weight: var(--font-weight-semibold);
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
  background: var(--color-bg-card);
  border-bottom: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  padding: 0 16px;
  gap: 12px;
}
.collapse-btn {
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: var(--font-size-lg);
  color: var(--color-text-secondary);
}
.topbar-title {
  font-size: 15px;
  font-weight: var(--font-weight-semibold);
}
.topbar-right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: var(--font-size-sm);
}
.org-name {
  color: var(--color-text-primary);
}
.role-tag {
  background: var(--color-brand-light);
  color: var(--color-brand);
  border-radius: var(--radius-sm);
  padding: 2px 8px;
}
.btn-link {
  border: none;
  background: transparent;
  color: var(--color-brand);
  cursor: pointer;
  font-size: var(--font-size-sm);
}
.btn-link:hover {
  text-decoration: underline;
}
.ws-banner {
  background: #fff7e6;
  color: var(--color-warning);
  text-align: center;
  padding: 6px;
  font-size: var(--font-size-sm);
}
.content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
}

.mobile-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 999;
}

.mobile-menu-btn {
  display: none;
  flex-direction: column;
  gap: 4px;
  background: none;
  border: none;
  cursor: pointer;
  padding: 4px;
}
.mobile-menu-btn span {
  display: block;
  width: 20px;
  height: 2px;
  background: var(--color-text-secondary);
  border-radius: 2px;
  transition: transform 0.2s;
}

@media (max-width: 768px) {
  .sider {
    position: fixed;
    left: 0;
    top: 0;
    bottom: 0;
    z-index: 1000;
    transform: translateX(-100%);
    transition: transform 0.2s;
    width: 220px;
  }
  .sider.open {
    transform: translateX(0);
  }
  .collapse-btn {
    display: none;
  }
  .mobile-menu-btn {
    display: flex;
  }
  .topbar-title {
    font-size: 14px;
  }
}
</style>
