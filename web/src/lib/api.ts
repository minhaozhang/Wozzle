import type {
  Container,
  Event,
  Image,
  InspectResponse,
  LogsResponse,
  SystemInfo,
} from '../types'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init)
  if (!res.ok) {
    let detail = ''
    try {
      const body = await res.text()
      detail = body ? `: ${body.slice(0, 200)}` : ''
    } catch {
      /* ignore body read error */
    }
    throw new Error(`${res.status} ${res.statusText}${detail}`)
  }
  return (await res.json()) as T
}

export const api = {
  systemInfo: () => request<SystemInfo>('/api/system/info'),

  containers: () => request<Container[]>('/api/containers'),

  container: (id: string) => request<InspectResponse>(`/api/containers/${id}`),

  containerLogs: (id: string, tail = 300, since?: string, until?: string) => {
    const q = new URLSearchParams()
    q.set('tail', String(tail))
    if (since) q.set('since', since)
    if (until) q.set('until', until)
    return request<LogsResponse>(`/api/containers/${id}/logs?${q.toString()}`)
  },

  images: () => request<Image[]>('/api/images'),

  // 危险操作需 ?confirm=1，否则后端返回 400
  containerAction: (id: string, verb: 'start' | 'stop' | 'kill' | 'restart') =>
    request<{ ok: true }>(`/api/containers/${id}/${verb}?confirm=1`, { method: 'POST' }),

  removeContainer: (id: string, force = false) =>
    request<{ ok: true }>(`/api/containers/${id}?confirm=1&force=${force ? 1 : 0}`, {
      method: 'DELETE',
    }),
}
