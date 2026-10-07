<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import uPlot from 'uplot'
import 'uplot/dist/uplot.min.css'
import { getStatSeries, statsFrame, STATS_WINDOW_MS } from '../stores/stats'

const props = defineProps<{
  containerId: string
  height?: number
}>()

const el = ref<HTMLDivElement | null>(null)
let chart: uPlot | null = null
let observer: ResizeObserver | null = null

const WINDOW_S = STATS_WINDOW_MS / 1000

function fmtClock(s: number): string {
  const d = new Date(s * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function makeOptions(width: number, height: number): uPlot.Options {
  return {
    width,
    height,
    class: 'wozzle-chart',
    mode: 1,
    cursor: {
      drag: { x: false, y: false },
      focus: false,
    },
    select: { show: false },
    legend: {
      show: true,
      live: true,
    },
    scales: {
      x: { time: false, range: () => [Date.now() / 1000 - WINDOW_S - 2, Date.now() / 1000] },
      y: { range: (u, min) => [Math.min(0, min), 100] },
    },
    axes: [
      {
        stroke: '#5c6878',
        grid: { show: true, stroke: '#1c2430', width: 1 },
        ticks: { show: false },
        values: (u, vals) => vals.map((v) => (v == null ? '--' : fmtClock(v))),
        font: '10px ui-monospace, Consolas, monospace',
        size: 46,
      },
      {
        stroke: '#5c6878',
        grid: { show: true, stroke: '#1c2430', width: 1 },
        ticks: { show: false },
        font: '10px ui-monospace, Consolas, monospace',
        size: 34,
        values: (u, vals) => vals.map((v) => (v == null ? '--' : String(Math.round(v)))),
      },
    ],
    series: [
      {},
      {
        label: 'CPU %',
        stroke: '#6ca0f6',
        width: 1.6,
        fill: 'rgba(108,160,246,0.13)',
        points: { show: false },
      },
      {
        label: 'MEM %',
        stroke: '#7ee787',
        width: 1.6,
        points: { show: false },
      },
    ],
  }
}

function updateData(): void {
  if (!chart) return
  const series = getStatSeries(props.containerId)
  const t: number[] = []
  const cpu: number[] = []
  const mem: number[] = []
  for (const s of series) {
    t.push(s.t / 1000)
    cpu.push(s.cpu)
    mem.push(s.mem)
  }
  chart.setData([t, cpu, mem] as unknown as uPlot.AlignedData, false)
}

onMounted(() => {
  if (!el.value) return
  const rect = el.value.getBoundingClientRect()
  chart = new uPlot(makeOptions(Math.max(200, rect.width), props.height ?? 118), [[]] as unknown as uPlot.AlignedData, el.value)
  updateData()
  observer = new ResizeObserver((entries) => {
    if (!chart) return
    for (const entry of entries) {
      const w = Math.max(200, entry.contentRect.width)
      chart.setSize({ width: w, height: props.height ?? 118 })
    }
  })
  observer.observe(el.value)
})

watch(statsFrame, () => {
  updateData()
})

onBeforeUnmount(() => {
  observer?.disconnect()
  observer = null
  chart?.destroy()
  chart = null
})
</script>

<template>
  <div ref="el" class="stat-chart" />
</template>

<style scoped>
.stat-chart {
  width: 100%;
}
</style>

<style>
/* uPlot 全局主题微调（非 scoped：uPlot 自建 DOM） */
.wozzle-chart .legend {
  font-size: 10.5px;
  color: #8b98a9;
  margin: 2px 4px 0 4px;
}
.wozzle-chart .legend .series th {
  font-weight: 500;
}
.wozzle-chart .legend.inline tr {
  margin-right: 10px;
}
.wozzle-chart .u-cursor-x,
.wozzle-chart .u-cursor-y {
  display: none;
}
</style>
