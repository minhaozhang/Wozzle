<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { containersStore, refreshContainers, sortedContainers } from '../stores/containers'
import { getStatSeries, statOf, statsFrame, statsStore } from '../stores/stats'
import { eventsStore } from '../stores/events'
import { loadSystemInfo, systemStore } from '../stores/system'
import StatBars from '../components/StatBars.vue'
import EventTimeline from '../components/EventTimeline.vue'
import { formatBytes, formatRate, fmtUptime, timeAgo } from '../lib/format'
import type { Container } from '../types'

const router = useRouter()

onMounted(() => {
  if (containersStore.loadedAt === 0) void refreshContainers()
  if (!systemStore.loaded) void loadSystemInfo()
})

const runningCount = computed(() => containersStore.list.filter((c) => c.state === 'running').length)

/** 主机（WSL utility VM）状态条数据 */
const host = computed(() => statsStore.host)
const barWidth = (pct: number | undefined | null): string =>
  pct === undefined || pct === null || !Number.isFinite(pct) ? '0%' : `${Math.min(100, Math.max(0, pct))}%`

/** 网络速率：与 ~10s 前的样本求差（历史序列非响应式，需依赖帧计数器） */
function netRateOf(id: string): { rx: string; tx: string } {
  void statsFrame()
  const arr = getStatSeries(id)
  if (arr.length < 2) return { rx: '—', tx: '—' }
  const last = arr[arr.length - 1]
  const cutoff = last.t - 10_000
  let ref = arr[0]
  for (let i = arr.length - 1; i >= 0; i--) {
    if (arr[i].t <= cutoff) {
      ref = arr[i]
      break
    }
  }
  const dt = (last.t - ref.t) / 1000
  if (dt <= 0) return { rx: '—', tx: '—' }
  return {
    rx: formatRate(Math.max(0, (last.netRx - ref.netRx) / dt)),
    tx: formatRate(Math.max(0, (last.netTx - ref.netTx) / dt)),
  }
}

/** 悬浮提示：累计流量（wslc stats 口径） */
function netTitleOf(id: string): string {
  const s = statOf(id)
  if (!s) return '暂无数据'
  return `累计 ↓ ${formatBytes(s.netRx)} · ↑ ${formatBytes(s.netTx)}`
}

/** Dozzle 式负载分档：≤50 绿 / ≤70 蓝 / ≤90 琥珀 / >90 红 */
function loadTone(pct: number | undefined | null): string {
  if (pct === undefined || pct === null || !Number.isFinite(pct)) return ''
  if (pct > 90) return 'hot'
  if (pct > 70) return 'warm'
  if (pct > 50) return 'mild'
  return 'ok'
}

/** CPU 整数百分比（Dozzle 同款 toFixed(0)%） */
const cpuText = (id: string): string => {
  const s = statOf(id)
  return s && Number.isFinite(s.cpuPercent) ? `${Math.round(s.cpuPercent)}%` : '—'
}

/** MEM 显示绝对用量（Dozzle 同款） */
const memText = (id: string): string => {
  const s = statOf(id)
  return s ? formatBytes(s.memBytes) : '—'
}

const pidsText = (id: string): string => {
  const s = statOf(id)
  return s ? String(s.pids) : '—'
}

const stateText = (c: Container): string => {
  switch (c.state) {
    case 'running':
      return '运行中'
    case 'exited':
      return '已退出'
    case 'paused':
      return '已暂停'
    case 'created':
      return '已创建'
    case 'restarting':
      return '重启中'
    case 'dead':
      return '已死亡'
    default:
      return c.state
  }
}

/** 启动/创建时间：运行中显示启动时长，其余显示创建时长 */
const whenText = (c: Container): string =>
  c.state === 'running' && c.startedAt ? `启动于 ${timeAgo(c.startedAt)}` : `创建于 ${timeAgo(c.createdAt)}`

const infoLine = computed(() => {
  const info = systemStore.info
  if (!info) return ''
  const segs = [
    info.os,
    info.kernel ? `kernel ${info.kernel}` : '',
    info.wslVersion ? `WSL ${info.wslVersion}` : '',
  ].filter(Boolean)
  return segs.join(' · ')
})

function openLogs(id: string): void {
  void router.push(`/container/${encodeURIComponent(id)}/logs`)
}
</script>

