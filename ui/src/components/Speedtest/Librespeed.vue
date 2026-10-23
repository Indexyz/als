<script setup lang="ts">
import { computed, onUnmounted, ref, shallowRef } from 'vue'
import type { ApexOptions } from 'apexcharts'
import VueApexCharts from 'vue3-apexcharts'
import { ArrowDown, ArrowUp, Play, Square } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { useChartTheme } from '@/composables/chart'
import { useAppStore } from '@/stores/app'

interface WorkerStatus {
  testState: number
  dlStatus: string
  ulStatus: string
  dlProgress: number
  ulProgress: number
}

const MAX_CHART_POINTS = 100
const appStore = useAppStore()
const { baseOptions } = useChartTheme()

let workerInstance: Worker | null = null
let workerTimer: ReturnType<typeof setInterval> | null = null
const working = ref(false)
const hasResult = ref(false)
const state = ref(-1)
const download = ref('')
const upload = ref('')
const dlProgress = ref(0)
const ulProgress = ref(0)
const downloadPoints = shallowRef<number[]>([])
const uploadPoints = shallowRef<number[]>([])

const series = computed(() => [
  { name: 'Download', data: downloadPoints.value },
  { name: 'Upload', data: uploadPoints.value }
])
const chartOptions = computed<ApexOptions>(() => ({
  ...baseOptions.value,
  chart: { ...baseOptions.value.chart, id: 'librespeed', animations: { enabled: false } },
  xaxis: {
    type: 'numeric',
    labels: { show: false },
    axisBorder: { show: false },
    axisTicks: { show: false },
    tooltip: { enabled: false }
  },
  yaxis: {
    min: 0,
    forceNiceScale: true,
    labels: { formatter: (value: number) => `${Math.round(value)} Mbps` }
  },
  tooltip: { ...baseOptions.value.tooltip, x: { show: false } }
}))

const overallProgress = computed(() =>
  Math.round(((dlProgress.value + ulProgress.value) / 2) * 100)
)

const metrics = computed(() => [
  {
    key: 'download',
    label: 'librespeed_download',
    icon: ArrowDown,
    value: download.value,
    unit: 'Mbps',
    active: working.value && state.value === 1,
    accent: 'text-chart-1'
  },
  {
    key: 'upload',
    label: 'librespeed_upload',
    icon: ArrowUp,
    value: upload.value,
    unit: 'Mbps',
    active: working.value && state.value === 3,
    accent: 'text-chart-2'
  }
])

function stopSpeedtest(): void {
  if (workerTimer !== null) {
    clearInterval(workerTimer)
    workerTimer = null
  }
  if (workerInstance !== null) {
    workerInstance.onmessage = null
    workerInstance.postMessage('abort')
    workerInstance.terminate()
    workerInstance = null
  }
  working.value = false
}

function append(points: typeof downloadPoints, value: number): void {
  const next = [...points.value, value]
  points.value = next.length > MAX_CHART_POINTS ? next.slice(-MAX_CHART_POINTS) : next
}

function handleStatus(data: WorkerStatus): void {
  state.value = data.testState
  if (data.testState >= 4) {
    stopSpeedtest()
    return
  }
  dlProgress.value = data.dlProgress || 0
  ulProgress.value = data.ulProgress || 0
  if (data.testState === 1 && data.dlStatus) {
    download.value = data.dlStatus
    append(downloadPoints, Number(data.dlStatus))
  } else if (data.testState === 3 && data.ulStatus) {
    upload.value = data.ulStatus
    append(uploadPoints, Number(data.ulStatus))
  }
}

function startOrStopSpeedtest(): void {
  if (workerInstance !== null) {
    stopSpeedtest()
    return
  }
  download.value = ''
  upload.value = ''
  dlProgress.value = 0
  ulProgress.value = 0
  downloadPoints.value = []
  uploadPoints.value = []
  working.value = true
  hasResult.value = true
  const worker = new Worker('./speedtest_worker.js')
  workerInstance = worker
  worker.onmessage = (e: MessageEvent<string>) => {
    if (workerInstance !== worker) return
    try {
      handleStatus(JSON.parse(e.data) as WorkerStatus)
    } catch (error) {
      console.error('Invalid speedtest worker payload', error)
    }
  }
  const session = appStore.sessionId ?? ''
  worker.postMessage(
    'start ' +
      JSON.stringify({
        test_order: 'D_U',
        url_dl: `./session/${session}/speedtest/download`,
        url_ul: `./session/${session}/speedtest/upload`,
        url_ping: `./session/${session}/speedtest/upload`
      })
  )
  workerTimer = setInterval(() => {
    if (workerInstance === worker) worker.postMessage('status')
  }, 200)
}

onUnmounted(stopSpeedtest)
</script>

<template>
  <div class="space-y-5 px-5 pb-5 sm:px-6 sm:pb-6">
    <div class="grid grid-cols-2 gap-3">
      <div
        v-for="metric in metrics"
        :key="metric.key"
        class="bg-muted/30 rounded-lg border p-3.5 transition-colors"
        :class="metric.active && 'border-primary/40 bg-primary/5'"
        :data-testid="`librespeed-${metric.key}`"
      >
        <div class="text-muted-foreground flex items-center gap-1.5 text-xs">
          <component :is="metric.icon" class="size-3.5" :class="metric.accent" />
          {{ $t(metric.label) }}
        </div>
        <div class="mt-1.5 flex items-baseline gap-1">
          <span
            class="tabular font-mono text-3xl font-semibold tracking-tight"
            :class="!metric.value && 'text-muted-foreground/50'"
          >
            {{ metric.value || '-' }}
          </span>
          <span class="text-muted-foreground text-xs">{{ metric.unit }}</span>
        </div>
      </div>
    </div>

    <div v-show="hasResult" class="space-y-3">
      <div class="h-48">
        <VueApexCharts type="area" height="100%" :options="chartOptions" :series="series" />
      </div>
      <Progress v-if="working" :model-value="overallProgress" class="h-1" />
    </div>

    <Button
      size="lg"
      class="w-full gap-2 sm:w-auto"
      :variant="working ? 'outline' : 'default'"
      @click="startOrStopSpeedtest"
    >
      <template v-if="working">
        <Square class="fill-current" />{{ $t('librespeed_stop') }}
      </template>
      <template v-else> <Play class="fill-current" />{{ $t('librespeed_begin') }} </template>
    </Button>
  </div>
</template>
