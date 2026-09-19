<script setup lang="ts">
import { reactive, watch } from 'vue'
import type { TunnelItem, TunnelPayload } from '../api'

const props = defineProps<{ editing: TunnelItem | null }>()
const emit = defineEmits<{ submit: [TunnelPayload]; cancel: [] }>()

const form = reactive({
  name: '',
  target: 'http://127.0.0.1:8080',
  autoStart: false,
})

watch(
  () => props.editing,
  (item) => {
    form.name = item?.name ?? ''
    form.target = item ? `${item.scheme}://${item.host}:${item.port}${item.path}` : 'http://127.0.0.1:8080'
    form.autoStart = item?.autoStart ?? true
  },
  { immediate: true },
)

function submit() {
  emit('submit', {
    name: form.name,
    target: form.target,
    autoStart: form.autoStart,
  } as TunnelPayload)
}
</script>

<template>
  <form class="card" @submit.prevent="submit">
    <h2>{{ props.editing ? '编辑隧道' : '新增隧道' }}</h2>
    <div class="grid">
      <label>
        <span>隧道名称</span>
        <input v-model="form.name" placeholder="（选填）" />
      </label>
      <label>
        <span>输入地址（协议://地址:端口/路径 等）</span>
        <input v-model="form.target" placeholder="http://127.0.0.1:8080" required />
      </label>
    </div>
    <label class="check">
      <input v-model="form.autoStart" type="checkbox" />
      <span>立即启动 并 自动启动</span>
    </label>
    <div class="actions">
      <button type="button" @click="emit('cancel')">取消</button>
      <button type="submit" class="primary">{{ props.editing ? '保存' : '创建' }}</button>
    </div>
  </form>
</template>

<style scoped>
.card {
  margin-top: 16px;
  padding: 16px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-left: 3px solid var(--blue);
  border-radius: 8px;
}

h2 {
  margin: 0 0 14px;
  font-size: 15px;
}

label {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

label + label {
  margin-top: 12px;
}

.grid {
  display: grid;
  grid-template-columns: 1fr 2fr;
  gap: 12px;
  margin-top: 0;
}

.grid label + label {
  margin-top: 0;
}

@media (max-width: 720px) {
  .grid {
    grid-template-columns: 1fr;
  }
}

label span {
  color: var(--muted);
  font-size: 12px;
}

.check {
  flex-direction: row;
  align-items: center;
  gap: 8px;
  margin-top: 14px;
}

.check input {
  width: auto;
}

.check span {
  font-size: 13px;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 16px;
}
</style>
