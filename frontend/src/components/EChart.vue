<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'

const props = defineProps<{
  option: Record<string, unknown>
  height?: string
}>()

const el = ref<HTMLDivElement | null>(null)
let chart: echarts.ECharts | null = null

function render(): void {
  if (!el.value) return
  if (!chart) {
    chart = echarts.init(el.value)
  }
  chart.setOption(props.option as EChartsOption, true)
}

function resize(): void {
  chart?.resize()
}

onMounted(() => {
  render()
  window.addEventListener('resize', resize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', resize)
  chart?.dispose()
  chart = null
})

watch(
  () => props.option,
  () => render(),
  { deep: true },
)
</script>

<template>
  <div ref="el" :style="{ width: '100%', height: height ?? '300px' }" />
</template>
