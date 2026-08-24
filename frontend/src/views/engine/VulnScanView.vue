<script setup lang="ts">
import { ref, onMounted } from 'vue'
import Skeleton from '../../components/Skeleton.vue'

const loading = ref(false)

onMounted(() => {
  loading.value = true
  setTimeout(() => {
    loading.value = false
  }, 600)
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <h2 class="text-lg font-semibold text-gray-800">漏洞扫描</h2>
    <div v-if="loading" class="space-y-2">
      <Skeleton class="h-10 w-full" />
      <Skeleton class="h-10 w-full" />
      <Skeleton class="h-10 w-full" />
    </div>
    <div v-else class="text-center py-12 text-gray-500 text-sm">
      漏洞扫描引擎正在实现中
    </div>
  </div>
</template>
