<script setup lang="ts">
import { computed, ref, watch, type Component } from 'vue'
import { Globe, MapPin, Megaphone, Server, UserRound } from '@lucide/vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import CopyButton from '@/components/CopyButton.vue'
import { useAppStore, type AppConfig } from '@/stores/app'

const appStore = useAppStore()

interface InfoItem {
  key: keyof AppConfig
  label: string
  icon: Component
  copyable: boolean
}

const items: InfoItem[] = [
  { key: 'location', label: 'server_location', icon: MapPin, copyable: false },
  { key: 'public_ipv4', label: 'ipv4_address', icon: Server, copyable: true },
  { key: 'public_ipv6', label: 'ipv6_address', icon: Globe, copyable: true },
  { key: 'my_ip', label: 'my_address', icon: UserRound, copyable: true }
]

const visibleItems = computed(() =>
  items
    .map((item) => ({ ...item, value: appStore.config?.[item.key] }))
    .filter(
      (item): item is InfoItem & { value: string } =>
        typeof item.value === 'string' && item.value !== ''
    )
)

// The Markdown renderer bundles highlight.js; only load it when a sponsor message exists.
const sponsorMessage = ref('')
async function renderSponsor(message: string): Promise<void> {
  if (!message) {
    sponsorMessage.value = ''
    return
  }
  const { renderMarkdown } = await import('@/helper/markdown')
  if (message === appStore.config?.sponsor_message) sponsorMessage.value = renderMarkdown(message)
}
watch(
  () => appStore.config?.sponsor_message ?? '',
  (message) => void renderSponsor(message),
  { immediate: true }
)
</script>

<template>
  <Card class="gap-0 py-0">
    <CardHeader class="border-b px-5 py-4 [.border-b]:pb-4">
      <CardTitle class="text-sm">{{ $t('server_info') }}</CardTitle>
    </CardHeader>
    <CardContent class="divide-y px-0">
      <div
        v-for="item in visibleItems"
        :key="item.key"
        class="group flex items-center gap-3 px-5 py-3"
        :data-testid="`info-${item.key}`"
      >
        <div
          class="bg-muted text-muted-foreground flex size-8 shrink-0 items-center justify-center rounded-md"
        >
          <component :is="item.icon" class="size-4" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="text-muted-foreground text-xs">{{ $t(item.label) }}</div>
          <div
            class="truncate text-sm font-medium"
            :class="item.copyable && 'font-mono'"
            :title="item.value"
          >
            {{ item.value }}
          </div>
        </div>
        <CopyButton
          v-if="item.copyable"
          :value="item.value"
          class="opacity-60 group-hover:opacity-100"
        />
      </div>
    </CardContent>
  </Card>

  <Card v-if="sponsorMessage" class="gap-0 py-0">
    <CardHeader class="border-b px-5 py-4 [.border-b]:pb-4">
      <CardTitle class="flex items-center gap-2 text-sm">
        <Megaphone class="text-primary size-4" />
        {{ $t('sponsor_message') }}
      </CardTitle>
    </CardHeader>
    <CardContent class="px-5 py-4">
      <!-- markdown-it runs with html disabled and rejects unsafe link protocols -->
      <div class="prose-md" v-html="sponsorMessage" />
    </CardContent>
  </Card>
</template>
