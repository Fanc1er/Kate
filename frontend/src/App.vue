<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from './stores/auth'
import Toast from './components/Toast.vue'
import { registerToast, type ToastExpose } from './utils/toast'

const router = useRouter()
const auth = useAuthStore()
const toastRef = ref<InstanceType<typeof Toast> | null>(null)

onMounted(async () => {
  registerToast(toastRef.value as unknown as ToastExpose)
  if (auth.isLoggedIn && !auth.user) {
    try {
      await auth.fetchMe()
    } catch {
      await auth.logout()
      router.replace('/login')
    }
  }
})
</script>

<template>
  <router-view />
  <Toast ref="toastRef" />
</template>

<style>
* {
  box-sizing: border-box;
}
html,
body,
#app {
  margin: 0;
  padding: 0;
  height: 100%;
  font-family:
    -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial,
    'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
  background: #f5f7fa;
  color: #333;
}
a {
  color: inherit;
  text-decoration: none;
}
</style>
