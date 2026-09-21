<script setup lang="ts">
import { computed } from 'vue'
import type { TunnelItem } from '../api'
import { useCopy } from '../composables/useCopy'
import { useConfirm } from '../composables/useConfirm'
import { maskUrl } from '../utils'
import Icon from './Icon.vue'

const props = defineProps<{ item: TunnelItem; busy: boolean }>()
const emit = defineEmits<{ start: []; stop: []; pause: []; resume: []; edit: []; remove: []; logs: [] }>()

const stateText: Record<TunnelItem['state'], string> = {
  stopped: '已停止',
  starting: '启动中',
  running: '运行中',
  paused: '已暂停',
  error: '异常',
}

const running = computed(() => props.item.state === 'running' || props.item.state === 'starting' || props.item.state === 'paused')
const target = computed(() => `${props.item.scheme}://${props.item.host}:${props.item.port}${props.item.path || ''}`)

const { copyTip, urlRef, copy } = useCopy(() => props.item.url)
void urlRef // 消除 ts-plugin 对模板 ref 属性的"未读取"误报

const { confirming, doConfirm } = useConfirm(
  () => `[data-row-id="${props.item.id}"]`,
  (kind) => {
    if (kind === 'stop') emit('stop')
    else if (kind === 'remove') emit('remove')
  },
)
</script>

<template>
  <div class="row" :class="'state-' + item.state" :data-row-id="item.id" @pointerdown.self="confirming = ''">
    <div class="info">
      <div class="title">
        <span class="dot" :class="item.state"></span>
        <strong>{{ item.name }}</strong>
        <span class="state">{{ stateText[item.state] }}<span v-if="item.state === 'starting'" class="spinner"></span></span>
        <span class="target"> {{ target }}<template v-if="item.pid"> · PID {{ item.pid }}</template></span>
      </div>
      <div v-if="item.url" class="url">
        <a ref="urlRef" :href="item.url" target="_blank" rel="noreferrer">{{ maskUrl(item.url) }}</a>
        <button class="mini" title="复制" aria-label="复制" @click="copy"><Icon name="copy" /></button>
        <span v-if="copyTip" class="tip">{{ copyTip }}</span>
      </div>
      <div v-else-if="item.lastError" class="err">{{ item.lastError }}</div>
    </div>
    <div v-if="confirming" class="ops confirm">
      <button :disabled="busy" @click="confirming = ''">取消</button>
      <button
        class="confirm-danger"
        :disabled="busy"
        @click="doConfirm"
      >
        {{ confirming === 'remove' ? '确认删除' : '确认停止' }}
      </button>
    </div>
    <div v-else class="ops">
      <button
        v-if="!running"
        class="primary"
        :disabled="busy"
        :title="item.state === 'paused' ? '重新启动会断开当前连接并分配新的公网地址' : '启动隧道（会分配新的公网地址）'"
        aria-label="启动"
        @click="emit('start')"
      ><Icon name="play" /></button>
      <button
        v-if="item.state === 'running'"
        :disabled="busy"
        title="暂停后公网地址保持不变，访客将看到维护页面，可随时恢复"
        aria-label="暂停"
        @click="emit('pause')"
      ><Icon name="pause" /></button>
      <button
        v-if="item.state === 'paused'"
        class="primary"
        :disabled="busy"
        title="恢复后继续使用原公网地址，访客请求将正常转发到本地服务"
        aria-label="恢复"
        @click="emit('resume')"
      ><Icon name="resume" /></button>
      <button
        v-if="running"
        :disabled="busy"
        title="停止后公网地址失效，再次启动会分配新的公网地址"
        aria-label="停止"
        @click="confirming = 'stop'"
      ><Icon name="stop" /></button>
      <button :disabled="busy" title="编辑" aria-label="编辑" @click="emit('edit')"><Icon name="edit" /></button>
      <button :disabled="busy" title="日志" aria-label="日志" @click="emit('logs')"><Icon name="logs" /></button>
      <button class="danger" :disabled="busy" title="删除" aria-label="删除" @click="confirming = 'remove'"><Icon name="trash" /></button>
    </div>
  </div>
</template>

<style scoped>
.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 16px;
  padding: 14px 16px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-left: 3px solid var(--muted);
  border-radius: 8px;
}

.row.state-running {
  border-left-color: var(--ok);
}

.row.state-starting {
  border-left-color: var(--accent);
}

.row.state-paused {
  border-left-color: var(--warn);
}

.row.state-error {
  border-left-color: var(--err);
}

.info {
  flex: 1 1 auto;
  min-width: 0;
}

.title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--muted);
}

.dot.running {
  background: var(--ok);
}

.dot.starting {
  background: var(--accent);
}

.dot.paused {
  background: var(--warn);
}

.dot.error {
  background: var(--err);
}

.state {
  color: var(--muted);
  font-size: 12px;
}

.spinner {
  display: inline-block;
  width: 12px;
  height: 12px;
  border: 2px solid var(--ok);
  border-top-color: transparent;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  vertical-align: middle;
  margin-left: 4px;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.target {
  color: var(--muted);
  font-size: 12px;
  white-space: nowrap;
}

.url {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 6px;
}

.url a {
  color: var(--accent);
  font-size: 13px;
  word-break: break-all;
}

.err {
  margin-top: 6px;
  color: var(--err-text);
  font-size: 12px;
}

.mini {
  padding: 2px 8px;
  font-size: 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.tip {
  font-size: 12px;
  color: var(--ok-text);
  white-space: nowrap;
}

.ops {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.ops button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 6px 8px;
}

.confirm {
  align-items: center;
  flex-shrink: 1;
  min-width: 0;
}

.confirm-text {
  color: var(--warn-text);
  font-size: 12px;
  text-align: right;
}
</style>
