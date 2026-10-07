<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTerminal } from '../composables/useTerminal'
import { containerById } from '../stores/containers'

const route = useRoute()
const router = useRouter()
const id = computed(() => String(route.params.id ?? ''))
const container = computed(() => containerById(id.value))

const hostEl = ref<HTMLElement | null>(null)
const { state, start, reconnect } = useTerminal(id, hostEl, 'sh')

onMounted(() => {
  start()
})

const overlay = computed(() => {
  if (state.value.phase === 'exited') {
    return {
      title: `进程已退出（exit code ${state.value.exitCode ?? '?'}）`,
      hint: '点击「重新连接」会重新执行 cmd=sh 建立新会话。',
      danger: true,
    }
  }
  if (state.value.phase === 'error') {
    return {
      title: '终端错误',
      hint: state.value.errorMessage || 'unknown error',
      danger: true,
    }
  }
  if (state.value.phase === 'disconnected') {
    return {
      title: '连接已断开',
      hint: 'WebSocket /api/ws/exec 已关闭，可重新连接。',
      danger: true,
    }
  }
  return null
})

function goTab(tab: 'logs' | 'inspect'): void {
  router.push(`/container/${encodeURIComponent(id.value)}/${tab}`)
}
</script>

<template>
  <div class="termpage">
    <header class="term-toolbar">
      <div class="lt-left">
        <span class="dot" :class="container?.state ?? 'unknown'" />
        <span class="lt-name" :title="container?.id ?? id">{{ container?.name ?? id }}</span>
        <span v-if="container?.statusText" class="lt-status">{{ container.statusText }}</span>
        <nav class="lt-tabs">
          <button class="btn" @click="goTab('logs')">Logs</button>
          <button class="btn" @click="goTab('inspect')">Inspect</button>
          <button class="btn active">Terminal</button>
        </nav>
      </div>
      <div class="term-right">
        <span class="term-state" :class="state.phase">
          <span class="dot" :class="state.phase === 'ready' ? 'running' : state.phase === 'connecting' ? 'restarting' : 'offline'" />
          {{ state.phase === 'ready' ? 'pty ready · sh' : state.phase === 'connecting' ? '连接中…' : state.phase }}
        </span>
        <button class="btn" @click="reconnect()">重新连接</button>
      </div>
    </header>

    <div class="term-host-wrap">
      <div ref="hostEl" class="term-host"></div>

      <div v-if="overlay" class="term-overlay">
        <div class="term-overlay-card">
          <div class="toc-title" :class="{ danger: overlay.danger }">{{ overlay.title }}</div>
          <div class="toc-hint">{{ overlay.hint }}</div>
          <button class="btn primary-btn" @click="reconnect()">重新连接</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.termpage {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: #0b0f14;
}
.term-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border-soft);
  background: var(--bg-panel);
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
.term-right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 10px;
}
.term-state {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-family: var(--mono);
  font-size: 11px;
  color: var(--text-dim);
}
.term-host-wrap {
  flex: 1;
  min-height: 0;
  position: relative;
  padding: 8px 6px 8px 10px;
}
.term-host {
  width: 100%;
  height: 100%;
}
.term-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(8, 11, 15, 0.72);
  backdrop-filter: blur(2px);
}
.term-overlay-card {
  text-align: center;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 22px 30px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  align-items: center;
  box-shadow: 0 18px 48px rgba(0, 0, 0, 0.5);
}
.toc-title {
  font-size: 15px;
  font-weight: 650;
}
.toc-title.danger {
  color: var(--red);
}
.toc-hint {
  color: var(--text-dim);
  font-size: 12.5px;
  max-width: 380px;
}
.primary-btn {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}
</style>
