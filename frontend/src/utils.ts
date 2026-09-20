// 隧道地址脱敏：保留前 12 位和后 18 位，中间用 * 代替
// 示例：https://abc123***xyz789.trycloudflare.com
export function maskUrl(url: string): string {
  if (!url || url.length <= 30) return url
  return url.slice(0, 12) + '********' + url.slice(-14)
}
