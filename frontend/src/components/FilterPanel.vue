<script setup lang="ts">
defineProps<{ clearable?: boolean }>()
const emit = defineEmits<{ clear: [] }>()
</script>

<template>
  <aside class="filter-panel">
    <div class="filter-head">
      <span class="filter-title">筛选</span>
      <button v-if="clearable" class="filter-clear" @click="emit('clear')">清除全部</button>
    </div>
    <slot />
  </aside>
</template>

<style scoped>
.filter-panel {
  background: var(--color-bg-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 16px;
}
.filter-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
.filter-title {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--color-text-primary);
}
.filter-clear {
  border: none;
  background: transparent;
  color: var(--color-brand);
  cursor: pointer;
  font-size: var(--font-size-sm);
  padding: 0;
}
.filter-clear:hover {
  text-decoration: underline;
}
</style>
