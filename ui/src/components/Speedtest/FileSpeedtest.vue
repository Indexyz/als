<script setup lang="ts">
import { computed } from 'vue'
import { Download } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { useAppStore } from '@/stores/app'

interface FileGroup {
  key: string
  title: string
  base: string
}

const appStore = useAppStore()
const files = computed(() => appStore.config?.speedtest_files ?? [])

const groups = computed<FileGroup[]>(() => {
  const config = appStore.config
  if (!config) return []
  const sessionPath = `session/${appStore.sessionId ?? ''}/speedtest/file/`
  const url = new URL(location.href)
  const port = url.port ? `:${url.port}` : ''
  const followDomain = Boolean(config.filetest_follow_domain)
  if ((!config.public_ipv4 && !config.public_ipv6) || followDomain) {
    return [{ key: 'default', title: 'file_speedtest', base: `./${sessionPath}` }]
  }
  const result: FileGroup[] = []
  if (config.public_ipv4)
    result.push({
      key: 'ipv4',
      title: 'file_ipv4_speedtest',
      base: `${url.protocol}//${config.public_ipv4}${port}/${sessionPath}`
    })
  if (config.public_ipv6)
    result.push({
      key: 'ipv6',
      title: 'file_ipv6_speedtest',
      base: `${url.protocol}//[${config.public_ipv6}]${port}/${sessionPath}`
    })
  return result
})
</script>

<template>
  <div class="space-y-4 p-5 sm:p-6">
    <div>
      <h3 class="text-sm font-semibold">{{ $t('file_speedtest') }}</h3>
      <p class="text-muted-foreground mt-0.5 text-xs">{{ $t('file_speedtest_hint') }}</p>
    </div>
    <div class="grid gap-4" :class="groups.length > 1 && 'md:grid-cols-2'">
      <div v-for="group in groups" :key="group.key" class="bg-muted/30 rounded-lg border p-3">
        <div v-if="groups.length > 1" class="text-muted-foreground mb-2.5 text-xs font-medium">
          {{ $t(group.title) }}
        </div>
        <div class="flex flex-wrap gap-2">
          <Button
            v-for="fileSize in files"
            :key="fileSize"
            variant="outline"
            size="sm"
            as="a"
            target="_blank"
            rel="noopener"
            class="font-mono text-xs"
            :href="`${group.base}${fileSize}.test`"
          >
            <Download class="text-muted-foreground" />
            {{ fileSize }}
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
