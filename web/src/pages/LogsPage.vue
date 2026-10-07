<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useLogsStream, type LogRow } from '../composables/useLogsStream'
import { containerById } from '../stores/containers'
import { ansiToHtml, stripAnsi } from '../lib/ansi'
import { fmtLogTime } from '../lib/format'

const route = useRoute()
const router = useRouter()
const id = computed(() => String(route.params.id ?? ''))

const { rows, status, paused, ended, togglePause, reconnect } = useLogsStream(id, { tail: 300, maxRows: 5000 })

const container = computed(() => containerById(id.value))

// ---- 过滤（前端：文本 / 正则） ----
const filterText = ref('')
const regexMode = ref(false)

const regexInfo = computed<{ re: RegExp | null; error: string }>(() => {
  if (!regexMode.value || filterText.value === '') return { re: null, error: '' }
  try {
    return { re: new RegExp(filterText.value, 'i'), error: '' }
  } catch (e) {
    return { re: null, error: e instanceof Error ? e.message : 'invalid regex' }
  }
})

const filteredRows = computed<LogRow[]>(() => {
  const q = filterText.value
  if (q === '') return rows.value
  const re = regexInfo.value.re
  if (re) return rows.value.filter((r) => re.test(r.notice ?? stripAnsi(r.text)))
  const needle = q.toLowerCase()
  return rows.value.filter((r) => (r.notice ?? stripAnsi(r.text)).toLowerCase().includes(needle))
})

// ---- 虚拟滚动 ----
const ROW_H = 20
const OVERSCAN = 12
const scrollEl = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const viewportH = ref(400)
const follow = ref(true)

const total = computed(() => filteredRows.value.length)
const startIdx = computed(() => Math.max(0, Math.floor(scrollTop.value / ROW_H) - OVERSCAN))
const endIdx = computed(() =>
  Math.min(total.value, startIdx.value + Math.ceil(viewportH.value / ROW_H) + OVERSCAN * 2),
)
const visibleRows = computed(() => filteredRows.value.slice(startIdx.value, endIdx.value))
const topPad = computed(() => startIdx.value * ROW_H)
const bottomPad = computed(() => Math.max(0, (total.value - endIdx.value) * ROW_H))

const showTs = ref(true)

function onScroll(): void {
  const el = scrollEl.value
  if (!el) return
  scrollTop.value = el.scrollTop
  viewportH.value = el.clientHeight
  follow.value = el.scrollTop + el.clientHeight >= el.scrollHeight - 28
}

function scrollToEnd(): void {
  const el = scrollEl.value
  if (!el) return
  el.scrollTop = el.scrollHeight
}

function jumpToBottom(): void {
  follow.value = true
  void nextTick(scrollToEnd)
}

watch(
  () => filteredRows.value.length,
  () => {
    if (follow.value) void nextTick(scrollToEnd)
  },
)

watch([filterText, showTs], () => {
  if (follow.value) void nextTick(scrollToEnd)
})

function rowHtml(r: LogRow): string {
  return ansiToHtml(r.text)
}

