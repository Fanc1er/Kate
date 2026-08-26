<script setup lang="ts">
import EngineDetail, { type EngineDetection } from '../../components/EngineDetail.vue'

const detections: EngineDetection[] = [
  { type: 'port_exposed', desc: 'CommonPorts 列表或显式端口开放（FTP/SSH/RDP/Redis/MySQL 等）', severity: 'high' },
]
</script>

<template>
  <EngineDetail
    title="端口服务"
    intro="TCP Connect 扫描目标常见高危端口，识别对外开放的服务与暴露面。"
    :detections="detections"
    suggestion="收敛暴露面+补丁加固"
  />
</template>
