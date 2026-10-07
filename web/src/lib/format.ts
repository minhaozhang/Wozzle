// 展示格式化辅助

export function formatBytes(n: number | undefined | null): string {
  if (n === undefined || n === null || !Number.isFinite(n)) return '—'
  if (n < 1024) return `${n} B`
  const units = ['KiB', 'MiB', 'GiB', 'TiB', 'PiB']
  let v = n
  let i = -1
  do {
    v /= 1024
    i += 1
  } while (v >= 1024 && i < units.length - 1)
  return `${v >= 100 ? v.toFixed(0) : v.toFixed(1)} ${units[i]}`
}

function parseTs(ts: string | null | undefined): Date | null {
  if (!ts) return null
  const d = new Date(ts)
  return Number.isNaN(d.getTime()) ? null : d
}

const pad = (n: number) => String(n).padStart(2, '0')

/** 日志行时间戳：HH:MM:SS */
export function fmtLogTime(ts: string | null | undefined): string {
  const d = parseTs(ts)
  if (!d) return ''
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

/** 完整时间：YYYY-MM-DD HH:MM:SS（本地时区） */
export function fmtDateTime(ts: string | null | undefined): string {
  const d = parseTs(ts)
  if (!d) return '—'
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

/** 相对时间：just now / 5m ago / 2h ago / 3d ago */
export function timeAgo(ts: string | null | undefined): string {
  const d = parseTs(ts)
  if (!d) return '—'
  const s = Math.max(0, (Date.now() - d.getTime()) / 1000)
  if (s < 10) return 'just now'
  if (s < 60) return `${Math.floor(s)}s ago`
  if (s < 3600) return `${Math.floor(s / 60)}m ago`
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`
  return `${Math.floor(s / 86400)}d ago`
}

/** 事件时间轴用：今天显示 HH:MM:SS，否则 MM-DD HH:MM:SS */
export function fmtEventTime(ts: string | null | undefined): string {
  const d = parseTs(ts)
  if (!d) return '—'
  const now = new Date()
  const sameDay =
    d.getFullYear() === now.getFullYear() &&
    d.getMonth() === now.getMonth() &&
    d.getDate() === now.getDate()
  if (sameDay) return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

export function formatPercent(n: number | undefined | null): string {
  if (n === undefined || n === null || !Number.isFinite(n)) return '—'
  return `${n >= 100 ? n.toFixed(0) : n.toFixed(1)}%`
}
