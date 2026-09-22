<script setup lang="ts">
import { computed } from 'vue'
import type { TunnelItem } from '../api'
import { useCopy } from '../composables/useCopy'
import { useConfirm } from '../composables/useConfirm'
import Icon from './Icon.vue'

const props = defineProps<{ item: TunnelItem; busy: boolean }>()
const emit = defineEmits<{
  start: []
  stop: []
  pause: []
  resume: []
}>()

const running = computed(
  () =>
    props.item.state === 'running' ||
    props.item.state === 'starting' ||
    props.item.state === 'paused',
)

const { copied, copyTip: _copyTip, urlRef, copy } = useCopy(() => props.item.url)
void _copyTip
void urlRef

const { confirming, doConfirm } = useConfirm(
  () => `[data-compact-id="${props.item.id}"]`,
  () => emit('stop'),
)

function onCardClick() {
  if (props.item.url) copy()
}
</script>

<template>
  <div
    class="compact"
    :class="['state-' + item.state, { clickable: !!item.url }]"
    :data-compact-id="item.id"
    :title="item.url ? '点击复制公网地址' : ''"
    @pointerdown.self="confirming = ''"
    @click="onCardClick"
  >
    <div class="head">
      <span class="dot" :class="item.state"></span>
      <strong v-if="copied" class="name copied-text">已复制地址</strong>
      <strong v-else class="name" :title="item.name">{{ item.name }}</strong>
      <span v-if="item.state === 'starting'" class="spinner"></span>
    </div>

    <div v-if="confirming" class="confirm">
      <button :disabled="busy" title="取消" aria-label="取消" @click.stop="confirming = ''"><Icon name="check" /></button>
      <button
        class="confirm-danger"
        :disabled="busy"
        title="确认停止"
        aria-label="确认停止"
        @click.stop="doConfirm"
      ><Icon name="stop" /></button>
    </div>

    <div v-else class="ops">
      <button
        v-if="item.state === 'stopped' || item.state === 'error'"
        class="primary"
        :disabled="busy"
        title="启动隧道（会分配新的公网地址）"
        aria-label="启动"
        @click.stop="emit('start')"
      ><Icon name="play" /></button>
      <button
        v-else-if="item.state === 'running'"
        :disabled="busy"
        title="暂停后公网地址保持不变，可随时恢复"
        aria-label="暂停"
        @click.stop="emit('pause')"
      ><Icon name="pause" /></button>
      <button
        v-else-if="item.state === 'paused'"
        class="primary"
        :disabled="busy"
        title="恢复后继续使用原公网地址"
        aria-label="恢复"
        @click.stop="emit('resume')"
      ><Icon name="resume" /></button>
      <button v-else disabled aria-label="启动中"><Icon name="play" /></button>

      <button
        v-if="running"
        :disabled="busy"
        title="停止后公网地址失效，再次启动会分配新的公网地址"
        aria-label="停止"
        @click.stop="confirming = 'stop'"
      ><Icon name="stop" /></button>
    </div>
  </div>
</template>

<style scoped>
.compact {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 8px 10px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-left: 3px solid var(--muted);
  border-radius: 8px;
  min-width: 0;
  box-shadow: var(--shadow-sm);
  transition: box-shadow 0.15s ease;
}

.compact:hover {
  box-shadow: var(--shadow);
}

.compact.state-running {
  border-left-color: var(--ok);
}

.compact.state-starting {
  border-left-color: var(--accent);
}

.compact.state-paused {
  border-left-color: var(--warn);
}

.compact.state-error {
  border-left-color: var(--err);
}

.head {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
  flex: 1 1 auto;
}

.dot {
  width: 9px;
  height: 9px;
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

.name {
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.compact.clickable {
  cursor: pointer;
}

.copied-text {
  color: var(--ok);
}

.spinner {
  display: inline-block;
  width: 10px;
  height: 10px;
  border: 2px solid var(--ok);
  border-top-color: transparent;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  flex-shrink: 0;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.ops,
.confirm {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}

.ops button,
.confirm button {
  padding: 3px 7px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.ops button svg,
.confirm button svg {
  width: 14px;
  height: 14px;
  display: block;
}

.ops button.copied {
  background: var(--ok);
  border-color: var(--ok);
  color: var(--on-ok);
}
</style>
