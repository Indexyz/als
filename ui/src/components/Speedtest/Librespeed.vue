<script setup lang="ts">
import { computed, onUnmounted, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowDown, ArrowUp, Loader2, Play, RotateCw, Square } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import TrendChart from '@/components/Speedtest/TrendChart.vue'
import {
  NOT_STARTED,
  MULTI_STREAMS,
  createHttpTransport,
  runSpeedtest,
  type Phase,
  type ThreadMode,
  type Throughput
} from '@/helper/speedtest/engine'
import { MAX_DURATION, SAMPLE_INTERVAL } from '@/helper/speedtest/stability'
import { formatBytes } from '@/helper/unit'
import { useAppStore } from '@/stores/app'

type Direction = 'download' | 'upload'

const appStore = useAppStore()
const { t } = useI18n()
const TREND_CAPACITY = Math.round(MAX_DURATION / SAMPLE_INTERVAL)

const mode = ref<ThreadMode>('multi')
const phase = ref<Phase>('idle')
const latency = ref<number | null>(null)
const jitter = ref<number | null>(null)
const download = shallowRef<Throughput>(NOT_STARTED)
const upload = shallowRef<Throughput>(NOT_STARTED)
const failed = ref(false)
let controller: AbortController | null = null

const running = computed(() => ['latency', 'download', 'upload'].includes(phase.value))
const modeModel = computed({
  get: () => mode.value,
  set: (value: unknown) => {
    if (value === 'single' || value === 'multi') mode.value = value
  }
})

function reset(): void {
  latency.value = null
  jitter.value = null
  download.value = NOT_STARTED
  upload.value = NOT_STARTED
  failed.value = false
}

async function start(): Promise<void> {
  if (running.value || !appStore.sessionId) return
  reset()
  const run = new AbortController()
  controller = run
  const transport = createHttpTransport(
    `./session/${encodeURIComponent(appStore.sessionId)}/speedtest`
  )
  try {
    await runSpeedtest(transport, { mode: mode.value }, run.signal, {
      phase: (value) => (phase.value = value),
      latency: (ms) => (latency.value = ms),
      jitter: (ms) => (jitter.value = ms),
      download: (value) => (download.value = value),
      upload: (value) => (upload.value = value)
    })
  } catch {
    if (controller === run && !run.signal.aborted) failed.value = true
  } finally {
    if (controller === run) {
      controller = null
      if (phase.value !== 'done') phase.value = 'idle'
    }
  }
}

function stop(): void {
  controller?.abort()
  controller = null
  phase.value = 'idle'
}

onUnmounted(stop)

const hasRun = computed(() => latency.value !== null || download.value !== NOT_STARTED)

/** Split "123.45 Mbps" into the number and unit; bandwidth is Mbps on the wire. */
function speedParts(mbps: number | null): { value: string; unit: string } {
  if (mbps === null) return { value: '—', unit: 'Mbps' }
  if (mbps >= 1000) return { value: (mbps / 1000).toFixed(2), unit: 'Gbps' }
  return { value: mbps.toFixed(2), unit: 'Mbps' }
}

const ms = (value: number | null, digits = 0) => (value === null ? '—' : value.toFixed(digits))

const directions = computed(() =>
  (['download', 'upload'] as const).map((direction: Direction) => {
    const data = direction === 'download' ? download.value : upload.value
    return {
      direction,
      label: t(`librespeed_${direction}`),
      icon: direction === 'download' ? ArrowDown : ArrowUp,
      active: phase.value === direction,
      data,
      speed: speedParts(data.speed),
      stats: [
        {
          key: 'latency',
          label: t(`speedtest.${direction}_latency`),
          value: ms(data.latency),
          unit: 'ms'
        },
        {
          key: 'jitter',
          label: t(`speedtest.${direction}_jitter`),
          value: ms(data.jitter, 1),
          unit: 'ms'
        },
        {
          key: 'bytes',
          label: t(`speedtest.${direction === 'download' ? 'downloaded' : 'uploaded'}`),
          ...splitBytes(data.bytes)
        },
        {
          key: 'elapsed',
          label: t('speedtest.duration'),
          value: data.elapsed ? data.elapsed.toFixed(1) : '—',
          unit: 's'
        }
      ]
    }
  })
)

function splitBytes(bytes: number): { value: string; unit: string } {
  if (!bytes) return { value: '—', unit: 'MB' }
  const [value = '—', unit = ''] = formatBytes(bytes, 1).split(' ')
  return { value, unit }
}

const phaseLabel = computed(() => t(`speedtest.phase.${phase.value}`))
</script>

