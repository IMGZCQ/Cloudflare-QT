<script setup lang="ts">
import { computed, onMounted, onUpdated, ref, watch } from 'vue'

// 隧道标题前的 favicon 图标：加载失败或未缓存时自动隐藏
// version 由父组件在隧道目标地址变化时更新，用于破浏览器缓存并重置失败状态
const props = withDefaults(
  defineProps<{ id: string; version?: string | number; size?: number }>(),
  { size: 18 },
)
const emit = defineEmits<{ (e: 'update:loaded', loaded: boolean): void }>()

// 与 api.ts 保持一致：从当前页面路径推导 API 前缀，兼容统一网关子路径
const base = new URL('.', window.location.href).pathname.replace(/\/$/, '')

const imgRef = ref<HTMLImageElement | null>(null)

// 三态：loading（默认）/ ok / fail
const status = ref<'loading' | 'ok' | 'fail'>('loading')

// 唯一 key：id+version 任一变化都强制重建 <img>
const imgKey = computed(() => `${props.id}::${props.version ?? ''}`)

const src = computed(() => {
  const v = props.version
  return v
    ? `${base}/api/tunnels/${props.id}/favicon?v=${encodeURIComponent(v)}`
    : `${base}/api/tunnels/${props.id}/favicon`
})

function setStatus(s: 'loading' | 'ok' | 'fail') {
  if (status.value === s) return
  status.value = s
  emit('update:loaded', s === 'ok')
}

function onLoad() {
  setStatus('ok')
}
function onError() {
  setStatus('fail')
}

// 主动检查图片是否已从缓存加载完成（load 事件可能不触发）
function checkComplete() {
  const img = imgRef.value
  if (!img) return
  if (status.value !== 'loading') return
  if (!img.complete) return // 仍在加载，等 load/error 事件
  setStatus(img.naturalWidth > 0 ? 'ok' : 'fail')
}

// key 变化时重置状态；DOM 重建由 :key 自动处理
watch(imgKey, () => {
  setStatus('loading')
})

// 挂载和每次 DOM 更新后都检查一次（覆盖所有生命周期场景）
onMounted(checkComplete)
onUpdated(checkComplete)
</script>

<template>
  <img
    v-if="status !== 'fail'"
    v-show="status === 'ok'"
    :key="imgKey"
    ref="imgRef"
    class="favicon"
    :src="src"
    :style="{ width: props.size + 'px', height: props.size + 'px' }"
    alt=""
    @load="onLoad"
    @error="onError"
  />
</template>

<style scoped>
.favicon {
  border-radius: 4px;
  flex-shrink: 0;
  object-fit: contain;
  background: var(--panel-2, transparent);
  display: block;
}
</style>
