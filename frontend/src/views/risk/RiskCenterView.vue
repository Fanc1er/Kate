<script setup lang="ts">
interface Tab {
  key: string
  label: string
  path: string
}

const tabs: Tab[] = [
  { key: 'findings', label: '发现', path: '/risk/findings' },
  { key: 'events', label: '安全事件', path: '/risk/events' },
  { key: 'alerts', label: '告警', path: '/risk/alerts' },
  { key: 'vulnerabilities', label: '漏洞', path: '/risk/vulnerabilities' },
]
</script>

<template>
  <div class="risk-center">
    <div class="tab-bar" role="tablist">
      <router-link
        v-for="t in tabs"
        :key="t.key"
        :to="t.path"
        class="tab"
        active-class="active"
      >
        {{ t.label }}
      </router-link>
    </div>
    <div class="tab-panel">
      <router-view />
    </div>
  </div>
</template>

<style scoped>
.risk-center {
  display: flex;
  flex-direction: column;
}
.tab-bar {
  display: flex;
  gap: 4px;
  border-bottom: 1px solid var(--color-border);
  margin-bottom: 16px;
}
.tab {
  display: block;
  padding: 10px 16px;
  cursor: pointer;
  font-size: var(--font-size-md);
  color: var(--color-text-secondary);
  text-decoration: none;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
}
.tab:hover {
  color: var(--color-text-primary);
}
.tab.active {
  color: var(--color-brand);
  font-weight: var(--font-weight-semibold);
  border-bottom-color: var(--color-brand);
}
.tab-panel {
  min-height: 0;
}
</style>
