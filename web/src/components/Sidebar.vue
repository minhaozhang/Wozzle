<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { containersStore, filteredContainers, refreshContainers } from '../stores/containers'
import { statOf, statsStore } from '../stores/stats'
import { eventsStore } from '../stores/events'
import { loadSystemInfo, systemStore } from '../stores/system'
import ContainerCard from './ContainerCard.vue'

const route = useRoute()

onMounted(() => {
  void loadSystemInfo()
})

const versionText = computed(() => {
  const info = systemStore.info
  if (!info) return ''
  const parts: string[] = []
  if (info.wslcVersion) parts.push(`wslc ${info.wslcVersion}`)
  if (info.wozzleVersion) parts.push(`wozzle ${info.wozzleVersion}`)
  return parts.join(' · ')
})

const runningCount = computed(() => containersStore.list.filter((c) => c.state === 'running').length)

const backendOnline = computed(() => containersStore.error === '' && containersStore.list.length > 0)
const backendUnknown = computed(
  () => containersStore.error === '' && containersStore.list.length === 0 && containersStore.loadedAt === 0,
)

const wsDotClass = (s: string): string =>
  s === 'open' ? 'running' : s === 'connecting' || s === 'reconnecting' ? 'restarting' : 'offline'
</script>

<template>
  <aside class="sidebar">
    <div class="side-head">
      <router-link to="/" class="brand">
        <svg viewBox="0 0 32 32" class="brand-logo" aria-hidden="true">
          <rect width="32" height="32" rx="7" fill="#151b23" stroke="#2a3442" />
          <circle cx="16" cy="17" r="7.5" fill="none" stroke="#4f8cff" stroke-width="2.5" />
          <circle cx="24" cy="9" r="3" fill="#3fb950" />
        </svg>
        <span class="brand-name">wozzle</span>
      </router-link>
      <div class="brand-sub" :title="versionText || (systemStore.error ? `后端不可达：${systemStore.error}` : '')">
        <template v-if="versionText">{{ versionText }}</template>
        <template v-else-if="systemStore.error">backend unreachable</template>
        <template v-else>&nbsp;</template>
      </div>
    </div>

    <nav class="side-nav">
      <router-link to="/" class="nav-item" :class="{ active: route.path === '/' }">
        <svg viewBox="0 0 16 16" class="nav-ico" aria-hidden="true">
          <rect x="1" y="1" width="6" height="6" rx="1.5" />
          <rect x="9" y="1" width="6" height="6" rx="1.5" />
          <rect x="1" y="9" width="6" height="6" rx="1.5" />
          <rect x="9" y="9" width="6" height="6" rx="1.5" />
        </svg>
        <span>仪表盘</span>
      </router-link>
    </nav>

    <div class="side-search">
      <span class="search-icon">⌕</span>
      <input
        v-model="containersStore.filter"
        class="input search-input"
        type="text"
        placeholder="过滤容器（名称 / 镜像 / ID）"
        spellcheck="false"
      />
      <span v-if="containersStore.filter" class="search-count">{{ filteredContainers.length }}</span>
    </div>

    <div class="side-group">容器</div>
    <div class="side-list">
      <div v-if="containersStore.error && containersStore.list.length === 0" class="side-empty">
        <span class="icon">✕</span>
        无法连接后端
        <div class="side-empty-detail">{{ containersStore.error }}</div>
        <button class="btn" @click="refreshContainers()">重试</button>
      </div>
      <div v-else-if="filteredContainers.length === 0" class="side-empty">
        <span class="icon">▢</span>
        <template v-if="containersStore.filter">没有匹配的容器</template>
        <template v-else>暂无容器</template>
      </div>
      <ContainerCard
        v-for="c in filteredContainers"
        :key="c.id"
        :container="c"
        :stat="statOf(c.id)"
        :active="route.params.id === c.id"
      />
    </div>

    <div class="side-foot">
      <span class="foot-item" :title="`REST /api/containers · ${runningCount}/${containersStore.list.length} running`">
        <span class="dot" :class="backendOnline ? 'running' : backendUnknown ? 'restarting' : 'offline'" />
        {{ runningCount }}/{{ containersStore.list.length }} running
      </span>
      <span class="foot-item" title="/api/ws/stats">
        <span class="dot" :class="wsDotClass(statsStore.status)" /> stats
      </span>
      <span class="foot-item" title="/api/ws/events">
        <span class="dot" :class="wsDotClass(eventsStore.status)" /> events
      </span>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  width: var(--sidebar-w);
  flex: none;
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg-panel);
  border-right: 1px solid var(--border-soft);
}
.side-head {
  padding: 14px 14px 10px;
}
.brand {
  display: flex;
  align-items: center;
  gap: 9px;
  color: var(--text);
}
.brand-logo {
  width: 26px;
  height: 26px;
  flex: none;
}
.brand-name {
  font-size: 17px;
  font-weight: 700;
  letter-spacing: 0.01em;
}
.brand-sub {
  margin-top: 3px;
  margin-left: 35px;
  font-size: 10.5px;
  color: var(--text-faint);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.side-nav {
  padding: 0 12px 10px;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 7px 10px;
  border-radius: 8px;
  color: var(--text-dim);
  font-size: 13px;
  font-weight: 550;
  text-decoration: none;
  transition: background 0.1s, color 0.1s;
}
.nav-item:hover {
  background: var(--bg-elev);
  color: var(--text);
}
.nav-item.active {
  background: rgba(79, 140, 255, 0.12);
  color: var(--text);
}
.nav-ico {
  width: 15px;
  height: 15px;
  flex: none;
  fill: currentColor;
  opacity: 0.75;
}
.nav-item.active .nav-ico {
  opacity: 1;
  fill: #4f8cff;
}
.side-group {
  padding: 0 14px 6px;
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0.06em;
  color: var(--text-faint);
}
.side-search {
  position: relative;
  padding: 0 12px 10px;
}
.search-icon {
  position: absolute;
  left: 22px;
  top: 7px;
  color: var(--text-faint);
  font-size: 14px;
  pointer-events: none;
}
.search-input {
  padding-left: 28px;
  font-size: 12.5px;
}
.search-count {
  position: absolute;
  right: 22px;
  top: 6px;
  font-size: 10.5px;
  color: var(--text-faint);
  font-family: var(--mono);
}
.side-list {
  flex: 1;
  overflow-y: auto;
  padding: 0 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.side-empty {
  color: var(--text-dim);
  text-align: center;
  padding: 36px 8px;
  font-size: 12.5px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}
.side-empty .icon {
  font-size: 24px;
  opacity: 0.5;
}
.side-empty-detail {
  font-size: 11px;
  color: var(--text-faint);
  word-break: break-all;
}
.side-foot {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 9px 14px;
  border-top: 1px solid var(--border-soft);
  font-size: 10.5px;
  color: var(--text-faint);
}
.foot-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-family: var(--mono);
}
</style>
