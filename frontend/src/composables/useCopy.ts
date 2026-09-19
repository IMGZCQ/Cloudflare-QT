import { onUnmounted, ref } from 'vue'

// 复制到剪贴板的多级兜底逻辑：clipboard API → execCommand → 选中提示手动复制
export function useCopy(getUrl: () => string) {
  const copied = ref(false)
  const copyTip = ref('')
  const urlRef = ref<HTMLElement | null>(null)
  let copiedTimer = 0
  let tipTimer = 0

  function flashCopied() {
    copied.value = true
    clearTimeout(copiedTimer)
    copiedTimer = window.setTimeout(() => (copied.value = false), 1500)
  }

  function showTip(text: string) {
    copyTip.value = text
    clearTimeout(tipTimer)
    tipTimer = window.setTimeout(() => (copyTip.value = ''), 1800)
  }

  function legacyCopy(text: string): boolean {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.setAttribute('readonly', '')
    ta.style.cssText = 'position:fixed;top:0;left:0;width:1px;height:1px;opacity:0;'
    document.body.appendChild(ta)
    ta.select()
    ta.setSelectionRange(0, text.length)
    let ok = false
    try {
      ok = document.execCommand('copy')
    } catch {
      ok = false
    }
    document.body.removeChild(ta)
    return ok
  }

  function selectUrl() {
    const el = urlRef.value
    if (!el) return
    const range = document.createRange()
    range.selectNodeContents(el)
    const sel = window.getSelection()
    sel?.removeAllRanges()
    sel?.addRange(range)
  }

  async function copy() {
    const url = getUrl()
    if (!url) return
    if (navigator.clipboard && window.isSecureContext) {
      try {
        await navigator.clipboard.writeText(url)
        flashCopied()
        showTip('已复制')
        return
      } catch {
        // 权限策略未放开 clipboard-write，走兜底
      }
    }
    if (legacyCopy(url)) {
      flashCopied()
      showTip('已复制')
      return
    }
    selectUrl()
    showTip('已选中，请按 Ctrl+C')
  }

  onUnmounted(() => {
    clearTimeout(copiedTimer)
    clearTimeout(tipTimer)
  })

  return { copied, copyTip, urlRef, copy, showTip, flashCopied }
}