<template>
  <div class="space-y-4 px-5 pb-5 sm:px-6 sm:pb-6">
    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <Button
        size="lg"
        class="h-auto min-h-16 gap-2 text-base"
        :variant="running ? 'outline' : 'default'"
        :disabled="!appStore.sessionId"
        data-testid="speedtest-start"
        @click="running ? stop() : start()"
      >
        <template v-if="running"
          ><Square class="fill-current" />{{ t('librespeed_stop') }}</template
        >
        <template v-else-if="phase === 'done'"><RotateCw />{{ t('speedtest.again') }}</template>
        <template v-else><Play class="fill-current" />{{ t('librespeed_begin') }}</template>
      </Button>

      <div class="bg-muted/30 flex flex-col justify-center gap-1.5 rounded-lg border px-3.5 py-2.5">
        <span class="text-muted-foreground text-xs">{{ t('speedtest.connections') }}</span>
        <ToggleGroup
          v-model="modeModel"
          type="single"
          size="sm"
          :disabled="running"
          :aria-label="t('speedtest.connections')"
          class="w-full"
        >
          <ToggleGroupItem
            value="single"
            class="data-[state=on]:bg-background data-[state=on]:border-input h-7 flex-1 border border-transparent text-xs data-[state=on]:shadow-xs"
          >
            {{ t('speedtest.single') }}
          </ToggleGroupItem>
          <ToggleGroupItem
            value="multi"
            class="data-[state=on]:bg-background data-[state=on]:border-input h-7 flex-1 border border-transparent text-xs data-[state=on]:shadow-xs"
          >
            {{ t('speedtest.multi', { count: MULTI_STREAMS }) }}
          </ToggleGroupItem>
        </ToggleGroup>
      </div>

      <div
        v-for="stat in [
          { key: 'latency', label: t('speedtest.latency'), value: ms(latency) },
          { key: 'jitter', label: t('speedtest.jitter'), value: ms(jitter, 1) }
        ]"
        :key="stat.key"
        class="bg-muted/30 rounded-lg border px-3.5 py-2.5 transition-colors"
        :class="phase === 'latency' && 'border-primary/40 bg-primary/5'"
        :data-testid="`speedtest-${stat.key}`"
      >
        <div class="text-muted-foreground text-xs">{{ stat.label }}</div>
        <div class="mt-0.5 flex items-baseline gap-1">
          <span
            class="tabular font-mono text-2xl font-semibold"
            :class="stat.value === '—' && 'text-muted-foreground/50'"
            >{{ stat.value }}</span
          >
          <span class="text-muted-foreground text-xs">ms</span>
        </div>
      </div>
    </div>

    <div class="grid gap-3 md:grid-cols-2">
      <section
        v-for="item in directions"
        :key="item.direction"
        class="space-y-3"
        :data-testid="`speedtest-${item.direction}`"
      >
        <div
          class="bg-muted/30 overflow-hidden rounded-lg border transition-colors"
          :class="item.active && 'border-primary/40'"
        >
          <div class="px-4 pt-3.5">
            <div class="text-muted-foreground flex items-center gap-1.5 text-xs">
              <component
                :is="item.icon"
                class="size-3.5"
                :class="item.direction === 'download' ? 'text-chart-1' : 'text-chart-2'"
              />
              {{ item.label }}
              <Loader2 v-if="item.active" class="ml-auto size-3.5 animate-spin" />
            </div>
            <div class="mt-1 flex items-baseline gap-1.5">
              <span
                class="tabular font-mono text-4xl font-semibold tracking-tight"
                :class="item.data.speed === null && 'text-muted-foreground/50'"
                data-testid="speed-value"
                >{{ item.speed.value }}</span
              >
              <span class="text-muted-foreground text-sm">{{ item.speed.unit }}</span>
            </div>
          </div>
          <div class="mt-2 h-24">
            <TrendChart
              :samples="item.data.trend"
              :capacity="TREND_CAPACITY"
              :marker="item.data.sustained"
              :marker-label="t('speedtest.sustained')"
              :tone="item.direction"
            />
          </div>
        </div>
        <dl class="grid grid-cols-2 gap-3">
          <div
            v-for="stat in item.stats"
            :key="stat.key"
            class="bg-muted/30 rounded-lg border px-3.5 py-2.5"
            :data-testid="`speedtest-${item.direction}-${stat.key}`"
          >
            <dt class="text-muted-foreground truncate text-xs">{{ stat.label }}</dt>
            <dd class="mt-0.5 flex items-baseline gap-1">
              <span
                class="tabular font-mono text-lg font-semibold"
                :class="stat.value === '—' && 'text-muted-foreground/50'"
                >{{ stat.value }}</span
              >
              <span class="text-muted-foreground text-xs">{{ stat.unit }}</span>
            </dd>
          </div>
        </dl>
      </section>
    </div>

    <div class="text-muted-foreground flex min-h-5 items-center gap-2 text-xs" aria-live="polite">
      <template v-if="failed">
        <span class="text-destructive">{{ t('speedtest.failed') }}</span>
      </template>
      <template v-else-if="running">
        <Loader2 class="size-3.5 animate-spin" />{{ phaseLabel }}
      </template>
      <template v-else-if="hasRun">{{ t('speedtest.hint') }}</template>
    </div>
  </div>
</template>
