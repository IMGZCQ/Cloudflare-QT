export type TunnelState = 'stopped' | 'starting' | 'running' | 'paused' | 'error'

export interface TunnelItem {
  id: string
  name: string
  scheme: string
  host: string
  port: number
  path: string
  edgeIpVersion: string
  autoStart: boolean
  paused: boolean
  createdAt: number
  state: TunnelState
  url: string
  pid: number
  startedAt: number
  lastError: string
}

export interface BinaryStatus {
  path: string
  ready: boolean
  version: string
  downloading: boolean
  progress: number
  message: string
}

export interface TunnelPayload {
  name: string
  scheme: string
  host: string
  port: number
  path: string
  edgeIpVersion: string
  autoStart: boolean
  target?: string
}

// 统一网关会把应用挂在 /app/xxx/ 子路径下，用页面路径推导 API 前缀
const base = new URL('.', window.location.href).pathname.replace(/\/$/, '')

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(base + path, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  const text = await res.text()
  const data = text ? JSON.parse(text) : {}
  if (!res.ok) {
    throw new Error(data.error || `请求失败 (${res.status})`)
  }
  return data as T
}

export interface ListResult {
  items: TunnelItem[]
  binary: BinaryStatus
  etag: string
  notModified: boolean
}

export const api = {
  // 带 ETag 的列表拉取：数据无变化时后端返回 304，前端跳过重渲染
  list: async (etag?: string): Promise<ListResult> => {
    const res = await fetch(base + '/api/tunnels', {
      headers: etag ? { 'If-None-Match': etag } : {},
    })
    if (res.status === 304) {
      return { items: [], binary: null as unknown as BinaryStatus, etag: etag!, notModified: true }
    }
    const text = await res.text()
    const data = text ? JSON.parse(text) : {}
    if (!res.ok) {
      throw new Error(data.error || `请求失败 (${res.status})`)
    }
    return {
      items: data.items ?? [],
      binary: data.binary,
      etag: res.headers.get('ETag') ?? '',
      notModified: false,
    }
  },

  create: (payload: TunnelPayload) =>
    request<{ item: TunnelItem; error?: string }>('/api/tunnels', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),

  update: (id: string, payload: TunnelPayload) =>
    request<{ item: TunnelItem; error?: string }>(`/api/tunnels/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    }),

  remove: (id: string) => request<{ ok: boolean }>(`/api/tunnels/${id}`, { method: 'DELETE' }),

  start: (id: string) =>
    request<{ item: TunnelItem; error?: string }>(`/api/tunnels/${id}/start`, { method: 'POST' }),

  stop: (id: string) =>
    request<{ item: TunnelItem; error?: string }>(`/api/tunnels/${id}/stop`, { method: 'POST' }),

  pause: (id: string) =>
    request<{ item: TunnelItem; error?: string }>(`/api/tunnels/${id}/pause`, { method: 'POST' }),

  resume: (id: string) =>
    request<{ item: TunnelItem; error?: string }>(`/api/tunnels/${id}/resume`, { method: 'POST' }),

  logs: (id: string, from?: number) =>
    request<{ logs: string[]; total: number }>(
      `/api/tunnels/${id}/logs${from ? `?from=${from}` : ''}`,
    ),

  downloadBinary: () => request<BinaryStatus>('/api/binary/download', { method: 'POST' }),
}
