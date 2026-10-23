<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight, ChevronDown, CircleAlert, Loader2, Square, TerminalSquare } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { cn } from '@/lib/utils'
import { useAppStore, type FeatureKey } from '@/stores/app'
import {
  diagnosticURL,
  formatMilliseconds,
  isValidTarget,
  normalizeTarget,
  latencyTone,
  lossTone,
  rowKey,
  summarizePing,
  updateDiagnosticRows,
  type DiagnosticFamily,
  type DiagnosticMessage,
  type DiagnosticRow,
  type DiagnosticTool,
  type LatencyTone
} from '@/helper/diagnostics'

type Status =
  'idle' | 'connecting' | 'running' | 'stopping' | 'completed' | 'stopped' | 'timeout' | 'error'

interface Column {
  key: string
  label: string
  align?: 'left' | 'right'
  className?: string
  text: (row: DiagnosticRow) => string
  tone?: (row: DiagnosticRow) => LatencyTone
  mono?: boolean
}

const appStore = useAppStore()
const { t } = useI18n()
const target = ref('')
const tool = ref<DiagnosticTool>('ping')
const family = ref<DiagnosticFamily>('auto')
const status = ref<Status>('idle')
const errorCode = ref('')
const touched = ref(false)
const rows = ref<DiagnosticRow[]>([])
const output = ref<string[]>([])
const address = ref('')
const runTool = ref<DiagnosticTool>('ping')
const runTarget = ref('')
const outputOpen = ref(false)
let socket: WebSocket | undefined
let connectTimer: ReturnType<typeof setTimeout> | undefined
let runTimer: ReturnType<typeof setTimeout> | undefined
let stopTimer: ReturnType<typeof setTimeout> | undefined
let finished = false

const allTools: { label: string; value: DiagnosticTool }[] = [
  { label: 'Ping', value: 'ping' },
  { label: 'Traceroute', value: 'traceroute' },
  { label: 'MTR', value: 'mtr' },
  { label: 'NextTrace', value: 'nexttrace' }
]
const tools = computed(() =>
  allTools.filter((entry) => appStore.isEnabled(`feature_${entry.value}` as FeatureKey))
)
const families = computed<{ label: string; value: DiagnosticFamily }[]>(() => [
  { label: t('diagnostics.auto'), value: 'auto' },
  { label: 'IPv4', value: '4' },
  { label: 'IPv6', value: '6' }
])
watch(
  tools,
  (options) => {
    const first = options[0]
    if (first && !options.some((entry) => entry.value === tool.value)) tool.value = first.value
  },
  { immediate: true }
)

// ToggleGroup emits undefined when the active item is clicked again; keep a selection.
const toolModel = computed({
  get: () => tool.value,
  set: (value: unknown) => {
    if (typeof value === 'string' && value) tool.value = value as DiagnosticTool
  }
})
const familyModel = computed({
  get: () => family.value,
  set: (value: unknown) => {
    if (typeof value === 'string' && value) family.value = value as DiagnosticFamily
  }
})

const working = computed(() => ['connecting', 'running', 'stopping'].includes(status.value))
// Whitespace is trimmed and IDNs are converted to Punycode; validation and the request use the
// cleaned value, and it is written back to the input on blur and submit.
const normalizedTarget = computed(() => normalizeTarget(target.value))
const invalid = computed(
  () => touched.value && normalizedTarget.value !== '' && !isValidTarget(normalizedTarget.value)
)
const canStart = computed(
  () => isValidTarget(normalizedTarget.value) && Boolean(tool.value) && !appStore.connecting
)
function commitTarget(): void {
  touched.value = true
  if (target.value !== normalizedTarget.value) target.value = normalizedTarget.value
}
const knownErrors = new Set([
  'invalid_request',
  'invalid_target',
  'invalid_family',
  'invalid_tool',
  'disabled',
  'busy',
  'resolve_failed',
  'unavailable',
  'execution_failed',
  'output_limit',
  'protocol_error',
  'connection_failed'
])

