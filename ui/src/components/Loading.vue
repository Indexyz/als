<script setup lang="ts">
import { Loader2 } from '@lucide/vue'
import { Skeleton } from '@/components/ui/skeleton'
</script>

<template>
  <div class="grid gap-6 lg:grid-cols-[340px_minmax(0,1fr)]" aria-busy="true">
    <div class="space-y-4">
      <div class="bg-card space-y-4 rounded-xl border p-5 shadow-sm">
        <Skeleton class="h-4 w-32" />
        <div v-for="i in 4" :key="i" class="space-y-2">
          <Skeleton class="h-3 w-20" />
          <Skeleton class="h-5 w-full" />
        </div>
      </div>
    </div>
    <div
      class="bg-card flex min-h-80 flex-col items-center justify-center gap-3 rounded-xl border p-8 text-center shadow-sm"
    >
      <Loader2 class="text-primary size-6 animate-spin" />
      <p class="font-medium">{{ $t('connecting') }}</p>
      <p class="text-muted-foreground text-sm">{{ $t('connecting_hint') }}</p>
    </div>
  </div>
</template>
