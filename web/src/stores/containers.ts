import { computed, reactive } from 'vue'
import { api } from '../lib/api'
import type { Container } from '../types'
import { onContainerEvent, startEventsStream } from './events'
import { startStatsStream } from './stats'

// 排序：运行中优先，其次 paused/restarting，再按名称（PLAN.md 默认偏好）
const STATE_RANK: Record<string, number> = {
  running: 0,
  restarting: 1,
  paused: 2,
  created: 3,
  exited: 4,
  dead: 5,
  unknown: 6,
}

interface ContainersState {
  list: Container[]
  loading: boolean
  error: string
  loadedAt: number
  filter: string
}

export const containersStore = reactive<ContainersState>({
  list: [],
  loading: false,
  error: '',
  loadedAt: 0,
  filter: '',
})

let refreshTimer: ReturnType<typeof setTimeout> | null = null
let started = false

export async function refreshContainers(): Promise<void> {
  if (containersStore.loading) return
  containersStore.loading = true
  try {
    const list = await api.containers()
    containersStore.list = Array.isArray(list) ? list : []
    containersStore.error = ''
    containersStore.loadedAt = Date.now()
  } catch (e) {
    containersStore.error = e instanceof Error ? e.message : String(e)
  } finally {
    containersStore.loading = false
  }
}

/** 事件驱动的防抖刷新（事件可能成串到达） */
function scheduleRefresh(delay = 400): void {
  if (refreshTimer !== null) clearTimeout(refreshTimer)
  refreshTimer = setTimeout(() => {
    refreshTimer = null
    void refreshContainers()
  }, delay)
}

export function containerById(id: string | string[]): Container | undefined {
  const key = Array.isArray(id) ? id[0] : id
  if (!key) return undefined
  return containersStore.list.find((c) => c.id === key || c.shortId === key || c.name === key)
}

export async function containerAction(id: string, verb: 'start' | 'stop' | 'kill' | 'restart'): Promise<void> {
  await api.containerAction(id, verb)
  // 状态变更可能有延迟：立即刷一次，稍后再补一次
  scheduleRefresh(150)
  scheduleRefresh(2500)
}

export async function removeContainer(id: string, force = false): Promise<void> {
  await api.removeContainer(id, force)
  scheduleRefresh(150)
  scheduleRefresh(2500)
}

/** 全局数据源启动（App 挂载时调用一次） */
export function startGlobalStreams(): void {
  if (started) return
  started = true
  startEventsStream()
  startStatsStream()
  onContainerEvent(() => scheduleRefresh())
  void refreshContainers()
}

export const sortedContainers = computed<Container[]>(() => {
  const rank = (s: string) => STATE_RANK[s] ?? 3
  return [...containersStore.list].sort(
    (a, b) => rank(a.state) - rank(b.state) || (a.name || '').localeCompare(b.name || ''),
  )
})

export const filteredContainers = computed<Container[]>(() => {
  const q = containersStore.filter.trim().toLowerCase()
  if (!q) return sortedContainers.value
  return sortedContainers.value.filter((c) => {
    const hay = `${c.name ?? ''}\n${c.image ?? ''}\n${c.shortId ?? ''}\n${c.id ?? ''}`.toLowerCase()
    return hay.includes(q)
  })
})
