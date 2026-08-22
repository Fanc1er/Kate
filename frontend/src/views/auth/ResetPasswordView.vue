<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { resetPassword } from '../../api/auth'

const router = useRouter()
const email = ref('')
const code = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const loading = ref(false)
const error = ref('')
const done = ref(false)

async function submit(): Promise<void> {
  if (!email.value || !code.value || !newPassword.value) {
    error.value = '请填写邮箱、验证码与新密码'
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    error.value = '两次输入的密码不一致'
    return
  }
  loading.value = true
  error.value = ''
  try {
    await resetPassword(email.value, code.value, newPassword.value)
    done.value = true
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="page">
    <div class="card">
      <h2>重置密码</h2>
      <p v-if="done" class="success">密码已重置，请使用新密码登录。</p>
      <template v-else>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="field">
          <label>邮箱</label>
          <input v-model="email" type="email" class="input" placeholder="name@example.com" />
        </div>
        <div class="field">
          <label>验证码</label>
          <input v-model="code" class="input" placeholder="邮件中的验证码" />
        </div>
        <div class="field">
          <label>新密码</label>
          <input v-model="newPassword" type="password" class="input" placeholder="至少 12 位，含大小写字母、数字与特殊字符" />
        </div>
        <div class="field">
          <label>确认新密码</label>
          <input v-model="confirmPassword" type="password" class="input" placeholder="再次输入新密码" />
        </div>
        <button class="submit-btn" :disabled="loading" @click="submit">
          {{ loading ? '提交中…' : '重置密码' }}
        </button>
      </template>
      <button class="link-btn" @click="router.push('/login')">返回登录</button>
    </div>
  </div>
</template>

<style scoped>
.page {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-bg-page);
}
.card {
  width: 380px;
  background: #fff;
  border-radius: var(--radius-lg);
  padding: 32px;
  box-shadow: var(--shadow-hover);
}
.field {
  margin-bottom: 16px;
}
.field label {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
}
.input {
  width: 100%;
  height: 36px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 0 10px;
  outline: none;
}
.input:focus {
  border-color: var(--color-brand);
}
.submit-btn {
  width: 100%;
  height: 38px;
  background: var(--color-brand);
  color: #fff;
  border: none;
  border-radius: var(--radius-md);
  cursor: pointer;
  margin-bottom: 8px;
}
.link-btn {
  width: 100%;
  border: none;
  background: transparent;
  color: var(--color-brand);
  cursor: pointer;
}
.error {
  color: var(--color-danger);
  font-size: 13px;
}
.success {
  color: var(--color-success);
  font-size: 13px;
}
</style>
