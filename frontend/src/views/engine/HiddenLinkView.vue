<script setup lang="ts">
import EngineDetail, { type EngineDetection } from '../../components/EngineDetail.vue'

const detections: EngineDetection[] = [
  { type: 'hidden_element', desc: 'display:none / visibility:hidden 等隐藏元素，疑似暗链', severity: 'medium' },
  { type: 'external_iframe', desc: '页面嵌入外部域名 iframe', severity: 'high' },
  { type: 'javascript_protocol', desc: 'javascript: 协议链接，存在 XSS/恶意跳转风险', severity: 'medium' },
  { type: 'data_uri', desc: 'data: URI 引用，需人工复核内容', severity: 'low' },
]
</script>

<template>
  <EngineDetail
    title="暗链检测"
    intro="识别页面隐藏元素、外部 iframe、javascript: 协议与 data URI 等暗链挂马特征。"
    :detections="detections"
    suggestion="隔离+溯源+加固"
  />
</template>
