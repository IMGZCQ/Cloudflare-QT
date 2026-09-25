<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { api, type BinaryStatus, type TunnelItem, type TunnelPayload } from './api'
import { appVersion } from './appInfo'
import TunnelForm from './components/TunnelForm.vue'
import TunnelRow from './components/TunnelRow.vue'
import TunnelCard from './components/TunnelCard.vue'
import TunnelCompactCard from './components/TunnelCompactCard.vue'
import LogPanel from './components/LogPanel.vue'
import ModalShell from './components/ModalShell.vue'
import AboutDialog from './components/AboutDialog.vue'
import Icon from './components/Icon.vue'

const items = ref<TunnelItem[]>([])
const binary = ref<BinaryStatus | null>(null)
const message = ref('')
const messageKind = ref<'ok' | 'err' | 'hint'>('ok')
const busy = ref<Record<string, boolean>>({})
const formOpen = ref(false)
const editing = ref<TunnelItem | null>(null)
const logId = ref('')
const aboutOpen = ref(false)

// 视图模式：list（列表） / card（卡片） / compact（极简），持久化到 localStorage
type ViewMode = 'list' | 'card' | 'compact'
const VIEW_KEY = 'cfqt.viewMode'
const VIEW_MODES: ViewMode[] = ['list', 'card', 'compact']
const storedView = localStorage.getItem(VIEW_KEY) as ViewMode | null
const viewMode = ref<ViewMode>(storedView && VIEW_MODES.includes(storedView) ? storedView : 'list')
watch(viewMode, (v) => localStorage.setItem(VIEW_KEY, v))

function cycleView() {
  const idx = VIEW_MODES.indexOf(viewMode.value)
  viewMode.value = VIEW_MODES[(idx + 1) % VIEW_MODES.length]
}

// 主题：dark（深色，默认） / light（浅色），同步到 <html data-theme> 并持久化
type Theme = 'dark' | 'light'
const THEME_KEY = 'cfqt.theme'
const theme = ref<Theme>(
  (localStorage.getItem(THEME_KEY) as Theme | null) === 'light' ? 'light' : 'dark',
)
function applyTheme(v: Theme) {
  if (v === 'light') document.documentElement.dataset.theme = 'light'
  else delete document.documentElement.dataset.theme
}
applyTheme(theme.value)
watch(theme, (v) => {
  applyTheme(v)
  localStorage.setItem(THEME_KEY, v)
})

let timer: number | undefined
let etag = ''

// 首次/唯一隧道提示：创建成功且当前仅此一条时给出初始化提示，10 秒后消失

function notify(text: string, kind: 'ok' | 'err' | 'hint' = 'ok', duration = 5000) {
  message.value = text
  messageKind.value = kind
  if (text) {
    window.setTimeout(() => {
      if (message.value === text) message.value = ''
    }, duration)
  }
}

function notifyFirstTunnel() {
  notify('首次建立隧道需要初始化，请稍后或尝试刷新…', 'hint', 15000)
}

async function refresh() {
  try {
    const result = await api.list(etag)
    if (result.notModified) return // 数据无变化，跳过重渲染
    etag = result.etag
    items.value = result.items ?? []
    binary.value = result.binary
  } catch (e) {
    notify((e as Error).message, 'err')
  }
}

function startPolling() {
  stopPolling()
  timer = window.setInterval(() => {
    // 页面不可见时跳过轮询，减少后台 CPU/网络开销
    if (document.hidden) return
    refresh()
  }, 3000)
}

function stopPolling() {
  if (timer) {
    window.clearInterval(timer)
    timer = undefined
  }
}

function onVisibilityChange() {
  // 页面重新可见时立即刷新一次，保证数据及时
  if (!document.hidden) refresh()
}

// 统一处理返回结构：后端在“配置已保存但启动失败”时会同时返回 item 和 error
function applyResult(res: { item?: TunnelItem; error?: string }) {
  if (res.item) {
    const idx = items.value.findIndex((i) => i.id === res.item!.id)
    if (idx >= 0) items.value[idx] = res.item
    else items.value.push(res.item)
  }
  if (res.error) notify(res.error, 'err')
}

async function withBusy(id: string, fn: () => Promise<void>) {
  busy.value = { ...busy.value, [id]: true }
  try {
    await fn()
  } catch (e) {
    notify((e as Error).message, 'err')
  } finally {
    busy.value = { ...busy.value, [id]: false }
  }
}

function openCreate() {
  editing.value = null
  formOpen.value = true
}

function openEdit(item: TunnelItem) {
  editing.value = item
  formOpen.value = true
}

function closeForm() {
  formOpen.value = false
  editing.value = null
}

async function submitForm(payload: TunnelPayload) {
  const id = editing.value?.id
  // 记录创建前是否为空列表：用于判断这是当前唯一一条隧道
  const wasEmpty = !id && items.value.length === 0
  await withBusy(id ?? 'new', async () => {
    // 后端保存后立刻返回（需要重启时在后台进行），这里直接关表单，条目以“启动中”呈现
    const res = id ? await api.update(id, payload) : await api.create(payload)
    formOpen.value = false
    editing.value = null
    applyResult(res)
    if (!res.error) {
      if (wasEmpty) {
        notifyFirstTunnel()
      } else {
        notify(id ? '已保存' : '已新增隧道')
      }
    }
    await refresh()
  })
}

