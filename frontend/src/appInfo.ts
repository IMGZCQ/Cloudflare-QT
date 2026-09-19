// 应用元信息：版本号唯一来源是 fnOS 打包描述文件 cloudflare_qt/manifest 的 version 字段
import manifest from '../../cloudflare_qt/manifest?raw'

export const appVersion = manifest.match(/^version=(.*)$/m)?.[1].trim() ?? ''
