<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowDown, ArrowUp, ExternalLink, Loader2, Play, Timer } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { formatBytes } from '@/helper/unit'
import { useAppStore } from '@/stores/app'

type Phase = '' | 'ping' | 'download' | 'upload'

type SpeedtestEvent =
  | { type: 'queue'; pos: number; totalPos: number }
  | {
      type: 'testStart'
      server: { id: number | string; name: string; country: string; location: string }
    }
  | { type: 'ping'; ping: { latency: number } }
  | { type: 'download'; download: { bandwidth: number; progress: number } }
  | { type: 'upload'; upload: { bandwidth: number; progress: number } }
  | {
      type: 'result'
      result: { url: string }
      download: { bandwidth: number }
      upload: { bandwidth: number }
    }

const appStore = useAppStore()
const { t } = useI18n()

let abortController: AbortController | null = null
let listeningSource: EventSource | null = null

const working = ref(false)
const serverId = ref('')
const isQueue = ref(false)
const started = ref(false)
const phase = ref<Phase>('')
const queue = ref({ pos: 0, total: 0 })
const progress = ref({ sub: 0, full: 0 })
const result = ref({
  ping: '',
  download: '',
  upload: '',
  url: '',
  server: { id: '', name: '', pos: '' }
})

const resetResult = () => {
  result.value = {
    ping: '',
    download: '',
    upload: '',
    url: '',
    server: { id: '', name: '', pos: '' }
  }
}

function handleEvent(data: SpeedtestEvent): void {
  switch (data.type) {
    case 'queue':
      isQueue.value = true
      queue.value = { pos: data.pos, total: data.totalPos }
      break
    case 'testStart':
      isQueue.value = false
      result.value.server = {
        id: String(data.server.id),
        name: data.server.name,
        pos: `${data.server.country} - ${data.server.location}`
      }
      break
    case 'ping':
      phase.value = 'ping'
      result.value.ping = String(data.ping.latency)
      break
    case 'download':
      phase.value = 'download'
      result.value.download = formatBytes(data.download.bandwidth, 2, true)
      progress.value.sub = Math.round(data.download.progress * 100)
      progress.value.full = Math.round(progress.value.sub / 2)
      break
    case 'upload':
      phase.value = 'upload'
      result.value.upload = formatBytes(data.upload.bandwidth, 2, true)
      progress.value.sub = Math.round(data.upload.progress * 100)
      progress.value.full = 50 + Math.round(progress.value.sub / 2)
      break
    case 'result':
      result.value.url = data.result.url
      result.value.download = formatBytes(data.download.bandwidth, 2, true)
      result.value.upload = formatBytes(data.upload.bandwidth, 2, true)
      progress.value = { sub: 100, full: 100 }
      break
  }
}

const handleMessage = (e: MessageEvent<string>) => {
  try {
    handleEvent(JSON.parse(e.data) as SpeedtestEvent)
  } catch (error) {
    console.error('Invalid speedtest SSE payload', error)
  }
}

const removeMessageListener = () => {
  listeningSource?.removeEventListener('SpeedtestStream', handleMessage)
  listeningSource = null
}

const addMessageListener = () => {
  removeMessageListener()
  if (appStore.source) {
    appStore.source.addEventListener('SpeedtestStream', handleMessage)
    listeningSource = appStore.source
  }
}

watch(
  () => appStore.source,
  () => {
    if (working.value) addMessageListener()
  }
)

async function speedtest(): Promise<void> {
  if (working.value) return
  const controller = new AbortController()
  abortController = controller
  working.value = true
  started.value = true
  isQueue.value = false
  phase.value = ''
  progress.value = { sub: 0, full: 0 }
  resetResult()
  addMessageListener()
  try {
    await appStore.requestMethod(
      'speedtest_dot_net',
      { node_id: serverId.value },
      controller.signal
    )
  } catch {}
  if (abortController === controller) {
    removeMessageListener()
    abortController = null
    working.value = false
    isQueue.value = false
  }
}

onUnmounted(() => {
  abortController?.abort()
  abortController = null
  removeMessageListener()
})

