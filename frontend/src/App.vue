<script setup lang="ts">
import { onMounted, ref } from 'vue'
import type { HealthResponse } from './api'

const api = ref<HealthResponse | null>(null)
const error = ref('')

onMounted(async () => {
  try {
    api.value = await fetch('/api/health').then((r) => r.json())
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  }
})
</script>

<template>
  <main>
    <h1>Kate</h1>
    <p v-if="api">{{ api.name }} · {{ api.status }}</p>
    <p v-else-if="error" class="error">后端连接失败：{{ error }}</p>
    <p v-else>正在连接后端…</p>
  </main>
</template>

<style>
body {
  margin: 0;
  font-family: system-ui, -apple-system, sans-serif;
  background: #0f172a;
  color: #e2e8f0;
}
main {
  max-width: 640px;
  margin: 10vh auto;
  padding: 0 1rem;
}
.error {
  color: #f87171;
}
</style>
