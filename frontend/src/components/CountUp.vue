<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'

const props = withDefaults(
  defineProps<{
    value: number
    duration?: number
  }>(),
  { duration: 1000 },
)

const display = ref(0)
let raf = 0

function animate(from: number, to: number): void {
  cancelAnimationFrame(raf)
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches || to === from) {
    display.value = to
    return
  }
  const start = performance.now()
  const step = (now: number): void => {
    const p = Math.min((now - start) / props.duration, 1)
    const eased = 1 - Math.pow(1 - p, 3)
    display.value = Math.round(from + (to - from) * eased)
    if (p < 1) {
      raf = requestAnimationFrame(step)
    }
  }
  raf = requestAnimationFrame(step)
}

watch(
  () => props.value,
  (v, old) => animate(old ?? 0, v),
)

onMounted(() => animate(0, props.value))
</script>

<template>
  <span>{{ display.toLocaleString() }}</span>
</template>
