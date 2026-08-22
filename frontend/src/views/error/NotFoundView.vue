<script setup lang="ts">
import { useRouter } from 'vue-router'
const router = useRouter()
</script>

<template>
  <div class="page">
    <h1>404</h1>
    <p>页面不存在</p>
    <button class="btn" @click="router.push('/')">返回首页</button>
  </div>
</template>

<style scoped>
.page {
  height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
}
h1 {
  margin: 0;
  font-size: 64px;
  color: var(--color-text-primary);
}
.btn {
  border: none;
  background: var(--color-brand);
  color: #fff;
  padding: 8px 20px;
  border-radius: var(--radius-md);
  cursor: pointer;
}
</style>
