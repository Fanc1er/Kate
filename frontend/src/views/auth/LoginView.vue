<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'

const router = useRouter()
const auth = useAuthStore()

const username = ref('')
const password = ref('')
const loading = ref(false)
const error = ref('')

async function submit(): Promise<void> {
  if (!username.value || !password.value) {
    error.value = '请输入用户名和密码'
    return
  }
  loading.value = true
  error.value = ''
  try {
    await auth.login(username.value, password.value)
    router.replace('/')
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-card">
      <h1 class="title">CInsight</h1>
      <p class="subtitle">安全监测平台</p>
      <form @submit.prevent="submit">
        <div class="field">
          <label>用户名</label>
          <input v-model="username" class="input" placeholder="请输入用户名" autocomplete="username" />
        </div>
        <div class="field">
          <label>密码</label>
          <input
            v-model="password"
            type="password"
            class="input"
            placeholder="请输入密码"
            autocomplete="current-password"
          />
        </div>
        <p v-if="error" class="error">{{ error }}</p>
        <button type="submit" class="submit-btn" :disabled="loading">
          {{ loading ? '登录中…' : '登录' }}
        </button>
      </form>
      <div class="links">
        <router-link to="/forgot-password">忘记密码</router-link>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, var(--color-text-primary) 0%, var(--color-brand) 100%);
}
.login-card {
  width: 360px;
  background: #fff;
  border-radius: var(--radius-lg);
  padding: 40px 32px;
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.2);
}
.title {
  margin: 0 0 4px;
  font-size: 24px;
  color: var(--color-text-primary);
  text-align: center;
}
.subtitle {
  margin: 0 0 24px;
  color: var(--color-text-tertiary);
  text-align: center;
  font-size: 13px;
}
.field {
  margin-bottom: 16px;
}
.field label {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
  color: var(--color-text-secondary);
}
.input {
  width: 100%;
  height: 36px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 0 10px;
  font-size: 14px;
  outline: none;
}
.input:focus {
  border-color: var(--color-brand);
}
.error {
  color: var(--color-danger);
  font-size: 13px;
  margin: 0 0 12px;
}
.submit-btn {
  width: 100%;
  height: 38px;
  background: var(--color-brand);
  color: #fff;
  border: none;
  border-radius: var(--radius-md);
  font-size: 14px;
  cursor: pointer;
}
.submit-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
.links {
  margin-top: 16px;
  text-align: center;
}
.links a {
  color: var(--color-brand);
  font-size: 13px;
}
</style>
