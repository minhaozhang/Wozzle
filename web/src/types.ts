// 数据模型 —— 严格来自 docs/API.md（前后端唯一真相源），字段名不可自创。

export interface PortMapping {
  host: number
  container: number
  protocol: string
}

export interface Container {
  id: string // 必填，完整 ID
  shortId: string // 前 12 位
  name: string // 必填
  image: string
  state: string // created|running|exited|paused|restarting|dead|unknown（必填）
  health: string // healthy|unhealthy|starting|none
  statusText: string
  createdAt: string // RFC3339
  startedAt: string // RFC3339
  command: string
  ports: PortMapping[]
  labels: Record<string, string>
}

export interface LogLine {
  ts: string | null
  text: string // 可含 ANSI 颜色码
}

export interface Stat {
  id: string
  cpuPercent: number
  memBytes: number
  memLimitBytes: number
  memPercent: number
  netRx: number
  netTx: number
  blockRead: number
  blockWrite: number
  pids: number
  ts: string
}

/** WSL 宿主机（容器所在的 utility VM）实时指标，stats 帧可选字段 */
export interface HostStat {
  cpuPercent: number
  memBytes: number
  memTotalBytes: number
  memPercent: number
  uptimeSeconds: number
}

export interface Event {
  type: string
  id: string
  name: string
  image: string
  ts: string
  raw?: unknown
}

export interface Image {
  id: string
  repository: string
  tag: string
  sizeBytes: number
  createdAt: string
}

export interface SystemInfo {
  wslVersion: string
  wslcVersion: string
  kernel: string
  os: string
  wozzleVersion: string
  running: number
  total: number
}

export interface InspectResponse {
  container: Container
  inspect: unknown // wslc inspect 原始 JSON
}

export interface LogsResponse {
  lines: LogLine[]
}

// ---- WebSocket 消息（均为 JSON 文本帧，`t` 字段区分类型）----

// /api/ws/logs/{id}?tail=300
export type LogsServerMessage =
  | { t: 'backfill'; lines: LogLine[] }
  | { t: 'line'; line: LogLine }
  | { t: 'notice'; message: string }
  | { t: 'end'; reason: string }
  | { t: 'pong' }

export type LogsClientMessage =
  | { t: 'pause' }
  | { t: 'resume' }
  | { t: 'tail'; n: number }
  | { t: 'ping' }

// /api/ws/stats
export type StatsServerMessage =
  | { t: 'stats'; ts: string; stats: Stat[]; host?: HostStat }
  | { t: 'statsError'; error: string }

// /api/ws/events
export type EventsServerMessage = { t: 'event'; event: Event }

// /api/ws/exec/{id}
export type ExecServerMessage =
  | { t: 'output'; data: string }
  | { t: 'exit'; code: number }
  | { t: 'error'; message: string }
  | { t: 'ready' }

export type ExecClientMessage =
  | { t: 'input'; data: string }
  | { t: 'resize'; rows: number; cols: number }
  | { t: 'ping' }