<template>
  <div class="page">
    <div class="page-header">
      <h1>Dashboard</h1>
      <span class="sub">
        {{ runningCount }} running / {{ containersStore.list.length }} total<template v-if="infoLine"> · {{ infoLine }}</template>
      </span>
      <span class="spacer" />
      <button class="btn" :disabled="containersStore.loading" @click="refreshContainers()">刷新</button>
    </div>

    <p v-if="containersStore.error" class="error-text">
      无法连接后端（GET /api/containers）：{{ containersStore.error }}
    </p>
    <p v-else-if="statsStore.error" class="stats-warn">stats 流错误：{{ statsStore.error }}</p>

    <!-- 主机（WSL utility VM）状态条：容器真正的宿主 -->
    <div v-if="host" class="host-strip">
      <span class="host-tag">HOST · WSL</span>
      <div class="host-metric" title="WSL utility VM 的整体 CPU 使用率">
        <span class="hk">CPU</span>
        <div class="host-bar">
          <div class="host-fill" :class="loadTone(host.cpuPercent)" :style="{ width: barWidth(host.cpuPercent) }" />
        </div>
        <span class="hv" :class="loadTone(host.cpuPercent)">{{ Math.round(host.cpuPercent) }}%</span>
      </div>
      <div
        class="host-metric"
        :title="`已用 ${formatBytes(host.memBytes)} / 总量 ${formatBytes(host.memTotalBytes)}（Windows 的一部分，受 WSL 配置上限约束）`"
      >
        <span class="hk">MEM</span>
        <div class="host-bar">
          <div class="host-fill" :class="loadTone(host.memPercent)" :style="{ width: barWidth(host.memPercent) }" />
        </div>
        <span class="hv" :class="loadTone(host.memPercent)">{{ formatBytes(host.memBytes) }} / {{ formatBytes(host.memTotalBytes) }}</span>
      </div>
      <span class="host-chip" title="WSL utility VM 开机时长">运行 {{ fmtUptime(host.uptimeSeconds) }}</span>
      <span class="host-chip">{{ runningCount }}/{{ containersStore.list.length }} 容器</span>
    </div>

    <div v-if="containersStore.list.length === 0" class="empty">
      <span class="icon">◔</span>
      <template v-if="containersStore.loading">正在加载容器列表…</template>
      <template v-else>当前没有容器</template>
    </div>

    <!-- Dozzle 式容器表：一行一个容器，斑马纹 + 行悬浮，含已退出容器 -->
    <div v-else class="container-table">
      <table>
        <thead>
          <tr>
            <th class="col-name">名称</th>
            <th class="col-state">状态</th>
            <th class="col-stat">CPU</th>
            <th class="col-stat">内存</th>
            <th class="col-net">网络</th>
            <th class="col-num">PIDs</th>
            <th class="col-when">时间</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="c in sortedContainers"
            :key="c.id"
            class="row"
            :class="{ stopped: c.state !== 'running' }"
            @click="openLogs(c.id)"
          >
            <td class="col-name">
              <div class="name-line">
                <span class="dot" :class="c.state" />
                <span class="name" :title="`${c.name} (${c.shortId})`">{{ c.name }}</span>
                <span v-if="c.health && c.health !== 'none'" class="badge" :class="c.health">{{ c.health }}</span>
              </div>
              <div class="image-line" :title="c.image">{{ c.image }}</div>
            </td>
            <td class="col-state" :title="c.statusText">
              <span :class="['state-text', c.state !== 'running' ? 'is-stopped' : '']">{{ stateText(c) }}</span>
            </td>
            <td class="col-stat">
              <div class="stat-cell">
                <StatBars :container-id="c.id" kind="cpu" />
                <span class="stat-val" :class="loadTone(statOf(c.id)?.cpuPercent)">{{ cpuText(c.id) }}</span>
              </div>
            </td>
            <td class="col-stat">
              <div class="stat-cell">
                <StatBars :container-id="c.id" kind="mem" />
                <span class="stat-val" :class="loadTone(statOf(c.id)?.memPercent)">{{ memText(c.id) }}</span>
              </div>
            </td>
            <td class="col-net mono" :title="netTitleOf(c.id)">
              <span class="net-rx">↓ {{ netRateOf(c.id).rx }}</span>
              <span class="net-tx">↑ {{ netRateOf(c.id).tx }}</span>
            </td>
            <td class="col-num mono">{{ pidsText(c.id) }}</td>
            <td class="col-when" :title="c.state === 'running' ? c.startedAt : c.createdAt">{{ whenText(c) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <section class="timeline-section">
      <div class="section-head">
        <h2>事件时间线</h2>
        <span class="sub">
          <span class="dot" :class="eventsStore.status === 'open' ? 'running' : eventsStore.status === 'ended' ? 'offline' : 'restarting'" />
          /api/ws/events
        </span>
      </div>
      <EventTimeline />
    </section>
  </div>
</template>

<style scoped>
.stats-warn {
  color: var(--amber);
  font-size: 12.5px;
  margin: 0 0 12px;
}
/* ---- 主机状态条 ---- */
.host-strip {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px 22px;
  background: var(--bg-panel);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius);
  padding: 9px 16px;
  margin-bottom: 14px;
  font-size: 12px;
}
.host-tag {
  font-family: var(--mono);
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.08em;
  color: var(--text-faint);
  border: 1px solid var(--border-soft);
  border-radius: 5px;
  padding: 2px 7px;
  flex: none;
}
.host-metric {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.hk {
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0.05em;
  color: var(--text-faint);
  flex: none;
}
.host-bar {
  width: 90px;
  height: 5px;
  border-radius: 3px;
  background: rgba(255, 255, 255, 0.08);
  overflow: hidden;
  flex: none;
}
.host-fill {
  height: 100%;
  border-radius: 3px;
  transition: width 0.3s;
}
.host-fill.ok {
  background: var(--green);
}
.host-fill.mild {
  background: #6ca0f6;
}
.host-fill.warm {
  background: var(--amber);
}
.host-fill.hot {
  background: var(--red);
}
.hv {
  font-family: var(--mono);
  font-size: 11.5px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  color: var(--text-dim);
}
.hv.ok {
  color: var(--green);
}
.hv.mild {
  color: #6ca0f6;
}
.hv.warm {
  color: var(--amber);
}
.hv.hot {
  color: var(--red);
}
.host-chip {
  font-family: var(--mono);
  font-size: 11px;
  color: var(--text-faint);
  white-space: nowrap;
}
/* ---- /主机状态条 ---- */
.container-table {
  background: var(--bg-panel);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius);
  overflow: hidden;
}
.container-table table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.container-table th {
  text-align: left;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.05em;
  color: var(--text-faint);
  padding: 9px 14px;
  border-bottom: 1px solid var(--border-soft);
  white-space: nowrap;
}
.container-table td {
  padding: 8px 14px;
  border-bottom: 1px solid var(--border-soft);
  vertical-align: middle;
}
.container-table tbody tr:last-child td {
  border-bottom: none;
}
/* 斑马纹 + 行悬浮（Dozzle 同款观感） */
.container-table tbody tr:nth-child(even) {
  background: rgba(255, 255, 255, 0.015);
}
.row {
  cursor: pointer;
  transition: background 0.1s;
}
.row:hover {
  background: var(--bg-elev) !important;
}
.row.stopped {
  opacity: 0.55;
}
.row.stopped:hover {
  opacity: 0.85;
}
.col-name {
  min-width: 220px;
}
.name-line {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.name {
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.image-line {
  color: var(--text-faint);
  font-size: 11px;
  font-family: var(--mono);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-top: 2px;
  padding-left: 16px; /* 对齐名称（dot 8px + gap 8px） */
}
.col-state {
  white-space: nowrap;
  color: var(--text-dim);
}
.state-text.is-stopped {
  color: var(--text-faint);
}
.col-stat {
  white-space: nowrap;
}
.stat-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}
.stat-val {
  font-family: var(--mono);
  font-size: 11.5px;
  min-width: 62px;
  text-align: right;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
  color: var(--text-dim);
}
.stat-val.ok {
  color: var(--green);
}
.stat-val.mild {
  color: #6ca0f6;
}
.stat-val.warm {
  color: var(--amber);
}
.stat-val.hot {
  color: var(--red);
}
.col-net {
  white-space: nowrap;
  text-align: right;
  font-size: 11.5px;
}
.net-rx {
  color: #6ca0f6;
  font-variant-numeric: tabular-nums;
}
.net-tx {
  color: var(--green);
  font-variant-numeric: tabular-nums;
  margin-left: 10px;
}
.col-num {
  white-space: nowrap;
  text-align: right;
  color: var(--text-dim);
}
.mono {
  font-family: var(--mono);
  font-variant-numeric: tabular-nums;
}
.col-when {
  white-space: nowrap;
  color: var(--text-faint);
  font-size: 12px;
  text-align: right;
}
.timeline-section {
  margin-top: 22px;
}
.section-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}
.section-head h2 {
  font-size: 14px;
  font-weight: 650;
  margin: 0;
}
.section-head .sub {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--text-faint);
  font-size: 11px;
  font-family: var(--mono);
}
.timeline {
  background: var(--bg-panel);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius);
  padding: 8px;
}
</style>
