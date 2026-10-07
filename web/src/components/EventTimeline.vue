<script setup lang="ts">
import { computed } from 'vue'
import { eventsStore } from '../stores/events'
import { fmtEventTime } from '../lib/format'

const MAX_SHOWN = 100

const shown = computed(() => eventsStore.events.slice(0, MAX_SHOWN))

const TYPE_CLASS: Record<string, string> = {
  start: 'ok',
  die: 'bad',
  stop: 'bad',
  kill: 'bad',
  destroy: 'bad',
  remove: 'bad',
  restart: 'info',
  create: 'info',
  health_status: 'warn',
  oom: 'bad',
}

function typeClass(t: string): string {
  return TYPE_CLASS[t] ?? 'info'
}
</script>

<template>
  <div class="timeline">
    <div v-if="shown.length === 0" class="timeline-empty">
      {{ eventsStore.status === 'open' ? '暂无事件，等待 /api/ws/events 推送…' : '正在连接事件流…' }}
    </div>
    <div v-for="(ev, i) in shown" :key="i" class="tl-row">
      <span class="tl-time">{{ fmtEventTime(ev.ts) }}</span>
      <span class="tl-type" :class="typeClass(ev.type)">{{ ev.type }}</span>
      <span class="tl-name" :title="ev.id">{{ ev.name || ev.id }}</span>
      <span class="tl-image" :title="ev.image">{{ ev.image }}</span>
    </div>
  </div>
</template>

<style scoped>
.timeline {
  max-height: 320px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-family: var(--mono);
  font-size: 12px;
}
.timeline-empty {
  color: var(--text-faint);
  padding: 20px 0;
  text-align: center;
  font-family: var(--sans);
  font-size: 12.5px;
}
.tl-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 3px 8px;
  border-radius: 5px;
}
.tl-row:hover {
  background: var(--bg-elev);
}
.tl-time {
  color: var(--text-faint);
  flex: none;
  width: 86px;
}
.tl-type {
  flex: none;
  width: 96px;
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.04em;
  padding: 1px 0;
  border-radius: 4px;
  text-align: center;
}
.tl-type.ok {
  color: var(--green);
  background: rgba(63, 185, 80, 0.12);
}
.tl-type.bad {
  color: var(--red);
  background: rgba(248, 81, 73, 0.12);
}
.tl-type.warn {
  color: var(--amber);
  background: rgba(210, 153, 34, 0.12);
}
.tl-type.info {
  color: var(--accent);
  background: rgba(79, 140, 255, 0.12);
}
.tl-name {
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}
.tl-image {
  margin-left: auto;
  color: var(--text-faint);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 40%;
}
</style>
