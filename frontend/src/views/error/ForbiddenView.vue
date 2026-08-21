<script setup lang="ts">
import { useRouter } from 'vue-router'
const router = useRouter()
</script>

<template>
  <div class="page">
    <h1>403</h1>
    <p>您没有访问该页面的权限</p>
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
  color: #1d2129;
}
.btn {
  border: none;
  background: #3370ff;
  color: #fff;
  padding: 8px 20px;
  border-radius: 6px;
  cursor: pointer;
}
</style>
