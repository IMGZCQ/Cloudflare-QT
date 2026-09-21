<script setup lang="ts">
import { computed } from 'vue'
import type { TunnelItem } from '../api'
import { useCopy } from '../composables/useCopy'
import { useConfirm } from '../composables/useConfirm'
import { maskUrl } from '../utils'
import Icon from './Icon.vue'

const props = defineProps<{ item: TunnelItem; busy: boolean }>()
const emit = defineEmits<{
  start: []
  stop: []
  pause: []
  resume: []
  edit: []
  remove: []
  logs: []
}>()

const target = computed(
  () => `${props.item.scheme}://${props.item.host}:${props.item.port}${props.item.path || ''}`,
)

const running = computed(
  () =>
    props.item.state === 'running' ||
    props.item.state === 'starting' ||
    props.item.state === 'paused',
)

const { copied, copyTip: _copyTip, urlRef, copy } = useCopy(() => props.item.url)
// 卡片用按钮图标反馈代替文字提示，保留 copyTip 接口但模板不展示
void _copyTip
void urlRef // 消除 ts-plugin 对模板 ref 属性的"未读取"误报

const { confirming, doConfirm } = useConfirm(
  () => `[data-card-id="${props.item.id}"]`,
  (kind) => {
    if (kind === 'stop') emit('stop')
    else if (kind === 'remove') emit('remove')
  },
)
</script>

<template>
  <div class="card" :class="'state-' + item.state" :data-card-id="item.id" @pointerdown.self="confirming = ''">
    <!-- 标题行：左 状态点+名称+状态文字 / 右 状态对应的主按钮 -->
    <div class="title-row">
      <div class="title">
        <span class="dot" :class="item.state"></span>
        <strong>{{ item.name }}</strong>
        <span v-if="item.state === 'starting'" class="state">
          <span class="spinner"></span>
        </span>
      </div>
      <button
        v-if="item.state === 'stopped' || item.state === 'error'"
        class="primary"
        :disabled="busy"
        title="启动隧道（会分配新的公网地址）"
        aria-label="启动"
        @click="emit('start')"
      ><Icon name="play" /></button>
      <button
        v-else-if="item.state === 'running'"
        :disabled="busy"
        title="暂停后公网地址保持不变，访客将看到维护页面，可随时恢复"
        aria-label="暂停"
        @click="emit('pause')"
      ><Icon name="pause" /></button>
      <button
        v-else-if="item.state === 'paused'"
        class="primary"
        :disabled="busy"
        title="恢复后继续使用原公网地址"
        aria-label="恢复"
        @click="emit('resume')"
      ><Icon name="resume" /></button>
      <button v-else disabled aria-label="启动中"><Icon name="play" /></button>
    </div>

    <div class="target-row">
      <div class="target-text">
        {{ target }}<template v-if="item.pid"> · PID {{ item.pid }}</template>
      </div>
    </div>

    <div v-if="item.url" class="url-card">
      <a ref="urlRef" :href="item.url" target="_blank" rel="noreferrer">{{ maskUrl(item.url) }}</a>
    </div>
    <div v-else-if="item.lastError" class="err">{{ item.lastError }}</div>

    <div v-if="confirming" class="confirm">
      <div class="confirm-ops">
        <button :disabled="busy" @click="confirming = ''">取消</button>
        <button
          class="confirm-danger"
          :disabled="busy"
          @click="doConfirm"
        >
          {{ confirming === 'remove' ? '确认删除' : '确认停止' }}
        </button>
      </div>
    </div>

    <div v-else class="row">
      <button
        :class="{ copied }"
        :disabled="!item.url"
        :title="item.url ? '复制公网地址' : '暂无公网地址'"
        aria-label="复制"
        @click="copy"
      ><Icon :name="copied ? 'check' : 'copy'" /></button>
      <button title="编辑" aria-label="编辑" @click="emit('edit')"><Icon name="edit" /></button>
      <button title="日志" aria-label="日志" @click="emit('logs')"><Icon name="logs" /></button>
      <button
        v-if="running"
        :disabled="busy"
        title="停止后公网地址失效，再次启动会分配新的公网地址"
        aria-label="停止"
        @click="confirming = 'stop'"
      ><Icon name="stop" /></button>
      <button
        v-else
        class="danger"
        title="删除后配置和公网地址都会失效"
        aria-label="删除"
        @click="confirming = 'remove'"
      ><Icon name="trash" /></button>
    </div>
  </div>
</template>

<style scoped>
.card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 18px 18px 16px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-left: 3px solid var(--muted);
  border-radius: 12px;
  min-width: 0;
}

.card.state-running {
  border-left-color: var(--ok);
}

.card.state-starting {
  border-left-color: var(--accent);
}

.card.state-paused {
  border-left-color: var(--warn);
}

.card.state-error {
  border-left-color: var(--err);
}

.title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  min-width: 0;
  flex: 1 1 auto;
}

.dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--muted);
  flex-shrink: 0;
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

.target-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.target-text {
  color: var(--muted);
  font-size: 12px;
  word-break: break-all;
}

.url-card {
  padding: 10px 12px;
  background: var(--panel-2);
  border: 1px solid var(--border);
  border-radius: 8px;
}

.url-card a {
  color: var(--accent);
  font-size: 13px;
  word-break: break-all;
}

.tip {
  font-size: 12px;
  color: var(--ok-text);
  white-space: nowrap;
}

.err {
  color: var(--err-text);
  font-size: 12px;
}

.row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
}

.title-row button {
  padding: 2px 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.title-row button svg {
  width: 20px;
  height: 20px;
}

.row button {
  width: 100%;
  padding: 6px 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.row button.copied {
  background: var(--ok);
  border-color: var(--ok);
  color: var(--on-ok);
  font-weight: 600;
}

.confirm {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding: 4px 0;
}

.confirm-ops {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}
</style>