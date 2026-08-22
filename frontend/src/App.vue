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
  font-family: var(--font-family-base);
  background: var(--color-bg-page);
  color: var(--color-text-primary);
  font-size: var(--font-size-md);
  line-height: var(--line-height-base);
}
a {
  color: inherit;
  text-decoration: none;
}
:focus-visible {
  outline: 2px solid var(--color-brand);
  outline-offset: 1px;
}
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}
</style>
