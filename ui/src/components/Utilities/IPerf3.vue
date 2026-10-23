<script setup lang="ts">
import { computed, markRaw, onMounted, onUnmounted, ref, useTemplateRef } from 'vue'
import { useI18n } from 'vue-i18n'
import '@xterm/xterm/css/xterm.css'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { Loader2, Play, Square } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import CopyButton from '@/components/CopyButton.vue'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
const { t } = useI18n()
const working = ref(false)
const port = ref<string | null>(null)

const terminal = markRaw(
  new Terminal({
    convertEol: true,
    disableStdin: true,
    // xterm measures glyphs on a canvas, which does not understand generic stacks like
    // `ui-monospace`; list concrete fonts so cell width matches the rendered glyphs.
    fontFamily: "Menlo, Consolas, 'DejaVu Sans Mono', 'Liberation Mono', 'Courier New', monospace",
    fontSize: 12,
    // Fixed dark palette: the terminal stays dark in both light and dark themes.
    theme: { background: '#11151c', foreground: '#d5dae3', cursor: '#11151c' }
  })
)
const fitAddon = markRaw(new FitAddon())
const terminalEl = useTemplateRef<HTMLDivElement>('terminalContainer')
let resizeObserver: ResizeObserver | undefined

const commands = computed(() => {
  const config = appStore.config
  if (!port.value || !config) return []
  return [
    { label: 'IPv4', host: config.public_ipv4 },
    { label: 'IPv6', host: config.public_ipv6 }
  ]
    .filter((entry) => entry.host)
    .map((entry) => ({ label: entry.label, command: `iperf3 -c ${entry.host} -p ${port.value}` }))
})

const handlePortChange = (e: MessageEvent<string>) => {
  port.value = e.data
}

const handleMessage = (e: MessageEvent<string>) => {
  for (const line of e.data.split('\n')) terminal.writeln(line)
}

onMounted(() => {
  if (!terminalEl.value) return
  terminal.loadAddon(fitAddon)
  terminal.open(terminalEl.value)
  fitAddon.fit()
  resizeObserver = new ResizeObserver(() => fitAddon.fit())
  resizeObserver.observe(terminalEl.value)
})

let abortController: AbortController | null = null
let listeningSource: EventSource | undefined

function removeListeners(): void {
  listeningSource?.removeEventListener('Iperf3', handlePortChange)
  listeningSource?.removeEventListener('Iperf3Stream', handleMessage)
  listeningSource = undefined
}

async function startServer(): Promise<void> {
  working.value = true
  terminal.clear()
  port.value = null
  const controller = new AbortController()
  abortController = controller
  removeListeners()
  listeningSource = appStore.source
  listeningSource?.addEventListener('Iperf3', handlePortChange)
  listeningSource?.addEventListener('Iperf3Stream', handleMessage)

  try {
    await appStore.requestMethod('iperf3/server', {}, controller.signal)
  } catch {}

  if (abortController === controller) {
    abortController = null
    removeListeners()
    working.value = false
    port.value = null
  }
}

function stopServer(): void {
  if (!abortController) return
  abortController.abort('Stopped')
  abortController = null
  removeListeners()
  working.value = false
  port.value = null
  terminal.writeln(`\x1b[2m${t('iperf3.stopped')}\x1b[0m`)
}

onUnmounted(() => {
  stopServer()
  resizeObserver?.disconnect()
  terminal.dispose()
})
</script>

<template>
  <div class="flex h-full flex-col gap-4">
    <Button
      class="w-full gap-2"
      size="lg"
      :variant="working ? 'destructive' : 'default'"
      @click="working ? stopServer() : startServer()"
    >
      <template v-if="working"><Square class="fill-current" />{{ t('iperf3.stop') }}</template>
      <template v-else><Play class="fill-current" />{{ t('iperf3.start') }}</template>
    </Button>

    <div v-if="working && !port" class="text-muted-foreground flex items-center gap-2 text-sm">
      <Loader2 class="size-4 animate-spin" />{{ t('iperf3.waiting') }}
    </div>

    <div
      v-if="commands.length"
      class="bg-primary/5 border-primary/20 space-y-2 rounded-lg border p-4"
    >
      <p class="text-sm font-medium">{{ t('iperf3.connect_hint') }}</p>
      <div
        v-for="item in commands"
        :key="item.label"
        class="bg-background flex items-center gap-2 rounded-md border py-1 pr-1 pl-3"
      >
        <span class="text-muted-foreground w-9 shrink-0 text-[11px] font-medium">{{
          item.label
        }}</span>
        <code class="min-w-0 flex-1 truncate font-mono text-xs">{{ item.command }}</code>
        <CopyButton :value="item.command" />
      </div>
    </div>

    <div class="min-h-80 flex-1 overflow-hidden rounded-lg border bg-[#11151c]">
      <div class="flex items-center gap-1.5 border-b border-white/5 px-3 py-2">
        <span class="size-2.5 rounded-full bg-[#ff5f57]" />
        <span class="size-2.5 rounded-full bg-[#febc2e]" />
        <span class="size-2.5 rounded-full bg-[#28c840]" />
        <span class="ml-2 font-mono text-[11px] text-white/40">iperf3</span>
      </div>
      <div ref="terminalContainer" class="h-[calc(100%-33px)] min-h-72 p-3" />
    </div>
  </div>
</template>
