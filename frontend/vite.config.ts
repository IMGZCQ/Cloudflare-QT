import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  // 相对路径，保证在 fnOS 统一网关子路径（如 /app/cfquicktunnel/）下也能加载资源
  base: './',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    port: 5180,
    // 关于弹窗要读取项目根目录的 cloudflare_qt/manifest，dev 下需放开上级目录
    fs: {
      allow: ['..'],
    },
    proxy: {
      '/api': 'http://127.0.0.1:9970',
    },
  },
})
