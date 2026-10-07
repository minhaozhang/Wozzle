<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    message: string
    confirmLabel?: string
    danger?: boolean
    showForce?: boolean
    error?: string
  }>(),
  {
    confirmLabel: '确认',
    danger: false,
    showForce: false,
    error: '',
  },
)

const emit = defineEmits<{
  confirm: [force: boolean]
  cancel: []
}>()

const force = ref(false)
const modalEl = ref<HTMLElement | null>(null)

function onKeydown(e: KeyboardEvent): void {
  if (e.key === 'Escape' && props.open) emit('cancel')
}

watch(
  () => props.open,
  (v) => {
    if (v) {
      force.value = false
      document.addEventListener('keydown', onKeydown)
      void nextTick(() => modalEl.value?.querySelector<HTMLButtonElement>('.btn-confirm')?.focus())
    } else {
      document.removeEventListener('keydown', onKeydown)
    }
  },
)

onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div v-if="open" class="modal-overlay" @mousedown.self="emit('cancel')">
        <div ref="modalEl" class="modal" role="dialog" aria-modal="true">
          <h3 class="modal-title">
            <span v-if="danger" class="warn-icon">⚠</span>
            {{ title }}
          </h3>
          <p class="modal-msg">{{ message }}</p>
          <label v-if="showForce" class="modal-force">
            <input v-model="force" type="checkbox" />
            强制移除（force=1）
          </label>
          <p v-if="error" class="error-text modal-error">{{ error }}</p>
          <div class="modal-actions">
            <button class="btn" @click="emit('cancel')">取消</button>
            <button class="btn btn-confirm" :class="danger ? 'danger' : 'primary'" @click="emit('confirm', force)">
              {{ confirmLabel }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(5, 8, 12, 0.62);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 300;
}
.modal {
  width: min(420px, calc(100vw - 48px));
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 18px 20px;
  box-shadow: 0 18px 48px rgba(0, 0, 0, 0.5);
}
.modal-title {
  margin: 0 0 8px;
  font-size: 15px;
  font-weight: 650;
  display: flex;
  align-items: center;
  gap: 8px;
}
.warn-icon {
  color: var(--amber);
}
.modal-msg {
  margin: 0 0 14px;
  color: var(--text-dim);
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-word;
}
.modal-force {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: -4px 0 14px;
  font-size: 12.5px;
  color: var(--text-dim);
  cursor: pointer;
}
.modal-error {
  margin: 0 0 12px;
}
.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
.btn.primary {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.14s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
