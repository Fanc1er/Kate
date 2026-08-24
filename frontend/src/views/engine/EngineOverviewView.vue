<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()
const engines = ref([
  { name: 'vuln_scan', label: '漏洞扫描', status: 'skeleton', desc: '基于 CVE/NVD 的漏洞匹配骨架' },
  { name: 'hidden_link', label: '暗链检测', status: 'skeleton', desc: '隐藏链接与暗链识别骨架' },
  { name: 'webshell', label: 'Webshell 检测', status: 'skeleton', desc: 'Webshell 特征码匹配骨架' },
  { name: 'phishing', label: '钓鱼检测', status: 'skeleton', desc: '域名相似度与证书异常检测骨架' },
  { name: 'port_service', label: '端口服务', status: 'skeleton', desc: '端口扫描与服务指纹骨架' },
  { name: 'dns_security', label: 'DNS 安全', status: 'active', desc: 'DNS 解析检查与内网劫持检测' },
  { name: 'reputation', label: '威胁情报', status: 'skeleton', desc: 'IP/域名信誉评分骨架' },
  { name: 'intelligence', label: '情报关联', status: 'skeleton', desc: '多源情报聚合与关联分析骨架' },
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
