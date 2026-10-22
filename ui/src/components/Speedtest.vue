<script setup>
import { computed, defineAsyncComponent } from 'vue'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
const FileSpeedtest = defineAsyncComponent(() => import('@/components/Speedtest/FileSpeedtest.vue'))
const Librespeed = defineAsyncComponent(() => import('@/components/Speedtest/Librespeed.vue'))
const hasLibrespeed = computed(() => appStore.config?.feature_librespeed)
const hasFileSpeedtest = computed(() => appStore.config?.feature_filespeedtest)
</script>

<template>
  <n-card v-if="hasLibrespeed || hasFileSpeedtest">
    <template #header> {{ $t('server_speedtest') }} </template>
    <Librespeed v-if="hasLibrespeed" />
    <n-divider v-if="hasFileSpeedtest && hasLibrespeed" />
    <FileSpeedtest v-if="hasFileSpeedtest" />
  </n-card>
</template>