const phaseLabel = computed(() =>
  phase.value ? t(`speedtest_net.progress_${phase.value}`) : t('speedtest_net.progress')
)

const metrics = computed(() => [
  {
    key: 'ping',
    label: 'speedtest_net.latency',
    icon: Timer,
    value: result.value.ping ? `${result.value.ping} ms` : ''
  },
  {
    key: 'download',
    label: 'speedtest_net.download',
    icon: ArrowDown,
    value: result.value.download
  },
  { key: 'upload', label: 'speedtest_net.upload', icon: ArrowUp, value: result.value.upload }
])

const serverRows = computed(() => [
  { label: 'speedtest_net.server_id_label', value: result.value.server.id },
  { label: 'speedtest_net.server_location', value: result.value.server.pos },
  { label: 'speedtest_net.server_name', value: result.value.server.name }
])
</script>

<template>
  <div class="space-y-5">
    <form class="flex gap-2" @submit.prevent="speedtest">
      <Input
        v-model="serverId"
        class="h-10 font-mono"
        inputmode="numeric"
        :disabled="working"
        :placeholder="t('speedtest_net.server_id')"
      />
      <Button type="submit" class="h-10 gap-2 px-5" :disabled="working">
        <Loader2 v-if="working" class="animate-spin" />
        <Play v-else class="fill-current" />
        {{ t('speedtest_net.run') }}
      </Button>
    </form>

    <div
      v-if="isQueue"
      class="bg-muted/50 text-muted-foreground flex items-center gap-2 rounded-lg border p-4 text-sm"
    >
      <Loader2 class="size-4 animate-spin" />
      {{ t('speedtest_net.queue', { pos: queue.pos, total: queue.total }) }}
    </div>
    <div
      v-else-if="working && !phase"
      class="bg-muted/50 text-muted-foreground flex items-center gap-2 rounded-lg border p-4 text-sm"
    >
      <Loader2 class="size-4 animate-spin" />
      {{ t('speedtest_net.starting') }}
    </div>

    <template v-if="started && (phase || result.server.id)">
      <div class="grid grid-cols-3 gap-3">
        <div v-for="metric in metrics" :key="metric.key" class="bg-muted/30 rounded-lg border p-3">
          <div class="text-muted-foreground flex items-center gap-1 text-xs">
            <component :is="metric.icon" class="size-3.5" />
            {{ t(metric.label) }}
          </div>
          <div class="tabular mt-1 truncate font-mono text-base font-semibold sm:text-lg">
            {{ metric.value || '—' }}
          </div>
        </div>
      </div>

      <div v-if="working && phase" class="space-y-3">
        <div class="space-y-1.5">
          <div class="flex justify-between text-xs">
            <span>{{ phaseLabel }}</span>
            <span class="tabular font-mono">{{ progress.sub }}%</span>
          </div>
          <Progress :model-value="progress.sub" class="h-1.5" />
        </div>
        <div class="space-y-1.5">
          <div class="text-muted-foreground flex justify-between text-xs">
            <span>{{ t('speedtest_net.total_progress') }}</span>
            <span class="tabular font-mono">{{ progress.full }}%</span>
          </div>
          <Progress :model-value="progress.full" class="h-1.5" />
        </div>
      </div>

      <dl v-if="result.server.id" class="divide-y rounded-lg border text-sm">
        <div v-for="row in serverRows" :key="row.label" class="flex gap-4 px-4 py-2.5">
          <dt class="text-muted-foreground w-28 shrink-0">{{ t(row.label) }}</dt>
          <dd class="min-w-0 break-words">{{ row.value }}</dd>
        </div>
      </dl>

      <a
        v-if="result.url"
        :href="result.url"
        target="_blank"
        rel="noopener"
        class="group block overflow-hidden rounded-lg border bg-white p-2"
      >
        <img :src="`${result.url}.png`" alt="Speedtest.net result" class="mx-auto max-w-sm" />
        <span
          class="text-primary mt-2 flex items-center justify-center gap-1 text-xs group-hover:underline"
        >
          {{ t('speedtest_net.result') }} <ExternalLink class="size-3" />
        </span>
      </a>
    </template>
  </div>
</template>
