<script setup lang="ts">
import { computed } from 'vue'
import { getStatSeries, statsFrame, STATS_WINDOW_MS } from '../stores/stats'

/**
 * Dozzle 式柱状 sparkline：
 * - 固定桶数下采样，3px 柱 / 2px 间距 / 圆角顶，高 16px
 * - 柱高按窗口内最大值归一化（空闲容器也有可读的起伏，不再贴地）
 * - 颜色按 Dozzle 阈值：≤50 绿 / ≤70 蓝 / ≤90 琥珀 / >90 红
 */
const props = defineProps<{
  containerId: string
  kind: 'cpu' | 'mem'
}>()

const BUCKETS = 26
const BAR_PITCH = 5 // 3px 柱 + 2px 间距

const bars = computed(() => {
  void statsFrame() // 依赖帧计数器：每帧 stats 重新计算
  const series = getStatSeries(props.containerId)
  if (series.length === 0) return [] as Array<{ h: number; tone: string }>
  const cutoff = Date.now() - STATS_WINDOW_MS
  const vals: number[] = []
  for (const s of series) {
    if (s.t < cutoff) continue
    vals.push(props.kind === 'cpu' ? s.cpu : s.mem)
  }
  if (vals.length === 0) return [] as Array<{ h: number; tone: string }>
  const out: number[] = []
  const per = Math.max(1, Math.ceil(vals.length / BUCKETS))
  for (let i = 0; i < vals.length; i += per) {
    const slice = vals.slice(i, i + per)
    let sum = 0
    for (const v of slice) sum += v
    out.push(sum / slice.length)
  }
  while (out.length > BUCKETS) out.shift()
  const max = Math.max(...out)
  return out.map((v) => {
    const tone = v > 90 ? 'hot' : v > 70 ? 'warm' : v > 50 ? 'mild' : 'ok'
    const h = max > 0 ? Math.max(v > 0 ? 4 : 0, Math.round((v / max) * 100)) : 0
    return { h, tone }
  })
})
</script>

<template>
  <div class="stat-bars" :style="{ width: BUCKETS * BAR_PITCH - 2 + 'px' }">
    <div v-for="(b, i) in bars" :key="i" class="bar" :class="b.tone" :style="{ height: b.h + '%' }" />
  </div>
</template>

<style scoped>
.stat-bars {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  height: 16px;
  flex: none;
}
.stat-bars .bar {
  width: 3px;
  border-radius: 2px 2px 0 0;
  opacity: 0.9;
}
.stat-bars .bar.ok {
  background: var(--green);
}
.stat-bars .bar.mild {
  background: #6ca0f6;
}
.stat-bars .bar.warm {
  background: var(--amber);
}
.stat-bars .bar.hot {
  background: var(--red);
}
</style>
