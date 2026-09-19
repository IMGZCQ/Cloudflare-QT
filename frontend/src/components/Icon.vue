<script setup lang="ts">
defineProps<{
  name: 'play' | 'stop' | 'pause' | 'resume' | 'edit' | 'logs' | 'trash' | 'copy' | 'check' | 'view-list' | 'view-grid'
}>()
</script>

<template>
<!-- 启动：实心三角（放大版本，和 stop/pause/resume 视觉体量对齐） -->
<svg v-if="name === 'play'" viewBox="0 0 24 24" aria-hidden="true">
  <path d="M8 5.5L18.5 12L8 18.5z" fill="currentColor" />
</svg>

  <!-- 停止：实心方块【放大】 -->
  <svg v-else-if="name === 'stop'" viewBox="0 0 24 24" aria-hidden="true">
    <rect x="5.5" y="5.5" width="13" height="13" rx="1.4" fill="currentColor" />
  </svg>

  <!-- 暂停：两根竖条【加宽+放大】 -->
  <svg v-else-if="name === 'pause'" viewBox="0 0 24 24" aria-hidden="true">
    <rect x="5.2" y="4.5" width="4.2" height="15" rx="0.9" fill="currentColor" />
    <rect x="14.6" y="4.5" width="4.2" height="15" rx="0.9" fill="currentColor" />
  </svg>

  <!-- 恢复：环形播放【放大环形+内部三角】 -->
  <svg v-else-if="name === 'resume'" viewBox="0 0 24 24" aria-hidden="true">
    <path d="M12 3a9 9 0 109 9M12 1v2l5-2-5-2M9 8l6 4-6 4z" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>
  </svg>

  <!-- 编辑：铅笔，简化路径，视觉更干净 -->
  <svg v-else-if="name === 'edit'" viewBox="0 0 24 24" aria-hidden="true">
    <path d="M16.5 3.5a2.1 2.1 0 013 3L7 19l-4 1 1-4L16.5 3.5z" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
  </svg>

  <!-- 日志：文档横线，简化，小尺寸更清晰 -->
  <svg v-else-if="name === 'logs'" viewBox="0 0 24 24" aria-hidden="true">
    <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8L14 2zM14 2v6h6M8 13h8M8 17h5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
  </svg>

  <!-- 删除：垃圾桶，线条更柔和，小图标不容易糊 -->
  <svg v-else-if="name === 'trash'" viewBox="0 0 24 24" aria-hidden="true">
    <path d="M3 6h18M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2M10 11v6M14 11v6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
  </svg>

  <!-- 复制：双层文档 -->
  <svg v-else-if="name === 'copy'" viewBox="0 0 24 24" aria-hidden="true">
    <path d="M16 4h2a2 2 0 012 2v14a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2M14 2H10a2 2 0 00-2 2v2a2 2 0 002 2h4a2 2 0 002-2V2z" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
  </svg>

  <!-- 对勾 -->
  <svg v-else-if="name === 'check'" viewBox="0 0 24 24" aria-hidden="true">
    <path d="M5 12l5 5L19 7" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
  </svg>

  <!-- 列表视图：三行 -->
  <svg v-else-if="name === 'view-list'" viewBox="0 0 24 24" aria-hidden="true">
    <path d="M4 6h16M4 12h16M4 18h16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
  </svg>

  <!-- 网格视图：2x2 -->
  <svg v-else-if="name === 'view-grid'" viewBox="0 0 24 24" aria-hidden="true">
    <rect x="3" y="3" width="7" height="7" rx="1.2" fill="none" stroke="currentColor" stroke-width="2"/>
    <rect x="14" y="3" width="7" height="7" rx="1.2" fill="none" stroke="currentColor" stroke-width="2"/>
    <rect x="3" y="14" width="7" height="7" rx="1.2" fill="none" stroke="currentColor" stroke-width="2"/>
    <rect x="14" y="14" width="7" height="7" rx="1.2" fill="none" stroke="currentColor" stroke-width="2"/>
  </svg>
</template>

<style scoped>
svg {
  width: 16px;
  height: 16px;
  display: block;
  flex-shrink: 0;
  shape-rendering: geometricPrecision;
}
</style>
