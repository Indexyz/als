<script setup lang="ts">
import { computed, defineAsyncComponent, shallowRef, type Component } from 'vue'
import { ChevronRight, Gauge, Network } from '@lucide/vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle
} from '@/components/ui/sheet'
import { useAppStore, type FeatureKey } from '@/stores/app'

interface Tool {
  id: string
  label: string
  description: string
  icon: Component
  feature: FeatureKey
  component: Component
}

const appStore = useAppStore()
const activeTool = shallowRef<Tool | null>(null)

const tools: Tool[] = [
  {
    id: 'iperf3',
    label: 'iPerf3',
    description: 'iperf3.description',
    icon: Network,
    feature: 'feature_iperf3',
    component: defineAsyncComponent(() => import('./Utilities/IPerf3.vue'))
  },
  {
    id: 'speedtest-net',
    label: 'Speedtest.net',
    description: 'speedtest_net.description',
    icon: Gauge,
    feature: 'feature_speedtest_dot_net',
    component: defineAsyncComponent(() => import('./Utilities/SpeedtestNet.vue'))
  }
]
const enabledTools = computed(() => tools.filter((tool) => appStore.isEnabled(tool.feature)))

const open = computed({
  get: () => activeTool.value !== null,
  set: (value: boolean) => {
    if (!value) activeTool.value = null
  }
})
</script>

<template>
  <Card v-if="enabledTools.length" class="gap-0 py-0">
    <CardHeader class="border-b px-5 py-4 [.border-b]:pb-4">
      <CardTitle class="text-sm">{{ $t('more_tools') }}</CardTitle>
    </CardHeader>
    <CardContent class="space-y-1 p-2">
      <button
        v-for="tool in enabledTools"
        :key="tool.id"
        type="button"
        class="hover:bg-accent focus-visible:ring-ring/50 group flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left transition-colors outline-none focus-visible:ring-3"
        :aria-label="tool.label"
        @click="activeTool = tool"
      >
        <div
          class="bg-primary/10 text-primary flex size-8 shrink-0 items-center justify-center rounded-md"
        >
          <component :is="tool.icon" class="size-4" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="text-sm font-medium">{{ tool.label }}</div>
          <div class="text-muted-foreground truncate text-xs">{{ $t(tool.description) }}</div>
        </div>
        <ChevronRight
          class="text-muted-foreground size-4 transition-transform group-hover:translate-x-0.5"
        />
      </button>
    </CardContent>
  </Card>

  <Sheet v-model:open="open">
    <SheetContent class="w-full gap-0 sm:max-w-2xl">
      <SheetHeader class="border-b px-6 py-5">
        <SheetTitle class="flex items-center gap-2">
          <component :is="activeTool?.icon" v-if="activeTool" class="text-primary size-4" />
          {{ activeTool?.label }}
        </SheetTitle>
        <SheetDescription v-if="activeTool">{{ $t(activeTool.description) }}</SheetDescription>
      </SheetHeader>
      <div class="min-h-0 flex-1 overflow-y-auto p-6">
        <component :is="activeTool.component" v-if="activeTool" />
      </div>
    </SheetContent>
  </Sheet>
</template>
