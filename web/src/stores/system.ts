import { reactive } from 'vue'
import { api } from '../lib/api'
import type { SystemInfo } from '../types'

export const systemStore = reactive({
  info: null as SystemInfo | null,
  error: '',
  loaded: false,
})

export async function loadSystemInfo(): Promise<void> {
  try {
    systemStore.info = await api.systemInfo()
    systemStore.error = ''
  } catch (e) {
    systemStore.error = e instanceof Error ? e.message : String(e)
  } finally {
    systemStore.loaded = true
  }
}
