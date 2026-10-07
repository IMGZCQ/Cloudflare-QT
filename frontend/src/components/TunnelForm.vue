<script setup lang="ts">
import { reactive, watch } from 'vue'
import type { TunnelItem, TunnelPayload, TunnelType } from '../api'

const props = defineProps<{ editing: TunnelItem | null }>()
const emit = defineEmits<{ submit: [TunnelPayload]; cancel: [] }>()

const form = reactive({
  name: '',
  type: 'quick' as TunnelType,
  token: '',
  target: 'http://127.0.0.1:5666',
  edgeIpVersion: '4',
  autoStart: false,
})

watch(
  () => props.editing,
  (item) => {
    form.name = item?.name ?? ''
    form.type = item?.type ?? 'quick'
    form.token = item?.token ?? ''
    form.target = item ? `${item.scheme}://${item.host}:${item.port}${item.path}` : 'http://127.0.0.1:5666'
    form.edgeIpVersion = item?.edgeIpVersion || '4'
    form.autoStart = item?.autoStart ?? true
  },
  { immediate: true },
)

function submit() {
  emit('submit', {
    name: form.name,
    type: form.type,
    token: form.type === 'named' ? form.token : '',
    target: form.type === 'quick' ? form.target : '',
    edgeIpVersion: form.edgeIpVersion,
    autoStart: form.autoStart,
  } as TunnelPayload)
}

// 兼容用户整段粘贴安装/运行命令：从 "cloudflared.exe service install <token>" 中提取 Token
function extractToken(raw: string): string {
  const s = raw.trim()
  if (!s) return ''
  const fields = s.split(/\s+/)
  for (let i = fields.length - 1; i >= 0; i--) {
    const f = fields[i].replace(/^["']+|["']+$/g, '')
    if (f.startsWith('eyJ') && f.length > 20) return f
  }
  return fields[fields.length - 1].replace(/^["']+|["']+$/g, '')
}

function onTokenPaste(e: ClipboardEvent) {
  const text = e.clipboardData?.getData('text') ?? ''
  const extracted = extractToken(text)
  // 仅当粘贴的是整条命令（提取结果与原文本不同）时接管，避免影响正常粘贴
  if (extracted && extracted !== text.trim()) {
    e.preventDefault()
    form.token = extracted
  }
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
        <span>隧道类型</span>
        <select v-model="form.type" :disabled="!!props.editing" :title="props.editing ? '隧道类型创建后不可更改' : ''">
          <option value="quick">快捷隧道</option>
          <option value="named">命名隧道</option>
        </select>
      </label>
      <label>
        <span>隧道连接方式</span>
        <select v-model="form.edgeIpVersion">
          <option value="auto">自动</option>
          <option value="4">IPv4</option>
          <option value="6">IPv6</option>
        </select>
      </label>
      <label v-if="form.type === 'quick'">
        <span>输入地址（协议://地址:端口/路径 等）</span>
        <input v-model="form.target" placeholder="http://127.0.0.1:8080" required />
      </label>
      <label v-else>
        <span>Token</span>
        <input
          v-model="form.token"
          placeholder="粘贴命名隧道 Token，或直接粘贴 cloudflared 安装命令"
          required
          @paste="onTokenPaste"
          @blur="form.token = extractToken(form.token)"
        />
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
  border-left: 5px solid var(--blue);
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
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
  grid-template-columns: 1fr auto auto 2fr;
  gap: 12px;
  margin-top: 0;
}

.grid label + label {
  margin-top: 0;
}

.grid select {
  min-width: 92px;
}

.grid select:disabled {
  opacity: 0.6;
  cursor: not-allowed;
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
