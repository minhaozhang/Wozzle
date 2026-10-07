<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { Container, Stat } from '../types'
import { containerAction, removeContainer } from '../stores/containers'
import { formatBytes, formatPercent } from '../lib/format'
import ConfirmDialog from './ConfirmDialog.vue'

const props = defineProps<{
  container: Container
  stat?: Stat
  active: boolean
}>()

const router = useRouter()

const menuOpen = ref(false)
const menuStyle = reactive({ top: '0px', left: '0px' })
const menuEl = ref<HTMLElement | null>(null)
const btnEl = ref<HTMLElement | null>(null)

const REMOVE_HINT = '将向 wslc 发送 remove；容器若在运行需勾选强制移除。'

interface Pending {
  verb: 'stop' | 'kill' | 'restart' | 'remove'
  title: string
  message: string
  confirmLabel: string
}

const pending = ref<Pending | null>(null)
const busy = ref(false)
const actionError = ref('')
const confirmDanger = computed(() => pending.value !== null && pending.value.verb !== 'restart')

const stateText = computed(() => props.container.state ?? 'unknown')
const health = computed(() => props.container.health ?? 'none')
const showHealth = computed(() => ['healthy', 'unhealthy', 'starting'].includes(health.value))

const cpuPct = computed(() => {
  const v = props.stat?.cpuPercent
  return v === undefined || !Number.isFinite(v) ? null : Math.max(0, Math.min(100, v))
})
const memPct = computed(() => {
  const v = props.stat?.memPercent
  return v === undefined || !Number.isFinite(v) ? null : Math.max(0, Math.min(100, v))
})
const memText = computed(() =>
  props.stat ? `${formatBytes(props.stat.memBytes)} / ${formatBytes(props.stat.memLimitBytes)}` : '',
)
const pidsText = computed(() => (props.stat ? `pids ${props.stat.pids}` : ''))

function toggleMenu(e: MouseEvent): void {
  e.preventDefault()
  e.stopPropagation()
  if (menuOpen.value) {
    menuOpen.value = false
    return
  }
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const menuW = 196
  const menuH = 300
  const left = Math.max(8, Math.min(rect.right - menuW, window.innerWidth - menuW - 8))
  const top = Math.min(rect.bottom + 4, Math.max(8, window.innerHeight - menuH - 8))
  menuStyle.top = `${top}px`
  menuStyle.left = `${left}px`
  menuOpen.value = true
}

function onDocMouseDown(e: MouseEvent): void {
  if (!menuOpen.value) return
  const target = e.target as Node
  if (menuEl.value?.contains(target) || btnEl.value?.contains(target)) return
  menuOpen.value = false
}

function go(page: 'logs' | 'inspect' | 'terminal'): void {
  menuOpen.value = false
  router.push(`/container/${encodeURIComponent(props.container.id)}/${page}`)
}

function onMenuAction(verb: 'logs' | 'inspect' | 'terminal' | 'start' | 'stop' | 'kill' | 'restart' | 'remove'): void {
  if (verb === 'logs' || verb === 'inspect' || verb === 'terminal') {
    go(verb)
    return
  }
  menuOpen.value = false
  if (verb === 'start') {
    void runStart()
    return
  }
  const c = props.container
  const labels: Record<string, Pending> = {
    stop: {
      verb: 'stop',
      title: '停止容器',
      message: `确认停止容器「${c.name}」？POST /api/containers/{id}/stop?confirm=1`,
      confirmLabel: '停止',
    },
    kill: {
      verb: 'kill',
      title: '强杀容器',
      message: `确认向容器「${c.name}」发送 kill？POST /api/containers/{id}/kill?confirm=1`,
      confirmLabel: 'Kill',
    },
    restart: {
      verb: 'restart',
      title: '重启容器',
      message: `确认重启容器「${c.name}」？POST /api/containers/{id}/restart?confirm=1`,
      confirmLabel: '重启',
    },
    remove: {
      verb: 'remove',
      title: '移除容器',
      message: `确认移除容器「${c.name}」？DELETE /api/containers/{id}?confirm=1。${REMOVE_HINT}`,
      confirmLabel: '移除',
    },
  }
  actionError.value = ''
  pending.value = labels[verb]
}

async function runStart(): Promise<void> {
  busy.value = true
  actionError.value = ''
  try {
    await containerAction(props.container.id, 'start')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = false
  }
}

async function doConfirm(force: boolean): Promise<void> {
  const p = pending.value
  if (!p) return
  busy.value = true
  actionError.value = ''
  try {
    if (p.verb === 'remove') await removeContainer(props.container.id, force)
    else await containerAction(props.container.id, p.verb)
    pending.value = null
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = false
  }
}

function cancelConfirm(): void {
  if (busy.value) return
  actionError.value = ''
  pending.value = null
}

onMounted(() => document.addEventListener('mousedown', onDocMouseDown))
onBeforeUnmount(() => document.removeEventListener('mousedown', onDocMouseDown))
</script>

