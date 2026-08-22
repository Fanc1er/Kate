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
    if (auth.needSelectOrg || auth.organizations.length > 1) {
      router.replace('/select-org')
    } else if (auth.isSuperAdmin) {
      router.replace('/platform')
    } else {
      router.replace('/')
    }
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
      <p class="subtitle">多租户安全监测平台</p>
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
  background: linear-gradient(135deg, #1d2129 0%, #3370ff 100%);
}
.login-card {
  width: 360px;
  background: #fff;
  border-radius: 12px;
  padding: 40px 32px;
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.2);
}
.title {
  margin: 0 0 4px;
  font-size: 24px;
  color: #1d2129;
  text-align: center;
}
.subtitle {
  margin: 0 0 24px;
  color: #86909c;
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
  color: #4e5969;
}
.input {
  width: 100%;
  height: 36px;
  border: 1px solid #e5e6eb;
  border-radius: 6px;
  padding: 0 10px;
  font-size: 14px;
  outline: none;
}
.input:focus {
  border-color: #3370ff;
}
.error {
  color: #d03050;
  font-size: 13px;
  margin: 0 0 12px;
}
.submit-btn {
  width: 100%;
  height: 38px;
  background: #3370ff;
  color: #fff;
  border: none;
  border-radius: 6px;
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
  color: #3370ff;
  font-size: 13px;
}
</style>
