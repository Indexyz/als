<script setup>
import { computed, h, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import {
  diagnosticURL,
  formatMilliseconds,
  isValidTarget,
  rowKey,
  updateDiagnosticRows
} from '@/helper/diagnostics'

const appStore = useAppStore()
const { t } = useI18n()
const target = ref('')
const tool = ref('ping')
const family = ref('auto')
const status = ref('idle')
const errorCode = ref('')
const touched = ref(false)
const rows = ref([])
const output = ref([])
const address = ref('')
const runTool = ref('ping')
const runTarget = ref('')
let socket
let connectTimer
let runTimer
let stopTimer
let finished = false

const tools = computed(() =>
  [
    { label: 'Ping', value: 'ping' },
    { label: 'Traceroute', value: 'traceroute' },
    { label: 'MTR', value: 'mtr' },
    { label: 'NextTrace', value: 'nexttrace' }
  ].filter((entry) => appStore.config?.[`feature_${entry.value}`])
)
const families = computed(() => [
  { label: t('diagnostics.auto'), value: 'auto' },
  { label: 'IPv4', value: '4' },
  { label: 'IPv6', value: '6' }
])
watch(
  tools,
  (options) => {
    if (!options.some((entry) => entry.value === tool.value)) tool.value = options[0]?.value
  },
  { immediate: true }
)
const working = computed(() => ['connecting', 'running', 'stopping'].includes(status.value))
const invalid = computed(() => touched.value && !isValidTarget(target.value))
const statusType = computed(() =>
  status.value === 'error' || status.value === 'timeout'
    ? 'error'
    : working.value
      ? 'info'
      : 'default'
)
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

function cleanup() {
  clearTimeout(connectTimer)
  clearTimeout(runTimer)
  clearTimeout(stopTimer)
  const old = socket
  socket = undefined
  if (old) old.close()
}
function fail(code) {
  errorCode.value = knownErrors.has(code) ? code : 'execution_failed'
  status.value = 'error'
  finished = true
  cleanup()
}
function start() {
  if (working.value) return
  touched.value = true
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
  let ws
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
  ws.onmessage = ({ data }) => {
    if (socket !== ws) return
    let message
    try {
      message = JSON.parse(data)
    } catch {
      fail('protocol_error')
      return
    }
    switch (message.type) {
      case 'started':
        address.value = message.address
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
        status.value = ['completed', 'stopped', 'timeout', 'error'].includes(message.status)
          ? message.status
          : 'error'
        finished = true
        cleanup()
        break
    }
  }
  ws.onerror = () => {
    if (socket === ws) fail('connection_failed')
  }
  ws.onclose = () => {
    if (socket === ws && !finished) fail('connection_failed')
  }
}
function stop() {
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

const columns = computed(() => {
  const hop = { title: '#', key: runTool.value === 'ping' ? 'seq' : 'hop', width: 48 }
  const host = {
    title: t('diagnostics.host'),
    key: 'host',
    minWidth: 170,
    render: (row) =>
      h(
        'span',
        { class: 'diagnostic-host' },
        row.host === '*' || row.host === '???' ? t('diagnostics.no_reply') : row.host
      )
  }
  const metric = (name, key) => ({
    title: t(`diagnostics.${name}`),
    key,
    width: 105,
    render: (row) => (row.loss_percent === 100 ? '—' : formatMilliseconds(row[key]))
  })
  if (runTool.value === 'ping')
    return [
      hop,
      host,
      { title: 'TTL', key: 'ttl', width: 65, render: (row) => row.ttl ?? '—' },
      {
        title: t('diagnostics.latency'),
        key: 'latency_ms',
        width: 130,
        render: (row) =>
          row.timeout ? t('diagnostics.no_reply') : formatMilliseconds(row.latency_ms)
      }
    ]
  if (runTool.value === 'mtr')
    return [
      hop,
      host,
      {
        title: t('diagnostics.loss'),
        key: 'loss_percent',
        width: 85,
        render: (row) =>
          typeof row.loss_percent === 'number' ? `${row.loss_percent.toFixed(1)}%` : '—'
      },
      { title: t('diagnostics.sent'), key: 'sent', width: 65 },
      ...[
        ['last', 'last_ms'],
        ['average', 'avg_ms'],
        ['best', 'best_ms'],
        ['worst', 'worst_ms'],
        ['stddev', 'stddev_ms']
      ].map(([label, key]) => metric(label, key))
    ]
  const result = [
    hop,
    host,
    {
      title: t('diagnostics.latency'),
      key: 'rtts_ms',
      minWidth: 230,
      render: (row) =>
        [
          ...(row.rtts_ms || []).map(formatMilliseconds),
          ...Array.from({ length: Math.min(row.timeouts || 0, 3) }, () => '*')
        ].join(' / ') || '—'
    }
  ]
  if (runTool.value === 'nexttrace')
    result.push(
      { title: 'ASN', key: 'asn', width: 100, render: (row) => row.asn || '—' },
      {
        title: t('diagnostics.location'),
        key: 'location',
        minWidth: 160,
        render: (row) => [row.location, row.owner].filter(Boolean).join(' · ') || '—'
      }
    )
  return result
})
const tableWidth = computed(() =>
  runTool.value === 'mtr' ? 1050 : runTool.value === 'nexttrace' ? 900 : 550
)
</script>

<template>
  <section v-if="tools.length" class="diagnostics" :aria-label="t('network_tools')">
    <n-text depth="3">{{ t('diagnostics.description') }}</n-text>
    <form @submit.prevent="start">
      <div class="diagnostic-form">
        <label
          ><span>{{ t('diagnostics.tool') }}</span>
          <n-select
            v-model:value="tool"
            :options="tools"
            :disabled="working"
            :aria-label="t('diagnostics.tool')"
          />
        </label>
        <label class="diagnostic-target"
          ><span>{{ t('diagnostics.target') }}</span>
          <n-input
            v-model:value="target"
            :disabled="working"
            :placeholder="t('diagnostics.placeholder')"
            :status="invalid ? 'error' : undefined"
            :aria-label="t('diagnostics.target')"
            :maxlength="254"
            @blur="touched = true"
          />
        </label>
        <label
          ><span>{{ t('diagnostics.family') }}</span>
          <n-select
            v-model:value="family"
            :options="families"
            :disabled="working"
            :aria-label="t('diagnostics.family')"
          />
        </label>
        <n-button
          v-if="!working"
          type="primary"
          attr-type="submit"
          :disabled="!isValidTarget(target) || !tool || appStore.connecting"
          >{{ t('diagnostics.start') }}</n-button
        >
        <n-button v-else type="error" :disabled="status === 'stopping'" @click="stop">{{
          t('diagnostics.stop')
        }}</n-button>
      </div>
      <n-text v-if="invalid" type="error" role="alert">{{
        t('diagnostics.errors.invalid_target')
      }}</n-text>
      <n-text v-else depth="3" class="diagnostic-hint">{{ t('diagnostics.input_hint') }}</n-text>
    </form>
    <n-alert v-if="errorCode" type="error" role="alert">{{
      t(`diagnostics.errors.${errorCode}`)
    }}</n-alert>
    <div class="diagnostic-status" aria-live="polite">
      <n-spin v-if="working" :size="16" />
      <n-tag :type="statusType" size="small">{{ t(`diagnostics.status.${status}`) }}</n-tag>
      <n-text v-if="runTarget"
        >{{ runTarget
        }}<template v-if="address && address !== runTarget"> → {{ address }}</template></n-text
      >
    </div>
    <n-data-table
      v-if="rows.length"
      :columns="columns"
      :data="rows"
      :row-key="(row) => rowKey(runTool, row)"
      :scroll-x="tableWidth"
      :max-height="420"
      size="small"
      :single-line="false"
    />
    <n-empty
      v-else
      :description="t(working ? 'diagnostics.waiting' : 'diagnostics.empty')"
      size="small"
      class="diagnostic-empty"
    />
    <n-text v-if="runTool === 'mtr' && working" depth="3">{{ t('diagnostics.mtr_hint') }}</n-text>
    <n-text v-if="status === 'completed'" depth="3">{{ t('diagnostics.complete_hint') }}</n-text>
    <n-collapse v-if="output.length">
      <n-collapse-item :title="t('diagnostics.raw_output')" name="output">
        <pre class="diagnostic-output">{{ output.join('\n') }}</pre>
      </n-collapse-item>
    </n-collapse>
  </section>
</template>

<style scoped>
.diagnostics {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}
.diagnostic-status {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.diagnostic-form {
  display: grid;
  grid-template-columns: 150px minmax(180px, 1fr) 120px auto;
  gap: 12px;
  align-items: end;
}
.diagnostic-form label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}
.diagnostic-hint {
  display: block;
  margin-top: 6px;
  font-size: 12px;
}
.diagnostic-empty {
  padding: 12px 0;
}
.diagnostic-output {
  margin: 0;
  max-height: 280px;
  overflow: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  font-size: 12px;
}
.diagnostics :deep(.diagnostic-host) {
  font-family: monospace;
  overflow-wrap: anywhere;
}
@media (max-width: 640px) {
  .diagnostic-form {
    grid-template-columns: 1fr 1fr;
  }
  .diagnostic-target {
    grid-column: 1 / -1;
    grid-row: 1;
  }
  .diagnostic-form > button {
    grid-column: 1 / -1;
  }
}
</style>