function cleanup(): void {
  clearTimeout(connectTimer)
  clearTimeout(runTimer)
  clearTimeout(stopTimer)
  const old = socket
  socket = undefined
  if (old) old.close()
}
function fail(code: string | undefined): void {
  errorCode.value = code && knownErrors.has(code) ? code : 'execution_failed'
  status.value = 'error'
  finished = true
  cleanup()
}
function handleMessage(message: DiagnosticMessage): void {
  switch (message.type) {
    case 'started':
      address.value = message.address ?? ''
      break
    case 'output':
      output.value.push(message.line || '')
      if (output.value.length > 1000) output.value.shift()
      break
    case 'row':
    case 'snapshot':
      rows.value = updateDiagnosticRows(rows.value, runTool.value, message)
      break
    case 'error':
      fail(message.code)
      break
    case 'done':
      status.value =
        (['completed', 'stopped', 'timeout', 'error'] as const).find(
          (value) => value === message.status
        ) ?? 'error'
      finished = true
      cleanup()
      break
  }
}
function start(): void {
  if (working.value) return
  commitTarget()
  if (!isValidTarget(target.value)) return
  if (!tool.value || !appStore.sessionId || appStore.connecting) return
  cleanup()
  rows.value = []
  output.value = []
  address.value = ''
  errorCode.value = ''
  runTool.value = tool.value
  runTarget.value = target.value
  finished = false
  status.value = 'connecting'
  let ws: WebSocket
  try {
    ws = new WebSocket(diagnosticURL(window.location.href, appStore.sessionId))
  } catch {
    fail('connection_failed')
    return
  }
  socket = ws
  connectTimer = setTimeout(() => fail('connection_failed'), 15000)
  runTimer = setTimeout(() => {
    status.value = 'timeout'
    finished = true
    cleanup()
  }, 135000)
  ws.onopen = () => {
    if (socket !== ws) return
    clearTimeout(connectTimer)
    status.value = 'running'
    ws.send(
      JSON.stringify({
        type: 'start',
        tool: runTool.value,
        target: runTarget.value,
        family: family.value
      })
    )
  }
  ws.onmessage = ({ data }: MessageEvent<string>) => {
    if (socket !== ws) return
    let message: DiagnosticMessage
    try {
      message = JSON.parse(data) as DiagnosticMessage
    } catch {
      fail('protocol_error')
      return
    }
    handleMessage(message)
  }
  ws.onerror = () => {
    if (socket === ws) fail('connection_failed')
  }
  ws.onclose = () => {
    if (socket === ws && !finished) fail('connection_failed')
  }
}
function stop(): void {
  if (!working.value || status.value === 'stopping') return
  if (socket?.readyState === WebSocket.OPEN) {
    status.value = 'stopping'
    socket.send(JSON.stringify({ type: 'stop' }))
    stopTimer = setTimeout(() => {
      status.value = 'stopped'
      finished = true
      cleanup()
    }, 5000)
  } else {
    status.value = 'stopped'
    finished = true
    cleanup()
  }
}
onUnmounted(cleanup)
watch(
  () => appStore.sessionId,
  () => {
    if (working.value) fail('connection_failed')
  }
)

const noReply = (host: string | undefined) => host === '*' || host === '???' || !host
const ms =
  (key: keyof DiagnosticRow): Column['text'] =>
  (row) =>
    row.loss_percent === 100 ? '—' : formatMilliseconds(row[key])

