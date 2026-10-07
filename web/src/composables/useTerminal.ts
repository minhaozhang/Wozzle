import { onBeforeUnmount, ref, type Ref } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'
import { ReconnectingSocket, wsUrl, type SocketStatus } from './ReconnectingSocket'
import type { ExecClientMessage, ExecServerMessage } from '../types'

export type TermPhase = 'idle' | 'connecting' | 'ready' | 'exited' | 'disconnected' | 'error'

export interface TerminalState {
  phase: TermPhase
  exitCode: number | null
  errorMessage: string
  status: SocketStatus
}

/**
 * /api/ws/exec/{id}?cmd=sh&rows=N&cols=M 网页终端：
 * output→term.write；input/resize→WS；exit→显示退出码；ready→pty 建立。
 * 断线/退出不自动重连（新会话由用户手动发起）。
 */
export function useTerminal(containerId: Ref<string>, hostEl: Ref<HTMLElement | null>, cmd = 'sh') {
  const state = ref<TerminalState>({
    phase: 'idle',
    exitCode: null,
    errorMessage: '',
    status: 'closed',
  })

  let term: Terminal | null = null
  let fit: FitAddon | null = null
  let socket: ReconnectingSocket | null = null
  let observer: ResizeObserver | null = null
  let pingTimer: ReturnType<typeof setInterval> | null = null
  let session = 0
  let lastCols = 80
  let lastRows = 24

  function setPhase(patch: Partial<TerminalState>): void {
    state.value = { ...state.value, ...patch }
  }

  function ensureTerm(): Terminal {
    if (term) return term
    term = new Terminal({
      fontFamily: "ui-monospace, 'Cascadia Mono', 'JetBrains Mono', Consolas, monospace",
      fontSize: 13,
      cursorBlink: true,
      scrollback: 5000,
      theme: {
        background: '#0b0f14',
        foreground: '#e6edf3',
        cursor: '#4f8cff',
        cursorAccent: '#0b0f14',
        selectionBackground: 'rgba(79,140,255,0.28)',
        black: '#3b4252',
        red: '#f87171',
        green: '#7ee787',
        yellow: '#e3b341',
        blue: '#6ca0f6',
        magenta: '#d2a8ff',
        cyan: '#76c7d9',
        white: '#c9d1d9',
        brightBlack: '#6e7681',
        brightRed: '#ffa198',
        brightGreen: '#9ae6a0',
        brightYellow: '#f2cc60',
        brightBlue: '#91cbff',
        brightMagenta: '#e2c5ff',
        brightCyan: '#a5e5f5',
        brightWhite: '#ffffff',
      },
    })
    fit = new FitAddon()
    term.loadAddon(fit)
    term.loadAddon(new WebLinksAddon())
    if (hostEl.value) term.open(hostEl.value)
    try {
      fit.fit()
    } catch {
      /* 容器尚未布局好时忽略 */
    }
    lastCols = term.cols
    lastRows = term.rows

    term.onData((data) => {
      socket?.send({ t: 'input', data } satisfies ExecClientMessage)
    })
    term.onResize(({ rows, cols }) => {
      lastRows = rows
      lastCols = cols
      socket?.send({ t: 'resize', rows, cols } satisfies ExecClientMessage)
    })
    return term
  }

  function handleMessage(msg: unknown): void {
    const m = msg as ExecServerMessage
    switch (m.t) {
      case 'ready':
        setPhase({ phase: 'ready', errorMessage: '' })
        break
      case 'output':
        term?.write(m.data ?? '')
        break
      case 'exit':
        setPhase({ phase: 'exited', exitCode: m.code ?? 0 })
        socket?.suspend()
        break
      case 'error':
        setPhase({ phase: 'error', errorMessage: m.message ?? 'unknown error' })
        socket?.suspend()
        break
      default:
        break
    }
  }

  function connect(): void {
    session += 1
    const mySession = session
    ensureTerm()
    setPhase({ phase: 'connecting', exitCode: null, errorMessage: '' })

    const url = wsUrl(
      `/api/ws/exec/${encodeURIComponent(containerId.value)}?cmd=${encodeURIComponent(cmd)}&rows=${lastRows}&cols=${lastCols}`,
    )
    socket = new ReconnectingSocket(url, {
      onMessage: (msg) => {
        if (mySession === session) handleMessage(msg)
      },
      onStatus: (s) => {
        if (mySession !== session) return
        state.value.status = s
      },
      onClose: () => {
        if (mySession !== session) return
        // pty 会话随连接消失，不自动重连；由用户点击「重新连接」
        if (state.value.phase !== 'exited' && state.value.phase !== 'error') {
          setPhase({ phase: 'disconnected' })
        }
        socket?.suspend()
      },
    })
    socket.connect()

    if (pingTimer === null) {
      pingTimer = setInterval(() => {
        if (socket !== null && socket.isOpen) socket.send({ t: 'ping' } satisfies ExecClientMessage)
      }, 30_000)
    }
  }

  function reconnect(): void {
    term?.reset()
    socket?.dispose()
    socket = null
    connect()
  }

  function start(): void {
    ensureTerm()
    observer = new ResizeObserver(() => {
      try {
        fit?.fit()
      } catch {
        /* ignore */
      }
    })
    if (hostEl.value) observer.observe(hostEl.value)
    connect()
  }

  function dispose(): void {
    if (pingTimer !== null) {
      clearInterval(pingTimer)
      pingTimer = null
    }
    observer?.disconnect()
    observer = null
    socket?.dispose()
    socket = null
    term?.dispose()
    term = null
    fit = null
  }

  onBeforeUnmount(dispose)

  return {
    state,
    start,
    reconnect,
    dispose,
  }
}
