<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { api, type TunnelItem } from '../api'
import { useCopy } from '../composables/useCopy'

const props = defineProps<{ item: TunnelItem }>()
const emit = defineEmits<{ close: [] }>()

const lines = ref<string[]>([])
let timer: number | undefined

const { copied, copyTip, copy } = useCopy(() => lines.value.join('\n'))

const createdAtText = computed(() => {
  if (!props.item.createdAt) return ''
  const d = new Date(props.item.createdAt * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}.${pad(d.getMonth() + 1)}.${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
})

async function load() {
  try {
    lines.value = (await api.logs(props.item.id)).logs ?? []
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

watch(() => props.item.id, load)

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
        <button class="mini" :disabled="!lines.length" @click="copy">
          {{ copied ? '已复制' : '复制' }}
        </button>
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
