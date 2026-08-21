<script setup lang="ts">
import { ref } from 'vue'

export interface ToastItem {
  id: number
  type: 'success' | 'error' | 'warning' | 'info'
  message: string
  duration: number
}

export interface ToastExpose {
  success: (message: string, duration?: number) => void
  error: (message: string, duration?: number) => void
  warning: (message: string, duration?: number) => void
  info: (message: string, duration?: number) => void
}

const items = ref<ToastItem[]>([])
let seed = 0

function push(type: ToastItem['type'], message: string, duration: number): void {
  const id = ++seed
  items.value.push({ id, type, message, duration })
  if (duration > 0) {
    window.setTimeout(() => dismiss(id), duration)
  }
}

function dismiss(id: number): void {
  items.value = items.value.filter((it) => it.id !== id)
}

function toast(type: ToastItem['type'], message: string, duration = 3000): void {
  push(type, message, duration)
}

function success(message: string, duration = 3000): void {
  toast('success', message, duration)
}
function error(message: string, duration = 4000): void {
  toast('error', message, duration)
}
function warning(message: string, duration = 3500): void {
  toast('warning', message, duration)
}
function info(message: string, duration = 3000): void {
  toast('info', message, duration)
}

defineExpose({ success, error, warning, info })
</script>

<template>
  <div class="toast-container">
    <transition-group name="toast">
      <div v-for="it in items" :key="it.id" class="toast-item" :class="`toast-${it.type}`">
        <span class="toast-dot"></span>
        <span class="toast-text">{{ it.message }}</span>
        <button class="toast-close" aria-label="关闭" @click="dismiss(it.id)">×</button>
      </div>
    </transition-group>
  </div>
</template>

<style scoped>
.toast-container {
  position: fixed;
  top: 16px;
  right: 16px;
  z-index: 3000;
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-width: 360px;
}
.toast-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 6px;
  background: #fff;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.14);
  border-left: 4px solid #4b6bfb;
  font-size: 13px;
  line-height: 1.4;
  color: #333;
  word-break: break-word;
}
.toast-success {
  border-left-color: #2fbf6f;
}
.toast-warning {
  border-left-color: #f5a623;
}
.toast-error {
  border-left-color: #f5222d;
}
.toast-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex: none;
  background: #4b6bfb;
}
.toast-success .toast-dot {
  background: #2fbf6f;
}
.toast-warning .toast-dot {
  background: #f5a623;
}
.toast-error .toast-dot {
  background: #f5222d;
}
.toast-text {
  flex: 1;
}
.toast-close {
  border: none;
  background: none;
  color: #999;
  font-size: 16px;
  cursor: pointer;
  padding: 0 2px;
  line-height: 1;
}
.toast-close:hover {
  color: #333;
}
.toast-enter-active,
.toast-leave-active {
  transition: all 0.25s ease;
}
.toast-enter-from {
  opacity: 0;
  transform: translateX(20px);
}
.toast-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
