<script setup lang="ts">
import EngineDetail, { type EngineDetection } from '../../components/EngineDetail.vue'

const detections: EngineDetection[] = [
  { type: 'dns_internal_ip', desc: 'DNS 解析到内网/回环/链路本地地址，疑似劫持', severity: 'high' },
  { type: 'dns_resolver_inconsistent', desc: '多节点 DNS 解析结果不一致，疑似污染', severity: 'high' },
  { type: 'dns_resolution_failed', desc: '主机无法解析', severity: 'medium' },
  { type: 'dns_no_records', desc: '主机无 IP 记录返回', severity: 'medium' },
  { type: 'cert_expired', desc: 'TLS 证书已过期', severity: 'high' },
  { type: 'cert_hostname_mismatch', desc: 'TLS 证书不覆盖访问主机名', severity: 'high' },
  { type: 'cert_not_yet_valid', desc: 'TLS 证书尚未生效', severity: 'high' },
  { type: 'cert_expiring_soon', desc: 'TLS 证书将在 30 天内过期', severity: 'low' },
]
</script>

<template>
  <EngineDetail
    title="DNS 安全"
    intro="DNS 解析检查、内网地址劫持判定、多节点解析对比与证书有效期/SAN 监测。"
    :detections="detections"
    suggestion="核查 DNS 配置并处理证书"
  />
</template>
