<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { forgotPassword } from '../../api/auth'

const router = useRouter()
const email = ref('')
const loading = ref(false)
const error = ref('')
const sent = ref(false)

async function submit(): Promise<void> {
  if (!email.value) {
    error.value = '请输入邮箱'
    return
  }
  loading.value = true
  error.value = ''
  try {
    await forgotPassword(email.value)
    sent.value = true
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
      <h2>忘记密码</h2>
      <p class="hint">输入注册邮箱，5 分钟内将收到一次性验证码。</p>
      <p v-if="sent" class="success">验证码已发送，请前往邮箱查看，并在 5 分钟内完成重置。</p>
      <p v-else-if="error" class="error">{{ error }}</p>
      <div v-if="!sent" class="field">
        <label>邮箱</label>
        <input v-model="email" type="email" class="input" placeholder="name@example.com" />
      </div>
      <button v-if="!sent" class="submit-btn" :disabled="loading" @click="submit">
        {{ loading ? '发送中…' : '发送验证码' }}
      </button>
      <button class="link-btn" @click="router.push(sent ? '/reset-password' : '/login')">
        {{ sent ? '前往重置密码' : '返回登录' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.page {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
}
.card {
  width: 360px;
  background: #fff;
  border-radius: 12px;
  padding: 32px;
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.08);
}
.hint {
  color: #86909c;
  font-size: 13px;
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
  border: 1px solid #e5e6eb;
  border-radius: 6px;
  padding: 0 10px;
  outline: none;
}
.input:focus {
  border-color: #3370ff;
}
.submit-btn {
  width: 100%;
  height: 38px;
  background: #3370ff;
  color: #fff;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  margin-bottom: 8px;
}
.link-btn {
  width: 100%;
  border: none;
  background: transparent;
  color: #3370ff;
  cursor: pointer;
}
.error {
  color: #d03050;
  font-size: 13px;
}
.success {
  color: #00b42a;
  font-size: 13px;
}
</style>
