<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'

const props = defineProps<{ title: string }>()
const emit = defineEmits<{ close: [] }>()

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}

onMounted(() => {
  window.addEventListener('keydown', onKey)
  document.body.style.overflow = 'hidden'
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKey)
  document.body.style.overflow = ''
})
</script>

<template>
  <div class="mask" @mousedown.self="emit('close')">
    <div class="box" role="dialog" aria-modal="true">
      <div class="head">
        <div class="head-title">
          <strong>{{ props.title }}</strong>
          <slot name="title-extra" />
        </div>
        <div class="head-actions">
          <slot name="actions" />
          <button class="mini" @click="emit('close')">关闭</button>
        </div>
      </div>
      <div class="body">
        <slot />
      </div>
    </div>
  </div>
</template>

<style scoped>
.mask {
  position: fixed;
  inset: 0;
  background: rgba(8, 12, 22, 0.7);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 48px 16px;
  z-index: 100;
  overflow: auto;
}

.box {
  width: 100%;
  max-width: 720px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  max-height: calc(100vh - 96px);
}

.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
}

.head-title {
  display: flex;
  align-items: baseline;
  gap: 10px;
  min-width: 0;
}

.head-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.mini {
  padding: 2px 10px;
  font-size: 12px;
}

.body {
  padding: 16px;
  overflow: auto;
}

.body :deep(form.card) {
  margin-top: 0;
  border: none;
  padding: 0;
  background: transparent;
}

.body :deep(.panel) {
  margin-top: 0;
  border: none;
  background: transparent;
}

.body :deep(.panel .head) {
  display: none;
}

.body :deep(.panel pre) {
  max-height: none;
  background: var(--panel-2);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 14px 16px;
}

.body :deep(.panel .empty) {
  background: var(--panel-2);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 16px;
}

@media (max-width: 720px) {
  .mask {
    padding: 16px 12px;
  }

  .box {
    max-height: calc(100vh - 32px);
  }
}
</style>