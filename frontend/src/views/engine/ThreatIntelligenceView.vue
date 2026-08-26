<script setup lang="ts">
import EngineDetail, { type EngineDetection } from '../../components/EngineDetail.vue'

const detections: EngineDetection[] = [
  { type: 'threat_domain', desc: '域名匹配已知恶意模式（malware/phishing/botnet 等）', severity: 'high' },
  { type: 'intel_threat_score', desc: '外部威胁情报源评分 ≥60/100', severity: 'high' },
  { type: 'suspicious_domain_entropy', desc: '高熵随机化域名，疑似算法生成', severity: 'medium' },
  { type: 'direct_ip_host', desc: '目标以 IP 直连而非域名，建议核实归属', severity: 'info' },
]
</script>

<template>
  <EngineDetail
    title="威胁情报"
    intro="本地恶意域名模式、高熵随机域名与 IP 直连启发式检测，支持注入外部威胁情报源评分。"
    :detections="detections"
    suggestion="确认域名归属并处置"
  />
</template>
