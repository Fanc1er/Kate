<script setup lang="ts">
import EngineDetail, { type EngineDetection } from '../../components/EngineDetail.vue'

const detections: EngineDetection[] = [
  { type: 'webshell_pattern', desc: 'eval / assert / 系统命令 / 文件写入等危险函数调用', severity: 'high' },
  { type: 'webshell_obfuscated', desc: 'eval/assert + base64_decode 等编码混淆组合', severity: 'critical' },
  { type: 'webshell_php', desc: 'PHP 代码含危险函数组合，疑似后门文件', severity: 'critical' },
]
</script>

<template>
  <EngineDetail
    title="Webshell 检测"
    intro="基于特征码与模式匹配检测后门文件，覆盖危险函数、编码混淆与 PHP 后门特征。"
    :detections="detections"
    suggestion="立即下线+清除+溯源"
  />
</template>