const columns = computed<Column[]>(() => {
  const hop: Column = {
    key: 'hop',
    label: '#',
    className: 'w-12',
    text: (row) => String((runTool.value === 'ping' ? row.seq : row.hop) ?? ''),
    mono: true
  }
  const host: Column = {
    key: 'host',
    label: t('diagnostics.host'),
    className: 'min-w-44',
    text: (row) => (noReply(row.host) ? t('diagnostics.no_reply') : (row.host ?? '')),
    mono: true
  }
  const metric = (label: string, key: keyof DiagnosticRow): Column => ({
    key,
    label: t(`diagnostics.${label}`),
    align: 'right',
    className: 'w-24',
    text: ms(key),
    tone: (row) => (row.loss_percent === 100 ? 'none' : latencyTone(row[key])),
    mono: true
  })
  if (runTool.value === 'ping')
    return [
      hop,
      host,
      {
        key: 'ttl',
        label: 'TTL',
        align: 'right',
        className: 'w-16',
        text: (row) => String(row.ttl ?? '—'),
        mono: true
      },
      {
        key: 'latency',
        label: t('diagnostics.latency'),
        align: 'right',
        className: 'w-32',
        text: (row) =>
          row.timeout ? t('diagnostics.no_reply') : formatMilliseconds(row.latency_ms),
        tone: (row) => (row.timeout ? 'poor' : latencyTone(row.latency_ms)),
        mono: true
      }
    ]
  if (runTool.value === 'mtr')
    return [
      hop,
      host,
      {
        key: 'loss',
        label: t('diagnostics.loss'),
        align: 'right',
        className: 'w-20',
        text: (row) =>
          typeof row.loss_percent === 'number' ? `${row.loss_percent.toFixed(1)}%` : '—',
        tone: (row) => lossTone(row.loss_percent),
        mono: true
      },
      {
        key: 'sent',
        label: t('diagnostics.sent'),
        align: 'right',
        className: 'w-16',
        text: (row) => String(row.sent ?? '—'),
        mono: true
      },
      metric('last', 'last_ms'),
      metric('average', 'avg_ms'),
      metric('best', 'best_ms'),
      metric('worst', 'worst_ms'),
      { ...metric('stddev', 'stddev_ms'), tone: undefined }
    ]
  const result: Column[] = [
    hop,
    host,
    {
      key: 'rtts',
      label: t('diagnostics.latency'),
      className: 'min-w-52',
      text: (row) =>
        [
          ...(row.rtts_ms ?? []).map(formatMilliseconds),
          ...Array.from({ length: Math.min(row.timeouts ?? 0, 3) }, () => '*')
        ].join(' / ') || '—',
      tone: (row) => latencyTone(row.rtts_ms?.length ? Math.min(...row.rtts_ms) : null),
      mono: true
    }
  ]
  if (runTool.value === 'nexttrace')
    result.push(
      { key: 'asn', label: 'ASN', className: 'w-24', text: (row) => row.asn || '—', mono: true },
      {
        key: 'location',
        label: t('diagnostics.location'),
        className: 'min-w-48',
        text: (row) => [row.location, row.owner].filter(Boolean).join(' · ') || '—'
      }
    )
  return result
})

const toneClass: Record<LatencyTone, string> = {
  good: 'text-success',
  fair: 'text-warning',
  poor: 'text-destructive',
  none: ''
}

const pingSummary = computed(() =>
  runTool.value === 'ping' && rows.value.length ? summarizePing(rows.value) : null
)
const summaryItems = computed<{ label: string; value: string; tone: LatencyTone }[]>(() => {
  const summary = pingSummary.value
  if (!summary) return []
  return [
    { label: t('diagnostics.sent'), value: String(summary.sent), tone: 'none' },
    {
      label: t('diagnostics.loss'),
      value: `${summary.loss.toFixed(1)}%`,
      tone: lossTone(summary.loss)
    },
    {
      label: t('diagnostics.best'),
      value: formatMilliseconds(summary.min),
      tone: latencyTone(summary.min)
    },
    {
      label: t('diagnostics.average'),
      value: formatMilliseconds(summary.avg),
      tone: latencyTone(summary.avg)
    },
    {
      label: t('diagnostics.worst'),
      value: formatMilliseconds(summary.max),
      tone: latencyTone(summary.max)
    }
  ]
})

const statusDot = computed(() => {
  switch (status.value) {
    case 'connecting':
    case 'running':
    case 'stopping':
      return 'bg-primary animate-pulse'
    case 'completed':
      return 'bg-success'
    case 'timeout':
    case 'error':
      return 'bg-destructive'
    default:
      return 'bg-muted-foreground/50'
  }
})
</script>