// ---- 下载当前缓冲 ----
function download(): void {
  const lines = filteredRows.value.map((r) => {
    if (r.notice) return `---- ${r.notice} ----`
    const ts = showTs.value && r.ts ? `${r.ts} ` : ''
    return ts + stripAnsi(r.text)
  })
  const blob = new Blob([lines.join('\n') + '\n'], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  const name = (container.value?.name || id.value).replace(/[^\w.-]+/g, '_')
  a.href = url
  a.download = `${name}.log`
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

const statusClass = computed(() => status.value)
const statusText = computed(() => {
  if (ended.value) return 'ended'
  switch (status.value) {
    case 'open':
      return 'live'
    case 'connecting':
      return 'connecting'
    case 'reconnecting':
      return 'reconnecting'
    default:
      return 'closed'
  }
})

onMounted(() => {
  const el = scrollEl.value
  if (el) {
    viewportH.value = el.clientHeight
    scrollToEnd()
  }
})

function goTab(tab: 'inspect' | 'terminal'): void {
  router.push(`/container/${encodeURIComponent(id.value)}/${tab}`)
}
</script>

<template>
  <div class="logspage">
    <header class="logs-toolbar">
      <div class="lt-left">
        <span class="dot" :class="container?.state ?? 'unknown'" />
        <span class="lt-name" :title="container?.id ?? id">{{ container?.name ?? id }}</span>
        <span v-if="container?.statusText" class="lt-status">{{ container.statusText }}</span>
        <nav class="lt-tabs">
          <button class="btn active">Logs</button>
          <button class="btn" @click="goTab('inspect')">Inspect</button>
          <button class="btn" @click="goTab('terminal')">Terminal</button>
        </nav>
      </div>
      <div class="lt-right">
        <div class="filter-box" :class="{ invalid: regexInfo.error !== '' }">
          <input
            v-model="filterText"
            class="input filter-input"
            type="text"
            placeholder="过滤…"
            spellcheck="false"
          />
          <button
            class="btn regex-btn"
            :class="{ active: regexMode }"
            title="正则模式（前端过滤，忽略大小写）"
            @click="regexMode = !regexMode"
          >
            .*
          </button>
        </div>
        <span v-if="regexInfo.error" class="regex-error" :title="regexInfo.error">regex!</span>
        <button class="btn" :class="{ active: showTs }" title="时间戳开关" @click="showTs = !showTs">TS</button>
        <button
          class="btn"
          :class="{ active: paused }"
          :title="paused ? '继续（resume）' : '暂停（pause）'"
          @click="togglePause()"
        >
          {{ paused ? '▶ 继续' : '⏸ 暂停' }}
        </button>
        <button class="btn" title="下载当前缓冲为 .log" @click="download">⬇ 下载</button>
        <span class="lt-count" :title="`过滤后 ${filteredRows.length} / 缓冲 ${rows.length} 行（上限 5000）`">
          {{ filteredRows.length }}/{{ rows.length }}
        </span>
        <span class="lt-live" :class="statusClass">
          <span class="dot" :class="ended ? 'offline' : status === 'open' ? 'running' : 'restarting'" />
          {{ statusText }}
        </span>
      </div>
    </header>

    <div v-if="ended" class="logs-banner ended">
      <span>日志流已结束：{{ ended }}</span>
      <button class="btn" @click="reconnect()">重新连接</button>
    </div>
    <div v-else-if="status === 'reconnecting' || status === 'connecting'" class="logs-banner info">
      {{ status === 'connecting' ? '正在连接日志流…' : '连接断开，正在重连…（指数退避）' }}
    </div>
    <div v-if="paused" class="logs-banner paused">已暂停（服务端停止推送，resume 恢复）</div>

    <div ref="scrollEl" class="log-scroll" @scroll="onScroll">
      <div :style="{ height: topPad + 'px' }"></div>
      <div
        v-for="r in visibleRows"
        :key="r.seq"
        class="log-row"
        :class="{ 'notice-row': r.notice }"
      >
        <template v-if="r.notice">
          <span class="notice-text">{{ r.notice }}</span>
        </template>
        <template v-else>
          <span v-if="showTs" class="log-ts">{{ fmtLogTime(r.ts) }}</span>
          <span class="log-text" v-html="rowHtml(r)"></span>
        </template>
      </div>
      <div :style="{ height: bottomPad + 'px' }"></div>
    </div>

    <div v-if="total === 0" class="log-empty">
      <template v-if="filterText">没有匹配的日志行</template>
      <template v-else-if="status === 'open'">暂无日志输出，等待容器写入…</template>
      <template v-else>正在连接 /api/ws/logs/{{ id.slice(0, 12) }}…</template>
    </div>

    <Transition name="pop">
      <button v-if="!follow" class="jump-btn" title="跳到底部" @click="jumpToBottom">⇓</button>
    </Transition>
  </div>
</template>

<style scoped>
.logspage {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  position: relative;
  background: #0b0f14;
}
.logs-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border-soft);
  background: var(--bg-panel);
  flex-wrap: wrap;
}
.lt-left {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
}
.lt-name {
  font-weight: 650;
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 260px;
}
.lt-status {
  color: var(--text-dim);
  font-size: 11.5px;
  white-space: nowrap;
}
.lt-tabs {
  display: flex;
  gap: 6px;
}
.lt-right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 8px;
}
.filter-box {
  display: flex;
  align-items: center;
  gap: 6px;
}
.filter-box.invalid .filter-input {
  border-color: var(--red);
}
.filter-input {
  width: 180px;
  font-size: 12px;
  font-family: var(--mono);
}
.regex-btn {
  font-family: var(--mono);
  padding: 5px 9px;
}
.regex-error {
  color: var(--red);
  font-size: 11px;
  cursor: help;
}
.lt-count {
  font-family: var(--mono);
  font-size: 11px;
  color: var(--text-faint);
  white-space: nowrap;
}
.lt-live {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  font-family: var(--mono);
  color: var(--text-dim);
  white-space: nowrap;
}
.logs-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 16px;
  font-size: 12.5px;
  border-bottom: 1px solid var(--border-soft);
}
.logs-banner.ended {
  background: rgba(248, 81, 73, 0.1);
  color: #ff9b94;
}
.logs-banner.info {
  background: rgba(79, 140, 255, 0.08);
  color: var(--text-dim);
}
.logs-banner.paused {
  background: rgba(210, 153, 34, 0.1);
  color: var(--amber);
}
.log-scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 4px 0 14px 12px;
  font-family: var(--mono);
  font-size: 12.5px;
}
.log-row {
  height: 20px;
  line-height: 20px;
  white-space: pre;
  display: flex;
  align-items: center;
}
.log-ts {
  flex: none;
  color: var(--text-faint);
  padding-right: 10px;
  user-select: none;
  font-size: 11.5px;
}
.log-text {
  min-width: 0;
}
.notice-row {
  justify-content: center;
}
.notice-text {
  color: var(--amber);
  font-size: 11.5px;
  user-select: none;
}
.notice-row::before,
.notice-row::after {
  content: '';
  flex: 1;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(210, 153, 34, 0.35));
  margin: 0 12px;
  min-width: 24px;
}
.notice-row::after {
  background: linear-gradient(90deg, rgba(210, 153, 34, 0.35), transparent);
}
.log-empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-faint);
  font-size: 13px;
  pointer-events: none;
  font-family: var(--sans);
}
.jump-btn {
  position: absolute;
  right: 22px;
  bottom: 22px;
  width: 38px;
  height: 38px;
  border-radius: 50%;
  border: 1px solid var(--accent);
  background: rgba(21, 27, 35, 0.92);
  color: var(--accent);
  font-size: 17px;
  cursor: pointer;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.45);
}
.jump-btn:hover {
  background: var(--accent-soft);
}
.pop-enter-active,
.pop-leave-active {
  transition: opacity 0.15s, transform 0.15s;
}
.pop-enter-from,
.pop-leave-to {
  opacity: 0;
  transform: translateY(6px);
}
</style>
