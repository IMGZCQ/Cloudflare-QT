import { onMounted, onUnmounted, ref, type Ref } from 'vue'

type ConfirmKind = 'stop' | 'remove' | ''

// 行内二次确认：点击"停止/删除"先进入确认态，点击其他区域或"取消"退出
// containerSelector: 用于判断点击是否发生在当前行/卡片内（如 [data-row-id="xxx"]）
export function useConfirm(
  containerSelector: () => string,
  onConfirm: (kind: 'stop' | 'remove') => void,
) {
  const confirming: Ref<ConfirmKind> = ref('')

  function doConfirm() {
    const kind = confirming.value
    confirming.value = ''
    if (kind === 'stop' || kind === 'remove') onConfirm(kind)
  }

  function onDocPointerDown(e: PointerEvent) {
    if (!confirming.value) return
    const target = e.target as Node | null
    const el = document.querySelector(containerSelector())
    if (el && el.contains(target)) return
    confirming.value = ''
  }

  onMounted(() => document.addEventListener('pointerdown', onDocPointerDown))
  onUnmounted(() => document.removeEventListener('pointerdown', onDocPointerDown))

  return { confirming, doConfirm }
}
