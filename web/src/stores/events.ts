import { reactive } from 'vue'
import { ReconnectingSocket, wsUrl, type SocketStatus } from '../composables/ReconnectingSocket'
import type { Event, EventsServerMessage } from '../types'

const MAX_EVENTS = 200

interface EventsState {
  status: SocketStatus
  events: Event[]
}

export const eventsStore = reactive<EventsState>({
  status: 'connecting',
  events: [],
})

type Listener = (ev: Event) => void
const listeners = new Set<Listener>()

export function onContainerEvent(fn: Listener): () => void {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

let socket: ReconnectingSocket | null = null

export function startEventsStream(): void {
  if (socket) return
  socket = new ReconnectingSocket(wsUrl('/api/ws/events'), {
    onMessage: (msg) => {
      const m = msg as EventsServerMessage
      if (m.t === 'event' && m.event && typeof m.event === 'object') {
        const ev = m.event
        eventsStore.events.unshift(ev)
        if (eventsStore.events.length > MAX_EVENTS) eventsStore.events.length = MAX_EVENTS
        for (const fn of listeners) fn(ev)
      }
    },
    onStatus: (s) => {
      eventsStore.status = s
    },
  })
  socket.connect()
}
