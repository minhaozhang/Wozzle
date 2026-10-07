<script setup lang="ts">
import { computed, ref } from 'vue'

const props = withDefaults(
  defineProps<{
    label?: string
    value: unknown
    depth?: number
  }>(),
  {
    label: '',
    depth: 0,
  },
)

const isContainer = computed(() => typeof props.value === 'object' && props.value !== null)
const isArray = computed(() => Array.isArray(props.value))

const entries = computed<[string, unknown][]>(() => {
  if (!isContainer.value) return []
  if (isArray.value) {
    return (props.value as unknown[]).map((v, i) => [String(i), v] as [string, unknown])
  }
  return Object.entries(props.value as Record<string, unknown>)
})

const expanded = ref(props.depth < 2)

const braceOpen = computed(() => (isArray.value ? '[' : '{'))
const braceClose = computed(() => (isArray.value ? ']' : '}'))
const summary = computed(() => `${entries.value.length} ${isArray.value ? 'items' : 'keys'}`)

const primClass = computed(() => {
  const v = props.value
  if (v === null || v === undefined) return 't-null'
  if (typeof v === 'string') return 't-string'
  if (typeof v === 'number') return 't-number'
  if (typeof v === 'boolean') return 't-boolean'
  return 't-other'
})

const primText = computed(() => {
  const v = props.value
  if (v === null || v === undefined) return 'null'
  if (typeof v === 'string') return JSON.stringify(v)
  return String(v)
})
</script>

<template>
  <div class="jtree-node">
    <!-- 对象/数组 -->
    <div v-if="isContainer" class="jt-row">
      <button class="jt-caret" :class="{ open: expanded }" :aria-expanded="expanded" @click="expanded = !expanded">
        <span class="caret-icon">{{ expanded ? '▾' : '▸' }}</span>
      </button>
      <span v-if="label !== ''" class="jt-key">{{ label }}:</span>
      <span class="jt-brace">{{ braceOpen }}</span>
      <button v-if="!expanded" class="jt-collapsed-hint" :title="`展开（${summary}）`" @click="expanded = true">
        {{ summary }} {{ braceClose }}
      </button>
      <span v-else class="jt-summary">{{ summary }}</span>
    </div>
    <div v-if="isContainer && expanded" class="jt-children">
      <div v-if="entries.length === 0" class="jt-empty">{{ braceClose }}<span class="jt-hint">空</span></div>
      <JsonTree
        v-for="[k, v] in entries"
        :key="k"
        :label="isArray ? '' : k"
        :value="v"
        :depth="depth + 1"
      />
      <div class="jt-row jt-close"><span class="jt-brace">{{ braceClose }}</span></div>
    </div>

    <!-- 叶子节点 -->
    <div v-if="!isContainer" class="jt-row">
      <span v-if="label !== ''" class="jt-key">{{ label }}:</span>
      <span class="jt-val" :class="primClass" :title="primText">{{ primText }}</span>
    </div>
  </div>
</template>

<style scoped>
.jtree-node {
  font-family: var(--mono);
  font-size: 12px;
  line-height: 1.7;
}
.jt-row {
  display: flex;
  align-items: baseline;
  gap: 6px;
  min-width: 0;
}
.jt-caret {
  border: none;
  background: transparent;
  color: var(--text-faint);
  cursor: pointer;
  padding: 0 1px;
  width: 14px;
  flex: none;
  font-size: 10px;
  text-align: center;
}
.caret-icon {
  display: inline-block;
  transition: transform 0.1s;
}
.jt-key {
  color: var(--accent);
  white-space: nowrap;
}
.jt-brace {
  color: var(--text-dim);
}
.jt-summary {
  color: var(--text-faint);
  font-size: 10.5px;
  transform: translateY(-1px);
}
.jt-collapsed-hint {
  border: none;
  background: transparent;
  color: var(--text-faint);
  cursor: pointer;
  padding: 0 4px;
  border-radius: 4px;
  font-family: var(--mono);
  font-size: 11px;
}
.jt-collapsed-hint:hover {
  background: var(--bg-hover);
  color: var(--text);
}
.jt-children {
  margin-left: 8px;
  padding-left: 10px;
  border-left: 1px solid var(--border-soft);
}
.jt-empty {
  color: var(--text-faint);
  display: flex;
  gap: 6px;
  align-items: baseline;
}
.jt-close {
  margin-left: 0;
}
.jt-val {
  white-space: pre-wrap;
  word-break: break-all;
  min-width: 0;
  max-width: 640px;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
}
.t-string {
  color: #7ee787;
}
.t-number {
  color: #6ca0f6;
}
.t-boolean {
  color: #d2a8ff;
}
.t-null {
  color: var(--text-faint);
  font-style: italic;
}
.t-other {
  color: var(--text);
}
</style>
