<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import type { ApexOptions } from 'apexcharts'
import VueApexCharts from 'vue3-apexcharts'
import { ArrowDown, ArrowUp, Activity, Loader2 } from '@lucide/vue'
import { Card } from '@/components/ui/card'
import { useChartTheme } from '@/composables/chart'
import { appendTrafficSample, parseTrafficEvent, type InterfaceTraffic } from '@/helper/traffic'
import { formatBytes } from '@/helper/unit'
import { useAppStore } from '@/stores/app'

interface CacheEntry {
  InterfaceName: string
  Caches: [number, number, number][] | null
}

const appStore = useAppStore()
const { baseOptions } = useChartTheme()
const interfaces = ref<Record<string, InterfaceTraffic>>({})

function addSample(name: string, time: number, rx: number, tx: number): void {
  interfaces.value[name] = appendTrafficSample(interfaces.value[name], name, time, rx, tx)
}

const handleCache = (e: MessageEvent<string>) => {
  let data: Record<string, CacheEntry | null>
  try {
    data = JSON.parse(e.data) as Record<string, CacheEntry | null>
  } catch {
    return
  }
  for (const entry of Object.values(data)) {
    if (!entry) continue
    for (const [time, rx, tx] of entry.Caches ?? []) addSample(entry.InterfaceName, time, rx, tx)
  }
}

const handleTrafficUpdate = (e: MessageEvent<string>) => {
  const sample = parseTrafficEvent(e.data)
  if (sample) addSample(sample.name, sample.time, sample.rx, sample.tx)
}

let listening: EventSource | undefined
function listen(source: EventSource | undefined): void {
  listening?.removeEventListener('InterfaceCache', handleCache)
  listening?.removeEventListener('InterfaceTraffic', handleTrafficUpdate)
  listening = source
  source?.addEventListener('InterfaceCache', handleCache)
  source?.addEventListener('InterfaceTraffic', handleTrafficUpdate)
}
onMounted(() => listen(appStore.source))
watch(() => appStore.source, listen)
onUnmounted(() => listen(undefined))

const sortedInterfaces = computed(() =>
  Object.values(interfaces.value).sort((a, b) => a.name.localeCompare(b.name))
)

const chartOptions = computed<ApexOptions>(() => ({
  ...baseOptions.value,
  chart: { ...baseOptions.value.chart, animations: { enabled: false } },
  xaxis: {
    type: 'datetime',
    labels: { datetimeUTC: false, format: 'HH:mm:ss', style: { fontSize: '10px' } },
    axisBorder: { show: false },
    axisTicks: { show: false },
    tooltip: { enabled: false }
  },
  yaxis: {
    min: 0,
    forceNiceScale: true,
    labels: {
      formatter: (value: number) => formatBytes(value, 1, true),
      style: { fontSize: '10px' }
    }
  },
  tooltip: {
    ...baseOptions.value.tooltip,
    x: { format: 'HH:mm:ss' },
    y: { formatter: (value: number) => formatBytes(value, 2, true) }
  }
}))

function seriesFor(iface: InterfaceTraffic) {
  return [
    { name: 'RX', data: iface.points.map((point) => [point.time, point.rx]) },
    { name: 'TX', data: iface.points.map((point) => [point.time, point.tx]) }
  ]
}
</script>

<template>
  <Card class="gap-0 overflow-hidden py-0">
    <div class="flex items-start justify-between gap-3 p-5 sm:p-6">
      <div>
        <h2 class="text-base font-semibold tracking-tight">{{ $t('server_bandwidth_graph') }}</h2>
        <p class="text-muted-foreground mt-1 text-sm">{{ $t('traffic_hint') }}</p>
      </div>
      <Activity class="text-primary size-5 animate-pulse" />
    </div>
    <div
      v-if="!sortedInterfaces.length"
      class="text-muted-foreground flex items-center justify-center gap-2 border-t py-10 text-sm"
    >
      <Loader2 class="size-4 animate-spin" />
      {{ $t('traffic_waiting') }}
    </div>
    <div v-else class="grid border-t" :class="sortedInterfaces.length > 1 && 'xl:grid-cols-2'">
      <section
        v-for="iface in sortedInterfaces"
        :key="iface.name"
        class="border-b p-5 last:border-b-0 sm:p-6 xl:odd:border-r xl:[&:nth-last-child(2):nth-child(odd)]:border-b-0"
        :data-testid="`traffic-${iface.name}`"
      >
        <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
          <span class="bg-muted rounded-md px-2 py-0.5 font-mono text-xs font-medium">
            {{ iface.name }}
          </span>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <div class="text-muted-foreground flex items-center gap-1 text-xs">
              <ArrowDown class="text-chart-1 size-3.5" />
              {{ $t('server_bandwidth_graph_receive') }}
            </div>
            <div class="tabular mt-0.5 font-mono text-lg font-semibold">
              {{ formatBytes(iface.points.at(-1)?.rx ?? 0, 2, true) }}
            </div>
            <div class="text-muted-foreground tabular font-mono text-[11px]">
              Σ {{ formatBytes(iface.totalRx) }}
            </div>
          </div>
          <div>
            <div class="text-muted-foreground flex items-center gap-1 text-xs">
              <ArrowUp class="text-chart-2 size-3.5" />
              {{ $t('server_bandwidth_graph_sended') }}
            </div>
            <div class="tabular mt-0.5 font-mono text-lg font-semibold">
              {{ formatBytes(iface.points.at(-1)?.tx ?? 0, 2, true) }}
            </div>
            <div class="text-muted-foreground tabular font-mono text-[11px]">
              Σ {{ formatBytes(iface.totalTx) }}
            </div>
          </div>
        </div>
        <div class="-mx-2 mt-3 h-40">
          <VueApexCharts
            type="area"
            height="100%"
            :options="chartOptions"
            :series="seriesFor(iface)"
          />
        </div>
      </section>
    </div>
  </Card>
</template>
