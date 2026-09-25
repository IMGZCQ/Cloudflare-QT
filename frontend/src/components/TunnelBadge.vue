<script setup lang="ts">
import { ref } from 'vue'
import TunnelFavicon from './TunnelFavicon.vue'

// 隧道标题前的状态徽章：优先显示 favicon，未加载成功时回退到状态圆点
// 封装后父组件只需一行，无需关心图标/圆点切换逻辑
withDefaults(
  defineProps<{
    id: string
    state: string
    version?: string | number
    dotSize?: number // 圆点直径（px）
    iconSize?: number // 图标边长（px）
  }>(),
  { dotSize: 14, iconSize: 22 },
)

const iconLoaded = ref(false)
</script>

<template>
  <span
    v-if="!iconLoaded"
    class="dot"
    :class="state"
    :style="{ width: dotSize + 'px', height: dotSize + 'px' }"
  ></span>
  <TunnelFavicon :id="id" :version="version" :size="iconSize" @update:loaded="iconLoaded = $event" />
</template>

<style scoped>
.dot {
  border-radius: 50%;
  flex-shrink: 0;
  background: var(--muted, #888);
}
.dot.running { background: var(--ok, #3fb950); }
.dot.starting { background: var(--accent, #d29922); }
.dot.paused { background: var(--warn, #d29922); }
.dot.error { background: var(--err, #f85149); }
</style>