<template>
  <div
    class="ccard"
    :class="{ active }"
    @click="go('logs')"
  >
    <div class="ccard-head">
      <span class="dot" :class="stateText" />
      <span class="ccard-name" :title="container.name">{{ container.name }}</span>
      <button
        ref="btnEl"
        class="menu-btn"
        title="操作"
        aria-label="容器操作菜单"
        @click="toggleMenu"
      >
        ⋮
      </button>
    </div>
    <div class="ccard-image" :title="container.image">{{ container.image || '—' }}</div>
    <div class="ccard-meta">
      <span class="state-text" :class="stateText">{{ stateText }}</span>
      <span v-if="showHealth" class="badge" :class="health">{{ health }}</span>
      <span v-if="container.statusText" class="status-text" :title="container.statusText">{{ container.statusText }}</span>
    </div>
    <div class="mini-bars">
      <div class="mini-bar-row">
        <span class="mini-label">CPU</span>
        <div class="mini-track"><div class="mini-fill cpu" :style="{ width: (cpuPct ?? 0) + '%' }" /></div>
        <span class="mini-val">{{ cpuPct === null ? '—' : formatPercent(cpuPct) }}</span>
      </div>
      <div class="mini-bar-row">
        <span class="mini-label">MEM</span>
        <div class="mini-track"><div class="mini-fill mem" :style="{ width: (memPct ?? 0) + '%' }" /></div>
        <span class="mini-val">{{ memPct === null ? '—' : formatPercent(memPct) }}</span>
      </div>
      <div v-if="memText || pidsText" class="mini-sub">
        <span v-if="memText">{{ memText }}</span>
        <span v-if="pidsText">{{ pidsText }}</span>
      </div>
    </div>
  </div>

  <Teleport to="body">
    <div v-if="menuOpen" ref="menuEl" class="card-menu" :style="menuStyle" @click.stop>
      <button class="menu-item" @click="onMenuAction('logs')">Logs</button>
      <button class="menu-item" @click="onMenuAction('inspect')">Inspect</button>
      <button class="menu-item" @click="onMenuAction('terminal')">Terminal</button>
      <div class="menu-sep" />
      <button class="menu-item" :disabled="busy" @click="onMenuAction('start')">Start</button>
      <button class="menu-item" :disabled="busy" @click="onMenuAction('stop')">Stop</button>
      <button class="menu-item danger" :disabled="busy" @click="onMenuAction('kill')">Kill</button>
      <button class="menu-item" :disabled="busy" @click="onMenuAction('restart')">Restart</button>
      <button class="menu-item danger" :disabled="busy" @click="onMenuAction('remove')">Remove</button>
    </div>
  </Teleport>

  <ConfirmDialog
    :open="pending !== null"
    :title="pending?.title ?? ''"
    :message="pending?.message ?? ''"
    :confirm-label="pending?.confirmLabel"
    :danger="confirmDanger"
    :show-force="pending?.verb === 'remove'"
    :error="actionError"
    @confirm="doConfirm"
    @cancel="cancelConfirm"
  />
</template>

<style scoped>
.ccard {
  background: var(--bg-panel);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius);
  padding: 10px 12px 12px;
  cursor: pointer;
  transition: border-color 0.12s, background 0.12s;
}
.ccard:hover {
  border-color: var(--border);
  background: var(--bg-elev);
}
.ccard.active {
  border-color: var(--accent);
  background: var(--bg-active);
}
.ccard-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.ccard-name {
  flex: 1;
  min-width: 0;
  font-weight: 600;
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.menu-btn {
  flex: none;
  width: 22px;
  height: 22px;
  border: none;
  border-radius: 5px;
  background: transparent;
  color: var(--text-dim);
  font-size: 15px;
  line-height: 1;
  cursor: pointer;
}
.menu-btn:hover {
  background: var(--bg-hover);
  color: var(--text);
}
.ccard-image {
  margin-top: 2px;
  color: var(--text-dim);
  font-size: 11.5px;
  font-family: var(--mono);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.ccard-meta {
  margin-top: 6px;
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 18px;
}
.state-text {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.03em;
  color: var(--text-dim);
}
.state-text.running {
  color: var(--green);
}
.state-text.paused {
  color: var(--amber);
}
.state-text.restarting {
  color: var(--accent);
}
.state-text.dead {
  color: var(--red);
}
.status-text {
  color: var(--text-faint);
  font-size: 11px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
  flex: 1;
}
.mini-bars {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.mini-bar-row {
  display: flex;
  align-items: center;
  gap: 7px;
}
.mini-label {
  font-size: 9.5px;
  letter-spacing: 0.06em;
  color: var(--text-faint);
  width: 26px;
  flex: none;
}
.mini-track {
  flex: 1;
  height: 4px;
  border-radius: 3px;
  background: #232d3a;
  overflow: hidden;
}
.mini-fill {
  height: 100%;
  border-radius: 3px;
  transition: width 0.5s ease;
}
.mini-fill.cpu {
  background: linear-gradient(90deg, #2f6fd8, #6ca0f6);
}
.mini-fill.mem {
  background: linear-gradient(90deg, #2f9e5b, #7ee787);
}
.mini-val {
  font-size: 10px;
  color: var(--text-dim);
  font-family: var(--mono);
  width: 42px;
  text-align: right;
  flex: none;
}
.mini-sub {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  font-size: 9.5px;
  color: var(--text-faint);
  font-family: var(--mono);
}
.card-menu {
  position: fixed;
  width: 196px;
  background: var(--bg-elev);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 4px;
  z-index: 250;
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.5);
  display: flex;
  flex-direction: column;
}
.menu-item {
  text-align: left;
  border: none;
  background: transparent;
  color: var(--text);
  padding: 7px 10px;
  border-radius: 5px;
  font-size: 12.5px;
  font-family: var(--sans);
  cursor: pointer;
}
.menu-item:hover {
  background: var(--bg-hover);
}
.menu-item.danger {
  color: var(--red);
}
.menu-item.danger:hover {
  background: rgba(248, 81, 73, 0.12);
}
.menu-item:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.menu-sep {
  height: 1px;
  background: var(--border-soft);
  margin: 4px 8px;
}
</style>
