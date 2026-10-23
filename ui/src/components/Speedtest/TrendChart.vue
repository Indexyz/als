<script setup lang="ts">
import { computed, useId } from 'vue'
import { trendGeometry } from '@/helper/speedtest/curve'

const props = defineProps<{
  samples: readonly number[]
  /** Sample slots across the full width; the chart fills left to right. */
  capacity: number
  marker: number | null
  markerLabel: string
  tone: 'download' | 'upload'
}>()

const WIDTH = 100
const HEIGHT = 40
const gradientId = `trend-${useId()}`
const geometry = computed(() =>
  trendGeometry(
    props.samples,
    Math.max(props.samples.length, props.capacity),
    props.marker,
    WIDTH,
    HEIGHT
  )
)
const markerTop = computed(() =>
  geometry.value.markerY === null ? null : (geometry.value.markerY / HEIGHT) * 100
)
</script>

<template>
  <div
    class="relative h-full w-full"
    :class="tone === 'download' ? 'text-chart-1' : 'text-chart-2'"
  >
    <svg
      :viewBox="`0 0 ${WIDTH} ${HEIGHT}`"
      preserveAspectRatio="none"
      aria-hidden="true"
      class="block h-full w-full overflow-visible"
    >
      <defs>
        <linearGradient :id="gradientId" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stop-color="currentColor" stop-opacity="0.28" />
          <stop offset="100%" stop-color="currentColor" stop-opacity="0.02" />
        </linearGradient>
      </defs>
      <line
        x1="0"
        :y1="HEIGHT"
        :x2="WIDTH"
        :y2="HEIGHT"
        class="stroke-border"
        stroke-width="1"
        vector-effect="non-scaling-stroke"
      />
      <path v-if="geometry.area" :d="geometry.area" :fill="`url(#${gradientId})`" />
      <path
        v-if="geometry.line"
        :d="geometry.line"
        fill="none"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linejoin="round"
        vector-effect="non-scaling-stroke"
      />
      <line
        v-if="geometry.markerY !== null"
        x1="0"
        :y1="geometry.markerY"
        :x2="WIDTH"
        :y2="geometry.markerY"
        class="stroke-muted-foreground"
        stroke-width="1.25"
        stroke-dasharray="4 3"
        vector-effect="non-scaling-stroke"
      />
    </svg>
    <span
      v-if="markerTop !== null"
      class="text-muted-foreground pointer-events-none absolute right-3 -translate-y-full pb-0.5 text-[10px] font-medium tracking-wide"
      :style="{ top: `${markerTop}%` }"
    >
      {{ markerLabel }}
    </span>
  </div>
</template>