async function start(item: TunnelItem) {
  await withBusy(item.id, async () => {
    applyResult(await api.start(item.id))
  })
}

async function stop(item: TunnelItem) {
  await withBusy(item.id, async () => {
    applyResult(await api.stop(item.id))
    notify('已停止')
  })
}

async function pause(item: TunnelItem) {
  await withBusy(item.id, async () => {
    applyResult(await api.pause(item.id))
    notify('已暂停')
  })
}

async function resume(item: TunnelItem) {
  await withBusy(item.id, async () => {
    applyResult(await api.resume(item.id))
    notify('已恢复')
  })
}

async function remove(item: TunnelItem) {
  await withBusy(item.id, async () => {
    await api.remove(item.id)
    items.value = items.value.filter((i) => i.id !== item.id)
    if (logId.value === item.id) logId.value = ''
    notify('已删除')
  })
}

const logItem = computed<TunnelItem | null>(
  () => items.value.find((i) => i.id === logId.value) ?? null,
)

// 卡片模式下 LogPanel 头部被 ModalShell 隐藏，通过 ref 拿它的创建于/复制状态放到弹窗头部
type LogPanelExpose = {
  copied: boolean
  copyTip: string
  copy: () => void
  hasLogs: boolean
  createdAtText: string
}
const logPanelRef = ref<LogPanelExpose | null>(null)

const logTitle = computed(() => (logItem.value ? `${logItem.value.name}` : '日志'))

async function download() {
  await withBusy('binary', async () => {
    binary.value = await api.downloadBinary()
    notify('cloudflared 已就绪')
  })
}

