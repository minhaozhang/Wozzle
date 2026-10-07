// 轻量 ANSI(SGR) → HTML 转换：保留日志颜色，转义 HTML，剔除光标控制序列。
// 手写实现（未引入 ansi_up 依赖）。

const BASIC_16 = [
  '#4f5666', '#f87171', '#7ee787', '#e3b341', '#6ca0f6', '#d2a8ff', '#76c7d9', '#c9d1d9',
  '#6e7681', '#ffa198', '#9ae6a0', '#f2cc60', '#91cbff', '#e2c5ff', '#a5e5f5', '#ffffff',
]

function color256(n: number): string {
  if (n < 16) return BASIC_16[n]
  if (n < 232) {
    const v = n - 16
    const step = [0, 95, 135, 175, 215, 255]
    const r = step[Math.floor(v / 36) % 6]
    const g = step[Math.floor(v / 6) % 6]
    const b = step[v % 6]
    return `rgb(${r},${g},${b})`
  }
  const gray = 8 + (n - 232) * 10
  return `rgb(${gray},${gray},${gray})`
}

interface SgrState {
  fg: string | null
  bg: string | null
  bold: boolean
  dim: boolean
  italic: boolean
  underline: boolean
  inverse: boolean
}

function styleOf(s: SgrState): string {
  const parts: string[] = []
  let fg = s.fg
  let bg = s.bg
  if (s.inverse) {
    const t = fg
    fg = bg ?? '#161b22'
    bg = t ?? '#e6edf3'
  }
  if (fg) parts.push(`color:${fg}`)
  if (bg) parts.push(`background-color:${bg}`)
  if (s.bold) parts.push('font-weight:600')
  if (s.dim) parts.push('opacity:0.6')
  if (s.italic) parts.push('font-style:italic')
  if (s.underline) parts.push('text-decoration:underline')
  return parts.join(';')
}

function escapeHtml(s: string): string {
  return s
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
}

/** 解析一段 SGR 参数（不含 "m"），更新状态 */
function applySgr(paramsStr: string, st: SgrState): void {
  const params = paramsStr === '' ? [0] : paramsStr.split(';').map((p) => parseInt(p, 10))
  let i = 0
  while (i < params.length) {
    const p = Number.isNaN(params[i]) ? 0 : params[i]
    if (p === 0 || p === 39 || p === 49 || p === 22 || p === 23 || p === 24 || p === 27) {
      if (p === 0) {
        st.fg = null
        st.bg = null
        st.bold = false
        st.dim = false
        st.italic = false
        st.underline = false
        st.inverse = false
      } else if (p === 39) st.fg = null
      else if (p === 49) st.bg = null
      else if (p === 22) {
        st.bold = false
        st.dim = false
      } else if (p === 23) st.italic = false
      else if (p === 24) st.underline = false
      else if (p === 27) st.inverse = false
      i += 1
      continue
    }
    if (p >= 30 && p <= 37) {
      st.fg = BASIC_16[p - 30]
      i += 1
      continue
    }
    if (p >= 90 && p <= 97) {
      st.fg = BASIC_16[8 + (p - 90)]
      i += 1
      continue
    }
    if (p >= 40 && p <= 47) {
      st.bg = BASIC_16[p - 40]
      i += 1
      continue
    }
    if (p >= 100 && p <= 107) {
      st.bg = BASIC_16[8 + (p - 100)]
      i += 1
      continue
    }
    if (p === 1) {
      st.bold = true
      i += 1
      continue
    }
    if (p === 2) {
      st.dim = true
      i += 1
      continue
    }
    if (p === 3) {
      st.italic = true
      i += 1
      continue
    }
    if (p === 4) {
      st.underline = true
      i += 1
      continue
    }
    if (p === 7) {
      st.inverse = true
      i += 1
      continue
    }
    if ((p === 38 || p === 48) && i + 1 < params.length) {
      const mode = params[i + 1]
      let color: string | null = null
      let consumed = 2
      if (mode === 5 && i + 2 < params.length) {
        color = color256(Math.max(0, Math.min(255, params[i + 2] || 0)))
        consumed = 3
      } else if (mode === 2 && i + 4 < params.length) {
        const r = Math.max(0, Math.min(255, params[i + 2] || 0))
        const g = Math.max(0, Math.min(255, params[i + 3] || 0))
        const b = Math.max(0, Math.min(255, params[i + 4] || 0))
        color = `rgb(${r},${g},${b})`
        consumed = 5
      }
      if (color) {
        if (p === 38) st.fg = color
        else st.bg = color
        i += consumed
        continue
      }
      i += 2
      continue
    }
    i += 1
  }
}

/** 把含 ANSI 码的日志文本转成带 <span style> 的 HTML 字符串 */
export function ansiToHtml(text: string): string {
  if (!text.includes('\x1b')) return escapeHtml(text)

  const st: SgrState = {
    fg: null,
    bg: null,
    bold: false,
    dim: false,
    italic: false,
    underline: false,
    inverse: false,
  }
  let out = ''
  let run = ''
  const flush = () => {
    if (run === '') return
    const style = styleOf(st)
    out += style ? `<span style="${style}">${escapeHtml(run)}</span>` : escapeHtml(run)
    run = ''
  }

  let i = 0
  const n = text.length
  while (i < n) {
    const ch = text[i]
    if (ch !== '\x1b') {
      run += ch
      i += 1
      continue
    }
    // ESC 序列
    const next = i + 1 < n ? text[i + 1] : ''
    if (next === '[') {
      // CSI: 参数 + 最终字节(@~)
      let j = i + 2
      while (j < n && !/[ -~]/.test(text[j])) j += 1 // 跳过非常规字节
      if (j >= n) break // 未闭合，丢弃
      let params = ''
      while (j < n && /[0-9;:?<=> ]/.test(text[j])) {
        params += text[j]
        j += 1
      }
      if (j >= n) break
      const finalByte = text[j]
      if (finalByte === 'm') {
        flush()
        applySgr(params, st)
      }
      // 其它 CSI（光标移动等）直接剔除
      i = j + 1
      continue
    }
    if (next === ']') {
      // OSC：直到 BEL 或 ST(ESC \)
      let j = i + 2
      while (j < n) {
        if (text[j] === '\x07') {
          j += 1
          break
        }
        if (text[j] === '\x1b' && j + 1 < n && text[j + 1] === '\\') {
          j += 2
          break
        }
        j += 1
      }
      i = j
      continue
    }
    // 其它单字符转义（ESC 后跟一个字符）
    i += 2
  }
  flush()
  return out
}

/** 去除所有 ANSI 转义序列（下载 .log 用） */
export function stripAnsi(text: string): string {
  return text.replace(/\x1b\[[0-9;:]*[ -/]*[@-~]/g, '').replace(/\x1b\][^\x07]*(?:\x07|\x1b\\)/g, '').replace(/\x1b./g, '')
}
