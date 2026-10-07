<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { containersStore, refreshContainers, sortedContainers } from '../stores/containers'
import { statsStore, statOf } from '../stores/stats'
import { eventsStore } from '../stores/events'
import { loadSystemInfo, systemStore } from '../stores/system'
import StatChart from '../components/StatChart.vue'
import EventTimeline from '../components/EventTimeline.vue'
import { formatBytes, formatPercent, timeAgo } from '../lib/format'

const router = useRouter()

onMounted(() => {
  if (containersStore.loadedAt === 0) void refreshContainers()
  if (!systemStore.loaded) void loadSystemInfo()
})

const running = computed(() => sortedContainers.value.filter((c) => c.state === 'running'))
const stopped = computed(() => sortedContainers.value.filter((c) => c.state !== 'running'))

const cpuOf = (id: string) => {
  const s = statOf(id)
  return s ? formatPercent(s.cpuPercent) : '—'
}
const memOf = (id: string) => {
  const s = statOf(id)
  return s ? formatPercent(s.memPercent) : '—'
}
const memBytesOf = (id: string) => {
  const s = statOf(id)
  return s ? formatBytes(s.memBytes) : '—'
}
const pidsOf = (id: string) => {
  const s = statOf(id)
  return s ? String(s.pids) : '—'
}

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
</script>

<template>
  <div class="page">
    <div class="page-header">
      <h1>Dashboard</h1>
      <span class="sub">
        {{ running.length }} running / {{ containersStore.list.length }} total<template v-if="infoLine"> · {{ infoLine }}</template>
      </span>
      <span class="spacer" />
      <button class="btn" :disabled="containersStore.loading" @click="refreshContainers()">刷新</button>
    </div>

    <p v-if="containersStore.error" class="error-text">
      无法连接后端（GET /api/containers）：{{ containersStore.error }}
    </p>
    <p v-else-if="statsStore.error" class="stats-warn">stats 流错误：{{ statsStore.error }}</p>

    <div v-if="running.length === 0" class="empty">
      <span class="icon">◔</span>
      <template v-if="containersStore.list.length === 0">当前没有容器</template>
      <template v-else>没有运行中的容器（{{ stopped.length }} 个已停止）</template>
    </div>

    <div v-else class="charts-grid">
      <div
        v-for="c in running"
        :key="c.id"
        class="chart-card"
        @click="router.push(`/container/${encodeURIComponent(c.id)}/logs`)"
      >
        <div class="chart-card-head">
          <span class="dot" :class="c.state" />
          <span class="chart-card-name" :title="c.name">{{ c.name }}</span>
          <span v-if="c.health && c.health !== 'none'" class="badge" :class="c.health">{{ c.health }}</span>
          <span class="spacer" />
          <span class="chart-now cpu">CPU {{ cpuOf(c.id) }}</span>
          <span class="chart-now mem">MEM {{ memOf(c.id) }}</span>
        </div>
        <div class="chart-card-image" :title="c.image">{{ c.image }}</div>
        <StatChart :container-id="c.id" :height="118" />
        <div class="chart-card-foot">
          <span>{{ memBytesOf(c.id) }} mem</span>
          <span v-if="c.startedAt">started {{ timeAgo(c.startedAt) }}</span>
          <span v-if="statOf(c.id)">pids {{ pidsOf(c.id) }}</span>
        </div>
      </div>
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
.charts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 14px;
}
.chart-card {
  background: var(--bg-panel);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius);
  padding: 12px 14px 10px;
  cursor: pointer;
  transition: border-color 0.12s, background 0.12s;
}
.chart-card:hover {
  border-color: var(--border);
  background: var(--bg-elev);
}
.chart-card-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.chart-card-name {
  font-weight: 600;
  font-size: 13.5px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}
.chart-now {
  font-family: var(--mono);
  font-size: 11px;
  flex: none;
}
.chart-now.cpu {
  color: #6ca0f6;
}
.chart-now.mem {
  color: #7ee787;
}
.chart-card-image {
  color: var(--text-faint);
  font-size: 11px;
  font-family: var(--mono);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin: 2px 0 4px;
}
.chart-card-foot {
  display: flex;
  gap: 14px;
  color: var(--text-faint);
  font-size: 10.5px;
  font-family: var(--mono);
  padding: 4px 2px 0;
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
