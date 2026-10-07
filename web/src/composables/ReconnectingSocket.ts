// 带自动重连（指数退避，上限 10s）的 WebSocket 封装。
// 所有 WS 端点共用；JSON 文本帧收发。

export type SocketStatus = 'connecting' | 'open' | 'closed' | 'reconnecting' | 'ended'

export interface SocketHandlers {
  onMessage?: (msg: unknown, raw: MessageEvent) => void
  onOpen?: () => void
  onClose?: (ev: CloseEvent) => void
  onStatus?: (status: SocketStatus) => void
}

const MIN_DELAY = 1000
const MAX_DELAY = 10000

export function wsUrl(path: string): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}${path}`
}

export class ReconnectingSocket {
  private ws: WebSocket | null = null
  private retry = 0
  private timer: ReturnType<typeof setTimeout> | null = null
  private disposed = false
  /** 收到业务层「终态」（如 logs 的 end）后置 true，不再自动重连 */
  private ended = false

  constructor(
    private readonly url: string,
    private readonly handlers: SocketHandlers = {},
  ) {}

  connect(): void {
    if (this.disposed || this.ws) return
    this.setStatus(this.retry === 0 && !this.ended ? 'connecting' : 'reconnecting')
    const ws = new WebSocket(this.url)
    this.ws = ws

    ws.onopen = () => {
      if (this.ws !== ws) return
      this.retry = 0
      this.setStatus('open')
      this.handlers.onOpen?.()
    }

    ws.onmessage = (raw: MessageEvent) => {
      if (this.ws !== ws) return
      let msg: unknown = null
      try {
        msg = JSON.parse(typeof raw.data === 'string' ? raw.data : '')
      } catch {
        return // 忽略非 JSON 帧
      }
      if (msg !== null && typeof msg === 'object') this.handlers.onMessage?.(msg, raw)
    }

    ws.onclose = (ev: CloseEvent) => {
      if (this.ws !== ws) return
      this.ws = null
      this.handlers.onClose?.(ev)
      if (this.disposed || this.ended) {
        this.setStatus('ended')
        return
      }
      this.scheduleReconnect()
    }

    ws.onerror = () => {
      /* onclose 随后触发，统一在 onclose 处理 */
    }
  }

  private scheduleReconnect(): void {
    const delay = Math.min(MIN_DELAY * Math.pow(2, this.retry), MAX_DELAY)
    this.retry += 1
    this.setStatus('reconnecting')
    this.timer = setTimeout(() => {
      this.timer = null
      this.connect()
    }, delay)
  }

  private setStatus(s: SocketStatus): void {
    this.handlers.onStatus?.(s)
  }

  /** 发送 JSON 文本帧；连接不可用时返回 false */
  send(msg: unknown): boolean {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg))
      return true
    }
    return false
  }

  /** 业务终态：服务器已声明流结束（end/exit），停止自动重连 */
  suspend(): void {
    this.ended = true
    try {
      this.ws?.close()
    } catch {
      /* ignore */
    }
  }

  /** 手动重连（清除终态标记并立即连接） */
  restart(): void {
    this.ended = false
    this.retry = 0
    if (this.timer !== null) {
      clearTimeout(this.timer)
      this.timer = null
    }
    try {
      this.ws?.close()
    } catch {
      /* ignore */
    }
    this.ws = null
    this.connect()
  }

  get isOpen(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN
  }

  dispose(): void {
    this.disposed = true
    this.ended = true
    if (this.timer !== null) {
      clearTimeout(this.timer)
      this.timer = null
    }
    try {
      this.ws?.close()
    } catch {
      /* ignore */
    }
    this.ws = null
  }
}