onMounted(() => {
  refresh()
  startPolling()
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onUnmounted(() => {
  stopPolling()
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
</script>

<template>
  <div class="page">
    <header>
      <div>
        <h1>
          Cloudflare快捷隧道
          <span v-if="appVersion" class="ver">v{{ appVersion }}</span>
        </h1>
        <p class="sub">快速把本地服务暴露到公网，小白的福音，开发者的调试利器</p>
      </div>
      <div class="ops">
        <button
          class="view-toggle"
          :title="viewMode === 'list' ? '切换到卡片视图' : viewMode === 'card' ? '切换到极简视图' : '切换到列表视图'"
          :aria-label="viewMode === 'list' ? '当前为列表视图，点击切换到卡片视图' : viewMode === 'card' ? '当前为卡片视图，点击切换到极简视图' : '当前为极简视图，点击切换到列表视图'"
          @click="cycleView"
        >
          <Icon :name="viewMode === 'list' ? 'view-list' : viewMode === 'card' ? 'view-grid' : 'view-compact'" />
        </button>
        <button
          class="view-toggle"
          :title="theme === 'dark' ? '切换到浅色模式' : '切换到深色模式'"
          :aria-label="theme === 'dark' ? '当前为深色模式，点击切换到浅色模式' : '当前为浅色模式，点击切换到深色模式'"
          @click="theme = theme === 'dark' ? 'light' : 'dark'"
        >
          <Icon :name="theme === 'dark' ? 'sun' : 'moon'" />
        </button>
        <button @click="aboutOpen = true">关于</button>
        <button class="primary" :disabled="busy['new']" @click="openCreate">新增隧道</button>
      </div>
    </header>

    <div v-if="message || binary" class="binary" :class="{ warn: !message && binary && !binary.ready, err: message && messageKind === 'err', hint: message && messageKind === 'hint' }">
      <span v-if="message">{{ message }}</span>
      <template v-else-if="binary">
        <span v-if="binary.ready">
          cloudflared 就绪<template v-if="binary.version"> · {{ binary.version }}</template>
        </span>
        <span v-else-if="binary.downloading">
          {{ binary.message || '正在下载 Cloudflared' }} · {{ binary.progress }}%
        </span>
        <span v-else>未找到 Cloudflared：{{ binary.message || binary.path }}</span>
        <button v-if="!binary.ready" :disabled="binary.downloading || busy['binary']" @click="download">
          立即下载
        </button>
      </template>
    </div>

    <TunnelForm
      v-if="formOpen && !editing"
      :editing="null"
      @submit="submitForm"
      @cancel="closeForm"
    />

    <div v-if="!items.length && !formOpen" class="empty">还没有隧道，点击“新增隧道”创建。</div>

    <!-- 列表视图：行内展开编辑表单和日志 -->
    <div v-else-if="items.length && viewMode === 'list'" class="list">
      <template v-for="item in items" :key="item.id">
        <TunnelForm
          v-if="editing && editing.id === item.id"
          :editing="editing"
          @submit="submitForm"
          @cancel="closeForm"
        />
        <TunnelRow
          v-else
          :item="item"
          :busy="!!busy[item.id]"
          @start="start(item)"
          @stop="stop(item)"
          @pause="pause(item)"
          @resume="resume(item)"
          @edit="openEdit(item)"
          @remove="remove(item)"
          @logs="logId = logId === item.id ? '' : item.id"
        />
        <LogPanel v-if="logId === item.id" :item="item" @close="logId = ''" />
      </template>
    </div>

    <!-- 卡片视图：自适应网格；编辑表单与日志用全屏浮层承载 -->
    <div v-else-if="items.length && viewMode === 'card'" class="grid">
      <TunnelCard
        v-for="item in items"
        :key="item.id"
        :item="item"
        :busy="!!busy[item.id]"
        @start="start(item)"
        @stop="stop(item)"
        @pause="pause(item)"
        @resume="resume(item)"
        @edit="openEdit(item)"
        @remove="remove(item)"
        @logs="logId = logId === item.id ? '' : item.id"
      />
    </div>

    <!-- 极简视图：更密的自适应网格，仅保留名称/状态与常用操作 -->
    <div v-else-if="items.length" class="compact-grid">
      <TunnelCompactCard
        v-for="item in items"
        :key="item.id"
        :item="item"
        :busy="!!busy[item.id]"
        @start="start(item)"
        @stop="stop(item)"
        @pause="pause(item)"
        @resume="resume(item)"
      />
    </div>

    <ModalShell v-if="viewMode !== 'list' && editing" :title="'编辑隧道'" @close="closeForm">
      <TunnelForm :editing="editing" @submit="submitForm" @cancel="closeForm" />
    </ModalShell>
    <ModalShell
      v-if="viewMode !== 'list' && logId"
      class="log-modal"
      :title="logTitle"
      @close="logId = ''"
    >
      <template #title-extra>
        <span v-if="logPanelRef?.createdAtText" class="created">创建于：{{ logPanelRef.createdAtText }}</span>
      </template>
      <template #actions>
        <span v-if="logPanelRef?.copyTip" class="copy-tip">{{ logPanelRef.copyTip }}</span>
        <button
          class="icon-btn"
          :class="{ copied: logPanelRef?.copied }"
          :disabled="!logPanelRef?.hasLogs"
          :title="logPanelRef?.hasLogs ? '复制日志' : '暂无日志'"
          aria-label="复制日志"
          @click="logPanelRef?.copy()"
        ><Icon :name="logPanelRef?.copied ? 'check' : 'copy'" /></button>
      </template>
      <LogPanel v-if="logItem" ref="logPanelRef" :item="logItem" @close="logId = ''" />
    </ModalShell>

    <AboutDialog v-if="aboutOpen" @close="aboutOpen = false" />
  </div>
</template>

<style scoped>
.created {
  font-size: 12px;
  color: var(--muted);
  white-space: nowrap;
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

/* 卡片/极简模式下日志弹窗与列表模式同宽 */
.log-modal :deep(.box) {
  max-width: 900px;
}

.page {
  max-width: 940px;
  margin: 0 auto;
  padding: 24px 20px 48px;
}

header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

header > div:first-child {
  min-width: 0;
}

.ops {
  display: flex;
  gap: 10px;
  flex-shrink: 0;
}

h1 {
  margin: 0;
  font-size: 22px;
}

.ver {
  margin-left: 8px;
  font-size: 12px;
  font-weight: normal;
  color: var(--muted);
  vertical-align: middle;
}

.sub {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 13px;
}

.binary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 16px;
  padding: 10px 14px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-left: 5px solid var(--ok);
  border-radius: 6px;
  color: var(--muted);
  font-size: 13px;
  box-shadow: var(--shadow-sm);
}

.binary.warn {
  border-left-color: var(--accent);
}

.binary.err {
  border-left-color: var(--err);
  color: var(--err-text);
}

/* 唯一隧道初始化提示：更大、加粗、亮黄色 */
.binary.hint {
  border-left-color: var(--accent);
  color: var(--accent);
  font-size: 15px;
  font-weight: 700;
}

.list {
  margin-top: 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* 行内编辑表单和日志面板的间距由列表的 gap 控制 */
.list :deep(form.card),
.list :deep(.panel) {
  margin-top: 0;
}

/* 卡片视图：自适应网格。940px 容器默认放下 3 列（280px × 3 + 14 × 2 = 868） */
.grid {
  margin-top: 16px;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 14px;
  align-items: stretch;
}

.grid > :deep(.card) {
  height: 100%;
}

/* 极简视图：更密的网格，940px 容器一行 4 列 */
.compact-grid {
  margin-top: 16px;
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
}

@media (max-width: 940px) {
  .compact-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 720px) {
  .compact-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 440px) {
  .compact-grid {
    grid-template-columns: 1fr;
  }
}

.view-toggle {
  min-width: 56px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 6px 12px;
}

.view-toggle svg {
  width: 18px;
  height: 18px;
}

.empty {
  margin-top: 40px;
  text-align: center;
  color: var(--muted);
}

@media (max-width: 720px) {
  .page {
    padding: 16px 14px 40px;
  }

  header {
    flex-direction: column;
    align-items: stretch;
  }

  .ops button {
    flex: 1;
  }

  .binary {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }
}
</style>
