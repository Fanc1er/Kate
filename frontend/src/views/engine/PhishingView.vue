<script setup lang="ts">
import EngineDetail, { type EngineDetection } from '../../components/EngineDetail.vue'

const detections: EngineDetection[] = [
  { type: 'typosquatting', desc: '域名与知名品牌编辑距离 ≤2，疑似仿冒钓鱼站点', severity: 'high' },
  { type: 'suspicious_domain', desc: '域名数字比例过高或连字符过多', severity: 'medium' },
  { type: 'no_https', desc: '目标使用 HTTP 明文，存在中间人攻击风险', severity: 'low' },
  { type: 'cert_expired', desc: 'TLS 证书已过期', severity: 'high' },
  { type: 'cert_hostname_mismatch', desc: 'TLS 证书不覆盖访问主机名', severity: 'high' },
  { type: 'cert_not_yet_valid', desc: 'TLS 证书尚未生效', severity: 'high' },
  { type: 'cert_expiring_soon', desc: 'TLS 证书将在 30 天内过期', severity: 'low' },
]
</script>

<template>
  <EngineDetail
    title="钓鱼检测"
    intro="基于 Levenshtein 域名相似度检测 typosquatting，并校验 HTTPS 证书有效期与域名覆盖。"
    :detections="detections"
    suggestion="下线仿冒+备案"
  />
</template>
