<script setup lang="ts">
import EngineDetail, { type EngineDetection } from '../../components/EngineDetail.vue'

const detections: EngineDetection[] = [
  { type: 'webshell_pattern', desc: '系统命令执行 / 动态函数构造等危险调用；eval、base64_decode 等在 PHP 语境下判定，普通前端 JS 自动降噪', severity: 'high' },
  { type: 'webshell_obfuscated', desc: '编码函数嵌套执行或请求参数直连危险函数的组合特征', severity: 'critical' },
  { type: 'webshell_php', desc: 'PHP 脚本内请求参数直连执行原语，疑似后门文件', severity: 'critical' },
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
