<script setup>
import { computed, defineAsyncComponent, shallowRef } from 'vue'
import { useAppStore } from '@/stores/app'
import Diagnostics from './Utilities/Diagnostics.vue'

const appStore = useAppStore()
const activeTool = shallowRef(null)
const tools = [
  {
    label: 'IPerf3',
    feature: 'feature_iperf3',
    component: defineAsyncComponent(() => import('./Utilities/IPerf3.vue'))
  },
  {
    label: 'Speedtest.net',
    feature: 'feature_speedtest_dot_net',
    component: defineAsyncComponent(() => import('./Utilities/SpeedtestNet.vue'))
  }
]
const enabledTools = computed(() => tools.filter((tool) => appStore.config?.[tool.feature]))
const hasDiagnostics = computed(() =>
  ['ping', 'traceroute', 'mtr', 'nexttrace'].some((tool) => appStore.config?.[`feature_${tool}`])
)
const drawerOpen = computed({
  get: () => activeTool.value !== null,
  set: (value) => {
    if (!value) activeTool.value = null
  }
})
</script>

<template>
  <n-card v-if="hasDiagnostics || enabledTools.length">
    <template #header>{{ $t('network_tools') }}</template>
    <Diagnostics v-if="hasDiagnostics" />
    <n-divider v-if="hasDiagnostics && enabledTools.length" />
    <n-space v-if="enabledTools.length">
      <n-button v-for="tool in enabledTools" :key="tool.label" @click="activeTool = tool">{{
        tool.label
      }}</n-button>
    </n-space>
  </n-card>
  <n-drawer v-model:show="drawerOpen" :width="appStore.drawerWidth" placement="right">
    <n-drawer-content :title="activeTool?.label" closable>
      <component :is="activeTool?.component" v-if="activeTool" @closed="activeTool = null" />
    </n-drawer-content>
  </n-drawer>
</template>
