<script setup lang="ts">
import EngineDetail, { type EngineDetection } from '../../components/EngineDetail.vue'

const detections: EngineDetection[] = [
  { type: 'path_exposure', desc: '敏感路径返回 200（.env / .git / wp-admin / phpmyadmin 等）', severity: 'medium' },
]
</script>

<template>
  <EngineDetail
    title="漏洞扫描"
    intro="探测目标常见敏感路径与暴露入口，识别潜在信息泄露与攻击面。"
    :detections="detections"
    suggestion="按 CVE 修复并复测"
  />
</template>
