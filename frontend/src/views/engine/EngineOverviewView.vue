<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()
const engines = ref([
  { name: 'vuln_scan', label: '漏洞扫描', status: 'active', desc: '敏感路径探测与暴露面识别' },
  { name: 'hidden_link', label: '暗链检测', status: 'active', desc: '隐藏元素/外部 iframe/危险协议' },
  { name: 'webshell', label: 'Webshell 检测', status: 'active', desc: '特征码与混淆模式匹配' },
  { name: 'phishing', label: '钓鱼检测', status: 'active', desc: '域名相似度与证书异常' },
  { name: 'port_service', label: '端口服务', status: 'active', desc: 'CommonPorts 扫描与服务指纹' },
  { name: 'dns_security', label: 'DNS 安全', status: 'active', desc: '解析检查/多节点对比/证书监测' },
  { name: 'threat_intelligence', label: '威胁情报', status: 'active', desc: '信誉评分与恶意特征启发式' },
  { name: 'intelligence', label: '情报关联', status: 'active', desc: '组件版本 CVE 匹配' },
])
</script>

<template>
  <div class="flex flex-col gap-4">
    <h2 class="text-lg font-semibold text-gray-800">引擎总览</h2>
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      <div
        v-for="e in engines"
        :key="e.name"
        class="p-4 border border-gray-200 rounded-lg hover:shadow-md transition-shadow cursor-pointer"
        @click="router.push(`/engines/${e.name}`)"
      >
        <div class="flex items-center justify-between mb-2">
          <span class="text-sm font-medium text-gray-800">{{ e.label }}</span>
          <span
            class="text-xs px-2 py-0.5 rounded-full"
            :class="e.status === 'active' ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'"
          >
            {{ e.status === 'active' ? '已启用' : '骨架' }}
          </span>
        </div>
        <p class="text-xs text-gray-500">{{ e.desc }}</p>
      </div>
    </div>
  </div>
</template>
