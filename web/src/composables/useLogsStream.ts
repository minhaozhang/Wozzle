import { onBeforeUnmount, ref, type Ref } from 'vue'
import { ReconnectingSocket, wsUrl, type SocketStatus } from './ReconnectingSocket'
import type { LogLine, LogsClientMessage, LogsServerMessage } from '../types'

export interface LogRow {
  seq: number
  ts: string | null
  text: string
  /** 存在时渲染为分隔线（notice 消息） */
  notice?: string
}

export interface LogsStreamOptions {
  tail?: number
  maxRows?: number
}

let seqCounter = 0

function toRow(line: LogLine): LogRow {
  seqCounter += 1
  return { seq: seqCounter, ts: line?.ts ?? null, text: line?.text ?? '' }
}

function toNoticeRow(message: string): LogRow {
  seqCounter += 1
  return { seq: seqCounter, ts: null, text: '', notice: message }
}

/**
 * /api/ws/logs/{id}?tail=N 流封装：
 * backfill 先渲染（与已有缓冲做后缀去重合并），实时行追加；
 * notice → 分隔行；end → 终态（停止自动重连，可手动重连）。
 */
export function useLogsStream(containerId: Ref<string>, opts: LogsStreamOptions = {}) {
  const tail = opts.tail ?? 300
  const maxRows = opts.maxRows ?? 5000

  const rows = ref<LogRow[]>([])
  const status = ref<SocketStatus>('connecting')
  const paused = ref(false)
  const ended = ref<string | null>(null)

  let socket: ReconnectingSocket | null = null
  let pending: LogRow[] = []
  let flushTimer: ReturnType<typeof setTimeout> | null = null

  function scheduleFlush(): void {
    if (flushTimer !== null) return
    flushTimer = setTimeout(() => {
      flushTimer = null
      flush()
    }, 120)
  }

  function flush(): void {
    if (pending.length === 0) return
    const batch = pending
    pending = []
    rows.value.push(...batch)
    if (rows.value.length > maxRows) rows.value.splice(0, rows.value.length - maxRows)
  }

  function appendLines(lines: LogRow[]): void {
    pending.push(...lines)
    if (pending.length >= 400) {
      if (flushTimer !== null) {
        clearTimeout(flushTimer)
        flushTimer = null
      }
      flush()
      return
    }
    scheduleFlush()
  }

  function replaceRows(lines: LogRow[]): void {
    if (flushTimer !== null) {
      clearTimeout(flushTimer)
      flushTimer = null
    }
    pending = []
    rows.value = lines.slice(-maxRows)
  }

  /** 重连后的 backfill 与现有缓冲按共同后缀去重合并 */
  function mergeBackfill(lines: LogLine[]): void {
    const incoming = lines.map(toRow)
    const cur = rows.value
    if (cur.length === 0) {
      replaceRows(incoming)
      return
    }
    let k = 0
    while (k < incoming.length && k < cur.length) {
      const a = incoming[incoming.length - 1 - k]
      const b = cur[cur.length - 1 - k]
      if (a.ts === b.ts && a.text === b.text) k += 1
      else break
    }
    if (k === 0) {
      if (incoming.length >= cur.length) replaceRows(incoming)
      else appendRows(incoming)
      return
    }
    const fresh = incoming.slice(0, incoming.length - k)
    if (fresh.length > 0) appendRows(fresh)
  }

  function appendRows(list: LogRow[]): void {
    if (list.length === 0) return
    appendLines(list)
  }

  function handleMessage(msg: unknown): void {
    const m = msg as LogsServerMessage
    switch (m.t) {
      case 'backfill':
        ended.value = null
        mergeBackfill(Array.isArray(m.lines) ? m.lines : [])
        break
      case 'line':
        appendRows([toRow(m.line)])
        break
      case 'notice':
        appendRows([toNoticeRow(m.message ?? '')])
        break
      case 'end':
        ended.value = m.reason ?? 'stream ended'
        socket?.suspend()
        break
      case 'pong':
        break
      default:
        break
    }
  }

  function connect(): void {
    socket = new ReconnectingSocket(wsUrl(`/api/ws/logs/${encodeURIComponent(containerId.value)}?tail=${tail}`), {
      onMessage: handleMessage,
      onStatus: (s) => {
        status.value = s
      },
      onOpen: () => {
        // 重连成功后恢复之前的暂停状态
        if (paused.value) socket?.send({ t: 'pause' } satisfies LogsClientMessage)
      },
    })
    socket.connect()
  }

  function pause(): void {
    paused.value = true
    socket?.send({ t: 'pause' } satisfies LogsClientMessage)
  }

  function resume(): void {
    paused.value = false
    socket?.send({ t: 'resume' } satisfies LogsClientMessage)
  }

  function togglePause(): void {
    if (paused.value) resume()
    else pause()
  }

  /** end/断开后手动重连 */
  function reconnect(): void {
    ended.value = null
    socket?.restart()
  }

  function dispose(): void {
    socket?.dispose()
    socket = null
    if (flushTimer !== null) {
      clearTimeout(flushTimer)
      flushTimer = null
    }
    pending = []
  }

  connect()

  onBeforeUnmount(dispose)

  return {
    rows,
    status,
    paused,
    ended,
    togglePause,
    reconnect,
    dispose,
  }
}
