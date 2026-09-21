<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { api, type TunnelItem } from '../api'
import { useCopy } from '../composables/useCopy'
import Icon from './Icon.vue'

const props = defineProps<{ item: TunnelItem }>()
const emit = defineEmits<{ close: [] }>()

const lines = ref<string[]>([])
let timer: number | undefined
let total = 0 // 服务端日志总行数，用于增量拉取

const { copied, copyTip, copy } = useCopy(() => lines.value.join('\n'))

const createdAtText = computed(() => {
  if (!props.item.createdAt) return ''
  const d = new Date(props.item.createdAt * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}.${pad(d.getMonth() + 1)}.${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
})

// 卡片模式下日志面板头部被 ModalShell 隐藏，父组件通过这些暴露项在外层渲染复制按钮
defineExpose({ copied, copyTip, copy, hasLogs: computed(() => lines.value.length > 0), createdAtText })

async function load() {
  try {
    const res = await api.logs(props.item.id, total)
    if (res.logs?.length) {
      // total 往前回退说明服务端缓冲被截断或隧道重启，用返回内容整体替换重对齐
      if (res.total < total + res.logs.length) {
        lines.value = res.logs
      } else {
        lines.value = lines.value.concat(res.logs)
        // 与服务端 maxLogLines 对齐，防止本地无限增长
        if (lines.value.length > 200) {
          lines.value = lines.value.slice(-200)
        }
      }
    } else if (res.total < total) {
      // 无新增但 total 变小了（隧道重启清零），重新全量拉
      total = 0
      const full = await api.logs(props.item.id)
      lines.value = full.logs ?? []
      total = full.total
      return
    }
    total = res.total
  } catch {
    // 隧道可能刚被删除，静默忽略
  }
}

function startPolling() {
  stopPolling()
  timer = window.setInterval(() => {
    if (document.hidden) return
    load()
  }, 2000)
}

function stopPolling() {
  if (timer) {
    window.clearInterval(timer)
    timer = undefined
  }
}

function onVisibilityChange() {
  if (!document.hidden) load()
}

watch(() => props.item.id, () => {
  total = 0
  lines.value = []
  load()
})

onMounted(() => {
  load()
  startPolling()
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onUnmounted(() => {
  stopPolling()
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
</script>

<template>
  <div class="panel">
    <div class="head">
      <div class="title">
        <strong>{{ item.name }}</strong>
        <span v-if="createdAtText" class="created">创建于：{{ createdAtText }}</span>
      </div>
      <div class="actions">
        <span v-if="copyTip" class="copy-tip">{{ copyTip }}</span>
        <button
          class="icon-btn"
          :class="{ copied }"
          :disabled="!lines.length"
          :title="lines.length ? '复制日志' : '暂无日志'"
          aria-label="复制日志"
          @click="copy"
        ><Icon :name="copied ? 'check' : 'copy'" /></button>
        <button class="mini" @click="emit('close')">关闭</button>
      </div>
    </div>
    <pre v-if="lines.length">{{ lines.join('\n') }}</pre>
    <p v-else class="empty">暂无日志</p>
  </div>
</template>

<style scoped>
.panel {
  margin-top: 16px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
}

.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
}

.title {
  display: flex;
  align-items: baseline;
  gap: 10px;
  min-width: 0;
}

.created {
  font-size: 12px;
  color: var(--muted);
  white-space: nowrap;
}

.actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.copy-tip {
  font-size: 12px;
  color: var(--muted);
}

.icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 3px 8px;
}

.icon-btn svg {
  width: 15px;
  height: 15px;
  display: block;
}

.icon-btn.copied {
  background: var(--ok);
  border-color: var(--ok);
  color: var(--on-ok);
}

pre {
  margin: 0;
  padding: 12px 14px;
  max-height: 300px;
  overflow: auto;
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
  line-height: 1.6;
  color: var(--muted);
  white-space: pre-wrap;
  word-break: break-all;
}

.empty {
  margin: 0;
  padding: 16px 14px;
  color: var(--muted);
  font-size: 13px;
}

.mini {
  padding: 2px 10px;
  font-size: 12px;
}
</style>