<template>
  <Card v-if="tools.length" class="gap-0 overflow-hidden py-0" :aria-label="t('network_tools')">
    <form class="space-y-5 p-5 sm:p-6" novalidate @submit.prevent="start">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 class="text-base font-semibold tracking-tight">{{ t('network_tools') }}</h2>
          <p class="text-muted-foreground mt-1 text-sm">{{ t('diagnostics.description') }}</p>
        </div>
        <ToggleGroup
          v-model="toolModel"
          type="single"
          variant="outline"
          size="sm"
          :disabled="working"
          :aria-label="t('diagnostics.tool')"
          class="bg-muted/50 w-full sm:w-auto"
        >
          <ToggleGroupItem
            v-for="entry in tools"
            :key="entry.value"
            :value="entry.value"
            class="data-[state=on]:bg-background data-[state=on]:text-primary flex-1 px-3 text-xs font-medium sm:flex-none"
          >
            {{ entry.label }}
          </ToggleGroupItem>
        </ToggleGroup>
      </div>

      <div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto]">
        <div class="space-y-1.5">
          <label for="diagnostic-target" class="text-sm font-medium">{{
            t('diagnostics.target')
          }}</label>
          <Input
            id="diagnostic-target"
            v-model="target"
            class="h-11 font-mono text-base md:text-sm"
            autocomplete="off"
            autocapitalize="off"
            spellcheck="false"
            :disabled="working"
            :placeholder="t('diagnostics.placeholder')"
            :aria-invalid="invalid || undefined"
            :aria-describedby="invalid ? 'diagnostic-target-error' : undefined"
            @blur="commitTarget"
          />
        </div>
        <div class="flex items-end">
          <Button
            v-if="!working"
            type="submit"
            size="lg"
            class="h-11 w-full gap-2 sm:w-40"
            :disabled="!canStart"
          >
            {{ t('diagnostics.start') }}
            <ArrowRight />
          </Button>
          <Button
            v-else
            type="button"
            size="lg"
            variant="destructive"
            class="h-11 w-full gap-2 sm:w-40"
            :disabled="status === 'stopping'"
            @click="stop"
          >
            <Square class="fill-current" />
            {{ t('diagnostics.stop') }}
          </Button>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
        <span class="text-muted-foreground text-xs">{{ t('diagnostics.family') }}</span>
        <ToggleGroup
          v-model="familyModel"
          type="single"
          size="sm"
          :disabled="working"
          :aria-label="t('diagnostics.family')"
        >
          <ToggleGroupItem
            v-for="entry in families"
            :key="entry.value"
            :value="entry.value"
            class="data-[state=on]:border-input data-[state=on]:bg-background h-7 border border-transparent px-2.5 text-xs data-[state=on]:shadow-xs"
          >
            {{ entry.label }}
          </ToggleGroupItem>
        </ToggleGroup>
        <p
          v-if="invalid"
          id="diagnostic-target-error"
          role="alert"
          class="text-destructive basis-full text-xs"
        >
          {{ t('diagnostics.errors.invalid_target') }}
        </p>
      </div>
    </form>

    <div
      class="bg-muted/40 flex min-h-10 flex-wrap items-center gap-x-3 gap-y-1 border-t px-5 py-2 font-mono text-xs sm:px-6"
      aria-live="polite"
      data-testid="diagnostic-status"
    >
      <span class="flex items-center gap-2">
        <span class="size-1.5 rounded-full" :class="statusDot" />
        <span>{{ t(`diagnostics.status.${status}`) }}</span>
      </span>
      <span v-if="runTarget" class="text-muted-foreground min-w-0 truncate">
        {{ runTool }} · {{ runTarget
        }}<template v-if="address && address !== runTarget"> → {{ address }}</template>
      </span>
      <Loader2 v-if="working" class="text-muted-foreground ml-auto size-3.5 animate-spin" />
    </div>

    <div
      v-if="errorCode"
      role="alert"
      class="border-destructive/30 bg-destructive/5 text-destructive flex items-start gap-2 border-t px-5 py-3 text-sm sm:px-6"
    >
      <CircleAlert class="mt-0.5 size-4 shrink-0" />
      {{ t(`diagnostics.errors.${errorCode}`) }}
    </div>

    <div v-if="rows.length" class="border-t">
      <div
        v-if="summaryItems.length"
        class="grid grid-cols-3 divide-x border-b sm:grid-cols-5"
        data-testid="ping-summary"
      >
        <div v-for="item in summaryItems" :key="item.label" class="px-4 py-3 sm:px-5">
          <div class="text-muted-foreground text-[11px] tracking-wide uppercase">
            {{ item.label }}
          </div>
          <div class="tabular mt-0.5 font-mono text-sm font-semibold" :class="toneClass[item.tone]">
            {{ item.value }}
          </div>
        </div>
      </div>
      <div class="max-h-[480px] overflow-auto">
        <Table class="text-[13px]">
          <TableHeader class="bg-card sticky top-0 z-10 shadow-[inset_0_-1px_0_var(--border)]">
            <TableRow class="hover:bg-transparent">
              <TableHead
                v-for="column in columns"
                :key="column.key"
                :class="
                  cn(
                    'text-muted-foreground h-9 text-xs font-medium first:pl-5 last:pr-5 sm:first:pl-6 sm:last:pr-6',
                    column.align === 'right' && 'text-right',
                    column.className
                  )
                "
              >
                {{ column.label }}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="row in rows" :key="rowKey(runTool, row)">
              <TableCell
                v-for="column in columns"
                :key="column.key"
                :class="
                  cn(
                    'tabular py-2 first:pl-5 last:pr-5 sm:first:pl-6 sm:last:pr-6',
                    column.mono && 'font-mono',
                    column.align === 'right' && 'text-right',
                    column.key === 'hop' && 'text-muted-foreground',
                    column.key === 'host' && 'break-all whitespace-normal',
                    column.key === 'host' && noReply(row.host) && 'text-muted-foreground italic',
                    column.key === 'location' && 'text-muted-foreground whitespace-normal',
                    column.tone && toneClass[column.tone(row)]
                  )
                "
              >
                {{ column.text(row) }}
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </div>
    <div
      v-else
      class="text-muted-foreground flex flex-col items-center justify-center gap-2 border-t px-6 py-12 text-center text-sm"
    >
      <Loader2 v-if="working" class="size-5 animate-spin" />
      <TerminalSquare v-else class="size-6 opacity-60" />
      {{ t(working ? 'diagnostics.waiting' : 'diagnostics.empty') }}
    </div>

    <p
      v-if="(runTool === 'mtr' && working) || status === 'completed'"
      class="text-muted-foreground border-t px-5 py-2.5 text-xs sm:px-6"
    >
      {{ t(runTool === 'mtr' && working ? 'diagnostics.mtr_hint' : 'diagnostics.complete_hint') }}
    </p>

    <Collapsible v-if="output.length" v-model:open="outputOpen" class="border-t">
      <CollapsibleTrigger
        class="hover:bg-muted/50 flex w-full items-center gap-2 px-5 py-2.5 text-left text-xs font-medium sm:px-6"
      >
        <ChevronDown
          class="size-3.5 transition-transform"
          :class="outputOpen ? 'rotate-0' : '-rotate-90'"
        />
        {{ t('diagnostics.raw_output') }}
        <span class="text-muted-foreground ml-auto font-mono">{{ output.length }}</span>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <pre
          class="bg-terminal text-terminal-foreground max-h-72 overflow-auto px-5 py-4 font-mono text-xs leading-relaxed break-all whitespace-pre-wrap sm:px-6"
          >{{ output.join('\n') }}</pre>
      </CollapsibleContent>
    </Collapsible>
  </Card>
</template>
