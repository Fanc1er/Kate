<script setup lang="ts">
import EngineDetail, { type EngineDetection } from '../../components/EngineDetail.vue'

const detections: EngineDetection[] = [
  { type: 'cve_match', desc: '组件版本命中内置 CVE 规则（nginx/apache 等）或外部情报源', severity: 'high' },
]
</script>

<template>
  <EngineDetail
    title="情报关联"
    intro="识别目标 Web 组件版本并匹配内置 CVE 规则，支持注入外部 CVE/CNVD/CNNVD 情报源。"
    :detections="detections"
    suggestion="按情报修复漏洞并复核"
  />
</template>
