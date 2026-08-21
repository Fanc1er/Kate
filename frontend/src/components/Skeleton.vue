<script setup lang="ts">
defineProps<{
  rows?: number
  cols?: number
  variant?: 'table' | 'cards'
}>()
</script>

<template>
  <div class="skeleton" :class="`skeleton-${variant ?? 'table'}`">
    <div v-for="r in rows ?? 5" :key="r" class="skeleton-row">
      <div v-for="c in cols ?? 4" :key="c" class="skeleton-col">
        <span class="skeleton-bar" :style="{ width: `${70 + ((r * 7 + c * 13) % 26)}%` }"></span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.skeleton {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 8px 0;
}
.skeleton-row {
  display: flex;
  gap: 16px;
}
.skeleton-col {
  flex: 1;
}
.skeleton-bar {
  display: block;
  height: 14px;
  border-radius: 4px;
  background: linear-gradient(90deg, #eef0f3 25%, #f7f8fa 50%, #eef0f3 75%);
  background-size: 200% 100%;
  animation: skeleton-pulse 1.4s ease-in-out infinite;
}
.skeleton-cards .skeleton-row {
  flex-wrap: wrap;
}
.skeleton-cards .skeleton-col {
  min-width: 220px;
  height: 96px;
  border-radius: 8px;
  border: 1px solid #f2f3f5;
  background: #fff;
}
@keyframes skeleton-pulse {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -200% 0;
  }
}
</style>
