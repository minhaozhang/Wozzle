import { reactive } from 'vue'
import { ReconnectingSocket, wsUrl, type SocketStatus } from '../composables/ReconnectingSocket'
import type { HostStat, Stat, StatsServerMessage } from '../types'

/** 图表滚动窗口：约 5 分钟 */
export const STATS_WINDOW_MS = 5 * 60 * 1000
const RETAIN_MS = STATS_WINDOW_MS + 30_000
const STALE_MS = 10 * 60 * 1000

export interface StatSample {
  t: number // ms epoch
  cpu: number // %
  mem: number // %
  memBytes: number
  memLimitBytes: number
  netRx: number
  netTx: number
}

interface StatsState {
  status: SocketStatus
  error: string
  latest: Record<string, Stat>
  host: HostStat | null
}

export const statsStore = reactive<StatsState>({
  status: 'connecting',
  error: '',
  latest: {},
  host: null,
})

/** 非响应式历史序列（chart 组件按 frame 计数器拉取） */
const history = new Map<string, StatSample[]>()
/** 每次 stats 帧自增，驱动图表刷新 */
const state = reactive({ frame: 0 })
export const statsFrame = () => state.frame

let socket: ReconnectingSocket | null = null

function handleFrame(msg: Extract<StatsServerMessage, { t: 'stats' }>): void {
  const now = Date.now()
  for (const s of msg.stats) {
    if (!s || typeof s.id !== 'string') continue
    statsStore.latest[s.id] = s
    let arr = history.get(s.id)
    if (!arr) {
      arr = []
      history.set(s.id, arr)
    }
    const t = s.ts ? Date.parse(s.ts) : NaN
    arr.push({
      t: Number.isNaN(t) ? now : t,
      cpu: Number.isFinite(s.cpuPercent) ? s.cpuPercent : 0,
      mem: Number.isFinite(s.memPercent) ? s.memPercent : 0,
      memBytes: s.memBytes ?? 0,
      memLimitBytes: s.memLimitBytes ?? 0,
      netRx: s.netRx ?? 0,
      netTx: s.netTx ?? 0,
    })
    const cutoff = now - RETAIN_MS
    while (arr.length > 0 && arr[0].t < cutoff) arr.shift()
  }
  statsStore.host = msg.host ?? null
  // 清理长时间没有数据的容器序列
  for (const [id, arr] of history) {
    if (arr.length === 0 || now - arr[arr.length - 1].t > STALE_MS) history.delete(id)
  }
  state.frame += 1
}

export function startStatsStream(): void {
  if (socket) return
  socket = new ReconnectingSocket(wsUrl('/api/ws/stats'), {
    onMessage: (msg) => {
      const m = msg as StatsServerMessage
      if (m.t === 'stats') {
        statsStore.error = ''
        handleFrame(m)
      } else if (m.t === 'statsError') {
        statsStore.error = m.error ?? 'stats error'
      }
    },
    onStatus: (s) => {
      statsStore.status = s
    },
  })
  socket.connect()
}

/** 取某容器的滚动序列（供 sparkline / 网络速率计算） */
export function getStatSeries(id: string): StatSample[] {
  return history.get(id) ?? []
}

export function statOf(id: string): Stat | undefined {
  return statsStore.latest[id]
}
