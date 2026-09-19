<script setup lang="ts">
import { ref } from 'vue'
import { appVersion } from '../appInfo'

const emit = defineEmits<{ close: [] }>()

const showPay = ref(false)

const version = appVersion || '未知'

const info = [
  { k: '开发者：', v: '米恋泥' },
  { k: '企鹅群：', v: '1039270739' },
  { k: '版本号：', v: version },
  { k: '哩个系：', v: 'Cloudflared（Quick Tunnel）' },
]
</script>

<template>
  <div class="mask" @click.self="emit('close')">
    <div class="dialog" role="dialog" aria-modal="true" aria-label="关于 Cloudflare快捷隧道">
      <div class="head">
        <strong>关于 Cloudflare快捷隧道</strong>
        <button class="mini" @click="emit('close')">关闭</button>
      </div>
      <div class="body">
        <p>
          无需注册 Cloudflare 账号、无需 API 密钥或 Token，直接为本地服务创建临时隧道，
          由 Cloudflare 自动分配一个公网域名，简直是小白的福音，开发者的调试利器。
        </p>

        <h3>使用说明</h3>
        <ol>
          <li>新增隧道 > 填写 地址 和 端口 > 保存 > 启动 OK？</li>
<!--           <li>首次使用先确认顶部提示 cloudflared 已就绪，缺失时点「立即下载」自动获取。</li>
          <li>点「新增隧道」填写本地地址与端口（如 <code>127.0.0.1:8080</code>）后保存。</li>
          <li>启动需要十几秒，条目会从「启动中」变为「运行中」并显示公网地址，可一键复制。</li>
          <li>编辑时只改备注或自动启动不会断开连接，仅当地址或端口变化才在后台重启隧道。</li>
          <li>点「日志」可查看 cloudflared 的实时输出，启动失败时先看这里。</li> -->
        </ol>

        <h3>注意事项</h3>
        <ul>
          <li>临时隧道域名随机分配，隧道重启后地址会变化，不适合需要固定域名的场景。</li>
          <li>公网地址本身没有鉴权，任何人都能访问该服务，请勿暴露无鉴权的敏感服务。</li>
          <li>勾选了自动启动会在服务重启后自动建立隧道。（但重启后会重新分配域名）</li>
          <li>「暂停」不会终止 cloudflared 进程，「恢复」后公网地址保持不变。</li>
        </ul>

        <dl>
          <template v-for="row in info" :key="row.k">
            <dt>{{ row.k }}</dt>
            <dd>
              {{ row.v }}
              <a v-if="row.k === '开发者：'" class="pay-link" href="javascript:void(0)" @click="showPay = true">【投喂入口】</a>
            </dd>
          </template>
        </dl>
      </div>
    </div>
    <div v-if="showPay" class="pay-mask" @click.self="showPay = false">
      <div class="pay-dialog" role="dialog" aria-modal="true" aria-label="投喂入口">
        <div class="head">
          <strong>投喂入口</strong>
          <button class="mini" @click="showPay = false">关闭</button>
        </div>
        <div class="pay-body">
          <p class="pay-tip">
            如果您觉得这个工具对您有帮助，欢迎通过以下方式赞赏支持开发者。<br />
            您的支持是我持续开发和维护的动力！感谢每一位用户的认可与鼓励。
          </p>
          <img src="https://fndesk.imcq.top/?url=pay" alt="投喂二维码" />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.mask {
  position: fixed;
  inset: 0;
  z-index: 30;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: rgba(6, 9, 16, 0.72);
}

.dialog {
  width: 100%;
  max-width: 560px;
  max-height: calc(100vh - 40px);
  display: flex;
  flex-direction: column;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
}

.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
}

.body {
  padding: 4px 16px 18px;
  overflow: auto;
  font-size: 13px;
  line-height: 1.7;
}

h3 {
  margin: 16px 0 6px;
  font-size: 13px;
  color: var(--accent);
}

p {
  margin: 12px 0 0;
  color: var(--muted);
}

ol,
ul {
  margin: 0;
  padding-left: 20px;
  color: var(--muted);
}

li {
  margin: 3px 0;
}

code {
  padding: 1px 5px;
  background: var(--panel-2);
  border-radius: 4px;
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
  color: var(--text);
}

dl {
  display: grid;
  grid-template-columns: 76px 1fr;
  gap: 6px 12px;
  margin: 18px 0 0;
  padding-top: 14px;
  border-top: 1px solid var(--border);
}

dt {
  color: var(--muted);
  font-size: 12px;
}

dd {
  margin: 0;
  word-break: break-all;
}

.mini {
  padding: 2px 10px;
  font-size: 12px;
  flex-shrink: 0;
}

.pay-link {
  margin-left: 6px;
  color: var(--muted);
  cursor: pointer;
  text-decoration: none;
}

.pay-link:hover {
  color: var(--accent);
}

.pay-mask {
  position: absolute;
  inset: 0;
  z-index: 40;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: rgba(6, 9, 16, 0.72);
}

.pay-dialog {
  display: flex;
  flex-direction: column;
  max-width: 90%;
  max-height: 90%;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
}

.pay-body {
  padding: 12px 16px;
  overflow: auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.pay-tip {
  margin: 0;
  text-align: center;
  font-size: 12px;
  line-height: 1.7;
  color: var(--muted);
}

.pay-body img {
  max-width: 320px;
  max-height: 60vh;
  display: block;
  border-radius: 8px;
}

@media (max-width: 720px) {
  .mask {
    padding: 0;
    align-items: flex-end;
  }

  .dialog {
    max-width: none;
    max-height: 88vh;
    border-radius: 12px 12px 0 0;
  }

  dl {
    grid-template-columns: 1fr;
    gap: 2px;
  }

  dd {
    margin-bottom: 8px;
  }
}
</style>
