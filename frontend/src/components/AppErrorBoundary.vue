<script setup lang="ts">
import { ref, onErrorCaptured } from 'vue'

const hasError = ref(false)
const errorMessage = ref('')

onErrorCaptured((err) => {
  hasError.value = true
  errorMessage.value = err instanceof Error ? err.message : String(err)
  return false
})

function reload(): void {
  hasError.value = false
  errorMessage.value = ''
  window.location.reload()
}
</script>

<template>
  <div v-if="hasError" class="boundary">
    <div class="box">
      <h3>页面渲染异常</h3>
      <p class="msg">{{ errorMessage || '发生未知错误' }}</p>
      <button class="retry" @click="reload">重新加载</button>
    </div>
  </div>
  <slot v-else />
</template>

<style scoped>
.boundary {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 50vh;
}
.box {
  text-align: center;
  padding: 32px 40px;
  border-radius: 10px;
  background: #fff;
  box-shadow: var(--shadow-hover);
}
h3 {
  margin: 0 0 8px;
  font-size: 16px;
}
.msg {
  margin: 0 0 16px;
  color: var(--color-text-tertiary);
  font-size: 13px;
  max-width: 420px;
  word-break: break-all;
}
.retry {
  border: none;
  background: var(--color-brand);
  color: #fff;
  border-radius: var(--radius-md);
  padding: 6px 18px;
  font-size: 13px;
  cursor: pointer;
}
.retry:hover {
  background: var(--color-brand-hover);
}
</style>
