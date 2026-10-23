<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import { Card } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
const FileSpeedtest = defineAsyncComponent(() => import('@/components/Speedtest/FileSpeedtest.vue'))
const Librespeed = defineAsyncComponent(() => import('@/components/Speedtest/Librespeed.vue'))
const hasLibrespeed = computed(() => appStore.isEnabled('feature_librespeed'))
const hasFileSpeedtest = computed(
  () =>
    appStore.isEnabled('feature_filespeedtest') && Boolean(appStore.config?.speedtest_files?.length)
)
</script>

<template>
  <Card v-if="hasLibrespeed || hasFileSpeedtest" class="gap-0 overflow-hidden py-0">
    <div class="p-5 sm:p-6">
      <h2 class="text-base font-semibold tracking-tight">{{ $t('server_speedtest') }}</h2>
      <p class="text-muted-foreground mt-1 text-sm">{{ $t('librespeed_hint') }}</p>
    </div>
    <Librespeed v-if="hasLibrespeed" />
    <Separator v-if="hasFileSpeedtest && hasLibrespeed" />
    <FileSpeedtest v-if="hasFileSpeedtest" />
  </Card>
</template>
