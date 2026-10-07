<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../lib/api'
import type { Container } from '../types'
import { fmtDateTime, timeAgo } from '../lib/format'
import JsonTree from '../components/JsonTree.vue'

const route = useRoute()
const router = useRouter()
const id = computed(() => String(route.params.id ?? ''))

const container = ref<Container | null>(null)
const inspect = ref<unknown>(null)
const loading = ref(false)
const error = ref('')

async function load(): Promise<void> {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    const res = await api.container(id.value)
    container.value = res.container ?? null
    inspect.value = res.inspect ?? null
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

onMounted(load)

const portsText = computed(() => {
  const c = container.value
  if (!c || !Array.isArray(c.ports) || c.ports.length === 0) return []
  return c.ports.map((p) => `${p.host} → ${p.container}/${p.protocol}`)
})

const labelEntries = computed(() => {
  const l = container.value?.labels
  if (!l || typeof l !== 'object') return []
  return Object.entries(l as Record<string, string>)
})

function goTab(tab: 'logs' | 'terminal'): void {
  router.push(`/container/${encodeURIComponent(id.value)}/${tab}`)
}
</script>

<template>
  <div class="page">
    <div class="page-header">
      <span class="dot" :class="container?.state ?? 'unknown'" />
      <h1>{{ container?.name ?? id }}</h1>
      <span v-if="container?.statusText" class="sub">{{ container.statusText }}</span>
      <span class="spacer" />
      <nav class="lt-tabs">
        <button class="btn" @click="goTab('logs')">Logs</button>
        <button class="btn active">Inspect</button>
        <button class="btn" @click="goTab('terminal')">Terminal</button>
      </nav>
      <button class="btn" :disabled="loading" @click="load()">{{ loading ? '加载中…' : '刷新' }}</button>
    </div>

    <p v-if="error" class="error-text">GET /api/containers/{id} 失败：{{ error }}</p>
    <div v-if="loading && !container" class="empty">加载中…</div>

    <template v-if="container">
      <section class="info-card">
        <div class="info-grid">
          <div class="info-item">
            <div class="info-label">镜像</div>
            <div class="info-value mono">{{ container.image || '—' }}</div>
          </div>
          <div class="info-item">
            <div class="info-label">状态</div>
            <div class="info-value">
              <span class="state-chip" :class="container.state">{{ container.state }}</span>
              <span v-if="container.health && container.health !== 'none'" class="badge" :class="container.health">
                {{ container.health }}
              </span>
            </div>
          </div>
          <div class="info-item">
            <div class="info-label">命令</div>
            <div class="info-value mono" :title="container.command">{{ container.command || '—' }}</div>
          </div>
          <div class="info-item">
            <div class="info-label">创建时间</div>
            <div class="info-value" :title="container.createdAt">
              {{ container.createdAt ? `${fmtDateTime(container.createdAt)}（${timeAgo(container.createdAt)}）` : '—' }}
            </div>
          </div>
          <div class="info-item">
            <div class="info-label">启动时间</div>
            <div class="info-value" :title="container.startedAt">
              {{ container.startedAt ? `${fmtDateTime(container.startedAt)}（${timeAgo(container.startedAt)}）` : '—' }}
            </div>
          </div>
          <div class="info-item">
            <div class="info-label">端口</div>
            <div class="info-value mono">
              <template v-if="portsText.length > 0">
                <span v-for="p in portsText" :key="p" class="port-chip">{{ p }}</span>
              </template>
              <template v-else>—</template>
            </div>
          </div>
          <div class="info-item info-item-wide">
            <div class="info-label">容器 ID</div>
            <div class="info-value mono" :title="container.id">{{ container.id }}</div>
          </div>
        </div>

        <div v-if="labelEntries.length > 0" class="labels-block">
          <div class="info-label">Labels</div>
          <div class="labels">
            <span v-for="[k, v] in labelEntries" :key="k" class="label-chip" :title="`${k}=${v}`">
              <b>{{ k }}</b>={{ v }}
            </span>
          </div>
        </div>
      </section>

      <section class="inspect-section">
        <div class="section-head">
          <h2>inspect 原始 JSON</h2>
          <span class="sub">来自 GET /api/containers/{id} 的 inspect 字段</span>
        </div>
        <div class="inspect-tree">
          <JsonTree v-if="inspect !== null && inspect !== undefined" :value="inspect" />
          <div v-else class="empty">inspect 数据为空</div>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.lt-tabs {
  display: flex;
  gap: 6px;
}
.info-card {
  background: var(--bg-panel);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius);
  padding: 16px 18px;
  margin-bottom: 18px;
}
.info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 14px 24px;
}
.info-item-wide {
  grid-column: 1 / -1;
}
.info-label {
  font-size: 10.5px;
  text-transform: uppercase;
  letter-spacing: 0.07em;
  color: var(--text-faint);
  margin-bottom: 3px;
}
.info-value {
  font-size: 13px;
  color: var(--text);
  min-width: 0;
  word-break: break-all;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.info-value.mono {
  font-family: var(--mono);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  display: block;
}
.state-chip {
  display: inline-flex;
  padding: 1px 9px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 650;
}
.state-chip.running {
  color: var(--green);
  background: rgba(63, 185, 80, 0.14);
}
.state-chip.paused {
  color: var(--amber);
  background: rgba(210, 153, 34, 0.14);
}
.state-chip.restarting {
  color: var(--accent);
  background: var(--accent-soft);
}
.state-chip.exited,
.state-chip.created {
  color: var(--text-dim);
  background: #232d3a;
}
.state-chip.dead {
  color: var(--red);
  background: rgba(248, 81, 73, 0.14);
}
.state-chip.unknown {
  color: var(--text-dim);
  background: #232d3a;
}
.port-chip {
  background: #0d1117;
  border: 1px solid var(--border-soft);
  border-radius: 5px;
  padding: 2px 8px;
  font-size: 11.5px;
}
.labels-block {
  margin-top: 16px;
  border-top: 1px solid var(--border-soft);
  padding-top: 12px;
}
.labels {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 4px;
}
.label-chip {
  font-family: var(--mono);
  font-size: 11px;
  color: var(--text-dim);
  background: #0d1117;
  border: 1px solid var(--border-soft);
  border-radius: 5px;
  padding: 2px 8px;
  max-width: 420px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.label-chip b {
  color: var(--accent);
  font-weight: 600;
}
.inspect-section {
  background: var(--bg-panel);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius);
  padding: 14px 18px;
}
.section-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 10px;
}
.section-head h2 {
  font-size: 13.5px;
  font-weight: 650;
  margin: 0;
}
.section-head .sub {
  color: var(--text-faint);
  font-size: 11.5px;
}
.inspect-tree {
  max-height: 60vh;
  overflow: auto;
}
</style>
