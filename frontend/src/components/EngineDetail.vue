<script lang="ts">
export interface EngineDetection {
  type: string
  desc: string
  severity: 'critical' | 'high' | 'medium' | 'low' | 'info'
}
</script>

<script setup lang="ts">
const props = defineProps<{
  title: string
  intro: string
  detections: EngineDetection[]
  suggestion: string
}>()

const severityClass: Record<EngineDetection['severity'], string> = {
  critical: 'bg-red-100 text-red-700',
  high: 'bg-orange-100 text-orange-700',
  medium: 'bg-yellow-100 text-yellow-700',
  low: 'bg-blue-100 text-blue-700',
  info: 'bg-gray-100 text-gray-600',
}

const severityLabel: Record<EngineDetection['severity'], string> = {
  critical: '严重',
  high: '高危',
  medium: '中危',
  low: '低危',
  info: '提示',
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <h2 class="text-lg font-semibold text-gray-800">{{ title }}</h2>
    <p class="text-sm text-gray-600">{{ props.intro }}</p>
    <div class="border border-gray-200 rounded-lg overflow-hidden">
      <div class="px-4 py-2 bg-gray-50 text-xs font-medium text-gray-600 border-b border-gray-200">
        检测项
      </div>
      <table class="w-full text-sm">
        <tbody>
          <tr
            v-for="d in props.detections"
            :key="d.type"
            class="border-b border-gray-100 last:border-0"
          >
            <td class="px-4 py-2 font-mono text-xs text-gray-700 w-56">{{ d.type }}</td>
            <td class="px-4 py-2 text-gray-600">{{ d.desc }}</td>
            <td class="px-4 py-2 text-right">
              <span class="text-xs px-2 py-0.5 rounded-full" :class="severityClass[d.severity]">
                {{ severityLabel[d.severity] }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="px-4 py-3 bg-blue-50 rounded-lg text-sm text-blue-800">
      <span class="font-medium">处置建议：</span>{{ props.suggestion }}
    </div>
  </div>
</template>
